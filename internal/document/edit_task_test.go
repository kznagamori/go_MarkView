package document

import (
	"errors"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// TestPlanTask はチェックボックスの書き換えを検証する
// （UT-109。根拠: FR-141, MD-022 / IMP-106, IMP-121）。
func TestPlanTask(t *testing.T) {
	r := renderer.New()

	tests := []struct {
		name        string
		raw         string
		index       int
		checked     bool
		wantOffset  int
		wantOld     string
		wantNew     string
		wantChanged bool
	}{
		{"1: オフをオンにする", "- [ ] a\n", 0, true, 3, " ", "x", true},
		{"2: x をオフにする", "- [x] a\n", 0, false, 3, "x", " ", true},
		// ケース 3: X をオフにすると半角空白になる（FR-141）
		{"3: X をオフにする", "- [X] a\n", 0, false, 3, "X", " ", true},
		// ケース 6: どちらもオフとみなす（goldmark の \s。IMP-106）
		{"6: 括弧の中がタブ", "- [\t] a\n", 0, true, 3, "\t", "x", true},
		{"6: 括弧の中が改ページ", "- [\f] a\n", 0, true, 3, "\f", "x", true},
		// ケース 7: 引用記号の後ろの、括弧の中を指す
		{"7: 引用の中のリスト", "> - [ ] a\n", 0, true, 5, " ", "x", true},
		// ケース 8: 文書の中の出現順で数える
		{"8: 入れ子の内側", "- [ ] a\n  - [ ] b\n- [ ] c\n", 1, true, 13, " ", "x", true},
		{"8: 入れ子の後の外側", "- [ ] a\n  - [ ] b\n- [ ] c\n", 2, true, 21, " ", "x", true},
		{"8: ゆるいリスト", "- [ ] a\n\n- [ ] b\n", 1, true, 12, " ", "x", true},
		// ケース 9: コードブロックの中を数えない
		{"9: コードブロックの後の本物のタスク", "```\n- [ ] x\n```\n\n- [ ] y\n", 0, true, 20, " ", "x", true},
		// ケース 10: 生 HTML を数えない
		{"10: 生 HTML のチェックボックスの後の本物のタスク", "<input type=\"checkbox\">\n\n- [ ] y\n", 0, true, 28, " ", "x", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, changed, err := PlanTask(r, []byte(tt.raw), Ref{Kind: RefTask, Index: tt.index}, tt.checked)
			if err != nil {
				t.Fatalf("PlanTask がエラーを返した: %v", err)
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

	// ケース 4・5: 反転ではなく「この状態にする」
	unchanged := []struct {
		name    string
		raw     string
		checked bool
	}{
		{"4: X をオンにする（大文字もオンとみなす）", "- [X] a\n", true},
		{"5: x をオンにする", "- [x] a\n", true},
		{"5: 空白をオフにする", "- [ ] a\n", false},
	}

	for _, tt := range unchanged {
		t.Run(tt.name, func(t *testing.T) {
			_, changed, err := PlanTask(r, []byte(tt.raw), Ref{Kind: RefTask, Index: 0}, tt.checked)
			if err != nil {
				t.Fatalf("PlanTask がエラーを返した: %v", err)
			}
			if changed {
				t.Error("changed が真（既にその状態）")
			}
		})
	}

	t.Run("11: 番号がタスクの数を超える", func(t *testing.T) {
		_, _, err := PlanTask(r, []byte("- [ ] a\n- [ ] b\n"), Ref{Kind: RefTask, Index: 2}, true)
		if !errors.Is(err, ErrRefNotFound) {
			t.Errorf("エラー = %v, want ErrRefNotFound", err)
		}
	})

	// ケース 12: タブの字下げで位置がずれる実装を捕まえる。括弧の外を書き換えない（IMP-106）
	tabs := []struct {
		name  string
		raw   string
		index int
	}{
		{"12: - とタブの後の [ ] a", "-\t[ ] a\n", 0},
		{"12: タブ 1 つで字下げした入れ子の内側", "- [ ] a\n\t- [ ] b\n", 1},
	}

	for _, tt := range tabs {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(tt.raw)
			p, _, err := PlanTask(r, raw, Ref{Kind: RefTask, Index: tt.index}, true)
			if err != nil {
				t.Fatalf("PlanTask がエラーを返した: %v", err)
			}
			if p.Offset < 1 || p.Offset+1 >= len(raw) {
				t.Fatalf("Offset = %d が文書の範囲外（len=%d）", p.Offset, len(raw))
			}
			if raw[p.Offset-1] != '[' || raw[p.Offset+1] != ']' {
				t.Errorf("raw[Offset-1] = %q, raw[Offset+1] = %q, want '[' と ']'", raw[p.Offset-1], raw[p.Offset+1])
			}
			if string(p.Old) != " " {
				t.Errorf("Old = %q, want 半角空白", p.Old)
			}
		})
	}
}
