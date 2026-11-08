package renderer

import (
	"strconv"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// refKeyKey は変換 1 回分の目印の鍵を parser.Context へ置く鍵（IMP-120）。
//
// AST 変換器は 1 度しか登録されないため、変換ごとに変わる値は Context で渡す
// （baseDirKey と同じ。IMP-118）。
var refKeyKey = parser.NewContextKey()

// 目印の属性名（IMP-120）。
const (
	attrDataRef    = "data-ref"
	attrDataLink   = "data-link"
	attrDataLocked = "data-locked" // 編集できないコードブロック（値は 1 だけ。IMP-115, IMP-122）
)

// textRefsKey は修飾の目印（text:<n>）を付けるかを parser.Context へ置く鍵（Options.TextRefs。IMP-120）。
var textRefsKey = parser.NewContextKey()

// refVisitor は walkRefs が GFM の要素を渡す先（IMP-120, IMP-121）。
//
// 使わない種類の関数は nil でよい。
type refVisitor struct {
	// task は文書の中で index 番目のタスクのチェックボックスを受け取る。
	task func(n *extast.TaskCheckBox, index int)
	// table は文書の中で index 番目の GFM の表を受け取る。
	table func(n *extast.Table, index int)
	// cell は table 番目の表の row 行 col 列のセルを受け取る。row は 0 が見出し行。
	// **補われたセル（ソース上に存在しない。Lines().Len() == 0）も渡す**——列の番号を
	// 数えるためである。付けるか・位置を持つかは受け手が決める（isSourceCell）。
	cell func(n *extast.TableCell, table, row, col int)
	// link はリンク（*ast.Link）と自動リンク（*ast.AutoLink）を受け取る。
	link func(n ast.Node)

	// code は文書の中で index 番目のフェンス付きのコードブロック（図と数式を除く）を受け取る
	// （IMP-120。4.75.0）。字下げ形式のコードブロックは数えない。
	code func(n *ast.FencedCodeBlock, index int)
	// mermaid / plantuml は文書の中で index 番目の図のブロックを受け取る（描画に失敗するもの・
	// 取り込み指令で拒んだものも数える。FR-120 の「同じ対象」）。**図の番号も walkRefs で振る**
	// （4.75.0。図も書き換えの対象になったため。FR-145）。
	mermaid  func(n *mermaidBlock, index int)
	plantuml func(n *plantUMLBlock, index int)
	// text は文書の中で index 番目の修飾できるブロックを受け取る（IMP-120。4.75.0）。渡すのは
	// 目印を書く要素のノードである: Paragraph・Heading と、最初の子が TextBlock の ListItem
	// （TextBlock は要素を出さないため li に書く。2 つ目以降の TextBlock は数えない）。
	text func(n ast.Node, index int)
	// block は構文木のすべてのブロックのノードを行きがけ順に受け取る（Locations.Blocks。IMP-121）。
	block func(n ast.Node)
}

// walkRefs は GFM の要素を文書の出現順に辿り、番号を付けて v へ渡す（IMP-120）。
//
// **描画の目印（refTransformer）と書き換え位置の特定（Locate。IMP-121）は、
// 必ずこの関数で数える。** 数え方を 2 か所に書くと、描画で付けた番号と書き込みで
// 探す番号が食い違い、別のセルを書き換える（UT-218 ケース 1）。
//
// 生 HTML の `<input>` や `<table>` は goldmark のノードにならないため、ここには
// 現れず、番号にも入らない（UT-217 ケース 2）。コードブロックの中の `- [ ] x` も
// ノードにならない。
func walkRefs(doc ast.Node, v refVisitor) {
	tasks, tables, codes, mermaids, plantumls, texts := 0, 0, 0, 0, 0, 0

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if n.Type() == ast.TypeBlock && v.block != nil {
			v.block(n)
		}

		switch node := n.(type) {
		case *ast.FencedCodeBlock:
			// 図（mermaidBlock / plantUMLBlock）と数式（mathBlock）は、目印の変換より前に
			// 専用のノードへ差し替わっている。残ったものがコードブロックである。
			if v.code != nil {
				v.code(node, codes)
			}
			codes++

		case *mermaidBlock:
			if v.mermaid != nil {
				v.mermaid(node, mermaids)
			}
			mermaids++

		case *plantUMLBlock:
			if v.plantuml != nil {
				v.plantuml(node, plantumls)
			}
			plantumls++

		case *ast.Paragraph, *ast.Heading:
			if v.text != nil {
				v.text(node, texts)
			}
			texts++

		case *ast.ListItem:
			if _, ok := node.FirstChild().(*ast.TextBlock); ok {
				if v.text != nil {
					v.text(node, texts)
				}
				texts++
			}
		case *extast.TaskCheckBox:
			if v.task != nil {
				v.task(node, tasks)
			}
			tasks++

		case *extast.Table:
			if v.table != nil {
				v.table(node, tables)
			}
			if v.cell != nil {
				walkCells(node, tables, v.cell)
			}
			tables++
			// セルの中に表もタスクも現れない。リンクだけは中にありうるため辿る。

		case *ast.Link, *ast.AutoLink:
			if v.link != nil {
				v.link(node)
			}
		}
		return ast.WalkContinue, nil
	})
}

// walkCells は 1 つの表のセルを [行][列] の順に渡す。0 行目が見出し行。
func walkCells(table *extast.Table, index int, cell func(n *extast.TableCell, table, row, col int)) {
	row := 0
	for r := table.FirstChild(); r != nil; r = r.NextSibling() {
		// 見出し行（TableHeader）も本体の行（TableRow）も、子がセルである。
		col := 0
		for c := r.FirstChild(); c != nil; c = c.NextSibling() {
			if tc, ok := c.(*extast.TableCell); ok {
				cell(tc, index, row, col)
				col++
			}
		}
		row++
	}
}

// isSourceCell はセルがソース上に存在するかを返す（IMP-120）。
//
// 行のセル数が見出し行より少ないとき、goldmark は位置を持たない空の TableCell を
// 補う（extension/table.go の parseRow）。ソース上にある空のセル（`|  |`）は、
// 長さ 0 の区間を 1 つ持つため区別できる。
func isSourceCell(n *extast.TableCell) bool {
	return n.Lines().Len() > 0
}

// refExtension は編集・並べ替え・リンクの目印を付ける（IMP-120）。
type refExtension struct{}

func (refExtension) Extend(md goldmark.Markdown) {
	// **番号は、すべての AST 変換を終えた後の木（描画される木）の順で振る**（IMP-120, IMP-121。
	// BUG-027）。goldmark は優先度の小さい順に変換器を走らせるため、goldmark の脚注の変換（999。定義を
	// 文書の末尾へ移し、参照されていない定義を除き、参照の順に並べ替える）より大きい 1000 に置く。
	// **v1.1.0 は 80 で振っていた**——脚注の定義の中にタスクや表があると、目印は移す前の木の番号、
	// 書き換え位置（Locate）は移した後の木の番号になり、押したものと別のチェックボックスやセルを
	// 書き換えた（定義が本文より前・参照されていない定義・定義と参照の順の違い、の 3 つの形）。
	// Locate と Visible も同じ構成で構文解析するため、同じ木で数える。
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(refTransformer{}, 1000),
	))
	md.Renderer().AddOptions(gmrenderer.WithNodeRenderers(
		// goldmark 標準のタスクの描画器（優先度 500）を差し替える。登録は値の
		// 大きい順に行われ、後から登録した値の小さいものが勝つ。
		util.Prioritized(taskCheckBoxRenderer{}, 100),
	))
}

// refTransformer は GFM の要素に目印の属性を与える（IMP-120）。
//
// 表・セル・リンクは、属性を与えれば標準の描画器が data- 属性として出力する
// （html.RenderAttributes は data- で始まる属性をフィルタに関わらず出す）。
// タスクのチェックボックスだけは標準の描画器が属性を出さないため、
// taskCheckBoxRenderer が読む。
type refTransformer struct{}

func (refTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	key, _ := pc.Get(refKeyKey).(string)
	if key == "" {
		// 鍵が空なら目印を付けない（IMP-110。テストのためだけの形）。
		return
	}
	source := reader.Source()
	prefix := key + ":"
	textRefs, _ := pc.Get(textRefsKey).(bool)

	walkRefs(doc, refVisitor{
		code: func(n *ast.FencedCodeBlock, index int) {
			n.SetAttributeString(attrDataRef, []byte(prefix+"code:"+strconv.Itoa(index)))
			// 前置きが行ごとに違うものには data-locked="1"（FR-145。書き込みと同じ判定。IMP-122）。
			if fenceLocation(n, source, pc, 0).Locked {
				n.SetAttributeString(attrDataLocked, []byte("1"))
			}
		},
		mermaid: func(n *mermaidBlock, index int) {
			n.ref = diagramRef(key, "mermaid", index)
			n.locked = n.fence != nil && fenceLocation(n.fence, source, pc, 0).Locked
		},
		plantuml: func(n *plantUMLBlock, index int) {
			n.ref = diagramRef(key, "plantuml", index)
			n.locked = n.fence != nil && fenceLocation(n.fence, source, pc, 0).Locked
		},
		text: func(n ast.Node, index int) {
			// 修飾の目印は TextRefs が真のときだけ（編集モードの間だけ。IMP-120）。付けないときも
			// 番号は同じに数える（walkRefs が数える）。
			if textRefs {
				n.SetAttributeString(attrDataRef, []byte(prefix+"text:"+strconv.Itoa(index)))
			}
		},
		task: func(n *extast.TaskCheckBox, index int) {
			n.SetAttributeString(attrDataRef, []byte(prefix+"task:"+strconv.Itoa(index)))
		},
		table: func(n *extast.Table, index int) {
			n.SetAttributeString(attrDataRef, []byte(prefix+"table:"+strconv.Itoa(index)))
		},
		cell: func(n *extast.TableCell, table, row, col int) {
			// 補われたセルには付けない。付けると、フロントエンドがそこへ書き込もうと
			// して区切りを足し、表の形を変える（FR-142, UT-217 ケース 1・17）。
			if !isSourceCell(n) {
				return
			}
			n.SetAttributeString(attrDataRef, []byte(prefix+"cell:"+
				strconv.Itoa(table)+":"+strconv.Itoa(row)+":"+strconv.Itoa(col)))
		},
		link: func(n ast.Node) {
			var dest []byte
			switch l := n.(type) {
			case *ast.Link:
				// 書かれたとおりのリンク先は Destination とする（FR-063, IMP-120）。
				// 参照リンクは定義に書かれた形、山括弧は外れた形になる。
				dest = l.Destination
			case *ast.AutoLink:
				// URL() は www. に http:// を足すため使わない（UT-217 ケース 13）。
				// メールアドレスの mailto: は href にだけ付き、ここには入らない。
				dest = l.Label(source)
			}
			// 値のエスケープは描画器（html.RenderAttributes）が行う。
			n.SetAttributeString(attrDataLink, append([]byte(prefix), dest...))
		},
	})
}

// taskCheckBoxRenderer はタスクのチェックボックスを描画する（IMP-120）。
//
// **goldmark 標準の描画器と同じ文字列（属性の順と、末尾の半角空白 `> ` を含む）に
// data-ref を足しただけにする。** 違えると、ゴールデンテストの差分が目印以外にも
// 出る（UT-214）。XHTML 形式（` /> `）は使っていない（IMP-111 の構成）。
type taskCheckBoxRenderer struct{}

func (taskCheckBoxRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(extast.KindTaskCheckBox, renderTaskCheckBox)
}

func renderTaskCheckBox(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*extast.TaskCheckBox)

	if n.IsChecked {
		_, _ = w.WriteString(`<input checked="" disabled="" type="checkbox"`)
	} else {
		_, _ = w.WriteString(`<input disabled="" type="checkbox"`)
	}
	if v, ok := n.AttributeString(attrDataRef); ok {
		if b, ok := v.([]byte); ok {
			_, _ = w.WriteString(` ` + attrDataRef + `="`)
			_, _ = w.Write(util.EscapeHTML(b))
			_ = w.WriteByte('"')
		}
	}
	_, _ = w.WriteString("> ")
	return ast.WalkContinue, nil
}
