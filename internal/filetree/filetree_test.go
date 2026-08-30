package filetree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// mkTree は t.TempDir() の下に構造を作り、その根を返す。
//
// 末尾が / の要素はディレクトリ、それ以外は空のファイルとして作る。
func mkTree(t *testing.T, spec ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, s := range spec {
		p := filepath.Join(root, filepath.FromSlash(strings.TrimSuffix(s, "/")))

		if strings.HasSuffix(s, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatalf("ディレクトリを作れない: %v", err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("親ディレクトリを作れない: %v", err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("ファイルを作れない: %v", err)
		}
	}
	return root
}

// names は Node の名前だけを取り出す。
func names(nodes []Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}

func readDir(t *testing.T, dir string) []Node {
	t.Helper()

	nodes, err := ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) がエラーを返した: %v", dir, err)
	}
	return nodes
}

// TestReadDir_Filter は絞り込み規則を検証する
// （UT-301。根拠: FR-031 / IMP-132）。
func TestReadDir_Filter(t *testing.T) {
	tests := []struct {
		name string
		tree []string
		want []string
	}{
		// UT-301 ケース 7: 空のディレクトリ（境界値を先に。UT-013）
		{"空のディレクトリ", nil, []string{}},

		// UT-301 ケース 1: Markdown 以外のファイルを除く
		{"Markdown 以外を除く", []string{"a.md", "b.txt", "c.png"}, []string{"a.md"}},

		// UT-301 ケース 2: ドットで始まる名前
		{"ドットで始まるファイル", []string{"a.md", ".hidden.md"}, []string{"a.md"}},
		{"ドットで始まるディレクトリ", []string{"a.md", ".config/x.md"}, []string{"a.md"}},

		// UT-301 ケース 3・4: 既定で除外するディレクトリ
		{"除外ディレクトリ", []string{
			"a.md",
			".git/x.md", "node_modules/x.md", "vendor/x.md",
			"dist/x.md", "build/x.md", "target/x.md",
		}, []string{"a.md"}},

		// UT-301 ケース 5: Markdown を含むディレクトリ
		{"Markdown を含むディレクトリ", []string{"docs/x.md"}, []string{"docs"}},

		// UT-301 ケース 6: 拡張子の大文字小文字
		{"大文字の拡張子", []string{"A.MD"}, []string{"A.MD"}},
		{"その他の対応拡張子", []string{"a.markdown", "b.mdown", "c.mkd"}, []string{"a.markdown", "b.mdown", "c.mkd"}},

		// FR-031: Markdown を含まないディレクトリは表示しない
		{"Markdown を含まないディレクトリ", []string{"a.md", "img/x.png"}, []string{"a.md"}},
		{"空のディレクトリは表示しない", []string{"a.md", "empty/"}, []string{"a.md"}},

		// IMP-133: 1 階層下に Markdown があれば表示する
		{"1 階層下に Markdown", []string{"docs/sub/x.md"}, []string{"docs"}},
		// 2 階層下は判定できないため表示する（IMP-133 が許容する状態）
		{"2 階層下は判定できないので表示する", []string{"docs/a/b/x.md"}, []string{"docs"}},
		// 除外対象しか含まないディレクトリは表示しない
		{"除外対象しか含まないディレクトリ", []string{"a.md", "pkg/node_modules/x.md"}, []string{"a.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := names(readDir(t, mkTree(t, tt.tree...)))

			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("結果 = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestReadDir_Order は並び順を検証する（UT-302。根拠: FR-031 / IMP-132）。
func TestReadDir_Order(t *testing.T) {
	tests := []struct {
		name string
		tree []string
		want []string
	}{
		// UT-302 ケース 1: ディレクトリ優先、名前昇順
		{
			"ディレクトリ優先",
			[]string{"b.md", "a.md", "dir2/x.md", "dir1/x.md"},
			[]string{"dir1", "dir2", "a.md", "b.md"},
		},

		// UT-302 ケース 2: 大文字小文字を区別しない比較
		{"大文字小文字を区別しない", []string{"B.md", "a.md"}, []string{"a.md", "B.md"}},
		{"ディレクトリも同様", []string{"B/x.md", "a/x.md"}, []string{"a", "B"}},

		// UT-302 ケース 3: 辞書順（自然順ソートはしない）
		{"数字は辞書順", []string{"10.md", "2.md"}, []string{"10.md", "2.md"}},

		// UT-090 に従って追加
		{"日本語のファイル名", []string{"い.md", "あ.md"}, []string{"あ.md", "い.md"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := names(readDir(t, mkTree(t, tt.tree...)))

			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("並び = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestReadDir_Truncation は件数の上限を検証する
// （UT-303。根拠: FR-032 / IMP-131）。
//
// **上限ちょうどは切り詰めではない。**
func TestReadDir_Truncation(t *testing.T) {
	tests := []struct {
		name          string
		count         int
		wantLen       int
		wantTruncated bool
	}{
		{"上限の 1 つ下", MaxEntriesPerDir - 1, MaxEntriesPerDir - 1, false},
		{"上限ちょうど", MaxEntriesPerDir, MaxEntriesPerDir, false},
		{"上限を 1 つ超える", MaxEntriesPerDir + 1, MaxEntriesPerDir, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := make([]string, tt.count)
			for i := range spec {
				spec[i] = fmt.Sprintf("f%05d.md", i)
			}

			nodes := readDir(t, mkTree(t, spec...))

			if len(nodes) != tt.wantLen {
				t.Errorf("件数 = %d, want %d", len(nodes), tt.wantLen)
			}
			for i, n := range nodes {
				if n.Truncated != tt.wantTruncated {
					t.Errorf("%d 件目の Truncated = %v, want %v", i, n.Truncated, tt.wantTruncated)
					break
				}
			}
		})
	}
}

// TestReadDir_NotRecursive は再帰しないことを検証する
// （UT-304。根拠: FR-032 / IMP-131）。
//
// 深い階層を作って実行時間を測る方法は環境に依存するため採らない（UT-035）。
// **構造で確認する。**
func TestReadDir_NotRecursive(t *testing.T) {
	root := mkTree(t, "a.md", "sub/b.md")
	nodes := readDir(t, root)

	// UT-304 ケース 1: 下位のファイルを含まない
	for _, n := range nodes {
		if n.Name == "b.md" {
			t.Error("下位ディレクトリのファイルが含まれている")
		}
	}

	// UT-304 ケース 2: ディレクトリの子は未読込
	var sub *Node
	for i := range nodes {
		if nodes[i].Name == "sub" {
			sub = &nodes[i]
		}
	}
	if sub == nil {
		t.Fatalf("sub が結果にない: %v", names(nodes))
	}
	if sub.Children != nil {
		t.Errorf("sub.Children = %v, want nil（未読込）", sub.Children)
	}
	if sub.Loaded {
		t.Error("sub.Loaded = true, want false（未読込）")
	}
}

// TestReadDir_Path はノードのパスが絶対であることを検証する（IMP-025）。
func TestReadDir_Path(t *testing.T) {
	root := mkTree(t, "a.md", "sub/b.md")

	for _, n := range readDir(t, root) {
		if !filepath.IsAbs(n.Path) {
			t.Errorf("%q の Path = %q, 絶対パスでない", n.Name, n.Path)
		}
		if filepath.Dir(n.Path) != root {
			t.Errorf("%q の Path = %q, want %q の直下", n.Name, n.Path, root)
		}
	}
}

// TestReadDir_Errors は読めないディレクトリの扱いを検証する。
func TestReadDir_Errors(t *testing.T) {
	root := mkTree(t, "a.md")

	if _, err := ReadDir(filepath.Join(root, "nosuch")); err == nil {
		t.Error("存在しないディレクトリでエラーが返らない")
	}
	if _, err := ReadDir(filepath.Join(root, "a.md")); err == nil {
		t.Error("ファイルを指定してもエラーが返らない")
	}
}

// TestPathTo は経路の算出を検証する（UT-305。根拠: FR-032 / IMP-131）。
//
// ファイルシステムには触れないため、存在しないパスで検証できる。
// 区切り文字の違いを持ち込まないよう、比較はスラッシュ区切りで行う（UT-035）。
func TestPathTo(t *testing.T) {
	tests := []struct {
		name   string
		root   string
		target string
		want   []string
	}{
		// UT-305 ケース 2: 直下（境界値を先に。UT-013）
		{"直下のファイル", "/r", "/r/c.md", []string{}},
		{"target が root と同じ", "/r", "/r", []string{}},

		// UT-305 ケース 1: 経路上のディレクトリ
		{"2 階層下", "/r", "/r/a/b/c.md", []string{"/r/a", "/r/a/b"}},
		{"1 階層下", "/r", "/r/a/c.md", []string{"/r/a"}},
		{"4 階層下", "/r", "/r/a/b/c/d/e.md", []string{"/r/a", "/r/a/b", "/r/a/b/c", "/r/a/b/c/d"}},

		// UT-090 に従って追加
		{"冗長な区切りを含む root", "/r/", "/r/a/c.md", []string{"/r/a"}},
		{"target に ./ を含む", "/r", "/r/./a/c.md", []string{"/r/a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PathTo(tt.root, tt.target)
			if err != nil {
				t.Fatalf("PathTo(%q, %q) がエラーを返した: %v", tt.root, tt.target, err)
			}

			slashed := make([]string, len(got))
			for i, p := range got {
				slashed[i] = filepath.ToSlash(p)
			}
			if fmt.Sprint(slashed) != fmt.Sprint(tt.want) {
				t.Errorf("PathTo(%q, %q) = %v, want %v", tt.root, tt.target, slashed, tt.want)
			}
		})
	}
}

// TestPathTo_OutsideRoot はツリー外の対象を検証する（UT-305 ケース 3）。
//
// 仕様は「エラーまたは空。どちらかに固定する」としている。**エラーに固定した。**
// 空を返すと、ツリー外のファイルを開いたときに「展開する経路がない」のか
// 「ツリー外なので経路がない」のかを呼び出し側が区別できない（FR-052）。
func TestPathTo_OutsideRoot(t *testing.T) {
	for _, tt := range []struct{ name, root, target string }{
		{"別のディレクトリ", "/r", "/other/c.md"},
		{"root の親", "/r/sub", "/r/c.md"},
		{"相対的に上へ出る", "/r", "/r/../c.md"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PathTo(tt.root, tt.target)
			if !errors.Is(err, ErrOutsideRoot) {
				t.Errorf("PathTo(%q, %q) = %v, %v。want ErrOutsideRoot", tt.root, tt.target, got, err)
			}
		})
	}
}

// TestPathTo_CaseInsensitiveRoot は、大文字小文字だけが異なるルートの扱いを
// 検証する（IMP-025）。
//
// 期待値が OS で異なるのは仕様どおりである。Windows のパス比較は大文字小文字を
// 区別せず、Linux は区別する。
func TestPathTo_CaseInsensitiveRoot(t *testing.T) {
	got, err := PathTo("/R", "/r/a/c.md")

	if runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("Windows では大文字小文字を区別しないはず: %v", err)
		}
		if len(got) != 1 || filepath.ToSlash(got[0]) != "/R/a" {
			t.Errorf("PathTo = %v, want [/R/a]", got)
		}
		return
	}

	if !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("Linux では別のディレクトリとして扱うはず: %v, %v", got, err)
	}
}
