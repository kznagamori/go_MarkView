package desktop

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
)

// 本ファイルは編集モードのバインドメソッドを持つ（IMP-195, IMP-316, FR-140〜FR-144）。
//
// **ここに書くのは、錠を取ること、ファイルを読み書きする関数を呼ぶこと、イベントを送ること
// だけとする。** 編集モードの状態と、書き込んでよいかの判断は document.EditSession（IMP-109）が
// 持つ（IMP-012）。desktop は単体テストの対象外であり（UT-002）、判断をここに書くと
// 書き込みの安全性を担う部分が検証されない。外部エディタ（editor.go）と混ぜない。
//
// **全体を ioMu で包む**（IMP-190）。edit は ioMu が守る。**ioMu を持ったまま読み直すため、
// open ではなく openLocked を呼ぶ**——sync.Mutex は再入できず、open を呼ぶと止まる。
//
// **書き込みの戻り値に DocumentDTO を載せない**（AR-061）。表示は document:changed だけで届き、
// 戻り値とイベントの到着順は決まっていない（IMP-261）。

// SetEditMode は編集モードを始める・終える（IMP-310, IMP-195, FR-140）。
//
// **開始できなかった場合も失敗として通知しない**（ボタンは淡色のはずである。UI-021）。結果の
// 状態と版を返し、フロントエンドはそれを写す（IMP-260）。
//
// **編集モードを始めたら（編集モードでない状態から始めたとき）、表示中の文書が Markdown なら、同じ
// ioMu の内側で修飾の目印付きで描き直し、document:changed を送る**（IMP-195, IMP-120。4.75.0。利用者の
// 判断）。openLocked（trigger は reload）を呼び、**鍵は引き継がない**（描き直しは自分の書き込みでは
// ない。FR-143）。描き直しが届くまでの間に押したチェックボックスは Stale で戻る（通知しない）。
// **図のファイルでは描き直さない**——修飾の目印を付けない（FR-146 の対象外）ため、図を描き直すだけに
// なる。読み直しに失敗したら、書き込みと同じく error イベントを送る（reloadAfterEdit）。
// **編集モードを終えても描き直さない**（利用者の判断。残った修飾の目印は次の再描画で消え、編集モード
// でない指示は Go 側が拒む。IMP-109 の 1）。
func (a *App) SetEditMode(on bool) (res EditModeDTO) {
	defer a.recoverEditMode(&res)

	// 取らないと、文書を開く処理が edit を決めてから document:changed を送るまでの間に
	// 割り込み、状態が食い違う（IMP-195）。
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	if on {
		current, showing := a.shownDocument()
		// Start は既に編集モードの間も真を返す（ボタンの二重押しで履歴を消さない。IMP-109）。
		// 描き直すのは、いま始めたときだけにする。
		wasOn := a.edit.On()
		if a.edit.Start(current, showing) && !wasOn && current.Kind == mdfile.KindMarkdown {
			// TextRefs は開く処理が決める（編集モードの間・表示中・同じファイル。IMP-192）。
			a.reloadAfterEdit(current, openRequest{path: current.Path, src: openFromReload, trigger: triggerReload})
		}
	} else {
		a.edit.Stop()
	}

	return EditModeDTO{On: a.edit.On(), Seq: a.edit.Seq()}
}

// SetEditBoxOpen は、フロントエンドで編集欄（セル・コードブロック）が開いたか閉じたかを受け取る
// （IMP-310, IMP-195, FR-140。4.75.0）。
//
// **ドロップを拒むためにだけある。** 値は App.editBoxOpen に写し、ドロップの受け口（IMP-313）だけが
// 読む。Linux ではドロップが JavaScript を通らずに Go へ届くため、フロントエンドだけでは止められない。
// **書き込みの判断には使わない**（Go 側の判断は IMP-109）。フロントエンドから渡すのは真偽値だけで、
// パスも目印も受け取らない。通知しない（IMP-310）。フロントエンドは前の呼び出しの応答を待ってから
// 次を送る（到着順が入れ替わりうるため。IMP-264）。
func (a *App) SetEditBoxOpen(open bool) {
	defer recoverQuiet()

	a.mu.Lock()
	defer a.mu.Unlock()

	a.editBoxOpen = open
}

// SetTask はチェックボックスを書き換える（IMP-310, FR-141, FR-143）。
func (a *App) SetTask(ref string, checked bool) (res EditResultDTO) {
	defer a.recoverEdit(&res)

	return a.writeEdit(document.Op{Kind: document.OpTask, Ref: ref, Checked: checked})
}

// SetCell は表のセルを書き換える（IMP-310, FR-142, FR-143）。
func (a *App) SetCell(ref string, text string) (res EditResultDTO) {
	defer a.recoverEdit(&res)

	return a.writeEdit(document.Op{Kind: document.OpCell, Ref: ref, Text: text})
}

// SetCode はコードブロック・図の中身を書き換える（IMP-310, FR-145, FR-143）。
//
// text は編集欄の値（前置きを除き、改行は LF。IMP-122）。前置きと改行コードの書き戻し・囲いの延長・
// 確かめは document.PlanCode が行う。
func (a *App) SetCode(ref string, text string) (res EditResultDTO) {
	defer a.recoverEdit(&res)

	return a.writeEdit(document.Op{Kind: document.OpCode, Ref: ref, Text: text})
}

// ApplyFormat は選んだ文字に取り消し線・文字色を付ける・外す（IMP-310, FR-146, FR-147, FR-143）。
//
// start / end はブロックの見える文字の中の **Unicode の符号位置での位置**（半開区間）、visible はその
// ブロックの見える文字の全体（IMP-123, IMP-266, IMP-316）。Go 側が組み立てたものと 1 文字でも違えば
// Stale（FR-146）。style の値の検査は document.PlanFormat が行い、フロントエンドを信頼しない。
// **成功して手書きの取り消し線を全体で外したときは Notice に format-widened を載せる**（writeEdit）。
func (a *App) ApplyFormat(ref string, start, end int, visible string, style FormatStyleDTO) (res EditResultDTO) {
	defer a.recoverEdit(&res)

	return a.writeEdit(document.Op{
		Kind: document.OpFormat,
		Ref:  ref,
		Format: document.FormatOp{
			Start:   start,
			End:     end,
			Visible: visible,
			Style:   document.FormatStyle{Strike: style.Strike, Color: style.Color},
		},
	})
}

// UndoEdit は直前の書き換えを取り消す（IMP-310, FR-144）。
func (a *App) UndoEdit() (res EditResultDTO) {
	defer a.recoverEdit(&res)

	return a.writeEdit(document.Op{Kind: document.OpUndo})
}

// RedoEdit は取り消した書き換えをやり直す（IMP-310, FR-144）。
func (a *App) RedoEdit() (res EditResultDTO) {
	defer a.recoverEdit(&res)

	return a.writeEdit(document.Op{Kind: document.OpRedo})
}

// GetCellSource はセルの編集欄に入れるソースを返す（IMP-310, IMP-195, FR-142）。
//
// 書き込みの 1〜3 と同じ手順を踏む。**ファイルと把握している内容が食い違っていれば、書き込みの
// 4 と同じく edit-conflict として読み直す**——編集欄を開く前に食い違いに気づかせる。
//
// **読めない・古い指示・編集できないセルは Stale を真で返し、通知しない。** edit-failed の文言は
// 「Failed to save: <path>」であり、保存していない操作には合わない。編集欄が開かないだけで済み、
// ファイルの削除や権限の変化は監視と次の書き込みが伝える。
func (a *App) GetCellSource(ref string) (res CellSourceDTO) {
	defer recoverCellSource(&res)

	text, errDTO, stale := a.editSource(document.Op{Kind: document.OpCell, Ref: ref},
		func(_ *document.Document, raw []byte) (string, error) {
			return a.edit.CellSource(a.renderer, raw, ref)
		})

	return CellSourceDTO{Text: text, Stale: stale, Error: errDTO}
}

// GetCodeSource はコードブロック・図の編集欄に入れるソースを返す（IMP-310, IMP-195, FR-145）。
//
// GetCellSource と同じ手順を踏む（食い違えば edit-conflict として読み直す。読めない・古い指示は
// Stale）。**前置きが行ごとに違うブロック（document.ErrLocked）は code-locked を返す**——編集欄を
// 開かず、警告を出す（FR-145）。種類は表示中の文書のもの（Markdown の中のブロックか図のファイルか）。
func (a *App) GetCodeSource(ref string) (res CodeSourceDTO) {
	defer recoverCodeSource(&res)

	text, errDTO, stale := a.editSource(document.Op{Kind: document.OpCode, Ref: ref},
		func(current *document.Document, raw []byte) (string, error) {
			return a.edit.CodeSource(a.renderer, raw, current.Kind, ref)
		})

	return CodeSourceDTO{Text: text, Stale: stale, Error: errDTO}
}

// editSource は編集欄に入れるソースを取り出す（GetCellSource / GetCodeSource。IMP-195）。
// **呼び出し元は ioMu を持たないこと**（ここで取る）。
//
// 書き込みの 1〜3 と同じ手順を踏み、get でソースを取り出す。戻り値は (ソース, 知らせる ErrorDTO, Stale)。
//
//	1, 2. 指示を作った描画がいまの描画でない（Check）              → Stale
//	3.    実体の解決・読み込みの失敗                                → Stale（edit-failed の文言は合わない）
//	4.    把握している内容と一致しない（ErrChanged）                → edit-conflict（読み直す。resolveConflict）
//	      前置きが行ごとに違うコードブロック（ErrLocked）           → code-locked
//	      それ以外の失敗（古い指示・編集できないもの）              → Stale
func (a *App) editSource(op document.Op, get func(*document.Document, []byte) (string, error)) (string, *ErrorDTO, bool) {
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	// 1, 2. 指示を作った描画がいまの描画か（IMP-109 の Check）。
	current, showing := a.shownDocument()
	if a.edit.Check(current, showing, op) != nil {
		return "", nil, true
	}

	// 3. 実体を読む。
	_, raw, err := readForEdit(current.Path)
	if err != nil && !errors.Is(err, document.ErrChanged) {
		return "", nil, true
	}

	// 4. 把握している内容と一致するかを確かめてから、ソースを取り出す。
	text := ""
	if err == nil {
		text, err = get(current, raw)
	}
	switch {
	case errors.Is(err, document.ErrChanged):
		return "", a.resolveConflict(current), false
	case errors.Is(err, document.ErrLocked):
		return "", newEditErrorDTO(current.Path, err), false
	case err != nil:
		return "", nil, true
	}

	return text, nil, false
}

// writeEdit は書き込みの処理（IMP-195 の表の 1〜8）を行う。**呼び出し元は ioMu を持たないこと。**
//
// **順序を変えない。** 番号は IMP-109 の判断の番号と揃えている。
func (a *App) writeEdit(op document.Op) EditResultDTO {
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	// 1, 2. **省かない。** フロントエンドは見た目を先に変えてから呼ぶ（IMP-261）ため、呼び出しは
	// 描画より遅れて届きうる。その間にドロップなどで文書が切り替わっていると、current は既に
	// 別の文書を指している——鍵の照合がそれを止める（FR-143）。Stale は通知しない。
	current, showing := a.shownDocument()
	if a.edit.Check(current, showing, op) != nil {
		return EditResultDTO{Stale: true}
	}

	// 3. 実体のパスを求めて読む。解決・読み込みの失敗は edit-failed。大きすぎるなら ErrChanged。
	real, raw, err := readForEdit(current.Path)
	if err != nil && !errors.Is(err, document.ErrChanged) {
		return EditResultDTO{Error: newEditErrorDTO(current.Path, err)}
	}

	// 4, 5. 把握している内容と一致するかを確かめ、書き換えを作る（IMP-109 の Plan）。
	// 構文解析は mu の外で行う（ioMu の内側ではある）。
	// 種類は表示中の文書のもの（コードブロックと図の書き換えが Markdown と図のファイルで分かれる）。
	var patch document.Patch
	changed, widened := false, false
	if err == nil {
		patch, changed, widened, err = a.edit.Plan(a.renderer, raw, current.Kind, op)
	}
	switch {
	case errors.Is(err, document.ErrChanged):
		return EditResultDTO{Error: a.resolveConflict(current)}
	case errors.Is(err, document.ErrStale):
		return EditResultDTO{Stale: true}
	case err != nil:
		// 確かめで拒んだ（ErrRejected / ErrLocked / ErrCannotFormat）・不正なバイト列や位置の
		// 食い違い（ErrNotEditable）。**黙って戻すと確定した入力が通知も無く消える**
		// （FR-142, FR-110）。種別は newEditErrorDTO が分ける（IMP-315）。
		return EditResultDTO{Error: newEditErrorDTO(current.Path, err)}
	case !changed:
		// 内容が変わらない・取り消すものが無い。何も書かない（FR-142, FR-144）。
		return EditResultDTO{}
	}

	// 6. 新しい内容を作り、**3 で解決した実体のパスへ**書く。Replace の中で改めて解決すると、
	// その間にリンク先が変わった場合に、読んだファイルと書くファイルが食い違う。
	after, err := patch.Apply(raw)
	if errors.Is(err, document.ErrChanged) {
		return EditResultDTO{Error: a.resolveConflict(current)}
	}
	if err == nil {
		err = document.Replace(real, after)
	}
	if err != nil {
		// **Commit を呼ばない**（履歴を動かさない。IMP-108）。
		return EditResultDTO{Error: newEditErrorDTO(current.Path, err)}
	}

	// 7. 履歴を確定する。手書きの取り消し線を全体で外したら、成功のうえで知らせる（FR-146。警告）。
	a.edit.Commit(op, patch, after)
	res := EditResultDTO{Changed: true}
	if widened {
		res.Notice = errKindFormatWidened
	}

	// 8. **監視のイベントを待たずに読み直して送る**（FR-014, AR-061）。内容が書き込んだものと
	// 一致すれば鍵を引き継ぐ（IMP-102）——引き継がないと、画面の目印が古くなって続けてクリック
	// した指示が Stale で黙って戻る（FR-143）。同意は開く処理が引き継ぐ（FR-016）。
	// 150 ms 後に届く監視のイベントは、内容が同じなので送らない（IMP-192）。
	a.reloadAfterEdit(current, openRequest{
		path:         current.Path,
		src:          openFromReload,
		trigger:      triggerEdit,
		refKey:       current.RefKey,
		expectDigest: sha256.Sum256(after),
	})

	return res
}

// resolveConflict は、書き込む前にファイルが変更されていたときの処理（IMP-195 の 4）を行い、
// edit-conflict の ErrorDTO を返す。**呼び出し元は ioMu を持っていること。**
//
// 書き込まず、**取り消し履歴を捨ててから**読み直して document:changed を送る。履歴は読み直しに
// 成功すれば edit.Loaded が作り直す（FR-143, FR-144）。**把握している内容は捨てない**
// （Discard）——読み直しに失敗して描画が古いまま残ったとき、次の書き込みが要約の確かめを
// 通ってしまう。
func (a *App) resolveConflict(current *document.Document) *ErrorDTO {
	a.edit.Discard()
	a.reloadAfterEdit(current, openRequest{path: current.Path, src: openFromReload, trigger: triggerReload})

	return newEditErrorDTO(current.Path, document.ErrChanged)
}

// reloadAfterEdit は書き込みの前後の読み直し（IMP-195 の 4 と 8）を行い、結果をイベントで送る。
// **呼び出し元は ioMu を持っていること**（openLocked を呼ぶ）。
//
// **読み直しに失敗したら error イベントで ErrorDTO を送る**（IMP-320）。状態画面になる失敗
// （しきい値をまたいだ・50 MB を超えた・変換に失敗した）では、openLocked が edit.Left() と
// target の書き換えと監視の解除を済ませており（IMP-192）、**送らないと Go 側だけが状態画面へ
// 移り、画面は編集モードのまま残る**（以後の指示はすべて Stale で黙って戻る）。
func (a *App) reloadAfterEdit(current *document.Document, req openRequest) {
	dto, err := a.openLocked(req)
	switch {
	case err != nil:
		a.emit(eventError, newErrorDTO(current.Path, err))
	case dto != nil:
		a.emit(eventDocumentChanged, dto)
	}
}

// shownDocument は表示中の文書と、画面がそれを表示しているかを控える（IMP-195 の 1）。
func (a *App) shownDocument() (*document.Document, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.current, a.showing
}

// readForEdit は書き込みの前に表示中ファイルの実体を読む（IMP-195 の 3）。
//
// **大きさが document.MaxSize を超えていれば、読まずに ErrChanged を返す。** 把握している内容は
// MaxSize 以下で読み込んだものであり、一致しえない。解決・大きさの取得・読み込みの失敗は、
// そのままのエラーを返す（呼び出し元が edit-failed、または GetCellSource なら Stale にする）。
func readForEdit(path string) (real string, raw []byte, err error) {
	real, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, err
	}

	info, err := os.Stat(real)
	if err != nil {
		return "", nil, err
	}
	if info.Size() > document.MaxSize {
		return real, nil, document.ErrChanged
	}

	raw, err = os.ReadFile(real)
	if err != nil {
		return "", nil, err
	}

	return real, raw, nil
}
