package renderer

import (
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	gmrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// newHighlighting は chroma によるシンタックスハイライトを構成する（IMP-114）。
//
// TODO(T2-8): 登録言語を MD-031 の一覧へ限定するか（AR-033）を決める。
// 配色 CSS の生成（DSP-250）も T2-8 で扱う。
func newHighlighting() goldmark.Extender {
	return highlighting.NewHighlighting(
		highlighting.WithFormatOptions(
			// **クラスのみを出力し、インラインスタイルを書かせない。**
			// chroma が style 属性で色を書き込むと、テーマ切り替え（FR-070）の
			// たびに Markdown の再変換が必要になり、「ちらつきなく即座に
			// 切り替える」（UI-105）が満たせなくなる（IMP-114）。
			chromahtml.WithClasses(true),

			// 行番号は出さない（MD-032）。
			chromahtml.WithLineNumbers(false),
		),
		// 文書側が ```go {linenos=table} と書いても行番号を出させない。
		// goldmark-highlighting は info string の属性から行番号を有効に
		// できてしまうが、MD-032 は行番号を出さないことを求めている。
		// CodeBlockOptions は属性由来の設定より後に適用されるため、
		// ここで確実に打ち消せる。
		highlighting.WithCodeBlockOptions(func(highlighting.CodeBlockContext) []chromahtml.Option {
			return []chromahtml.Option{chromahtml.WithLineNumbers(false)}
		}),

		highlighting.WithWrapperRenderer(codeBlockWrapper),
	)
}

// codeBlockWrapper はフェンス付きコードブロックを共通のラッパで包む（IMP-115）。
//
// フロントエンドはこのラッパを目印に、コピーボタン（FR-060）の対象を見つける。
//
// goldmark-highlighting は、ハイライトした場合は chroma が出す <pre><code> を
// この関数の出力で挟むだけだが、**ハイライトしない場合は pre/code を書かない**。
// 未知の言語（UT-206 ケース 4）と言語指定なし（同 2）はその経路を通るため、
// ここで補う。
func codeBlockWrapper(w util.BufWriter, ctx highlighting.CodeBlockContext, entering bool) {
	if !entering {
		if !ctx.Highlighted() {
			_, _ = w.WriteString("</code></pre>")
		}
		_, _ = w.WriteString("</div>\n")
		return
	}

	_, _ = w.WriteString(`<div class="code-block"`)

	// 言語指定がない場合は data-lang を付けない（UT-206 ケース 2）。
	if lang, ok := ctx.Language(); ok {
		_, _ = w.WriteString(` data-lang="`)
		_, _ = w.Write(util.EscapeHTML(lang))
		_ = w.WriteByte('"')
	}
	_ = w.WriteByte('>')

	if !ctx.Highlighted() {
		_, _ = w.WriteString("<pre><code>")
	}
}

// codeBlockRenderer はインデント形式のコードブロックを描画する（IMP-115）。
//
// goldmark-highlighting が扱うのはフェンス付きだけであり、インデント形式は
// goldmark の既定の描画器が <pre><code> をそのまま出す。ラッパで包まれないと
// コピーボタンの対象から漏れるため（UT-206 ケース 3）、ここで置き換える。
type codeBlockRenderer struct{}

func (codeBlockRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindCodeBlock, renderIndentedCodeBlock)
}

func renderIndentedCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</code></pre></div>\n")
		return ast.WalkContinue, nil
	}

	// インデント形式には言語指定がないため data-lang は付かない。
	_, _ = w.WriteString(`<div class="code-block"><pre><code>`)

	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i) // Value はポインタレシーバのため一度変数へ受ける
		_, _ = w.Write(util.EscapeHTML(line.Value(source)))
	}

	return ast.WalkContinue, nil
}
