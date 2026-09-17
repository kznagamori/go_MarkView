package main

import (
	"io"
	"os"
	"path/filepath"
)

// 本ファイルは、取得した資産の置き換えに使うファイル操作を持つ（BR-043 の「置き換え方」）。
//
// main.go が 400 行の目安（IMP-011）を超えたため分けた。

// moveDir は staging の階層を本番へ移す。
//
// **os.Rename はファイルシステムをまたげない。** Windows で TEMP が
// リポジトリと別のドライブにあると必ず失敗する（実測: C: と P:。2026-09-03）。
// しかもこの関数は移す前に本番を消しており、**失敗すると資産が消えたまま残る。**
// 落ちる余地を作らないよう、rename がだめなら複製へ落とす。
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	return copyTree(src, dst)
}

// copyTree は src の階層を dst へ複製する（BR-042 の preserve）。
//
// src が無い場合は何もしない。初回の取得ではまだ置かれていないため。
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return copyFile(src, dst)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	for _, e := range entries {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // 読むだけ

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck

		return err
	}

	return out.Close()
}
