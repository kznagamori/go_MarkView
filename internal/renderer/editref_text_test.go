package renderer

import (
	"reflect"
	"strings"
	"testing"
)

// renderOpts は Options を渡して変換し、出力の要素を返す。
func renderOpts(t *testing.T, source string, opts Options) (Result, []element) {
	t.Helper()

	res, err := New().Render([]byte(source), opts)
	if err != nil {
		t.Fatalf("Render(%q) がエラーを返した: %v", source, err)
	}
	return res, scanElements(t, res.HTML)
}

// refsOf は data-ref を持つ要素の、鍵を除いた値を文書の順に返す。
// kinds を渡すと、その種類（"text" / "code" ほか）の目印だけを返す。
func refsOf(els []element, kinds ...string) []string {
	var got []string
	for _, e := range els {
		v, ok := e.Attr("data-ref")
		if !ok {
			continue
		}
		rest := strings.TrimPrefix(v, testRefKey+":")
		if len(kinds) == 0 {
			got = append(got, rest)
			continue
		}
		kind, _, _ := strings.Cut(rest, ":")
		for _, k := range kinds {
			if kind == k {
				got = append(got, rest)
				break
			}
		}
	}
	return got
}

// textRefElements は text: の目印を持つ要素を文書の順に返す。
func textRefElements(els []element) []element {
	var got []element
	for _, e := range els {
		if v, ok := e.Attr("data-ref"); ok && strings.HasPrefix(v, testRefKey+":text:") {
			got = append(got, e)
		}
	}
	return got
}

// textRefDoc は UT-217 のケース 24〜26 の文書。
//
// 段落・ATX と設定見出し・詰まったリストの項目・広いリストの項目の段落・引用と Alerts の本文の
// 段落・本文から参照する脚注の段落を含み、あわせてタスク・補われたセルを含む GFM の表・
// フェンス付きのコードブロック・```math・Mermaid と PlantUML のブロック・生 HTML のブロックを含む
// （4.78.0 で明記した——ケース 26 は比べる目印が無いと何も確かめない）。
//
// 広いリストは詰まったリストと記号を変え、間に段落を置いて別のリストにする。
const textRefDoc = "# 見出し\n\n" +
	"段落1\n\n" +
	"設定見出し\n---\n\n" +
	"- 詰まった項目1\n- 詰まった項目2\n\n" +
	"区切り\n\n" +
	"* 広い項目1\n\n* 広い項目2\n\n" +
	"> 引用の段落\n\n" +
	"> [!NOTE]\n> Alert の本文\n\n" +
	"本文[^1]\n\n" +
	"- [ ] タスク\n\n" +
	"| a | b |\n| --- | --- |\n| 1 |\n\n" +
	"```go\nx\n```\n\n" +
	"```math\na+b\n```\n\n" +
	"```mermaid\ngraph TD\n```\n\n" +
	"```plantuml\n@startuml\nA -> B\n@enduml\n```\n\n" +
	"<div>生 HTML のブロック</div>\n\n" +
	"[^1]: 脚注の段落\n"

// TestRender_RefMarkers_V120 は v1.2.0 の目印（code: / text: と、すべての目印を描画される順に
// 振ること）を検証する（UT-217 のケース 22〜30。根拠: FR-145, FR-146, UI-053, FR-141, FR-142 /
// IMP-115, IMP-119, IMP-120, IMP-121。BUG-027）。
func TestRender_RefMarkers_V120(t *testing.T) {
	// ケース 22: フェンス付きのコードブロックは図と別に数える。数式と字下げ形式には付かず、
	// 番号も飛ばない（FR-145）。
	t.Run("ケース22_コードブロックの番号", func(t *testing.T) {
		const src = "```go\nx := 1\n```\n\n" +
			"```mermaid\ngraph TD\n```\n\n" +
			"```math\na+b\n```\n\n" +
			"para\n\n    indented\n\n" +
			"```py\nprint(1)\n```\n\n" +
			"~~~\nplain\n~~~\n"
		res, els := renderWithKey(t, src, testRefKey)

		blocks := withClass(byTag(els, "div"), "code-block")
		// go・mermaid・字下げ形式・py・~~~ の 5 つ（数式は math-block）。
		wantCount(t, "div.code-block", blocks, 5, res.HTML)
		for i, want := range []string{"code:0", "mermaid:0", "", "code:1", "code:2"} {
			if want == "" {
				wantNoAttr(t, "字下げ形式のコードブロック", blocks[i], "data-ref", res.HTML)
				continue
			}
			wantAttr(t, "ブロック "+want, blocks[i], "data-ref", ref(want), res.HTML)
		}
	})

	// ケース 23: 前置きが行ごとに違うブロックと、タブの途中から始まる行を持つブロック
	// （UT-118 のケース 20 の 2 つ）と、前置きが行ごとに違う Mermaid のブロックに data-locked="1"。
	// 前置きのそろったブロックには付かない（UI-053 の淡色の編集ボタン。IMP-115）。
	t.Run("ケース23_data-locked", func(t *testing.T) {
		tests := []struct {
			name   string
			in     string
			locked bool
		}{
			{"前置きが行ごとに違う", "> ```\n> a\n>b\n> ```\n", true},
			{"タブの途中から始まる行", "1. ```\n\tz\n   ```\n", true},
			{"前置きが行ごとに違うMermaid", "> ```mermaid\n> graph TD\n>A-->B\n> ```\n", true},
			{"前置きのそろった引用の中", "> ```\n> a\n> ```\n", false},
			{"最上位", "```\na\n```\n", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res, els := renderWithKey(t, tt.in, testRefKey)
				blocks := withClass(byTag(els, "div"), "code-block")
				wantCount(t, "div.code-block", blocks, 1, res.HTML)
				if tt.locked {
					wantAttr(t, "ブロック", blocks[0], "data-locked", "1", res.HTML)
				} else {
					wantNoAttr(t, "ブロック", blocks[0], "data-locked", res.HTML)
				}
			})
		}
	})

	// ケース 24: TextRefs が真なら、p / h1〜h6 / li に text:<n> が描画される順に付く
	// （詰まったリストの項目は li に付く。IMP-120）。
	t.Run("ケース24_textの目印", func(t *testing.T) {
		res, els := renderOpts(t, textRefDoc, Options{RefKey: testRefKey, TextRefs: true})

		want := []struct {
			tag  string
			text string // 要素の文字に含まれるもの
		}{
			{"h1", "見出し"},
			{"p", "段落1"},
			{"h2", "設定見出し"},
			{"li", "詰まった項目1"},
			{"li", "詰まった項目2"},
			{"p", "区切り"},
			{"p", "広い項目1"},
			{"p", "広い項目2"},
			{"p", "引用の段落"},
			{"p", "Alert の本文"},
			{"p", "本文"},
			{"li", "タスク"},
			{"p", "脚注の段落"}, // 脚注は末尾（脚注の変換の後の木の順）
		}
		got := textRefElements(els)
		wantCount(t, "text: の目印を持つ要素", got, len(want), res.HTML)
		for i, w := range want {
			n := "text:" + itoa(i)
			if got[i].Tag != w.tag || !strings.Contains(got[i].Text, w.text) {
				t.Errorf("%s が <%s> %q に付いた, want <%s> %q を含む\n出力: %s", n, got[i].Tag, got[i].Text, w.tag, w.text, res.HTML)
			}
			wantAttr(t, n, got[i], "data-ref", ref(n), res.HTML)
		}
	})

	// ケース 25: Alerts の見出し・表のセル・コードブロック・数式のブロック・生 HTML のブロックには
	// text: が無い（FR-146 の対象外。セルは cell: の目印を使う）。
	t.Run("ケース25_textの目印が無いもの", func(t *testing.T) {
		res, els := renderOpts(t, textRefDoc, Options{RefKey: testRefKey, TextRefs: true})

		check := func(what string, targets []element) {
			t.Helper()
			if len(targets) == 0 {
				t.Fatalf("%s が出力に無い（検査の前提が崩れている）\n出力: %s", what, res.HTML)
			}
			for _, e := range targets {
				if v, ok := e.Attr("data-ref"); ok && strings.Contains(v, ":text:") {
					t.Errorf("%s に %s が付いた\n出力: %s", what, v, res.HTML)
				}
			}
		}
		check("Alerts の見出し", withClass(els, "markdown-alert-title"))
		check("表のセル", byTag(els, "th", "td"))
		check("コードブロック", withClass(byTag(els, "div"), "code-block"))
		check("数式のブロック", withClass(els, "math-block"))
		var raw []element
		for _, d := range byTag(els, "div") {
			if strings.Contains(d.Text, "生 HTML のブロック") {
				raw = append(raw, d)
			}
		}
		check("生 HTML のブロック", raw)
	})

	// ケース 26: TextRefs が偽なら text: は 1 つも無く、ほかの目印は 24 と同じ
	// （修飾の目印は編集モードの間だけ。利用者の判断。IMP-120）。
	t.Run("ケース26_TextRefsが偽", func(t *testing.T) {
		_, on := renderOpts(t, textRefDoc, Options{RefKey: testRefKey, TextRefs: true})
		res, off := renderOpts(t, textRefDoc, Options{RefKey: testRefKey})

		if got := refsOf(off, "text"); len(got) != 0 {
			t.Errorf("TextRefs が偽なのに text: の目印がある: %v\n出力: %s", got, res.HTML)
		}
		others := []string{"task", "table", "cell", "code", "mermaid", "plantuml"}
		wantOthers := refsOf(on, others...)
		if len(wantOthers) == 0 {
			t.Fatalf("比べる目印が無い（検査の前提が崩れている）\n出力: %s", res.HTML)
		}
		if got := refsOf(off, others...); !reflect.DeepEqual(got, wantOthers) {
			t.Errorf("text: 以外の目印 = %v, want %v（TextRefs が真のとき）", got, wantOthers)
		}
	})

	// ケース 27: UT-109 のケース 13 の形 1〜3。task:<n> が描画される順に付く（BUG-027）。
	t.Run("ケース27_脚注の中のタスク", func(t *testing.T) {
		for _, f := range footnoteTaskForms {
			t.Run(f.name, func(t *testing.T) {
				res, els := renderWithKey(t, f.src, testRefKey)
				got := taskLabels(els)
				if !reflect.DeepEqual(got, f.order) {
					t.Errorf("描画される順のタスク = %q, want %q\n出力: %s", got, f.order, res.HTML)
				}
				for i, in := range byTag(els, "input") {
					wantAttr(t, "タスク "+itoa(i), in, "data-ref", ref("task:"+itoa(i)), res.HTML)
				}
			})
		}
	})

	// ケース 28: 本文から参照する脚注の定義の中の Mermaid のブロック・コードブロック・段落が、
	// 本文のものより前に書いてある文書。すべての目印を同じ木で数える（図の番号も walkRefs が振る）。
	t.Run("ケース28_脚注の中の図とコードと段落", func(t *testing.T) {
		res, els := renderOpts(t, footnoteBlocksDoc, Options{RefKey: testRefKey, TextRefs: true})

		// 描画される順は、本文の段落・Mermaid・コードブロック、その後に脚注の中のもの。
		if got, want := refsOf(withClass(byTag(els, "div"), "code-block")), []string{"mermaid:0", "code:0", "mermaid:1", "code:1"}; !reflect.DeepEqual(got, want) {
			t.Errorf("図とコードブロックの目印 = %v, want %v\n出力: %s", got, want, res.HTML)
		}
		texts := textRefElements(els)
		wantCount(t, "text: の目印を持つ要素", texts, 2, res.HTML)
		if !strings.Contains(texts[0].Text, "本文の段落") || !strings.Contains(texts[1].Text, "脚注の段落") {
			t.Errorf("text:0 / text:1 = %q / %q, want 本文の段落 / 脚注の段落\n出力: %s", texts[0].Text, texts[1].Text, res.HTML)
		}
		// 本文の Mermaid が mermaid:0 であること（data-source で特定する）。
		for _, b := range withClass(byTag(els, "div"), "code-block") {
			if src, _ := b.Attr("data-source"); src == "graph LR" {
				wantAttr(t, "本文の Mermaid", b, "data-ref", ref("mermaid:0"), res.HTML)
			}
		}
	})

	// ケース 29: 鍵が空文字で TextRefs が真なら、code: も text: も出ない（ケース 4 と同じ）。
	t.Run("ケース29_鍵が空", func(t *testing.T) {
		res, els := renderOpts(t, textRefDoc, Options{TextRefs: true})
		for _, e := range els {
			wantNoAttr(t, "<"+e.Tag+">", e, "data-ref", res.HTML)
		}
	})

	// ケース 30: ListItem の最初の子が TextBlock のときだけ数え、2 つ目以降は数えない
	// （li には属性を 1 つしか書けない。4.78.0。IMP-120）。
	t.Run("ケース30_リストの項目の2つ目のTextBlock", func(t *testing.T) {
		res, els := renderOpts(t, listSecondTextBlock, Options{RefKey: testRefKey, TextRefs: true})
		texts := textRefElements(els)
		wantCount(t, "text: の目印を持つ要素", texts, 1, res.HTML)
		if texts[0].Tag != "li" {
			t.Errorf("text:0 が <%s> に付いた, want <li>\n出力: %s", texts[0].Tag, res.HTML)
		}
		wantAttr(t, "a の項目", texts[0], "data-ref", ref("text:0"), res.HTML)
	})
	t.Run("ケース30_最初の子がコードブロックの項目", func(t *testing.T) {
		res, els := renderOpts(t, listFirstFence, Options{RefKey: testRefKey, TextRefs: true})
		if got := refsOf(els, "text"); len(got) != 0 {
			t.Errorf("最初の子がコードブロックの項目に text: が付いた: %v\n出力: %s", got, res.HTML)
		}
	})
}

// listSecondTextBlock と listFirstFence は UT-217 のケース 30 の 2 つの項目
// （UT-220 のケース 25、UT-222 のケース 18 も使う）。
const (
	listSecondTextBlock = "- a\n  ```\n  x\n  ```\n  b\n"
	listFirstFence      = "- ```\n  x\n  ```\n  z\n"
)

// footnoteBlocksDoc は UT-217 のケース 28 の文書。脚注の定義が本文より前にある。
const footnoteBlocksDoc = "[^1]: 脚注の段落\n\n" +
	"    ```mermaid\n    graph TD\n    ```\n\n" +
	"    ```go\n    y\n    ```\n\n" +
	"本文の段落[^1]\n\n" +
	"```mermaid\ngraph LR\n```\n\n" +
	"```go\nx\n```\n"

// footnoteTaskForms は UT-109 のケース 13 の 3 つの形（BUG-027 の 1.1）。
// 脚注の中の字下げは半角空白 4 つ。order は描画される順のタスクの文字。
var footnoteTaskForms = []struct {
	name  string
	src   string
	order []string
}{
	{
		// 形 1: 脚注の定義が本文のタスクより前。
		name:  "形1_定義が本文より前",
		src:   "本文[^1]\n\n[^1]: 脚注:\n\n    - [ ] 脚注のタスク\n\n- [ ] 本文のタスク\n",
		order: []string{"本文のタスク", "脚注のタスク"},
	},
	{
		// 形 2: 参照されていない定義が本文のタスクより前（goldmark は描かない）。
		name:  "形2_参照されていない定義",
		src:   "[^z]: 使っていない脚注:\n\n    - [ ] 見えないタスク\n\n- [ ] 本文のタスク A\n- [ ] 本文のタスク B\n",
		order: []string{"本文のタスク A", "本文のタスク B"},
	},
	{
		// 形 3: 定義の順と参照の順が違う（脚注は参照の順に並ぶ）。
		name:  "形3_定義と参照の順が違う",
		src:   "本文[^a]と[^b]\n\n[^b]: B:\n\n    - [ ] 脚注 b のタスク\n\n[^a]: A:\n\n    - [ ] 脚注 a のタスク\n",
		order: []string{"脚注 a のタスク", "脚注 b のタスク"},
	},
}

// taskLabels は、出力のチェックボックスごとに、それを含む li の文字（前後の空白を除く）を
// 描画される順に返す。
func taskLabels(els []element) []string {
	var (
		labels []string
		li     element
	)
	for _, e := range els {
		switch e.Tag {
		case "li":
			li = e
		case "input":
			labels = append(labels, strings.TrimSpace(li.Text))
		}
	}
	return labels
}

// itoa は小さい整数を文字列にする（strconv を読むまでもない添字のため）。
func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + itoa(i%10)
}
