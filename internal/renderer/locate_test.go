package renderer

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLocate_SharesCounting は、Render が付けた番号と Locate の番号が同じ
// 項目を指すことを検証する（UT-218 ケース 1。根拠: FR-141, FR-142 / IMP-120, IMP-121）。
//
// **数え方を 2 か所に書くと、描画で付けた番号と書き込みで探す番号が食い違い、
// 別のセルを書き換える。** 片方だけのテストでは見えない。
func TestLocate_SharesCounting(t *testing.T) {
	// タスク 3 つ（入れ子を含む）と表 2 つを交互に置く。
	const src = "- [ ] t0\n" +
		"  - [x] t1\n\n" +
		"| a | b |\n| --- | --- |\n| 1 | 2 |\n\n" +
		"> - [ ] t2\n\n" +
		"| x |\n| --- |\n| y |\n"

	locs, err := New().Locate([]byte(src))
	if err != nil {
		t.Fatalf("Locate がエラーを返した: %v", err)
	}
	res, els := renderWithKey(t, src, testRefKey)

	// タスク: 番号 n の項目のラベル（人が書いた期待値）。
	wantLabels := []string{"t0", "t1", "t2"}
	if len(locs.Tasks) != len(wantLabels) {
		t.Fatalf("Tasks が %d 個, want %d 個", len(locs.Tasks), len(wantLabels))
	}
	for n, label := range wantLabels {
		s := locs.Tasks[n]
		if s.Stop-s.Start != 1 || s.Start < 1 || s.Stop >= len(src) {
			t.Fatalf("Tasks[%d] = %+v が括弧の中の 1 文字ではない", n, s)
		}
		// 括弧の中の 1 文字の前後が [ と ]、その後ろが同じ項目のラベル。
		if src[s.Start-1] != '[' || !strings.HasPrefix(src[s.Stop:], "] "+label) {
			t.Errorf("Tasks[%d] が %q の括弧を指していない: %q", n, label, src[s.Start-1:])
		}
	}

	// Render の目印 task:n が、同じラベルの項目のチェックボックスに付いていること。
	// チェックボックスは、直前に始まった li の項目に属する。
	var (
		curLabel  string
		gotLabels = map[string]string{} // data-ref → その項目のラベル
		inputs    int
	)
	for _, e := range els {
		switch e.Tag {
		case "li":
			curLabel = ""
			if f := strings.Fields(e.Text); len(f) > 0 {
				curLabel = f[0]
			}
		case "input":
			inputs++
			v, _ := e.Attr("data-ref")
			gotLabels[v] = curLabel
		}
	}
	if inputs != len(wantLabels) {
		t.Fatalf("input が %d 個, want %d 個\n出力: %s", inputs, len(wantLabels), res.HTML)
	}
	for n, label := range wantLabels {
		key := ref("task:" + strconv.Itoa(n))
		got, ok := gotLabels[key]
		if !ok {
			t.Errorf("目印 %q のチェックボックスが無い\n出力: %s", key, res.HTML)
			continue
		}
		if got != label {
			t.Errorf("目印 %q の項目のラベル = %q, want %q", key, got, label)
		}
	}

	// セル: Tables[t].Cells[r][c] の内容と、Render の cell:t:r:c のセルの文字が
	// 人が書いた期待値と一致する。
	wantCells := []struct {
		tb, r, c int
		want     string
	}{
		{0, 0, 0, "a"}, {0, 0, 1, "b"}, {0, 1, 0, "1"}, {0, 1, 1, "2"},
		{1, 0, 0, "x"}, {1, 1, 0, "y"},
	}
	if len(locs.Tables) != 2 {
		t.Fatalf("Tables が %d 個, want 2 個", len(locs.Tables))
	}

	cellText := map[string]string{} // data-ref → セルの文字
	for _, e := range byTag(els, "th", "td") {
		if v, ok := e.Attr("data-ref"); ok {
			cellText[v] = e.Text
		}
	}
	if len(cellText) != len(wantCells) {
		t.Errorf("目印の付いたセルが %d 個, want %d 個\n出力: %s", len(cellText), len(wantCells), res.HTML)
	}

	for _, w := range wantCells {
		cells := locs.Tables[w.tb].Cells
		if w.r >= len(cells) || w.c >= len(cells[w.r]) || cells[w.r][w.c] == nil {
			t.Errorf("Tables[%d].Cells[%d][%d] が無い", w.tb, w.r, w.c)
		} else {
			span := cells[w.r][w.c].Content
			if got := src[span.Start:span.Stop]; got != w.want {
				t.Errorf("Tables[%d].Cells[%d][%d] の内容 = %q, want %q", w.tb, w.r, w.c, got, w.want)
			}
		}

		key := ref(fmt.Sprintf("cell:%d:%d:%d", w.tb, w.r, w.c))
		if got, ok := cellText[key]; !ok {
			t.Errorf("目印 %q のセルが無い\n出力: %s", key, res.HTML)
		} else if got != w.want {
			t.Errorf("目印 %q のセルの文字 = %q, want %q", key, got, w.want)
		}
	}
}

// TestLocate_Tasks はタスクの括弧の中の位置を検証する
// （UT-218 ケース 2〜7。根拠: FR-141 / IMP-121）。
//
// 位置は正規化後のテキストの上のバイト位置である（IMP-121 の NOTE）。
func TestLocate_Tasks(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Span
	}{
		{"ケース2_基本", "- [ ] a", Span{3, 4}},
		{"ケース3_引用の中", "> - [x] a", Span{5, 6}},
		{"ケース4_マーカーの後の空白が2つ", "-  [ ] a", Span{4, 5}},
		{"ケース4_マーカーの後がタブ", "-\t[x] a", Span{3, 4}},
		// Front Matter を含めたソース上の位置。除いた長さを足し戻す。
		{"ケース5_TOMLのFrontMatter", "+++\na = 1\n+++\n- [ ] a", Span{17, 18}},
		{"ケース6_閉じたYAMLのFrontMatter", "---\na: 1\n---\n- [ ] a", Span{16, 17}},
		// 前処理が先頭に足した 1 バイトを引く（IMP-121）。
		{"ケース7_閉じの無いYAML", "---\n- [ ] a", Span{7, 8}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locs, err := New().Locate([]byte(tt.in))
			if err != nil {
				t.Fatalf("Locate(%q) がエラーを返した: %v", tt.in, err)
			}
			if len(locs.Tasks) != 1 {
				t.Fatalf("Tasks が %d 個, want 1 個: %+v", len(locs.Tasks), locs.Tasks)
			}
			if locs.Tasks[0] != tt.want {
				t.Errorf("Tasks[0] = %+v, want %+v", locs.Tasks[0], tt.want)
			}
		})
	}
}

// TestLocate_Cells はセルの位置を検証する
// （UT-218 ケース 8〜13。根拠: FR-142 / IMP-121）。
func TestLocate_Cells(t *testing.T) {
	locate := func(t *testing.T, src string) [][]*CellSpan {
		t.Helper()
		locs, err := New().Locate([]byte(src))
		if err != nil {
			t.Fatalf("Locate(%q) がエラーを返した: %v", src, err)
		}
		if len(locs.Tables) != 1 {
			t.Fatalf("Tables が %d 個, want 1 個", len(locs.Tables))
		}
		return locs.Tables[0].Cells
	}
	cellAt := func(t *testing.T, cells [][]*CellSpan, r, c int) *CellSpan {
		t.Helper()
		if r >= len(cells) || c >= len(cells[r]) || cells[r][c] == nil {
			t.Fatalf("Cells[%d][%d] が無い: %+v", r, c, cells)
		}
		return cells[r][c]
	}

	t.Run("ケース8_空のセル", func(t *testing.T) {
		// 本体の行は 24 バイト目から。| が 24、空白が 25・26、閉じる | が 27。
		const src = "| a | b |\n| --- | --- |\n|  | x |"
		cell := cellAt(t, locate(t, src), 1, 0)

		if cell.Content != (Span{27, 27}) {
			t.Errorf("Content = %+v, want {27 27}（閉じる縦棒の位置）", cell.Content)
		}
		if cell.Between != (Span{25, 27}) {
			t.Errorf("Between = %+v, want {25 27}", cell.Between)
		}
	})

	t.Run("ケース9_行頭に縦棒が無い行", func(t *testing.T) {
		// 本体の行は 16 バイト目から。1 が 16、区切りの | が 18。
		const src = "a | b\n--- | ---\n1 | 2"
		cell := cellAt(t, locate(t, src), 1, 0)

		if cell.Between.Start != 16 {
			t.Errorf("Between.Start = %d, want 16（行の内容の先頭）", cell.Between.Start)
		}
		if cell.Between != (Span{16, 18}) {
			t.Errorf("Between = %+v, want {16 18}", cell.Between)
		}
		if cell.Content != (Span{16, 17}) {
			t.Errorf("Content = %+v, want {16 17}", cell.Content)
		}
	})

	t.Run("ケース10_エスケープした縦棒を含むセル", func(t *testing.T) {
		const src = "| a |\n| --- |\n| x \\| y |"
		cell := cellAt(t, locate(t, src), 1, 0)

		if got := src[cell.Content.Start:cell.Content.Stop]; got != `x \| y` {
			t.Errorf("Content が指す文字列 = %q, want %q", got, `x \| y`)
		}
	})

	t.Run("ケース11_見出し行より多いセルは位置を持たない", func(t *testing.T) {
		const src = "| a |\n| --- |\n| 1 | 2 |"
		cells := locate(t, src)

		if len(cells) != 2 {
			t.Fatalf("行が %d 行, want 2 行", len(cells))
		}
		if len(cells[1]) != 1 {
			t.Errorf("本体の行の Cells の長さ = %d, want 1（見出し行と同じ）", len(cells[1]))
		}
	})

	t.Run("ケース12_見出し行より少ないセルは nil", func(t *testing.T) {
		const src = "| a | b |\n| --- | --- |\n| 1 |"
		cells := locate(t, src)

		if len(cells) != 2 || len(cells[1]) != 2 {
			t.Fatalf("Cells の形 = %+v, want 2 行 2 列", cells)
		}
		if cells[1][0] == nil {
			t.Error("Cells[1][0] が nil（ソースにあるセル）")
		}
		if cells[1][1] != nil {
			t.Errorf("Cells[1][1] = %+v, want nil（ソースに無いセル）", cells[1][1])
		}
	})

	t.Run("ケース13_引用の中の表", func(t *testing.T) {
		// 本体の行は 18 バイト目から。> が 18、| が 20、1 が 22、閉じる | が 24。
		const src = "> | a |\n> | --- |\n> | 1 |"
		cell := cellAt(t, locate(t, src), 1, 0)

		if cell.Between != (Span{21, 24}) {
			t.Errorf("Between = %+v, want {21 24}（引用記号は外）", cell.Between)
		}
		if cell.Content != (Span{22, 23}) {
			t.Errorf("Content = %+v, want {22 23}", cell.Content)
		}
	})
}

// TestLocate_DoesNotCrash は異常な入力で落ちないことを検証する
// （UT-218 ケース 14。根拠: FR-111 / IMP-022, IMP-121）。
func TestLocate_DoesNotCrash(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"空入力", ""},
		{"1000 段のネストしたリスト", nestedList(1000)},
		{"閉じていないリンク", strings.Repeat("[", 10000)},
		{"閉じていない HTML", strings.Repeat("<div>", 5000)},
		{"不正な UTF-8", "a\xffb\xfe\xfec"},
		{"1 MB の 1 行", strings.Repeat("a", 1<<20)},
		{"1000 列の表", wideTable(1000)},
		{"NUL バイトを含む", "a\x00b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				defer close(done)
				// パニックがここまで漏れれば、テストはその時点で失敗する。
				_, _ = New().Locate([]byte(tt.in))
			}()

			select {
			case <-done:
			case <-time.After(renderLimit()):
				t.Fatalf("Locate が %s 以内に終わらなかった", renderLimit())
			}
		})
	}
}
