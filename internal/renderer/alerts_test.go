package renderer

import (
	"strings"
	"testing"
)

// TestRender_Alerts は GitHub Alerts の変換を検証する
// （UT-204。根拠: MD-040 / IMP-112）。
func TestRender_Alerts(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-204 ケース 6: 通常の引用は変換しない（境界値を先に置く。UT-013）
		{
			name:        "通常の引用",
			in:          "> text",
			contains:    []string{"<blockquote>", "<p>text</p>"},
			notContains: []string{"markdown-alert"},
		},

		// UT-204 ケース 4: 未知の種別は通常の引用として残す
		{
			name:        "未知の種別",
			in:          "> [!FOO]\n> body",
			contains:    []string{"<blockquote>", "[!FOO]"},
			notContains: []string{"markdown-alert"},
		},

		// UT-204 ケース 1: 基本形
		{
			name: "NOTE",
			in:   "> [!NOTE]\n> body",
			contains: []string{
				`<div class="markdown-alert markdown-alert-note">`,
				`<p class="markdown-alert-title">Note</p>`,
				"<p>body</p>",
			},
			notContains: []string{"<blockquote>", "[!NOTE]"},
		},

		// UT-204 ケース 2: 大文字小文字を区別しない
		{
			name:     "小文字の tip",
			in:       "> [!tip]\n> body",
			contains: []string{"markdown-alert-tip", `<p class="markdown-alert-title">Tip</p>`},
		},
		{
			name:     "大文字小文字が混在",
			in:       "> [!CaUtIoN]\n> body",
			contains: []string{"markdown-alert-caution", `<p class="markdown-alert-title">Caution</p>`},
		},

		// UT-204 ケース 3: 残りの種別
		{
			name:     "IMPORTANT",
			in:       "> [!IMPORTANT]\n> body",
			contains: []string{"markdown-alert-important", `<p class="markdown-alert-title">Important</p>`},
		},
		{
			name:     "WARNING",
			in:       "> [!WARNING]\n> body",
			contains: []string{"markdown-alert-warning", `<p class="markdown-alert-title">Warning</p>`},
		},
		{
			name:     "CAUTION",
			in:       "> [!CAUTION]\n> body",
			contains: []string{"markdown-alert-caution", `<p class="markdown-alert-title">Caution</p>`},
		},

		// UT-204 ケース 5: 本文がない
		{
			name:        "マーカーのみ",
			in:          "> [!NOTE]",
			contains:    []string{"markdown-alert-note", `<p class="markdown-alert-title">Note</p>`},
			notContains: []string{"[!NOTE]", "<p></p>"},
		},

		// UT-090 に従って追加した境界値
		{
			name:        "マーカーと同じ行に本文がある場合は変換しない",
			in:          "> [!NOTE] body",
			contains:    []string{"<blockquote>", "[!NOTE] body"},
			notContains: []string{"markdown-alert"},
		},
		{
			name:        "マーカーが 2 行目にある場合は変換しない",
			in:          "> body\n> [!NOTE]",
			contains:    []string{"<blockquote>"},
			notContains: []string{"markdown-alert"},
		},
		{
			name:     "引用記号の後に空白がない",
			in:       ">[!NOTE]\n>body",
			contains: []string{"markdown-alert-note", "<p>body</p>"},
		},
		{
			name:     "本文が複数段落",
			in:       "> [!NOTE]\n> first\n>\n> second",
			contains: []string{"markdown-alert-note", "<p>first</p>", "<p>second</p>"},
		},
		{
			name:     "本文にインライン記法を含む",
			in:       "> [!NOTE]\n> **強調**と`code`",
			contains: []string{"markdown-alert-note", "<strong>強調</strong>", "<code>code</code>"},
		},
		// v1.1.0 の「入れ子の引用の中の Alert」（Alert にする期待値）は、UT-204 のケース 9 で
		// 改めた（BUG-025。4.75.0 で MD-040 を改訂した）。TestRender_AlertsNested が見る。
		{
			name:        "空の種別",
			in:          "> [!]\n> body",
			contains:    []string{"<blockquote>"},
			notContains: []string{"markdown-alert"},
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

			// UT-204 ケース 7: Go 側は SVG を出力しない。
			// アイコンはフロントエンドが後処理で付与する（IMP-112, IMP-225）。
			if strings.Contains(got, "<svg") {
				t.Errorf("出力に <svg> が含まれている。アイコンは Go 側で出力しない\n出力: %s", got)
			}
		})
	}
}

// TestRender_AlertStructure は IMP-112 が固定した HTML 構造そのものを検証する。
//
// クラス名と要素の並びは、CSS（DSP-260）とフロントエンドのアイコン付与
// （IMP-225）の両方が前提にしている。部分一致では並びの入れ替わりを
// 見逃すため、ここだけは出力全体を突き合わせる。
func TestRender_AlertStructure(t *testing.T) {
	const want = `<div class="markdown-alert markdown-alert-warning">
<p class="markdown-alert-title">Warning</p>
<p>本文</p>
</div>
`

	if got := render(t, "> [!WARNING]\n> 本文"); got != want {
		t.Errorf("出力が構造と一致しない\n got: %q\nwant: %q", got, want)
	}
}

// TestRender_AlertHeadings は、Alert の中の見出しも抽出されることを検証する
// （IMP-112 と IMP-117 の組み合わせ）。
//
// Alert への差し替えで Blockquote が別のノードに変わるため、見出しの走査が
// 途切れていないかを見る。
func TestRender_AlertHeadings(t *testing.T) {
	res, err := New().Render([]byte("> [!NOTE]\n> # 見出し"), Options{})
	if err != nil {
		t.Fatalf("Render がエラーを返した: %v", err)
	}

	want := []Heading{{Level: 1, Text: "見出し", ID: "user-content-見出し"}}
	if len(res.Headings) != 1 || res.Headings[0] != want[0] {
		t.Errorf("Headings = %+v, want %+v", res.Headings, want)
	}
}

// TestRender_AlertsNested は、文書の一番外側の引用だけを Alert にすることを検証する
// （UT-204 のケース 8〜11。根拠: MD-040, MD-023 / IMP-112。BUG-025）。
//
// GitHub は入れ子の引用を Alert として描かない（「Alerts cannot be nested within other elements.」）。
// v1.0.0 から、入れ子の引用も Alert にしていた。**ケース 11 と対で書く**——8〜10 だけなら
// 「Alert を一切作らない」実装が通る。
func TestRender_AlertsNested(t *testing.T) {
	// ケース 8〜10 の入れ子の形。
	const (
		inList     = "- > [!NOTE]\n  > 本文\n"
		inQuote    = "> > [!NOTE]\n> > 本文\n"
		inFootnote = "本文[^1]\n\n[^1]: 注\n\n    > [!TIP]\n    > 中身\n"
	)

	tests := []struct {
		name       string
		in         string
		blockquote int // 通常の <blockquote> の数
	}{
		// ケース 8: リストの項目の中の引用。[!NOTE] の文字が本文に残る。
		{"8 リストの項目の中", inList, 1},
		// ケース 9: 引用の中の引用。外側も内側も通常の <blockquote>。
		{"9 引用の中の引用", inQuote, 2},
		// ケース 10: 本文から参照する脚注の定義の中の引用（参照されていない定義は goldmark が除く）。
		{"10 脚注の定義の中", inFootnote, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)
			els := scanElements(t, got)

			if n := len(withClass(els, "markdown-alert")); n != 0 {
				t.Errorf("入れ子の引用が Alert になった（%d 個）\n出力: %s", n, got)
			}
			if n := len(byTag(els, "blockquote")); n != tt.blockquote {
				t.Errorf("<blockquote> が %d 個、want %d\n出力: %s", n, tt.blockquote, got)
			}
			if !strings.Contains(got, "[!") {
				t.Errorf("[!種別] の文字が本文に残っていない\n出力: %s", got)
			}
		})
	}

	// ケース 11: 8〜10 と同じ文書の、文書の一番外側の > [!WARNING] は Alert になる（過検出の検査）。
	t.Run("11 一番外側の引用は Alert", func(t *testing.T) {
		doc := inList + "\n" + inQuote + "\n> [!WARNING]\n> 外側\n\n" + inFootnote
		got := render(t, doc)
		els := scanElements(t, got)

		alerts := withClass(els, "markdown-alert")
		if len(alerts) != 1 {
			t.Fatalf("Alert が %d 個、want 1（一番外側の WARNING だけ）\n出力: %s", len(alerts), got)
		}
		if len(withClass(els, "markdown-alert-warning")) != 1 {
			t.Errorf("一番外側の WARNING が Alert になっていない\n出力: %s", got)
		}
		// 入れ子の 3 つは通常の引用のまま（リスト 1・引用の中 2・脚注 1）。
		if n := len(byTag(els, "blockquote")); n != 4 {
			t.Errorf("<blockquote> が %d 個、want 4\n出力: %s", n, got)
		}
	})
}
