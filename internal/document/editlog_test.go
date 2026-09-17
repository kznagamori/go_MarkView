package document

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// hashOf はテストで使う要約を作る。要約の値そのものに意味は無く、互いに違えばよい。
func hashOf(s string) [sha256.Size]byte {
	return sha256.Sum256([]byte(s))
}

// samePatch は 2 つの Patch が同じ値かを返す（nil と空のスライスを区別しない）。
func samePatch(a, b Patch) bool {
	return a.Offset == b.Offset && string(a.Old) == string(b.Old) && string(a.New) == string(b.New)
}

// TestEditLog は取り消し履歴を検証する
// （UT-113。根拠: FR-144 / IMP-108）。
//
// **ケース 4 が「取り出す」と「確定する」を分けた理由そのものである。** 書き込みに
// 失敗した取り消しで履歴が動くと、次の Ctrl+Z が 1 つ先を取り消す。
func TestEditLog(t *testing.T) {
	h0, h1, h2 := hashOf("h0"), hashOf("h1"), hashOf("h2")

	// p は括弧の中の空白を x にする書き込み。inv はその逆（人が書いた値。UT-031）。
	p := Patch{Offset: 3, Old: []byte(" "), New: []byte("x")}
	inv := Patch{Offset: 3, Old: []byte("x"), New: []byte(" ")}

	t.Run("1: 作ったばかり", func(t *testing.T) {
		l := NewEditLog(h0)

		if _, ok := l.Undo(); ok {
			t.Error("Undo の ok が真")
		}
		if _, ok := l.Redo(); ok {
			t.Error("Redo の ok が真")
		}
		if l.Known() != h0 {
			t.Error("Known が作るときに渡した値でない")
		}
	})

	t.Run("2: Record の後に Undo", func(t *testing.T) {
		l := NewEditLog(h0)
		l.Record(p, h1)

		got, ok := l.Undo()
		if !ok {
			t.Fatal("Undo の ok が偽")
		}
		if !samePatch(got, inv) {
			t.Errorf("Undo = %+v, want %+v（p の Inverse）", got, inv)
		}
		// Record が Known を after へ進め、Undo は確定していないため動かさない（IMP-108）。
		if l.Known() != h1 {
			t.Error("Known が h1 でない")
		}
	})

	t.Run("3: 2 の後に CommitUndo", func(t *testing.T) {
		l := NewEditLog(h0)
		l.Record(p, h1)
		if _, ok := l.Undo(); !ok {
			t.Fatal("前提: Undo の ok が偽")
		}
		l.CommitUndo(h0)

		if l.Known() != h0 {
			t.Error("Known が h0 でない")
		}
		got, ok := l.Redo()
		if !ok {
			t.Fatal("Redo の ok が偽")
		}
		if !samePatch(got, p) {
			t.Errorf("Redo = %+v, want %+v", got, p)
		}
	})

	t.Run("4: 確定せずにもう一度 Undo", func(t *testing.T) {
		l := NewEditLog(h0)
		l.Record(Patch{Offset: 1, Old: []byte(" "), New: []byte("x")}, h1)
		l.Record(p, h2)

		first, ok := l.Undo()
		if !ok {
			t.Fatal("1 回目の Undo の ok が偽")
		}
		second, ok := l.Undo()
		if !ok {
			t.Fatal("2 回目の Undo の ok が偽")
		}
		if !samePatch(first, inv) || !samePatch(second, inv) {
			t.Errorf("Undo = %+v と %+v, want どちらも %+v（履歴が動いていない）", first, second, inv)
		}
	})

	t.Run("5: 取り消して確定した後に Record", func(t *testing.T) {
		l := NewEditLog(h0)
		l.Record(p, h1)
		if _, ok := l.Undo(); !ok {
			t.Fatal("前提: Undo の ok が偽")
		}
		l.CommitUndo(h0)
		l.Record(Patch{Offset: 5, Old: []byte("1"), New: []byte("2")}, h2)

		if _, ok := l.Redo(); ok {
			t.Error("Redo の ok が真（やり直しの履歴を捨てていない）")
		}
	})

	// ケース 6・7: 上限（MaxUndo = 100）ちょうどと、それを 1 つ超える場合
	undoAll := func(t *testing.T, l *EditLog) []int {
		t.Helper()
		var offsets []int
		for i := 0; i < MaxUndo+5; i++ {
			got, ok := l.Undo()
			if !ok {
				break
			}
			offsets = append(offsets, got.Offset)
			l.CommitUndo(hashOf(fmt.Sprintf("undo-%d", i)))
		}
		return offsets
	}

	t.Run("6: ちょうど 100 回 Record", func(t *testing.T) {
		l := NewEditLog(h0)
		for i := 0; i < 100; i++ {
			l.Record(Patch{Offset: i, Old: []byte(" "), New: []byte("x")}, hashOf(fmt.Sprintf("r-%d", i)))
		}

		offsets := undoAll(t, l)
		if len(offsets) != 100 {
			t.Fatalf("取り出せた回数 = %d, want 100（101 回目は ok が偽）", len(offsets))
		}
		// 新しいものから取り出す
		if offsets[0] != 99 || offsets[99] != 0 {
			t.Errorf("取り出した順 = 先頭 %d / 末尾 %d, want 99 / 0", offsets[0], offsets[99])
		}
	})

	t.Run("7: 101 回 Record", func(t *testing.T) {
		l := NewEditLog(h0)
		for i := 0; i < 101; i++ {
			l.Record(Patch{Offset: i, Old: []byte(" "), New: []byte("x")}, hashOf(fmt.Sprintf("r-%d", i)))
		}

		offsets := undoAll(t, l)
		if len(offsets) != 100 {
			t.Fatalf("取り出せた回数 = %d, want 100", len(offsets))
		}
		for _, off := range offsets {
			if off == 0 {
				t.Error("最初に記録したもの（Offset 0）が取り出せた。古いものから捨てていない")
			}
		}
		if offsets[99] != 1 {
			t.Errorf("最後に取り出したもの = Offset %d, want 1", offsets[99])
		}
	})

	t.Run("8: Redo の後に CommitRedo", func(t *testing.T) {
		l := NewEditLog(h0)
		l.Record(p, h1)
		if _, ok := l.Undo(); !ok {
			t.Fatal("前提: Undo の ok が偽")
		}
		l.CommitUndo(h0)
		if _, ok := l.Redo(); !ok {
			t.Fatal("前提: Redo の ok が偽")
		}
		l.CommitRedo(h2)

		if l.Known() != h2 {
			t.Error("Known が CommitRedo に渡した値でない")
		}
		got, ok := l.Undo()
		if !ok {
			t.Fatal("Undo の ok が偽（再び取り出せない）")
		}
		if !samePatch(got, inv) {
			t.Errorf("Undo = %+v, want %+v", got, inv)
		}
	})
}
