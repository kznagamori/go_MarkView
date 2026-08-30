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

// needsMermaidKey は、この変換で Mermaid ブロックを出力したかを記録する鍵。
//
// Mermaid はフロントエンドで描画する（AR-031）。含まない文書では
// mermaid.min.js を読み込まない（NFR-013）。
var needsMermaidKey = parser.NewContextKey()

var kindMermaidBlock = ast.NewNodeKind("MermaidBlock")

// mermaidBlock は言語指定が mermaid のフェンス付きコードブロック（IMP-115, MD-080）。
type mermaidBlock struct {
	ast.BaseBlock

	source []byte // 図の元テキスト
}

func (n *mermaidBlock) Kind() ast.NodeKind { return kindMermaidBlock }

func (n *mermaidBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Source": string(n.source)}, nil)
}

// mermaidExtension は Mermaid ブロックの取り出しを登録する（IMP-111, IMP-115）。
type mermaidExtension struct{}

func (mermaidExtension) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(mermaidTransformer{}, 85),
	))
	md.Renderer().AddOptions(gmrenderer.WithNodeRenderers(
		util.Prioritized(mermaidRenderer{}, 500),
	))
}

// mermaidTransformer は mermaid のコードブロックを専用ノードへ差し替える。
//
// ハイライト（IMP-114）に渡す前に取り除く。Mermaid は図であり、
// シンタックスハイライトの対象ではない。
type mermaidTransformer struct{}

func (mermaidTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()

	// 走査しながら木を書き換えないよう、対象を集めてから差し替える。
	var blocks []*ast.FencedCodeBlock
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if b, ok := n.(*ast.FencedCodeBlock); ok && entering {
			blocks = append(blocks, b)
		}
		return ast.WalkContinue, nil
	})

	for _, b := range blocks {
		if !bytes.EqualFold(b.Language(source), []byte("mermaid")) {
			continue
		}
		m := &mermaidBlock{source: bytes.TrimRight(b.Lines().Value(source), "\n")}
		b.Parent().ReplaceChild(b.Parent(), b, m)
		pc.Set(needsMermaidKey, true)
	}
}

// mermaidRenderer は Mermaid ブロックを描画する（IMP-115）。
type mermaidRenderer struct{}

func (mermaidRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(kindMermaidBlock, renderMermaid)
}

// renderMermaid は IMP-115 が固定した構造を出力する。
//
// **data-source に原文を重複して持たせる。** Mermaid は描画後に <pre> が SVG へ
// 置き換わり、DOM から原文が失われる。これがないと描画後にコピーボタン
// （FR-060）がソースを取れず、テーマ切り替え時の再描画（IMP-231）もできない。
func renderMermaid(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	src := n.(*mermaidBlock).source

	_, _ = w.WriteString(`<div class="code-block" data-lang="mermaid" data-mermaid="1" data-source="`)
	_, _ = w.Write(escapeAttribute(src))
	_, _ = w.WriteString("\">\n")
	_, _ = w.WriteString(`<pre class="mermaid-source">`)
	_, _ = w.Write(util.EscapeHTML(src))
	_, _ = w.WriteString("</pre>\n</div>\n")

	return ast.WalkContinue, nil
}

// escapeAttribute は HTML 属性値としてエスケープする（IMP-115）。
//
// 改行は &#10; にする。Base64 等の追加のエンコードは行わない。
// デバッグ時に属性値を目視できる形を保つためである。
func escapeAttribute(v []byte) []byte {
	var b bytes.Buffer
	b.Grow(len(v))

	for _, c := range v {
		switch c {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\n':
			b.WriteString("&#10;")
		default:
			b.WriteByte(c)
		}
	}

	return b.Bytes()
}
