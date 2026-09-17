package desktop

import (
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/opener"
	"github.com/kznagamori/go_MarkView/internal/session"
)

// 本ファイルは外部エディタで開く機能（FR-090, FR-091）の境界の型を定める（IMP-309）。
// dto.go が 400 行の目安（IMP-011）を超えたため分けた。**編集モード（FR-140〜FR-144）の
// DTO は dto_editmode.go にある**——外部エディタ（editor.go）と編集モード（editmode.go）を
// 混ぜない（IMP-011）。

// editorCustom は「任意指定」を表す予約語（IMP-309, UI-103）。
//
// エディタ選択ウィンドウの末尾に置く `Other...` の行の ID である。
// **プリセットの ID にこの値を使わない**（IMP-172。UT-705 が確かめている）。
const editorCustom = "custom"

// EditorListDTO はエディタ選択ウィンドウの中身（IMP-309, UI-103）。
type EditorListDTO struct {
	Editors []EditorDTO `json:"editors"`
	Error   *ErrorDTO   `json:"error"`
}

// EditorDTO は一覧の 1 行（IMP-309）。
//
// **実行ファイルのパスを載せてはならない**（NFR-035 の 3）。画面に出す必要が
// なく、載せた時点でフロントエンドをパスが通ることになる。これは IMP-300 の 3
// が禁じている形そのものである。**フィールドを足すときは必ずここを読む。**
//
// Name はプリセットの表示名か、`custom` の場合は選ばれた**実行ファイル名**
// （`filepath.Base`。パスではない）。`Other...` というラベル自体はフロント
// エンドが持つ（IMP-290）。
type EditorDTO struct {
	ID        string `json:"id"`        // プリセットの ID、または editorCustom
	Name      string `json:"name"`      // 画面に出す表示名
	Available bool   `json:"available"` // 選択できるか（見つかったか）
	Selected  bool   `json:"selected"`  // 初期選択（UI-116）
}

// EditorResultDTO は起動の結果（IMP-309）。
type EditorResultDTO struct {
	Name  string    `json:"name"`  // 起動したエディタの表示名。ステータス表示に使う
	Error *ErrorDTO `json:"error"` // 失敗したとき。成功時は null
}

// newEditorList は選択ウィンドウの一覧を組み立てる（IMP-309, UI-103, UI-116）。
//
// saved は設定に保存されたエディタ（UI-116）、pending は BrowseEditor で
// 選ばれた確定前の候補（IMP-310）。どちらも絶対パスまたは空文字である。
//
// **並べ替えない。** 順序は IMP-172 の定義順に `custom` を足したものとし、
// 見つかったものを前へ出さない（UI-103）。見つからなかったものも
// `Available` が false の行として残す。消すと「なぜ自分のエディタが出ない
// のか」が分からない。
//
// **`Selected` が真の行は高々 1 つである。** 保存が無い場合と、保存された
// エディタが見つからない場合（アンインストール、更新によるパスの変更）は
// どの行も真にしない。UI-116 が「エラーとせず『初期選択が無い』状態として
// 扱う」と定めている。
func newEditorList(editors []opener.Editor, saved, pending string) EditorListDTO {
	// Browse で選ばれた候補があればそちらが初期選択になる。まだ選んで
	// いなければ、設定に保存されたエディタが初期選択となる（UI-103）。
	want := pending
	if want == "" {
		want = saved
	}

	list := make([]EditorDTO, 0, len(editors)+1)
	matched := false

	for _, e := range editors {
		// 見つからなかったものは選べない（UI-103）。Path が空であれば
		// samePath も一致しないが、意図を明示するために両方を見る。
		available := e.Path != ""
		selected := available && session.SamePath(e.Path, want)
		if selected {
			matched = true
		}

		list = append(list, EditorDTO{
			ID:        e.ID,
			Name:      e.Name,
			Available: available,
			Selected:  selected,
		})
	}

	// 末尾は常に custom の行とする（IMP-309, UI-103）。プリセットのどれとも
	// 一致しない指定はここに出る。
	custom := EditorDTO{ID: editorCustom}

	// **存在を確かめてから出す。** 保存されたエディタが消えている場合に
	// 選択済みとして出すと、押しても起動できない行が初期選択になる
	// （UI-116 の「初期選択が無い状態として扱う」）。
	if !matched && editorAvailable(want) {
		custom.Name = filepath.Base(want)
		custom.Available = true
		custom.Selected = true
	}

	return EditorListDTO{Editors: append(list, custom)}
}

// editorAvailable は指定された実行ファイルが今も存在するかを返す（UI-116）。
//
// 保存されたエディタがアンインストールされていることがある。**エラーには
// せず、「初期選択が無い」状態として扱う**（UI-116, UI-113）。
//
// **これは起動を許すかどうかの判断ではない。** 一覧の見え方を決めるだけで
// あり、起動の直前に opener.OpenWith が同じ検査をあらためて行う（IMP-171
// の 2）。食い違った場合も、起動時に弾かれて editor-failed になる。
func editorAvailable(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}

	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// newEditorResult は起動の結果を DTO へ写す（IMP-309, IMP-315）。
//
// name は起動したエディタの表示名。失敗したときは Error だけを載せる。
func newEditorResult(name string, err error) EditorResultDTO {
	if err != nil {
		return EditorResultDTO{Error: newEditorErrorDTO(err)}
	}

	return EditorResultDTO{Name: name}
}
