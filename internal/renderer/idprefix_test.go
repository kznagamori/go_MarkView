package renderer

import (
	"strings"
	"testing"
)

// idPrefix は文書から生まれる id の接頭辞（AR-053）。
const idPrefix = "user-content-"

// idsOf は出力のすべての id 属性の値を文書の順に返す。
func idsOf(els []element) []string {
	var ids []string
	for _, e := range els {
		if v, ok := e.Attr("id"); ok {
			ids = append(ids, v)
		}
	}
	return ids
}

// hasID は出力に id がちょうど want の要素があるかを返す。
func hasID(els []element, want string) bool {
	for _, id := range idsOf(els) {
		if id == want {
			return true
		}
	}
	return false
}

// TestRender_IDPrefix は文書から生まれる id に接頭辞が付くことを検証する
// （UT-219 ケース 1〜6・9〜13。根拠: AR-053, MD-021, MD-050, MD-072 /
// IMP-111, IMP-116, IMP-117）。
//
// 文書の書き手は id を自由に決められる。**画面と同梱資産の id を乗っ取らせない
// 手段は、名前空間を分けることだけである**（[BUG-011]）。
func TestRender_IDPrefix(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string // 出力に現れる id（順不同）
		notWant []string // 出力に現れてはならない id
	}{
		// UT-219 ケース 1
		{"ケース1_見出しTooltip", "## Tooltip", []string{"user-content-tooltip"}, []string{"tooltip"}},
		// UT-219 ケース 2: BUG-011 の本体（plantuml.js が #status を書き換える）
		{"ケース2_見出しStatus", "# Status", []string{"user-content-status"}, []string{"status"}},
		// UT-219 ケース 3: 重複の連番はスラッグで数える
		{
			"ケース3_同じ見出しが3つ", "# Test\n\n# Test\n\n# Test",
			[]string{"user-content-test", "user-content-test-1", "user-content-test-2"},
			[]string{"test", "test-1", "test-2"},
		},
		// UT-219 ケース 5: 生 HTML の id
		{
			"ケース5_生HTMLのid",
			"<div id=\"status\">a</div>\n\n<h2 id=\"tooltip\">b</h2>\n\n<ul><li id=\"overlay\">c</li></ul>",
			[]string{"user-content-status", "user-content-tooltip", "user-content-overlay"},
			[]string{"status", "tooltip", "overlay"},
		},
		// UT-219 ケース 6: 既に接頭辞で始まる生 HTML の id には付けない
		{
			"ケース6_生HTMLの接頭辞付きid", "<div id=\"user-content-x\">x</div>",
			[]string{"user-content-x"}, []string{"user-content-user-content-x"},
		},
		// UT-219 ケース 9: 見出しには必ず付ける（二重に付けないのは生 HTML だけ）
		{
			"ケース9_接頭辞で始まる見出し", "## user-content-foo",
			[]string{"user-content-user-content-foo"}, []string{"user-content-foo"},
		},
		// UT-219 ケース 11: 行の中の生 HTML
		{
			"ケース11_段落の中のsup", "para <sup id=\"status\">x</sup> text",
			[]string{"user-content-status"}, []string{"status"},
		},
		// UT-219 ケース 12: id の数を数えて走査を省く実装を捕まえる。
		// bluemonday は object を中身ごと捨てるため、見出しの id が 1 つ消え、
		// 生 HTML の id 1 つと数が合う（IMP-116）。
		{
			"ケース12_objectの中の見出しと生HTMLのid",
			"<object>\n\n# A\n\n</object>\n\n<div id=\"statusbar\">x</div>",
			[]string{"user-content-statusbar"}, []string{"statusbar"},
		},
		// UT-219 ケース 13: 開始タグだけを組み立て直す実装を捕まえる
		{
			"ケース13_自己終了タグの生HTML", "<div id=\"status\"/>",
			[]string{"user-content-status"}, []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, els := renderWithKey(t, tt.in, "")

			for _, want := range tt.want {
				if !hasID(els, want) {
					t.Errorf("id %q が無い（出力の id: %q）\n出力: %s", want, idsOf(els), res.HTML)
				}
			}
			for _, ng := range tt.notWant {
				if hasID(els, ng) {
					t.Errorf("接頭辞の無い id %q が残っている（出力の id: %q）\n出力: %s", ng, idsOf(els), res.HTML)
				}
			}
		})
	}
}

// TestRender_IDPrefix_HeadingID は、Heading.ID が id 属性と同じ接頭辞付きの値に
// なることを検証する（UT-219 ケース 1・10。根拠: AR-053 / IMP-117, IMP-224）。
//
// アウトラインは Heading.ID で本文の見出しを探す。id 属性とずれると、
// アウトラインから移動できない。
func TestRender_IDPrefix_HeadingID(t *testing.T) {
	t.Run("ケース1_Heading.IDもid属性と同じ", func(t *testing.T) {
		res, els := renderWithKey(t, "## Tooltip", "")

		if len(res.Headings) != 1 || res.Headings[0].ID != "user-content-tooltip" {
			t.Errorf("Headings = %+v, want ID %q", res.Headings, "user-content-tooltip")
		}
		h2 := byTag(els, "h2")
		wantCount(t, "h2", h2, 1, res.HTML)
		wantAttr(t, "見出し", h2[0], "id", "user-content-tooltip", res.HTML)
	})

	t.Run("ケース10_空の見出しにidを出さない", func(t *testing.T) {
		res, els := renderWithKey(t, "#\n\n#\n", "")

		h1 := byTag(els, "h1")
		wantCount(t, "h1", h1, 2, res.HTML)
		for _, h := range h1 {
			// user-content- だけの id を出さない。空の見出しが並んでも同じ id が並ばない。
			wantNoAttr(t, "空の見出し", h, "id", res.HTML)
		}
		if len(res.Headings) != 2 {
			t.Fatalf("Headings が %d 件, want 2 件: %+v", len(res.Headings), res.Headings)
		}
		for i, h := range res.Headings {
			if h.ID != "" {
				t.Errorf("Headings[%d].ID = %q, want 空", i, h.ID)
			}
		}
	})
}

// TestRender_IDPrefix_Footnotes は脚注の id と相互リンクに接頭辞が付くことを
// 検証する（UT-219 ケース 4。根拠: MD-050, AR-053 / IMP-111）。
func TestRender_IDPrefix_Footnotes(t *testing.T) {
	res, els := renderWithKey(t, "text[^1]\n\n[^1]: note", "")

	ids := idsOf(els)
	if len(ids) < 2 {
		t.Fatalf("脚注の参照と定義の id が %d 個しかない: %q\n出力: %s", len(ids), ids, res.HTML)
	}
	for _, id := range ids {
		if !strings.HasPrefix(id, idPrefix) {
			t.Errorf("脚注の id %q が %q で始まらない\n出力: %s", id, idPrefix, res.HTML)
		}
	}

	hrefs := 0
	for _, a := range byTag(els, "a") {
		href, _ := a.Attr("href")
		if !strings.HasPrefix(href, "#") {
			continue
		}
		hrefs++
		if !strings.HasPrefix(href, "#"+idPrefix) {
			t.Errorf("脚注のリンク %q が %q で始まらない\n出力: %s", href, "#"+idPrefix, res.HTML)
		}
	}
	if hrefs < 2 {
		t.Errorf("脚注の相互リンクが %d 本しかない\n出力: %s", hrefs, res.HTML)
	}
}

// TestRender_IDPrefix_AllIDs は、見出し・脚注・生 HTML を混ぜた文書で、
// 出力のすべての id が接頭辞で始まることを検証する（UT-219 ケース 7。根拠: AR-053）。
//
// **このケースが要である。** 経路ごとのケースは、接頭辞を付け忘れた経路が
// 1 つ残っても他は通る。画面の要素を乗っ取る id は 1 つあれば足りる。
func TestRender_IDPrefix_AllIDs(t *testing.T) {
	const src = "# Status\n\n" +
		"## Tooltip\n\n" +
		"本文[^1] と <sup id=\"inline\">行の中</sup>\n\n" +
		"<div id=\"overlay\">\n<div id=\"statusbar\">raw</div>\n</div>\n\n" +
		"<ul><li id=\"tree\">li</li></ul>\n\n" +
		"[^1]: note"
	res, els := renderWithKey(t, src, "")

	ids := idsOf(els)
	// 見出し 2・脚注の参照と定義 2・生 HTML の id 4（sup / div / div / li）。
	if len(ids) < 8 {
		t.Fatalf("id が %d 個しかない（検証用の文書が効いていない）: %q\n出力: %s", len(ids), ids, res.HTML)
	}
	for _, id := range ids {
		if !strings.HasPrefix(id, idPrefix) {
			t.Errorf("id %q が %q で始まらない\n出力: %s", id, idPrefix, res.HTML)
		}
	}
}

// TestRender_IDPrefix_SlugUnchanged は、接頭辞の後ろのスラッグが UT-202 の規則の
// ままであることを検証する（UT-219 ケース 8。根拠: MD-021 / IMP-117）。
//
// **逆向きの確認である。** スラッグの関数の中で接頭辞を付けると UT-202 が落ち、
// 2 か所で付けると user-content-user-content- が生まれる（IMP-117 の IMPORTANT）。
func TestRender_IDPrefix_SlugUnchanged(t *testing.T) {
	tests := []struct {
		heading string
		slug    string // UT-202 の期待値
	}{
		{"Hello World", "hello-world"},
		{"What's New?", "whats-new"},
		{"概要", "概要"},
		{"a   b", "a---b"},
		{"1. はじめに", "1-はじめに"},
		{"foo_bar-baz", "foo_bar-baz"},
		{"**強調**を含む", "強調を含む"},
		{"`code` を含む", "code-を含む"},
		{"正引き（要求 → 実装・表示）", "正引き要求--実装表示"},
		{"a ! b", "a--b"},
		{"概要　詳細", "概要-詳細"},
	}

	for _, tt := range tests {
		t.Run(tt.heading, func(t *testing.T) {
			res, els := renderWithKey(t, "## "+tt.heading, "")

			want := idPrefix + tt.slug
			h2 := byTag(els, "h2")
			wantCount(t, "h2", h2, 1, res.HTML)
			wantAttr(t, "見出し", h2[0], "id", want, res.HTML)

			if len(res.Headings) != 1 || res.Headings[0].ID != want {
				t.Errorf("Headings = %+v, want ID %q", res.Headings, want)
			}
		})
	}
}
