package document

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Replace は path の実体を data で置き換える（IMP-107, FR-143）。
// 返しうるエラー: ErrNotFound / ErrPermission / 書き込み時のエラー
//
// 処理の順序を IMP-107 の表に固定する。**どこで失敗しても、元のファイルは変わらず、
// 一時ファイルは残らない。**
//
//  1. filepath.EvalSymlinks で実体のパスを得る。以降はすべて実体に対して行う
//  2. os.Stat で権限ビットを控える
//  3. **書き込み用に開けるかを確かめ、すぐ閉じる**（切り詰めない）
//  4. 実体と同じディレクトリに `.<名前>.markview-*.tmp` を作る
//  5. 一時ファイルへ書き、Sync して閉じる
//  6. Windows 以外では、一時ファイルの権限ビットを 2 で控えた値にする
//  7. 一時ファイルを実体へリネームする
func Replace(path string, data []byte) error {
	// 1. 実体を得る。**リンクそのものを置き換えない**——実体へリネームするため、
	// シンボリックリンクは残る（FR-143, UT-112 ケース 7）。宛先の無いリンクは
	// ここで失敗し、宛先にファイルを作らない（ケース 10）。
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return replaceError(path, err, ErrNotFound)
	}

	// 2. 権限ビットを控える。
	info, err := os.Stat(real)
	if err != nil {
		return replaceError(real, err, ErrNotFound)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s: %w", real, ErrNotFound)
	}
	perm := info.Mode().Perm()

	// 3. **省かない。** ディレクトリに書き込めればリネームは成功するため、読み取り専用の
	// ファイルを置き換えてしまう（FR-143 の「元のファイルが書き込み可能でなければ置き換えない」。
	// UT-112 ケース 1）。Windows でも、読み取り専用属性とアクセス制御の両方をこの 1 回で確かめる。
	f, err := os.OpenFile(real, os.O_WRONLY, 0)
	if err != nil {
		return replaceError(real, err, ErrPermission)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("%s: %w", real, err)
	}

	// 4. **一時ファイルは実体と同じディレクトリに置く。** 別のボリュームへのリネームは
	// 置き換えにならない。`%TEMP%` に置かない（NFR-033 の例外の範囲。FR-143）。
	// **名前を `.` で始める**——ファイルツリーは `.` で始まる名前を出さない（FR-031）。
	tmp, err := os.CreateTemp(filepath.Dir(real), "."+filepath.Base(real)+".markview-*.tmp")
	if err != nil {
		// ディレクトリに書けない（UT-112 ケース 3）。
		return replaceError(real, err, ErrPermission)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	// 5. 書き、Sync して閉じる。
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%s: cannot write: %w", real, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%s: cannot sync: %w", real, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%s: cannot close: %w", real, err)
	}

	// 6. CreateTemp は 0600 で作る。元の権限ビットを保つ（UT-112 ケース 8）。
	// Windows の権限ビットは読み取り専用属性だけを表し、ここでは意味を持たない。
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpName, perm); err != nil {
			return fmt.Errorf("%s: cannot set permissions: %w", real, err)
		}
	}

	// 7. 置き換える。
	if err := os.Rename(tmpName, real); err != nil {
		return replaceError(real, err, nil)
	}
	committed = true
	return nil
}

// replaceError は Replace の途中のエラーを番兵エラーへ写す（IMP-021, IMP-107）。
//
// 存在しないものは ErrNotFound、権限の無いものは ErrPermission とする。どちらにも
// 当たらない場合は、その段階の既定（fallback）で包む。fallback が nil ならそのまま包む。
func replaceError(path string, err error, fallback error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s: %w", path, ErrNotFound)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("%s: %w", path, ErrPermission)
	case fallback != nil:
		return fmt.Errorf("%s: %w: %v", path, fallback, err)
	default:
		return fmt.Errorf("%s: %w", path, err)
	}
}
