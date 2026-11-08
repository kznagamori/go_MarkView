package document

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// TestLoad_FIFO は、開ける拡張子の名前の FIFO を読まずに ErrNotSupported とすることを検証する
// （UT-106 のケース 7。根拠: FR-110 / IMP-102。4.87.0）。Linux でだけ走る（FIFO を作れる OS で、
// この開発ホストでは走らない。CI で見る）。
//
// **読み込みで止まらないことも見る。** FIFO を os.ReadFile で読むと、書き手が現れるまで戻らない。
// 待つのは time.Sleep ではなくチャネルと select（UT-037）。
func TestLoad_FIFO(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"x.md", "x.puml", "x.png"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, name)
			if err := syscall.Mkfifo(p, 0o600); err != nil {
				t.Skipf("FIFO を作れない: %v", err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := Load(renderer.New(), p, LoadOptions{})
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, ErrNotSupported) {
					t.Errorf("エラー = %v, want ErrNotSupported", err)
				}
			case <-time.After(5 * time.Second):
				// 書き手を開いて、止まった読み込みを戻してから失敗にする。
				if w, err := os.OpenFile(p, os.O_WRONLY, 0); err == nil {
					_ = w.Close()
				}
				t.Fatal("5 秒以内に Load が戻らなかった（FIFO を読んでいる）")
			}
		})
	}
}
