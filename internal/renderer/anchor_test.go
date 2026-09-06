package renderer

import (
	"reflect"
	"strings"
	"testing"
)

// TestSlug は GitHub 互換のスラッグ生成規則を検証する
// （UT-202。根拠: MD-021 / IMP-117）。
//
// インライン記法を含む見出し（UT-202 ケース 8・9）は AST が要るため、
// TestRender_HeadingID が担当する。
func TestSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		// UT-202 ケース 10: 何も残らない見出し（境界値を先に置く。UT-013）
		{"空文字", "", ""},
		{"空白のみ", "   ", ""},
		{"記号のみ", "???", ""},

		// UT-202 ケース 1・2: 小文字化と記号の除去
		{"英語の見出し", "Hello World", "hello-world"},
		{"アポストロフィと疑問符を除去する", "What's New?", "whats-new"},

		// UT-202 ケース 3: 非 ASCII は保持する
		{"日本語の見出し", "概要", "概要"},

		// UT-202 ケース 4: 空白 1 つにつきハイフン 1 つ。**まとめない**（MD-021 の 3）
		{"空白 1 つにつきハイフン 1 つ", "a b", "a-b"},
		{"連続する空白はまとめない", "a   b", "a---b"},

		// UT-202 ケース 10・11: 全角の記号は除去し、その跡の空白は詰めない。
		// **本仕様書 90 章のアンカーがこれに当たる。** 2 つの規則が両方
		// そろって初めて #903-正引き要求--実装表示 が解決する。
		{"全角の括弧・矢印・中黒を除去する", "正引き（要求 → 実装・表示）", "正引き要求--実装表示"},
		{"記号を落とした跡の空白は詰めない", "a ! b", "a--b"},
		{"全角の読点・句点を除去する", "概要、詳細。", "概要詳細"},

		// UT-202 ケース 5: 数字と記号と日本語の組み合わせ
		{"番号付きの日本語見出し", "1. はじめに", "1-はじめに"},

		// UT-202 ケース 6: - と _ は残す
		{"アンダースコアとハイフンは保持する", "foo_bar-baz", "foo_bar-baz"},

		// UT-090 に従って追加した境界値
		{"大文字を小文字にする", "ABC", "abc"},
		{"前後の空白ではハイフンを作らない", "  a  ", "a"},
		{"タブも空白として扱う", "a\tb", "a-b"},
		{"全角空白も空白として扱う", "a\u3000b", "a-b"},
		{"日本語と英語の混在", "日本語 と English", "日本語-と-english"},
		{"末尾の記号を除去する", "Note:", "note"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newSlugger().Slug(tt.in); got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestSlugger_Duplicates は重複した見出しへの連番付与を検証する
// （UT-202 ケース 7。根拠: MD-021 / IMP-117）。
func TestSlugger_Duplicates(t *testing.T) {
	s := newSlugger()

	for i, want := range []string{"test", "test-1", "test-2", "test-3"} {
		if got := s.Slug("Test"); got != want {
			t.Errorf("%d 回目の Slug(\"Test\") = %q, want %q", i+1, got, want)
		}
	}

	// 連番は文書ごとに数え直す。slugger は変換 1 回ごとに作られる。
	if got := newSlugger().Slug("Test"); got != "test" {
		t.Errorf("別の slugger での Slug(\"Test\") = %q, want %q", got, "test")
	}
}

// TestSlugger_EmptyIsNotNumbered は、アンカーを持たない見出しが連番を
// 消費しないことを検証する（UT-202 ケース 10 の固定した扱い）。
//
// 空文字に連番を振ると id="-1" のような、利用者が辿れないアンカーが残る。
func TestSlugger_EmptyIsNotNumbered(t *testing.T) {
	s := newSlugger()

	for i := range 3 {
		if got := s.Slug(""); got != "" {
			t.Errorf("%d 回目の Slug(\"\") = %q, want %q", i+1, got, "")
		}
	}
}

// TestRender_HeadingID は、見出しに ID が付くこととインライン記法の除去を
// 検証する（UT-202 ケース 7〜9。根拠: MD-021 / IMP-117）。
func TestRender_HeadingID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string // 出力 HTML に含まれること
	}{
		// UT-202 ケース 8・9: インライン記法を除いてからスラッグにする
		{"強調を除去する", "## **強調**を含む", []string{`<h2 id="強調を含む">`}},
		{"コードスパンを除去する", "## `code` を含む", []string{`<h2 id="code-を含む">`}},
		{"リンクを除去する", "## [GitHub](https://github.com) の話", []string{`<h2 id="github-の話">`}},

		// UT-202 ケース 7: 同一文書内の重複
		{
			name: "同じ見出しが 3 回",
			in:   "# Test\n# Test\n# Test",
			want: []string{`<h1 id="test">`, `<h1 id="test-1">`, `<h1 id="test-2">`},
		},

		// UT-202 ケース 10: アンカーを持たない見出しには id を出さない
		{"空の見出しに id を付けない", "#\n\npara", []string{"<h1></h1>"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)

			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
		})
	}
}

// TestRender_Headings は見出しの抽出を検証する
// （UT-203。根拠: FR-040 / IMP-110, IMP-117）。
func TestRender_Headings(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Heading
	}{
		// UT-203 ケース 5: 見出しがない場合は nil ではなく空スライス
		{
			name: "見出しのない文書",
			in:   "para\n\npara2",
			want: []Heading{},
		},

		// UT-203 ケース 3: コードブロック内は見出しではない
		{
			name: "コードブロック内の # は見出しではない",
			in:   "```\n# not heading\n```",
			want: []Heading{},
		},

		// UT-203 ケース 1: レベルと順序
		{
			name: "3 レベルの見出し",
			in:   "# A\n## B\n### C",
			want: []Heading{
				{Level: 1, Text: "A", ID: "a"},
				{Level: 2, Text: "B", ID: "b"},
				{Level: 3, Text: "C", ID: "c"},
			},
		},

		// UT-203 ケース 2: レベルの飛びは補正しない
		{
			name: "レベルが飛んでいても補正しない",
			in:   "# A\n### C",
			want: []Heading{
				{Level: 1, Text: "A", ID: "a"},
				{Level: 3, Text: "C", ID: "c"},
			},
		},

		// UT-203 ケース 4: プレーンテキスト化
		{
			name: "強調を含む見出し",
			in:   "## **強調**見出し",
			want: []Heading{{Level: 2, Text: "強調見出し", ID: "強調見出し"}},
		},

		// UT-203 ケース 6: Setext 形式
		{
			name: "Setext 形式の見出し",
			in:   "A\n===",
			want: []Heading{{Level: 1, Text: "A", ID: "a"}},
		},

		// UT-090 に従って追加した境界値
		{
			name: "引用の中の見出しも拾う",
			in:   "> # A",
			want: []Heading{{Level: 1, Text: "A", ID: "a"}},
		},
		{
			// 自動リンクは子を持たないため、ラベルを直接拾う必要がある。
			name: "自動リンクの見出し",
			in:   "## https://example.com",
			want: []Heading{{Level: 2, Text: "https://example.com", ID: "httpsexamplecom"}},
		},
		{
			// 絵文字ショートコードは goldmark-emoji が専用ノードにするため、
			// アンカーには含まれない。GitHub との差異は MD-002 の目視比較
			// （T2-11 の showcase.md）で確認する。
			name: "絵文字を含む見出し",
			in:   "## :sparkles: A",
			want: []Heading{{Level: 2, Text: " A", ID: "a"}},
		},
		{
			name: "レベル 6 の見出し",
			in:   "###### F",
			want: []Heading{{Level: 6, Text: "F", ID: "f"}},
		},
		{
			name: "アンカーを持たない見出しは ID が空になる",
			in:   "# ???",
			want: []Heading{{Level: 1, Text: "???", ID: ""}},
		},
		{
			// UT-203 ケース 7: ID は UT-202 の規則と一致する
			name: "重複した見出しの ID に連番が付く",
			in:   "# Test\n# Test",
			want: []Heading{
				{Level: 1, Text: "Test", ID: "test"},
				{Level: 1, Text: "Test", ID: "test-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "")
			if err != nil {
				t.Fatalf("Render(%q) がエラーを返した: %v", tt.in, err)
			}
			if !reflect.DeepEqual(res.Headings, tt.want) {
				t.Errorf("Headings = %+v, want %+v", res.Headings, tt.want)
			}
		})
	}
}
