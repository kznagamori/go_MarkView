// genlicenses は licenses/THIRD_PARTY.md を生成する（BR-040, FR-101）。
//
//	go run ./scripts/genlicenses
//
// 収集の対象は 2 つある。
//
//  1. 実行ファイルに入る Go モジュール。`go list -deps` で main パッケージ
//     から実際に到達するものだけを採る。go.mod の require をそのまま並べると、
//     ビルドに含まれないものまで載る。
//  2. フロントエンドへ同梱する資産。Go のツールでは収集できないため別に集める
//     （BR-040）。**一覧は frontend/vendor/vendor.json が正であり、この中に
//     決め打たない**（BR-042）。名前・版・ライセンス種別・全文の位置がすべて
//     あちらにある。決め打ちで残すのは、vendor.json の管理下に無い 2 件だけ
//     （github-markdown-css と Octicons）。
//
// **対象プラットフォームを明示して和集合を採る。** Windows だけに入る
// モジュール（go-webview2）があり、生成したホストによって内容が変わると
// BR-041 の鮮度検査が環境ごとに違う結果を出してしまう。
//
// go-licenses のような外部ツールを使わないのは、CI へ導入の手間と
// ネットワークの前提を持ち込まないためである。BR-040 は「go-licenses 等」
// と定めており、同等の収集ができれば手段は問わない。
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// texts は同梱資産のうち、ライセンス全文をリポジトリ内に持たないものの原文。
//
//go:embed texts/*.txt
var texts embed.FS

const outPath = "licenses/THIRD_PARTY.md"

// targets は収集対象のプラットフォーム（BR-010 の配布対象）。
//
// GOARCH は分けない。amd64 と arm64 でモジュールの集合は変わらない。
var targets = []struct {
	goos string
	tags string
}{
	{goos: "windows"},
	{goos: "linux", tags: "webkit2_41"}, // AR-003, BR-010
}

// licenseNames は各モジュールのライセンス全文を探すときのファイル名。
// 上にあるものを優先する。
var licenseNames = []string{
	"LICENSE", "LICENSE.txt", "LICENSE.md",
	"LICENCE", "LICENCE.txt",
	"COPYING", "COPYING.txt",
	"LICENSE-MIT", "LICENSE.rst",
}

// entry は一覧の 1 件。
type entry struct {
	name    string
	version string
	kind    string // ライセンス種別（MIT など）
	note    string // 版が無いものの補足（"bundled in PlantUML"）
	text    string // 全文
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genlicenses:", err)
		os.Exit(1)
	}
}

func run() error {
	mods, err := collectModules()
	if err != nil {
		return err
	}

	assets, err := collectAssets()
	if err != nil {
		return err
	}

	body := render(mods, assets)

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(outPath, []byte(body), 0o644)
}

// vendorEntry は frontend/vendor/vendor.json の 1 件（BR-042, IMP-181 と同じ形）。
type vendorEntry struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	SPDX      string `json:"spdx"`
	License   string `json:"license"` // 全文の位置。frontend/vendor/ からの相対
	BundledIn string `json:"bundledIn"`
}

// collectAssets はフロントエンドへ同梱する資産の一覧を返す（BR-040）。
//
// **vendor.json を正として読む**（BR-042）。名前・版・ライセンス種別・全文の
// 位置はすべてあちらにあり、ここで決め打つと二重管理になる。実際、PlantUML を
// 加えたときに **Viz.js / Graphviz / Expat を含む 4 件が丸ごと漏れる**ところだった。
//
// **同梱物の中に含まれるもの（`bundledIn` を持つ行）も同じ経路で載せる**
// （BR-042, NFR-051）。Graphviz は EPL-2.0 であり、著作権表示と全文の保持が
// 再配布の条件になっている。**ここから漏らせない。**
//
// 決め打ちで残すのは 2 件だけである。どちらも vendor.json の管理下に無い。
func collectAssets() ([]entry, error) {
	vendors, err := readVendors()
	if err != nil {
		return nil, err
	}

	out := make([]entry, 0, len(vendors)+2)

	for _, v := range vendors {
		if v.License == "" {
			return nil, fmt.Errorf("vendor.json の %s に license がない（全文の位置。BR-042）", v.Name)
		}

		path := filepath.Join("frontend", "vendor", filepath.FromSlash(v.License))

		text, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s のライセンス全文を読めない: %w", v.Name, err)
		}

		// 種別は記録を使う。記録が空のときだけ全文から見立てる。
		kind := v.SPDX
		if kind == "" {
			kind = detect(string(text))
		}

		note := ""
		if v.BundledIn != "" {
			note = "bundled in " + v.BundledIn
		}

		out = append(out, entry{
			name:    v.Name,
			version: v.Version,
			kind:    kind,
			note:    note,
			text:    normalize(string(text)),
		})
	}

	gmc, err := texts.ReadFile("texts/github-markdown-css.txt")
	if err != nil {
		return nil, err
	}

	octicons, err := texts.ReadFile("texts/octicons.txt")
	if err != nil {
		return nil, err
	}

	return append(out,
		// 同梱物のファイルは持たず、frontend/css/markdown.css として
		// 書き起こしている（IMP-200）。参照した版を記録する。
		entry{name: "github-markdown-css", version: "5.9.0", kind: "MIT", text: normalize(string(gmc))},

		// SVG を frontend/icons/ へ手で写している（IMP-203）。**BR-042 の
		// 管理対象には加えない。** あちらは「取得したファイルを改変せずに
		// 格納する」ことが前提で、BR-043 が rc タグで自動更新する。名前を
		// 変えて置いており、実際に描画に使うのは index.html へ写した
		// <symbol> であるため、自動更新すると記録と実物が静かにずれる。
		//
		// 版はここに記録する。2026-09-02 に 20 個すべてのパスデータが
		// この版と一致することを確認した。
		entry{name: "Octicons", version: "19.33.0", kind: "MIT", text: normalize(string(octicons))},
	), nil
}

// readVendors は vendor.json を読む（BR-042）。
func readVendors() ([]vendorEntry, error) {
	data, err := os.ReadFile(filepath.Join("frontend", "vendor", "vendor.json"))
	if err != nil {
		return nil, fmt.Errorf("vendor.json を読めない: %w", err)
	}

	var entries []vendorEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("vendor.json を解析できない: %w", err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("vendor.json が空である")
	}

	return entries, nil
}

// detect はライセンス全文から種別を判定する。
//
// 判定できない場合は "see text below" とし、**推測で名前を付けない。**
// 全文は必ず載るため、種別が空でも FR-101 は満たせる。
func detect(text string) string {
	t := strings.ToLower(text)

	switch {
	case strings.Contains(t, "apache license") && strings.Contains(t, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(t, "mozilla public license") && strings.Contains(t, "version 2.0"):
		return "MPL-2.0"
	case strings.Contains(t, "gnu general public license"):
		return "GPL"
	case strings.Contains(t, "redistribution and use in source and binary forms"):
		if strings.Contains(t, "neither the name") {
			return "BSD-3-Clause"
		}
		return "BSD-2-Clause"
	case strings.Contains(t, "permission to use, copy, modify, and") && strings.Contains(t, "distribute this software"):
		return "ISC"
	case strings.Contains(t, "permission is hereby granted, free of charge"):
		return "MIT"
	}

	return "see text below"
}

// normalize は改行を LF へ揃え、末尾の空白行を落とす。
//
// 生成結果を安定させるためで、これがないと同梱物の改行コードの違いが
// そのまま差分として現れ、BR-041 の検査が環境で揺れる。
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	return strings.TrimRight(s, "\n \t")
}

// render は Markdown を組み立てる。
//
// **利用者に見える文書のため英語で書く**（CLAUDE.md）。情報ダイアログの
// テキストボックスへそのまま流し込まれる（FR-101, DSP-171）。
func render(mods, assets []entry) string {
	var b strings.Builder

	b.WriteString("# Third-party licenses\n\n")
	b.WriteString("MarkView includes the third-party software listed below.\n\n")
	b.WriteString("This file is generated by `go run ./scripts/genlicenses`. Do not edit it by hand.\n")
	b.WriteString("The Go modules are those reachable from the main package on Windows and Linux;\n")
	b.WriteString("the bundled assets are the JavaScript, CSS, font and icon files shipped inside the executable.\n\n")

	b.WriteString("## Summary\n\n")
	b.WriteString("| Name | Version | License |\n| --- | --- | --- |\n")
	for _, e := range append(append([]entry{}, mods...), assets...) {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", e.name, versionCell(e), e.kind)
	}

	writeSection(&b, "Go modules", mods)
	writeSection(&b, "Bundled assets", assets)

	return b.String()
}

func writeSection(b *strings.Builder, title string, list []entry) {
	fmt.Fprintf(b, "\n## %s\n", title)

	for _, e := range list {
		fmt.Fprintf(b, "\n### %s (%s)\n\n%s\n", heading(e), e.kind, e.text)
	}
}

// heading は見出しの「名前と版」の部分を組み立てる。
//
// **版が無いものがある**（BR-042）。`viz-global.js` の告知は Graphviz と Expat の
// 版を書いておらず、空のまま並べると `Graphviz  (EPL-2.0)` のように空白が空く。
// 版の代わりに、どこに同梱されているかを書く。
func heading(e entry) string {
	name := e.name
	if e.version != "" {
		name += " " + e.version
	}

	if e.note != "" {
		name += ", " + e.note
	}

	return name
}

// versionCell は一覧表の Version 欄を返す。版が無いものは補足で埋める。
func versionCell(e entry) string {
	if e.version != "" {
		return e.version
	}

	if e.note != "" {
		return e.note
	}

	return "-"
}
