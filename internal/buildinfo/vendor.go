package buildinfo

import (
	"encoding/json"
	"runtime"
	"strings"
)

// VendorEntry は同梱資産 1 件の情報（IMP-181, BR-042）。
//
// Mermaid と KaTeX は実ファイルをリポジトリにコミットして管理し、その素性を
// vendor.json に記録する。アプリケーション情報ウィンドウの Bundled 行
// （UI-100）に表示する。
type VendorEntry struct {
	Name      string `json:"name"`
	Version   string `json:"version"` // 分からない場合は空（BR-042）
	SPDX      string `json:"spdx"`    // ライセンス種別。NFR-051 の判定に使う
	License   string `json:"license"` // 全文の位置。frontend/vendor/ からの相対パス
	Source    string `json:"source"`
	Fetched   string `json:"fetched"`
	BundledIn string `json:"bundledIn,omitempty"` // 同梱元の資産名。最上位なら空
}

// vendorJSON は埋め込まれた vendor.json の中身。
//
// このパッケージからは go:embed で読めない。実体は frontend/vendor/ にあり、
// go:embed はパッケージのディレクトリより上を参照できないためである。
// main.go が埋め込んだ内容を SetVendorJSON で渡す。
var vendorJSON []byte

// SetVendorJSON は埋め込んだ vendor.json を登録する（IMP-181）。
//
// main.go が起動時に 1 度だけ呼ぶ。読めなかった場合は呼ばなくてよく、
// そのとき Vendors は空スライスを返す。
func SetVendorJSON(data []byte) {
	vendorJSON = data
}

// Vendors は埋め込まれた vendor.json を解析して返す（IMP-181）。
//
// **壊れていても落とさない。** 情報表示が欠けるだけで、文書の閲覧は続けられる
// （FR-111）。戻り値は常に非 nil であり、呼び出し側は長さだけを見ればよい。
func Vendors() []VendorEntry {
	return parseVendors(vendorJSON)
}

// Bundled は Bundled 行（UI-100）に出す資産だけを返す（IMP-181）。
//
// BundledIn が空のもの、つまり最上位の資産に限る。同梱物の中に含まれるもの
// （Viz.js / Graphviz / Expat）は版を持たないことがあり、並べても確認の役に
// 立たない。それらはライセンス表示（FR-101）に全文とともに現れる。
//
// **絞り込みをここ 1 か所に置く**（IMP-181）。フロントエンドで絞ると、
// 絞り方が 2 つに分かれる（IMP-306）。
//
// 記録の順序をそのまま保つ。vendor.json の並びが Bundled 行の並びになる。
// 戻り値は常に非 nil であり、呼び出し側は長さだけを見ればよい（FR-111）。
func Bundled() []VendorEntry {
	all := Vendors()

	bundled := make([]VendorEntry, 0, len(all))
	for _, entry := range all {
		if entry.BundledIn == "" {
			bundled = append(bundled, entry)
		}
	}

	return bundled
}

// parseVendors は vendor.json を解析する。
//
// 形式は VendorEntry の配列とする（BR-042）。
func parseVendors(data []byte) []VendorEntry {
	entries := []VendorEntry{}

	if len(data) == 0 {
		return entries
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		// 途中まで読めていても捨てる。半端な一覧を見せる意味はない。
		return []VendorEntry{}
	}
	if entries == nil {
		// JSON の null は解析に成功したうえで nil を書き込む。
		// 常に非 nil を返す約束を保つ。
		return []VendorEntry{}
	}

	return entries
}

// Environment は実行環境を 1 行にまとめる（IMP-181, UI-100）。
//
//	windows/amd64  Go 1.26.5  WebView2 120.0.0
//
// webviewVersion が空の場合、その区画ごと省略する。Wails から取得できない
// ことがあり、「WebView2 」とだけ書かれた行は情報として役に立たないため。
func Environment(webviewVersion string) string {
	parts := []string{
		runtime.GOOS + "/" + runtime.GOARCH,
		"Go " + strings.TrimPrefix(runtime.Version(), "go"),
	}

	if webviewVersion != "" {
		parts = append(parts, webviewName()+" "+webviewVersion)
	}

	return strings.Join(parts, "  ")
}

// webviewName は OS ごとの WebView の名称を返す（AR-001）。
func webviewName() string {
	if runtime.GOOS == "windows" {
		return "WebView2"
	}
	return "WebKitGTK"
}
