package main

// edit.go — `-edit`: generated/edit/ の中身を作り直す（E2E-012, G18）。
//
// **ディレクトリそのものは消さず、中身だけを作り直す。** MarkView が generated/edit/ の中の文書を
// 表示したまま -edit を実行する手順がある（E2E-381 の手順 10、E2E-386 の手順 6）。ディレクトリごと
// 消すと、Windows では監視のハンドルが残って消せないか作り直せず、Linux では監視が切れる。
//
// **消す前に、ファイルの属性とディレクトリの権限を戻す。** readonly.md は読み取り専用で作り、
// E2E-386 の手順 9 は locked/ の書き込みの権限を落とす。戻さないと 2 回目の -edit が消せない。
//
// **generated/edit/ に README.md を置かない。** E2E-381 は、このディレクトリを引数に起動したときに
// 操作案内が出ることを使う（FR-013）。

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// editFile は generated/edit/ の 1 つのファイル。path は generated/edit/ からの相対パス（/ 区切り）。
type editFile struct {
	path     string
	content  string
	readOnly bool        // 作った後に読み取り専用にする（Windows は属性、Linux は 0444）
	perm     fs.FileMode // 0 でなければ、作った後にこの権限にする（umask に左右されない）
}

// editFiles は generated/edit/ に置くファイルの一覧を返す（E2E-012 の表の順）。
func editFiles() []editFile {
	files := []editFile{
		{path: "tasks-lf.md", content: tasksLF()},
		{path: "table.md", content: tableMD()},
		{path: "invalid-utf8.md", content: invalidUTF8MD},
		{path: "readonly.md", content: readonlyMD, readOnly: true},
		{path: "conflict.md", content: conflictMD},
		{path: "undo.md", content: undoMD},
		{path: "carry.md", content: carryMD()},
		{path: "locked/locked.md", content: lockedMD},

		// E2E-385: BOM と CRLF・末尾に改行なし / LF / CR だけ・Front Matter / CRLF の表
		// **4 つとも日本語の行を持つ**（japaneseNote。E2E-237 の確認内容 1）
		{path: "bytes/tasks-crlf-bom.md", content: withNewlines(tasksBytes+japaneseNote, "\r\n", true, false)},
		{path: "bytes/tasks-lf.md", content: withNewlines(tasksBytes+japaneseNote, "\n", false, true)},
		{path: "bytes/tasks-cr.md", content: withNewlines(frontMatter+tasksBytes+japaneseNote, "\r", false, true)},
		{path: "bytes/table-crlf.md", content: withNewlines(tableGFM+japaneseNote, "\r\n", false, true)},

		// E2E-385 の手順 1〜3 をした後の内容と、E2E-387 の元の内容（人が書いたリテラル）
		{path: "expected/tasks-crlf-bom.md", content: withNewlines(tasksBytesExpected+japaneseNote, "\r\n", true, false)},
		{path: "expected/tasks-lf.md", content: withNewlines(tasksBytesExpected+japaneseNote, "\n", false, true)},
		{path: "expected/tasks-cr.md", content: withNewlines(frontMatter+tasksBytesExpected+japaneseNote, "\r", false, true)},
		{path: "expected/table-crlf.md", content: withNewlines(tableCRLFExpected+japaneseNote, "\r\n", false, true)},
		{path: "expected/undo-original.md", content: undoMD},
	}

	// 権限ビットとシンボリックリンクは Linux で作ったときだけ置く（E2E-385 の手順 6〜8）。
	if runtime.GOOS == "linux" {
		files = append(files,
			editFile{path: "bytes/perm.md", content: permMD, perm: 0o640},
			editFile{path: "bytes/real/linked.md", content: linkedMD},
		)
	}

	return files
}

// generateEdit は dir（generated/edit/）の中身を消して作り直す。
func generateEdit(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("生成先を作れない: %w", err)
	}

	if err := clearDir(dir); err != nil {
		return err
	}

	fmt.Printf("%-28s %8s  %s\n", "ファイル", "バイト", "")

	for _, file := range editFiles() {
		path := filepath.Join(dir, filepath.FromSlash(file.path))
		if err := writeEditFile(path, file); err != nil {
			return err
		}

		note := ""
		if file.readOnly {
			note = "読み取り専用"
		}
		if file.perm != 0 {
			note = fmt.Sprintf("権限 %#o", file.perm)
		}
		fmt.Printf("%-28s %8d  %s\n", file.path, len(file.content), note)
	}

	if runtime.GOOS == "linux" {
		link := filepath.Join(dir, "bytes", "link.md")
		if err := os.Symlink("real/linked.md", link); err != nil {
			return fmt.Errorf("シンボリックリンクを作れない (%s): %w", link, err)
		}
		fmt.Printf("%-28s %8s  %s\n", "bytes/link.md", "-", "-> real/linked.md")
	}

	fmt.Printf("\n生成先: %s\n", dir)

	return nil
}

// writeEditFile は 1 つのファイルを書き、権限を整える。
func writeEditFile(path string, file editFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ディレクトリを作れない (%s): %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
		return fmt.Errorf("作成できない (%s): %w", path, err)
	}

	switch {
	case file.readOnly:
		// Windows では書き込みのビットを落とすと読み取り専用の属性が付く（os.Chmod）。
		if err := os.Chmod(path, 0o444); err != nil {
			return fmt.Errorf("読み取り専用にできない (%s): %w", path, err)
		}
	case file.perm != 0:
		if err := os.Chmod(path, file.perm); err != nil {
			return fmt.Errorf("権限を変えられない (%s): %w", path, err)
		}
	}

	return nil
}

// clearDir は dir の中身を消す。dir そのものは残す。
func clearDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("生成先を読めない: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := removeAll(path); err != nil {
			return err
		}
	}

	return nil
}

// removeAll は、読み取り専用のファイルと書き込めなくしたディレクトリを戻してから path を消す。
func removeAll(path string) error {
	if err := makeWritable(path); err != nil {
		return err
	}

	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("削除できない (%s): %w", path, err)
	}

	return nil
}

// makeWritable は root 以下のディレクトリを 0755、ファイルを 0644 に戻す。root が無ければ何もしない。
//
// **ディレクトリは中を読む前に戻す**（WalkDir は中を読む前にディレクトリそのものを渡す）。読めなく
// なっていても戻してから降りられる。**シンボリックリンクはたどらない**——リンク先の実体は、同じ木の
// 中にあれば、そちらを通るときに戻す。
func makeWritable(root string) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return nil
		case d.IsDir():
			return os.Chmod(path, 0o755)
		default:
			return os.Chmod(path, 0o644)
		}
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("権限を戻せない (%s): %w", root, err)
	}

	return nil
}
