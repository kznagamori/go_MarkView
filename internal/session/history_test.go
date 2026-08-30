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

// mustBack は Back が成功することを前提に 1 つ戻る。
func mustBack(t *testing.T, h *History) Entry {
	t.Helper()

	e, ok := h.Back()
	if !ok {
		t.Fatal("Back できない")
	}
	return e
}

// TestHistory_BackForward は前後移動を検証する
// （UT-804 ケース 1・2。根拠: FR-051 / IMP-191）。
func TestHistory_BackForward(t *testing.T) {
	h := NewHistory()
	pushPaths(h, "a", "b", "c")

	// UT-804 ケース 1: A → B → C を積んで 2 回戻る
	if got := mustBack(t, h); got.Path != "b" {
		t.Errorf("1 回目の Back = %q, want b", got.Path)
	}
	if got := mustBack(t, h); got.Path != "a" {
		t.Errorf("2 回目の Back = %q, want a", got.Path)
	}

	// UT-804 ケース 2: そこから 1 回進む
	got, ok := h.Forward()
	if !ok {
		t.Fatal("Forward できない")
	}
	if got.Path != "b" {
		t.Errorf("Forward = %q, want b", got.Path)
	}
}

// TestHistory_Limits は端での振る舞いを検証する
// （UT-804 ケース 4。根拠: FR-051 / IMP-191）。
//
// **空の履歴で Back を呼んでも落ちない。** 起動直後に Alt+← を押されうる。
func TestHistory_Limits(t *testing.T) {
	t.Run("空の履歴で Back", func(t *testing.T) {
		if got, ok := NewHistory().Back(); ok {
			t.Errorf("Back = %+v, true。want 失敗", got)
		}
	})

	t.Run("空の履歴で Forward", func(t *testing.T) {
		if got, ok := NewHistory().Forward(); ok {
			t.Errorf("Forward = %+v, true。want 失敗", got)
		}
	})

	t.Run("1 件だけの履歴では戻れない", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "a")

		if got, ok := h.Back(); ok {
			t.Errorf("Back = %+v, true。want 失敗（現在位置が先頭）", got)
		}
	})

	t.Run("先頭から先へは戻れない", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "a", "b")

		mustBack(t, h)
		if got, ok := h.Back(); ok {
			t.Errorf("Back = %+v, true。want 失敗", got)
		}
	})

	t.Run("末尾から先へは進めない", func(t *testing.T) {
		h := NewHistory()
		pushPaths(h, "a", "b")

		if got, ok := h.Forward(); ok {
			t.Errorf("Forward = %+v, true。want 失敗", got)
		}
	})

	t.Run("空の履歴で SetScrollTop", func(t *testing.T) {
		// 落ちないことが要求（FR-111）。
		NewHistory().SetScrollTop(100)
	})
}

// TestHistory_PushDiscardsForward は前方履歴の破棄を検証する
// （UT-804 ケース 3。根拠: FR-051 / IMP-191）。
//
// 戻った状態から別の文書を開くと、進む先は失われる。ブラウザと同じ挙動。
func TestHistory_PushDiscardsForward(t *testing.T) {
	h := NewHistory()
	pushPaths(h, "a", "b", "c")

	mustBack(t, h) // b
	mustBack(t, h) // a

	h.Push(Entry{Path: "d"})

	if got, ok := h.Forward(); ok {
		t.Errorf("Forward = %+v, true。want 失敗（前方履歴は破棄される）", got)
	}
	if got := mustBack(t, h); got.Path != "a" {
		t.Errorf("Back = %q, want a", got.Path)
	}
}

// TestHistory_MaxEntries は保持上限を検証する
// （UT-804 ケース 5。根拠: FR-051 / IMP-191）。
//
// 内部の件数を数えるのではなく、**戻れる回数と、戻り着いた先**で確かめる
// （UT-011）。
func TestHistory_MaxEntries(t *testing.T) {
	h := NewHistory()
	for i := range MaxHistory + 1 {
		h.Push(Entry{Path: fmt.Sprintf("p%02d", i)})
	}

	// 上限は 50 件。現在位置は最後に積んだもので、そこから 49 回戻れる。
	for i := range MaxHistory - 1 {
		if _, ok := h.Back(); !ok {
			t.Fatalf("%d 回目の Back ができない。want %d 回", i+1, MaxHistory-1)
		}
	}

	// 最も古い p00 は捨てられ、先頭は p01 になっている。
	if got, ok := h.Forward(); !ok || got.Path != "p02" {
		t.Errorf("Forward = %+v, %v。want p02", got, ok)
	}
	mustBack(t, h)

	if got, ok := h.Back(); ok {
		t.Errorf("さらに Back = %+v, true。want 失敗（p01 が先頭）", got)
	}
}

// TestHistory_ScrollTop はスクロール位置の記録と復元を検証する
// （UT-804 ケース 6。根拠: FR-050, FR-051 / IMP-191）。
//
// リンクをクリックした位置へ戻れることが、この機能の要である。
func TestHistory_ScrollTop(t *testing.T) {
	h := NewHistory()
	h.Push(Entry{Path: "a"})

	// a を 480 px スクロールした位置でリンクを踏み、b を開いた。
	h.SetScrollTop(480)
	h.Push(Entry{Path: "b"})

	got := mustBack(t, h)
	if got.Path != "a" {
		t.Fatalf("Back = %q, want a", got.Path)
	}
	if got.ScrollTop != 480 {
		t.Errorf("ScrollTop = %d, want 480", got.ScrollTop)
	}

	// 進んだ先の位置は書き換わっていない。
	if fwd, ok := h.Forward(); !ok || fwd.ScrollTop != 0 {
		t.Errorf("Forward = %+v, %v。want ScrollTop 0", fwd, ok)
	}
}

// TestHistory_ScrollTop_Middle は、途中の位置に対して記録されることを
// 検証する（UT-804 ケース 6）。
//
// 履歴が 1 件だけだと現在位置が常に先頭になり、「現在位置へ書いているか」を
// 確かめられない。2 件以上ある状態で見る。
func TestHistory_ScrollTop_Middle(t *testing.T) {
	h := NewHistory()
	pushPaths(h, "a", "b")

	h.SetScrollTop(200)
	h.Push(Entry{Path: "c"})

	if got := mustBack(t, h); got.Path != "b" || got.ScrollTop != 200 {
		t.Errorf("1 つ前 = %+v, want b の ScrollTop 200", got)
	}
	if got := mustBack(t, h); got.Path != "a" || got.ScrollTop != 0 {
		t.Errorf("2 つ前 = %+v, want a の ScrollTop 0", got)
	}
}

// TestHistory_Anchor はアンカーが保持されることを検証する（IMP-191）。
func TestHistory_Anchor(t *testing.T) {
	h := NewHistory()
	h.Push(Entry{Path: "a"})
	h.Push(Entry{Path: "b", Anchor: "section-1"})

	mustBack(t, h)
	got, ok := h.Forward()
	if !ok {
		t.Fatal("Forward できない")
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
