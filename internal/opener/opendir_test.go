package opener

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestOpenDir_Rejects は、ディレクトリでないものを起動しないことを検証する
// （UT-706 のケース 1〜5。根拠: FR-036, NFR-035, FR-110 / IMP-170, IMP-173）。
//
// **起動しないことを確かめるケースを先に書く**（UT-013）。**ケース 3〜5 が要である**——この経路は
// OS の既定のアプリケーションを呼び、ファイルを渡すと実行ファイルなら実行される（NFR-035）。
func TestOpenDir_Rejects(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("# a"), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "a.sh")
	if runtime.GOOS == "windows" {
		exe = filepath.Join(dir, "a.exe")
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		dir  string
		want error
	}{
		{"1 空文字", "", ErrNotFound},
		{"2 存在しない絶対パス", filepath.Join(dir, "nosuch"), ErrNotFound},
		{"3 通常のファイル", file, ErrNotDir},
		{"4 実行ファイル", exe, ErrNotDir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := record(t)
			if err := OpenDir(tt.dir); !errors.Is(err, tt.want) {
				t.Errorf("OpenDir(%q) = %v, want %v", tt.dir, err, tt.want)
			}
			if len(*calls) != 0 {
				t.Errorf("起動した: %+v", *calls)
			}
		})
	}

	// ケース 1 の 2 つ目: 実在する相対パスも拒む（実在しない相対パスでは、絶対パスかを確かめない
	// 実装も os.Stat の失敗で通る。4.78.0）。
	t.Run("1 実在する相対パス", func(t *testing.T) {
		t.Chdir(dir)
		calls := record(t)
		if err := OpenDir("docs"); !errors.Is(err, ErrNotFound) {
			t.Errorf(`OpenDir("docs") = %v, want ErrNotFound（実在しても絶対パスでなければ拒む）`, err)
		}
		if len(*calls) != 0 {
			t.Errorf("相対パスなのに起動した: %+v", *calls)
		}
	})

	// ケース 5: ファイルを指すシンボリックリンク（リンク先の実体で見る）。
	t.Run("5 ファイルを指すシンボリックリンク", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("Linux でのみ実行する（シンボリックリンクの作成に権限が要る。CI の Linux ジョブで実行する。UT-706）")
		}
		link := filepath.Join(dir, "filelink")
		if err := os.Symlink(file, link); err != nil {
			t.Fatal(err)
		}
		calls := record(t)
		if err := OpenDir(link); !errors.Is(err, ErrNotDir) {
			t.Errorf("OpenDir(link) = %v, want ErrNotDir", err)
		}
		if len(*calls) != 0 {
			t.Errorf("起動した: %+v", *calls)
		}
	})
}

// TestOpenDir_Command は、ディレクトリを OS ごとの形の 1 つの引数で渡すことを検証する
// （UT-706 のケース 6〜9。根拠: FR-036, NFR-035 / IMP-170, IMP-173）。
//
// ケース 8 は「名前で判定する」実装を捕まえる（過検出の検査）。
func TestOpenDir_Command(t *testing.T) {
	base := t.TempDir()
	mkdir := func(t *testing.T, name string) string {
		t.Helper()
		p := filepath.Join(base, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}

	tests := []struct {
		name string
		dir  string
	}{
		// ケース 6: 正常なディレクトリ。
		{"6 正常なディレクトリ", mkdir(t, "docs")},
		// ケース 7: 空白・,・&・;・'・^・括弧を含む名前（シェルを経由しない）。
		{"7 記号を含む名前", mkdir(t, "a b,c&d;e'f^g(h)")},
		// ケース 8: 名前が x.exe で終わるディレクトリ（種類は名前ではなく実体で見る）。
		{"8 x.exeという名前のディレクトリ", mkdir(t, "x.exe")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := record(t)
			if err := OpenDir(tt.dir); err != nil {
				t.Fatalf("OpenDir(%q) がエラーを返した: %v", tt.dir, err)
			}
			if len(*calls) != 1 {
				t.Fatalf("起動が %d 回, want 1", len(*calls))
			}
			checkCommand(t, (*calls)[0], tt.dir)
		})
	}

	// ケース 9: ディレクトリを指すシンボリックリンク。
	t.Run("9 ディレクトリを指すシンボリックリンク", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("Linux でのみ実行する（シンボリックリンクの作成に権限が要る。CI の Linux ジョブで実行する。UT-706）")
		}
		target := mkdir(t, "real")
		link := filepath.Join(base, "dirlink")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		calls := record(t)
		if err := OpenDir(link); err != nil {
			t.Fatalf("OpenDir(link) がエラーを返した: %v", err)
		}
		if len(*calls) != 1 {
			t.Fatalf("起動が %d 回, want 1", len(*calls))
		}
		checkCommand(t, (*calls)[0], link)
	})
}
