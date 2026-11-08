package filetree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
)

// skipUnlessLinux は Linux 以外で t.Skip する（シンボリックリンクの作成に権限が要る。UT-035）。
// **CI の Linux ジョブで必ず実行する**（BR-052）。
func skipUnlessLinux(t *testing.T, why string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("Linux でのみ実行する（%s。CI の Linux ジョブで実行する。UT-035）", why)
	}
}

// symlink は newname → oldname のシンボリックリンクを作る。
func symlink(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Fatalf("シンボリックリンクを作れない: %v", err)
	}
}

// byName は名前で要素を引く。
func byName(nodes []Node, name string) (Node, bool) {
	for _, n := range nodes {
		if n.Name == name {
			return n, true
		}
	}
	return Node{}, false
}

// TestReadDir_Kinds は要素の種類を検証する（UT-301 のケース 10〜12。根拠: FR-031, FR-037, UI-030 /
// IMP-130, IMP-131, IMP-135）。
func TestReadDir_Kinds(t *testing.T) {
	// ケース 10: 種類ごとのアイコンの材料。ディレクトリの Kind は空文字。
	t.Run("10 種類", func(t *testing.T) {
		root := mkTree(t, "a.md", "b.txt", "c.png", "d.puml", "e.mmd", "f.iuml", "docs/x.md")
		nodes := readDirShow(t, root, Show{Diagrams: true, Images: true})
		want := map[string]mdfile.Kind{
			"a.md": mdfile.KindMarkdown, "c.png": mdfile.KindImage,
			"d.puml": mdfile.KindPlantUML, "e.mmd": mdfile.KindMermaid, "docs": mdfile.KindNone,
		}
		for name, kind := range want {
			n, ok := byName(nodes, name)
			if !ok {
				t.Errorf("%s が無い: %v", name, names(nodes))
				continue
			}
			if n.Kind != kind {
				t.Errorf("%s の Kind = %q, want %q", name, n.Kind, kind)
			}
		}
	})

	// ケース 11: 種類は Classify で決める（UT-308 のケース 18〜20 と同じ判定）。
	t.Run("11 シンボリックリンク", func(t *testing.T) {
		skipUnlessLinux(t, "シンボリックリンク")
		outside := mkTree(t, "x.md", "y.png", "d.txt", "dir/a.md")
		root := mkTree(t, "a.md")
		symlink(t, filepath.Join(outside, "x.md"), filepath.Join(root, "link.md"))
		symlink(t, filepath.Join(outside, "d.txt"), filepath.Join(root, "link2.md"))
		symlink(t, filepath.Join(outside, "y.png"), filepath.Join(root, "link3.md"))
		symlink(t, filepath.Join(outside, "dir"), filepath.Join(root, "dirlink"))

		nodes := readDirShow(t, root, Show{Diagrams: true, Images: true})
		if n, ok := byName(nodes, "link.md"); !ok || n.Kind != mdfile.KindMarkdown {
			t.Errorf("link.md = %+v（有無 %v）, want markdown で含まれる", n, ok)
		}
		for _, name := range []string{"link2.md", "link3.md"} {
			if _, ok := byName(nodes, name); ok {
				t.Errorf("名前と実体の種類が違う %s が含まれた", name)
			}
		}
		if n, ok := byName(nodes, "dirlink"); !ok || !n.IsDir || n.Kind != mdfile.KindNone {
			t.Errorf("dirlink = %+v（有無 %v）, want IsDir が真・Kind が空で含まれる", n, ok)
		}
	})

	// ケース 12: Classify が失敗した項目はツリーに出さない。ディレクトリ全体を失敗にしない（4.78.0）。
	t.Run("12 宛先の無いシンボリックリンク", func(t *testing.T) {
		skipUnlessLinux(t, "シンボリックリンク")
		root := mkTree(t, "a.md")
		symlink(t, filepath.Join(root, "nosuch.md"), filepath.Join(root, "dead.md"))
		if got := names(readDir(t, root)); fmt.Sprint(got) != fmt.Sprint([]string{"a.md"}) {
			t.Errorf("結果 = %v, want [a.md]", got)
		}
	})
}

// TestReadDir_TruncationAfterFilter は、件数の上限を絞り込んだ後の件数で数えることを検証する
// （UT-303 のケース 5・6。根拠: FR-032, FR-037 / IMP-131, IMP-134）。
//
// ケース 5 は、隠している画像の数で上限に掛かる実装を捕まえる（画像を隠しているのに Markdown が欠ける）。
func TestReadDir_TruncationAfterFilter(t *testing.T) {
	spec := make([]string, 0, 1500)
	for i := range 1000 {
		spec = append(spec, fmt.Sprintf("f%05d.md", i))
	}
	for i := range 500 {
		spec = append(spec, fmt.Sprintf("g%05d.png", i))
	}
	root := mkTree(t, spec...)

	for _, tt := range []struct {
		name        string
		show        Show
		wantOmitted int
	}{
		{"5 画像を隠す", Show{Diagrams: true, Images: false}, 0},
		{"6 画像も出す", Show{Diagrams: true, Images: true}, 500},
	} {
		t.Run(tt.name, func(t *testing.T) {
			nodes := readDirShow(t, root, tt.show)
			if len(nodes) != MaxEntriesPerDir {
				t.Errorf("件数 = %d, want %d", len(nodes), MaxEntriesPerDir)
			}
			if len(nodes) > 0 && nodes[0].Omitted != tt.wantOmitted {
				t.Errorf("Omitted = %d, want %d", nodes[0].Omitted, tt.wantOmitted)
			}
		})
	}
}

// TestShow は表示する種類を検証する（UT-307。根拠: FR-031, FR-037 / IMP-133, IMP-134）。
//
// **絞り込みを Go 側で行う理由を、ケース 5〜7 が支える**（IMP-134）。フロントエンドは読み込んで
// いないディレクトリの中身を知らないため、画像しか無いディレクトリを隠せない。
func TestShow(t *testing.T) {
	all := []Show{{}, {Diagrams: true}, {Images: true}, {Diagrams: true, Images: true}}

	// ケース 1〜4: Includes。
	t.Run("1 KindNoneはどの選択でも出さない", func(t *testing.T) {
		for _, s := range all {
			if s.Includes(mdfile.KindNone) {
				t.Errorf("%+v.Includes(KindNone) が真", s)
			}
		}
	})
	t.Run("2 Markdownは外せない", func(t *testing.T) {
		for _, s := range all {
			if !s.Includes(mdfile.KindMarkdown) {
				t.Errorf("%+v.Includes(KindMarkdown) が偽", s)
			}
		}
	})
	t.Run("3 図はDiagramsに従う", func(t *testing.T) {
		for _, s := range all {
			for _, k := range []mdfile.Kind{mdfile.KindPlantUML, mdfile.KindMermaid} {
				if got := s.Includes(k); got != s.Diagrams {
					t.Errorf("%+v.Includes(%q) = %v, want %v", s, k, got, s.Diagrams)
				}
			}
		}
	})
	t.Run("4 画像はImagesに従う", func(t *testing.T) {
		for _, s := range all {
			if got := s.Includes(mdfile.KindImage); got != s.Images {
				t.Errorf("%+v.Includes(KindImage) = %v, want %v", s, got, s.Images)
			}
		}
	})

	// ケース 5〜8: ディレクトリの先読みも表示する種類で判断する（IMP-133）。
	tests := []struct {
		name string
		tree []string
		dir  string
		off  Show // dir が出ないはずの選択
		on   Show // dir が出るはずの選択（ゼロ値ならどれでも出ない）
	}{
		{"5 画像だけのサブディレクトリ", []string{"a.md", "imgs/a.png"}, "imgs", Show{Diagrams: true}, Show{Images: true}},
		{"6 図だけのサブディレクトリ", []string{"a.md", "diag/a.mmd"}, "diag", Show{Images: true}, Show{Diagrams: true}},
		{"7 1階層下にだけ画像", []string{"a.md", "x/y/a.png"}, "x", Show{Diagrams: true}, Show{Images: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := mkTree(t, tt.tree...)
			if got := names(readDirShow(t, root, tt.off)); contains(got, tt.dir) {
				t.Errorf("%+v で %s が出た: %v", tt.off, tt.dir, got)
			}
			if got := names(readDirShow(t, root, tt.on)); !contains(got, tt.dir) {
				t.Errorf("%+v で %s が出ない: %v", tt.on, tt.dir, got)
			}
		})
	}
	t.Run("8 開けないものだけのディレクトリ", func(t *testing.T) {
		root := mkTree(t, "a.md", "x/a.txt", "x/b.iuml")
		for _, s := range all {
			if got := names(readDirShow(t, root, s)); contains(got, "x") {
				t.Errorf("%+v で x が出た: %v", s, got)
			}
		}
	})
}

// checkFixture は UT-308 の構造を作り、ツリールート root・ツリールートの外 other・隣の root2 を返す。
func checkFixture(t *testing.T) (root, other, root2 string) {
	t.Helper()

	base := t.TempDir()
	root = filepath.Join(base, "root")
	other = filepath.Join(base, "other")
	root2 = filepath.Join(base, "root2")
	for _, f := range []string{
		"root/a.md", "root/b.png", "root/c.puml", "root/d.txt", "root/docs/e.md",
		"root/.hidden/f.md", "root/node_modules/g.md", "root/.x.md",
		"other/x.md", "other/y.png", "root2/x.md",
	} {
		p := filepath.Join(base, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, other, root2
}

// TestCheck はツリーの操作の検査を検証する（UT-308 のケース 1〜21・23・24。根拠: FR-032, FR-033, FR-036,
// FR-037, FR-090, NFR-030, NFR-035, FR-110, FR-027 / IMP-131, IMP-132, IMP-135）。
//
// **フロントエンドが渡すパスを信じないことが、この検査の目的である**（NFR-030, NFR-035）。通らない
// ケースを先に書く（UT-013）。**ケース 6 と 13、18 と 19 は対で意味を持つ。**
func TestCheck(t *testing.T) {
	root, other, root2 := checkFixture(t)
	in := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

	reject := []struct {
		name      string
		root      string
		path      string
		show      Show
		want      Want
		allowRoot bool
	}{
		{"1 rootが空", "", in("a.md"), defaultShow, WantOpenable, false},
		{"2 相対パス", root, "a.md", defaultShow, WantOpenable, false},
		{"3 ツリールートの外", root, filepath.Join(other, "x.md"), defaultShow, WantOpenable, false},
		{"4 Cleanの後で外", root, root + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Join("other", "x.md"), defaultShow, WantOpenable, false},
		{"5 ツリールートそのもの", root, root, defaultShow, WantDir, false},
		{"6 隠したディレクトリの中", root, in(".hidden/f.md"), defaultShow, WantOpenable, false},
		{"6 隠したディレクトリの中の存在しないファイル", root, in(".hidden/none.md"), defaultShow, WantOpenable, false},
		{"7 既定で隠すディレクトリ", root, in("node_modules/g.md"), defaultShow, WantOpenable, false},
		{"8 開けない種類 WantOpenable", root, in("d.txt"), defaultShow, WantOpenable, false},
		{"8 開けない種類 WantAny", root, in("d.txt"), defaultShow, WantAny, false},
		{"9 フィルターで隠した画像", root, in("b.png"), defaultShow, WantOpenable, false},
		{"10 画像はエディタで開かない", root, in("b.png"), Show{Diagrams: true, Images: true}, WantText, false},
		{"11 図を隠しているときの図", root, in("c.puml"), Show{}, WantText, false},
		{"12 ディレクトリにWantOpenable", root, in("docs"), defaultShow, WantOpenable, false},
		{"12 ファイルにWantDir", root, in("a.md"), defaultShow, WantDir, false},
		{"23 名前の頭が同じ兄弟", root, filepath.Join(root2, "x.md"), defaultShow, WantOpenable, false},
		{"24 隠したディレクトリそのもの", root, in(".hidden"), defaultShow, WantDir, false},
		{"24 既定で隠すディレクトリそのもの", root, in("node_modules"), defaultShow, WantDir, false},
		{"24 ドットで始まるファイル", root, in(".x.md"), defaultShow, WantOpenable, false},
	}
	for _, tt := range reject {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := Check(tt.root, tt.path, tt.show, tt.want, tt.allowRoot); !errors.Is(err, ErrNotInTree) {
				t.Errorf("Check = %q, %v, want ErrNotInTree", got, err)
			}
		})
	}

	// ケース 13: 存在しないファイルは ErrMissing（not-found を出す。FR-036）。
	t.Run("13 存在しないファイル", func(t *testing.T) {
		if _, err := Check(root, in("gone.md"), defaultShow, WantOpenable, false); !errors.Is(err, ErrMissing) {
			t.Errorf("エラー = %v, want ErrMissing", err)
		}
	})

	pass := []struct {
		name      string
		path      string
		show      Show
		want      Want
		allowRoot bool
		wantPath  string
	}{
		{"5 allowRootならツリールート", root, defaultShow, WantDir, true, root},
		{"9 画像を出しているなら通る", in("b.png"), Show{Diagrams: true, Images: true}, WantOpenable, false, in("b.png")},
		{"11 図はWantTextで通る", in("c.puml"), defaultShow, WantText, false, in("c.puml")},
		{"15 Cleanしたパス", in("a.md"), defaultShow, WantOpenable, false, in("a.md")},
		{"15 ./を含む", root + string(filepath.Separator) + "." + string(filepath.Separator) + "a.md", defaultShow, WantOpenable, false, in("a.md")},
		{"16 ディレクトリにWantDir", in("docs"), defaultShow, WantDir, false, in("docs")},
		{"16 ディレクトリにWantAny", in("docs"), defaultShow, WantAny, false, in("docs")},
		{"16 ファイルにWantAny", in("a.md"), defaultShow, WantAny, false, in("a.md")},
	}
	for _, tt := range pass {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Check(root, tt.path, tt.show, tt.want, tt.allowRoot)
			if err != nil || got != tt.wantPath {
				t.Errorf("Check = %q, %v, want %q, nil", got, err, tt.wantPath)
			}
		})
	}

	// ケース 17: ツリールート自身の名前と、それより上には規則を当てはめない。
	t.Run("17 ドットで始まるツリールート", func(t *testing.T) {
		dot := filepath.Join(t.TempDir(), ".proj")
		if err := os.MkdirAll(dot, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dot, "a.md")
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := Check(dot, p, defaultShow, WantOpenable, false); err != nil || got != p {
			t.Errorf("Check = %q, %v, want %q, nil", got, err, p)
		}
	})
}

// TestCheck_Linux は Linux でだけ成り立つツリーの操作の検査を検証する（UT-308 のケース 14・18〜21）。
//
// **Windows では t.Skip し、CI の Linux ジョブで必ず実行する**（BR-052。UT-407 と同じ）。
func TestCheck_Linux(t *testing.T) {
	skipUnlessLinux(t, "シンボリックリンクと権限")
	root, other, _ := checkFixture(t)
	in := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

	// ケース 14: 読めないディレクトリの中のファイル。ErrMissing でも ErrNotInTree でもない（permission）。
	t.Run("14 読めないディレクトリの中", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("実行ユーザが root のため、権限を無視して読める（UT-308 ケース 14。UT-035）")
		}
		locked := in("locked")
		if err := os.MkdirAll(locked, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(locked, "z.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		// 戻さないと t.TempDir() の後始末が消せない。
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

		_, err := Check(root, filepath.Join(locked, "z.md"), defaultShow, WantOpenable, false)
		if errors.Is(err, ErrMissing) || errors.Is(err, ErrNotInTree) || !errors.Is(err, fs.ErrPermission) {
			t.Errorf("エラー = %v, want fs.ErrPermission（ErrMissing でも ErrNotInTree でもない）", err)
		}
	})

	// ケース 18: ツリールートの外の実体を指すリンクは、ツリー上のパスで通る。
	t.Run("18 外を指すリンク", func(t *testing.T) {
		symlink(t, filepath.Join(other, "x.md"), in("link.md"))
		if got, err := Check(root, in("link.md"), defaultShow, WantOpenable, false); err != nil || got != in("link.md") {
			t.Errorf("Check = %q, %v, want %q, nil", got, err, in("link.md"))
		}
	})

	// ケース 19: 名前と実体の種類が違うリンク。
	t.Run("19 名前と実体の種類が違う", func(t *testing.T) {
		symlink(t, in("d.txt"), in("link2.md"))
		symlink(t, filepath.Join(other, "y.png"), in("link3.md"))
		for _, name := range []string{"link2.md", "link3.md"} {
			if _, err := Check(root, in(name), defaultShow, WantOpenable, false); !errors.Is(err, ErrNotInTree) {
				t.Errorf("%s のエラー = %v, want ErrNotInTree", name, err)
			}
		}
	})

	// ケース 20: ディレクトリかはリンク先の実体で見る。
	t.Run("20 ディレクトリへのリンク", func(t *testing.T) {
		symlink(t, other, in("dirlink"))
		if got, err := Check(root, in("dirlink"), defaultShow, WantDir, false); err != nil || got != in("dirlink") {
			t.Errorf("Check = %q, %v, want %q, nil", got, err, in("dirlink"))
		}
	})

	// ケース 21: 宛先の無いリンクは ErrMissing。
	t.Run("21 宛先の無いリンク", func(t *testing.T) {
		symlink(t, in("nosuch.md"), in("dead.md"))
		if _, err := Check(root, in("dead.md"), defaultShow, WantOpenable, false); !errors.Is(err, ErrMissing) {
			t.Errorf("エラー = %v, want ErrMissing", err)
		}
	})
}

// TestClassify は種類の判定を検証する（UT-308 のケース 22。根拠: FR-036, FR-027 / IMP-131, IMP-135）。
func TestClassify(t *testing.T) {
	root, _, _ := checkFixture(t)
	in := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

	tests := []struct {
		path  string
		isDir bool
		kind  mdfile.Kind
	}{
		{in("a.md"), false, mdfile.KindMarkdown},
		{in("docs"), true, mdfile.KindNone},
		{in("d.txt"), false, mdfile.KindNone},
	}
	for _, tt := range tests {
		t.Run(filepath.Base(tt.path), func(t *testing.T) {
			isDir, kind, err := Classify(tt.path)
			if err != nil || isDir != tt.isDir || kind != tt.kind {
				t.Errorf("Classify = (%v, %q, %v), want (%v, %q, nil)", isDir, kind, err, tt.isDir, tt.kind)
			}
		})
	}
	t.Run("存在しないパス", func(t *testing.T) {
		if _, _, err := Classify(in("gone.md")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("エラー = %v, want fs.ErrNotExist", err)
		}
	})
}
