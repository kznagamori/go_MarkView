package renderer

// このファイルは書き換え位置の特定（IMP-121）を持つ。editref.go が 400 行の目安（IMP-011）を
// 超えたため分けた（v1.2.0。目印の付与と番号付けは editref.go に残る。数え方は walkRefs の 1 つ）。

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/kznagamori/go_MarkView/internal/applog"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

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

// FenceLocation はフェンス付きのコードブロック・図のブロックの位置（IMP-121, IMP-122。4.75.0）。
type FenceLocation struct {
	Open   Span // 開きの囲いの文字の並び（「```」の部分）
	Close  Span // 閉じの囲いの文字の並び（Closed が偽なら中身の終わりの位置で、長さ 0）
	Closed bool // 閉じの囲いがあるか（文書や引用の終わりで閉じたものは偽）
	// Body は中身の行の全体（開きの囲いの行の次の行の頭から、閉じの囲いの行の頭まで。
	// 閉じが無ければ最後の中身の行の終わりまで。開きの囲いの行に改行が無ければ、文書の終わりで長さ 0）。
	Body   Span
	Lines  []Span // 中身の各行の区間（前置きの後ろから、改行の手前まで）
	Info   string // 言語指定（確かめに使う）
	Prefix string // ブロックの前置き（IMP-122）
	Locked bool   // 前置きが行ごとに違う・タブの途中から始まる（FR-145。IMP-122）
}

// Locations は書き換え位置の一覧（IMP-121）。位置は正規化後のテキストの上の値である。
type Locations struct {
	Tasks  []Span          // 各タスクの括弧の中の 1 文字
	Tables []TableLocation // GFM の表

	Codes     []FenceLocation // code:<n>（4.75.0）
	Mermaids  []FenceLocation // mermaid:<n>
	PlantUMLs []FenceLocation // plantuml:<n>
	Texts     int             // 修飾できるブロックの数（text:<n>。位置は IMP-123 が求める）
	Blocks    []string        // 構文木の順のブロックの種類（ノードの Kind の名前）。書き込みの確かめに使う（IMP-122, IMP-124）
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

	locs = Locations{
		Tasks: []Span{}, Tables: []TableLocation{},
		Codes: []FenceLocation{}, Mermaids: []FenceLocation{}, PlantUMLs: []FenceLocation{},
	}
	fence := func(fc *ast.FencedCodeBlock) FenceLocation {
		if fc == nil {
			return FenceLocation{}
		}
		return fenceLocation(fc, body, pc, shift)
	}
	walkRefs(doc, refVisitor{
		code: func(n *ast.FencedCodeBlock, _ int) {
			locs.Codes = append(locs.Codes, fence(n))
		},
		mermaid: func(n *mermaidBlock, _ int) {
			locs.Mermaids = append(locs.Mermaids, fence(n.fence))
		},
		plantuml: func(n *plantUMLBlock, _ int) {
			locs.PlantUMLs = append(locs.PlantUMLs, fence(n.fence))
		},
		text: func(ast.Node, int) {
			locs.Texts++
		},
		block: func(n ast.Node) {
			locs.Blocks = append(locs.Blocks, n.Kind().String())
		},
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
