package desktop

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/document"
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
func (a *App) SetEditMode(on bool) (res EditModeDTO) {
	defer a.recoverEditMode(&res)

	// 取らないと、文書を開く処理が edit を決めてから document:changed を送るまでの間に
	// 割り込み、状態が食い違う（IMP-195）。
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	if on {
		current, showing := a.shownDocument()
		a.edit.Start(current, showing)
	} else {
		a.edit.Stop()
	}

	return EditModeDTO{On: a.edit.On(), Seq: a.edit.Seq()}
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

	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	// 1, 2. 指示を作った描画がいまの描画か（IMP-109 の Check）。
	current, showing := a.shownDocument()
	if a.edit.Check(current, showing, document.Op{Kind: document.OpCell, Ref: ref}) != nil {
		return CellSourceDTO{Stale: true}
	}

	// 3. 実体を読む。
	_, raw, err := readForEdit(current.Path)
	if err != nil && !errors.Is(err, document.ErrChanged) {
		return CellSourceDTO{Stale: true}
	}

	// 4. 把握している内容と一致するかを確かめてから、セルのソースを取り出す。
	text := ""
	if err == nil {
		text, err = a.edit.CellSource(a.renderer, raw, ref)
	}
	switch {
	case errors.Is(err, document.ErrChanged):
		return CellSourceDTO{Error: a.resolveConflict(current)}
	case err != nil:
		return CellSourceDTO{Stale: true}
	}

	return CellSourceDTO{Text: text}
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
	var patch document.Patch
	changed := false
	if err == nil {
		patch, changed, err = a.edit.Plan(a.renderer, raw, op)
	}
	switch {
	case errors.Is(err, document.ErrChanged):
		return EditResultDTO{Error: a.resolveConflict(current)}
	case errors.Is(err, document.ErrStale):
		return EditResultDTO{Stale: true}
	case err != nil:
		// 表の形の確かめで拒んだ（ErrNotEditable）。**黙って戻すと確定した入力が通知も無く
		// 消える**（FR-142, FR-110）。
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

	// 7. 履歴を確定する。
	a.edit.Commit(op, patch, after)

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

	return EditResultDTO{Changed: true}
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
