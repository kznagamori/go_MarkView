package renderer

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// needsPlantUMLKey は、この変換で描画対象の PlantUML ブロックを出力したかを
// 記録する鍵（MD-085, NFR-013）。
//
// **取り込み指令で拒んだブロックは数えない**（IMP-119）。全部のブロックが
// 拒まれた文書で 5 MiB の資産を読むのは無駄である。
var needsPlantUMLKey = parser.NewContextKey()

var kindPlantUMLBlock = ast.NewNodeKind("PlantUMLBlock")

// plantUMLBlock は言語指定が plantuml / puml のフェンス付きコードブロック
// （IMP-119, MD-083）。
type plantUMLBlock struct {
	ast.BaseBlock

	source []byte // 図の元テキスト

	// rejected は取り込み指令を含むため描画対象から外したことを表す
	// （MD-084, NFR-032）。原文は残し、フロントエンドが理由とともに出す。
	rejected bool
}

func (n *plantUMLBlock) Kind() ast.NodeKind { return kindPlantUMLBlock }

func (n *plantUMLBlock) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Source":   string(n.source),
		"Rejected": map[bool]string{true: "true", false: "false"}[n.rejected],
	}, nil)
}

// plantUMLExtension は PlantUML ブロックの取り出しを登録する（IMP-111, IMP-119）。
type plantUMLExtension struct{}

func (plantUMLExtension) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		// Mermaid（85）の次に置く。互いに別の言語名を見るため結果は変わらないが、
		// 順序を決めておくと出力が実行ごとに揺れない。
		util.Prioritized(plantUMLTransformer{}, 86),
	))
	md.Renderer().AddOptions(gmrenderer.WithNodeRenderers(
		util.Prioritized(plantUMLRenderer{}, 500),
	))
}

// plantUMLLanguages は対象とする言語指定（IMP-119, MD-083）。
//
// **`uml` を入れてはならない。** MD-083 が対象外と定めている。
var plantUMLLanguages = []string{"plantuml", "puml"}

// plantUMLTransformer は plantuml / puml のコードブロックを専用ノードへ差し替える。
//
// ハイライト（IMP-114）に渡す前に取り除く。PlantUML は図であり、
// シンタックスハイライトの対象ではない。
type plantUMLTransformer struct{}

func (plantUMLTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
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
		if !isPlantUMLLanguage(b.Language(source)) {
			continue
		}

		src := bytes.TrimRight(b.Lines().Value(source), "\n")
		p := &plantUMLBlock{
			source:   src,
			rejected: hasIncludeDirective(string(src)),
		}
		b.Parent().ReplaceChild(b.Parent(), b, p)

		// **拒んだブロックでは立てない**（IMP-119, NFR-013）。
		if !p.rejected {
			pc.Set(needsPlantUMLKey, true)
		}
	}
}

// isPlantUMLLanguage は言語指定が PlantUML かどうかを返す（IMP-119）。
//
// **完全一致で見る。** 前方一致にすると `plantuml2` まで拾う。
// 大文字小文字は区別しない（chroma の言語名解決と揃える）。
func isPlantUMLLanguage(lang []byte) bool {
	for _, name := range plantUMLLanguages {
		if bytes.EqualFold(lang, []byte(name)) {
			return true
		}
	}
	return false
}

// includeDirectives は外部を取り込むプリプロセッサ指令（MD-084, IMP-119）。
//
// **`!includesub` と `!includeurl` を `!include` の前方一致で巻き込まない。**
// どれも拒むため結果は同じだが、将来 `!include` だけを許すことになったときに
// 壊れる。1 つずつ名指しして持つ。
var includeDirectives = []string{
	"!include",
	"!includeurl",
	"!includesub",
	"!import",
}

// hasIncludeDirective は PlantUML ソースが外部を取り込む指令を含むかを返す
// （MD-084, IMP-119, NFR-032）。
//
// **判定を Go 側に置くのは、描画処理系の振る舞いに依存しないためである**
// （AR-031）。資産は BR-043 で自動更新されるため、上流がリモート取得を
// 有効化してもこちらは気づかない。フロント側で XMLHttpRequest を潰す方式は
// 採らない（IMP-119）。
func hasIncludeDirective(source string) bool {
	for _, line := range strings.Split(source, "\n") {
		// **行頭にあるものだけを見る**（前に空白があってもよい）。
		// PlantUML のプリプロセッサは行頭でしか効かない。
		line = strings.TrimLeft(line, " \t")

		// コメント行は検査の対象外とする（IMP-119）。
		if strings.HasPrefix(line, "'") {
			continue
		}
		if !strings.HasPrefix(line, "!") {
			continue
		}

		lower := strings.ToLower(line)

		for _, directive := range includeDirectives {
			if _, ok := cutDirective(lower, directive); ok {
				return true
			}
		}

		// **`from` を伴う `!theme` だけを拒む**（MD-083, IMP-119）。
		// 組み込みテーマの `!theme plain` は使えると定めている。
		if rest, ok := cutDirective(lower, "!theme"); ok && hasFromKeyword(rest) {
			return true
		}
	}

	return false
}

// cutDirective は行が指令で始まるかを見て、続きを返す（IMP-119）。
//
// **指令名の直後が英数字であってはならない。** ここを見ないと `!include` が
// `!includesub` を巻き込み、指令ごとの判定にならない。
func cutDirective(lower, directive string) (rest string, ok bool) {
	if !strings.HasPrefix(lower, directive) {
		return "", false
	}

	rest = lower[len(directive):]
	if rest == "" {
		return rest, true
	}

	if r := []rune(rest)[0]; isASCIIAlnum(r) {
		return "", false
	}
	return rest, true
}

// hasFromKeyword は `!theme` の続きに from が語として現れるかを返す（IMP-119）。
func hasFromKeyword(rest string) bool {
	for _, field := range strings.Fields(rest) {
		if field == "from" {
			return true
		}
	}
	return false
}

// plantUMLRenderer は PlantUML ブロックを描画する（IMP-119）。
type plantUMLRenderer struct{}

func (plantUMLRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(kindPlantUMLBlock, renderPlantUML)
}

// renderPlantUML は IMP-119 が固定した構造を出力する。
//
// **data-source に原文を重複して持たせる。** PlantUML は描画後に <pre> が SVG へ
// 置き換わり、DOM から原文が失われる。理由は Mermaid（IMP-115）と同じで、
// コピーボタン（FR-060）とテーマ切り替え時の再描画（IMP-233）が要る。
//
// **拒んだブロックにも data-source を付ける。** そちらは <pre> が残るため
// 厳密には無くても足りるが、フロントエンドがコピー（DSP-272 の「描けなかった
// ブロックのコピーボタンから元のソースが取れる」）で場合分けせずに済む。
func renderPlantUML(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	block := n.(*plantUMLBlock)

	_, _ = w.WriteString(`<div class="code-block" data-lang="plantuml" `)
	if block.rejected {
		// フロントエンドはこれを見て理由を表示し、**描画を試みない**（DSP-272）。
		_, _ = w.WriteString(`data-puml-error="include" `)
	} else {
		_, _ = w.WriteString(`data-plantuml="1" `)
	}
	_, _ = w.WriteString(`data-source="`)
	_, _ = w.Write(escapeAttribute(block.source))
	_, _ = w.WriteString("\">\n")
	_, _ = w.WriteString(`<pre class="plantuml-source">`)
	_, _ = w.Write(util.EscapeHTML(block.source))
	_, _ = w.WriteString("</pre>\n</div>\n")

	return ast.WalkContinue, nil
}
