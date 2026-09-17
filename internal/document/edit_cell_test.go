package document

import (
	"errors"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// 表中の縦棒とバックスラッシュ（UT-110 は表のセルを割らないため文字で書いている）。
const (
	pipe = "|"
	bs   = `\`
)

// baseTable は UT-110 の基本の文書。区切り行の後に本体が 2 行。2 行目はセルが 1 つしかない。
const baseTable = "| a | b |\n| --- | --- |\n| 1 | 2 |\n| x |\n"

// bodyCell は本体 1 行目 1 列目（基本の文書の `1`）。
var bodyCell = Ref{Kind: RefCell, Index: 0, Row: 1, Col: 0}

// TestPlanCell はセルの書き換えを検証する
// （UT-110 ケース 1〜14・17〜20。根拠: FR-142, MD-024 / IMP-106, IMP-121）。
//
// **縦棒を含む入力は、表の形を壊す境界である**（ケース 3〜6・12・20）。
func TestPlanCell(t *testing.T) {
	r := renderer.New()

	tests := []struct {
		name        string
		raw         string
		ref         Ref
		text        string
		wantOffset  int
		wantOld     string
		wantNew     string
		wantChanged bool
	}{
		// ケース 1: 前後の空白と区切りの縦棒は Patch に含まれない
		{"1: 本体 1 行目 1 列目を 9 にする", baseTable, bodyCell, "9", 26, "1", "9", true},
		{"2: 前後に空白", baseTable, bodyCell, "  hello  ", 26, "1", "hello", true},
		// ケース 3: 区切りにならないようエスケープする
		{"3: 縦棒を含む", baseTable, bodyCell, "a" + pipe + "b", 26, "1", "a" + bs + pipe + "b", true},
		// ケース 4: 直前の 1 文字が \ なのでそのまま
		{"4: エスケープ済みの縦棒", baseTable, bodyCell, "a" + bs + pipe + "b", 26, "1", "a" + bs + pipe + "b", true},
		// ケース 5: goldmark は \ の個数を数えない（IMP-106）
		{"5: \\ 2 つと縦棒", baseTable, bodyCell, "a" + bs + bs + pipe + "b", 26, "1", "a" + bs + bs + pipe + "b", true},
		// ケース 6: 内容と区切りの間に空白があるので何も足さない
		{"6: 末尾が \\（空白のある表）", baseTable, bodyCell, "abc" + bs, 26, "1", "abc" + bs, true},
		{"7: CRLF を含む", baseTable, bodyCell, "a\r\nb", 26, "1", "a b", true},
		{"7: LF を含む", baseTable, bodyCell, "a\nb", 26, "1", "a b", true},
		{"7: CR を含む", baseTable, bodyCell, "a\rb", 26, "1", "a b", true},
		// ケース 9: 空のセルになる
		{"9: 空文字", baseTable, bodyCell, "", 26, "1", "", true},
		// ケース 10: 区切りの間の空白の 1 文字目の直後へ入り、空白の数が変わらない
		{"10: 空白 2 つの空のセル", "| a | b |\n| --- | --- |\n|  | 2 |\n", bodyCell, "x", 26, "", "x", true},
		// ケース 11: 前の縦棒の直後へ入る
		{"11: 空白の無い空のセル", "| a | b |\n| --- | --- |\n|| 2 |\n", bodyCell, "x", 25, "", "x", true},
		{"13: 見出し行のセル", baseTable, Ref{Kind: RefCell, Index: 0, Row: 0, Col: 0}, "z", 2, "a", "z", true},
		// ケース 14: 空白を詰め直さない
		{"14: 桁揃えの空白を持つセル", "| a | b |\n| --- | --- |\n| 1    | 2 |\n", bodyCell, "9", 26, "1", "9", true},
		{"17: 行頭に縦棒が無い行の最初のセル", "| a | b |\n| --- | --- |\n1 | 2 |\n", bodyCell, "9", 24, "1", "9", true},
		// ケース 18: 引用記号の外の、正しいセル
		{"18: 引用の中の表", "> | a | b |\n> | --- | --- |\n> | 1 | 2 |\n", Ref{Kind: RefCell, Index: 0, Row: 1, Col: 1}, "9", 36, "2", "9", true},
		// ケース 19: 生 HTML を数えない
		{"19: 生 HTML の表の後の GFM の表", "<table><tr><td>h</td></tr></table>\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n", bodyCell, "9", 62, "1", "9", true},
		// ケース 20: \ が区切りの縦棒をエスケープしないよう、末尾に半角空白を 1 つ足す（IMP-106）
		{"20: 空白の無い表で末尾が \\", "|a|b|\n|---|---|\n|1|2|\n", bodyCell, "abc" + bs, 17, "1", "abc" + bs + " ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, changed, err := PlanCell(r, []byte(tt.raw), tt.ref, tt.text)
			if err != nil {
				t.Fatalf("PlanCell がエラーを返した: %v", err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if p.Offset != tt.wantOffset {
				t.Errorf("Offset = %d, want %d", p.Offset, tt.wantOffset)
			}
			if string(p.Old) != tt.wantOld || string(p.New) != tt.wantNew {
				t.Errorf("Old = %q, New = %q, want %q と %q", p.Old, p.New, tt.wantOld, tt.wantNew)
			}
		})
	}

	t.Run("8: 元と同じ内容", func(t *testing.T) {
		_, changed, err := PlanCell(r, []byte(baseTable), bodyCell, "1")
		if err != nil {
			t.Fatalf("PlanCell がエラーを返した: %v", err)
		}
		if changed {
			t.Error("changed が真（内容が変わっていない）")
		}
	})

	// ケース 12: 区切りを足すことになり、表の形を変える
	t.Run("12: 補われたセル", func(t *testing.T) {
		_, _, err := PlanCell(r, []byte(baseTable), Ref{Kind: RefCell, Index: 0, Row: 2, Col: 1}, "y")
		if !errors.Is(err, ErrRefNotFound) {
			t.Errorf("エラー = %v, want ErrRefNotFound", err)
		}
	})

	// ケース 22: 範囲外。panic しない
	outOfRange := []struct {
		name string
		ref  Ref
	}{
		{"22: 表の番号が表の数以上", Ref{Kind: RefCell, Index: 1, Row: 1, Col: 0}},
		{"22: 行が行数以上", Ref{Kind: RefCell, Index: 0, Row: 3, Col: 0}},
		{"22: 列が見出しの列数以上", Ref{Kind: RefCell, Index: 0, Row: 1, Col: 2}},
	}

	for _, tt := range outOfRange {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := PlanCell(r, []byte(baseTable), tt.ref, "9")
			if !errors.Is(err, ErrRefNotFound) {
				t.Errorf("エラー = %v, want ErrRefNotFound", err)
			}
		})
	}
}

// TestCellSource はセルのソースの取得を検証する
// （UT-110 ケース 15・16。根拠: FR-142 / IMP-106, IMP-121）。
func TestCellSource(t *testing.T) {
	r := renderer.New()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		// ケース 15: 記法を含み、前後の空白を除いたもの
		{"15: 太字のセル", "| a | b |\n| --- | --- |\n| **太字** | 2 |\n", "**太字**"},
		// ケース 16: \ を含んだまま返す（IMP-121）
		{"16: エスケープした縦棒を含むセル", "| a | b |\n| --- | --- |\n| a " + bs + pipe + " b | 2 |\n", "a " + bs + pipe + " b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CellSource(r, []byte(tt.raw), bodyCell)
			if err != nil {
				t.Fatalf("CellSource がエラーを返した: %v", err)
			}
			if got != tt.want {
				t.Errorf("CellSource = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPlanCell_KeepsTableShape は、書き換えた結果をパーサに通して表の形が
// 変わっていないことを確かめる（UT-110 ケース 21。根拠: FR-142 / IMP-106, IMP-121）。
//
// **エスケープの規則を手で書いても、パーサと食い違えば形は変わる。** 書き換えた
// 結果を renderer.Locate で解き直すのが、規則の誤りを捕まえる唯一の手段である。
// 入力はどれも LF だけで BOM も無いため、生バイト列と正規化後の位置は一致する。
func TestPlanCell_KeepsTableShape(t *testing.T) {
	r := renderer.New()

	tests := []struct {
		name string
		raw  string
		text string
	}{
		{"3: 縦棒を含む", baseTable, "a" + pipe + "b"},
		{"4: エスケープ済みの縦棒", baseTable, "a" + bs + pipe + "b"},
		{"5: \\ 2 つと縦棒", baseTable, "a" + bs + bs + pipe + "b"},
		{"6: 末尾が \\（空白のある表）", baseTable, "abc" + bs},
		{"10: 空白 2 つの空のセル", "| a | b |\n| --- | --- |\n|  | 2 |\n", "x"},
		{"11: 空白の無い空のセル", "| a | b |\n| --- | --- |\n|| 2 |\n", "x"},
		{"20: 空白の無い表で末尾が \\", "|a|b|\n|---|---|\n|1|2|\n", "abc" + bs},
	}

	for _, tt := range tests {
		t.Run("21: "+tt.name, func(t *testing.T) {
			raw := []byte(tt.raw)
			p, _, err := PlanCell(r, raw, bodyCell, tt.text)
			if err != nil {
				t.Fatalf("PlanCell がエラーを返した: %v", err)
			}
			after := splice(t, raw, p)

			before, err := r.Locate(raw)
			if err != nil {
				t.Fatalf("書き換える前の Locate がエラーを返した: %v", err)
			}
			got, err := r.Locate(after)
			if err != nil {
				t.Fatalf("書き換えた後の Locate がエラーを返した: %v", err)
			}

			if len(before.Tables) != 1 {
				t.Fatalf("書き換える前の表の数 = %d, want 1（テストの前提）", len(before.Tables))
			}
			if len(got.Tables) != len(before.Tables) {
				t.Fatalf("表の数 = %d, want %d", len(got.Tables), len(before.Tables))
			}

			bc, gc := before.Tables[0].Cells, got.Tables[0].Cells
			if len(gc) != len(bc) {
				t.Fatalf("行数 = %d, want %d", len(gc), len(bc))
			}
			for row := range bc {
				if len(gc[row]) != len(bc[row]) {
					t.Errorf("%d 行目のセルの数 = %d, want %d", row, len(gc[row]), len(bc[row]))
					continue
				}
				for col := range bc[row] {
					if row == bodyCell.Row && col == bodyCell.Col {
						continue // 書き換えたセル
					}
					b, g := bc[row][col], gc[row][col]
					if (b == nil) != (g == nil) {
						t.Errorf("[%d][%d] の有無が変わった: before=%v after=%v", row, col, b != nil, g != nil)
						continue
					}
					if b == nil {
						continue
					}
					bText := string(raw[b.Content.Start:b.Content.Stop])
					gText := string(after[g.Content.Start:g.Content.Stop])
					if gText != bText {
						t.Errorf("[%d][%d] の内容 = %q, want %q（書き換えていないセルが変わった）", row, col, gText, bText)
					}
				}
			}
		})
	}
}
