package renderer

import (
	"reflect"
	"strings"
	"testing"
)

// plantUMLBasic は UT-119 の基本の PlantUML ファイル（LF）。
// 図 0 の範囲は [0, 26)、図 1 は [28, 54)（どちらも @end… の行の改行を含まない）。
const plantUMLBasic = "@startuml a\nA -> B\n@enduml\n\n@startuml b\nC -> D\n@enduml\n"

// TestSplitPlantUML は PlantUML ファイルの切り分けを検証する
// （UT-223。根拠: FR-025, MD-086, MD-083 / IMP-126）。
//
// **ケース 5 と 6 は、@end… の数え方の境界である。** 「次の @end で終わる」実装はケース 5 で
// 2 つ目の図の @end まで取り込み、「次の @start で終わる」実装はケース 6 で図を 2 つに割る。
// 期待値は入力のバイトを数えて人が書いた（UT-031）。
func TestSplitPlantUML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Diagram
	}{
		// ケース 15: 空のファイル / 図の無いファイル（境界を先に置く。UT-013）。
		{"15 空のファイル", "", nil},
		{"15 図の無いファイル", "Alice -> Bob\n", nil},
		// ケース 14: 行頭でなければ始まらない。@end… だけでは図にならない。
		{"14 行の途中の@startuml", "x @startuml\nA\n@enduml\n", nil},
		{"14 @endumlの行だけ", "@enduml\n", nil},

		// ケース 1: UT-119 の基本のファイル。
		{"1 基本のファイル", plantUMLBasic, []Diagram{
			{Name: "a", Span: Span{0, 26}, Closed: true},
			{Name: "b", Span: Span{28, 54}, Closed: true},
		}},
		// ケース 2: 大文字小文字を区別しない。
		{"2 大文字小文字", "@StartUML\nA\n@EndUML\n", []Diagram{
			{Name: "Diagram 1", Span: Span{0, 19}, Closed: true},
		}},
		// ケース 3: @start… の行の頭から、@end… の行の改行の手前まで。
		{"3 行頭の空白", "  @startuml\nA\n  @enduml\n", []Diagram{
			{Name: "Diagram 1", Span: Span{0, 23}, Closed: true},
		}},
		// ケース 4: uml 以外の種類。
		{"4 mindmap", "@startmindmap\n* a\n@endmindmap\n", []Diagram{
			{Name: "Diagram 1", Span: Span{0, 29}, Closed: true},
		}},
		{"4 json", "@startjson\n{}\n@endjson\n", []Diagram{
			{Name: "Diagram 1", Span: Span{0, 22}, Closed: true},
		}},
		// ケース 5: 同じ種類の @end… でだけ終わる。ファイルの終わりまで。
		{"5 種類の違う@end", "@startuml\nA\n@endmindmap\n", []Diagram{
			{Name: "Diagram 1", Span: Span{0, 24}, Closed: false},
		}},
		// ケース 6: 図の中の別の @start… は、その図の @end… まで図に含める。
		{"6 図の中の別の@start", "@startuml\n@startmindmap\nA\n@enduml\n", []Diagram{
			{Name: "Diagram 1", Span: Span{0, 33}, Closed: true},
		}},
		// ケース 7: 図の外の行を無視する。
		{"7 図の外の行", "title x\n@startuml\nA\n@enduml\nfoo\n", []Diagram{
			{Name: "Diagram 1", Span: Span{8, 27}, Closed: true},
		}},
		// ケース 8: @end… の無い図はファイルの終わりまで（末尾の改行を含む。MD-086 の 4）。
		{"8 @endの無い図", "@startuml\nA -> B\n", []Diagram{
			{Name: "Diagram 1", Span: Span{0, 17}, Closed: false},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitPlantUML([]byte(tt.in))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitPlantUML(%q)\n got %+v\nwant %+v", tt.in, got, tt.want)
			}
		})
	}
}

// TestSplitPlantUML_Names は図の見出しの決め方を検証する（UT-223 のケース 9〜13。根拠: MD-086 の 3 / IMP-126）。
//
// 名前 → 図の中の最初の title の行 → Diagram <n>（1 起点）の順。
func TestSplitPlantUML_Names(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		// ケース 9: @start… の後ろの名前（前後の空白を除く）。
		{"9 名前", "@startuml name\nA\n@enduml\n", []string{"name"}},
		{"9 id=", "@startuml(id=foo)\nA\n@enduml\n", []string{"foo"}},
		{"9 空白を挟んだ名前", "@startuml   spaced  \nA\n@enduml\n", []string{"spaced"}},
		// ケース 10: 名前の無い図の中の最初の title の行（行頭の空白を除き、大文字小文字を区別しない）。
		{"10 title", "@startuml\ntitle Hello\nA\n@enduml\n", []string{"Hello"}},
		{"10 字下げした大文字のTITLE", "@startuml\n  TITLE Hi\nA\n@enduml\n", []string{"Hi"}},
		// ケース 11: 名前が title より優先。
		{"11 名前が優先", "@startuml a\ntitle t\n@enduml\n", []string{"a"}},
		// ケース 12: 名前も title の行も無い 2 つ目の図。
		{"12 Diagram 2", "@startuml a\nA\n@enduml\n@startuml\nB\n@enduml\n", []string{"a", "Diagram 2"}},
		// ケース 13: title が図の外にだけある / 複数行の title と end title だけがある。
		{"13 図の外のtitle", "title x\n@startuml\nA\n@enduml\n", []string{"Diagram 1"}},
		{"13 複数行のtitle", "@startuml\ntitle\nfoo\nend title\nA\n@enduml\n", []string{"Diagram 1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, d := range SplitPlantUML([]byte(tt.in)) {
				got = append(got, d.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitPlantUML(%q) の Name = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// renderPlantUMLFile は PlantUML ファイルを変換する。失敗した時点で打ち切る。
func renderPlantUMLFile(t *testing.T, source string, opts Options) (Result, []element) {
	t.Helper()

	res, err := New().RenderPlantUMLFile([]byte(source), opts)
	if err != nil {
		t.Fatalf("RenderPlantUMLFile(%q) がエラーを返した: %v", source, err)
	}
	return res, scanElements(t, res.HTML)
}

// TestRenderPlantUMLFile は PlantUML ファイルの変換を検証する
// （UT-224。根拠: FR-025, FR-040, MD-084, MD-085, MD-086, NFR-013, AR-053, FR-146 / IMP-117, IMP-119, IMP-126）。
//
// **ケース 1 の data-source が図ごとであることが要である。** 処理系は 1 回の描画で先頭の図しか
// 描かず、ファイル全体を渡すと 2 つ目以降の図が黙って消える（MD-086。実測）。**ケース 4 と 5 は、
// 拒んだ図を「数えるが描かない」ことを両側から見る**（IMP-119 と同じ）。鍵は 0123456789abcdef。
func TestRenderPlantUMLFile(t *testing.T) {
	opts := Options{RefKey: testRefKey}

	// ケース 1: 図ごとに、見出しと図のブロックを上から並べる。
	t.Run("1 基本のファイル", func(t *testing.T) {
		res, els := renderPlantUMLFile(t, plantUMLBasic, opts)

		var seq []element
		for _, e := range els {
			if e.Tag == "h2" || (e.Tag == "div" && len(withClass([]element{e}, "code-block")) == 1) {
				seq = append(seq, e)
			}
		}
		wantCount(t, "見出しと図のブロック", seq, 4, res.HTML)
		wantAttr(t, "見出し a", seq[0], "id", "user-content-a", res.HTML)
		if seq[0].Tag != "h2" || strings.TrimSpace(seq[0].Text) != "a" {
			t.Errorf("1 つ目が <h2>a</h2> ではない: %+v", seq[0])
		}
		wantAttr(t, "図 0", seq[1], "data-plantuml", "1", res.HTML)
		wantAttr(t, "図 0", seq[1], "data-ref", ref("plantuml:0"), res.HTML)
		wantAttr(t, "図 0", seq[1], "data-source", "@startuml a\nA -> B\n@enduml", res.HTML)
		wantAttr(t, "見出し b", seq[2], "id", "user-content-b", res.HTML)
		wantAttr(t, "図 1", seq[3], "data-ref", ref("plantuml:1"), res.HTML)
		wantAttr(t, "図 1", seq[3], "data-source", "@startuml b\nC -> D\n@enduml", res.HTML)

		want := []Heading{{Level: 2, Text: "a", ID: "user-content-a"}, {Level: 2, Text: "b", ID: "user-content-b"}}
		if !reflect.DeepEqual(res.Headings, want) {
			t.Errorf("Headings = %+v, want %+v", res.Headings, want)
		}
		if !res.NeedsPlantUML || res.NoDiagrams {
			t.Errorf("NeedsPlantUML / NoDiagrams = %v / %v, want true / false", res.NeedsPlantUML, res.NoDiagrams)
		}
	})

	// ケース 2: 同じ名前の図には slugger が -1 を付ける。
	t.Run("2 同じ名前", func(t *testing.T) {
		res, els := renderPlantUMLFile(t, "@startuml x\nA\n@enduml\n@startuml x\nB\n@enduml\n", opts)
		hs := byTag(els, "h2")
		wantCount(t, "h2", hs, 2, res.HTML)
		wantAttr(t, "1 つ目の見出し", hs[0], "id", "user-content-x", res.HTML)
		wantAttr(t, "2 つ目の見出し", hs[1], "id", "user-content-x-1", res.HTML)
	})

	// ケース 3: 見出しの文字は Markdown として解かず、HTML としてエスケープする。
	t.Run("3 見出しの文字", func(t *testing.T) {
		res, els := renderPlantUMLFile(t, "@startuml **x**\nA\n@enduml\n@startuml <b>y</b>\nB\n@enduml\n", opts)
		hs := byTag(els, "h2")
		wantCount(t, "h2", hs, 2, res.HTML)
		if got := strings.TrimSpace(hs[0].Text); got != "**x**" {
			t.Errorf("1 つ目の見出しの文字 = %q, want %q", got, "**x**")
		}
		if got := strings.TrimSpace(hs[1].Text); got != "<b>y</b>" {
			t.Errorf("2 つ目の見出しの文字 = %q, want %q（文字として出る）", got, "<b>y</b>")
		}
		if n := len(byTag(els, "b", "strong")); n != 0 {
			t.Errorf("見出しの名前が要素になった（%d 個）\n出力: %s", n, res.HTML)
		}
	})

	// ケース 4: 図ごとに取り込み指令を検査する（MD-086 の 6）。拒んだ図も目印と見出しはある。
	t.Run("4 図1だけ!include", func(t *testing.T) {
		res, els := renderPlantUMLFile(t, "@startuml a\nA -> B\n@enduml\n@startuml b\n!include x.puml\n@enduml\n", opts)
		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "図のブロック", blocks, 2, res.HTML)
		wantAttr(t, "図 0", blocks[0], "data-plantuml", "1", res.HTML)
		wantNoAttr(t, "図 1", blocks[1], "data-plantuml", res.HTML)
		wantAttr(t, "図 1", blocks[1], "data-puml-error", "include", res.HTML)
		wantAttr(t, "図 1", blocks[1], "data-ref", ref("plantuml:1"), res.HTML)
		if n := len(byTag(els, "h2")); n != 2 {
			t.Errorf("見出しが %d 個, want 2（拒んだ図にも見出しを付ける）", n)
		}
		if !res.NeedsPlantUML {
			t.Error("NeedsPlantUML が偽（描画の対象の図 0 がある）")
		}
	})

	// ケース 5: すべての図が !include を含むなら資産を読まない（NFR-013, MD-085）。
	t.Run("5 すべて!include", func(t *testing.T) {
		res, _ := renderPlantUMLFile(t, "@startuml a\n!include x.puml\n@enduml\n@startuml b\n!include y.puml\n@enduml\n", opts)
		if res.NeedsPlantUML {
			t.Error("NeedsPlantUML が真（描く図が無いのに資産を読む）")
		}
	})

	// ケース 6: 図の無いファイルは原文を等幅のコードブロックで出す。文言は Go 側に書かない（IMP-290）。
	t.Run("6 図の無いファイル", func(t *testing.T) {
		res, els := renderPlantUMLFile(t, "Alice -> Bob\n", opts)
		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "div.code-block", blocks, 1, res.HTML)
		wantAttr(t, "ブロック", blocks[0], "data-lang", "plantuml", res.HTML)
		wantNoAttr(t, "ブロック", blocks[0], "data-ref", res.HTML)
		wantNoAttr(t, "ブロック", blocks[0], "data-plantuml", res.HTML)
		codes := byTag(els, "code")
		if len(codes) != 1 || codes[0].Text != "Alice -> Bob\n" {
			t.Errorf("pre code の文字 = %+v, want %q", codes, "Alice -> Bob\n")
		}
		if !res.NoDiagrams || res.NeedsPlantUML || len(res.Headings) != 0 {
			t.Errorf("NoDiagrams / NeedsPlantUML / Headings = %v / %v / %v, want true / false / 空", res.NoDiagrams, res.NeedsPlantUML, res.Headings)
		}
		if strings.Contains(res.HTML, "No diagrams found") {
			t.Errorf("文言が Go 側で出ている（フロントエンドが出す）\n出力: %s", res.HTML)
		}
	})

	// ケース 7: 修飾の目印は付けない（FR-146 の対象外）。
	t.Run("7 TextRefs", func(t *testing.T) {
		_, els := renderPlantUMLFile(t, plantUMLBasic, Options{RefKey: testRefKey, TextRefs: true})
		if got := refsOf(els, "text"); len(got) != 0 {
			t.Errorf("text: の目印がある: %v", got)
		}
	})

	// ケース 8: 鍵が空なら data-ref が無い。
	t.Run("8 鍵が空", func(t *testing.T) {
		res, els := renderPlantUMLFile(t, plantUMLBasic, Options{})
		if len(withClass(byTag(els, "div"), "code-block")) == 0 {
			t.Fatalf("図のブロックが無い（検査の前提が崩れている）\n出力: %s", res.HTML)
		}
		for _, e := range els {
			wantNoAttr(t, "<"+e.Tag+">", e, "data-ref", res.HTML)
		}
	})

	// ケース 9: @end… の無い最後の図の data-source はファイルの終わりまで。
	t.Run("9 @endの無い最後の図", func(t *testing.T) {
		res, els := renderPlantUMLFile(t, "@startuml a\nA\n@enduml\n@startuml b\nB\n", opts)
		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "図のブロック", blocks, 2, res.HTML)
		wantAttr(t, "図 1", blocks[1], "data-source", "@startuml b\nB\n", res.HTML)
	})

	// ケース 10: 原文の < > & は属性・本文とも正しくエスケープされる（IMP-119 と同じ）。
	t.Run("10 エスケープ", func(t *testing.T) {
		const src = "@startuml\nA -> B : <b>&amp; x\n@enduml\n"
		res, els := renderPlantUMLFile(t, src, opts)
		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "図のブロック", blocks, 1, res.HTML)
		wantAttr(t, "図", blocks[0], "data-source", strings.TrimSuffix(src, "\n"), res.HTML)
		if n := len(byTag(els, "b")); n != 0 {
			t.Errorf("原文の <b> が要素になった\n出力: %s", res.HTML)
		}
		if pres := byTag(els, "pre"); len(pres) != 1 || !strings.Contains(pres[0].Text, "<b>&amp; x") {
			t.Errorf("pre の文字に原文が出ていない: %+v\n出力: %s", pres, res.HTML)
		}
	})

	// ケース 11: UT-213 の異常入力で panic しない。
	for _, tt := range robustInputs() {
		t.Run("11 "+tt.name, func(t *testing.T) {
			withinLimit(t, func() { _, _ = New().RenderPlantUMLFile([]byte(tt.in), opts) })
		})
	}
}
