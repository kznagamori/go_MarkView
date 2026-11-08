package desktop

import (
	"github.com/kznagamori/go_MarkView/internal/config"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
)

// 本ファイルは、起動直後の状態と設定の境界の型を定める（IMP-303）。
// dto.go が 400 行の目安（IMP-011）を超えるため分けた。履歴の型（HistoryDTO）は dto_tree.go にある。

// ConfigDTO はフロントエンドへ渡す設定（IMP-303）。
//
// **表示倍率を含めない。** 倍率は保存しない（UI-111, UI-115）ため、往路では
// 渡すものがなく、復路でも Go 側に受け取る先がない（IMP-150）。倍率は
// フロントエンドの state だけが持つ（IMP-210, IMP-242）。
//
// **ウィンドウの大きさも含めない。** 保存の直前に Wails のランタイムから
// 直接読み出す（IMP-194）。最大化状態はそもそも保存しない（UI-111）。
//
// **ツリーのフィルター（TreeShowDiagrams / TreeShowImages）は往路も復路も同じ意味である**
// （FR-037。IMP-303）。Go 側は UpdateConfig で受け取った値をすぐに App.cfg に写し、以後の
// ReadDir とツリーの操作の検査がその値で絞る（IMP-134, IMP-196）。
type ConfigDTO struct {
	Theme           string `json:"theme"` // "light" | "dark"（解決済み。FR-071）
	OutlineVisible  bool   `json:"outlineVisible"`
	FileTreeVisible bool   `json:"fileTreeVisible"`
	OutlineWidth    int    `json:"outlineWidth"`
	FileTreeWidth   int    `json:"fileTreeWidth"`

	TreeShowDiagrams bool `json:"treeShowDiagrams"` // ツリーのフィルター（FR-037）
	TreeShowImages   bool `json:"treeShowImages"`
}

// ExtensionsDTO は開けるファイルの拡張子の並び（mdfile の定義。IMP-105, IMP-303）。
//
// **フィルターの文言の拡張子の並びを mdfile の定義から渡すためにある**（FR-037 の「2 か所に
// 書かない」）。文言の枠は strings.js に置き（IMP-290）、拡張子の並びは書かない。
type ExtensionsDTO struct {
	Markdown []string `json:"markdown"` // mdfile.Extensions
	Diagrams []string `json:"diagrams"` // PlantUMLExtensions の後に MermaidExtensions
	Images   []string `json:"images"`   // ImageTypes の Ext
}

// InitialStateDTO は起動直後にフロントエンドが必要とするすべてを返す
// （IMP-303, FR-012, FR-013, UI-110）。
type InitialStateDTO struct {
	Config     ConfigDTO     `json:"config"`
	Extensions ExtensionsDTO `json:"extensions"` // ツリーのフィルターの文言に出す拡張子（FR-037）
	History    HistoryDTO    `json:"history"`    // 戻れるか・進めるか（FR-051, IMP-323）
	TreeRoot   string        `json:"treeRoot"`   // 絶対パス。未確定なら空文字
	Document   *DocumentDTO  `json:"document"`   // 表示対象がなければ null

	// StateKind は本文ペインに出す状態画面の種別（IMP-250）。
	//
	// welcome 以外になるのは、起動時の引数に大きすぎるファイルや壊れた
	// ファイルが指定された場合である（FR-012）。起動経路のためだけの専用画面を
	// 作らず、通常の状態画面と同じ処理で描画させる。
	StateKind string    `json:"stateKind"`
	Error     *ErrorDTO `json:"error"` // 状態画面に数値などを要する場合
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

		TreeShowDiagrams: cfg.TreeShowDiagrams,
		TreeShowImages:   cfg.TreeShowImages,
	}
}

// newExtensionsDTO は mdfile の拡張子の定義を並べる（IMP-303, IMP-105）。
//
// **新しいスライスへ写す。** mdfile の一覧はパッケージの変数であり、append で後ろに足すと
// 容量しだいで元の配列を書き換えうる。並びは mdfile の定義のまま（FR-037 の文言の並び）。
func newExtensionsDTO() ExtensionsDTO {
	diagrams := make([]string, 0, len(mdfile.PlantUMLExtensions)+len(mdfile.MermaidExtensions))
	diagrams = append(diagrams, mdfile.PlantUMLExtensions...)
	diagrams = append(diagrams, mdfile.MermaidExtensions...)

	images := make([]string, 0, len(mdfile.ImageTypes))
	for _, t := range mdfile.ImageTypes {
		images = append(images, t.Ext)
	}

	return ExtensionsDTO{
		Markdown: append([]string(nil), mdfile.Extensions...),
		Diagrams: diagrams,
		Images:   images,
	}
}
