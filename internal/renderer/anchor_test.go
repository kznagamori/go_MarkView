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
// 検証する（UT-202 ケース 7〜9。根拠: MD-021, AR-053 / IMP-117）。
//
// id 属性の値は user-content- + スラッグである（AR-053）。スラッグそのものは
// TestSlug が見る。
func TestRender_HeadingID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string // 出力 HTML に含まれること
	}{
		// UT-202 ケース 8・9: インライン記法を除いてからスラッグにする
		{"強調を除去する", "## **強調**を含む", []string{`<h2 id="user-content-強調を含む">`}},
		{"コードスパンを除去する", "## `code` を含む", []string{`<h2 id="user-content-code-を含む">`}},
		{"リンクを除去する", "## [GitHub](https://github.com) の話", []string{`<h2 id="user-content-github-の話">`}},

		// UT-202 ケース 7: 同一文書内の重複
		{
			name: "同じ見出しが 3 回",
			in:   "# Test\n# Test\n# Test",
			want: []string{`<h1 id="user-content-test">`, `<h1 id="user-content-test-1">`, `<h1 id="user-content-test-2">`},
		},

		// UT-202 ケース 10: アンカーを持たない見出しには id を出さない
		{"空の見出しに id を付けない", "#\n\npara", []string{"<h1></h1>"}},

		// UT-202 ケース 13: 空の見出しが 2 つ。2 つ目にも連番を付けない（IMP-117 の 5・6。4.78.0）
		{"空の見出しが 2 つ", "#\n\n#\n", []string{"<h1></h1>\n<h1></h1>"}},

		// UT-202 ケース 14: 生 HTML のタグを除き、中の文字だけを残す（MD-021 の 1。4.78.0）。
		// 修飾（FR-146, FR-147）を付けてもアンカーが変わらない。
		{"del のタグを除く", "## a<del>b</del>c", []string{`<h2 id="user-content-abc">`}},
		{"色の span のタグを除く", "## <span style=\"color: #d1242f\">赤</span>い", []string{`<h2 id="user-content-赤い">`}},

		// UT-202 ケース 15: 文字参照とエスケープを解いてから記号を除く（MD-021 の 1・3・4。4.78.0）。
		// & を落とした跡の空白は詰めない。
		{"文字参照を解いてから除く", "## A &amp; B", []string{`<h2 id="user-content-a--b">`}},
		{"エスケープを解いてから除く", "## a\\*b", []string{`<h2 id="user-content-ab">`}},

		// UT-202 ケース 16: インラインコードの中の文字参照は解かない（MD-021 の 1。4.84.0。利用者の判断）。
		{"インラインコードの中の文字参照", "## `&amp;`", []string{`<h2 id="user-content-amp">`}},
		{"インラインコードの中の文字参照 2", "## a `&lt;b&gt;` c", []string{`<h2 id="user-content-a-ltbgt-c">`}},
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
				{Level: 1, Text: "A", ID: "user-content-a"},
				{Level: 2, Text: "B", ID: "user-content-b"},
				{Level: 3, Text: "C", ID: "user-content-c"},
			},
		},

		// UT-203 ケース 2: レベルの飛びは補正しない
		{
			name: "レベルが飛んでいても補正しない",
			in:   "# A\n### C",
			want: []Heading{
				{Level: 1, Text: "A", ID: "user-content-a"},
				{Level: 3, Text: "C", ID: "user-content-c"},
			},
		},

		// UT-203 ケース 4: プレーンテキスト化
		{
			name: "強調を含む見出し",
			in:   "## **強調**見出し",
			want: []Heading{{Level: 2, Text: "強調見出し", ID: "user-content-強調見出し"}},
		},

		// UT-203 ケース 6: Setext 形式
		{
			name: "Setext 形式の見出し",
			in:   "A\n===",
			want: []Heading{{Level: 1, Text: "A", ID: "user-content-a"}},
		},

		// UT-090 に従って追加した境界値
		{
			name: "引用の中の見出しも拾う",
			in:   "> # A",
			want: []Heading{{Level: 1, Text: "A", ID: "user-content-a"}},
		},
		{
			// 自動リンクは子を持たないため、ラベルを直接拾う必要がある。
			name: "自動リンクの見出し",
			in:   "## https://example.com",
			want: []Heading{{Level: 2, Text: "https://example.com", ID: "user-content-httpsexamplecom"}},
		},
		{
			// 絵文字ショートコードは goldmark-emoji が専用ノードにするため、
			// アンカーには含まれない。GitHub との差異は MD-002 の目視比較
			// （T2-11 の showcase.md）で確認する。
			name: "絵文字を含む見出し",
			in:   "## :sparkles: A",
			want: []Heading{{Level: 2, Text: " A", ID: "user-content-a"}},
		},
		{
			name: "レベル 6 の見出し",
			in:   "###### F",
			want: []Heading{{Level: 6, Text: "F", ID: "user-content-f"}},
		},
		{
			name: "アンカーを持たない見出しは ID が空になる",
			in:   "# ???",
			want: []Heading{{Level: 1, Text: "???", ID: ""}},
		},
		{
			// UT-203 ケース 8: 生 HTML のタグを除き、文字参照を解く（MD-021 の 1、IMP-117。4.78.0）。
			// アウトラインに &amp; を出さない。
			name: "生 HTML のタグと文字参照を含む見出し",
			in:   "## <del>a</del> &amp; b",
			want: []Heading{{Level: 2, Text: "a & b", ID: "user-content-a--b"}},
		},
		{
			// UT-203 ケース 8 の 2 つ目: エスケープも解く（MD-021 の 1、IMP-117 の 1。4.84.0）。
			// goldmark の文字のノードはエスケープを書かれたとおり（\*）に持つ（実測）。スラッグは記号を
			// 落とすため UT-202 のケース 15 では見えず、見出しの文字（アウトライン）にだけ出る。
			name: "エスケープを含む見出し",
			in:   "## a\\*b",
			want: []Heading{{Level: 2, Text: "a*b", ID: "user-content-ab"}},
		},
		{
			// UT-202 ケース 16: インラインコードの中の文字参照は解かない（MD-021 の 1、IMP-117 の 1。4.84.0）。
			// 本文で文字どおりに表示される文字と同じにする。解くと Text が & になり、アンカーも失う。
			name: "インラインコードの中の文字参照",
			in:   "## `&amp;`\n\n## a `&lt;b&gt;` c",
			want: []Heading{
				{Level: 2, Text: "&amp;", ID: "user-content-amp"},
				{Level: 2, Text: "a &lt;b&gt; c", ID: "user-content-a-ltbgt-c"},
			},
		},
		{
			// UT-203 ケース 9: 数値の文字参照は描画（goldmark の Writer）と同じ規則で解く（10 進は基数 10、
			// 16 進は 6 桁まで。4.86.0。独立したレビューの指摘——`&#065;` を 8 進と読み `5` にし、16 進の
			// 7 桁も解いていた）。スラッグも画面の文字から作る。
			name: "数値の文字参照を含む見出し",
			in:   "## &#065;bc &#x0000041;",
			want: []Heading{{Level: 2, Text: "Abc &#x0000041;", ID: "user-content-abc-x0000041"}},
		},
		{
			// UT-203 ケース 9 の 2 つ目: インラインコードの中の改行は半角空白 1 つ（本文の code の中と同じ。
			// 4.86.0。独立したレビューの指摘）。スラッグは変わらない。
			name: "複数行のインラインコードを含む見出し",
			in:   "`a\nb`\n===",
			want: []Heading{{Level: 1, Text: "a b", ID: "user-content-a-b"}},
		},
		{
			// UT-203 ケース 7: ID は user-content- に続く部分が UT-202 の規則と一致する
			// （接頭辞は AR-053。UT-219）
			name: "重複した見出しの ID に連番が付く",
			in:   "# Test\n# Test",
			want: []Heading{
				{Level: 1, Text: "Test", ID: "user-content-test"},
				{Level: 1, Text: "Test", ID: "user-content-test-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), Options{})
			if err != nil {
				t.Fatalf("Render(%q) がエラーを返した: %v", tt.in, err)
			}
			if !reflect.DeepEqual(res.Headings, tt.want) {
				t.Errorf("Headings = %+v, want %+v", res.Headings, tt.want)
			}
		})
	}
}
