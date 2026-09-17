package renderer

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	"github.com/kznagamori/go_MarkView/internal/applog"

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
	attrDataRef  = "data-ref"
	attrDataLink = "data-link"
)

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
	tasks, tables := 0, 0

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
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
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(refTransformer{}, 80),
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

	walkRefs(doc, refVisitor{
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

// Span はソース上の半開区間 [Start, Stop)（IMP-121）。
type Span struct{ Start, Stop int }

// CellSpan は 1 つのセルの位置（IMP-121）。
type CellSpan struct {
	Content Span // 前後の空白を除いた内容（空のセルでは Start == Stop）
	Between Span // 前後の区切り（|）の間。空白を含む
}

// TableLocation は 1 つの GFM の表の位置（IMP-121）。
type TableLocation struct {
	Cells [][]*CellSpan // [行][列]。0 行目が見出し行。nil はソース上に存在しないセル
}

// Locations は書き換え位置の一覧（IMP-121）。位置は正規化後のテキストの上の値である。
type Locations struct {
	Tasks  []Span          // 各タスクの括弧の中の 1 文字
	Tables []TableLocation // GFM の表
}

// taskListPattern は GFM のタスクリストの正規表現（goldmark v1.8.5 の
// extension/tasklist.go と同じ）。括弧の中の 1 文字を求めるのに使う（IMP-121）。
var taskListPattern = regexp.MustCompile(`^\[([\sxX])\]\s*`)

// Locate は変換を行わずに構文木だけを作り、書き換え位置を返す（IMP-121）。
//
// **Render と同じ goldmark の構成と前処理を使い、HTML は出力しない。** サニタイズも
// 通さない。番号は描画の目印（IMP-120）と同じ walkRefs で数える。
//
// 位置は source（正規化後のテキスト）の上のバイト位置である。生バイト列の上の
// 位置へ移すのは document の役目である（IMP-106）。
func (r *Renderer) Locate(source []byte) (locs Locations, err error) {
	// 構文木の組み立て中のパニックをエラーへ変える（IMP-022, FR-111）。
	defer func() {
		if v := recover(); v != nil {
			applog.Recovered("renderer.Locate", v)
			locs, err = Locations{}, fmt.Errorf("markdown locating failed: %v", v)
		}
	}()

	body, shift := preprocess(source)

	// Render と同じ変換器が走る。変換ごとの値は Context で渡す（鍵は空＝目印を付けない）。
	pc := parser.NewContext()
	pc.Set(refKeyKey, "")
	doc := r.md.Parser().Parse(text.NewReader(body), parser.WithContext(pc))

	locs = Locations{Tasks: []Span{}, Tables: []TableLocation{}}
	walkRefs(doc, refVisitor{
		task: func(n *extast.TaskCheckBox, _ int) {
			locs.Tasks = append(locs.Tasks, taskSpan(n, body, shift))
		},
		table: func(_ *extast.Table, _ int) {
			locs.Tables = append(locs.Tables, TableLocation{})
		},
		cell: func(n *extast.TableCell, table, row, _ int) {
			t := &locs.Tables[table]
			for len(t.Cells) <= row {
				t.Cells = append(t.Cells, nil)
			}
			// 列は walkCells が 0 から順に渡すため、append で列の番号の位置に入る。
			var span *CellSpan
			if isSourceCell(n) {
				span = cellSpan(n, body, shift)
			}
			// ソース上に存在しないセルは nil のまま（IMP-121）。
			t.Cells[row] = append(t.Cells[row], span)
		},
	})
	return locs, nil
}

// taskSpan はタスクの括弧の中の 1 文字の位置を返す（IMP-121）。
//
// TaskCheckBox の親（TextBlock または Paragraph）の先頭行は括弧から始まる
// （goldmark のタスクの解析はその行の先頭で正規表現に一致したときだけノードを作る。
// リスト記号の後ろが空白 2 つ・タブ・入れ子・引用の中でも、行の区間は `[` を指す）。
//
// 一致しない（構文木と食い違う）場合は、長さ 0 の区間を返す。document は括弧の
// 前後と中の 1 バイトを確かめてから書き換えるため（IMP-106）、書き換えない。
func taskSpan(n *extast.TaskCheckBox, body []byte, shift int) Span {
	parent := n.Parent()
	if parent == nil || parent.Lines().Len() == 0 {
		return Span{}
	}
	seg := parent.Lines().At(0)
	line := body[seg.Start:seg.Stop]
	m := taskListPattern.FindSubmatchIndex(line)
	if m == nil {
		return Span{Start: seg.Start + shift, Stop: seg.Start + shift}
	}
	return Span{Start: seg.Start + m[2] + shift, Stop: seg.Start + m[3] + shift}
}

// cellSpan はソース上に存在するセルの位置を返す（IMP-121）。
//
// Content は goldmark が持つ区間（前後の空白を除いた内容。空のセルでは長さ 0 で、
// 閉じる `|` の位置）。Between はそれを区切りの `|` の直後・直前まで広げたもの。
// 広げるのは空白とタブだけで、行の外へは出ない。
//
//   - 行頭に `|` が無い行の最初のセルは、行の内容の先頭（行の Pos。引用の `> ` などの
//     後ろ）で止まる（UT-218 ケース 9）
//   - 行末に `|` が無い行の最後のセルは、行の終わりで止まる
func cellSpan(n *extast.TableCell, body []byte, shift int) *CellSpan {
	seg := n.Lines().At(0)

	rowStart := 0
	if row := n.Parent(); row != nil && row.Pos() >= 0 {
		rowStart = row.Pos()
	}
	rowEnd := len(body)
	if i := bytes.IndexByte(body[seg.Stop:], '\n'); i >= 0 {
		rowEnd = seg.Stop + i
	}

	// 左へは空白だけを越える。止まった位置の直前が区切りの `|` か、行の内容の先頭である。
	start := seg.Start
	for start > rowStart && isBlankByte(body[start-1]) {
		start--
	}

	// 右へも空白だけを越える。止まった位置が区切りの `|` か、行の終わりである。
	stop := seg.Stop
	for stop < rowEnd && isBlankByte(body[stop]) {
		stop++
	}

	return &CellSpan{
		Content: Span{Start: seg.Start + shift, Stop: seg.Stop + shift},
		Between: Span{Start: start + shift, Stop: stop + shift},
	}
}

// isBlankByte は行の中の空白（半角空白とタブ）かを返す。改行は含めない。
func isBlankByte(b byte) bool {
	return b == ' ' || b == '\t'
}
