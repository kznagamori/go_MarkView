package main

import (
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/buildinfo"
	"github.com/kznagamori/go_MarkView/internal/config"
	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/filetree"
	"github.com/kznagamori/go_MarkView/internal/renderer"
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
	errKindEncoding     = "encoding"
)

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
// **ウィンドウの大きさと最大化状態は含めない。** これらは Wails の API で
// 適用するものであり、フロントエンドが読む必要がない（IMP-194）。
type ConfigDTO struct {
	Theme           string `json:"theme"` // "light" | "dark"（解決済み。FR-071）
	Zoom            int    `json:"zoom"`
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
		Zoom:            cfg.Zoom,
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
