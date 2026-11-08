// Package mdfile は開けるファイルの拡張子と種類の判定を提供する（IMP-105）。
//
// 開けるファイルは Markdown・図のファイル（PlantUML と Mermaid）・画像ファイルの 3 種類である
// （1.7。v1.2.0 で図と画像を足した）。読み込み（IMP-102）、ファイル選択ダイアログのフィルタ
// （FR-010）、ドロップ・起動引数・リンクの判定（FR-011, FR-012, FR-050）、ファイルツリーの絞り込み
// とツリーの操作の検査（FR-031, IMP-134, IMP-135）、README の探索（FR-013）、ローカル画像の配信
// （AR-041, IMP-161）、外部エディタの対象（FR-090）が、いずれもこの 1 箇所を参照する。
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

// Extensions は FR-010 / FR-031 が定める Markdown の拡張子。
//
// 先頭のドットを含む小文字表記で持つ。判定時は入力側を小文字化して比較する。
var Extensions = []string{".md", ".markdown", ".mdown", ".mkd"}

// IsMarkdown は path の拡張子が Markdown のものかを判定する（KindOf(path) == KindMarkdown）。
// 比較は常に小文字化して行うため、README.MD も真となる。
//
// README の探索（FR-013, IMP-193）は v1.2.0 でもこの判定を使う（**Markdown のまま**。IMP-105 の表）。
func IsMarkdown(path string) bool {
	return KindOf(path) == KindMarkdown
}

// extOf は path の最終要素の拡張子を小文字で返す。拡張子が無いか、名前全体が拡張子なら空文字。
//
// 判定は最終要素の拡張子のみを見る。したがって "a.md.txt" は ".txt" であり、
// ディレクトリを含むパスでも結果は変わらない。
//
// filepath.Ext(".md") は空文字ではなく ".md" を返す。つまり ".md" という
// 名前は「拡張子だけのファイル」ではなく「全体が拡張子」と解釈される。
// これを拡張子として扱うと、拡張子しか持たない名前までその種類になるため、
// 名前全体が拡張子と一致する場合を除外する（UT-105 のケース 4）。
func extOf(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)

	if ext == "" || strings.EqualFold(ext, base) {
		return ""
	}

	return strings.ToLower(ext)
}
