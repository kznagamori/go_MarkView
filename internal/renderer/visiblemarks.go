package renderer

// このファイルは、Markdown の記法で囲むインラインの要素（強調・取り消し線・リンク・画像・インライン
// コード）の、ソース上の開きと閉じの記号の区間を求める（IMP-123 の InlineSpan の Open / Close）。
// 生 HTML のタグの対は visibletags.go が求める。visiblebuild.go が 400 行の目安（IMP-011）を超え
// ないよう分けた。

import "github.com/yuin/goldmark/ast"

// marked は記号で囲む要素（強調・取り消し線）を、中身の範囲 inner から開きと閉じの記号の区間を求めて
// 足し、記号を含む範囲を返す。求められなければ中身の端に長さ 0 の区間を置き、範囲は分からないとする。
func (b *textBuilder) marked(kind string, start int, inner ext, level int) ext {
	open, closing, ok := delimiters(b.src, inner, level)
	if !ok {
		at, end := b.contentEdges(start)
		b.inline(kind, start, Span{Start: at, Stop: at}, Span{Start: end, Stop: end})
		return ext{}
	}
	b.inline(kind, start, open, closing)

	return ext{start: open.Start, stop: closing.Stop, ok: true}
}

// delimiters は、中身の範囲 inner のすぐ外に並ぶ開きと閉じの記号の区間を返す。
//
// goldmark は、開きの記号の並びの**後ろの** k 文字と、閉じの記号の並びの**前の** k 文字を 1 つの要素に
// 使う（v1.8.5 の parser/delimiter.go の ProcessDelimiters）。したがって記号は中身の範囲（子の要素の
// 記号・タグを含む）に接して並ぶ。k は、強調なら段（level。1 か 2）。取り消し線（level 0）は前後に
// 並ぶ `~` の数で決まる（どちらも 2 つ以上なら 2、そうでなければ 1。Delimiter.CalcComsumption）。
// 子の範囲は構文から正確に求まる（記号の文字を確かめる必要は無い。2026-11-09 に確かめる処理を
// 壊しても赤くならなかったため外した）。範囲が分からないか、文書の外に出るときだけ求められない。
func delimiters(src []byte, inner ext, level int) (Span, Span, bool) {
	k := level
	if level == 0 {
		k = 1
		if runBefore(src, inner.start, '~') >= 2 && runAfter(src, inner.stop, '~') >= 2 {
			k = 2
		}
	}
	open := Span{Start: inner.start - k, Stop: inner.start}
	closing := Span{Start: inner.stop, Stop: inner.stop + k}
	if !inner.ok || open.Start < 0 || closing.Stop > len(src) {
		return Span{}, Span{}, false
	}

	return open, closing, true
}

// runBefore は pos の直前に並ぶ c の数を返す。
func runBefore(src []byte, pos int, c byte) int {
	n := 0
	for pos-n > 0 && src[pos-n-1] == c {
		n++
	}

	return n
}

// runAfter は pos から並ぶ c の数を返す。
func runAfter(src []byte, pos int, c byte) int {
	n := 0
	for pos+n < len(src) && src[pos+n] == c {
		n++
	}

	return n
}

// linkExt はリンク（openLen 1）と画像（openLen 2）の範囲と、開き（`[` / `![`）と閉じ（`]` から構文の
// 終わりまで。`](…)` / `][…]` / `]`）の区間を返す。開きの位置は goldmark の Pos()（v1.8.5 の
// parser/link.go が括弧の頭に置く）、閉じの位置は linkRecorder の記録である。記録が無ければ（区間の
// 終わりが閉じの括弧より後ろにならない）、今の位置に長さ 0 の区間を置き、範囲は分からないとする。
func (b *textBuilder) linkExt(n ast.Node, openLen int) (ext, Span, Span) {
	at := Span{Start: b.cur, Stop: b.cur}
	m := b.links[n]
	s := n.Pos()
	if s < 0 || m.bracket < s || m.end <= m.bracket || m.end > len(b.src) {
		return ext{}, at, at
	}
	if m.end > b.cur {
		b.cur = m.end
	}

	return ext{start: s, stop: m.end, ok: true}, Span{Start: s, Stop: s + openLen}, Span{Start: m.bracket, Stop: m.end}
}

// codeFences は、インラインコードの Pos()（開きのバッククォートの並びの頭。goldmark の inline の
// 構文解析が、そのノードを読み始めた位置を置く）から、開きの並びと、閉じの並び（開きの後ろで最初に
// 現れる、同じ長さの並び。CommonMark のコードスパン）を返す。
func codeFences(src []byte, pos int) (Span, Span, bool) {
	if pos < 0 {
		return Span{}, Span{}, false
	}
	n := runAfter(src, pos, '`')
	for i := pos + n; i < len(src); {
		if src[i] != '`' {
			i++
			continue
		}
		m := runAfter(src, i, '`')
		if m == n {
			return Span{Start: pos, Stop: pos + n}, Span{Start: i, Stop: i + m}, true
		}
		i += m
	}

	return Span{}, Span{}, false
}
