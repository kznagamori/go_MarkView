package desktop

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/kznagamori/go_MarkView/internal/applog"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/opener"
)

// 本ファイルは「エディタで開く」のバインドメソッド（ListEditors / BrowseEditor / OpenInEditor /
// CloseEditor）を持つ（IMP-310, IMP-331, FR-090, FR-091）。ツリーから開く ListEditorsForTree は
// tree.go（IMP-196）にあり、一覧の組み立て（listEditorsFor）をここと共有する。
//
// bind.go から分けているのは、bind.go が既に 400 行を超えており、責務で
// 分けるべき単位だからである（IMP-011）。link.go と同じ扱いとする。
//
// **フロントエンドが扱うのは識別子だけである。** 実行ファイルのパスは Go 側で
// 生まれて Go 側で消費され、DTO には載らない（IMP-309, IMP-300 の 3,
// NFR-035 の 3）。**この境界を越える引数や戻り値を足さない。**

// ListEditors は選択ウィンドウの一覧を返す（IMP-310, IMP-331, FR-091）。
//
// **そのときの画面の対象（target）を editorTarget に控える**（FR-091。4.75.0。利用者の判断）。
// ウィンドウを開いている間にドロップで文書が切り替わっても（UI-090）、ウィンドウに出ている名前の
// ファイルを開くためである。**対象が文字のファイルでなければ（画像ファイル・文書未表示）控えは空とし、
// 一覧を空で返す**（フロントエンドはウィンドウを出さない。IMP-252）。対象が無いことを失敗にしない
// ——対象の有無はボタンの活性で表す（UI-021, IMP-331）。
//
// **起動時に先読みしない。** プリセットの検出はファイルシステムを触るため、
// 押されるまで行わない（NFR-013）。実行中にエディタがインストール・アン
// インストールされることもある。
func (a *App) ListEditors() (res EditorListDTO) {
	defer recoverEditorList(&res)

	a.mu.Lock()
	target := a.target
	a.mu.Unlock()

	// 状態画面の間も target を使う（current ではない。IMP-190）。状態画面の対象が画像なら空にする。
	if !mdfile.IsText(target) {
		target = ""
	}

	return a.listEditorsFor(target)
}

// listEditorsFor は target を editorTarget に控え、確定前の候補を捨てて一覧を返す（IMP-310, IMP-196）。
// ListEditors と ListEditorsForTree が使う。**target が空なら控えも空にし、一覧を空で返す。**
//
// **確定前の候補を捨てる。** 押すたびに選択ウィンドウを出す設計であり（UI-103）、初期選択は設定に
// 保存されたエディタだけから決まる。前回 Browse したまま閉じた候補が残っていると、利用者には
// 「閉じた場合は何も保存しない」（FR-091）が破れたように見える。
func (a *App) listEditorsFor(target string) EditorListDTO {
	a.mu.Lock()
	a.pendingEditor = ""
	a.editorTarget = target
	saved := a.cfg.Editor
	a.mu.Unlock()

	if target == "" {
		return EditorListDTO{Editors: []EditorDTO{}}
	}

	// 検出はロックの外で行う（IMP-024）。ファイルシステムを触るため、
	// その間ほかのバインドメソッドを止める理由がない。
	list := newEditorList(opener.Editors(), saved, "")
	list.FileName = filepath.Base(target)
	return list
}

// CloseEditor は、エディタ選択ウィンドウを開かずに閉じたことを受け取る（IMP-310, FR-036, FR-091）。
//
// Cancel / Esc / 外のクリックで閉じたときに呼ばれる。**控え（editorTarget）と確定前の候補
// （pendingEditor）を捨てる。** 通知しない（IMP-310）。
func (a *App) CloseEditor() {
	defer recoverQuiet()

	a.mu.Lock()
	defer a.mu.Unlock()

	a.editorTarget = ""
	a.pendingEditor = ""
}

// BrowseEditor は実行ファイルを選ぶダイアログを開く（IMP-310, IMP-331）。
//
// 選ばれたパスは**確定前の候補として 1 つだけ保持する**。OpenInEditor("custom")
// はその候補に対してのみ有効とする。任意の実行ファイルを無条件に起動する経路を
// 作らないためであり、OpenConfirmed が確認待ちのパスを 1 つだけ保持するのと
// 同じ考え方である（IMP-314）。
//
// **ここでは設定へ保存しない**（UI-116）。保存するのは OpenInEditor が起動に
// 成功したときだけである。
func (a *App) BrowseEditor() (res EditorListDTO) {
	defer recoverEditorList(&res)

	path, err := runtime.OpenFileDialog(a.context(), runtime.OpenDialogOptions{
		// 文言は英語とする（UI-024）。
		Title:   "Choose an editor",
		Filters: editorFilters(),
	})

	// **ダイアログを出せなかった場合もキャンセルと同じ扱いとする。**
	// 候補は変えず、いまの一覧をそのまま返す（FR-110, OpenFileDialog と同じ）。
	//
	// 絶対パスでないものも候補にしない。ダイアログは絶対パスを返すが、
	// 確定前の候補も設定と同じ規則で扱う（IMP-153, NFR-035 の 5）。
	if err != nil || path == "" || !filepath.IsAbs(path) {
		return a.editorList()
	}

	a.mu.Lock()
	a.pendingEditor = path
	a.mu.Unlock()

	return a.editorList()
}

// OpenInEditor は選ばれたエディタで対象のファイルを開く（IMP-310, FR-090）。
//
// id はプリセットの ID、または editorCustom。**パスを引数に取らない**
// （IMP-300 の 3）。
//
// **開くのは、ウィンドウを開いた時点で控えた editorTarget である**（IMP-190, FR-091。4.75.0）。
// 表示中の文書（current）ではない——状態画面を出している間 current は前に開いていた文書のまま
// 残っており、それを渡すと利用者が見ているのと違うファイルが静かに開く（NFR-035 の 2）。v1.1.0 は
// 押した時点の target を開いており、ウィンドウを開いている間のドロップで文書が切り替わると、
// ウィンドウに出ている名前と違うファイルが開いた。**起動の成否に関わらず、終わったら控えを捨てる。**
func (a *App) OpenInEditor(id string) (res EditorResultDTO) {
	defer recoverEditorResult(&res)

	a.mu.Lock()
	target, pending, saved := a.editorTarget, a.pendingEditor, a.cfg.Editor
	a.mu.Unlock()
	defer a.dropEditorTarget(target)

	// ボタンが淡色であり（UI-021）、対象が文字のファイルでなければウィンドウも出ない（IMP-252）ため
	// 通常は起こらないが、防御的に扱う（IMP-310）。
	if target == "" {
		return newEditorResult("", errNoTarget)
	}

	path, name, err := resolveEditor(id, pending, saved)
	if err != nil {
		return newEditorResult("", err)
	}

	// 起動前の 5 つの検査は opener が行う（IMP-171）。
	if err := opener.OpenWith(path, target); err != nil {
		return newEditorResult("", err)
	}

	// **起動できたときに初めて保存する**（UI-116, IMP-310）。選んだだけ・
	// Browse しただけでは保存しない。起動に失敗したエディタを覚えると、
	// 次回もそれが初期選択になる。
	a.saveEditor(path)

	return newEditorResult(name, nil)
}

// saveEditor は起動できたエディタを設定へ入れ、保存を予約する（UI-116, IMP-310）。
//
// **錠は defer で解く。** OpenInEditor は defer で控えを捨てる（dropEditorTarget が mu を取る）ため、
// ここで落ちて mu が残ると、その defer が止まる。
func (a *App) saveEditor(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.Editor = path
	// 絶対パス以外を保持しない（IMP-153）。ここへ来る値は絶対パスだが、
	// 設定へ入れる経路はすべて同じ規則を通す。
	a.cfg.Normalize()
	a.scheduleSave()
}

// dropEditorTarget は、OpenInEditor が使った控えを捨てる（IMP-190）。
//
// **控えが、開くのに使った値（used）のままのときだけ捨てる。** 起動の間に一覧（ListEditors）が控えを
// 作り直していれば、それを残す。
func (a *App) dropEditorTarget(used string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.editorTarget == used {
		a.editorTarget = ""
	}
}

// editorList は現在の状態から一覧を組み立てる（IMP-309, UI-103）。
//
// BrowseEditor が使う。**確定前の候補を捨てない**点が ListEditors と異なる。**控えた対象のベース名
// （FileName）も載せる**——フロントエンドは戻り値で一覧全体を描き直すため、空にすると名前が消える
// （IMP-309, IMP-252。4.76.0）。
func (a *App) editorList() EditorListDTO {
	editors := opener.Editors()

	a.mu.Lock()
	saved, pending, target := a.cfg.Editor, a.pendingEditor, a.editorTarget
	a.mu.Unlock()

	list := newEditorList(editors, saved, pending)
	if target != "" {
		list.FileName = filepath.Base(target)
	}
	return list
}

// resolveEditor は ID を実行ファイルの絶対パスと表示名へ解決する
// （IMP-310, IMP-172）。
//
// **一覧に出せる状態のものだけを通す。** 見つからなかったプリセットや、
// 実体の無い custom は errUnknownEditor で拒む。フロントエンドから任意の
// 実行ファイルを起動する経路を作らないためである（IMP-300 の 3）。
//
// custom の出どころは 2 つあり、**どちらも Go 側が持つ値である。**
//
//  1. pending — BrowseEditor が選ばせた確定前の候補（IMP-310）
//  2. saved   — 設定に保存されたエディタ（UI-116。IMP-153 により絶対パス）
func resolveEditor(id, pending, saved string) (path, name string, err error) {
	if id == editorCustom {
		// **候補が無ければ保存された値を使う**（UI-116）。押すたびに選択
		// ウィンドウを出す設計であり（UI-103）、ListEditors は確定前の候補を
		// 捨てる（IMP-310）。捨てたあとに pending だけを見ると、**保存された
		// エディタは 2 回目以降けっして起動できない。**
		chosen := pending
		if chosen == "" {
			chosen = saved
		}

		// **一覧で選べる状態にしたものと同じ条件で通す。** newEditorList は
		// editorAvailable が真のときだけ custom の行を選べるようにしており
		// （IMP-309, UI-116）、ここも同じ検査にする。食い違うと「選べるのに
		// 起動できない」行ができる。
		if !editorAvailable(chosen) {
			return "", "", fmt.Errorf("%s: %w", id, errUnknownEditor)
		}

		// 表示名は実行ファイル名とする。パスは画面へ出さない（NFR-035 の 3）。
		return chosen, filepath.Base(chosen), nil
	}

	for _, e := range opener.Editors() {
		if e.ID != id || e.Path == "" {
			continue
		}

		return e.Path, e.Name, nil
	}

	return "", "", fmt.Errorf("%s: %w", id, errUnknownEditor)
}

// editorFilters は実行ファイル選択ダイアログのフィルタを返す（UI-103, UI-024）。
//
// **Linux では絞り込まない。** 実行ファイルに拡張子が無いためであり、
// `*.exe` に相当するパターンが存在しない。
func editorFilters() []runtime.FileFilter {
	if goruntime.GOOS == "windows" {
		return []runtime.FileFilter{
			{DisplayName: "Programs", Pattern: "*.exe"},
			{DisplayName: "All files", Pattern: "*.*"},
		}
	}

	return []runtime.FileFilter{{DisplayName: "All files", Pattern: "*"}}
}

// recoverEditorList はパニックを EditorListDTO の Error へ変える
// （IMP-022, IMP-310, FR-111）。
//
// **Editors を nil のままにしない。** JSON の null がフロントエンドへ渡ると、
// 一覧の走査が落ちて選択ウィンドウごと出なくなる（headingsOrEmpty と同じ理由）。
func recoverEditorList(res *EditorListDTO) {
	r := recover()
	if r == nil {
		return
	}

	applog.Recovered("app.editor", r)

	*res = EditorListDTO{
		Editors: []EditorDTO{},
		Error:   newEditorErrorDTO(fmt.Errorf("%w: %v", errPanic, r)),
	}
}

// recoverEditorResult はパニックを EditorResultDTO の Error へ変える
// （IMP-022, IMP-310, FR-111）。
//
// **render-error ではなく editor-failed とする**（IMP-315）。この結果は
// ステータス領域に出るものであり、「Failed to render this document.」は
// 状況と合わない。利用者は文書の変換に失敗したと受け取ってしまう。
func recoverEditorResult(res *EditorResultDTO) {
	r := recover()
	if r == nil {
		return
	}

	applog.Recovered("app.editor", r)

	*res = newEditorResult("", fmt.Errorf("%w: %v", errPanic, r))
}
