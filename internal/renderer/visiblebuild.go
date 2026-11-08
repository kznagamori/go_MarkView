package renderer

// このファイルは、1 つのブロックの見える文字とソースとの対応（TextBlock）を組み立てる走査を持つ
// （IMP-123 の数え方の表）。visible.go が 400 行の目安（IMP-011）を超えないよう分けた。
//
// **数え方は画面側（visible.js。IMP-266）と同じにする。** 一致は描画スモーク（BR-054, UT-817）が
// 全ブロックについて確かめる。部品（丸ごとにしか囲めないもの）の 1 文字は U+FFFC で表す。
//
// **インラインの要素の記号・タグの区間（InlineSpan の Open / Close）は、子の要素のソース上の範囲から
// 下から順に求める**（4.86.0。独立したレビューの指摘）。中身の最初の文字の直前の記号を数える方法では、
// 中身の端がリンク・コード・生 HTML のタグ・別の強調のときに、別の記号を指していた
// （`~~**a**~~` の取り消し線が `**` を指す）。IMP-124 の 4（外側に差し込む）と 5（手書きの取り消し線
// を外す）がこの区間を使う。

import (
	"bytes"
	"sort"

	emojiast "github.com/yuin/goldmark-emoji/ast"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// objectChar は 1 文字と数える部品（数式・画像・脚注の参照）の文字（IMP-123。4.77.0。利用者の判断）。
const objectChar = '￼'

// textBuilder は 1 つのブロックの見える文字を組み立てる。
type textBuilder struct {
	src   []byte    // 前処理の後の本文
	shift int       // 位置を元のソースへ戻すために足す量（preprocess）
	cur   int       // ここまでに読んだソースの位置（区間を持たないノードの位置を探す起点）
	links linkMarks // リンクと画像の閉じの位置（linkend.go）

	tb   TextBlock
	tags []rawTag
}

// ext はインラインの要素のソース上の範囲（前処理の後の本文の上。記号・タグを含む）。
type ext struct {
	start, stop int
	ok          bool // 分かったか（分からない要素を端に持つ強調は、記号の区間を求めない）
}

// buildText は inline の子を持つノード（Paragraph・Heading・TextBlock・TableCell）の見える文字を返す。
func buildText(n ast.Node, src []byte, shift int, links linkMarks) *TextBlock {
	b := &textBuilder{src: src, shift: shift, links: links}
	if n.Lines().Len() > 0 {
		b.cur = n.Lines().At(0).Start
	}
	b.tb.Visible = []rune{}
	b.tb.Source = []Span{}
	b.tb.Atoms = []Span{}
	b.tb.Inlines = []InlineSpan{}

	// ブロックの終わりの改行は数えない（IMP-123。入れ子のリストの手前で止まる TextBlock を含む）。
	// goldmark はブロックの最後の文字のノードに改行の印を付けないため、ここで落とす必要は無い
	// （2026-11-08。落とす処理を外しても UT-220 のケース 12 が緑のままであることで確かめた）。
	b.children(n)

	b.pairTags()
	// 入れ子の外側から順（IMP-123）。中身の区間が同じなら、記号・タグがソースで先に出る方が外側
	// （`***a***` の斜体は太字の外、`<del>**a**</del>` の del は太字の外）。
	sort.SliceStable(b.tb.Inlines, func(i, j int) bool {
		a, c := b.tb.Inlines[i], b.tb.Inlines[j]
		if a.Text.Start != c.Text.Start {
			return a.Text.Start < c.Text.Start
		}
		if a.Text.Stop != c.Text.Stop {
			return a.Text.Stop > c.Text.Stop
		}
		return a.Open.Start < c.Open.Start
	})

	return &b.tb
}

// add は文字と、その区間（前処理の後の本文の上）を足す。
func (b *textBuilder) add(r rune, s Span) {
	b.tb.Visible = append(b.tb.Visible, r)
	b.tb.Source = append(b.tb.Source, Span{Start: s.Start + b.shift, Stop: s.Stop + b.shift})
	if s.Stop > b.cur {
		b.cur = s.Stop
	}
}

// atom は start から今までの見える文字を、丸ごとにしか囲めない部品とする。
func (b *textBuilder) atom(start int) {
	if len(b.tb.Visible) > start {
		b.tb.Atoms = append(b.tb.Atoms, Span{Start: start, Stop: len(b.tb.Visible)})
	}
}

// find は、ソースの今の位置から pat を探し、その区間を返す。見つからなければ今の位置で長さ 0。
func (b *textBuilder) find(pat []byte) Span {
	if b.cur <= len(b.src) && len(pat) > 0 {
		if i := bytes.Index(b.src[b.cur:], pat); i >= 0 {
			return Span{Start: b.cur + i, Stop: b.cur + i + len(pat)}
		}
	}
	return Span{Start: b.cur, Stop: b.cur}
}

// found は find の区間を範囲にする（見つからなければ分からない）。
func found(s Span) ext {
	return ext{start: s.Start, stop: s.Stop, ok: s.Stop > s.Start}
}

// lineBreak は文字のノードの後ろの改行を 1 文字（\n）として足す（IMP-123）。区間は改行の印
// （ハード改行の空白やバックスラッシュ）から改行の文字まで。
func (b *textBuilder) lineBreak(from int, hard bool) {
	stop := from
	if i := bytes.IndexByte(b.src[from:], '\n'); i >= 0 {
		stop = from + i + 1
	}
	b.add('\n', Span{Start: from, Stop: stop})
	if hard {
		b.tb.HardBreaks++
	}
}

// visit は inline のノード 1 つを数え、その範囲を返す。
func (b *textBuilder) visit(n ast.Node) ext {
	switch v := n.(type) {
	case *ast.Text:
		return b.visitText(v, v.Segment.Start)

	case *ast.String:
		at := Span{Start: b.cur, Stop: b.cur}
		for _, r := range string(v.Value) {
			b.add(r, at)
		}
		return ext{}

	case *ast.CodeSpan:
		return b.visitCode(v)

	case *ast.Emphasis:
		start := len(b.tb.Visible)
		inner := b.children(v)
		kind := "emphasis"
		if v.Level == 2 {
			kind = "strong"
		}
		return b.marked(kind, start, inner, v.Level)

	case *extast.Strikethrough:
		start := len(b.tb.Visible)
		return b.marked("strike", start, b.children(v), 0)

	case *ast.Link:
		start := len(b.tb.Visible)
		b.children(v)
		e, open, closing := b.linkExt(v, 1)
		b.inline("link", start, open, closing)
		return e

	case *ast.AutoLink:
		label := v.Label(b.src)
		s := b.find(label)
		start := len(b.tb.Visible)
		runes, spans := decodeTextRaw(label, s.Start)
		for i, r := range runes {
			b.add(r, spans[i])
		}
		b.atom(start)
		if s.Start > 0 && s.Stop < len(b.src) && b.src[s.Start-1] == '<' && b.src[s.Stop] == '>' {
			return ext{start: s.Start - 1, stop: s.Stop + 1, ok: true}
		}
		return found(s)

	case *ast.Image:
		start := len(b.tb.Visible)
		e, open, closing := b.linkExt(v, 2)
		at := Span{Start: e.start, Stop: e.stop}
		if !e.ok {
			at = Span{Start: b.cur, Stop: b.cur}
		}
		b.add(objectChar, at)
		b.atom(start)
		b.inline("image", start, open, closing)
		return e

	case *mathInline:
		// 区間は前後の $ 1 つずつを含む（インラインの数式の区切りは常に $ 1 つ。math.go）。並んだ $ を
		// 数えると、続けて書いた数式（`$x$$y$`）の区間が重なる（4.86.0）。
		start := len(b.tb.Visible)
		s := Span{Start: v.segment.Start - 1, Stop: v.segment.Stop + 1}
		b.add(objectChar, s)
		b.atom(start)
		return found(s)

	case *extast.FootnoteLink:
		start := len(b.tb.Visible)
		s := b.find([]byte("[^"))
		if i := bytes.IndexByte(b.src[s.Stop:], ']'); i >= 0 && s.Stop > s.Start {
			s.Stop += i + 1
		}
		b.add(objectChar, s)
		b.atom(start)
		return found(s)

	case *extast.FootnoteBacklink:
		// 変換器が前に &#160; を 1 つ書く。構文木には無い。ソースには無いため区間は長さ 0（IMP-123）。
		start := len(b.tb.Visible)
		at := Span{Start: b.cur, Stop: b.cur}
		b.add(' ', at)
		b.add('↩', at)
		b.atom(start)
		return ext{}

	case *emojiast.Emoji:
		start := len(b.tb.Visible)
		s := b.find([]byte(":" + string(v.ShortName) + ":"))
		if v.Value != nil {
			for _, r := range v.Value.Unicode {
				b.add(r, s)
			}
		}
		b.atom(start)
		return found(s)

	case *ast.RawHTML:
		return b.visitRawHTML(v)

	case *extast.TaskCheckBox:
		// 0 文字。後ろの空白も数えない（4.76.0）——goldmark のタスクリストの構文解析が、括弧の後ろの
		// 空白を文字のノードから外している（2026-11-08。UT-220 のケース 11 で確かめた）。
		return ext{}

	default:
		return b.children(n)
	}
}

// visitText は文字のノードを数える（エスケープと文字参照を解く。IMP-123）。base はノードの文字の
// 始まりの位置で、ふつうは区間の頭である（children が閉じの記号の残りを置き直すときだけ違う）。
func (b *textBuilder) visitText(t *ast.Text, base int) ext {
	seg := t.Segment
	value := seg.Value(b.src)
	stop := seg.Stop + base - seg.Start

	if t.IsRaw() {
		b.addRaw(value, base)
	} else {
		runes, spans := decodeText(value, base)
		for i, r := range runes {
			b.add(r, spans[i])
		}
	}
	if stop > b.cur {
		b.cur = stop
	}

	switch {
	case t.HardLineBreak():
		b.lineBreak(stop, true)
	case t.SoftLineBreak():
		b.lineBreak(stop, false)
	}

	return ext{start: base, stop: stop, ok: true}
}

// addRaw はインラインコードの中の文字を数える。エスケープも文字参照も解かず、改行は半角空白 1 つ
// （CommonMark のコードスパン。変換器が code の中に書く文字と同じ。IMP-123。4.78.0）。
func (b *textBuilder) addRaw(value []byte, base int) {
	runes, spans := decodeTextRaw(value, base)
	for i, r := range runes {
		if r == '\n' {
			r = ' '
		}
		b.add(r, spans[i])
	}
}

// visitCode はインラインコードを部品として数え、開きと閉じのバッククォートの並びを区間にする。
func (b *textBuilder) visitCode(v *ast.CodeSpan) ext {
	start := len(b.tb.Visible)
	for c := v.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.addRaw(t.Segment.Value(b.src), t.Segment.Start)
		}
	}
	b.atom(start)

	open, closing, ok := codeFences(b.src, v.Pos())
	if !ok {
		at, end := b.contentEdges(start)
		b.inline("code", start, Span{Start: at, Stop: at}, Span{Start: end, Stop: end})
		return ext{}
	}
	b.inline("code", start, open, closing)
	if closing.Stop > b.cur {
		b.cur = closing.Stop
	}

	return ext{start: open.Start, stop: closing.Stop, ok: true}
}

// children は子を順に数え、最初の子の始まりから最後の子の終わりまでを範囲として返す。
//
// **閉じの記号の残りを置き直す。** goldmark は、閉じの記号の並び（`**a***` の `***`）の前の k 文字を
// 強調に使い、残りを文字のノードにするが、その区間を並びの頭に置いたままにする（goldmark v1.8.5 の
// Delimiter.ConsumeCharacters。開きの残りは正しい）。そのため残りの文字の区間が強調の閉じの記号と
// 重なる。前の兄弟の範囲の終わりより前から始まる記号だけの文字のノードは、その終わりへ置き直す
// （残りは、使われた記号のすぐ後ろに並ぶ）。
func (b *textBuilder) children(n ast.Node) ext {
	var first, last ext
	count, prev := 0, -1
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		var e ext
		if t, ok := c.(*ast.Text); ok && t.Segment.Start < prev && isDelimiterRun(t.Segment.Value(b.src)) {
			e = b.visitText(t, prev)
		} else {
			e = b.visit(c)
		}
		if count == 0 {
			first = e
		}
		last = e
		count++
		prev = -1
		if e.ok {
			prev = e.stop
		}
	}
	if count == 0 {
		return ext{}
	}

	return ext{start: first.start, stop: last.stop, ok: first.ok && last.ok}
}

// isDelimiterRun は v が強調か取り消し線の記号（`*` / `_` / `~`）だけの並びかを返す。
func isDelimiterRun(v []byte) bool {
	if len(v) == 0 || bytes.IndexByte([]byte("*_~"), v[0]) < 0 {
		return false
	}
	for _, c := range v {
		if c != v[0] {
			return false
		}
	}

	return true
}

// inline は見える文字の start から今までを中身とするインラインの要素を足す。
func (b *textBuilder) inline(kind string, start int, open, closing Span) {
	b.tb.Inlines = append(b.tb.Inlines, InlineSpan{
		Kind:  kind,
		Text:  Span{Start: start, Stop: len(b.tb.Visible)},
		Open:  Span{Start: open.Start + b.shift, Stop: open.Stop + b.shift},
		Close: Span{Start: closing.Start + b.shift, Stop: closing.Stop + b.shift},
	})
}

// contentEdges は見える文字の start から今までの、ソース上の最初の文字の頭と最後の文字の終わりを
// 返す（前処理の後の本文の上）。文字が無ければ今の位置。記号の区間が分からない要素に長さ 0 の区間を
// 置くのに使う（外側への差し込みや記号の取り除きが、何も変えないようにする）。
func (b *textBuilder) contentEdges(start int) (int, int) {
	if len(b.tb.Visible) == start {
		return b.cur, b.cur
	}

	return b.tb.Source[start].Start - b.shift, b.tb.Source[len(b.tb.Source)-1].Stop - b.shift
}
