package main

import (
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/buildinfo"
	"github.com/kznagamori/go_MarkView/internal/config"
	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/filetree"
	"github.com/kznagamori/go_MarkView/internal/opener"
	"github.com/kznagamori/go_MarkView/internal/renderer"
	"github.com/kznagamori/go_MarkView/internal/session"
)

// 本ファイルは Go とフロントエンドの間でやり取りする型を定める（IMP-302〜307）。
//
// **ここが両者の唯一の接点であり、境界の規約は厳密に守る**（AR-061）。
// json タグで JavaScript 側のフィールド名を明示し、null を返しうるフィールドは
// ポインタ型とする（IMP-301）。
//
// app.go から分けているのは、app.go が 13 種のバインドメソッドを持つ予定で
// あり、型定義と合わせると 400 行を超えるためである（IMP-011）。

// アプリケーション情報の固定値（FR-100, UI-100）。
//
// 利用者に見える文言であるため英語とする（UI-024）。ラベル（Author など）は
// フロントエンドの strings.js が持ち、ここが持つのは値だけである（IMP-290）。
const (
	appAuthor     = "kznagamori"
	appRepository = "https://github.com/kznagamori/go_MarkView"
	appLicense    = "MIT License"
)

// documentEncoding は読み込んだ文書の文字コード（IMP-302）。
//
// 入力は常に UTF-8 として扱い、不正なバイト列は置換して読み込みを続ける
// （FR-021, IMP-103）。したがってこの値は常に "UTF-8" である。
// AppTitle は文書未表示時のウィンドウタイトル（UI-013）。
const AppTitle = "MarkView"

const documentEncoding = "UTF-8"

// ScrollDTO.Mode の値（IMP-302）。
//
// スクロールの扱いは「どの経路で開いたか」に依存するため Go 側が決める
// （IMP-192）。フロントエンドに経路を意識させない。
const (
	scrollTop     = "top"     // 文書の先頭へ
	scrollAnchor  = "anchor"  // Anchor の見出しがペイン上端付近に来る位置へ
	scrollRestore = "restore" // Top の値へ復元する（履歴移動。FR-051）
	scrollKeep    = "keep"    // フロントエンドが現在位置を保つ。Top は使わない
)

// InitialStateDTO.StateKind の値（IMP-303）。
//
// 空文字は「文書を表示している」ことを表す。
const (
	stateNone         = ""
	stateWelcome      = "welcome"
	stateConfirmLarge = "confirm-large"
	stateTooLarge     = "too-large"
	stateRenderError  = "render-error"
)

// LinkResultDTO.Kind の値（IMP-305）。
const (
	linkDocument = "document"
	linkExternal = "external"
	linkAnchor   = "anchor"
	linkError    = "error"
)

// ErrorDTO.Kind の値（IMP-315）。
//
// フロントエンドはこの値で strings.js の文言を選ぶ。**値とキーは 1 対 1 で
// 対応させる**（IMP-290）。
const (
	errKindNotFound     = "not-found"
	errKindPermission   = "permission"
	errKindNotMarkdown  = "not-markdown"
	errKindNeedsConfirm = "needs-confirm"
	errKindTooLarge     = "too-large"
	errKindRenderError  = "render-error"
	errKindLinkNotFound = "link-not-found"
	errKindClipboard    = "clipboard"
	errKindRemoved      = "removed"
	errKindEditorFailed = "editor-failed"
	errKindEditorSelf   = "editor-self"
	errKindEncoding     = "encoding"
)

// editorCustom は「任意指定」を表す予約語（IMP-309, UI-103）。
//
// エディタ選択ウィンドウの末尾に置く `Other...` の行の ID である。
// **プリセットの ID にこの値を使わない**（IMP-172。UT-705 が確かめている）。
const editorCustom = "custom"

// ScrollDTO は描画後のスクロール指示（IMP-302）。
type ScrollDTO struct {
	Mode   string `json:"mode"`   // scrollTop | scrollAnchor | scrollRestore | scrollKeep
	Anchor string `json:"anchor"` // Mode == scrollAnchor のときの見出し ID
	Top    int    `json:"top"`    // Mode == scrollRestore のときの位置
}

// DocumentDTO は 1 つの文書の表示に必要な情報をまとめる（IMP-302）。
//
// **1 つの利用者操作に対する呼び出しは 1 回とし、必要な情報をまとめて返す**
// （IMP-300, AR-061）。
type DocumentDTO struct {
	Path        string `json:"path"`        // 絶対パス（IMP-025）
	DisplayPath string `json:"displayPath"` // ステータス表示用（UI-060）
	Name        string `json:"name"`        // ベース名（UI-013 のタイトル）
	OutsideTree bool   `json:"outsideTree"` // ツリー外の文書か（FR-052）

	HTML      string             `json:"html"`      // サニタイズ済み（IMP-116）
	Headings  []renderer.Heading `json:"headings"`  // アウトライン（FR-040）
	LineCount int                `json:"lineCount"` // 総行数（UI-060）
	Encoding  string             `json:"encoding"`  // 常に documentEncoding

	NeedsMermaid bool `json:"needsMermaid"` // Mermaid の遅延ロード判定（AR-021）
	NeedsKaTeX   bool `json:"needsKaTeX"`   // KaTeX の遅延ロード判定（AR-021）

	Scroll ScrollDTO `json:"scroll"` // 描画後のスクロール指示

	// Warnings は描画を継続する事象の Kind（IMP-315）。文言そのものではない。
	//
	// フロントエンドは Kind から strings.js の文言を選ぶ。文言の定義を
	// 1 箇所に集約するためであり、ErrorDTO.Kind と同じ扱いである（IMP-290）。
	Warnings []string `json:"warnings"`
}

// ConfigDTO はフロントエンドへ渡す設定（IMP-303）。
//
// **表示倍率を含めない。** 倍率は保存しない（UI-111, UI-115）ため、往路では
// 渡すものがなく、復路でも Go 側に受け取る先がない（IMP-150）。倍率は
// フロントエンドの state だけが持つ（IMP-210, IMP-242）。
//
// **ウィンドウの大きさも含めない。** 保存の直前に Wails のランタイムから
// 直接読み出す（IMP-194）。最大化状態はそもそも保存しない（UI-111）。
type ConfigDTO struct {
	Theme           string `json:"theme"` // "light" | "dark"（解決済み。FR-071）
	OutlineVisible  bool   `json:"outlineVisible"`
	FileTreeVisible bool   `json:"fileTreeVisible"`
	OutlineWidth    int    `json:"outlineWidth"`
	FileTreeWidth   int    `json:"fileTreeWidth"`
}

// InitialStateDTO は起動直後にフロントエンドが必要とするすべてを返す
// （IMP-303, FR-012, FR-013, UI-110）。
type InitialStateDTO struct {
	Config   ConfigDTO    `json:"config"`
	TreeRoot string       `json:"treeRoot"` // 絶対パス。未確定なら空文字
	Document *DocumentDTO `json:"document"` // 表示対象がなければ null

	// StateKind は本文ペインに出す状態画面の種別（IMP-250）。
	//
	// welcome 以外になるのは、起動時の引数に大きすぎるファイルや壊れた
	// ファイルが指定された場合である（FR-012）。起動経路のためだけの専用画面を
	// 作らず、通常の状態画面と同じ処理で描画させる。
	StateKind string    `json:"stateKind"`
	Error     *ErrorDTO `json:"error"` // 状態画面に数値などを要する場合
}

// TreeNodeDTO はファイルツリーの 1 要素（IMP-304）。
//
// 子ノードは含めない。展開のたびに ReadDir を呼ぶ（FR-032 の遅延展開）。
type TreeNodeDTO struct {
	Name  string `json:"name"`
	Path  string `json:"path"` // 絶対パス（IMP-025）
	IsDir bool   `json:"isDir"`

	// HasChild は展開可能かを示す（FR-032 の先読み結果）。
	HasChild bool `json:"hasChild"`

	// Omitted は **その要素が属する一覧から件数上限で除かれた数**
	// （IMP-130, FR-032）。切り詰めが起きた場合、返すすべての要素に同じ値が入る。
	// フロントエンドは先頭の要素を見て、末尾に `… and N more` を出す（DSP-112）。
	Omitted int `json:"omitted"`
}

// OpenResultDTO は文書を開く操作の結果（IMP-308）。
//
// **失敗を Go の error ではなくこの構造体で返す。** Wails v2 は error を
// メッセージ文字列としてしかフロントエンドへ渡せず（NewErrorCallback）、
// ErrorDTO の Kind・Size・Limit が失われる。それでは大きなファイルの確認画面
// （FR-016, IMP-314）を出せない。
//
// Document と Error がどちらも nil の場合は「何も起きなかった」を表す。
// ダイアログの取り消し、履歴の端、表示中の文書がない状態での再読み込みが
// これにあたる。フロントエンドは表示を変えない。
type OpenResultDTO struct {
	Document *DocumentDTO `json:"document"` // 成功したとき
	Error    *ErrorDTO    `json:"error"`    // 失敗したとき
}

// newOpenResult は open の戻り値を OpenResultDTO へ写す（IMP-308, IMP-315）。
//
// path は ErrorDTO に載せる対象。エラー値からは取り出せないため呼び出し側が渡す。
func newOpenResult(path string, dto *DocumentDTO, err error) OpenResultDTO {
	if err != nil {
		return OpenResultDTO{Error: newErrorDTO(path, err)}
	}

	return OpenResultDTO{Document: dto}
}

// LinkResultDTO は本文中のリンクを踏んだ結果（IMP-305）。
type LinkResultDTO struct {
	Kind     string       `json:"kind"`     // linkDocument | linkExternal | linkAnchor | linkError
	Document *DocumentDTO `json:"document"` // Kind == linkDocument のとき
	Anchor   string       `json:"anchor"`   // Kind == linkAnchor のとき
	Error    *ErrorDTO    `json:"error"`    // Kind == linkError のとき
}

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

// AboutDTO はアプリケーション情報ウィンドウの内容（IMP-306, FR-100, FR-101）。
type AboutDTO struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`

	Author      string `json:"author"`
	Repository  string `json:"repository"`
	License     string `json:"license"`
	Environment string `json:"environment"`

	Vendors  []buildinfo.VendorEntry `json:"vendors"`  // Bundled 行（UI-100）
	Licenses string                  `json:"licenses"` // THIRD_PARTY.md の全文（FR-101）
}

// ErrorDTO は異常をフロントエンドへ伝える（IMP-307, IMP-315）。
//
// **文言の組み立てはフロントエンドで行う。** Go 側は Kind と要素（パス・
// サイズ）を渡し、フロントエンドが strings.js の文言を選ぶ（IMP-290）。
// Message には Go 側が組み立てた英語も入れる。未知の Kind を受け取った場合の
// フォールバックとして用いる。
type ErrorDTO struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Path    string `json:"path"`  // 対象がある場合
	Size    int64  `json:"size"`  // サイズ関連のときのみ
	Limit   int64  `json:"limit"` // サイズ関連のときのみ
}

// newDocumentDTO は Document を DTO へ写す（IMP-302）。
//
// 表示用パスとスクロール指示は開いた経路によって決まるため、呼び出し側から
// 受け取る（IMP-192）。
func newDocumentDTO(doc *document.Document, display string, outside bool, scroll ScrollDTO) *DocumentDTO {
	return &DocumentDTO{
		Path:        doc.Path,
		DisplayPath: display,
		Name:        filepath.Base(doc.Path),
		OutsideTree: outside,

		HTML:      doc.HTML,
		Headings:  headingsOrEmpty(doc.Headings),
		LineCount: doc.LineCount,
		Encoding:  documentEncoding,

		NeedsMermaid: doc.NeedsMermaid,
		NeedsKaTeX:   doc.NeedsKaTeX,

		Scroll:   scroll,
		Warnings: warningKinds(doc.Warnings),
	}
}

// headingsOrEmpty は nil を空スライスへ均す。
//
// JSON の null をフロントエンドへ渡さないためである。null が来ると
// outline.js の走査が落ち、アウトラインだけでなく描画全体が止まる。
func headingsOrEmpty(headings []renderer.Heading) []renderer.Heading {
	if headings == nil {
		return []renderer.Heading{}
	}
	return headings
}

// warningKinds は警告を Kind の並びへ写す（IMP-315）。
//
// 戻り値は常に非 nil とする（headingsOrEmpty と同じ理由）。未知の種別は
// 落とす。フロントエンドが解釈できない値を渡しても表示できないためである。
func warningKinds(warnings []document.Warning) []string {
	kinds := make([]string, 0, len(warnings))

	for _, w := range warnings {
		if w.Kind == document.WarnInvalidEncoding {
			kinds = append(kinds, errKindEncoding)
		}
	}

	return kinds
}

// newTreeNodeDTOs は filetree の結果を DTO の並びへ写す（IMP-304）。
//
// HasChild はディレクトリかどうかと一致する。ReadDir が Markdown を含まない
// ディレクトリを既に除いており（FR-031, IMP-133）、残ったディレクトリは
// すべて展開する価値があるためである。
func newTreeNodeDTOs(nodes []filetree.Node) []TreeNodeDTO {
	dtos := make([]TreeNodeDTO, 0, len(nodes))

	for _, n := range nodes {
		dtos = append(dtos, TreeNodeDTO{
			Name:     n.Name,
			Path:     n.Path,
			IsDir:    n.IsDir,
			HasChild: n.IsDir,
			Omitted:  n.Omitted,
		})
	}

	return dtos
}

// newConfigDTO は設定をフロントエンドへ渡す形へ写す（IMP-303）。
//
// theme は OS 設定への追従（FR-071）まで解決した値を渡す。フロントエンドで
// prefers-color-scheme を判定して上書きさせないためである。
func newConfigDTO(cfg config.Config, theme string) ConfigDTO {
	return ConfigDTO{
		Theme:           theme,
		OutlineVisible:  cfg.OutlineVisible,
		FileTreeVisible: cfg.FileTreeVisible,
		OutlineWidth:    cfg.OutlineWidth,
		FileTreeWidth:   cfg.FileTreeWidth,
	}
}

// newAboutDTO はアプリケーション情報を組み立てる（IMP-306）。
//
// licenses は go:embed した THIRD_PARTY.md の全文、webviewVersion は Wails
// から得た WebView のバージョンである。どちらも取得できない場合は空文字でよい。
func newAboutDTO(licenses, webviewVersion string) AboutDTO {
	return AboutDTO{
		Version:   buildinfo.Version,
		Commit:    buildinfo.Commit,
		BuildTime: buildinfo.BuildTime,

		Author:      appAuthor,
		Repository:  appRepository,
		License:     appLicense,
		Environment: buildinfo.Environment(webviewVersion),

		Vendors:  buildinfo.Vendors(),
		Licenses: licenses,
	}
}
