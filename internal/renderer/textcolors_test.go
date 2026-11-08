package renderer

import (
	"regexp"
	"testing"
)

// TestTextColors は文字色の一覧の健全性を検証する
// （UT-221。根拠: FR-147, MD-072, FR-070, UI-105 / IMP-116, IMP-123, IMP-124）。
//
// 期待値は FR-147 の表から人が書いた（UT-031）。**同じ値を、フロントエンドの一覧（IMP-265）と
// CSS のトークン（DSP-010）にも書く**（IMP-123）。3 か所の一致は単体テストでは見えない。
// ここで固定するのは、サニタイズ（UT-209 のケース 27）と書き込み（UT-120）が使う Go 側の値である。
func TestTextColors(t *testing.T) {
	// ケース 1: 並びと値（名前・Light・Dark）。FR-147 の表の並び。
	t.Run("1 並びと値", func(t *testing.T) {
		want := []struct{ Name, Light, Dark string }{
			{"red", "#d1242f", "#f85149"},
			{"orange", "#bc4c00", "#db6d28"},
			{"yellow", "#9a6700", "#d29922"},
			{"green", "#1a7f37", "#3fb950"},
			{"blue", "#0969da", "#4493f8"},
			{"purple", "#8250df", "#ab7df8"},
			{"gray", "#59636e", "#9198a1"},
		}
		if len(TextColors) != len(want) {
			t.Fatalf("TextColors が %d 色, want %d 色: %v", len(TextColors), len(want), TextColors)
		}
		for i, w := range want {
			if TextColors[i] != w {
				t.Errorf("TextColors[%d] = %+v, want %+v", i, TextColors[i], w)
			}
		}
	})

	// ケース 2: 名前はすべて小文字で、重ならない。default を含まない（色を外す指示。IMP-124）。
	t.Run("2 名前", func(t *testing.T) {
		if len(TextColors) == 0 {
			t.Fatal("TextColors が空")
		}
		lower := regexp.MustCompile(`^[a-z]+$`)
		seen := map[string]bool{}
		for _, c := range TextColors {
			if !lower.MatchString(c.Name) {
				t.Errorf("名前 %q が小文字の英字だけではない", c.Name)
			}
			if seen[c.Name] {
				t.Errorf("名前 %q が重なっている", c.Name)
			}
			seen[c.Name] = true
		}
		if seen["default"] {
			t.Error("default が一覧に入っている（色を外す指示であり、色ではない）")
		}
	})

	// ケース 3: Light と Dark の値は # と小文字の 16 進 6 桁（ファイルに書く形。FR-147）。
	t.Run("3 値の形", func(t *testing.T) {
		if len(TextColors) == 0 {
			t.Fatal("TextColors が空")
		}
		hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
		for _, c := range TextColors {
			for _, v := range []string{c.Light, c.Dark} {
				if !hex.MatchString(v) {
					t.Errorf("%s の値 %q が # と小文字の 16 進 6 桁ではない", c.Name, v)
				}
			}
		}
	})
}
