package renderer

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// needsKaTeXKey は、この変換で数式を 1 つ以上出力したかを記録する鍵（IMP-113）。
//
// KaTeX はフロントエンドで実行する（AR-031）。Go 側の役割は数式を他の
// Markdown 記法から守り、フロントが識別できる形で出すことに限る。
// このフラグが false の文書では KaTeX を読み込まない（MD-061, NFR-013）。
var needsKaTeXKey = parser.NewContextKey()

var (
	kindMathInline = ast.NewNodeKind("MathInline")
	kindMathBlock  = ast.NewNodeKind("MathBlock")
)

// mathInline は $...$ のインライン数式（IMP-113）。
type mathInline struct {
	ast.BaseInline

	segment text.Segment // TeX ソースの位置
}

func (n *mathInline) Kind() ast.NodeKind { return kindMathInline }

func (n *mathInline) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"TeX": string(n.segment.Value(source))}, nil)
}

// mathBlock は $$...$$ および ```math のブロック数式（IMP-113, MD-060）。
type mathBlock struct {
	ast.BaseBlock

	value []byte // TeX ソース。複数行を連結するため位置ではなく実体を持つ
}

func (n *mathBlock) Kind() ast.NodeKind { return kindMathBlock }

func (n *mathBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"TeX": string(n.value)}, nil)
}

// mathExtension は数式の保護を登録する（IMP-111, IMP-113）。
type mathExtension struct{}

func (mathExtension) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(mathInlineParser{}, 500)),
		parser.WithASTTransformers(util.Prioritized(mathTransformer{}, 80)),
	)
	md.Renderer().AddOptions(gmrenderer.WithNodeRenderers(
		util.Prioritized(mathRenderer{}, 500),
	))
}

// mathInlineParser は $...$ を 1 つのノードとして取り込む（IMP-113）。
//
// インラインパーサとして実装するのは、**中身を Markdown として解釈させない**
// ためである。`$x_1 + x_2$` の `_` が強調になってしまう実装を避けるのが、
// この保護の動機そのものである（UT-205 ケース 5）。
//
// コードスパンの内側は対象にならない。goldmark は行を左から処理し、
// バッククォートに当たった時点でコードスパンを丸ごと取り込むため、内側の
// $ はこのパーサへ渡ってこない（IMP-113, UT-205 ケース 6）。
type mathInlineParser struct{}

func (mathInlineParser) Trigger() []byte { return []byte{'$'} }

func (mathInlineParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if len(line) < 2 {
		return nil
	}

	// $$ は段落の途中では数式にしない。ブロック数式は mathTransformer が
	// 段落ごと差し替える。ここで 2 文字を本文として送り出しておかないと、
	// 内側が単独の $ として拾われ、$<span>…</span>$ のような出力になる。
	if line[1] == '$' {
		block.Advance(2)
		return ast.NewTextSegment(seg.WithStop(seg.Start + 2))
	}

	// 開始の $ の直後が空白なら数式ではない（MD-060）。
	// この規則が `$100 と $200` を通貨表記のまま残す。
	if util.IsSpace(line[1]) {
		return nil
	}

	for i := 2; i < len(line); i++ {
		if line[i] != '$' {
			continue
		}
		// 終了の $ の直前が空白なら、その $ は終端ではない（MD-060）。
		if util.IsSpace(line[i-1]) {
			continue
		}

		block.Advance(i + 1)
		pc.Set(needsKaTeXKey, true)
		return &mathInline{segment: text.NewSegment(seg.Start+1, seg.Start+i)}
	}

	// 同じ行に終端がない。数式ではないので $ は本文のまま残す。
	return nil
}

// mathTransformer はブロック数式を専用ノードへ差し替える（IMP-113, MD-060）。
//
// 対象は 2 つ。
//
//   - 段落全体が $$…$$ であるもの。インライン解析の結果は捨て、段落の生テキスト
//     （Lines）から TeX を取り出す。ブロック要素を段落の内側に置けないため、
//     インラインパーサではなくここで段落ごと差し替える。
//   - 言語指定が math のフェンス付きコードブロック。
type mathTransformer struct{}

func (mathTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()

	// 走査しながら木を書き換えないよう、対象を集めてから差し替える。
	var targets []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.(type) {
		case *ast.Paragraph, *ast.FencedCodeBlock:
			targets = append(targets, n)
		}
		return ast.WalkContinue, nil
	})

	for _, n := range targets {
		b := mathBlockFrom(n, source)
		if b == nil {
			continue
		}
		n.Parent().ReplaceChild(n.Parent(), n, b)
		pc.Set(needsKaTeXKey, true)
	}
}

// mathBlockFrom はノードがブロック数式であれば、対応するノードを返す。
func mathBlockFrom(n ast.Node, source []byte) *mathBlock {
	switch v := n.(type) {
	case *ast.FencedCodeBlock:
		if !bytes.EqualFold(v.Language(source), []byte("math")) {
			return nil
		}
		return &mathBlock{value: bytes.TrimRight(v.Lines().Value(source), "\n")}

	case *ast.Paragraph:
		raw := bytes.TrimSpace(v.Lines().Value(source))
		// 開きと閉じで 4 文字。$$ だけの段落や $$$$ は数式にしない。
		if len(raw) < 5 || !bytes.HasPrefix(raw, []byte("$$")) || !bytes.HasSuffix(raw, []byte("$$")) {
			return nil
		}
		inner := bytes.TrimSpace(raw[2 : len(raw)-2])
		if len(inner) == 0 {
			return nil
		}
		return &mathBlock{value: inner}
	}

	return nil
}

// mathRenderer は数式ノードを描画する（IMP-113）。
type mathRenderer struct{}

func (mathRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(kindMathInline, renderMathInline)
	reg.Register(kindMathBlock, renderMathBlock)
}

// renderMathInline は <span class="math-inline"> を出力する。
//
// 中身は TeX ソースそのものであり、HTML としてエスケープして入れる。
// フロントエンドはこの要素の textContent を KaTeX へ渡す（IMP-232）。
func renderMathInline(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<span class="math-inline">`)
	_, _ = w.Write(util.EscapeHTML(n.(*mathInline).segment.Value(source)))
	_, _ = w.WriteString("</span>")

	return ast.WalkContinue, nil
}

// renderMathBlock は <div class="math-block"> を出力する。
func renderMathBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	_, _ = w.WriteString(`<div class="math-block">`)
	_, _ = w.Write(util.EscapeHTML(n.(*mathBlock).value))
	_, _ = w.WriteString("</div>\n")

	return ast.WalkContinue, nil
}
