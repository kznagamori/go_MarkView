package session

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
)

// pushPaths は path だけを持つエントリを順に積む。
func pushPaths(h *History, paths ...string) {
	for _, p := range paths {
		h.Push(Entry{Path: p})
	}
}

// peek は delta 先の項目の Path を返す。無ければ "-"。
//
// **現在位置は Peek(-1) と Peek(+1) の結果で観測する**（UT-011。UT-804 の前置き）。
func peek(h *History, delta int) string {
	e, ok := h.Peek(delta)
	if !ok {
		return "-"
	}
	return e.Path
}

// wantAround は Peek(-1) / Peek(+1) / CanBack / CanForward をまとめて確かめる。
// back / fwd の "-" は ok が偽であること。
func wantAround(t *testing.T, h *History, back, fwd string) {
	t.Helper()

	if got := peek(h, -1); got != back {
		t.Errorf("Peek(-1) = %q, want %q", got, back)
	}
	if got := peek(h, +1); got != fwd {
		t.Errorf("Peek(+1) = %q, want %q", got, fwd)
	}
	if got, want := h.CanBack(), back != "-"; got != want {
		t.Errorf("CanBack = %v, want %v", got, want)
	}
	if got, want := h.CanForward(), fwd != "-"; got != want {
		t.Errorf("CanForward = %v, want %v", got, want)
	}
}

// TestHistory は戻る・進むの履歴を検証する（UT-804 のケース 1〜12。根拠: FR-051, UI-021 / IMP-191, IMP-323。
// BUG-023）。
//
// **v1.2.0 で、戻る・進むを「位置を動かさずに見る（Peek）→ 開く → 開けたら動かす（Step）/
// 開けなければ取り除く（Drop）」に分けた**（4.75.0〜4.76.0。v1.1.0 の Back / Forward は開く前に
// 位置を動かしていた）。「A・B・C で C にいる」は、A → B → C の順に Push した直後をいう。
//
// **ケース 12 が BUG-023 そのものである。** **ケース 2 と 8 も同じ誤りを捕まえる**——Peek が位置を
// 動かす実装は 2 で、Drop の後に位置をずらさない実装は 8 で落ちる。
func TestHistory(t *testing.T) {
	// ケース 1: 空の履歴（起動直後に Alt+← を押されうる）。panic しない。
	t.Run("1 空の履歴", func(t *testing.T) {
		h := NewHistory()
		wantAround(t, h, "-", "-")
		h.Drop(-1)
		h.Drop(0)
		h.SetScrollTop(100) // 落ちないことが要求（FR-111）
		wantAround(t, h, "-", "-")
	})

	// ケース 2: Peek は位置を動かさない。
	t.Run("2 Peekは位置を動かさない", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C")
		if a, b := peek(h, -1), peek(h, -1); a != "B" || b != "B" {
			t.Errorf("Peek(-1) を 2 回 = %q, %q, want B, B", a, b)
		}
		wantAround(t, h, "B", "-")
	})

	// ケース 3・4: Step(-1) で位置が動く。
	t.Run("3・4 Step", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C")
		h.Step(-1)
		wantAround(t, h, "A", "C") // 3: 位置が B へ動いた
		h.Step(-1)
		wantAround(t, h, "-", "B") // 4: 位置が A
	})

	// ケース 5: B にいるときに D を Push すると、前方の C を捨てる。
	t.Run("5 Pushは前方を捨てる", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C")
		h.Step(-1)
		h.Push(Entry{Path: "D"})
		wantAround(t, h, "B", "-")
	})

	// ケース 6: 保持は 50 件。捨てるのは最も古い P1（件数だけを見ると、51 件目を積まない実装や
	// 新しい方を捨てる実装が通る。4.78.0）。
	t.Run("6 保持は50件", func(t *testing.T) {
		h := NewHistory()
		for i := 1; i <= MaxHistory+1; i++ {
			h.Push(Entry{Path: fmt.Sprintf("P%d", i)})
		}
		if got := peek(h, -1); got != "P50" {
			t.Fatalf("積んだ直後の Peek(-1) = %q, want P50", got)
		}
		for i := 1; i <= 49; i++ {
			if got, want := peek(h, -1), fmt.Sprintf("P%d", 51-i); got != want {
				t.Fatalf("%d 回目の Peek(-1) = %q, want %q", i, got, want)
			}
			if i == 49 {
				break
			}
			h.Step(-1)
			if i == 48 {
				if got := peek(h, -1); got != "P2" {
					t.Errorf("48 回戻った後の Peek(-1) = %q, want P2", got)
				}
			}
		}
		h.Step(-1) // 49 回目
		if got := peek(h, -1); got != "-" {
			t.Errorf("50 回目の Peek(-1) = %q, want ok が偽（P1 は捨てた）", got)
		}
	})

	// ケース 7: Push の前に現在位置のスクロール位置を記録している（FR-050）。
	t.Run("7 スクロール位置", func(t *testing.T) {
		h := NewHistory()
		h.Push(Entry{Path: "A"})
		h.SetScrollTop(120)
		h.Push(Entry{Path: "B"})
		e, ok := h.Peek(-1)
		if !ok || e.Path != "A" || e.ScrollTop != 120 {
			t.Errorf("Peek(-1) = %+v, %v, want A の ScrollTop 120", e, ok)
		}
	})

	// ケース 8: 位置より前を取り除いたら、位置を 1 つ前へずらして同じ文書を指す。
	// C を末尾に置かない——末尾だと、範囲外になった位置を末尾へ寄せる実装と見分けられない。
	t.Run("8 位置より前をDrop", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C", "D")
		h.Step(-1) // C
		h.Drop(-1) // B を取り除く
		wantAround(t, h, "A", "D")
	})

	// ケース 9: 位置より後ろを取り除いても、位置は B のまま。続けて X を Push すると Peek(-1) が B
	// （今いる B を誤って取り除く実装を捕まえる。ケース 12 と対。4.78.0）。
	t.Run("9 位置より後ろをDrop", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C")
		h.Step(-1) // B
		h.Drop(+1) // C を取り除く
		wantAround(t, h, "A", "-")
		h.Push(Entry{Path: "X"})
		if got := peek(h, -1); got != "B" {
			t.Errorf("Push の後の Peek(-1) = %q, want B", got)
		}
	})

	// ケース 10: Drop(0) は位置の項目を取り除く。先頭なら位置は 0（App はこの使い方をしない境界）。
	t.Run("10 Drop(0)", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C")
		h.Step(-1) // B
		h.Drop(0)  // B を取り除き、位置が A
		wantAround(t, h, "-", "C")

		h2 := NewHistory()
		pushPaths(h2, "A", "B", "C")
		h2.Step(-1)
		h2.Step(-1) // A
		h2.Drop(0)  // A を取り除き、位置が B
		wantAround(t, h2, "-", "C")
		if e, ok := h2.Peek(+1); !ok || e.Path != "C" {
			t.Errorf("位置が B でない: Peek(+1) = %+v, %v", e, ok)
		}
	})

	// ケース 11: 範囲外の delta は何もしない。panic しない。
	t.Run("11 範囲外のDrop", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C")
		h.Step(-1)
		h.Step(-1) // A
		h.Drop(-1)
		wantAround(t, h, "-", "B")

		h2 := NewHistory()
		pushPaths(h2, "A", "B", "C") // C
		h2.Drop(+1)
		h2.Drop(+5)
		wantAround(t, h2, "B", "-")
	})

	// ケース 12: A・B・C で C にいて、B を開けなかったとして Drop(-1)、続けて D を Push すると、
	// 見ていた C が履歴に残る（BUG-023 の 2 つ目の症状）。
	t.Run("12 BUG-023", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B", "C")
		if got := peek(h, -1); got != "B" {
			t.Fatalf("前提: Peek(-1) = %q, want B", got)
		}
		h.Drop(-1) // B を開けなかった
		h.Push(Entry{Path: "D"})
		if got := peek(h, -1); got != "C" {
			t.Errorf("Peek(-1) = %q, want C（見ていた C が履歴に残る）", got)
		}
		h.Step(-1)
		if got := peek(h, -1); got != "A" {
			t.Errorf("Step(-1) の後の Peek(-1) = %q, want A", got)
		}
	})

	// ケース 13: 1 件だけの履歴で Drop(0) すると、空の履歴と同じになる（4.88.0）。位置が空の履歴を
	// 指したままだと、SetScrollTop が範囲の外を書いて panic する。
	t.Run("13 1件だけの履歴でDrop(0)", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A")
		h.Drop(0)
		wantAround(t, h, "-", "-")
		h.SetScrollTop(100) // 落ちないことが要求（FR-111）
		h.Push(Entry{Path: "B"})
		wantAround(t, h, "-", "-")
	})

	// ケース 14: 範囲外の delta の Step は何もしない（Drop と同じ。4.88.0）。panic しない。
	t.Run("14 範囲外のStep", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "A", "B") // B
		h.Step(+1)
		wantAround(t, h, "A", "-")
		h.Step(-5)
		wantAround(t, h, "A", "-")
		h.SetScrollTop(100)

		h2 := NewHistory()
		h2.Step(-1)
		wantAround(t, h2, "-", "-")
	})
}

// TestHistory_Anchor はアンカーが保持されることを検証する（IMP-191）。
func TestHistory_Anchor(t *testing.T) {
	h := NewHistory()
	h.Push(Entry{Path: "a"})
	h.Push(Entry{Path: "b", Anchor: "section-1"})
	h.Push(Entry{Path: "c"})

	got, ok := h.Peek(-1)
	if !ok {
		t.Fatal("Peek(-1) できない")
	}
	if got.Anchor != "section-1" {
		t.Errorf("Anchor = %q, want section-1", got.Anchor)
	}
}

// TestDisplayPath は表示用パスの算出を検証する
// （UT-805。根拠: UI-060, FR-052 / IMP-025）。
//
// 比較はスラッシュ区切りに揃える。区切り文字の違いを持ち込まないため（UT-035）。
func TestDisplayPath(t *testing.T) {
	tests := []struct {
		name        string
		root        string
		target      string
		want        string
		wantOutside bool
	}{
		// UT-805 ケース 3: ツリールート直下
		{"直下のファイル", "/r", "/r/a.md", "a.md", false},

		// UT-805 ケース 1: 下位ディレクトリ
		{"下位ディレクトリ", "/r", "/r/docs/a.md", "docs/a.md", false},
		{"深い階層", "/r", "/r/a/b/c.md", "a/b/c.md", false},

		// UT-805 ケース 2: ツリー外
		{"別のディレクトリ", "/r", "/other/a.md", "/other/a.md", true},
		{"ルートの親", "/r/sub", "/r/a.md", "/r/a.md", true},

		// UT-805 ケース 4: ツリールート未確定
		{"ルートが空", "", "/any/a.md", "/any/a.md", false},

		// UT-805 ケース 6: 名前の頭が同じ兄弟（文字列の前方一致で相対パスにしない。4.78.0）
		{"名前の頭が同じ兄弟", "/r", "/r2/a.md", "/r2/a.md", true},

		// UT-090 に従って追加
		{"冗長な区切り", "/r/", "/r/./docs/a.md", "docs/a.md", false},
		{"ルートそのもの", "/r", "/r", ".", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, outside := DisplayPath(tt.root, tt.target)

			if filepath.ToSlash(got) != tt.want {
				t.Errorf("DisplayPath(%q, %q) = %q, want %q", tt.root, tt.target, got, tt.want)
			}
			if outside != tt.wantOutside {
				t.Errorf("ツリー外の判定 = %v, want %v", outside, tt.wantOutside)
			}
		})
	}
}

// TestDisplayPath_CaseInsensitiveRoot は大文字小文字だけが異なるルートの扱いを
// 検証する（UT-805 ケース 5。根拠: IMP-025）。
//
// 期待値が OS で異なるのは仕様どおりである。Windows のパス比較は大文字小文字を
// 区別せず、Linux は区別する。
func TestDisplayPath_CaseInsensitiveRoot(t *testing.T) {
	got, outside := DisplayPath("/R", "/r/docs/a.md")

	if runtime.GOOS == "windows" {
		if outside {
			t.Error("Windows では同じツリー内として扱うはず")
		}
		if filepath.ToSlash(got) != "docs/a.md" {
			t.Errorf("DisplayPath = %q, want docs/a.md", got)
		}
		return
	}

	if !outside {
		t.Error("Linux では別のディレクトリとして扱うはず")
	}
}
