package opener

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// 本ファイルは UT-704 / UT-705 を実装したものである。
//
// **対象の実装（editor.go）はまだスタブであり、すべて失敗する。** これは
// 意図した状態であり、「書いたテストを一度失敗させて確かめる」（UT-033）を
// 実装の前に済ませるためのものである。T7-1 で緑にする。

// ---------------------------------------------------------------------------
// UT-704: エディタ起動の検査（IMP-171 / NFR-035）
// ---------------------------------------------------------------------------

// TestOpenWith_Rejects は起動前の検査を検証する
// （UT-704 ケース 1〜4, 7, 8。根拠: IMP-171 / NFR-035 の 5）。
//
// **起動しないことを確かめるケースを先に書く**（UT-013）。runCommand を
// 記録用に差し替え、実際にエディタを起動しない（UT-035, UT-702 と同じ）。
//
// 相対パスやコマンド名を許すと、`$PATH` や作業ディレクトリの内容によって
// 起動されるプログラムが変わる（NFR-035 の 5）。
func TestOpenWith_Rejects(t *testing.T) {
	dir := t.TempDir()

	editor := filepath.Join(dir, "editor"+exeSuffix())
	if err := os.WriteFile(editor, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(dir, "a.md")
	if err := os.WriteFile(doc, []byte("# a"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		editor  string
		path    string
		wantErr error
	}{
		// UT-704 ケース 1〜3: エディタが絶対パスでない
		{"エディタが相対パス", relativeEditor(), doc, ErrNotAbsolute},
		{"エディタがコマンド名だけ", "vim", doc, ErrNotAbsolute},
		{"エディタが空文字", "", doc, ErrNotAbsolute},

		// UT-704 ケース 4: エディタが存在しない
		{"エディタが存在しない絶対パス", filepath.Join(dir, "nosuch"+exeSuffix()), doc, ErrNotFound},

		// UT-704 ケース 7〜8: 対象が開けない
		{"対象が空文字", editor, "", ErrNotFound},
		{"対象が存在しない", editor, filepath.Join(dir, "nosuch.md"), ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := record(t)

			err := OpenWith(tt.editor, tt.path)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("OpenWith(%q, %q) = %v, want %v", tt.editor, tt.path, err, tt.wantErr)
			}
			if len(*calls) != 0 {
				t.Errorf("検査に落ちたのに起動した: %+v", *calls)
			}
		})
	}
}

// TestOpenWith_RejectsRelativeTarget は、対象が相対パスのときに
// **実在していても**拒むことを検証する（UT-704 ケース 8。根拠: IMP-171）。
//
// **カレントディレクトリを移し、そこに実在するファイルを相対パスで渡す。**
// 存在しない相対パスを渡すと「存在しないから拒まれた」のか「絶対パスでないから
// 拒まれた」のかが区別できず、**絶対パスの検査を落としても気づけない。**
// 実際、当初はそう書いていて素通りした（2026-09-03 の変異テストで検出）。
func TestOpenWith_RejectsRelativeTarget(t *testing.T) {
	dir := t.TempDir()

	editor := filepath.Join(dir, "editor"+exeSuffix())
	if err := os.WriteFile(editor, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("# a"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	calls := record(t)

	if err := OpenWith(editor, "a.md"); !errors.Is(err, ErrNotFound) {
		t.Errorf(`OpenWith(_, "a.md") = %v, want ErrNotFound（実在しても絶対パスでなければ拒む）`, err)
	}
	if len(*calls) != 0 {
		t.Errorf("相対パスなのに起動した: %+v", *calls)
	}
}

// TestOpenWith_RejectsSelf は MarkView 自身の指定を拒むことを検証する
// （UT-704 ケース 5〜6。根拠: NFR-035 の 6）。
//
// 許すと、押すたびにウィンドウが増える。テスト実行中の `os.Executable()` は
// テストバイナリ自身であり、これを「MarkView 自身」として扱える。
func TestOpenWith_RejectsSelf(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	doc := filepath.Join(t.TempDir(), "a.md")
	if err := os.WriteFile(doc, []byte("# a"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("実行ファイルそのもの", func(t *testing.T) {
		calls := record(t)

		if err := OpenWith(self, doc); !errors.Is(err, ErrSelf) {
			t.Errorf("OpenWith(自身) = %v, want ErrSelf", err)
		}
		if len(*calls) != 0 {
			t.Errorf("自身を起動した: %+v", *calls)
		}
	})

	t.Run("シンボリックリンク経由の自身", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "editor"+exeSuffix())
		if err := os.Symlink(self, link); err != nil {
			// **スキップの理由をメッセージに残す**（UT-704 の NOTE）。
			// EvalSymlinks の呼び出しを削っても黙って通るテストにしない。
			t.Skipf("シンボリックリンクを作れない環境のため確認できない: %v", err)
		}

		calls := record(t)

		if err := OpenWith(link, doc); !errors.Is(err, ErrSelf) {
			t.Errorf("OpenWith(自身へのリンク) = %v, want ErrSelf（EvalSymlinks で解決する。IMP-171）", err)
		}
		if len(*calls) != 0 {
			t.Errorf("自身を起動した: %+v", *calls)
		}
	})
}

// TestOpenWith_Command は正常時に組み立てるコマンドを検証する
// （UT-704 ケース 9〜11。根拠: IMP-171 / NFR-035 の 2 と 4）。
//
// **引数が対象のパス 1 つだけであることに意味がある。** 起動オプションや
// コマンドテンプレートを後から足すと、ここで落ちる（NFR-035 の 2）。
// 空白や記号を含む名前は、シェルを経由していれば分割されるか実行される。
func TestOpenWith_Command(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{"普通のファイル名", "a.md"},
		{"空白を含む", "my notes.md"},
		{"セミコロンとコマンドに見える文字列", "a; rm -rf x.md"},
		{"アンパサンド", "a & b.md"},
		{"パイプ", "a | b.md"},
		{"引用符", `a'b"c.md`},
		{"バッククォートとドル", "a$(x).md"},
		{"日本語", "設計書.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			editor := filepath.Join(dir, "editor"+exeSuffix())
			if err := os.WriteFile(editor, []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}

			doc := filepath.Join(dir, tt.file)
			if err := os.WriteFile(doc, []byte("# a"), 0o644); err != nil {
				t.Skipf("この名前のファイルを作れない環境: %v", err)
			}

			calls := record(t)

			if err := OpenWith(editor, doc); err != nil {
				t.Fatalf("OpenWith がエラーを返した: %v", err)
			}
			if len(*calls) != 1 {
				t.Fatalf("起動回数 = %d, want 1", len(*calls))
			}

			got := (*calls)[0]

			if got.name != editor {
				t.Errorf("起動したコマンド = %q, want %q", got.name, editor)
			}
			if len(got.args) != 1 {
				t.Fatalf("引数 = %q, want 対象 1 つだけ（NFR-035 の 2）", got.args)
			}
			if got.args[0] != doc {
				t.Errorf("引数 = %q, want %q（分割されている）", got.args[0], doc)
			}

			// シェルを経由していないこと（NFR-035 の 4, IMP-170）。
			for _, shell := range []string{"cmd", "cmd.exe", "sh", "bash", "powershell"} {
				if strings.EqualFold(filepath.Base(got.name), shell) {
					t.Errorf("シェルを経由している: %q", got.name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UT-705: プリセットの検出（IMP-172 / UI-103）
// ---------------------------------------------------------------------------

// TestEditors_ListsAllPresetsInOrder は一覧の中身と順序を検証する
// （UT-705 ケース 1。根拠: IMP-172 / UI-103）。
//
// **見つからなくても一覧から消さない。** 消すと「なぜ自分のエディタが
// 出ないのか」が分からない（UI-103）。
func TestEditors_ListsAllPresetsInOrder(t *testing.T) {
	stubLookup(t, nil)

	got := Editors()
	want := wantPresets()

	if len(got) != len(want) {
		t.Fatalf("件数 = %d, want %d（見つからなくても一覧から消さない。UI-103）", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID {
			t.Errorf("%d 件目の ID = %q, want %q（順序は定義順。UI-103）", i, got[i].ID, want[i].ID)
		}
		if got[i].Name != want[i].Name {
			t.Errorf("%s の表示名 = %q, want %q", want[i].ID, got[i].Name, want[i].Name)
		}
		if got[i].Path != "" {
			t.Errorf("%s の Path = %q, want 空（見つからなかったため）", want[i].ID, got[i].Path)
		}
	}
}

// TestEditors_KeepsOrderWhenPartiallyFound は、一部だけ見つかった場合にも
// 順序が変わらないことを検証する（UT-705 ケース 2。根拠: IMP-172 / UI-103）。
//
// **末尾の 1 件だけを見つかったことにする。** 先頭を選ぶと、「見つかった
// ものを前へ出す」実装でも通ってしまい、順序の検証にならない。
func TestEditors_KeepsOrderWhenPartiallyFound(t *testing.T) {
	want := wantPresets()
	last := want[len(want)-1].ID
	found := filepath.Join(t.TempDir(), "found"+exeSuffix())

	stubLookup(t, map[string]string{last: found})

	got := Editors()

	if len(got) != len(want) {
		t.Fatalf("件数 = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID {
			t.Fatalf("%d 件目の ID = %q, want %q（見つかったものを前へ並べ替えている）", i, got[i].ID, want[i].ID)
		}

		wantPath := ""
		if want[i].ID == last {
			wantPath = found
		}
		if got[i].Path != wantPath {
			t.Errorf("%s の Path = %q, want %q", want[i].ID, got[i].Path, wantPath)
		}
	}
}

// TestEditors_TableInvariants はプリセット表そのものを検証する
// （UT-705 ケース 4〜6。根拠: IMP-172 / IMP-309）。
//
// ファイルシステムに触れない。**プリセットを足したときに壊れる**ことに
// 意味がある。
func TestEditors_TableInvariants(t *testing.T) {
	stubLookup(t, nil)

	got := Editors()
	if len(got) == 0 {
		t.Fatal("プリセットが 1 件も無い")
	}

	seen := make(map[string]bool, len(got))
	for i, e := range got {
		// ケース 6: 画面に出す名前が欠けていないこと（UI-103）
		if e.ID == "" {
			t.Errorf("%d 件目の ID が空", i)
		}
		if e.Name == "" {
			t.Errorf("%s の表示名が空（UI-103 が一覧に出すため）", e.ID)
		}

		// ケース 5: `custom` はフロントエンドとの間の予約語（IMP-309）
		if e.ID == "custom" {
			t.Errorf("%d 件目の ID が custom（IMP-309 の予約語であり、プリセットに使えない）", i)
		}

		// ケース 4: ID の重複がないこと
		if seen[e.ID] {
			t.Errorf("ID が重複している: %q", e.ID)
		}
		seen[e.ID] = true
	}
}

// TestLookupPreset_TakesFirstExisting は候補の探索順を検証する
// （UT-705 ケース 3。根拠: IMP-172）。
//
// **差し替えた lookup ではなく、本物の lookupPreset を呼ぶ。** 「候補を順に
// 見て、先に見つかったものを採る」という規則そのものを確かめる。
//
// **「2 件とも存在する」ケースを必ず含める。** 存在する候補が 1 件しかない
// 表では、「先に見つかったものを採る」実装と「後に見つかったものを採る」実装が
// 同じ結果になり、**探索順を落としても気づけない。** 順序は実際に意味を持つ
// （IMP-172 の表では VS Code の LOCALAPPDATA 版、sakura の x86 版が先）。
func TestLookupPreset_TakesFirstExisting(t *testing.T) {
	missing, present, want := syntheticCandidates(t)

	t.Run("1 件目が無く 2 件目がある", func(t *testing.T) {
		got := lookupPreset(preset{ID: "test", Name: "Test", candidates: []string{missing, present[0]}})
		if got != want[0] {
			t.Errorf("lookupPreset = %q, want %q", got, want[0])
		}
	})

	t.Run("2 件とも存在する", func(t *testing.T) {
		got := lookupPreset(preset{ID: "test", Name: "Test", candidates: []string{present[0], present[1]}})
		if got != want[0] {
			t.Errorf("lookupPreset = %q, want %q（先に見つかったものを採る。IMP-172）", got, want[0])
		}
	})

	t.Run("どの候補も無い", func(t *testing.T) {
		got := lookupPreset(preset{ID: "test", Name: "Test", candidates: []string{missing}})
		if got != "" {
			t.Errorf("lookupPreset = %q, want 空", got)
		}
	})

	t.Run("候補が空", func(t *testing.T) {
		got := lookupPreset(preset{ID: "test", Name: "Test"})
		if got != "" {
			t.Errorf("lookupPreset = %q, want 空", got)
		}
	})
}

// TestLookupPreset_RejectsRelativePath は $PATH に相対パスの項目がある場合に
// その候補を採らないことを検証する（根拠: IMP-172 / NFR-035 の 5）。
//
// 採ってしまうと、作業ディレクトリに実行ファイルを置くだけで起動対象を
// 差し替えられる。**Windows の候補は絶対パスであり `$PATH` を引かない**ため、
// この観点は Linux にしか存在しない。
func TestLookupPreset_RejectsRelativePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows のプリセットは絶対パスで探すため $PATH を引かない（IMP-172）")
	}

	const name = "markview-test-relative"

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("PATH", ".")

	got := lookupPreset(preset{ID: "test", Name: "Test", candidates: []string{name}})
	if got != "" {
		t.Errorf("lookupPreset = %q, want 空（作業ディレクトリで起動対象が変わる。NFR-035 の 5）", got)
	}
}

// TestPresetTable_CandidatesAreAbsolute は Windows の候補がすべて絶対パスで
// あることを検証する（根拠: IMP-172 / NFR-035 の 5）。
//
// 候補は環境変数からの相対位置で組み立てる。**変数が空の環境でそのまま
// filepath.Join すると相対パスになり**、作業ディレクトリに置いた実行ファイルを
// 起動しうる。空の場合は候補ごと落とすこと。
//
// **Linux の候補はコマンド名であり、絶対パスではない**（$PATH から引く）。
func TestPresetTable_CandidatesAreAbsolute(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Linux の候補は $PATH から引くコマンド名である（IMP-172）")
	}

	// インストール先を指す変数が 1 つも無い環境を作る。
	for _, env := range []string{"WINDIR", "LOCALAPPDATA", "PROGRAMFILES", "PROGRAMFILES(X86)"} {
		t.Setenv(env, "")
	}

	for _, p := range presetTable() {
		for _, c := range p.candidates {
			if !filepath.IsAbs(c) {
				t.Errorf("%s の候補が絶対パスでない: %q（NFR-035 の 5）", p.ID, c)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// ヘルパ
// ---------------------------------------------------------------------------

// wantPresets は IMP-172 が定めるプリセットを、ID と表示名の組として
// 定義順に返す。
//
// **実装から読まず、仕様書（IMP-172）から書き写している**（UT-015, UT-031）。
// 表を並べ替えた・項目を落とした・名前を変えた場合にテストが落ちる。
//
// 端末エディタ（vim / nano / emacs -nw）と「OS の既定アプリケーション」は
// **プリセットに含めない**（IMP-172）。前者は端末を持たずに起動されて
// 何も起きず、後者は MarkView 自身を起動しうる。
func wantPresets() []Editor {
	if runtime.GOOS == "windows" {
		return []Editor{
			{ID: "notepad", Name: "Notepad"},
			{ID: "vscode", Name: "Visual Studio Code"},
			{ID: "notepadpp", Name: "Notepad++"},
			{ID: "hidemaru", Name: "Hidemaru"},
			{ID: "sakura", Name: "sakura editor"},
		}
	}

	return []Editor{
		{ID: "gnome-text-editor", Name: "GNOME Text Editor"},
		{ID: "gedit", Name: "gedit"},
		{ID: "kate", Name: "Kate"},
		{ID: "mousepad", Name: "Mousepad"},
		{ID: "vscode", Name: "Visual Studio Code"},
		{ID: "gvim", Name: "gVim"},
	}
}

// stubLookup は検出処理を差し替える（UT-705, UT-035）。
//
// found に入れた ID だけが「見つかった」ことになり、その値が Path になる。
// **実行環境にエディタが入っているかどうかに依存させない。**
// t.Cleanup で必ず元へ戻す（UT-017）。
func stubLookup(t *testing.T, found map[string]string) {
	t.Helper()

	original := lookup
	lookup = func(p preset) string { return found[p.ID] }
	t.Cleanup(func() { lookup = original })
}

// syntheticCandidates は lookupPreset に渡す候補を組み立てる。
//
// 候補の意味は OS で異なる（IMP-172）。Windows は絶対パス、Linux は
// $PATH から引くコマンド名である。**本物のエディタを使わない**（UT-035）。
//
// **存在する候補を 2 件返す。** 1 件しか用意しないと探索順を検証できない
// （TestLookupPreset_TakesFirstExisting のコメントを参照）。present[i] を
// 渡したときに返るはずの絶対パスが want[i] である。
func syntheticCandidates(t *testing.T) (missing string, present, want []string) {
	t.Helper()

	dir := t.TempDir()

	if runtime.GOOS == "windows" {
		// 既知の絶対パスを os.Stat で調べる
		for _, name := range []string{"first.exe", "second.exe"} {
			full := filepath.Join(dir, name)
			if err := os.WriteFile(full, []byte("x"), 0o755); err != nil {
				t.Fatal(err)
			}
			present = append(present, full)
			want = append(want, full)
		}

		return filepath.Join(dir, "missing.exe"), present, want
	}

	// $PATH からコマンド名で引く
	for _, name := range []string{"markview-test-first", "markview-test-second"} {
		full := filepath.Join(dir, name)
		if err := os.WriteFile(full, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		present = append(present, name)
		want = append(want, full)
	}
	t.Setenv("PATH", dir)

	return "markview-test-missing", present, want
}

// exeSuffix は実行ファイルの拡張子を返す。
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// relativeEditor は「絶対パスでない」エディタ指定を返す（UT-704 ケース 1）。
func relativeEditor() string {
	if runtime.GOOS == "windows" {
		return `.\notepad.exe`
	}
	return "./editor"
}
