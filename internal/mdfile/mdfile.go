// Package mdfile は Markdown ファイルの拡張子判定を提供する（IMP-105）。
//
// ファイル選択ダイアログのフィルタ（FR-010）、ドロップ判定（FR-011）、
// ファイルツリーのフィルタ（FR-031）、リンク遷移の判定（FR-050）、
// README の探索（FR-013）が、いずれもこの 1 箇所を参照する。
// 判定の定義を複数の場所へ分散させない。
//
// 本パッケージは**他のどのパッケージにも依存しない**。IMP-012 が
// internal 同士の依存を禁じているなかで、ここだけが参照を許されているのは
// 依存を持たない葉だからである。依存を追加してはならない。
package mdfile

import (
	"path/filepath"
	"strings"
)

// Extensions は FR-010 / FR-031 が定める対象拡張子。
//
// 先頭のドットを含む小文字表記で持つ。判定時は入力側を小文字化して比較する。
var Extensions = []string{".md", ".markdown", ".mdown", ".mkd"}

// IsMarkdown は path の拡張子が Markdown のものかを判定する。
// 比較は常に小文字化して行うため、README.MD も真となる。
//
// 判定は最終要素の拡張子のみを見る。したがって "a.md.txt" は偽であり、
// ディレクトリを含むパスでも結果は変わらない。
func IsMarkdown(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)

	// filepath.Ext(".md") は空文字ではなく ".md" を返す。つまり ".md" という
	// 名前は「拡張子だけのファイル」ではなく「全体が拡張子」と解釈される。
	// これを真としてしまうと、拡張子しか持たない名前まで Markdown 扱いになる
	// ため、名前全体が拡張子と一致する場合を除外する（UT-105 のケース 6）。
	if ext == "" || strings.EqualFold(ext, base) {
		return false
	}

	ext = strings.ToLower(ext)
	for _, e := range Extensions {
		if ext == e {
			return true
		}
	}
	return false
}
