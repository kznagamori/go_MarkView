package renderer

// このファイルは、リンクと画像の閉じの括弧 `]` と、構文の終わり（宛先の `)` や参照の `]` の後ろ）の
// 位置を、変換ごとの文脈（parser.Context）に残す（IMP-123 の InlineSpan の Open / Close）。
//
// goldmark v1.8.5 はリンクと画像の開きの位置を Pos() に置く（`[` / `![` の頭。link.go）が、閉じの
// 位置を残さない。見える文字の対応付けは、リンクの外側に差し込む（IMP-124 の 4）ために、リンクの記号
// の区間を要る。後からソースを読んで `(…)` を解くと、入れ子の括弧・山括弧の宛先・複数行のタイトル・
// 参照リンクの形で goldmark と食い違いうる。そこで標準のリンクの構文解析器を包み、リンクを作った時点
// の読み位置を記録する。
//
// **標準のものを置き換えて包む（並べて登録しない）。** リンクの構文解析器は、`]` で失敗したときに開き
// の括弧の記録を取り除く（副作用がある）。包んだものと標準のものを両方登録すると、包んだものが失敗した
// 後に標準のものが同じ `]` をもう一度読み、外側のリンクの開きの括弧まで取り除いてしまう（フェンスの
// 包み方（fence.go）は、Open に副作用が無いので並べて登録できる）。

import (
	"reflect"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// linkMarksKey は、変換 1 回分のリンクと画像の記録を parser.Context へ置く鍵。
var linkMarksKey = parser.NewContextKey()

// linkMark は 1 つのリンク（画像）の閉じの括弧 `]` の位置と、構文の終わりの位置（前処理の後の本文の上）。
type linkMark struct {
	bracket int
	end     int
}

// linkMarks はリンクと画像のノードごとの記録。
type linkMarks map[ast.Node]linkMark

// linkMarksOf は文脈の記録を返す。無ければ作って置く。
func linkMarksOf(pc parser.Context) linkMarks {
	if m, ok := pc.Get(linkMarksKey).(linkMarks); ok {
		return m
	}
	m := linkMarks{}
	pc.Set(linkMarksKey, m)

	return m
}

// newParser は goldmark の既定の構文解析器（goldmark.DefaultParser と同じ構成）を、リンクの構文解析器
// だけ linkRecorder で包んで作る。Render・Locate・Visible は同じ構成を使う。
func newParser() parser.Parser {
	inlines := parser.DefaultInlineParsers()
	linkType := reflect.TypeOf(parser.NewLinkParser())
	for i, v := range inlines {
		if ip, ok := v.Value.(parser.InlineParser); ok && reflect.TypeOf(v.Value) == linkType {
			inlines[i] = util.Prioritized(linkRecorder{InlineParser: ip}, v.Priority)
		}
	}

	return parser.NewParser(
		parser.WithBlockParsers(parser.DefaultBlockParsers()...),
		parser.WithInlineParsers(inlines...),
		parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
	)
}

// linkRecorder は標準のリンクの構文解析器を包む。判断はすべて標準のものに任せ、リンクか画像を作った
// ときだけ、呼ばれた位置（閉じの括弧 `]`）と、読み終えた位置（構文の終わり）を記録する。
type linkRecorder struct {
	parser.InlineParser
}

// Parse は標準の Parse を呼び、リンクか画像を返したら位置を記録する。
func (p linkRecorder) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	_, at := block.Position()
	n := p.InlineParser.Parse(parent, block, pc)
	switch n.(type) {
	case *ast.Link, *ast.Image:
		_, after := block.Position()
		linkMarksOf(pc)[n] = linkMark{bracket: at.Start, end: after.Start}
	}

	return n
}

// CloseBlock は標準のものへ渡す（閉じなかった括弧を文字に戻す。parser.CloseBlocker）。包んだ型が
// これを持たないと、goldmark は登録のときに CloseBlocker と見なさない。
func (p linkRecorder) CloseBlock(parent ast.Node, block text.Reader, pc parser.Context) {
	if cb, ok := p.InlineParser.(parser.CloseBlocker); ok {
		cb.CloseBlock(parent, block, pc)
	}
}
