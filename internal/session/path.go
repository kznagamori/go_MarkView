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

// SameFile は a と b が同じファイル（1.7）かを返す（IMP-191）。
//
// 両方を filepath.EvalSymlinks で解決し、SamePath で比べる。解決に失敗した側は、
// 解決前のパスのまま比べる（IMP-192）。同じ文書の再描画として状態を引き継ぐか
// （DocumentDTO.SameDocument）の判断そのものであり、desktop に置かない（UT-809）。
//
//   - **シンボリックリンクを解決した実体で比べる**（1.7）。リンク経由で開き直しただけで
//     編集モードや原寸表示が解除されないようにする（UT-809 ケース 8・9）
//   - **解決に失敗しても偽にしない。** 削除された文書の表記が一致すれば同じとみなす
//     （UT-809 ケース 10）。存在する側と存在しない側は、解決後と解決前の表記が違うため偽になる
//   - DisplayPath と違いファイルシステムに触れる（IMP-191）
func SameFile(a, b string) bool {
	return SamePath(resolveLinks(a), resolveLinks(b))
}

// resolveLinks はシンボリックリンクを解決したパスを返す。解決に失敗したら p のまま返す。
func resolveLinks(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
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
