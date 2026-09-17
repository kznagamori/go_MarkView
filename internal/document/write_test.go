package document

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// skipUnlessLinux は Linux 以外で t.Skip する（UT-112 の NOTE）。
//
// **Linux の CI で必ず実行する**（BR-052）。開発ホスト（Windows）では走らない。
func skipUnlessLinux(t *testing.T, why string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("Linux でだけ実行する（%s）。CI の Linux ジョブで確かめる", why)
	}
}

// skipIfRoot は root で実行しているときに t.Skip する（UT-112 の NOTE）。
//
// root は権限を無視して書けるため、「書き込めない」ケースが成立しない。
func skipIfRoot(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("root では権限を無視して書けるため、書き込めないことを確かめられない（UT-112 のケース 1・3）")
	}
}

// tempLeftovers は dir に残った一時ファイル（.<名前>.markview- で始まるもの）を返す（IMP-107）。
func tempLeftovers(t *testing.T, dir, name string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ディレクトリを読めない: %v", err)
	}
	var left []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "."+name+".markview-") {
			left = append(left, e.Name())
		}
	}
	return left
}

// countEntries は dir の直下の数を返す。
func countEntries(t *testing.T, dir string) int {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ディレクトリを読めない: %v", err)
	}
	return len(entries)
}

// readString はファイルの内容を返す。
func readString(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ファイルを読めない: %v", err)
	}
	return string(b)
}

// TestReplace_Refuses は、書き込まないことを先に確かめる
// （UT-112 ケース 1〜4・10。根拠: FR-143, NFR-031, NFR-033 / IMP-107）。
//
// **ケース 1 が要である。** ディレクトリに書き込めれば、リネームは読み取り専用の
// ファイルも置き換えてしまう。「書き込めた」だけを見るテストでは、読み取り専用を
// 無視する実装が緑になる。
func TestReplace_Refuses(t *testing.T) {
	// ケース 1・2: Windows は読み取り専用属性（os.Chmod が書き込みビットで切り替える）、
	// Linux は 0444。ディレクトリは書き込める。
	t.Run("1・2: 読み取り専用のファイル", func(t *testing.T) {
		skipIfRoot(t)

		dir := t.TempDir()
		p := writeFile(t, dir, "ro.md", "original\n")
		if err := os.Chmod(p, 0o444); err != nil {
			t.Fatalf("読み取り専用にできない: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

		err := Replace(p, []byte("changed\n"))
		if !errors.Is(err, ErrPermission) {
			t.Errorf("1: エラー = %v, want ErrPermission", err)
		}
		if got := readString(t, p); got != "original\n" {
			t.Errorf("1: 内容が変わった: %q", got)
		}
		if left := tempLeftovers(t, dir, "ro.md"); len(left) != 0 {
			t.Errorf("2: 一時ファイルが残っている: %v", left)
		}
	})

	t.Run("3: ディレクトリに書き込めない", func(t *testing.T) {
		skipUnlessLinux(t, "ディレクトリの権限 0555 は Windows で再現できない")
		skipIfRoot(t)

		dir := filepath.Join(t.TempDir(), "locked")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := writeFile(t, dir, "a.md", "original\n")
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatalf("ディレクトリの権限を落とせない: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

		if err := Replace(p, []byte("changed\n")); !errors.Is(err, ErrPermission) {
			t.Errorf("エラー = %v, want ErrPermission", err)
		}
		if got := readString(t, p); got != "original\n" {
			t.Errorf("元のファイルが変わった: %q", got)
		}
	})

	t.Run("4: 存在しないパス", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "nosuch.md")

		if err := Replace(p, []byte("x")); !errors.Is(err, ErrNotFound) {
			t.Errorf("エラー = %v, want ErrNotFound", err)
		}
		if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("ファイルを作った: Lstat のエラー = %v", err)
		}
		if n := countEntries(t, dir); n != 0 {
			t.Errorf("ディレクトリに %d 個のエントリができた, want 0", n)
		}
	})

	t.Run("10: 宛先の無いシンボリックリンク", func(t *testing.T) {
		skipUnlessLinux(t, "シンボリックリンクの作成に Windows では権限が要る")

		dir := t.TempDir()
		target := filepath.Join(dir, "gone.md")
		link := filepath.Join(dir, "link.md")
		if err := os.Symlink(target, link); err != nil {
			t.Fatalf("シンボリックリンクを作れない: %v", err)
		}

		if err := Replace(link, []byte("x")); !errors.Is(err, ErrNotFound) {
			t.Errorf("エラー = %v, want ErrNotFound", err)
		}
		if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("リンクの宛先にファイルを作った: Lstat のエラー = %v", err)
		}
	})
}

// TestReplace_Writes は置き換えによる書き込みを検証する
// （UT-112 ケース 5〜9。根拠: FR-143, NFR-031, NFR-033 / IMP-107）。
func TestReplace_Writes(t *testing.T) {
	t.Run("5・6: 通常のファイル", func(t *testing.T) {
		dir := t.TempDir()
		p := writeFile(t, dir, "a.md", "- [ ] a\n")
		writeFile(t, dir, "other.md", "x")
		before := countEntries(t, dir)

		if err := Replace(p, []byte("- [x] a\n")); err != nil {
			t.Fatalf("5: Replace がエラーを返した: %v", err)
		}
		if got := readString(t, p); got != "- [x] a\n" {
			t.Errorf("5: 内容 = %q, want %q", got, "- [x] a\n")
		}
		if left := tempLeftovers(t, dir, "a.md"); len(left) != 0 {
			t.Errorf("6: 一時ファイルが残っている: %v", left)
		}
		if n := countEntries(t, dir); n != before {
			t.Errorf("6: ファイルの数 = %d, want %d（書き込みの前と同じ）", n, before)
		}
	})

	t.Run("7: 別のディレクトリの実体を指すシンボリックリンク", func(t *testing.T) {
		skipUnlessLinux(t, "シンボリックリンクの作成に Windows では権限が要る")

		realDir := t.TempDir()
		linkDir := t.TempDir()
		realPath := writeFile(t, realDir, "real.md", "original\n")
		link := filepath.Join(linkDir, "link.md")
		if err := os.Symlink(realPath, link); err != nil {
			t.Fatalf("シンボリックリンクを作れない: %v", err)
		}

		if err := Replace(link, []byte("changed\n")); err != nil {
			t.Fatalf("Replace がエラーを返した: %v", err)
		}
		if got := readString(t, realPath); got != "changed\n" {
			t.Errorf("実体の内容 = %q, want %q", got, "changed\n")
		}
		info, err := os.Lstat(link)
		if err != nil {
			t.Fatalf("リンクを Lstat できない: %v", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("リンクがシンボリックリンクでなくなった: mode = %v", info.Mode())
		}
	})

	t.Run("8: 権限ビットが 0640 のファイル", func(t *testing.T) {
		skipUnlessLinux(t, "権限ビットは Windows で意味を持たない")

		dir := t.TempDir()
		p := writeFile(t, dir, "perm.md", "original\n")
		// umask に左右されないよう、作った後で明示的に設定する。
		if err := os.Chmod(p, 0o640); err != nil {
			t.Fatal(err)
		}

		if err := Replace(p, []byte("changed\n")); err != nil {
			t.Fatalf("Replace がエラーを返した: %v", err)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o640 {
			t.Errorf("権限ビット = %o, want 640", got)
		}
	})

	t.Run("9: 空のバイト列", func(t *testing.T) {
		dir := t.TempDir()
		p := writeFile(t, dir, "empty.md", "original\n")

		if err := Replace(p, []byte{}); err != nil {
			t.Fatalf("Replace がエラーを返した: %v", err)
		}
		if got := readString(t, p); got != "" {
			t.Errorf("内容 = %q, want 空", got)
		}
	})
}
