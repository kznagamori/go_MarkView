package renderer

import (
	"bytes"
	"strconv"

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

	// ref は図のブロックの目印の値（`<鍵>:mermaid:<n>`。IMP-115, IMP-120）。
	// 鍵が空なら空で、属性を出さない。**目印の変換（refTransformer）が walkRefs で振る**。
	ref string

	// locked は前置きが行ごとに違う（data-locked。FR-145。IMP-122）。目印の変換が決める。
	locked bool

	// fence は差し替える前のフェンス付きのコードブロック（囲いの位置を引く。IMP-121）。
	fence *ast.FencedCodeBlock
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

	// **番号はここでは振らない**（4.75.0）。目印の変換（refTransformer）が walkRefs で、ほかの目印と
	// 同じ木の順に振る（IMP-120。描画に失敗するものも数える）。
	for _, b := range blocks {
		if !bytes.EqualFold(b.Language(source), []byte("mermaid")) {
			continue
		}
		m := &mermaidBlock{
			source: bytes.TrimRight(b.Lines().Value(source), "\n"),
			fence:  b,
		}
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

	block := n.(*mermaidBlock)
	writeMermaidBlock(w, block.source, block.ref, block.locked)

	return ast.WalkContinue, nil
}

// blockWriter は図のブロックを書き出す先（goldmark の util.BufWriter と bytes.Buffer）。
type blockWriter interface {
	Write(p []byte) (int, error)
	WriteString(s string) (int, error)
}

// writeMermaidBlock は Mermaid の図のブロックを書く（IMP-115）。**Markdown の中の Mermaid の
// ブロックと Mermaid ファイル（IMP-127）が同じ関数を使う**——形が同じなら、フロントエンドの描画・
// コピー・原寸表示・拡大画面・テーマへの追従・目印の照合がそのまま働く（FR-026）。
func writeMermaidBlock(w blockWriter, src []byte, ref string, locked bool) {
	_, _ = w.WriteString(`<div class="code-block" data-lang="mermaid" data-mermaid="1" `)
	writeDiagramRef(w, ref)
	writeLocked(w, ref, locked)
	_, _ = w.WriteString(`data-source="`)
	_, _ = w.Write(escapeAttribute(src))
	_, _ = w.WriteString("\">\n")
	_, _ = w.WriteString(`<pre class="mermaid-source">`)
	_, _ = w.Write(util.EscapeHTML(src))
	_, _ = w.WriteString("</pre>\n</div>\n")
}

// diagramRef は図のブロックの目印の値を組み立てる（IMP-115, IMP-119, IMP-120）。
// 鍵が空なら空文字を返す（目印を付けない。IMP-110）。
func diagramRef(key, kind string, index int) string {
	if key == "" {
		return ""
	}
	return key + ":" + kind + ":" + strconv.Itoa(index)
}

// writeDiagramRef は図のブロックの data-ref 属性を、後ろに空白を付けて書く。
//
// **フロントエンドは鍵の合うブロックだけを描画し、その data-source だけを使う**
// （IMP-260）。生 HTML で class="code-block" と data-mermaid / data-plantuml と
// data-source を書けば同じ形を作れるため、目印が無いと Go 側の検査（IMP-119）を
// 経ていない図が描かれ、見えている内容と違う原文がコピーされる（BUG-014）。
func writeDiagramRef(w blockWriter, ref string) {
	if ref == "" {
		return
	}
	_, _ = w.WriteString(attrDataRef + `="`)
	_, _ = w.Write(escapeAttribute([]byte(ref)))
	_, _ = w.WriteString(`" `)
}

// writeLocked は、前置きが行ごとに違う図のブロックの data-locked 属性を、後ろに空白を付けて書く
// （FR-145。IMP-115, IMP-122）。目印を付けないとき（鍵が空）は書かない。
func writeLocked(w blockWriter, ref string, locked bool) {
	if ref == "" || !locked {
		return
	}
	_, _ = w.WriteString(attrDataLocked + `="1" `)
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
