package renderer

import (
	"reflect"
	"strings"
	"testing"
)

// locate は Locate を呼ぶ。失敗した時点で打ち切る。
func locate(t *testing.T, source string) Locations {
	t.Helper()

	locs, err := New().Locate([]byte(source))
	if err != nil {
		t.Fatalf("Locate(%q) がエラーを返した: %v", source, err)
	}
	return locs
}

// onlyCode は文書のただ 1 つのコードブロックの位置を返す。
func onlyCode(t *testing.T, source string) FenceLocation {
	t.Helper()

	locs := locate(t, source)
	if len(locs.Codes) != 1 {
		t.Fatalf("Locate(%q) の Codes が %d 個, want 1", source, len(locs.Codes))
	}
	return locs.Codes[0]
}

// TestLocate_Fences はコードブロックと図の位置を検証する
// （UT-222 のケース 1〜9・16・17・19。根拠: FR-145, AR-031 / IMP-111, IMP-121, IMP-122）。
//
// 位置は正規化後のテキストの上のバイト位置である（UT-218 と同じ）。期待値は入力のバイトを
// 数えて人が書いた（UT-031）。
func TestLocate_Fences(t *testing.T) {
	// ケース 1: UT-118 の基本の文書。
	t.Run("1 基本の文書", func(t *testing.T) {
		got := onlyCode(t, "```go\na\nb\n```\n")
		want := FenceLocation{
			Open: Span{0, 3}, Close: Span{10, 13}, Closed: true, Body: Span{6, 10},
			Lines: []Span{{6, 7}, {8, 9}}, Info: "go", Prefix: "", Locked: false,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Codes[0] = %+v\nwant      %+v", got, want)
		}
	})

	// ケース 2: ~ の囲いも同じ。
	t.Run("2 チルダの囲い", func(t *testing.T) {
		got := onlyCode(t, "~~~\na\n~~~\n")
		if got.Open != (Span{0, 3}) || got.Close != (Span{6, 9}) {
			t.Errorf("Open / Close = %v / %v, want [0, 3) / [6, 9)", got.Open, got.Close)
		}
	})

	// ケース 3: 文書の終わりで閉じたブロック。
	t.Run("3 文書の終わりで閉じた", func(t *testing.T) {
		got := onlyCode(t, "```\na")
		if got.Closed || got.Close != (Span{5, 5}) || got.Body != (Span{4, 5}) {
			t.Errorf("Closed / Close / Body = %v / %v / %v, want false / [5, 5) / [4, 5)", got.Closed, got.Close, got.Body)
		}
	})

	// ケース 4: 空のブロック。
	t.Run("4 空のブロック", func(t *testing.T) {
		got := onlyCode(t, "```\n```\n")
		if len(got.Lines) != 0 || got.Body != (Span{4, 4}) || got.Close != (Span{4, 7}) || got.Prefix != "" {
			t.Errorf("Lines / Body / Close / Prefix = %v / %v / %v / %q, want 空 / [4, 4) / [4, 7) / 空", got.Lines, got.Body, got.Close, got.Prefix)
		}
	})

	// ケース 5: 引用の中のブロック（UT-118 のケース 11）。Lines は前置きの後ろから。
	// 空の行は > の後ろの長さ 0 の区間。空の行の > は許す。
	t.Run("5 引用の中", func(t *testing.T) {
		got := onlyCode(t, "> ```\n> a\n>\n> b\n> ```\n")
		if got.Prefix != "> " || got.Locked {
			t.Errorf("Prefix / Locked = %q / %v, want %q / false", got.Prefix, got.Locked, "> ")
		}
		if want := []Span{{8, 9}, {11, 11}, {14, 15}}; !reflect.DeepEqual(got.Lines, want) {
			t.Errorf("Lines = %v, want %v", got.Lines, want)
		}
		if got.Body != (Span{6, 16}) || got.Close != (Span{18, 21}) {
			t.Errorf("Body / Close = %v / %v, want [6, 16) / [18, 21)", got.Body, got.Close)
		}
	})

	// ケース 6: リストの中のブロック（UT-118 のケース 12）。
	t.Run("6 リストの中", func(t *testing.T) {
		if got := onlyCode(t, "- ```\n  a\n  ```\n"); got.Prefix != "  " {
			t.Errorf("Prefix = %q, want 半角空白 2 つ", got.Prefix)
		}
	})

	// ケース 7: 前置きが行ごとに違うブロック / タブの途中から始まる行を持つブロック（UT-118 のケース 20）。
	// 描画の data-locked（UT-217 のケース 23）と書き込み（UT-118）が同じ値を使う。
	for _, src := range []string{"> ```\n> a\n>b\n> ```\n", "1. ```\n\tz\n   ```\n"} {
		t.Run("7 Locked "+strings.ReplaceAll(src, "\n", "⏎"), func(t *testing.T) {
			if got := onlyCode(t, src); !got.Locked {
				t.Errorf("Locked が偽: %+v", got)
			}
		})
	}

	// ケース 8: 字下げ形式と数式は数えない。図は種類ごと。
	t.Run("8 種類ごとに数える", func(t *testing.T) {
		locs := locate(t, "```go\nx\n```\n\npara\n\n    indented\n\n```math\na+b\n```\n\n"+
			"```mermaid\ngraph TD\n```\n\n```plantuml\n@startuml\nA -> B\n@enduml\n```\n\n```py\ny\n```\n")
		if len(locs.Codes) != 2 || len(locs.Mermaids) != 1 || len(locs.PlantUMLs) != 1 {
			t.Fatalf("Codes / Mermaids / PlantUMLs = %d / %d / %d, want 2 / 1 / 1", len(locs.Codes), len(locs.Mermaids), len(locs.PlantUMLs))
		}
		if locs.Codes[0].Info != "go" || locs.Codes[1].Info != "py" {
			t.Errorf("Codes の Info = %q / %q, want go / py", locs.Codes[0].Info, locs.Codes[1].Info)
		}
	})

	// ケース 9: TOML の Front Matter の後のブロック（UT-218 のケース 5 と同じ補正）。
	t.Run("9 TOMLのFrontMatter", func(t *testing.T) {
		if got := onlyCode(t, "+++\na = 1\n+++\n```\nx\n```\n"); got.Body != (Span{18, 20}) {
			t.Errorf("Body = %v, want [18, 20)", got.Body)
		}
	})

	// ケース 16: 開きの囲いの行で終わり、改行も中身も無い文書（UT-118 のケース 25。4.77.0）。
	t.Run("16 開きの囲いだけ", func(t *testing.T) {
		got := onlyCode(t, "```go")
		if got.Body != (Span{5, 5}) || got.Closed || len(got.Lines) != 0 || got.Info != "go" {
			t.Errorf("Body / Closed / Lines / Info = %v / %v / %v / %q, want [5, 5) / false / 空 / go", got.Body, got.Closed, got.Lines, got.Info)
		}
	})

	// ケース 17: 前置きは最初の空でない中身の行から取る / 閉じの囲いの行から取る /
	// 開きの囲いの行の前置きにリストの記号があり決められない（4.78.0。IMP-122 の「前置き」）。
	t.Run("17 1行目が空の引用の中", func(t *testing.T) {
		if got := onlyCode(t, "> ```\n>\n> a\n> ```\n"); got.Prefix != "> " || got.Locked {
			t.Errorf("Prefix / Locked = %q / %v, want %q / false", got.Prefix, got.Locked, "> ")
		}
	})
	t.Run("17 リストの中の空のブロック", func(t *testing.T) {
		if got := onlyCode(t, "- ```\n  ```\n"); got.Prefix != "  " {
			t.Errorf("Prefix = %q, want 半角空白 2 つ", got.Prefix)
		}
	})
	t.Run("17 リストの中で閉じも中身も無い", func(t *testing.T) {
		if got := onlyCode(t, "- ```"); !got.Locked {
			t.Errorf("Locked が偽: %+v", got)
		}
	})

	// ケース 19: 引用の記号の直後のタブ（4.86.0。独立したレビューの指摘）。goldmark の Pos() は
	// タブの残り（Segment.Padding）の桁数だけ後ろへずれるため、開きの囲いの位置に使えない。
	// 開きがずれると、囲いの文字を取り違えて閉じも記録できなくなる。
	for _, c := range []struct {
		name, src string
		want      FenceLocation
	}{
		{"19 引用の記号とタブ", ">\t```\n>\ta\n>\t```\n", FenceLocation{
			Open: Span{2, 5}, Close: Span{12, 15}, Closed: true, Body: Span{6, 10},
			Lines: []Span{{8, 9}}, Prefix: ">\t",
		}},
		{"19 字下げした引用の記号とタブ", "   >\t```go\n   >\ta\n   >\t```\n", FenceLocation{
			Open: Span{5, 8}, Close: Span{23, 26}, Closed: true, Body: Span{11, 18},
			Lines: []Span{{16, 17}}, Info: "go", Prefix: "   >\t",
		}},
		{"19 字下げした引用の記号とタブの空のブロック", "   >\t```\n   >\t```\n", FenceLocation{
			Open: Span{5, 8}, Close: Span{14, 17}, Closed: true, Body: Span{9, 9},
			Prefix: "   >\t",
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := onlyCode(t, c.src); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Locate(%q).Codes[0]\n got = %+v\nwant = %+v", c.src, got, c.want)
			}
			// 描画の data-locked も同じ判定を使う（IMP-115）。前置きはどの行も同じなので付かない。
			res, err := New().Render([]byte(c.src), Options{RefKey: testRefKey})
			if err != nil || strings.Contains(res.HTML, "data-locked") {
				t.Errorf("Render(%q) = %q, %v; data-locked が付いた", c.src, res.HTML, err)
			}
		})
	}
}

// TestLocate_FencesShareCounting は、Render の目印と Locate の番号が同じブロックを指すことを
// 検証する（UT-222 のケース 10・11・18。根拠: FR-145, FR-146 / IMP-120, IMP-121）。
//
// UT-218 のケース 1 と同じ考え方である。IMP-120 と IMP-121 が同じ数え方（walkRefs）を共有する。
func TestLocate_FencesShareCounting(t *testing.T) {
	// ケース 10: コードブロック 2 つ・Mermaid 2 つ・PlantUML 1 つ。TextRefs の真偽の両方で突き合わせる。
	const src = "```go\nc0\n```\n\n```mermaid\ngraph A\n```\n\npara\n\n" +
		"```plantuml\n@startuml\nP\n@enduml\n```\n\n```py\nc1\n```\n\n```mermaid\ngraph B\n```\n"
	locs := locate(t, src)
	if len(locs.Codes) != 2 || len(locs.Mermaids) != 2 || len(locs.PlantUMLs) != 1 {
		t.Fatalf("Codes / Mermaids / PlantUMLs = %d / %d / %d, want 2 / 2 / 1", len(locs.Codes), len(locs.Mermaids), len(locs.PlantUMLs))
	}
	byKind := map[string][]FenceLocation{"code": locs.Codes, "mermaid": locs.Mermaids, "plantuml": locs.PlantUMLs}

	for _, textRefs := range []bool{false, true} {
		name := "10 TextRefs=偽"
		if textRefs {
			name = "10 TextRefs=真"
		}
		t.Run(name, func(t *testing.T) {
			res, els := renderOpts(t, src, Options{RefKey: testRefKey, TextRefs: textRefs})
			blocks := withClass(byTag(els, "div"), "code-block")
			wantCount(t, "div.code-block", blocks, 5, res.HTML)
			for _, b := range blocks {
				v, ok := b.Attr("data-ref")
				if !ok {
					t.Errorf("目印の無いブロックがある: %+v\n出力: %s", b, res.HTML)
					continue
				}
				var kind string
				var n int
				rest := strings.TrimPrefix(v, testRefKey+":")
				kind, num, _ := strings.Cut(rest, ":")
				for _, r := range num {
					n = n*10 + int(r-'0')
				}
				fl := byKind[kind]
				if n >= len(fl) {
					t.Errorf("%s の番号が Locate の数（%d）を超える", rest, len(fl))
					continue
				}
				// 画面のブロックの原文（図は data-source、コードブロックは文字）と、Locate の中身が同じ。
				shown, ok := b.Attr("data-source")
				if !ok {
					shown = strings.TrimSpace(b.Text)
				}
				body := strings.TrimRight(src[fl[n].Body.Start:fl[n].Body.Stop], "\n")
				if shown != body {
					t.Errorf("%s の画面の原文 %q と Locate の中身 %q が違う", rest, shown, body)
				}
			}
		})
	}

	// ケース 11: Texts は TextRefs を真にした Render の text: の数と同じ（Alerts の見出しと表のセルを数えない）。
	t.Run("11 Texts", func(t *testing.T) {
		const doc = "p1\n\np2\n\n# h\n\n- i1\n- i2\n\n> [!NOTE]\n> body\n\n| a |\n| --- |\n| 1 |\n"
		if got := locate(t, doc).Texts; got != 6 {
			t.Errorf("Texts = %d, want 6", got)
		}
		res, els := renderOpts(t, doc, Options{RefKey: testRefKey, TextRefs: true})
		if n := len(refsOf(els, "text")); n != 6 {
			t.Errorf("Render の text: が %d 個, want 6\n出力: %s", n, res.HTML)
		}
	})

	// ケース 18: UT-217 のケース 30 の 2 つの項目。Texts がそれぞれ 1 / 0。
	t.Run("18 リストの項目のTextBlock", func(t *testing.T) {
		if got := locate(t, listSecondTextBlock).Texts; got != 1 {
			t.Errorf("Texts = %d, want 1", got)
		}
		if got := locate(t, listFirstFence).Texts; got != 0 {
			t.Errorf("Texts = %d, want 0", got)
		}
	})
}

// TestLocate_Blocks はブロックの種類の並びを検証する（UT-222 のケース 12〜14。根拠: FR-145, FR-146 /
// IMP-121, IMP-122, IMP-124）。
//
// **書き込みの確かめ（IMP-122 と IMP-124 の 9）の土台である。** 比べ方が甘いと（数だけを比べる、
// など）、引用をリストに変えるような書き込みを「構造を変えない」と判断して通す。**名前そのものは
// 固定しない**（UT-034, UT-042）。等しいか違うかだけを見る。
func TestLocate_Blocks(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		equal bool
	}{
		// ケース 12: 中の文字だけが違う 2 つの文書。
		{"12 文字だけ違う", "# a\n\nb\n\n- c\n", "# x\n\ny\n\n- z\n", true},
		// ケース 13: 段落を割った（数が違う）/ 引用と段落 2 つをリストに変えた（数は同じで種類が違う）。
		{"13 数が違う", "a b\n", "a\n\nb\n", false},
		{"13 種類が違う", "> a\n>\n> b\n", "- a\n", false},
		// ケース 14: インラインのタグはブロックを変えない（IMP-124 の 9）。
		{"14 delを差し込む", "a\n", "<del>a</del>\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := locate(t, tt.a).Blocks, locate(t, tt.b).Blocks
			if len(a) == 0 || len(b) == 0 {
				t.Fatalf("Blocks が空: %v / %v", a, b)
			}
			if got := reflect.DeepEqual(a, b); got != tt.equal {
				t.Errorf("Blocks が等しいか = %v, want %v\n%q: %v\n%q: %v", got, tt.equal, tt.a, a, tt.b, b)
			}
		})
	}
}

// TestLocate_FencesDoNotCrash は異常な入力で落ちないことを検証する（UT-222 のケース 15。根拠: FR-111 / IMP-022）。
func TestLocate_FencesDoNotCrash(t *testing.T) {
	for _, tt := range robustInputs() {
		t.Run(tt.name, func(t *testing.T) {
			withinLimit(t, func() { _, _ = New().Locate([]byte(tt.in)) })
		})
	}
}
