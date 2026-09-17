package watcher

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// linkedTarget は別のディレクトリにある実体と、それを指すリンクを作る。
//
// 実体は realDir/real.md、リンクは linkDir/link.md とする。**名前を変えておく**のは、
// 実体の名前と比べているか（ケース 4）を区別するためである。
func linkedTarget(t *testing.T) (realDir, real, link string) {
	t.Helper()

	realDir = t.TempDir()
	real = filepath.Join(realDir, "real.md")
	write(t, real, "x")

	link = filepath.Join(t.TempDir(), "link.md")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("シンボリックリンクを作れない: %v", err)
	}
	return realDir, real, link
}

// skipUnlessLinux は Linux 以外で UT-407 を飛ばす。
//
// シンボリックリンクの作成に Windows では権限が要る（UT-602 の IMPORTANT と同じ扱い）。
// **CI の Linux ジョブで必ず走る**（BR-052）。
func skipUnlessLinux(t *testing.T) {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skipf("UT-407 は Linux でだけ実行する（%s ではシンボリックリンクの作成に権限が要る。CI の Linux ジョブで実行する）", runtime.GOOS)
	}
}

// TestWatch_Symlink はシンボリックリンクの実体の監視を検証する
// （UT-407。根拠: FR-014, AR-070 / IMP-141）。
//
// **リンクのパスを Watch に渡す。** リンクの置き場所を監視する実装（v1.0.0）では、
// 実体への書き込みが一度も届かない——外部のエディタで保存しても、編集モードで
// 書き込んでも（IMP-107 は実体を書き換える）表示が更新されない。
func TestWatch_Symlink(t *testing.T) {
	skipUnlessLinux(t)

	// UT-407 ケース 1 と 2
	t.Run("実体への追記が届き、Path はリンクのパス", func(t *testing.T) {
		_, real, link := linkedTarget(t)

		w := newWatcher(t)
		if err := w.Watch(link); err != nil {
			t.Fatalf("Watch がエラーを返した: %v", err)
		}

		write(t, real, "xy")

		ev := waitEvent(t, w)
		// ケース 1
		if ev.Kind != Modified {
			t.Errorf("Kind = %v, want Modified", ev.Kind)
		}
		// ケース 2: 実体のパスを載せると、読み直しでウィンドウタイトルとステータスの
		// パスがリンクから実体へ変わる（IMP-141, IMP-190）
		if ev.Path != link {
			t.Errorf("Path = %q, want %q（Watch に渡したリンクのパス。実体は %q）", ev.Path, link, real)
		}
	})

	// UT-407 ケース 3
	t.Run("実体の置き場所でのリネーム型の保存は Modified", func(t *testing.T) {
		realDir, real, link := linkedTarget(t)

		w := newWatcher(t)
		if err := w.Watch(link); err != nil {
			t.Fatal(err)
		}

		tmp := filepath.Join(realDir, ".real.md.markview-1.tmp")
		write(t, tmp, "saved")
		if err := os.Rename(tmp, real); err != nil {
			t.Fatalf("リネームできない: %v", err)
		}

		if ev := waitEvent(t, w); ev.Kind != Modified {
			t.Errorf("Kind = %v, want Modified（削除と誤認している）", ev.Kind)
		}
	})

	// UT-407 ケース 4
	t.Run("実体のディレクトリにあるリンクと同じ名前のファイルは無視する", func(t *testing.T) {
		realDir, real, link := linkedTarget(t)

		w := newWatcher(t)
		if err := w.Watch(link); err != nil {
			t.Fatal(err)
		}

		// 実体とは違う名前（リンクと同じ link.md）。リンクの名前と比べる実装は、
		// 実体のディレクトリを監視していてもここで届けてしまう（IMP-141）。
		write(t, filepath.Join(realDir, filepath.Base(link)), "not the target")
		expectQuiet(t, w)

		// 上の「届かない」が、監視していないために成り立っただけでないことを見る。
		// 実体への書き込みは届く（ケース 1 と同じ）。
		write(t, real, "changed")
		if ev := waitEvent(t, w); ev.Kind != Modified {
			t.Errorf("実体への書き込みの Kind = %v, want Modified", ev.Kind)
		}
	})
}
