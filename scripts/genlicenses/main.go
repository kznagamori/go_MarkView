// genlicenses は licenses/THIRD_PARTY.md を生成する（BR-040, FR-101）。
//
//	go run ./scripts/genlicenses
//
// 収集の対象は 2 つある。
//
//  1. 実行ファイルに入る Go モジュール。`go list -deps` で main パッケージ
//     から実際に到達するものだけを採る。go.mod の require をそのまま並べると、
//     ビルドに含まれないものまで載る。
//  2. フロントエンドへ同梱する資産（Mermaid・KaTeX・github-markdown-css・
//     Octicons）。Go のツールでは収集できないため、この中に定義を持つ
//     （BR-040）。ライセンス全文は texts/ か frontend/vendor/ から読む。
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
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// collectModules は実行ファイルに入る Go モジュールを集める。
func collectModules() ([]entry, error) {
	// パスをキーにして和集合を採る。
	dirs := map[string][2]string{} // path -> {version, dir}

	for _, t := range targets {
		lines, err := listDeps(t.goos, t.tags)
		if err != nil {
			return nil, err
		}

		for _, line := range lines {
			parts := strings.Split(line, "\t")
			if len(parts) != 3 {
				continue
			}

			path, version, dir := parts[0], parts[1], parts[2]

			// メインモジュールはバージョンを持たない。自分自身は載せない。
			if version == "" {
				continue
			}
			if dir == "" {
				return nil, fmt.Errorf("%s のモジュールが展開されていない。go mod download を実行する", path)
			}

			dirs[path] = [2]string{version, dir}
		}
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("依存モジュールが 1 つも見つからない")
	}

	paths := make([]string, 0, len(dirs))
	for p := range dirs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]entry, 0, len(paths))
	for _, p := range paths {
		version, dir := dirs[p][0], dirs[p][1]

		text, err := readLicense(dir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}

		out = append(out, entry{name: p, version: version, kind: detect(text), text: text})
	}

	return out, nil
}

// listDeps は 1 つのプラットフォームについて依存モジュールを列挙する。
func listDeps(goos, tags string) ([]string, error) {
	args := []string{"list", "-deps", "-f", "{{if .Module}}{{.Module.Path}}\t{{.Module.Version}}\t{{.Module.Dir}}{{end}}"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, ".")

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS="+goos)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list (GOOS=%s): %v: %s", goos, err, strings.TrimSpace(stderr.String()))
	}

	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	return lines, nil
}

// readLicense はモジュールのディレクトリからライセンス全文を読む。
//
// **見つからなければエラーにする。** 黙って省くと、表示されないまま
// 配布してしまう（FR-101 は MUST）。
//
// 主たるライセンスに続けて、同じディレクトリにある NOTICE・PATENTS・
// 副次的なライセンスも併記する。**Apache-2.0 は NOTICE の再頒布を求めており**
// （gopkg.in/yaml.v2）、golang.org/x/* の PATENTS も許諾の一部である。
// 主たる 1 つだけを載せると、条件を満たさないまま配布することになる。
func readLicense(dir string) (string, error) {
	var primary string
	var primaryName string

	for _, name := range licenseNames {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			primary = normalize(string(data))
			primaryName = name

			break
		}
	}

	if primaryName == "" {
		return "", fmt.Errorf("ライセンスファイルが見つからない: %s", dir)
	}

	extras, err := extraFiles(dir, primaryName)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(primary)

	for _, name := range extras {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}

		fmt.Fprintf(&b, "\n\n----- %s -----\n\n%s", name, normalize(string(data)))
	}

	return b.String(), nil
}

// extraFiles は併記すべき副次的なファイルの名前を返す。
func extraFiles(dir, primary string) ([]string, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, item := range items {
		if item.IsDir() {
			continue
		}

		name := item.Name()
		if strings.EqualFold(name, primary) {
			continue
		}

		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "NOTICE") || strings.HasPrefix(upper, "PATENTS") ||
			strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "LICENCE") ||
			strings.HasPrefix(upper, "COPYING") {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names, nil
}

// collectAssets はフロントエンドへ同梱する資産の一覧を返す（BR-040）。
//
// Mermaid と KaTeX のバージョンは frontend/vendor/vendor.json が正であり
// （BR-042）、ここでは重複して持たずに読み出す。
func collectAssets() ([]entry, error) {
	mermaid, err := vendorVersion("Mermaid")
	if err != nil {
		return nil, err
	}

	katex, err := vendorVersion("KaTeX")
	if err != nil {
		return nil, err
	}

	mermaidText, err := os.ReadFile("frontend/vendor/mermaid/LICENSE")
	if err != nil {
		return nil, err
	}

	katexText, err := os.ReadFile("frontend/vendor/katex/LICENSE")
	if err != nil {
		return nil, err
	}

	gmc, err := texts.ReadFile("texts/github-markdown-css.txt")
	if err != nil {
		return nil, err
	}

	octicons, err := texts.ReadFile("texts/octicons.txt")
	if err != nil {
		return nil, err
	}

	return []entry{
		{name: "Mermaid", version: mermaid, kind: "MIT", text: normalize(string(mermaidText))},
		{name: "KaTeX", version: katex, kind: "MIT", text: normalize(string(katexText))},

		// 同梱物のファイルは持たず、frontend/css/markdown.css として
		// 書き起こしている（IMP-200）。参照した版を記録する。
		{name: "github-markdown-css", version: "5.9.0", kind: "MIT", text: normalize(string(gmc))},

		// SVG を frontend/icons/ へ手で写している（IMP-203）。**BR-042 の
		// 管理対象には加えない。** あちらは「取得したファイルを改変せずに
		// 格納する」ことが前提で、BR-043 が rc タグで自動更新する。名前を
		// 変えて置いており、実際に描画に使うのは index.html へ写した
		// <symbol> であるため、自動更新すると記録と実物が静かにずれる。
		//
		// 版はここに記録する。2026-09-02 に 20 個すべてのパスデータが
		// この版と一致することを確認した。
		{name: "Octicons", version: "19.33.0", kind: "MIT", text: normalize(string(octicons))},
	}, nil
}

// vendorVersion は vendor.json から資産のバージョンを引く（BR-042）。
//
// 依存を増やさないため、JSON の解析器を使わず素朴に走査する。
func vendorVersion(name string) (string, error) {
	data, err := os.ReadFile("frontend/vendor/vendor.json")
	if err != nil {
		return "", err
	}

	needle := `"name": "` + name + `"`
	at := strings.Index(string(data), needle)
	if at < 0 {
		return "", fmt.Errorf("vendor.json に %s がない", name)
	}

	rest := string(data)[at:]
	key := `"version": "`
	from := strings.Index(rest, key)
	if from < 0 {
		return "", fmt.Errorf("vendor.json の %s に version がない", name)
	}

	rest = rest[from+len(key):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return "", fmt.Errorf("vendor.json の %s の version が閉じていない", name)
	}

	return rest[:end], nil
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
		fmt.Fprintf(&b, "| %s | %s | %s |\n", e.name, e.version, e.kind)
	}

	writeSection(&b, "Go modules", mods)
	writeSection(&b, "Bundled assets", assets)

	return b.String()
}

func writeSection(b *strings.Builder, title string, list []entry) {
	fmt.Fprintf(b, "\n## %s\n", title)

	for _, e := range list {
		fmt.Fprintf(b, "\n### %s %s (%s)\n\n%s\n", e.name, e.version, e.kind, e.text)
	}
}
