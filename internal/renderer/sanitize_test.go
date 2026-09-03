package renderer

import (
	"strings"
	"testing"
)

// 1x1 の PNG。data:image/* の許可（MD-072）を確かめるために使う。
const pngDataURI = "data:image/png;base64," +
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// TestRender_Sanitize_Removes は危険な要素・属性が除去されることを検証する
// （UT-209 ケース 1〜3, 6, 7, 9。根拠: MD-072 / IMP-116, NFR-030）。
func TestRender_Sanitize_Removes(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		notContains []string
	}{
		// UT-209 ケース 1: スクリプト
		{"script 要素", "<script>alert(1)</script>", []string{"<script", "alert(1)"}},
		{"インラインの script", "text <script>alert(1)</script> text", []string{"<script", "alert(1)"}},

		// UT-209 ケース 2: イベントハンドラ属性
		{"img の onerror", `<img src="x" onerror="alert(1)">`, []string{"onerror", "alert(1)"}},
		{"div の onclick", `<div onclick="alert(1)">x</div>`, []string{"onclick", "alert(1)"}},
		{"onload", `<img src="x" onload="alert(1)">`, []string{"onload"}},

		// UT-209 ケース 3: 危険なスキーム
		{"javascript: の href", `<a href="javascript:alert(1)">x</a>`, []string{"javascript:"}},
		{"vbscript: の href", `<a href="vbscript:msgbox(1)">x</a>`, []string{"vbscript:"}},
		{"Markdown 記法の javascript:", "[x](javascript:alert(1))", []string{"javascript:"}},
		{"大文字混じりの JavaScript:", `<a href="JaVaScRiPt:alert(1)">x</a>`, []string{"avaScript:", "alert(1)"}},

		// UT-209 ケース 6: 埋め込み
		{"iframe", `<iframe src="https://example.com"></iframe>`, []string{"<iframe", "example.com"}},
		{"object", `<object data="x.swf"></object>`, []string{"<object"}},
		{"embed", `<embed src="x.swf">`, []string{"<embed"}},
		{"form と input", `<form><input type="text" name="a"></form>`, []string{"<form", `type="text"`}},
		{"style 要素", "<style>body{display:none}</style>", []string{"<style", "display:none"}},
		{"link と meta", `<link rel="stylesheet" href="x.css"><meta charset="utf-8">`, []string{"<link", "<meta"}},
		{"base", `<base href="https://example.com/">`, []string{"<base"}},

		// UT-209 ケース 7: インライン SVG
		{"svg ごと除去する", "<svg><script>alert(1)</script></svg>", []string{"<svg", "alert(1)"}},
		{"svg の図形も残さない", `<svg><circle cx="1"/></svg>`, []string{"<svg", "<circle"}},
		{"math 要素", "<math><mi>x</mi></math>", []string{"<math"}},

		// UT-209 ケース 9: 任意のクラス
		{"攻撃者が決めたクラス", `<div class="attacker-defined">x</div>`, []string{"attacker-defined"}},
		{"許可された語に似たクラス", `<div class="code-block-evil">x</div>`, []string{"code-block-evil"}},
		{"許可された語を含む並び", `<span class="k attacker">x</span>`, []string{"attacker"}},

		// UT-090 に従って追加。MD-072 が既定で除去すると定める style 属性。
		{"style 属性", `<div style="position:fixed">x</div>`, []string{"style=", "position:fixed"}},
		{"data:image/svg+xml", "![a](data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=)", []string{"svg+xml"}},
		{"チェックボックス以外の input", `<input type="text">`, []string{`type="text"`}},
		{"属性のない input", "<input>", []string{"<input"}},
		{"チェックボックスに見せた text", `<input type="checkbox" onclick="x()">`, []string{"onclick"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)

			for _, ng := range tt.notContains {
				if strings.Contains(got, ng) {
					t.Errorf("出力に %q が残っている\n出力: %s", ng, got)
				}
			}
		})
	}
}

// TestRender_Sanitize_Keeps は、サニタイズが自前の出力や許可要素まで
// 落としていないことを検証する（UT-209 ケース 4, 5, 8, 10〜12）。
//
// **除去のテストだけを書くと「すべて除去する」実装でも通ってしまう。**
// UT-209 が両方向を求めているのはそのためである。
func TestRender_Sanitize_Keeps(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		contains []string
	}{
		// UT-209 ケース 4・5・8: MD-072 の許可要素
		{
			name:     "details と summary",
			in:       "<details><summary>s</summary>b</details>",
			contains: []string{"<details>", "<summary>s</summary>", "b"},
		},
		{
			name:     "br / kbd / sub / sup",
			in:       "a<br><kbd>Ctrl</kbd><sub>1</sub><sup>2</sup>",
			contains: []string{"<br", "<kbd>Ctrl</kbd>", "<sub>1</sub>", "<sup>2</sup>"},
		},
		{
			name:     "生 HTML の table",
			in:       "<table><thead><tr><th>a</th></tr></thead><tbody><tr><td>b</td></tr></tbody></table>",
			contains: []string{"<table>", "<th>a</th>", "<td>b</td>"},
		},
		{
			name:     "その他の許可要素",
			in:       "<mark>m</mark><ins>i</ins><abbr>a</abbr><figure><figcaption>c</figcaption></figure>",
			contains: []string{"<mark>m</mark>", "<ins>i</ins>", "<abbr>a</abbr>", "<figcaption>c</figcaption>"},
		},

		// UT-209 ケース 10: 自前のクラスが残る
		{
			name:     "コードブロックとハイライトのクラス",
			in:       "```go\nfunc main() {}\n```",
			contains: []string{`class="code-block"`, `class="chroma"`, `<span class=`},
		},
		{
			name:     "Alerts のクラス",
			in:       "> [!NOTE]\n> body",
			contains: []string{`class="markdown-alert markdown-alert-note"`, `class="markdown-alert-title"`},
		},
		{
			name:     "数式のクラス",
			in:       "$a+b$ と\n\n$$c$$",
			contains: []string{`class="math-inline"`, `class="math-block"`},
		},

		// UT-209 ケース 11: 見出しアンカー
		{
			name:     "見出しの id",
			in:       "# Hello World",
			contains: []string{`<h1 id="hello-world">`},
		},
		{
			name:     "非 ASCII の id",
			in:       "## 概要",
			contains: []string{`id="概要"`},
		},

		// UT-209 ケース 12: Mermaid の属性
		{
			name: "data-lang / data-mermaid / data-source",
			in:   "```mermaid\ngraph TD\n```",
			contains: []string{
				`data-lang="mermaid"`, `data-mermaid="1"`, `data-source="graph TD"`,
				`class="mermaid-source"`,
			},
		},

		// UT-209 ケース 12: PlantUML の属性（IMP-116）。
		// **落とすと描画対象がフロントエンドから見えなくなる**（IMP-119, IMP-233）。
		{
			name: "data-plantuml / data-source",
			in:   "```plantuml\n@startuml\n@enduml\n```",
			contains: []string{
				`data-lang="plantuml"`, `data-plantuml="1"`, `data-source="@startuml`,
			},
		},
		{
			name:     "data-puml-error",
			in:       "```plantuml\n@startuml\n!include a.puml\n@enduml\n```",
			contains: []string{`data-puml-error="include"`},
		},

		// UT-209 ケース 13: pre の class 接頭辞（IMP-116）。
		// 接頭辞の一覧から漏れると、描画前のソース表示が素の pre になる。
		{
			name:     "pre class=mermaid-source",
			in:       "```mermaid\ngraph TD\n```",
			contains: []string{`<pre class="mermaid-source">`},
		},
		{
			name:     "pre class=plantuml-source",
			in:       "```plantuml\n@startuml\n@enduml\n```",
			contains: []string{`<pre class="plantuml-source">`},
		},

		// UT-090 に従って追加。落ちると機能が壊れるもの。
		{
			name:     "タスクリストのチェックボックス",
			in:       "- [x] done",
			contains: []string{"<input", `type="checkbox"`, "checked", "disabled"},
		},
		{
			name:     "表の桁揃え",
			in:       "| a | b |\n| :--- | ---: |\n| 1 | 2 |",
			contains: []string{`align="left"`, `align="right"`},
		},
		{
			name:     "脚注のリンクとクラス",
			in:       "x[^1]\n\n[^1]: note",
			contains: []string{`class="footnote-ref"`, `class="footnotes"`, `id="fn:1"`, `href="#fnref:1"`},
		},
		{
			name:     "相対リンクとアンカー",
			in:       "[a](./other.md) [b](#sec) [c](https://example.com) [d](mailto:x@example.com)",
			contains: []string{`href="./other.md"`, `href="#sec"`, `href="https://example.com"`, `href="mailto:x@example.com"`},
		},
		{
			name:     "内部アセットサーバのパス",
			in:       `<img src="/__local/a.png" alt="a" width="10" height="20">`,
			contains: []string{`src="/__local/a.png"`, `alt="a"`, `width="10"`, `height="20"`},
		},
		{
			name:     "data:image/png",
			in:       "![a](" + pngDataURI + ")",
			contains: []string{"data:image/png;base64,"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("出力から %q が失われている\n出力: %s", want, got)
				}
			}
		})
	}
}

// TestPolicy_RejectsSVG は、許可リストに svg が入っていないことを検証する
// （IMP-112 の IMPORTANT）。
//
// 「アイコンは SVG で表示する」（MD-040）と「生 HTML の svg は除去する」
// （MD-072）は、Go 側で SVG を出そうとすると衝突する。アイコンの付与を
// フロントエンドに寄せることで両立させており、**許可リストに svg を追加して
// 解決してはならない**。この線が動いていないことを直接見る。
func TestPolicy_RejectsSVG(t *testing.T) {
	for _, el := range allowedElements {
		if el == "svg" || el == "math" || el == "script" || el == "style" || el == "iframe" {
			t.Errorf("許可要素に %q が含まれている（MD-072 の除去対象）", el)
		}
	}

	if got := Policy().Sanitize("<svg><path d=\"M0 0\"/></svg>"); strings.Contains(got, "svg") {
		t.Errorf("svg が除去されていない: %q", got)
	}
}
