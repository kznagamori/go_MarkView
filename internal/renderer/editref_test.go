package renderer

import (
	"strconv"
	"strings"
	"testing"
)

// testRefKey は目印の鍵の固定値（UT-217, UT-218）。
//
// アプリケーションは変換のたびに乱数の鍵を渡す（IMP-102）。テストは固定の
// 鍵を渡し、出力を人が書いたリテラルと比べる（IMP-110, UT-031）。
const testRefKey = "0123456789abcdef"

// renderWithKey は鍵を渡して変換し、出力の要素を返す。
func renderWithKey(t *testing.T, source, key string) (Result, []element) {
	t.Helper()

	res, err := New().Render([]byte(source), "", key)
	if err != nil {
		t.Fatalf("Render(%q) がエラーを返した: %v", source, err)
	}
	return res, scanElements(t, res.HTML)
}

// ref は鍵を付けた目印の値を組み立てる（期待値の表記を短くするためだけに使う）。
func ref(rest string) string { return testRefKey + ":" + rest }

// wantAttr は要素 e の属性 name が want であることを確かめる。
func wantAttr(t *testing.T, what string, e element, name, want, html string) {
	t.Helper()

	got, ok := e.Attr(name)
	if !ok {
		t.Errorf("%s に %s が無い。want %q\n出力: %s", what, name, want, html)
		return
	}
	if got != want {
		t.Errorf("%s の %s = %q, want %q\n出力: %s", what, name, got, want, html)
	}
}

// wantNoAttr は要素 e に属性 name が無いことを確かめる。
func wantNoAttr(t *testing.T, what string, e element, name, html string) {
	t.Helper()

	if got, ok := e.Attr(name); ok {
		t.Errorf("%s に %s=%q が付いている（付けてはならない）\n出力: %s", what, name, got, html)
	}
}

// wantCount は要素の数を確かめる。数が違えば以降の添字が意味を失うため打ち切る。
func wantCount(t *testing.T, what string, els []element, want int, html string) {
	t.Helper()

	if len(els) != want {
		t.Fatalf("%s が %d 個, want %d 個\n出力: %s", what, len(els), want, html)
	}
}

// TestRender_RefMarkers_Absent は、目印を付けてはならない要素に付けないことを
// 検証する（UT-217 ケース 1〜4・17〜19。根拠: FR-130, FR-141, FR-142, MD-084,
// NFR-030 / IMP-115, IMP-119, IMP-120）。
//
// **付かないことを確かめるケースを先に書く**（UT-013）。「すべてに目印を付ける」
// 実装は、付くことを見るケース（TestRender_RefMarkers_Present）をすべて通る。
func TestRender_RefMarkers_Absent(t *testing.T) {
	t.Run("ケース1_補われたセルに data-ref が無い", func(t *testing.T) {
		const src = "| a | b |\n| --- | --- |\n| 1 |"
		res, els := renderWithKey(t, src, testRefKey)

		tables := tableRows(els)
		if len(tables) != 1 || len(tables[0]) != 2 {
			t.Fatalf("表 1 つ・2 行の出力を期待した: %v\n出力: %s", tables, res.HTML)
		}
		body := tables[0][1]
		wantCount(t, "本体の行のセル", body, 2, res.HTML)
		// 1 列目はソースにある。2 列目は goldmark が補ったセル（IMP-120）。
		wantNoAttr(t, "補われたセル（本体 1 行目 2 列目）", body[1], "data-ref", res.HTML)
	})

	t.Run("ケース2_生HTMLの表とチェックボックスに無く番号にも入らない", func(t *testing.T) {
		const src = "<table><tr><td>raw</td></tr></table>\n\n" +
			"<input type=\"checkbox\">\n\n" +
			"| a |\n| --- |\n| 1 |\n\n" +
			"- [ ] task"
		res, els := renderWithKey(t, src, testRefKey)

		tables := byTag(els, "table")
		wantCount(t, "table", tables, 2, res.HTML)
		wantNoAttr(t, "生 HTML の table", tables[0], "data-ref", res.HTML)
		// 生 HTML の表を数えないため、後ろの GFM の表が 0 番になる。
		wantAttr(t, "GFM の table", tables[1], "data-ref", ref("table:0"), res.HTML)

		for _, cell := range tableRows(els)[0] {
			for _, c := range cell {
				wantNoAttr(t, "生 HTML の td", c, "data-ref", res.HTML)
			}
		}

		inputs := byTag(els, "input")
		wantCount(t, "input", inputs, 2, res.HTML)
		wantNoAttr(t, "生 HTML の input", inputs[0], "data-ref", res.HTML)
		wantAttr(t, "GFM のタスク", inputs[1], "data-ref", ref("task:0"), res.HTML)
	})

	t.Run("ケース3_生HTMLのaに data-link が無い", func(t *testing.T) {
		const src = `段落の中の <a href="./a.md">raw</a> リンク`
		res, els := renderWithKey(t, src, testRefKey)

		anchors := byTag(els, "a")
		wantCount(t, "a", anchors, 1, res.HTML)
		wantAttr(t, "生 HTML の a", anchors[0], "href", "./a.md", res.HTML)
		wantNoAttr(t, "生 HTML の a", anchors[0], "data-link", res.HTML)
	})

	t.Run("ケース4_鍵が空文字なら目印を出さない", func(t *testing.T) {
		const src = "- [ ] task\n\n" +
			"| a |\n| --- |\n| 1 |\n\n" +
			"[link](./a.md) <https://example.com/p>\n\n" +
			"```mermaid\ngraph TD\n```\n\n" +
			"```plantuml\n@startuml\nA -> B\n@enduml\n```"
		res, els := renderWithKey(t, src, "")

		// 対象の要素がそろって出ていること（空振りで緑にしない）。
		for _, tag := range []string{"input", "table", "td", "a"} {
			if len(byTag(els, tag)) == 0 {
				t.Fatalf("%s が出力に無い\n出力: %s", tag, res.HTML)
			}
		}
		if n := len(withClass(byTag(els, "div"), "code-block")); n != 2 {
			t.Fatalf("図のブロックが %d 個, want 2 個\n出力: %s", n, res.HTML)
		}

		for _, e := range els {
			wantNoAttr(t, "<"+e.Tag+">", e, "data-ref", res.HTML)
			wantNoAttr(t, "<"+e.Tag+">", e, "data-link", res.HTML)
		}
	})

	t.Run("ケース17_縦棒だけの本体の行のセルに data-ref が無い", func(t *testing.T) {
		const src = "| a | b |\n| --- | --- |\n|\n| 1 | 2 |"
		res, els := renderWithKey(t, src, testRefKey)

		tables := tableRows(els)
		if len(tables) != 1 || len(tables[0]) != 3 {
			t.Fatalf("表 1 つ・3 行の出力を期待した: %v\n出力: %s", tables, res.HTML)
		}
		row := tables[0][1]
		wantCount(t, "縦棒だけの行のセル", row, 2, res.HTML)
		for i, c := range row {
			wantNoAttr(t, "縦棒だけの行の "+strconv.Itoa(i+1)+" 列目", c, "data-ref", res.HTML)
		}
	})

	t.Run("ケース18_図でないコードブロックに data-ref が無い", func(t *testing.T) {
		tests := []struct {
			name      string
			in        string
			codeBlock bool // div.code-block が出るはずか（math は math-block になる）
		}{
			{"go", "```go\nx := 1\n```", true},
			{"言語指定なし", "```\nplain\n```", true},
			{"math", "```math\na+b\n```", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				res, els := renderWithKey(t, tt.in, testRefKey)

				blocks := withClass(byTag(els, "div"), "code-block")
				if tt.codeBlock && len(blocks) != 1 {
					t.Fatalf("div.code-block が %d 個, want 1 個\n出力: %s", len(blocks), res.HTML)
				}
				for _, d := range byTag(els, "div") {
					wantNoAttr(t, "div", d, "data-ref", res.HTML)
				}
			})
		}
	})

	t.Run("ケース19_生HTMLの図のブロックに無く番号にも入らない", func(t *testing.T) {
		const src = "<div class=\"code-block\" data-plantuml=\"1\" data-source=\"@startuml\">\n" +
			"<pre class=\"plantuml-source\">raw</pre>\n" +
			"</div>\n\n" +
			"```plantuml\n@startuml\nA -> B\n@enduml\n```"
		res, els := renderWithKey(t, src, testRefKey)

		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "div.code-block", blocks, 2, res.HTML)
		if _, ok := blocks[0].Attr("data-lang"); ok {
			t.Fatalf("1 つ目の div.code-block が生 HTML のものではない\n出力: %s", res.HTML)
		}
		wantNoAttr(t, "生 HTML の図のブロック", blocks[0], "data-ref", res.HTML)
		wantAttr(t, "GFM の PlantUML のブロック", blocks[1], "data-ref", ref("plantuml:0"), res.HTML)
	})
}

// TestRender_RefMarkers_Present は、GFM の要素に文書の出現順の目印が付くことを
// 検証する（UT-217 ケース 5〜16・20・21。根拠: FR-023, FR-024, FR-061, FR-063,
// FR-130, FR-141, FR-142 / IMP-115, IMP-119, IMP-120）。
func TestRender_RefMarkers_Present(t *testing.T) {
	t.Run("ケース5_タスクの番号", func(t *testing.T) {
		res, els := renderWithKey(t, "- [ ] a\n- [x] b", testRefKey)

		inputs := byTag(els, "input")
		wantCount(t, "input", inputs, 2, res.HTML)
		wantAttr(t, "1 つ目のタスク", inputs[0], "data-ref", ref("task:0"), res.HTML)
		wantAttr(t, "2 つ目のタスク", inputs[1], "data-ref", ref("task:1"), res.HTML)
	})

	t.Run("ケース6_入れ子とゆるいリストも出現順", func(t *testing.T) {
		const src = "- [ ] a\n  - [ ] inner\n- [ ] c\n\n" +
			"para\n\n" +
			"- [ ] loose1\n\n- [ ] loose2"
		res, els := renderWithKey(t, src, testRefKey)

		inputs := byTag(els, "input")
		wantCount(t, "input", inputs, 5, res.HTML)
		for i, want := range []string{"task:0", "task:1", "task:2", "task:3", "task:4"} {
			wantAttr(t, "タスク "+want, inputs[i], "data-ref", ref(want), res.HTML)
		}
	})

	t.Run("ケース7_表の番号", func(t *testing.T) {
		const src = "| a |\n| --- |\n| 1 |\n\npara\n\n| b |\n| --- |\n| 2 |"
		res, els := renderWithKey(t, src, testRefKey)

		tables := byTag(els, "table")
		wantCount(t, "table", tables, 2, res.HTML)
		wantAttr(t, "1 つ目の表", tables[0], "data-ref", ref("table:0"), res.HTML)
		wantAttr(t, "2 つ目の表", tables[1], "data-ref", ref("table:1"), res.HTML)
	})

	t.Run("ケース8_セルの番号", func(t *testing.T) {
		const src = "| h1 | h2 | h3 |\n| --- | --- | --- |\n| a | b | c |\n| d | e | f |"
		res, els := renderWithKey(t, src, testRefKey)

		tables := tableRows(els)
		if len(tables) != 1 || len(tables[0]) != 3 {
			t.Fatalf("表 1 つ・3 行の出力を期待した: %v\n出力: %s", tables, res.HTML)
		}
		head, last := tables[0][0], tables[0][2]
		wantCount(t, "見出し行のセル", head, 3, res.HTML)
		wantCount(t, "本体 2 行目のセル", last, 3, res.HTML)

		if head[0].Tag != "th" || head[0].Text != "h1" {
			t.Fatalf("見出し行の 1 列目が th の h1 ではない: %+v", head[0])
		}
		wantAttr(t, "見出し行の 1 列目", head[0], "data-ref", ref("cell:0:0:0"), res.HTML)

		if last[2].Text != "f" {
			t.Fatalf("本体 2 行目 3 列目が f ではない: %+v", last[2])
		}
		wantAttr(t, "本体 2 行目 3 列目", last[2], "data-ref", ref("cell:0:2:2"), res.HTML)
	})

	// ケース 9〜16: リンク先は「書かれたとおり」（IMP-120 の Destination / Label）。
	links := []struct {
		name string
		in   string
		want string // data-link の鍵の後ろ
	}{
		{"ケース9_相対パスとフラグメント", "[x](./docs/design.md#api)", "./docs/design.md#api"},
		{"ケース10_非ASCIIのパス", "[x](./設計.md)", "./設計.md"},
		{"ケース11_参照リンク", "[x][r]\n\n[r]: ./a.md", "./a.md"},
		{"ケース12_山括弧の自動リンク", "<https://example.com/p>", "https://example.com/p"},
		{"ケース13_裸のwww", "www.example.com/p", "www.example.com/p"},
		{"ケース14_メールアドレス", "<foo@bar.com>", "foo@bar.com"},
		{"ケース15_山括弧で囲んだ宛先", "[x](<./a b.md>)", "./a b.md"},
		{"ケース16_属性値のエスケープ", `[x](./a"b<c&d.md)`, `./a"b<c&d.md`},
	}
	for _, tt := range links {
		t.Run(tt.name, func(t *testing.T) {
			res, els := renderWithKey(t, tt.in, testRefKey)

			anchors := byTag(els, "a")
			wantCount(t, "a", anchors, 1, res.HTML)
			wantAttr(t, "リンク", anchors[0], "data-link", ref(tt.want), res.HTML)
		})
	}

	t.Run("ケース10_hrefは百分率エンコードされたまま", func(t *testing.T) {
		res, els := renderWithKey(t, "[x](./設計.md)", testRefKey)

		anchors := byTag(els, "a")
		wantCount(t, "a", anchors, 1, res.HTML)
		wantAttr(t, "リンク", anchors[0], "href", "./%E8%A8%AD%E8%A8%88.md", res.HTML)
	})

	t.Run("ケース16_出力の属性値が閉じていない引用符を含まない", func(t *testing.T) {
		res, _ := renderWithKey(t, `[x](./a"b<c&d.md)`, testRefKey)

		// トークナイザが値を正しく取り出せても、生の出力に < や " が
		// そのまま入っていれば属性値のエスケープが抜けている。
		if strings.Contains(res.HTML, `a"b`) || strings.Contains(res.HTML, "b<c") {
			t.Errorf("data-link の値がエスケープされていない\n出力: %s", res.HTML)
		}
	})

	t.Run("ケース20_図のブロックは種類ごとに数える", func(t *testing.T) {
		const src = "```mermaid\ngraph TD\n```\n\n" +
			"```plantuml\n@startuml\nA -> B\n@enduml\n```\n\n" +
			"```mermaid\ngraph LR\n```"
		res, els := renderWithKey(t, src, testRefKey)

		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "div.code-block", blocks, 3, res.HTML)
		for i, want := range []string{"mermaid:0", "plantuml:0", "mermaid:1"} {
			wantAttr(t, "図のブロック "+want, blocks[i], "data-ref", ref(want), res.HTML)
		}
	})

	t.Run("ケース21_拒んだPlantUMLのブロックにも付き番号を数える", func(t *testing.T) {
		const src = "```plantuml\n@startuml\nA -> B\n@enduml\n```\n\n" +
			"```plantuml\n@startuml\n!include a.puml\n@enduml\n```\n\n" +
			"```plantuml\n@startuml\nC -> D\n@enduml\n```"
		res, els := renderWithKey(t, src, testRefKey)

		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "div.code-block", blocks, 3, res.HTML)
		if v, _ := blocks[1].Attr("data-puml-error"); v != "include" {
			t.Fatalf("2 つ目のブロックが拒んだブロックではない: %+v\n出力: %s", blocks[1], res.HTML)
		}
		for i, want := range []string{"plantuml:0", "plantuml:1", "plantuml:2"} {
			wantAttr(t, "PlantUML のブロック "+want, blocks[i], "data-ref", ref(want), res.HTML)
		}
	})
}
