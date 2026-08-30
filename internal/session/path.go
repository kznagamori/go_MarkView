package session

import (
	"path/filepath"
	"runtime"
	"strings"
)

// samePath は 2 つのパスが同じ場所を指すかを判定する（IMP-025）。
//
// 比較は Windows では大文字小文字を区別せず、Linux では区別する。
// ファイルシステムの大文字小文字の扱いに合わせるためである。
//
// シンボリックリンクの解決は行わない。解決が必要な場面（配信対象の検査など）は
// 呼び出し側が filepath.EvalSymlinks を通したうえで渡す（AR-041, NFR-031）。
func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
