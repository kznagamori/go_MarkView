package session

import (
	"path/filepath"
	"runtime"
	"strings"
)

// SamePath は 2 つのパスが同じ場所を指すかを判定する（IMP-025）。
//
// 比較は Windows では大文字小文字を区別せず、Linux では区別する。
// ファイルシステムの大文字小文字の扱いに合わせるためである。
//
// シンボリックリンクの解決は行わない。解決が必要な場面（配信対象の検査など）は
// 呼び出し側が filepath.EvalSymlinks を通したうえで渡す（AR-041, NFR-031）。
func SamePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// DisplayPath はステータス領域に出すパスと、ツリー外かどうかを返す
// （UI-060, FR-052, IMP-025）。
//
// ツリールートの内側なら相対パス、外側なら絶対パスを返す。外側の文書には
// 呼び出し側が `(outside tree)` を添える（UI-060）。
//
// ファイルシステムには触れない。target は絶対パスであることを前提とし、
// Clean だけを行う。存在しないパスでも算出できるほうが呼び出し側で扱いやすい。
//
// 区切り文字は OS のものをそのまま使う。ツリー外で絶対パスを出すときと
// 表記を揃えるためである。
func DisplayPath(root, target string) (display string, outside bool) {
	abs := filepath.Clean(target)

	// ツリールートが定まっていない場合。ツリーがないので「外」でもない。
	if root == "" {
		return abs, false
	}

	// filepath.Rel は Windows で大文字小文字を区別せずに比較する（IMP-025）。
	rel, err := filepath.Rel(filepath.Clean(root), abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return abs, true
	}

	return rel, false
}
