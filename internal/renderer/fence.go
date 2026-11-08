package renderer

// このファイルは、フェンス付きのコードブロック（図のブロックを含む）の囲いの行の位置を構文木に残し
// （IMP-111）、書き換えの位置（FenceLocation。IMP-121）と前置き（IMP-122）を求める。
//
// **goldmark の FencedCodeBlock は囲いの行の区間を持たない**（中身の行と言語指定だけ）。**閉じの囲いが
// あったか・どこかは残らない**。引用が先に終わった文書などがあるため、後からソースを探して推測すると
// 取り違える。そこで標準のフェンスの構文解析器を包み、開きと閉じの囲いを読んだ行を変換ごとの文脈
// （parser.Context）に記録する。**Render と Locate と Visible は同じ goldmark の構成を使う**ため、同じ
// 記録が残る。
//
// **開きの囲いの位置にも Pos() を使わない。** goldmark の Pos() は「行の区間の頭 + 字下げの桁数」であり、
// 行頭にタブの残り（Segment.Padding。`>` の直後のタブなど）があると、その桁数だけ後ろへずれる（独立した
// レビューの指摘。`>\t```` で 2 桁）。goldmark 自身が言語指定の位置に使う式（Start - Padding + 行の中の
// 位置）で記録する。
//
// 描画の data-locked（IMP-115, IMP-120）と書き込み（IMP-122）は、どちらも fenceLocation の Locked を使う。

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// fenceMarksKey は、変換 1 回分の「囲いの文字の位置」の記録を parser.Context へ置く鍵。
var fenceMarksKey = parser.NewContextKey()

// fenceMark は 1 つのブロックの囲いの文字の位置（どちらも前置きの後ろの、囲いの文字の並びの頭）。
type fenceMark struct {
	open   int
	close  int
	closed bool // 閉じの囲いの行で閉じたか（文書や引用の終わりで閉じたものは偽）
}

// fenceMarks はブロックごとの記録。
type fenceMarks map[*ast.FencedCodeBlock]*fenceMark

// marksOf は文脈の記録を返す。無ければ作って置く。
func marksOf(pc parser.Context) fenceMarks {
	if m, ok := pc.Get(fenceMarksKey).(fenceMarks); ok {
		return m
	}
	m := fenceMarks{}
	pc.Set(fenceMarksKey, m)

	return m
}

// fenceExtension は標準のフェンスの構文解析器（優先度 700）より先に、それを包んだものを登録する。
// 開く・続ける・閉じるの判断はすべて標準のものに任せ、閉じの囲いを読んだ位置だけを記録する。
type fenceExtension struct{}

func (fenceExtension) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithBlockParsers(
		util.Prioritized(fenceParser{BlockParser: parser.NewFencedCodeBlockParser()}, 699),
	))
}

// fenceParser は標準のフェンスの構文解析器を包む（IMP-111）。
type fenceParser struct {
	parser.BlockParser
}

// Open は標準の Open を呼び、ブロックを開いたら、開きの囲いの文字の位置を記録する。位置は標準の
// Open と同じ読み方で求める: 字下げの後ろの位置（BlockOffset）は、タブの残りを空白として前に足した
// 行の中の位置なので、行の区間の頭から Padding を引いて足す（goldmark v1.8.5 の fcode_block.go が
// 言語指定の位置に使う式）。読む前に控える——標準の Open は読み位置を進める。
func (p fenceParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, seg := reader.PeekLine()
	offset := pc.BlockOffset()
	node, st := p.BlockParser.Open(parent, reader, pc)
	if fc, ok := node.(*ast.FencedCodeBlock); ok && offset >= 0 && offset < len(line) {
		marksOf(pc)[fc] = &fenceMark{open: seg.Start - seg.Padding + offset}
	}

	return node, st
}

// Continue は標準の Continue を呼び、閉じの囲いの行で閉じたら、その囲いの文字の位置を記録する。
// 標準の Continue が Close を返すのは、閉じの囲いの行を読んだときだけである（goldmark v1.8.5 の
// fcode_block.go。文書や引用の終わりでは Continue を経ずに Close が呼ばれる）。
func (p fenceParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	_, seg := reader.PeekLine()
	st := p.BlockParser.Continue(node, reader, pc)
	if st&parser.Close == 0 {
		return st
	}

	fc, ok := node.(*ast.FencedCodeBlock)
	if !ok {
		return st
	}
	mark := marksOf(pc)[fc]
	src := reader.Source()
	if mark == nil || mark.open >= len(src) {
		return st
	}
	fence := src[mark.open]
	pos := seg.Start
	for pos < len(src) && src[pos] != fence && (src[pos] == ' ' || src[pos] == '\t') {
		pos++
	}
	if pos < len(src) && src[pos] == fence {
		mark.close, mark.closed = pos, true
	}

	return st
}

// fenceLocation はブロックの位置と前置きを求める（IMP-121, IMP-122）。source は前処理の後の本文、
// shift は位置を元のソースへ戻すために足す量（preprocess。描画では 0）。
func fenceLocation(fc *ast.FencedCodeBlock, source []byte, pc parser.Context, shift int) FenceLocation {
	var loc FenceLocation

	// 開きの囲いの文字の並び（Open で記録した位置。Pos() はタブの残りがあるとずれる）。記録は
	// fenceParser が開いたすべてのブロックにある。無いのは別の構文解析器が作ったものだけで、その
	// ときに限り Pos() に頼る。
	mark := marksOf(pc)[fc]
	if mark == nil {
		mark = &fenceMark{open: fc.Pos()}
	}
	open := min(max(mark.open, 0), len(source))
	loc.Open = Span{Start: open, Stop: runEnd(source, open)}
	if fc.Info != nil {
		loc.Info = strings.TrimSpace(string(fc.Info.Segment.Value(source)))
	}

	// 中身の行の全体の始まり: 開きの囲いの行の次の行の頭（改行が無ければ文書の終わり）。
	bodyStart := len(source)
	if i := bytes.IndexByte(source[open:], '\n'); i >= 0 {
		bodyStart = open + i + 1
	}

	// 中身の各行（前置きの後ろから、改行の手前まで）と、その行の前置き。
	lines := fc.Lines()
	var prefixes []string
	var empty []bool
	padded := false
	lastStop := bodyStart
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		stop := seg.Stop
		if stop > seg.Start && source[stop-1] == '\n' {
			stop--
		}
		loc.Lines = append(loc.Lines, Span{Start: seg.Start, Stop: stop})
		prefixes = append(prefixes, string(source[lineHead(source, seg.Start):seg.Start]))
		empty = append(empty, stop == seg.Start)
		if seg.Padding != 0 {
			padded = true
		}
		lastStop = seg.Stop
	}

	// 閉じの囲い。閉じが無ければ中身の終わりの位置で長さ 0。
	closePos, closed := mark.close, mark.closed
	if closed {
		loc.Closed = true
		loc.Close = Span{Start: closePos, Stop: runEnd(source, closePos)}
		loc.Body = Span{Start: bodyStart, Stop: lineHead(source, closePos)}
	} else {
		loc.Body = Span{Start: bodyStart, Stop: lastStop}
		loc.Close = Span{Start: lastStop, Stop: lastStop}
	}
	if loc.Body.Stop < loc.Body.Start {
		loc.Body.Stop = loc.Body.Start
	}

	// ブロックの前置き（IMP-122）: 最初の空でない中身の行の前置き → 閉じの囲いの行の頭から囲いの
	// 文字の手前まで → 開きの囲いの行の頭から囲いの文字の手前まで（ここにリストの記号があれば決められ
	// ないとして Locked）。
	prefix, decided := "", false
	for i, e := range empty {
		if !e {
			prefix, decided = prefixes[i], true
			break
		}
	}
	undecidable := false
	switch {
	case decided:
	case closed:
		prefix = string(source[lineHead(source, closePos):closePos])
	default:
		prefix = string(source[lineHead(source, open):open])
		if strings.Trim(prefix, " \t>") != "" {
			undecidable = true
		}
	}
	loc.Prefix = prefix

	// Locked（IMP-122）: 前置きがブロックの前置きと違う行がある（空の行は、末尾の空白とタブを除いた
	// 前置きとも一致してよい）・中身の行がタブの途中から始まる・前置きを決められない。
	loc.Locked = padded || undecidable
	trimmed := strings.TrimRight(prefix, " \t")
	for i, p := range prefixes {
		if p == prefix || (empty[i] && p == trimmed) {
			continue
		}
		loc.Locked = true
	}

	return loc.shifted(shift)
}

// shifted は位置を shift だけずらした写しを返す。
func (loc FenceLocation) shifted(shift int) FenceLocation {
	if shift == 0 {
		return loc
	}
	mv := func(s Span) Span { return Span{Start: s.Start + shift, Stop: s.Stop + shift} }
	loc.Open, loc.Close, loc.Body = mv(loc.Open), mv(loc.Close), mv(loc.Body)
	lines := make([]Span, len(loc.Lines))
	for i, s := range loc.Lines {
		lines[i] = mv(s)
	}
	loc.Lines = lines

	return loc
}

// runEnd は pos の文字と同じ文字が続く並びの終わりを返す（囲いの文字の並び）。
func runEnd(source []byte, pos int) int {
	if pos >= len(source) {
		return pos
	}
	c := source[pos]
	end := pos
	for end < len(source) && source[end] == c {
		end++
	}

	return end
}

// lineHead は pos を含む行の頭の位置を返す。
func lineHead(source []byte, pos int) int {
	if pos > len(source) {
		pos = len(source)
	}

	return bytes.LastIndexByte(source[:pos], '\n') + 1
}
