package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// waitBuffered は、取り出さないまま通知のチャネルに 1 件たまるのを待つ（IMP-140）。
//
// len でたまったことを確かめる。**バッファの無いチャネルでは len が常に 0 であり、
// 時間内にたまらず失敗する**（v1.0.0 の実装を捕まえる）。time.Sleep で待たず、
// ticker と select で確かめる（UT-037）。
func waitBuffered(t *testing.T, w *Watcher) {
	t.Helper()

	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(waitLimit)
	for {
		select {
		case <-tick.C:
			if len(w.Events()) == 1 {
				return
			}
		case <-deadline:
			t.Fatalf("%s 以内に通知のチャネルに 1 件たまらなかった（len = %d）。バッファ 1 で送っていない（IMP-140）", waitLimit, len(w.Events()))
		}
	}
}

// TestWatch_KeepsLatest は、取り出す前に次のイベントが出たら新しい値で置き換えることを
// 検証する（UT-404 ケース 3。根拠: FR-014 / IMP-140, IMP-142）。
//
// 受け手（IMP-192 の読み直し、IMP-195 の削除）が必要とするのは「いまファイルがあるか」の
// 最新の状態だけである。古い値を先に渡すと、削除された文書を読み直しに行く。
func TestWatch_KeepsLatest(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.md")
	write(t, target, "x")

	w := newWatcher(t)
	if err := w.Watch(target); err != nil {
		t.Fatalf("Watch がエラーを返した: %v", err)
	}

	write(t, target, "xy")
	waitBuffered(t, w) // Modified が 1 件たまった

	if err := os.Remove(target); err != nil {
		t.Fatalf("削除できない: %v", err)
	}
	// 削除のデバウンスが済むまで取り出さない（済む前に取り出すと、置き換える前の
	// Modified を受け取り、上書きしない実装と見分けられない）。expectQuiet と同じく
	// select と time.After で待つ。
	<-time.After(quietLimit)

	ev := waitEvent(t, w)
	if ev.Kind != Removed {
		t.Errorf("Kind = %v, want Removed（古い Modified を置き換えていない）", ev.Kind)
	}
	// 置き換えたのだから、後ろに Modified も Removed も続かない。
	expectQuiet(t, w)
}

// TestWatch_DoesNotBlock は、通知を取り出さないまま書き込みを重ねても Watch が戻ることを
// 検証する（UT-405 ケース 4。根拠: FR-111, NFR-020 / IMP-140, IMP-024）。
//
// **止まる筋書きは fsnotify v1.10.1 の Windows 実装にある。** 監視のゴルーチンが送信で
// 止まると fsnotify のイベントを読まなくなり、送信のバッファ（50 件）が埋まると読み取りの
// ゴルーチンが止まる。Add / Remove も同じゴルーチンが処理するため、Watch が戻らない。
// 文書を開く処理は ioMu を持ったまま Watch を呼びうるため、アプリ全体が止まる（IMP-192）。
func TestWatch_DoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.md")
	other := filepath.Join(dir, "b.md")
	write(t, target, "x")
	write(t, other, "x")

	w := newWatcher(t)
	if err := w.Watch(target); err != nil {
		t.Fatalf("Watch がエラーを返した: %v", err)
	}

	write(t, target, "0")
	// 1 件目の通知がバッファに入った。取り出さない。
	waitBuffered(t, w)

	// 2 件目の通知を出させる。**止まる実装は、バッファが埋まっているため、デバウンスの後に
	// この送信で止まる。** 止まる前に次へ進むと、監視のゴルーチンがまだ fsnotify のイベントを
	// 読んでおり、筋書きを起こせない（壊して確かめて分かった。T15-3）。
	write(t, target, "1")
	<-time.After(quietLimit)

	// fsnotify の送信のバッファ（50 件）を確実に越えるイベントを起こす。止まる実装では
	// 監視のゴルーチンが読まないため、fsnotify の読み取りのゴルーチンが送信で止まる。
	for i := 0; i < 200; i++ {
		write(t, target, "x"+string(rune('a'+i%26)))
	}

	// 別のディレクトリへ切り替え、Add / Remove を読み取りのゴルーチンに頼らせる。
	moved := filepath.Join(t.TempDir(), "c.md")
	write(t, moved, "x")

	done := make(chan error, 1)
	go func() { done <- w.Watch(moved) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Watch がエラーを返した: %v", err)
		}
	case <-time.After(waitLimit):
		t.Fatalf("%s 以内に Watch が戻らなかった（通知の送信で止まっている。IMP-140）", waitLimit)
	}
}
