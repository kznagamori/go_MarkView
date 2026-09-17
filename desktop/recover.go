package desktop

import (
	"fmt"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/applog"
)

// 本ファイルは、バインドメソッドの入口で回復したパニックの扱いを持つ（IMP-022, IMP-310, FR-111）。
//
// **種別は IMP-310 に従う。** 文書を開くものは render-error の状態画面、エディタの 3 つは
// editor-failed（editor.go）、書き込みの 4 つは edit-failed（recoverEdit）、SetEditMode と
// GetCellSource は通知しない（recoverEditMode / recoverCellSource）。**すべてを状態画面にしない。**

// recoverBind はパニックをエラーへ変える（IMP-022）。
//
// 戻り値に Go の error を持つメソッド（ReadDir / CopyToClipboard / ReadClipboard）で使う。
func recoverBind(err *error) {
	r := recover()
	if r == nil {
		return
	}

	// 開発モードでのみ、発生箇所とスタックを標準エラーへ出す
	// （IMP-023, NFR-041）。判定は applog が持つ。
	applog.Recovered("app.bind", r)

	*err = fmt.Errorf("%w: %v", errPanic, r)
}

// recoverOpen はパニックを OpenResultDTO の Error へ変える（IMP-022, IMP-308）。
//
// path は画面の対象にするパスを指す。**呼び出し元は対象が分かった時点でこの変数へ入れる**
// （defer の時点では分からない経路がある）。空なら対象は分からない。
func (a *App) recoverOpen(path *string, res *OpenResultDTO) {
	r := recover()
	if r == nil {
		return
	}

	applog.Recovered("app.open", r)

	*res = OpenResultDTO{Error: a.openPanicked(*path, *path, r)}
}

// recoverLink はパニックを LinkResultDTO の Error へ変える（IMP-022, IMP-305）。
//
// リンクの生値（href）からは開こうとしていたファイルが決まらないため、画面の対象は空とする。
func (a *App) recoverLink(href string, res *LinkResultDTO) {
	r := recover()
	if r == nil {
		return
	}

	applog.Recovered("app.link", r)

	*res = LinkResultDTO{Kind: linkError, Error: a.openPanicked(href, "", r)}
}

// openPanicked は、文書を開くバインドで回復したパニックを render-error の状態画面として
// Go 側の状態へ反映し、ErrorDTO を返す（IMP-022, IMP-310, IMP-192）。
//
// errPath は ErrorDTO.Path に載せる値、target は画面の対象（空なら分からない）。
//
// **フロントエンドは render-error の状態画面を出す**（IMP-250）。Go 側を「前の文書を表示中」の
// まま残すと、監視のイベントや F5 で状態画面が前の文書の表示に置き換わる（BUG-012 と同じ形の
// 食い違い）。そこで、開く処理の「状態画面を出した」と同じ反映（commit の outcomeState）を行う:
// showing を偽にし、監視を外し、編集モードを終える。**対象が分からなければ target を空にする**
// ——前の文書のまま残すと、F5 と「エディタで開く」が画面に無い前の文書を指す（IMP-190）。
//
// **錠は、パニックの時点で解けている。** ioMu と mu を取る箇所は、パニックが途中で起きても
// defer で解くように書く（open・commit・watch.go、bind.go の読み出し）。
func (a *App) openPanicked(errPath, target string, r any) *ErrorDTO {
	err := fmt.Errorf("%w: %v", errPanic, r)
	if target != "" {
		target = absPath(target)
	}

	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	a.edit.Left()
	c := a.commit(openRequest{path: target}, nil, err, outcomeState, target, documentView{})
	switchWatch(c.watcher, outcomeState, target)

	name := ""
	if target != "" {
		name = filepath.Base(target)
	}
	a.setWindowTitle(name)

	return newErrorDTO(errPath, c.err)
}

// recoverEdit は、書き込みの 4 つ（SetTask / SetCell / UndoEdit / RedoEdit）で回復したパニックを
// edit-failed として返す（IMP-022, IMP-310）。
//
// 結果はステータス領域に出る。render-error の「Failed to render this document.」は状況と合わない。
// **Go 側の状態は変えない**——書き込みは状態画面を出さない操作であり、画面は文書のまま残る。
func (a *App) recoverEdit(res *EditResultDTO) {
	r := recover()
	if r == nil {
		return
	}

	applog.Recovered("app.edit", r)

	current, _ := a.shownDocument()
	path := ""
	if current != nil {
		path = current.Path
	}
	*res = EditResultDTO{Error: newEditErrorDTO(path, fmt.Errorf("%w: %v", errPanic, r))}
}

// recoverEditMode は、SetEditMode で回復したパニックを通知せず、その時点の状態を返す
// （IMP-022, IMP-310）。
//
// edit-failed の文言（「Failed to save: <path>」）は保存していない操作に合わない。**その時点の
// edit.On() を返し、ボタンの表示を Go 側と揃える。** edit は ioMu が守るため、錠を取って読む
// （パニックの時点で錠は defer で解けている）。
func (a *App) recoverEditMode(res *EditModeDTO) {
	r := recover()
	if r == nil {
		return
	}

	applog.Recovered("app.editmode", r)

	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	*res = EditModeDTO{On: a.edit.On(), Seq: a.edit.Seq()}
}

// recoverCellSource は、GetCellSource で回復したパニックを通知せず、Stale として返す
// （IMP-022, IMP-310）。編集欄が開かないだけで済む。
func recoverCellSource(res *CellSourceDTO) {
	if r := recover(); r != nil {
		applog.Recovered("app.cellsource", r)
		*res = CellSourceDTO{Stale: true}
	}
}

// recoverQuiet はパニックを握りつぶす（IMP-022）。
//
// エラーを返せないメソッドで使う。設定の更新やスクロール位置の記録が
// 失敗しても、利用者の操作を妨げる理由はない。
func recoverQuiet() {
	if r := recover(); r != nil {
		applog.Recovered("app.quiet", r)
	}
}
