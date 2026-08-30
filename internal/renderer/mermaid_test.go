package renderer

import (
	"strings"
	"testing"
)

// TestRender_Mermaid は Mermaid ブロックの出力を検証する
// （UT-207。根拠: MD-080 / IMP-115）。
func TestRender_Mermaid(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-207 ケース 7: math は Mermaid ではない（境界値を先に。UT-013）
		{
			name:        "math のコードブロックは Mermaid にしない",
			in:          "```math\na+b\n```",
			contains:    []string{"math-block"},
			notContains: []string{"data-mermaid", "mermaid-source"},
		},
		{
			name:        "通常のコードブロック",
			in:          "```go\nx := 1\n```",
			notContains: []string{"data-mermaid", "mermaid-source", "data-source"},
		},

		// UT-207 ケース 1〜3
		{
			name: "Mermaid ブロック",
			in:   "```mermaid\ngraph TD\n```",
			contains: []string{
				`data-lang="mermaid"`,
				`data-mermaid="1"`,
				`data-source="graph TD"`,
				`<pre class="mermaid-source">graph TD</pre>`,
			},
			notContains: []string{"chroma"},
		},

		// UT-207 ケース 4: エスケープ
		{
			name: "記号を含むソース",
			in:   "```mermaid\nA-->B & \"q\" <x>\n```",
			contains: []string{
				`data-source="A--&gt;B &amp; &#34;q&#34; &lt;x&gt;"`,
				"<pre class=\"mermaid-source\">A--&gt;B &amp; &#34;q&#34; &lt;x&gt;</pre>",
			},
		},

		// UT-090 に従って追加した境界値
		{
			name: "複数行のソースは属性で &#10; になる",
			in:   "```mermaid\ngraph TD\n  A-->B\n```",
			contains: []string{`data-source="graph TD
  A--&gt;B"`},
		},
		{
			name:     "言語名が大文字",
			in:       "```MERMAID\ngraph TD\n```",
			contains: []string{`data-mermaid="1"`},
		},
		{
			name:     "空の Mermaid ブロック",
			in:       "```mermaid\n```",
			contains: []string{`data-source=""`, `<pre class="mermaid-source"></pre>`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, ng := range tt.notContains {
				if strings.Contains(got, ng) {
					t.Errorf("出力に %q が含まれている\n出力: %s", ng, got)
				}
			}
		})
	}
}

// TestRender_MermaidStructure は IMP-115 が固定した構造そのものを検証する。
//
// 期待値はサニタイズ後の形、つまりフロントエンドが実際に受け取る HTML である。
// 最後段のサニタイズ（IMP-116）が数値文字参照を実体へ戻すため、属性中の
// &#10; は生の改行として現れる。値として改行が保たれていればよい。
//
// data-source はフロントエンドの生命線である。Mermaid は描画後に <pre> が SVG へ
// 置き換わり、DOM から原文が失われる。属性名が変わるとコピーボタン（FR-060）と
// テーマ切り替え時の再描画（IMP-231）が同時に壊れる。
func TestRender_MermaidStructure(t *testing.T) {
	const want = `<div class="code-block" data-lang="mermaid" data-mermaid="1" data-source="graph TD
  A--&gt;B">
<pre class="mermaid-source">graph TD
  A--&gt;B</pre>
</div>
`

	if got := render(t, "```mermaid\ngraph TD\n  A-->B\n```"); got != want {
		t.Errorf("出力が構造と一致しない\n got: %q\nwant: %q", got, want)
	}
}

// TestRender_NeedsMermaid は遅延ロードの判定を検証する
// （UT-207 ケース 5・6。根拠: MD-080 / IMP-115, NFR-013）。
func TestRender_NeedsMermaid(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"Mermaid を含まない文書", "# H\n\npara", false},
		{"通常のコードブロック", "```go\nx := 1\n```", false},
		{"math のコードブロック", "```math\na+b\n```", false},
		{"mermaid という語を含む本文", "mermaid の話", false},
		{"インラインコードの mermaid", "`mermaid`", false},

		{"Mermaid ブロック", "```mermaid\ngraph TD\n```", true},
		{"言語名が大文字", "```MERMAID\ngraph TD\n```", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "")
			if err != nil {
				t.Fatalf("Render がエラーを返した: %v", err)
			}
			if res.NeedsMermaid != tt.want {
				t.Errorf("NeedsMermaid = %v, want %v\n出力: %s", res.NeedsMermaid, tt.want, res.HTML)
			}
		})
	}
}
