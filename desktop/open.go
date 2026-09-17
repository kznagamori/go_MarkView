package desktop

import (
	"crypto/sha256"
	"errors"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/session"
	"github.com/kznagamori/go_MarkView/internal/watcher"
)

// 本ファイルは文書を開く共通処理を持つ（IMP-192）。
//
// **FR-010 / FR-011 / FR-012 / FR-033 / FR-050 / FR-051 のすべてが、この 1 つの
// 内部処理を通る**（AR-060）。フロントエンドから任意のパスを開く汎用 API を
// 作らないのと同じ理由で、Go 側にも開く経路を 1 つしか置かない（IMP-300）。

// openRequest は open への指示（IMP-192）。
//
// アンカーと復元位置と鍵の引き継ぎを渡す必要があるため、位置引数ではなく構造体で
// 受ける。位置引数が並ぶと、呼び出し側でどの値がどの経路のためのものか読み取れなくなる。
type openRequest struct {
	path string
	src  openSource

	// anchor はアンカー付きリンク（./a.md#sec）を踏んだときのフラグメント（復号済み。IMP-302）。
	anchor string

	// scrollTop は openFromHistory で復元するスクロール位置（FR-051）。
	scrollTop int

	// confirmed は FR-016 の「Open anyway」。10 MB 超でも描画する。
	// 同じファイルの読み直しでは、呼び出し元が渡さなくても openLocked が同意を引き継ぐ。
	confirmed bool

	// trigger は DocumentDTO.Trigger（IMP-302）。空なら経路から決める（triggerOf）。
	trigger string

	// refKey が空でなければ、読んだ内容の要約が expectDigest と一致したときに鍵を引き継ぐ
	// （LoadOptions.RefKey / ExpectDigest。IMP-102）。書き込みの直後の読み直し（IMP-195 の 8）だけが渡す。
	refKey       string
	expectDigest [sha256.Size]byte
}

// openOutcome は開く処理の結果の 3 分類（IMP-192）。
//
// **画面の対象（target）・showing・監視・編集モードは、どれもこの 3 つで決める。**
// 判定を 1 か所（classifyOpen）に置き、ウィンドウタイトルも同じ値から決める。分けて書くと、
// 片方だけ直したときにタイトル・「エディタで開く」の対象・監視が食い違う（BUG-012）。
type openOutcome int

const (
	// outcomeKept は表示を変えない失敗（not-found / permission / not-markdown）。FR-110 の
	// 「直前の内容を維持」であり、画面の対象も監視も変えない。
	outcomeKept openOutcome = iota

	// outcomeShown は読み込みと変換に成功した。
	outcomeShown

	// outcomeState は状態画面（confirm-large / too-large / render-error）を出した。
	outcomeState
)

// openedBefore は、開く処理を始めた時点の表示の状態（IMP-192 の「直前の」）。
//
// current / showing / currentConfirmed を書き換えるのは ioMu の内側の開く処理だけであり、
// 控えた値は読み込みと変換の間に古くならない（IMP-190）。
type openedBefore struct {
	current   *document.Document
	showing   bool
	confirmed bool
}

// screenError は、状態画面を出した失敗に画面の対象の表示用パスを添える（IMP-307, IMP-192）。
//
// open は error を返し、呼び出し元が経路ごとに ErrorDTO へ写す（バインドの戻り値・リンク・
// ドロップ・監視の error イベント）。**写す場所がいくつあっても表示用パスが漏れないように、
// エラーそのものに載せて newErrorDTO が取り出す。** Unwrap を持つため、errors.Is / errors.As
// による分類（IMP-315）は包む前と変わらない。
type screenError struct {
	err         error
	displayPath string
	outsideTree bool
}

func (e *screenError) Error() string { return e.err.Error() }

func (e *screenError) Unwrap() error { return e.err }

// openCommit は、状態へ反映した結果のうち呼び出し元が錠の外で使うもの。
type openCommit struct {
	dto     *DocumentDTO
	err     error
	newRoot string           // ツリールートが変わったときだけ非空（FR-030）
	watcher *watcher.Watcher // 監視の切り替えは mu の外で行う（IMP-140）
}

// open は ioMu を取って openLocked を呼ぶ（IMP-192）。
//
// バインドメソッド・ドロップ・監視のイベントはこちらを呼ぶ。**ioMu を持っている呼び出し元
// （IMP-195 の書き込みの前後の読み直し）は openLocked を呼ぶ**——sync.Mutex は再入できず、
// open を呼ぶと止まる（IMP-190）。
func (a *App) open(req openRequest) (*DocumentDTO, error) {
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	return a.openLocked(req)
}

// openLocked は文書を開く唯一の内部処理（IMP-192）。**呼び出し元は ioMu を持っていること。**
//
// 経路による差異は IMP-192 の表に挙がった 3 つ（ツリールート・履歴・スクロール）だけとする。
// **この表以外の差異を持ち込まない。** 分岐が増えると、リンク遷移でツリールートが動かない
// という FR-030 の不変条件を壊しやすくなる。trigger による違い（同じ内容なら送らない・鍵の
// 引き継ぎ・同意の引き継ぎ）は IMP-192 の規則だけで決める。
//
// **(nil, nil) を返すのは、監視のイベントで読み直した内容が表示中のものと同じだったときだけ**
// である（IMP-321）。呼び出し元は何も送らない。
func (a *App) openLocked(req openRequest) (*DocumentDTO, error) {
	req.trigger = triggerOf(req)
	before := a.openedBefore()

	// 同意の引き継ぎ（FR-016）。確認して描画した 10 MB 超の文書を、保存や F5 のたびに確認画面へ
	// 戻さない。**書き込みの直後の読み直しが ErrNeedsConfirm になると、裏で編集モードが終わるのに
	// 画面は編集モードのまま残る**（FR-140, FR-143）。
	if inheritsConsent(before, req.path) {
		req.confirmed = true
	}

	// 読み込みと変換は mu の外で行う（ioMu の内側ではある）。10 MB 近い文書では時間がかかり、
	// その間 ioMu を取らないバインドメソッドを止める理由がない。renderer は状態を持たず、
	// 同時に呼んでよい（IMP-024）。
	doc, err := document.Load(a.renderer, req.path, document.LoadOptions{
		Confirmed:    req.confirmed,
		RefKey:       req.refKey,
		ExpectDigest: req.expectDigest,
	})

	// 書き込み（IMP-195 の 8）の直後に届く監視のイベントで、同じ再描画を繰り返さない。
	// **current を差し替えない**——差し替えると鍵だけが新しくなり、画面の目印と食い違って
	// 以後の指示がすべて Stale になる（IMP-192）。
	if err == nil && unchangedOnWatch(req, before, doc, a.edit.Deleted()) {
		return nil, nil
	}

	outcome, target := classifyOpen(req, doc, err)

	// 編集モードと取り消し履歴もここで決める（IMP-192 の表）。判断は EditSession が持ち
	// （IMP-109）、ここは呼ぶだけにする。edit は ioMu が守るため mu の外で呼ぶ（IMP-190）。
	view := documentView{trigger: req.trigger}
	switch outcome {
	case outcomeShown:
		view.sameDocument = sameDocument(before, doc)
		a.edit.Loaded(doc, view.sameDocument)

		// 返す値は Loaded の**後の**値とする（IMP-192）。
		view.scroll = scrollFor(req)
		view.editable = a.edit.CanStart(doc, true)
		view.editMode = a.edit.On()
		view.editSeq = a.edit.Seq()

	case outcomeState:
		a.edit.Left()
	}

	c := a.commit(req, doc, err, outcome, target, view)

	// 監視の切り替えと Wails の呼び出しは mu の外で行う（IMP-140, IMP-024）。ioMu の内側では
	// あるため、開く処理どうしの順に並ぶ（古い文書への Watch が後から追い越さない）。
	switchWatch(c.watcher, outcome, target)

	// 状態画面に対象ファイル名が出る場合も、タイトルはそれに合わせる。「描画したかどうか」
	// ではなく「いま何を開こうとしているか」を示す（UI-013）。表示を変えない失敗では、直前の
	// 表示のまま何も触らない（FR-110）。
	if outcome != outcomeKept {
		a.setWindowTitle(filepath.Base(target))
	}
	if c.newRoot != "" {
		a.emit(eventTreeRootChanged, c.newRoot)
	}

	return c.dto, c.err
}

// openedBefore は、開く処理を始める時点の表示の状態を控える（IMP-192）。
func (a *App) openedBefore() openedBefore {
	a.mu.Lock()
	defer a.mu.Unlock()

	return openedBefore{current: a.current, showing: a.showing, confirmed: a.currentConfirmed}
}

// inheritsConsent は、確認して開いた同意を引き継いで読むかを返す（FR-016, IMP-192）。
//
// 画面が current を表示していて（状態画面へ移った時点で同意は消える）、current を確認して
// 開いており、同じファイル（1.7）を開くときに限る。監視・再読み込み・書き込みの後の読み直し・
// 同じファイルの開き直し（ツリーやドロップ）のいずれも同じに扱う。
func inheritsConsent(before openedBefore, path string) bool {
	return before.showing && before.confirmed && before.current != nil &&
		session.SameFile(before.current.Path, absPath(path))
}

// unchangedOnWatch は、監視のイベントで読み直した内容が表示中のものと同じかを返す
// （FR-014, IMP-192, IMP-321）。**手動の再読み込み（reload）は、同じでも送る。**
//
// **削除の印が立っていれば、同じ内容でも偽とする。** 削除した後に同じ内容で戻る場合がある
// （git checkout、150 ms を超える削除と作成の保存）。送らないと、Go 側は削除の印が残って
// 編集モードを二度と始められず、フロントエンドは document:removed で淡色にしたボタンが戻らない。
func unchangedOnWatch(req openRequest, before openedBefore, doc *document.Document, deleted bool) bool {
	return req.trigger == triggerWatch && !deleted &&
		before.showing && before.current != nil && doc.Digest == before.current.Digest
}

// sameDocument は DocumentDTO.SameDocument を決める（1.7, IMP-192, IMP-302）。
//
// **直前が状態画面なら、同じファイルでも偽とする。** 状態画面へ移った時点で文書の切り替えが
// 起きており（DSP-352, IMP-109 の Left）、引き継ぐ状態が残っていない。current は前の文書の
// まま残っているため、current だけを見ると真になってしまう。
func sameDocument(before openedBefore, doc *document.Document) bool {
	return before.showing && before.current != nil &&
		session.SameFile(before.current.Path, doc.Path)
}

// classifyOpen は、読み込みの結果を 3 つに分け、画面の対象を返す（IMP-190, IMP-192）。
//
//	読み込みに成功した   → outcomeShown, 開いた文書の絶対パス
//	状態画面を出した     → outcomeState, その対象の絶対パス（描画していなくても）
//	表示を変えない失敗   → outcomeKept,  空文字（FR-110 の「直前の内容を維持」）
//
// **この判定を 2 か所に分けて書かない**（openOutcome）。
func classifyOpen(req openRequest, doc *document.Document, err error) (openOutcome, string) {
	if err == nil {
		// document.Load が絶対パスにしている（IMP-025）。
		return outcomeShown, doc.Path
	}

	if stateKindFor(newErrorDTO(req.path, err)) == stateWelcome {
		return outcomeKept, ""
	}

	// サイズ超過は絶対パスを持っている（document.SizeError）。
	var sizeErr *document.SizeError
	if errors.As(err, &sizeErr) {
		return outcomeState, sizeErr.Path
	}

	// 変換の失敗。呼び出し側は常に絶対パスを渡すが、target は絶対パスで
	// あることが前提のため（IMP-190）ここで確かめる。
	return outcomeState, absPath(req.path)
}

// absPath は絶対パスへ直す。直せなければ元の値を返す。
//
// **失敗を理由に空を返さない。** 空は「画面の対象を変えない」という別の
// 意味を持つ（classifyOpen）。
func absPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return abs
}

// commit は読み込みの結果を状態へ反映する（IMP-192）。
//
// **target・showing・currentConfirmed を書き換えるのはここだけである**（IMP-190）。1 つの錠の
// 内側にまとめ、「文書は差し替わったが対象はまだ前のもの」という中間状態を作らない。開くバインドで
// 回復したパニックも、outcomeState としてここを通す（openPanicked。IMP-310）。
//
//	結果               target       showing  currentConfirmed   監視（switchWatch）
//	読み込みに成功した  開いた文書   真       req.confirmed       開いた文書へ切り替える
//	状態画面を出した    その対象     偽       偽                  外す（BUG-012）
//	表示を変えない失敗  変えない     変えない 変えない            変えない
func (a *App) commit(req openRequest, doc *document.Document, err error, outcome openOutcome, target string, view documentView) openCommit {
	a.mu.Lock()
	defer a.mu.Unlock()

	c := openCommit{watcher: a.watcher}

	switch outcome {
	case outcomeShown:
		a.target = target
		a.showing = true
		a.currentConfirmed = req.confirmed
		c.dto, c.newRoot = a.commitOpen(doc, req, view)

	case outcomeState:
		a.target = target
		a.showing = false
		a.currentConfirmed = false
		c.newRoot = a.commitPending(req, err)

		// ツリールートは確認画面でツリーを移した後の値を使う（IMP-192）。対象が分からないのは
		// 回復したパニックだけであり（openPanicked）、表示用パスも空にする。
		screen := &screenError{err: err}
		if target != "" {
			screen.displayPath, screen.outsideTree = session.DisplayPath(a.treeRoot, target)
		}
		c.err = screen

	default:
		// 確認以外の失敗であり、確認待ちを捨てるだけでツリールートは動かない。
		a.commitPending(req, err)
		c.err = err
	}

	return c
}

// commitOpen は読み込んだ文書を状態へ反映し、DTO を組み立てる（IMP-192）。
//
// **呼び出し側は mu を保持していること。**
func (a *App) commitOpen(doc *document.Document, req openRequest, view documentView) (dto *DocumentDTO, newRoot string) {
	// ツリールートは、ファイルの出どころが利用者の明示的な指定である経路
	// でのみ変える（FR-030）。
	if changesTreeRoot(req.src) && a.setTreeRoot(filepath.Dir(doc.Path)) {
		newRoot = a.treeRoot
	}

	if pushesHistory(req.src) {
		a.history.Push(session.Entry{Path: doc.Path, Anchor: req.anchor})
	}

	a.current = doc

	// 開けたので確認待ちは解消した。覚えたままにすると、まったく別の文書を
	// 表示している状態で OpenConfirmed が通ってしまう（IMP-314）。
	a.pendingConfirm = ""

	// 表示用パスは都度算出する。保持すると、ツリールートが変わったときに
	// 古い値が残る（IMP-025）。
	view.displayPath, view.outsideTree = session.DisplayPath(a.treeRoot, doc.Path)

	return newDocumentDTO(doc, view), newRoot
}

// commitPending は読み込みに失敗した場合の確認待ちを反映する（IMP-314, FR-016）。
//
// 確認待ち（10 MB 超）のときは、**描画しないままツリールートと履歴だけを
// 対象へ移す**。FR-016 は「確認画面を表示した時点でタイトルとパス表示を対象の
// ものに更新し、履歴に積む。Alt+← で直前の文書へ戻れること」を求めており、
// 履歴に積まないと戻る先が 1 つずれる。適用する規則は成功時と同じ表に従う。
//
// **current は差し替えない。** まだその文書を表示していない。監視は呼び出し元（commit と
// switchWatch）が外す——描画を始めていないファイルは FR-014 の対象外であり、前の文書は
// 表示対象ではなくなっている。
//
// 確認以外の失敗では覚えていた値を捨てる。残したままにすると、確認画面を
// 閉じたあとの操作で OpenConfirmed が通ってしまう。
//
// **呼び出し側は mu を保持していること。**
func (a *App) commitPending(req openRequest, err error) (newRoot string) {
	var sizeErr *document.SizeError

	if !errors.As(err, &sizeErr) || !errors.Is(sizeErr.Err, document.ErrNeedsConfirm) {
		a.pendingConfirm = ""
		return ""
	}

	a.pendingConfirm = sizeErr.Path
	a.pendingSource = req.src

	if changesTreeRoot(req.src) && a.setTreeRoot(filepath.Dir(sizeErr.Path)) {
		newRoot = a.treeRoot
	}
	if pushesHistory(req.src) {
		a.history.Push(session.Entry{Path: sizeErr.Path, Anchor: req.anchor})
	}

	return newRoot
}

// openResult は open を呼び、結果を OpenResultDTO へ写す（IMP-192, IMP-308）。
//
// バインドメソッドはこちらを使う。open そのものは Go 側の内部処理であり、
// イベント送出（watch.go）からも呼ばれるため error のまま残す。
func (a *App) openResult(path string, req openRequest) OpenResultDTO {
	dto, err := a.open(req)

	return newOpenResult(path, dto, err)
}
