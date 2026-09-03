package main

import (
	"fmt"
	"path/filepath"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/kznagamori/go_MarkView/internal/applog"
	"github.com/kznagamori/go_MarkView/internal/opener"
)

// 本ファイルは「エディタで開く」の 3 つのバインドメソッドを持つ
// （IMP-310, IMP-331, FR-090, FR-091）。
//
// bind.go から分けているのは、bind.go が既に 400 行を超えており、責務で
// 分けるべき単位だからである（IMP-011）。link.go と同じ扱いとする。
//
// **フロントエンドが扱うのは識別子だけである。** 実行ファイルのパスは Go 側で
// 生まれて Go 側で消費され、DTO には載らない（IMP-309, IMP-300 の 3,
// NFR-035 の 3）。**この境界を越える引数や戻り値を足さない。**

// ListEditors は選択ウィンドウの一覧を返す（IMP-310, IMP-331, FR-091）。
//
// **画面の対象があるかを見ない**（IMP-331）。一覧を作るだけであり、対象の
// 有無はボタンの活性で表す（UI-021）。
//
// **起動時に先読みしない。** プリセットの検出はファイルシステムを触るため、
// 押されるまで行わない（NFR-013）。実行中にエディタがインストール・アン
// インストールされることもある。
func (a *App) ListEditors() (res EditorListDTO) {
	defer recoverEditorList(&res)

	// 検出はロックの外で行う（IMP-024）。ファイルシステムを触るため、
	// その間ほかのバインドメソッドを止める理由がない。
	editors := opener.Editors()

	a.mu.Lock()
	// **確定前の候補を捨てる。** 押すたびに選択ウィンドウを出す設計であり
	// （UI-103）、初期選択は設定に保存されたエディタだけから決まる。前回
	// Browse したまま閉じた候補が残っていると、利用者には「閉じた場合は
	// 何も保存しない」（FR-091）が破れたように見える。
	a.pendingEditor = ""
	saved := a.cfg.Editor
	a.mu.Unlock()

	return newEditorList(editors, saved, "")
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
// **開くのは App.target（画面がいま対象にしているファイル）である**
// （IMP-190）。表示中の文書（current）ではない。状態画面を出している間
// current は前に開いていた文書のまま残っており、それを渡すと利用者が見て
// いるのと違うファイルが静かに開く（NFR-035 の 2）。
func (a *App) OpenInEditor(id string) (res EditorResultDTO) {
	defer recoverEditorResult(&res)

	a.mu.Lock()
	target, pending, saved := a.target, a.pendingEditor, a.cfg.Editor
	a.mu.Unlock()

	// 文書未表示ではボタンが淡色であり（UI-021）通常は起こらないが、
	// 防御的に扱う（IMP-310）。
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
	a.mu.Lock()
	a.cfg.Editor = path
	// 絶対パス以外を保持しない（IMP-153）。ここへ来る値は絶対パスだが、
	// 設定へ入れる経路はすべて同じ規則を通す。
	a.cfg.Normalize()
	a.scheduleSave()
	a.mu.Unlock()

	return newEditorResult(name, nil)
}

// editorList は現在の状態から一覧を組み立てる（IMP-309, UI-103）。
//
// BrowseEditor が使う。**確定前の候補を捨てない**点が ListEditors と異なる。
func (a *App) editorList() EditorListDTO {
	editors := opener.Editors()

	a.mu.Lock()
	saved, pending := a.cfg.Editor, a.pendingEditor
	a.mu.Unlock()

	return newEditorList(editors, saved, pending)
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
