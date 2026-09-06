package renderer

import (
	"strings"
	"testing"
)

// TestRender_Math は数式の保護を検証する（UT-205。根拠: MD-060 / IMP-113）。
func TestRender_Math(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-205 ケース 5: この保護を実装する動機そのもの（境界値を先に。UT-013）
		{
			name:        "下付き文字の _ が強調にならない",
			in:          "$x_1 + x_2$",
			contains:    []string{`<span class="math-inline">x_1 + x_2</span>`},
			notContains: []string{"<em>"},
		},

		// UT-205 ケース 4: 通貨表記は数式にしない
		{
			name:        "通貨表記",
			in:          "$100 と $200",
			contains:    []string{"$100 と $200"},
			notContains: []string{"math-inline", "math-block"},
		},

		// UT-205 ケース 6・7: コードの内側は対象外
		{
			name:        "インラインコードの中の $",
			in:          "`$a$`",
			contains:    []string{"<code>$a$</code>"},
			notContains: []string{"math-inline", "math-block"},
		},
		{
			name: "コードブロックの中の $",
			in:   "```go\n$var\n```",
			// ハイライトが入ると $var は span に分割される。ここで主張したいのは
			// 「数式にならないこと」なので、コードブロックとして出ていれば足りる。
			contains:    []string{"code-block"},
			notContains: []string{"math-inline", "math-block"},
		},

		// UT-205 ケース 1〜3: 基本形
		{
			name:     "インライン数式",
			in:       "$a+b$",
			contains: []string{`<span class="math-inline">a+b</span>`},
		},
		{
			name:        "ブロック数式",
			in:          "$$a+b$$",
			contains:    []string{`<div class="math-block">a+b</div>`},
			notContains: []string{"math-inline"},
		},
		{
			name:        "math のコードブロック",
			in:          "```math\na+b\n```",
			contains:    []string{`<div class="math-block">a+b</div>`},
			notContains: []string{"<pre", "<code"},
		},

		// UT-090 に従って追加した境界値。$ の判定規則（MD-060）を固める。
		{
			name:        "開始の $ の直後が空白",
			in:          "$ a+b$",
			notContains: []string{"math-inline"},
		},
		{
			name:        "終了の $ の直前が空白",
			in:          "$a+b $",
			notContains: []string{"math-inline"},
		},
		{
			name:        "$ が 1 つだけ",
			in:          "a $ b",
			notContains: []string{"math-inline"},
		},
		{
			name:        "終端が同じ行にない",
			in:          "$a+b\n\nc$",
			notContains: []string{"math-inline"},
		},
		{
			name:     "1 行に 2 つの数式",
			in:       "$a$ と $b$",
			contains: []string{`<span class="math-inline">a</span>`, `<span class="math-inline">b</span>`},
		},
		{
			name:     "数式の中身を HTML エスケープする",
			in:       "$a<b$",
			contains: []string{`<span class="math-inline">a&lt;b</span>`},
		},
		{
			name:     "複数行のブロック数式",
			in:       "$$\n" + `\frac{a}{b}` + "\n$$",
			contains: []string{`<div class="math-block">\frac{a}{b}</div>`},
		},
		{
			name:        "$$ だけの段落は数式にしない",
			in:          "$$",
			notContains: []string{"math-block", "math-inline"},
		},
		{
			// 長さの下限だけでは弾けない。$$ と $$ の間に空白しかない形。
			name:        "中身が空白だけのブロック数式にしない",
			in:          "$$ $$",
			notContains: []string{"math-block", "math-inline"},
		},
		{
			name:        "中身のない $$$$ は数式にしない",
			in:          "$$$$",
			notContains: []string{"math-block", "math-inline"},
		},
		{
			// 段落の途中の $$…$$ は本文のまま残す。ブロック要素を段落の
			// 内側へ置けないため、インラインの数式には落とさない。
			name:        "段落の途中の $$",
			in:          "text $$a+b$$ more",
			contains:    []string{"$$a+b$$"},
			notContains: []string{"math-inline", "math-block"},
		},
		{
			name:     "ブロック数式に続く本文",
			in:       "$$a+b$$\n\npara",
			contains: []string{`<div class="math-block">a+b</div>`, "<p>para</p>"},
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

// TestRender_NeedsKaTeX は遅延ロードの判定を検証する
// （UT-205 ケース 8・9。根拠: MD-061 / IMP-113, NFR-013）。
//
// false になるべき文書で true を返すと、数式のない文書でも KaTeX を
// 読み込むことになり、NFR-013 が成立しなくなる。
func TestRender_NeedsKaTeX(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"数式を含まない文書", "# H\n\npara", false},
		{"通貨表記のみ", "$100 と $200", false},
		{"インラインコードの中の $", "`$a$`", false},
		{"コードブロックの中の $", "```go\n$var\n```", false},
		{"空の文書", "", false},

		{"インライン数式", "$a+b$", true},
		{"ブロック数式", "$$a+b$$", true},
		{"math のコードブロック", "```math\na+b\n```", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "")
			if err != nil {
				t.Fatalf("Render がエラーを返した: %v", err)
			}
			if res.NeedsKaTeX != tt.want {
				t.Errorf("NeedsKaTeX = %v, want %v\n出力: %s", res.NeedsKaTeX, tt.want, res.HTML)
			}
		})
	}
}

// TestRender_MathStructure は IMP-113 が定める出力の形を突き合わせる。
//
// フロントエンドは .math-inline / .math-block を要素単位で拾い、textContent を
// KaTeX へ渡す（IMP-232）。クラス名と要素の入れ子が変わると数式が描画されない。
func TestRender_MathStructure(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"インライン", "$a+b$", "<p><span class=\"math-inline\">a+b</span></p>\n"},
		{"ブロック", "$$a+b$$", "<div class=\"math-block\">a+b</div>\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := render(t, tt.in); got != tt.want {
				t.Errorf("出力が構造と一致しない\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
