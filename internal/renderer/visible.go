package renderer

// このファイルは見える文字の対応付け（IMP-123。4.75.0）を持つ。**画面の選択をソースの位置に対応
// 付けるのは Go 側である**（AR-031, FR-146）。フロントエンドは見える文字を数えて送るだけにし
// （IMP-266）、Go 側は構文木から同じ数え方で見える文字を組み立てて照合する（IMP-124 の 2）。
// 走査そのものは visiblebuild.go。

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/kznagamori/go_MarkView/internal/applog"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// errTextNotFound は、指定した修飾できるブロック（text:<n>）か、ソース上に存在するセルが無いことを表す。
var errTextNotFound = errors.New("text block not found")

// TextTarget は修飾するブロックの指定（document の Ref から作る）。
type TextTarget struct {
	Cell            bool // 表のセルなら真（Table / Row / Col を使う）
	Index           int  // text:<n> の n
	Table, Row, Col int
}

// TextBlock は 1 つのブロックの見える文字と、ソースとの対応。
type TextBlock struct {
	Visible    []rune       // 見える文字（IMP-123 の表の数え方）
	Source     []Span       // 見える文字 1 つずつの、ソース上の区間（正規化後のテキストの上）
	Atoms      []Span       // 丸ごとにしか囲めない部品の、見える文字の上の区間
	Inlines    []InlineSpan // インラインの要素（入れ子の外側から順）
	HardBreaks int          // ハード改行の数（IMP-124 の 9）
}

// InlineSpan はインラインの要素 1 つ。
type InlineSpan struct {
	// Kind は "strong" / "emphasis" / "strike"（記号）/ "link" / "code" / "image" /
	// "del" / "s" / "strike-tag" / "color"（生 HTML のタグの対）/ "html"（その他のタグの対）。
	Kind  string
	Text  Span   // 見える文字の上の区間（中身）
	Open  Span   // ソース上の開きの記号・タグの区間
	Close Span   // ソース上の閉じの記号・タグの区間
	Color string // Kind が "color" のときの色の名前（TextColors）
}

// TextColors は文字色の一覧（FR-147）。名前・ファイルに書く値（Light）・Dark で描く値の順。
//
// サニタイズ（IMP-116）と書き込み（IMP-124）の両方が使う。document → renderer の向きでしか
// 参照できないため renderer に置く（IMP-012）。**フロントエンドの色の一覧（IMP-265）と CSS の
// トークン（DSP-010）にも同じ値を書く**——名前と値は FR-147 の表が正である。
var TextColors = []struct{ Name, Light, Dark string }{
	{"red", "#d1242f", "#f85149"},
	{"orange", "#bc4c00", "#db6d28"},
	{"yellow", "#9a6700", "#d29922"},
	{"green", "#1a7f37", "#3fb950"},
	{"blue", "#0969da", "#4493f8"},
	{"purple", "#8250df", "#ab7df8"},
	{"gray", "#59636e", "#9198a1"},
}

// Visible は修飾するブロックの見える文字と、ソースとの対応を返す（IMP-123）。
//
// **Render・Locate と同じ goldmark の構成と前処理を使い**、目印と同じ walkRefs で対象を見つける
// （すべての変換を終えた後の木。IMP-120, BUG-027）。Source は前処理のずれを戻した位置である
// （IMP-121。UT-220 のケース 24）。対象が無ければエラー。パニックはエラーとして返す（IMP-022）。
func (r *Renderer) Visible(source []byte, target TextTarget) (tb *TextBlock, err error) {
	defer func() {
		if v := recover(); v != nil {
			applog.Recovered("renderer.Visible", v)
			tb, err = nil, fmt.Errorf("visible text failed: %v", v)
		}
	}()

	body, shift, doc, links := r.parseForText(source)

	var found ast.Node
	if target.Cell {
		walkRefs(doc, refVisitor{
			cell: func(n *extast.TableCell, table, row, col int) {
				if table == target.Table && row == target.Row && col == target.Col && isSourceCell(n) {
					found = n
				}
			},
		})
	} else {
		walkRefs(doc, refVisitor{
			text: func(n ast.Node, index int) {
				if index == target.Index {
					found = n
				}
			},
		})
	}
	if found == nil {
		return nil, fmt.Errorf("%w: %+v", errTextNotFound, target)
	}

	return buildText(textContainer(found), body, shift, links), nil
}

// VisibleTexts は、修飾できるすべてのブロック（text と cell）の見える文字を、目印から鍵を除いた値
// （"text:3" / "cell:0:1:2"）ごとに返す（描画スモークの照合。BR-054, IMP-123）。補われたセル
// （ソース上に存在しない）は入れない。
func (r *Renderer) VisibleTexts(source []byte) (texts map[string]string, err error) {
	defer func() {
		if v := recover(); v != nil {
			applog.Recovered("renderer.VisibleTexts", v)
			texts, err = nil, fmt.Errorf("visible texts failed: %v", v)
		}
	}()

	body, shift, doc, links := r.parseForText(source)

	texts = map[string]string{}
	walkRefs(doc, refVisitor{
		text: func(n ast.Node, index int) {
			texts["text:"+strconv.Itoa(index)] = string(buildText(textContainer(n), body, shift, links).Visible)
		},
		cell: func(n *extast.TableCell, table, row, col int) {
			if !isSourceCell(n) {
				return
			}
			key := "cell:" + strconv.Itoa(table) + ":" + strconv.Itoa(row) + ":" + strconv.Itoa(col)
			texts[key] = string(buildText(n, body, shift, links).Visible)
		},
	})

	return texts, nil
}

// parseForText は Render・Locate と同じ構成と前処理で構文木だけを作る（HTML を出さない）。
// 鍵は空（目印を付けない）、基準ディレクトリは置かない（画像の書き換えと os.Stat をしない。IMP-118）。
// links はリンクと画像の閉じの位置（linkend.go）。
func (r *Renderer) parseForText(source []byte) (body []byte, shift int, doc ast.Node, links linkMarks) {
	body, shift = preprocess(source)

	pc := parser.NewContext()
	pc.Set(refKeyKey, "")
	doc = r.md.Parser().Parse(text.NewReader(body), parser.WithContext(pc))

	return body, shift, doc, linkMarksOf(pc)
}

// textContainer は、修飾の目印を書く要素のノードから、見える文字を数えるノードを返す。リストの項目は
// 最初の子の TextBlock の中だけを数える（入れ子のリストやコードブロックと、2 つ目以降の TextBlock を
// 含まない。IMP-120, IMP-123。4.78.0）。
func textContainer(n ast.Node) ast.Node {
	if li, ok := n.(*ast.ListItem); ok && li.FirstChild() != nil {
		return li.FirstChild()
	}

	return n
}
