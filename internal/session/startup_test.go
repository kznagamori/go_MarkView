package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeFiles は dir 直下に空の Markdown を作る。
func writeFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("# test\n"), 0o600); err != nil {
			t.Fatalf("ファイルを作成できない %s: %v", n, err)
		}
	}
}

// TestFindReadme は README の探索を検証する（UT-802。根拠: FR-013 / IMP-193）。
func TestFindReadme(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		want      string // 期待するファイル名。空文字なら見つからないこと
		linuxOnly bool
	}{
		// UT-802 ケース 1〜4
		{name: "README.md がある", files: []string{"README.md"}, want: "README.md"},
		{name: "readme.md のみ", files: []string{"readme.md"}, want: "readme.md"},
		{name: "README.md と readme.md は完全一致を優先", files: []string{"README.md", "readme.md"}, want: "README.md", linuxOnly: true},
		{name: "Readme.md と readme.md は名前の昇順で先頭", files: []string{"Readme.md", "readme.md"}, want: "Readme.md", linuxOnly: true},

		// UT-802 ケース 5〜7
		{name: "README.txt のみ", files: []string{"README.txt"}, want: ""},
		{name: "Markdown が 1 つもない", files: []string{"a.txt", "b.png"}, want: ""},
		{name: "README.markdown", files: []string{"README.markdown"}, want: "README.markdown"},

		// UT-090 に従って追加した境界値
		{name: "README 以外の Markdown は対象外", files: []string{"index.md"}, want: ""},
		{name: "空のディレクトリ", files: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.linuxOnly && runtime.GOOS == "windows" {
				t.Skip("大文字小文字だけが異なる名前は Windows のファイルシステムに同時に置けない。" +
					"選択規則そのものは TestPickReadme が OS 非依存で検証する")
			}

			dir := t.TempDir()
			writeFiles(t, dir, tt.files...)

			got, ok := FindReadme(dir)

			if tt.want == "" {
				if ok {
					t.Errorf("FindReadme() = %q, ok = true。見つからないことを期待した", got)
				}
				return
			}
			if !ok {
				t.Fatalf("FindReadme() が見つけられなかった。%q を期待した", tt.want)
			}
			if want := filepath.Join(dir, tt.want); got != want {
				t.Errorf("FindReadme() = %q, want %q", got, want)
			}
		})
	}
}

// TestFindReadme_IgnoresDirectory は、README.md という名前のディレクトリを
// 表示対象にしないことを検証する（UT-090 に従って追加した境界値）。
func TestFindReadme_IgnoresDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "README.md"), 0o700); err != nil {
		t.Fatalf("ディレクトリを作成できない: %v", err)
	}

	if got, ok := FindReadme(dir); ok {
		t.Errorf("FindReadme() = %q, ok = true。ディレクトリは対象外にすべき", got)
	}
}

// TestFindReadme_MissingDir は、存在しないディレクトリでも
// エラーではなく「見つからない」として扱うことを検証する（FR-013 の 3）。
func TestFindReadme_MissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nosuch")

	if got, ok := FindReadme(missing); ok {
		t.Errorf("FindReadme() = %q, ok = true。見つからないことを期待した", got)
	}
}

// TestPickReadme は README の選択規則を、ファイルシステムを介さずに検証する
// （UT-802 のケース 3・4 を OS に依存せず確かめるため。UT-041）。
func TestPickReadme(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"大小が混在しても README.md を返す", []string{"readme.md", "README.md", "Readme.md"}, "README.md"},
		{"完全一致がなければ名前の昇順で先頭", []string{"readme.md", "Readme.md"}, "Readme.md"},
		// 昇順では "README.markdown" が先に来る（".m" の次が 'a' < 'd'）。
		// 完全一致の優先規則を外すとここだけが落ちるため、規則を区別できる
		// のはこのケースである。上の 1 件目は昇順だけでも同じ結果になる。
		{"完全一致は拡張子違いより優先", []string{"README.markdown", "README.md"}, "README.md"},
		{"対象拡張子であれば候補になる", []string{"README.mkd"}, "README.mkd"},
		{"README 以外の名前は候補外", []string{"index.md", "CHANGELOG.md"}, ""},
		{"Markdown 以外は候補外", []string{"README.txt", "README"}, ""},
		{"候補なし", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := pickReadme(tt.in)

			if tt.want == "" {
				if ok {
					t.Errorf("pickReadme(%q) = %q, ok = true。候補なしを期待した", tt.in, got)
				}
				return
			}
			if !ok {
				t.Fatalf("pickReadme(%q) が候補を返さなかった。%q を期待した", tt.in, tt.want)
			}
			if got != tt.want {
				t.Errorf("pickReadme(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestResolveStartup_NoArgs は、引数なし起動時の探索順を検証する
// （UT-803 のケース 4〜7。根拠: FR-013 / IMP-193）。
func TestResolveStartup_NoArgs(t *testing.T) {
	tests := []struct {
		name     string
		cwdFiles []string
		exeFiles []string
		wantRoot string // "cwd" または "exe"
		wantFile string // 期待する表示対象のファイル名。空文字なら表示対象なし
	}{
		{
			name:     "カレントに README がある",
			cwdFiles: []string{"README.md"},
			wantRoot: "cwd", wantFile: "README.md",
		},
		{
			name:     "カレントになく実行ファイルの隣にある",
			exeFiles: []string{"README.md"},
			wantRoot: "exe", wantFile: "README.md",
		},
		{
			name:     "どちらにもない",
			wantRoot: "cwd", wantFile: "",
		},
		{
			name:     "両方にある場合はカレントを優先",
			cwdFiles: []string{"README.md"},
			exeFiles: []string{"README.md"},
			wantRoot: "cwd", wantFile: "README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd, exeDir := t.TempDir(), t.TempDir()
			writeFiles(t, cwd, tt.cwdFiles...)
			writeFiles(t, exeDir, tt.exeFiles...)

			got, err := ResolveStartup(nil, cwd, exeDir)
			if err != nil {
				t.Fatalf("ResolveStartup() = %v。引数なしではエラーにならない", err)
			}

			wantRoot := cwd
			if tt.wantRoot == "exe" {
				wantRoot = exeDir
			}
			if got.TreeRoot != wantRoot {
				t.Errorf("TreeRoot = %q, want %q", got.TreeRoot, wantRoot)
			}

			wantInitial := ""
			if tt.wantFile != "" {
				wantInitial = filepath.Join(wantRoot, tt.wantFile)
			}
			if got.Initial != wantInitial {
				t.Errorf("Initial = %q, want %q", got.Initial, wantInitial)
			}
		})
	}
}

// TestResolveStartup_SameDir は、カレントと実行ファイルの場所が同一のときも
// 正しく解決できることを検証する（FR-013 の「探索は 1 回で済ませる」）。
func TestResolveStartup_SameDir(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, "README.md")

	got, err := ResolveStartup(nil, dir, dir)
	if err != nil {
		t.Fatalf("ResolveStartup() = %v", err)
	}
	if got.TreeRoot != dir {
		t.Errorf("TreeRoot = %q, want %q", got.TreeRoot, dir)
	}
	if want := filepath.Join(dir, "README.md"); got.Initial != want {
		t.Errorf("Initial = %q, want %q", got.Initial, want)
	}
}

// TestResolveStartup_WithArg は、引数でパスを指定した場合の解決を検証する
// （UT-803 のケース 1〜3, 8。根拠: FR-012 / IMP-193）。
func TestResolveStartup_WithArg(t *testing.T) {
	t.Run("引数に .md ファイル（親ディレクトリがツリールート）", func(t *testing.T) {
		cwd := t.TempDir()
		sub := filepath.Join(cwd, "docs")
		if err := os.Mkdir(sub, 0o700); err != nil {
			t.Fatalf("ディレクトリを作成できない: %v", err)
		}
		writeFiles(t, sub, "design.md")

		// 相対パスで渡す。プロセスのカレントではなく引数の cwd を基準に
		// 解決されることまで確かめる（UT-035）。
		got, err := ResolveStartup([]string{filepath.Join("docs", "design.md")}, cwd, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveStartup() = %v", err)
		}
		if got.TreeRoot != sub {
			t.Errorf("TreeRoot = %q, want %q", got.TreeRoot, sub)
		}
		if want := filepath.Join(sub, "design.md"); got.Initial != want {
			t.Errorf("Initial = %q, want %q", got.Initial, want)
		}
	})

	t.Run("引数にディレクトリ（README あり）", func(t *testing.T) {
		cwd, target := t.TempDir(), t.TempDir()
		writeFiles(t, target, "README.md")

		got, err := ResolveStartup([]string{target}, cwd, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveStartup() = %v", err)
		}
		if got.TreeRoot != target {
			t.Errorf("TreeRoot = %q, want %q", got.TreeRoot, target)
		}
		if want := filepath.Join(target, "README.md"); got.Initial != want {
			t.Errorf("Initial = %q, want %q", got.Initial, want)
		}
	})

	t.Run("引数にディレクトリ（README なし）", func(t *testing.T) {
		cwd, target := t.TempDir(), t.TempDir()
		writeFiles(t, target, "other.md")

		got, err := ResolveStartup([]string{target}, cwd, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveStartup() = %v", err)
		}
		if got.TreeRoot != target {
			t.Errorf("TreeRoot = %q, want %q", got.TreeRoot, target)
		}
		if got.Initial != "" {
			t.Errorf("Initial = %q, want \"\"（表示対象なし）", got.Initial)
		}
	})

	t.Run("存在しないパスはエラーを返すが起動は継続できる", func(t *testing.T) {
		cwd := t.TempDir()
		missing := filepath.Join(cwd, "nosuch.md")

		got, err := ResolveStartup([]string{missing}, cwd, t.TempDir())
		if err == nil {
			t.Fatal("エラーを期待したが nil だった")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("errors.Is(err, fs.ErrNotExist) が false。err = %v", err)
		}
		// ウィンドウは必ず開くため、ツリールートは埋まっている必要がある（FR-012）。
		if got.TreeRoot != cwd {
			t.Errorf("TreeRoot = %q, want %q。エラー時もツリールートは決まる", got.TreeRoot, cwd)
		}
		if got.Initial != "" {
			t.Errorf("Initial = %q, want \"\"", got.Initial)
		}
	})

	t.Run("引数が 2 つ以上なら 2 つ目以降を無視する", func(t *testing.T) {
		cwd := t.TempDir()
		writeFiles(t, cwd, "first.md", "second.md")

		got, err := ResolveStartup(
			[]string{filepath.Join(cwd, "first.md"), filepath.Join(cwd, "second.md")},
			cwd, t.TempDir(),
		)
		if err != nil {
			t.Fatalf("ResolveStartup() = %v", err)
		}
		if want := filepath.Join(cwd, "first.md"); got.Initial != want {
			t.Errorf("Initial = %q, want %q", got.Initial, want)
		}
	})
}
