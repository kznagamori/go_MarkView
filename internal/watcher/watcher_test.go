package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitLimit はイベントを待つ制限時間。
//
// CI の負荷で揺れるため、デバウンス（150 ms）に対して十分な余裕を取る（UT-019）。
const waitLimit = 3 * time.Second

// quietLimit は「イベントが来ない」ことを確かめる待ち時間。
//
// デバウンスの 150 ms を明確に超える長さにする。長くしすぎるとテスト全体が
// 遅くなるため、余裕は 4 倍程度に留める。
const quietLimit = 600 * time.Millisecond

// newWatcher はテスト用の Watcher を作る。終了処理も登録する。
func newWatcher(t *testing.T) *Watcher {
	t.Helper()

	w, err := New(t.Context())
	if err != nil {
		t.Fatalf("New がエラーを返した: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// write はファイルへ書き込む。
func write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("書き込めない: %v", err)
	}
}

// waitEvent はイベントを 1 件待つ。時間内に来なければ失敗させる。
//
// time.Sleep で待たず、チャネルと select で同期する（UT-019, UT-037）。
func waitEvent(t *testing.T, w *Watcher) Event {
	t.Helper()

	select {
	case ev, ok := <-w.Events():
		if !ok {
			t.Fatal("イベントチャネルが閉じている")
		}
		return ev
	case <-time.After(waitLimit):
		t.Fatalf("%s 以内にイベントが届かなかった", waitLimit)
		return Event{}
	}
}

// expectQuiet は一定時間イベントが来ないことを確かめる。
func expectQuiet(t *testing.T, w *Watcher) {
	t.Helper()

	select {
	case ev, ok := <-w.Events():
		if ok {
			t.Errorf("イベントが届いた: %+v", ev)
		}
	case <-time.After(quietLimit):
	}
}

// TestWatch_Modified は変更の検知を検証する
// （UT-401。根拠: FR-014 / IMP-140, IMP-141）。
func TestWatch_Modified(t *testing.T) {
	t.Run("監視対象への追記を検知する", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		write(t, target, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatalf("Watch がエラーを返した: %v", err)
		}

		write(t, target, "xy")

		ev := waitEvent(t, w)
		if ev.Kind != Modified {
			t.Errorf("Kind = %v, want Modified", ev.Kind)
		}
		if !samePath(ev.Path, target) {
			t.Errorf("Path = %q, want %q", ev.Path, target)
		}
	})

	// UT-401 ケース 2: 親ディレクトリを監視しているため届くが、捨てる
	t.Run("同じディレクトリの別ファイルは無視する", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		other := filepath.Join(dir, "b.md")
		write(t, target, "x")
		write(t, other, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}

		write(t, other, "changed")
		expectQuiet(t, w)
	})

	t.Run("別のディレクトリのファイルは無視する", func(t *testing.T) {
		dir, otherDir := t.TempDir(), t.TempDir()
		target := filepath.Join(dir, "a.md")
		other := filepath.Join(otherDir, "a.md")
		write(t, target, "x")
		write(t, other, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}

		write(t, other, "changed")
		expectQuiet(t, w)
	})
}

// TestWatch_Debounce はデバウンスを検証する（UT-402。根拠: FR-014 / IMP-142）。
//
// 書き込みの間隔を作るための待機は time.Sleep で行う。これは結果を待つための
// 同期ではなく、入力そのものを作る操作である（UT-037 が禁じるのは前者）。
func TestWatch_Debounce(t *testing.T) {
	t.Run("短い間隔の連続書き込みは 1 件にまとまる", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		write(t, target, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}

		for i := range 5 {
			write(t, target, string(rune('a'+i)))
			time.Sleep(20 * time.Millisecond)
		}

		if ev := waitEvent(t, w); ev.Kind != Modified {
			t.Errorf("Kind = %v, want Modified", ev.Kind)
		}
		// 2 件目が来ないことまで見る。まとまっていなければここで落ちる。
		expectQuiet(t, w)
	})

	t.Run("間隔が空いた書き込みは別々に届く", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		write(t, target, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}

		write(t, target, "1")
		if ev := waitEvent(t, w); ev.Kind != Modified {
			t.Errorf("1 件目の Kind = %v, want Modified", ev.Kind)
		}

		write(t, target, "2")
		if ev := waitEvent(t, w); ev.Kind != Modified {
			t.Errorf("2 件目の Kind = %v, want Modified", ev.Kind)
		}
	})
}

// TestWatch_RenameSave はリネーム型の保存を検証する
// （UT-403。根拠: FR-014 / IMP-141, IMP-142）。
//
// **エディタの多くはこの方式で保存する。** ファイル単体を監視する実装は、
// ここで監視ハンドルが外れて 2 回目以降を検知できなくなる。
func TestWatch_RenameSave(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.md")
	write(t, target, "x")

	w := newWatcher(t)
	if err := w.Watch(target); err != nil {
		t.Fatal(err)
	}

	// 一時ファイルを作り、監視対象へリネームして置き換える。
	tmp := filepath.Join(dir, "a.md.tmp")
	write(t, tmp, "saved")
	if err := os.Rename(tmp, target); err != nil {
		t.Fatalf("リネームできない: %v", err)
	}

	if ev := waitEvent(t, w); ev.Kind != Modified {
		t.Errorf("Kind = %v, want Modified（削除と誤認している）", ev.Kind)
	}

	// UT-403 ケース 2: 監視が張り直されているか
	write(t, target, "again")
	if ev := waitEvent(t, w); ev.Kind != Modified {
		t.Errorf("2 回目の Kind = %v, want Modified（監視が外れている）", ev.Kind)
	}
}

// TestWatch_Removed は削除の検知を検証する（UT-404。根拠: FR-014 / IMP-142）。
func TestWatch_Removed(t *testing.T) {
	t.Run("削除されたままなら Removed", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		write(t, target, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}

		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}

		if ev := waitEvent(t, w); ev.Kind != Removed {
			t.Errorf("Kind = %v, want Removed", ev.Kind)
		}
	})

	// UT-404 ケース 2: デバウンスの間に復活したら保存とみなす
	t.Run("すぐ再作成されたら Modified", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		write(t, target, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}

		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		write(t, target, "recreated")

		if ev := waitEvent(t, w); ev.Kind != Modified {
			t.Errorf("Kind = %v, want Modified（削除と誤認している）", ev.Kind)
		}
	})
}

// TestWatch_Switch は監視対象の切り替えを検証する
// （UT-405。根拠: NFR-020 / IMP-140）。
//
// 内部のウォッチャ数を数えるのではなく、**解除されたはずの対象から
// イベントが来ないこと**という観測できる振る舞いで確かめる（UT-011）。
func TestWatch_Switch(t *testing.T) {
	t.Run("切り替え前の対象からは届かない", func(t *testing.T) {
		dirA, dirB := t.TempDir(), t.TempDir()
		a := filepath.Join(dirA, "a.md")
		b := filepath.Join(dirB, "b.md")
		write(t, a, "x")
		write(t, b, "x")

		w := newWatcher(t)
		if err := w.Watch(a); err != nil {
			t.Fatal(err)
		}
		if err := w.Watch(b); err != nil {
			t.Fatal(err)
		}

		write(t, a, "changed")
		expectQuiet(t, w)

		write(t, b, "changed")
		if ev := waitEvent(t, w); !samePath(ev.Path, b) {
			t.Errorf("Path = %q, want %q", ev.Path, b)
		}
	})

	// UT-405 ケース 3: 繰り返してもウォッチャが積み上がらない
	t.Run("10 回切り替えても古い対象からは届かない", func(t *testing.T) {
		w := newWatcher(t)

		var files []string
		for range 10 {
			dir := t.TempDir()
			p := filepath.Join(dir, "a.md")
			write(t, p, "x")
			files = append(files, p)

			if err := w.Watch(p); err != nil {
				t.Fatal(err)
			}
		}

		for _, p := range files[:len(files)-1] {
			write(t, p, "changed")
		}
		expectQuiet(t, w)
	})

	t.Run("Unwatch すると届かなくなる", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		write(t, target, "x")

		w := newWatcher(t)
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}
		w.Unwatch()

		write(t, target, "changed")
		expectQuiet(t, w)
	})

	t.Run("同じディレクトリの別ファイルへ切り替える", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "a.md")
		b := filepath.Join(dir, "b.md")
		write(t, a, "x")
		write(t, b, "x")

		w := newWatcher(t)
		if err := w.Watch(a); err != nil {
			t.Fatal(err)
		}
		if err := w.Watch(b); err != nil {
			t.Fatal(err)
		}

		write(t, a, "changed")
		expectQuiet(t, w)

		write(t, b, "changed")
		if ev := waitEvent(t, w); !samePath(ev.Path, b) {
			t.Errorf("Path = %q, want %q", ev.Path, b)
		}
	})
}

// TestWatch_NonexistentDir は存在しないディレクトリの監視を検証する。
func TestWatch_NonexistentDir(t *testing.T) {
	w := newWatcher(t)

	if err := w.Watch(filepath.Join(t.TempDir(), "nosuch", "a.md")); err == nil {
		t.Error("存在しないディレクトリでエラーが返らない")
	}
}

// TestClose は終了処理を検証する（UT-406。根拠: IMP-024 / IMP-140）。
func TestClose(t *testing.T) {
	t.Run("Close でチャネルが閉じる", func(t *testing.T) {
		w, err := New(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		if err := w.Close(); err != nil {
			t.Errorf("Close がエラーを返した: %v", err)
		}
		expectClosed(t, w)
	})

	t.Run("context のキャンセルでチャネルが閉じる", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		w, err := New(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = w.Close() })

		cancel()
		expectClosed(t, w)
	})

	// UT-406 ケース 3: 終了処理が重なっても落ちない
	t.Run("Close を 2 回呼んでも落ちない", func(t *testing.T) {
		w, err := New(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		if err := w.Close(); err != nil {
			t.Errorf("1 回目の Close: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Errorf("2 回目の Close: %v", err)
		}
		expectClosed(t, w)
	})

	t.Run("監視中に Close しても落ちない", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a.md")
		write(t, target, "x")

		w, err := New(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Watch(target); err != nil {
			t.Fatal(err)
		}

		// 送出待ちのイベントを作ってから閉じる。emit が送信で止まっていても
		// 終了できることを見る。
		write(t, target, "changed")
		if err := w.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		expectClosed(t, w)
	})
}

// expectClosed はイベントチャネルが閉じることを確かめる。
//
// 閉じる前に届いたイベントは読み捨てる。Close の直前に発生した変更が
// 流れてくることがあり、それ自体は誤りではない。
func expectClosed(t *testing.T, w *Watcher) {
	t.Helper()

	deadline := time.After(waitLimit)
	for {
		select {
		case _, ok := <-w.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("イベントチャネルが閉じない（ゴルーチンが残っている）")
		}
	}
}
