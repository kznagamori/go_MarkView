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
			contains: []string{`<h1 id="user-content-hello-world">`},
		},
		{
			name:     "非 ASCII の id",
			in:       "## 概要",
			contains: []string{`id="user-content-概要"`},
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
			contains: []string{`class="footnote-ref"`, `class="footnotes"`, `id="user-content-fn:1"`, `href="#user-content-fnref:1"`},
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

// TestRender_Sanitize_Input は、type="checkbox" を持たない input が残らない
// ことを検証する（UT-209 ケース 14〜16・20・23・24。根拠: MD-072, NFR-030 / IMP-116）。
//
// **bluemonday は、許可した属性が 1 つでも残れば要素を残す。** `<input disabled>`
// は type が落ちて文字の入力欄として本文に出る（type の既定は text）。
// ケース 16 と 20 は、input をすべて落とす実装でタスクリストが消えることを捕まえる。
func TestRender_Sanitize_Input(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		checkbox int // 残る input（すべて type="checkbox" であること）の数
	}{
		// UT-209 ケース 14: type の無い input
		{"ケース14_data-refだけのinput", `<input data-ref="0123456789abcdef:task:0">`, 0},
		// UT-209 ケース 15
		{"ケース15_disabledだけのinput", "<input disabled>", 0},
		{"ケース15_type=textとchecked", `<input type="text" checked>`, 0},
		// UT-209 ケース 16: 過検出の検査
		{"ケース16_GFMのタスク", "- [ ] a", 1},
		// UT-209 ケース 20: 行の中の生 HTML（HTMLBlock だけを見て走査を省く実装を捕まえる）
		{"ケース20_段落の中のinputとGFMのタスク", "para <input disabled> text\n\n- [ ] a", 1},
		// UT-209 ケース 23: 自己終了タグ（開始タグだけを調べる走査を捕まえる）
		{"ケース23_自己終了タグのdisabled", "<input disabled/>", 0},
		{"ケース23_自己終了タグのdata-ref", `<input data-ref="0123456789abcdef:task:0"/>`, 0},
		// UT-209 ケース 24: <input の数を数えて走査を省く実装を捕まえる。
		// bluemonday は object を中身ごと捨てるため GFM のタスク 1 つが消え、
		// 生 HTML の input 1 つと数が合う。
		{"ケース24_objectの中のタスクと生HTMLのinput", "<object>\n\n- [ ] a\n\n</object>\n\n<input disabled>", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "", "")
			if err != nil {
				t.Fatalf("Render がエラーを返した: %v", err)
			}
			inputs := byTag(scanElements(t, res.HTML), "input")

			if len(inputs) != tt.checkbox {
				t.Errorf("input が %d 個, want %d 個\n出力: %s", len(inputs), tt.checkbox, res.HTML)
			}
			for _, in := range inputs {
				if v, _ := in.Attr("type"); v != "checkbox" {
					t.Errorf("type=%q の input が残っている（checkbox 以外を残さない）\n出力: %s", v, res.HTML)
				}
			}
		})
	}
}

// TestRender_Sanitize_Markers は、目印の属性を値の形と要素で縛って通すことを
// 検証する（UT-209 ケース 17〜19・21・22。根拠: NFR-030 / IMP-116, IMP-120）。
//
// **ケース 21 と 22 は対で意味を持つ。** 21 を欠くと図のブロックの目印を落とす
// 実装で図が 1 つも描かれず、22 を欠くとどの要素にも図の目印を通す実装が通る。
// **形を縛っても偽装は防げない**——防ぐのは鍵であり、それはフロントエンドの
// 照合（UT-815）が見る。
func TestRender_Sanitize_Markers(t *testing.T) {
	tests := []struct {
		name string
		in   string
		tag  string // 検査する要素（文書の中で最初のもの）
		attr string
		want string // 残るべき値。空なら属性が落ちること
	}{
		// UT-209 ケース 17: 値の形が違う・要素が違う
		{"ケース17_形の違うdata-ref", `<input type="checkbox" data-ref="bad">`, "input", "data-ref", ""},
		{"ケース17_鍵が15文字", `<input type="checkbox" data-ref="0123456789abcde:task:0">`, "input", "data-ref", ""},
		{"ケース17_p要素のdata-ref", `<p data-ref="0123456789abcdef:task:0">x</p>`, "p", "data-ref", ""},

		// UT-209 ケース 18: data-link は a に限る
		{"ケース18_aのdata-link", `<a href="https://example.com/" data-link="0123456789abcdef:x">x</a>`, "a", "data-link", "0123456789abcdef:x"},
		{"ケース18_divのdata-link", `<div data-link="0123456789abcdef:x">x</div>`, "div", "data-link", ""},

		// UT-209 ケース 19: 目印以外の data- 属性
		{"ケース19_data-foo", `<div data-foo="1">x</div>`, "div", "data-foo", ""},

		// UT-209 ケース 21: 図のブロックの目印は div で残る
		{"ケース21_divのmermaid", `<div data-ref="0123456789abcdef:mermaid:0">x</div>`, "div", "data-ref", "0123456789abcdef:mermaid:0"},
		{"ケース21_divのplantuml", `<div data-ref="0123456789abcdef:plantuml:2">x</div>`, "div", "data-ref", "0123456789abcdef:plantuml:2"},

		// UT-209 ケース 22: 要素と種類の組み合わせを縛る
		{"ケース22_divのtask", `<div data-ref="0123456789abcdef:task:0">x</div>`, "div", "data-ref", ""},
		{"ケース22_divのcode", `<div data-ref="0123456789abcdef:code:0">x</div>`, "div", "data-ref", ""},
		{"ケース22_divのmermaid:x", `<div data-ref="0123456789abcdef:mermaid:x">x</div>`, "div", "data-ref", ""},
		{"ケース22_divのmermaid:", `<div data-ref="0123456789abcdef:mermaid:">x</div>`, "div", "data-ref", ""},
		{"ケース22_spanのplantuml", `<span data-ref="0123456789abcdef:plantuml:0">x</span>`, "span", "data-ref", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "", "")
			if err != nil {
				t.Fatalf("Render がエラーを返した: %v", err)
			}
			els := byTag(scanElements(t, res.HTML), tt.tag)
			if len(els) == 0 {
				t.Fatalf("%s が出力に無い（検査の前提が崩れている）\n出力: %s", tt.tag, res.HTML)
			}

			got, ok := els[0].Attr(tt.attr)
			switch {
			case tt.want == "" && ok:
				t.Errorf("%s の %s=%q が残っている（落とすこと）\n出力: %s", tt.tag, tt.attr, got, res.HTML)
			case tt.want != "" && got != tt.want:
				t.Errorf("%s の %s = %q（有無 %v）, want %q\n出力: %s", tt.tag, tt.attr, got, ok, tt.want, res.HTML)
			}
		})
	}

	// UT-209 ケース 22: input の図の目印は、type が無いため input ごと残らない（ケース 14）
	t.Run("ケース22_inputのmermaid", func(t *testing.T) {
		res, err := New().Render([]byte(`<input data-ref="0123456789abcdef:mermaid:0">`), "", "")
		if err != nil {
			t.Fatalf("Render がエラーを返した: %v", err)
		}
		if n := len(byTag(scanElements(t, res.HTML), "input")); n != 0 {
			t.Errorf("input が %d 個残っている\n出力: %s", n, res.HTML)
		}
	})
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
