package desktop

import (
	"errors"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/session"
	"github.com/kznagamori/go_MarkView/internal/watcher"
)

// 本ファイルは、文書を開く共通処理（IMP-192。open.go）の結果を App の状態へ反映する部分を持つ。
// open.go が 400 行の目安（IMP-011）を超えたため分けた。
//
// **target・showing・currentConfirmed・ツリールート・履歴・確認待ちを書き換えるのはここだけである**
// （IMP-190, IMP-192）。1 つの錠（mu）の内側にまとめ、「文書は差し替わったが対象はまだ前のもの」と
// いう中間状態を作らない。

// openCommit は、状態へ反映した結果のうち呼び出し元が錠の外で使うもの。
type openCommit struct {
	dto     *DocumentDTO
	err     error
	newRoot string           // ツリールートが変わったときだけ非空（FR-030）
	watcher *watcher.Watcher // 監視の切り替えは mu の外で行う（IMP-140）
	history *HistoryDTO      // 履歴に積んだときだけ非 nil。送出は錠の外で行う（IMP-323）
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
	pushed := false

	switch outcome {
	case outcomeShown:
		a.target = target
		a.showing = true
		a.currentConfirmed = req.confirmed
		c.dto, c.newRoot, pushed = a.commitOpen(doc, req, view)

	case outcomeState:
		a.target = target
		a.showing = false
		a.currentConfirmed = false
		c.newRoot, pushed = a.commitState(req, err, target)

		// ツリールートは状態画面でツリーを移した後の値を使う（IMP-192）。対象が分からないのは
		// 回復したパニックだけであり（openPanicked）、表示用パスも空にする。
		screen := &screenError{err: err}
		if target != "" {
			screen.displayPath, screen.outsideTree = session.DisplayPath(a.treeRoot, target)
			screen.targetKind = string(mdfile.KindOf(target))
		}
		c.err = screen

	default:
		// 表示を変えない失敗。確認待ちを捨てるだけで、ツリールートも履歴も動かない（FR-110）。
		// 残したままにすると、確認画面を閉じたあとの操作で OpenConfirmed が通ってしまう（IMP-314）。
		a.pendingConfirm = ""
		c.err = err
	}

	// 履歴に積んだら、送る値を同じ錠の内側で読む（IMP-323）。送出は呼び出し元が錠の外で行う。
	if pushed {
		h := a.nextHistoryDTO()
		c.history = &h
	}

	return c
}

// commitOpen は読み込んだ文書を状態へ反映し、DTO を組み立てる（IMP-192）。
//
// **呼び出し側は mu を保持していること。** pushed は履歴に積んだか（history:changed を送る。IMP-323）。
func (a *App) commitOpen(doc *document.Document, req openRequest, view documentView) (dto *DocumentDTO, newRoot string, pushed bool) {
	// ツリールートは、ファイルの出どころが利用者の明示的な指定である経路
	// でのみ変える（FR-030）。
	if changesTreeRoot(req.src) && a.setTreeRoot(filepath.Dir(doc.Path)) {
		newRoot = a.treeRoot
	}

	if pushesHistory(req.src) {
		a.history.Push(session.Entry{Path: doc.Path, Anchor: req.anchor})
		pushed = true
	}

	a.current = doc

	// 開けたので確認待ちは解消した。覚えたままにすると、まったく別の文書を
	// 表示している状態で OpenConfirmed が通ってしまう（IMP-314）。
	a.pendingConfirm = ""

	// 表示用パスは都度算出する。保持すると、ツリールートが変わったときに
	// 古い値が残る（IMP-025）。
	view.displayPath, view.outsideTree = session.DisplayPath(a.treeRoot, doc.Path)

	return newDocumentDTO(doc, view), newRoot, pushed
}

// commitState は状態画面（confirm-large / too-large / render-error）を出したときの反映を行う
// （IMP-192, IMP-314, FR-016, FR-051）。target は画面の対象（classifyOpen が決めた値）。
//
// **描画しないまま、ツリールートと履歴を成功時と同じ表（IMP-192）のとおり対象へ移す。**
// FR-016 は「確認画面を表示した時点でタイトルとパス表示を対象のものに更新し、履歴に積む。
// Alt+← で直前の文書へ戻れること」を求めており、積まないと戻る先が 1 つずれる。
//
//   - **状態画面の 3 種類とも、積む経路（ダイアログ・ドロップ・引数・ツリー・リンク）なら積む**
//     （BUG-024。4.75.0）。v1.1.0 は確認画面だけを積み、上限超過とレンダリングエラーの状態画面から
//     Alt+← を押すと直前の文書を飛ばした。**経路の規則（pushesHistory）は変えない**——履歴・
//     再読み込み・確認からは積まない。
//   - **ツリールートも、状態画面の 3 種類とも表のとおり動かす**（ダイアログ・ドロップ・引数なら
//     親ディレクトリへ。FR-010, FR-011, FR-030。4.76.0。利用者の判断）。v1.1.0 は確認画面のとき
//     だけ移しており、起動時の引数（IMP-193）とで結果が違っていた。Open anyway（openFromConfirm）は
//     確認のときに移し済みであり、二重に動かさない（IMP-314）。
//
// 確認待ちのパス（pendingConfirm）は確認画面のときだけ立て、ほかの状態画面では捨てる（IMP-314）。
//
// **current は差し替えない。** まだその文書を表示していない。監視は呼び出し元（commit と
// switchWatch）が外す——描画を始めていないファイルは FR-014 の対象外であり、前の文書は
// 表示対象ではなくなっている。**対象が分からない（回復したパニック。openPanicked）ときは何も
// 動かさない。**
//
// **呼び出し側は mu を保持していること。** pushed は履歴に積んだか（history:changed を送る。IMP-323）。
func (a *App) commitState(req openRequest, err error, target string) (newRoot string, pushed bool) {
	var sizeErr *document.SizeError
	if errors.As(err, &sizeErr) && errors.Is(sizeErr.Err, document.ErrNeedsConfirm) {
		a.pendingConfirm = sizeErr.Path
		a.pendingSource = req.src
	} else {
		a.pendingConfirm = ""
	}

	if target == "" {
		return "", false
	}

	if changesTreeRoot(req.src) && a.setTreeRoot(filepath.Dir(target)) {
		newRoot = a.treeRoot
	}
	if pushesHistory(req.src) {
		a.history.Push(session.Entry{Path: target, Anchor: req.anchor})
		pushed = true
	}

	return newRoot, pushed
}
