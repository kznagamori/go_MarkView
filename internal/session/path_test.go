package session

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// touch は中身の小さいファイルを作る。
func touch(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("ディレクトリを作れない: %v", err)
	}
	if err := os.WriteFile(path, []byte("# x\n"), 0o644); err != nil {
		t.Fatalf("ファイルを作れない: %v", err)
	}
}

// symlink はシンボリックリンクを作る。呼ぶのは Linux のケースだけである。
func symlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("シンボリックリンクを作れない: %v", err)
	}
}

// TestSameFile は同じファイルの判定を検証する
// （UT-809。根拠: FR-014, FR-016, FR-140 / IMP-025, IMP-191, IMP-192）。
//
// すべて t.TempDir() の実ファイルで行う。**同じでないことを確かめるケースを先に書く**
// （UT-013）。ケース 5〜7 は「表記を比べるだけ」の実装でも通る。**1〜4 を欠くと
// 「常に真」の実装が通り、別の文書へ切り替えても編集モードが終わらない**（FR-140）。
// **8 を欠くと、リンク経由で開き直しただけで状態が解除される。**
//
// 呼び出し側（IMP-192 が SameDocument を算出すること、状態画面の後は偽にすること）は
// desktop にあり対象外である（UT-002）。見るのは E2E-381 と E2E-388。
func TestSameFile(t *testing.T) {
	sep := string(filepath.Separator)

	tests := []struct {
		name string
		// onlyOS が空でなければ、その OS でだけ実行する。
		onlyOS string
		// setup は dir の下に必要なファイルを作り、比べる 2 つのパスを返す。
		setup func(t *testing.T, dir string) (a, b string)
		want  bool
	}{
		// ケース 1
		{
			name: "同じディレクトリの別のファイル",
			setup: func(t *testing.T, dir string) (string, string) {
				a, b := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
				touch(t, a)
				touch(t, b)
				return a, b
			},
			want: false,
		},
		// ケース 2
		{
			name: "別のディレクトリの同じ名前のファイル",
			setup: func(t *testing.T, dir string) (string, string) {
				a, b := filepath.Join(dir, "x", "a.md"), filepath.Join(dir, "y", "a.md")
				touch(t, a)
				touch(t, b)
				return a, b
			},
			want: false,
		},
		// ケース 3: Linux は大文字小文字を区別する（IMP-025）
		{
			name:   "大文字小文字だけが違う別々に存在するファイル",
			onlyOS: "linux",
			setup: func(t *testing.T, dir string) (string, string) {
				a, b := filepath.Join(dir, "A.md"), filepath.Join(dir, "a.md")
				touch(t, a)
				touch(t, b)
				return a, b
			},
			want: false,
		},
		// ケース 4
		{
			name: "存在するファイルと存在しないファイル",
			setup: func(t *testing.T, dir string) (string, string) {
				a := filepath.Join(dir, "a.md")
				touch(t, a)
				return a, filepath.Join(dir, "c.md")
			},
			want: false,
		},
		// ケース 5
		{
			name: "同じ絶対パス",
			setup: func(t *testing.T, dir string) (string, string) {
				a := filepath.Join(dir, "a.md")
				touch(t, a)
				return a, a
			},
			want: true,
		},
		// ケース 6: filepath.Join は "." を畳むため、文字列で組み立てる
		{
			name: "表記だけが違う（dir/./a.md と dir/a.md）",
			setup: func(t *testing.T, dir string) (string, string) {
				a := filepath.Join(dir, "a.md")
				touch(t, a)
				return dir + sep + "." + sep + "a.md", a
			},
			want: true,
		},
		// ケース 7: Windows は大文字小文字を区別しない（IMP-025）
		{
			name:   "大文字小文字だけが違う表記",
			onlyOS: "windows",
			setup: func(t *testing.T, dir string) (string, string) {
				a := filepath.Join(dir, "a.md")
				touch(t, a)
				return a, filepath.Join(dir, "A.MD")
			},
			want: true,
		},
		// ケース 8: シンボリックリンクを解決した実体で比べる（1.7）
		{
			name:   "実体とそれを指すシンボリックリンク",
			onlyOS: "linux",
			setup: func(t *testing.T, dir string) (string, string) {
				real := filepath.Join(dir, "real", "a.md")
				touch(t, real)
				link := filepath.Join(dir, "link.md")
				symlink(t, real, link)
				return real, link
			},
			want: true,
		},
		// ケース 9
		{
			name:   "同じ実体を指す 2 つのシンボリックリンク",
			onlyOS: "linux",
			setup: func(t *testing.T, dir string) (string, string) {
				real := filepath.Join(dir, "real", "a.md")
				touch(t, real)
				one, two := filepath.Join(dir, "one.md"), filepath.Join(dir, "two.md")
				symlink(t, real, one)
				symlink(t, real, two)
				return one, two
			},
			want: true,
		},
		// ケース 10: 解決に失敗した側は、解決前のパスのまま比べる（IMP-192）。
		// 削除された文書の表記が一致すれば同じとみなす
		{
			name: "存在しないファイルと同じ表記",
			setup: func(t *testing.T, dir string) (string, string) {
				gone := filepath.Join(dir, "gone.md")
				return gone, gone
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.onlyOS != "" && runtime.GOOS != tt.onlyOS {
				t.Skipf("%s でだけ実行する（いまは %s）。Linux のケースは CI の Linux ジョブで実行する（BR-052）", tt.onlyOS, runtime.GOOS)
			}

			a, b := tt.setup(t, t.TempDir())

			if got := SameFile(a, b); got != tt.want {
				t.Errorf("SameFile(%q, %q) = %v, want %v", a, b, got, tt.want)
			}
		})
	}
}
