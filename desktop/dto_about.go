package desktop

import "github.com/kznagamori/go_MarkView/internal/buildinfo"

// 本ファイルはアプリケーション情報ウィンドウ（FR-100, FR-101）の境界の型を定める（IMP-306）。
// dto.go が 400 行の目安（IMP-011）を超えたため分けた。

// アプリケーション情報の固定値（FR-100, UI-100）。
//
// 利用者に見える文言であるため英語とする（UI-024）。ラベル（Author など）は
// フロントエンドの strings.js が持ち、ここが持つのは値だけである（IMP-290）。
const (
	appAuthor     = "kznagamori"
	appRepository = "https://github.com/kznagamori/go_MarkView"
	appLicense    = "MIT License"
)

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

		// **Bundled() を入れる。Vendors() の全体ではない**（IMP-306, IMP-181）。
		// 同梱物の中に含まれるもの（Viz.js / Graphviz / Expat）は Bundled 行に
		// 出さず、Licenses の中に全文として現れる（UI-100, FR-101）。
		Vendors:  buildinfo.Bundled(),
		Licenses: licenses,
	}
}
