package renderer

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// obj は 1 文字と数える部品（数式・画像・脚注の参照）の文字 U+FFFC（IMP-123。4.77.0。利用者の判断）。
const obj = "￼"

// backlink は脚注の戻りリンクの 2 文字（U+00A0 と ↩（U+21A9））。変換器が前に &#160; を書く。
const backlink = " ↩"

// visible は text:<index> の見える文字を求める。失敗した時点で打ち切る。
func visible(t *testing.T, source string, target TextTarget) *TextBlock {
	t.Helper()

	tb, err := New().Visible([]byte(source), target)
	if err != nil {
		t.Fatalf("Visible(%q, %+v) がエラーを返した: %v", source, target, err)
	}
	if tb == nil {
		t.Fatalf("Visible(%q, %+v) が nil を返した", source, target)
	}
	return tb
}

// text は修飾できるブロックの指定（text:<n>）を作る。
func textN(n int) TextTarget { return TextTarget{Index: n} }

// coveredByAtom は見える文字の位置 i が、いずれかの部品の区間に含まれるかを返す。
func coveredByAtom(tb *TextBlock, i int) bool {
	for _, a := range tb.Atoms {
		if a.Start <= i && i < a.Stop {
			return true
		}
	}
	return false
}

// hasAtom は部品の区間に want があるかを返す。
func hasAtom(tb *TextBlock, want Span) bool {
	for _, a := range tb.Atoms {
		if a == want {
			return true
		}
	}
	return false
}

// inlinesOf は Kind が kind のインラインの要素を返す。
func inlinesOf(tb *TextBlock, kind string) []InlineSpan {
	var got []InlineSpan
	for _, in := range tb.Inlines {
		if in.Kind == kind {
			got = append(got, in)
		}
	}
	return got
}

// TestVisible は見える文字の数え方と、ソースとの対応を検証する
// （UT-220 のケース 1〜19・23〜25。根拠: FR-146, FR-147, AR-031 / IMP-120, IMP-123, IMP-121）。
//
// **画面側（IMP-266）と同じ数え方であることは、ここでは見えない。** 一致は描画スモークが見る
// （UT-817）。ここで見るのは、Go 側が IMP-123 の表どおりに数えることである。
// 期待値は IMP-123 の表から人が書いた（UT-031）。
func TestVisible(t *testing.T) {
	// ケース 1: エスケープと文字参照を解いた文字を、元の区間に対応させる。
	t.Run("1 エスケープと文字参照", func(t *testing.T) {
		const src = `a\*b &amp; c`
		tb := visible(t, src, textN(0))
		if got := string(tb.Visible); got != "a*b & c" {
			t.Fatalf("Visible = %q, want %q", got, "a*b & c")
		}
		want := []Span{{0, 1}, {1, 3}, {3, 4}, {4, 5}, {5, 10}, {10, 11}, {11, 12}}
		if !reflect.DeepEqual(tb.Source, want) {
			t.Errorf("Source = %v, want %v", tb.Source, want)
		}
	})

	// ケース 2: ソフト改行は \n 1 つ。
	t.Run("2 ソフト改行", func(t *testing.T) {
		tb := visible(t, "a\nb", textN(0))
		if got := string(tb.Visible); got != "a\nb" || tb.HardBreaks != 0 {
			t.Errorf("Visible = %q, HardBreaks = %d, want %q, 0", got, tb.HardBreaks, "a\nb")
		}
	})

	// ケース 3: ハード改行（半角空白 2 つ / バックスラッシュ）も \n 1 つで、HardBreaks を数える。
	for _, src := range []string{"a  \nb", "a\\\nb"} {
		t.Run("3 ハード改行 "+src, func(t *testing.T) {
			tb := visible(t, src, textN(0))
			if got := string(tb.Visible); got != "a\nb" || tb.HardBreaks != 1 {
				t.Errorf("Visible = %q, HardBreaks = %d, want %q, 1", got, tb.HardBreaks, "a\nb")
			}
		})
	}

	// ケース 4: インラインコードは中の文字。部品。
	t.Run("4 インラインコード", func(t *testing.T) {
		tb := visible(t, "x `co` y", textN(0))
		if got := string(tb.Visible); got != "x co y" {
			t.Errorf("Visible = %q, want %q", got, "x co y")
		}
		if !hasAtom(tb, Span{2, 4}) {
			t.Errorf("Atoms = %v, want [2, 4) を含む", tb.Atoms)
		}
	})

	// ケース 5: 数式は 1 文字（U+FFFC）。部品。
	t.Run("5 数式", func(t *testing.T) {
		tb := visible(t, "x $a+b$ y", textN(0))
		if got := string(tb.Visible); got != "x "+obj+" y" {
			t.Errorf("Visible = %q, want %q", got, "x "+obj+" y")
		}
		if !hasAtom(tb, Span{2, 3}) {
			t.Errorf("Atoms = %v, want [2, 3) を含む", tb.Atoms)
		}
	})

	// ケース 6: 画像は、生 HTML の <img> も 1 文字。
	for _, src := range []string{"x ![alt](a.png) y", `x <img src="a.png"> y`} {
		t.Run("6 画像 "+src, func(t *testing.T) {
			tb := visible(t, src, textN(0))
			if got := string(tb.Visible); got != "x "+obj+" y" {
				t.Errorf("Visible = %q, want %q", got, "x "+obj+" y")
			}
			if !hasAtom(tb, Span{2, 3}) {
				t.Errorf("Atoms = %v, want [2, 3) を含む", tb.Atoms)
			}
		})
	}

	// ケース 7: 脚注の参照は 1 文字。脚注の段落の戻りリンクは 2 文字で部品、Source は長さ 0。
	t.Run("7 脚注", func(t *testing.T) {
		const src = "本文[^1]\n\n[^1]: 注\n"
		if got := string(visible(t, src, textN(0)).Visible); got != "本文"+obj {
			t.Errorf("text:0 = %q, want %q", got, "本文"+obj)
		}
		tb := visible(t, src, textN(1))
		if got := string(tb.Visible); got != "注"+backlink {
			t.Fatalf("text:1 = %q, want %q", got, "注"+backlink)
		}
		for _, i := range []int{1, 2} {
			if !coveredByAtom(tb, i) {
				t.Errorf("戻りリンクの %d 文字目が部品に入っていない: Atoms = %v", i, tb.Atoms)
			}
			if i < len(tb.Source) && tb.Source[i].Start != tb.Source[i].Stop {
				t.Errorf("戻りリンクの %d 文字目の Source = %v, want 長さ 0（ソースに無い）", i, tb.Source[i])
			}
		}
	})

	// ケース 8: 絵文字ショートコードは絵文字の値（部品）。ZWJ でつないだ絵文字は符号位置で数える。
	t.Run("8 絵文字ショートコード", func(t *testing.T) {
		tb := visible(t, ":sparkles:", textN(0))
		if got := string(tb.Visible); got != "✨" {
			t.Errorf("Visible = %q, want %q", got, "✨")
		}
		if !hasAtom(tb, Span{0, 1}) {
			t.Errorf("Atoms = %v, want [0, 1) を含む", tb.Atoms)
		}
	})
	t.Run("8 ZWJ の絵文字", func(t *testing.T) {
		const family = "\U0001F468‍\U0001F469‍\U0001F467" // 5 符号位置
		tb := visible(t, family, textN(0))
		if len(tb.Visible) != 5 {
			t.Errorf("len(Visible) = %d, want 5（符号位置で数える）", len(tb.Visible))
		}
	})

	// ケース 9: 自動リンクは中の文字。部品。
	for _, tt := range []struct{ src, want string }{
		{"<https://a.b>", "https://a.b"},
		{"www.a.b", "www.a.b"},
	} {
		t.Run("9 自動リンク "+tt.src, func(t *testing.T) {
			tb := visible(t, tt.src, textN(0))
			if got := string(tb.Visible); got != tt.want {
				t.Errorf("Visible = %q, want %q", got, tt.want)
			}
			if !hasAtom(tb, Span{0, len([]rune(tt.want))}) {
				t.Errorf("Atoms = %v, want [0, %d) を含む", tb.Atoms, len([]rune(tt.want)))
			}
		})
	}

	// ケース 10: 生 HTML のタグは 0 文字。中の文字は数える。
	t.Run("10 生HTMLのタグ", func(t *testing.T) {
		if got := string(visible(t, "a<b>c</b>d", textN(0)).Visible); got != "acd" {
			t.Errorf("Visible = %q, want %q", got, "acd")
		}
	})

	// ケース 11: チェックボックスは 0 文字で、後ろの空白も数えない（4.76.0）。
	t.Run("11 タスク", func(t *testing.T) {
		if got := string(visible(t, "- [ ] 買い物", textN(0)).Visible); got != "買い物" {
			t.Errorf("Visible = %q, want %q", got, "買い物")
		}
	})

	// ケース 12: 入れ子の手前で止め、文字の後ろの改行を数えない（4.76.0）。
	t.Run("12 入れ子のリスト", func(t *testing.T) {
		const src = "- 親\n  - 子\n"
		if got := string(visible(t, src, textN(0)).Visible); got != "親" {
			t.Errorf("text:0 = %q, want %q", got, "親")
		}
		if got := string(visible(t, src, textN(1)).Visible); got != "子" {
			t.Errorf("text:1 = %q, want %q", got, "子")
		}
	})

	// ケース 13: Alerts の見出しは修飾できるブロックではない。
	t.Run("13 Alerts", func(t *testing.T) {
		if got := string(visible(t, "> [!NOTE]\n> 本文", textN(0)).Visible); got != "本文" {
			t.Errorf("text:0 = %q, want %q", got, "本文")
		}
	})

	// ケース 14: 表のセルも対象（UT-110 の基本の文書の本体 1 行目 1 列目）。
	t.Run("14 表のセル", func(t *testing.T) {
		const src = "| a | b |\n| --- | --- |\n| 1 | 2 |\n| x |\n"
		tb := visible(t, src, TextTarget{Cell: true, Table: 0, Row: 1, Col: 0})
		if got := string(tb.Visible); got != "1" {
			t.Errorf("Visible = %q, want %q", got, "1")
		}
	})

	// ケース 15: インラインの要素の区間（見える文字の上の Text、ソースの上の Open / Close）。
	t.Run("15 インラインの要素", func(t *testing.T) {
		tb := visible(t, "**a**~~b~~[c](u)`d`", textN(0))
		check := func(kind string, want InlineSpan, checkMarks bool) {
			t.Helper()
			got := inlinesOf(tb, kind)
			if len(got) != 1 {
				t.Fatalf("Kind %q のインラインが %d 個, want 1: %+v", kind, len(got), tb.Inlines)
			}
			if got[0].Text != want.Text {
				t.Errorf("%s の Text = %v, want %v", kind, got[0].Text, want.Text)
			}
			if checkMarks && (got[0].Open != want.Open || got[0].Close != want.Close) {
				t.Errorf("%s の Open / Close = %v / %v, want %v / %v", kind, got[0].Open, got[0].Close, want.Open, want.Close)
			}
		}
		check("strong", InlineSpan{Text: Span{0, 1}, Open: Span{0, 2}, Close: Span{3, 5}}, true)
		check("strike", InlineSpan{Text: Span{1, 2}, Open: Span{5, 7}, Close: Span{8, 10}}, true)
		check("link", InlineSpan{Text: Span{2, 3}}, false)
		check("code", InlineSpan{Text: Span{3, 4}}, false)
	})

	// ケース 16: 生 HTML のタグの対の種類。色は IMP-116 が通すもの（UT-209 のケース 25〜28・35・36）と
	// 同じ判定で読む。
	t.Run("16 生HTMLのタグの対", func(t *testing.T) {
		tests := []struct {
			src   string
			kind  string
			color string
		}{
			{"<del>x</del>", "del", ""},
			{"<s>x</s>", "s", ""},
			{"<strike>x</strike>", "strike-tag", ""},
			{`<span style="color: #d1242f">x</span>`, "color", "red"},
			{`<span style="color:#D1242F">x</span>`, "color", "red"},
			{`<span style="color: #123456">x</span>`, "html", ""},
			{`<span style="color: #d1242f;">x</span>`, "color", "red"},
			{`<span style="color: #d1242f; font-weight: bold">x</span>`, "html", ""},
			{`<span style="color: #d1242f !important">x</span>`, "html", ""},
			{`<span style="color: #d1242f; color: #0969da">x</span>`, "html", ""},
		}
		for _, tt := range tests {
			t.Run(tt.src, func(t *testing.T) {
				tb := visible(t, tt.src, textN(0))
				if len(tb.Inlines) != 1 {
					t.Fatalf("Inlines が %d 個, want 1: %+v", len(tb.Inlines), tb.Inlines)
				}
				if got := tb.Inlines[0]; got.Kind != tt.kind || got.Color != tt.color {
					t.Errorf("Kind / Color = %q / %q, want %q / %q", got.Kind, got.Color, tt.kind, tt.color)
				}
				if tb.Inlines[0].Text != (Span{0, 1}) {
					t.Errorf("Text = %v, want [0, 1)", tb.Inlines[0].Text)
				}
			})
		}
	})

	// ケース 17: 同じブロックの中で開きと閉じがそろったものだけを対にする。
	t.Run("17 そろわないタグ", func(t *testing.T) {
		for _, tt := range []struct {
			src    string
			target int
		}{
			{"a<del>b", 0},
			{"a</del>b", 0},
			{"<del>a\n\nb</del>", 0},
			{"<del>a\n\nb</del>", 1},
		} {
			tb := visible(t, tt.src, textN(tt.target))
			if got := inlinesOf(tb, "del"); len(got) != 0 {
				t.Errorf("%q の text:%d に del がある: %+v", tt.src, tt.target, got)
			}
		}
	})

	// ケース 18: Go 側は中身を数え、生 HTML のタグは 0 文字とする（サニタイズの結果を解き直して
	// 合わせない。画面と食い違い、修飾できないだけで済む。IMP-123）。
	t.Run("18 サニタイズで消えるもの", func(t *testing.T) {
		if got := string(visible(t, "a <script>x</script> b", textN(0)).Visible); got != "a x b" {
			t.Errorf("Visible = %q, want %q", got, "a x b")
		}
		if got := string(visible(t, `<input type="checkbox"> c`, textN(0)).Visible); got != " c" {
			t.Errorf("Visible = %q, want %q", got, " c")
		}
	})

	// ケース 19: 目印と同じく、脚注の変換の後の木で数える（IMP-120, BUG-027）。
	t.Run("19 脚注の定義が本文より前", func(t *testing.T) {
		const src = "[^1]: 脚注の段落\n\n本文の段落[^1]\n"
		if got := string(visible(t, src, textN(0)).Visible); got != "本文の段落"+obj {
			t.Errorf("text:0 = %q, want %q（本文の段落）", got, "本文の段落"+obj)
		}
	})

	// ケース 23: インラインコードの中はエスケープも文字参照も解かず、改行は半角空白 1 つ（4.78.0）。
	t.Run("23 インラインコードの中", func(t *testing.T) {
		if got := string(visible(t, "x `a\\*b&amp;c` y", textN(0)).Visible); got != `x a\*b&amp;c y` {
			t.Errorf("Visible = %q, want %q", got, `x a\*b&amp;c y`)
		}
		if got := string(visible(t, "`a\nb`", textN(0)).Visible); got != "a b" {
			t.Errorf("Visible = %q, want %q", got, "a b")
		}
	})

	// ケース 24: Render・Locate と同じ前処理を使い、ずれを戻す（UT-218 のケース 5・7 と同じ補正）。
	for _, tt := range []struct {
		name string
		src  string
		want Span
	}{
		{"TOML", "+++\na = 1\n+++\n会議", Span{14, 17}},
		{"閉じの無いYAML", "---\n会議", Span{4, 7}},
	} {
		t.Run("24 Front Matter "+tt.name, func(t *testing.T) {
			tb := visible(t, tt.src, textN(0))
			if got := string(tb.Visible); got != "会議" {
				t.Fatalf("Visible = %q, want %q", got, "会議")
			}
			if tb.Source[0] != tt.want {
				t.Errorf("会 の Source = %v, want %v", tb.Source[0], tt.want)
			}
		})
	}

	// ケース 25: UT-217 のケース 30 の 2 つの項目。
	t.Run("25 リストの項目の2つ目のTextBlock", func(t *testing.T) {
		if got := string(visible(t, listSecondTextBlock, textN(0)).Visible); got != "a" {
			t.Errorf("text:0 = %q, want %q（2 つ目の TextBlock の b を数えない）", got, "a")
		}
	})
	t.Run("25 最初の子がコードブロックの項目", func(t *testing.T) {
		if _, err := New().Visible([]byte(listFirstFence), textN(0)); err == nil {
			t.Error("text: の無い項目で Visible がエラーを返さなかった")
		}
	})
}

// TestVisibleTexts は、修飾できるすべてのブロックの見える文字を目印ごとに返すことを検証する
// （UT-220 のケース 20。根拠: FR-146, FR-147, BR-054 / IMP-123）。
func TestVisibleTexts(t *testing.T) {
	const src = "段落\n\n# 見出し\n\n| a | b |\n| --- | --- |\n| 1 |\n"
	got, err := New().VisibleTexts([]byte(src))
	if err != nil {
		t.Fatalf("VisibleTexts がエラーを返した: %v", err)
	}
	// 補われたセル（cell:0:1:1）の目印は無い。
	want := map[string]string{
		"text:0":     "段落",
		"text:1":     "見出し",
		"cell:0:0:0": "a",
		"cell:0:0:1": "b",
		"cell:0:1:0": "1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("VisibleTexts = %v, want %v", got, want)
	}
}

// TestVisible_Errors は異常な指定と入力で落ちないことを検証する
// （UT-220 のケース 21・22。根拠: FR-111 / IMP-022, IMP-123）。
func TestVisible_Errors(t *testing.T) {
	// ケース 21: 存在しない text: の番号はエラー。panic しない。
	t.Run("21 存在しない番号", func(t *testing.T) {
		for _, target := range []TextTarget{textN(1), textN(-1), {Cell: true, Table: 3, Row: 0, Col: 0}} {
			if _, err := New().Visible([]byte("段落"), target); err == nil {
				t.Errorf("Visible(%+v) がエラーを返さなかった", target)
			}
		}
	})

	// ケース 22: UT-213 の異常入力で panic しない。
	for _, tt := range robustInputs() {
		t.Run("22 "+tt.name, func(t *testing.T) {
			withinLimit(t, func() {
				_, _ = New().Visible([]byte(tt.in), textN(0))
				_, _ = New().VisibleTexts([]byte(tt.in))
			})
		})
	}
}

// robustInputs は UT-213 の異常入力（Locate・Visible・図のファイルの変換が共有する）。
func robustInputs() []struct{ name, in string } {
	return []struct{ name, in string }{
		{"空入力", ""},
		{"1000 段のネストしたリスト", nestedList(1000)},
		{"1000 段の入れ子引用", strings.Repeat("> ", 1000) + "x"},
		{"閉じていないリンク", strings.Repeat("[", 10000)},
		{"閉じていない HTML", strings.Repeat("<div>", 5000)},
		{"閉じていないコードフェンス", "```go\nx"},
		{"不正な UTF-8", "a\xffb\xfe\xfec"},
		{"1 MB の 1 行", strings.Repeat("a", 1<<20)},
		{"1000 列の表", wideTable(1000)},
		{"NUL バイトを含む", "a\x00b"},
	}
}

// withinLimit は f を別のゴルーチンで走らせ、renderLimit 以内に終わることを確かめる。
// パニックがここまで漏れれば、テストはその時点で失敗する。
func withinLimit(t *testing.T, f func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(renderLimit()):
		t.Fatalf("%s 以内に終わらなかった", renderLimit())
	}
}
