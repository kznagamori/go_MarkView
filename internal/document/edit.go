package document

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// edit.go は書き換え位置の対応（IMP-106）のうち、目印の解読・Patch・生バイト列との
// 位置の対応・チェックボックスとセルのソースを持つ。セルの書き換え（PlanCell）は
// editcell.go にある。

// RefKind は書き換え対象の種類（IMP-106）。
type RefKind int

const (
	RefTask RefKind = iota // タスクリストのチェックボックス（FR-141）
	RefCell                // 表のセル（FR-142）
)

// Ref はフロントエンドから届く書き換え対象の指示を解いたもの（IMP-120 の目印の値）。
type Ref struct {
	Key   string // 描画の鍵（Document.RefKey と照合する）
	Kind  RefKind
	Index int // RefTask: 文書の中で何番目のタスクか（0 起点）。RefCell: 何番目の表か
	Row   int // RefCell: 行（0 が見出し行、1 以降が本体の行。ソース上の順）
	Col   int // RefCell: 列（0 起点）
}

// 書き換えの番兵エラー（IMP-106, IMP-021）。
var (
	ErrBadRef      = errors.New("malformed edit reference")
	ErrRefNotFound = errors.New("edit target not found")
	ErrNotEditable = errors.New("document is not editable")
	ErrChanged     = errors.New("file changed on disk")
)

// refPattern は書き込みの対象の目印の値の形（IMP-106, IMP-120）。
//
// **サニタイズが通す形（IMP-116 の editRefPattern）と揃える。** 鍵は小文字の 16 進
// 16 文字、数は符号なしの 10 進数だけを受け付ける。`strconv.Atoi` だけで解くと
// `+3` を受け付けてしまう（UT-108 ケース 5）。表の目印（`table:<t>`）と図の目印
// （`mermaid:<n>` / `plantuml:<n>`）は書き込みの対象ではないため、ここでは形に含めない。
var refPattern = regexp.MustCompile(`^([0-9a-f]{16}):(?:task:([0-9]+)|cell:([0-9]+):([0-9]+):([0-9]+))$`)

// ParseRef は data-ref 属性の値（IMP-120）を解く。形が違えば ErrBadRef。
// 書き込みの対象（task / cell）以外の目印（table / mermaid / plantuml。IMP-120）も ErrBadRef とする。
// 数は IMP-116 の正規表現と同じ形（符号なしの 10 進数）だけを受け付け、int に収まらなければ ErrBadRef。
func ParseRef(s string) (Ref, error) {
	m := refPattern.FindStringSubmatch(s)
	if m == nil {
		return Ref{}, ErrBadRef
	}

	// 部分一致の添字: 1 鍵、2 タスクの番号、3〜5 セルの表・行・列。
	// 使わなかった側の群は空文字になる。
	if m[2] != "" {
		index, ok := parseIndex(m[2])
		if !ok {
			return Ref{}, ErrBadRef
		}
		return Ref{Key: m[1], Kind: RefTask, Index: index}, nil
	}

	table, ok1 := parseIndex(m[3])
	row, ok2 := parseIndex(m[4])
	col, ok3 := parseIndex(m[5])
	if !ok1 || !ok2 || !ok3 {
		return Ref{}, ErrBadRef
	}
	return Ref{Key: m[1], Kind: RefCell, Index: table, Row: row, Col: col}, nil
}

// parseIndex は符号なしの 10 進数を int に解く。int に収まらなければ ok は偽
// （UT-108 ケース 9。panic しない）。形は refPattern が確かめ済みである。
func parseIndex(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// Patch は生バイト列の 1 か所の置き換え。取り消し（IMP-108）にもそのまま使う。
type Patch struct {
	Offset int    // 生バイト列の上の位置
	Old    []byte // 置き換える前のバイト列
	New    []byte // 置き換えた後のバイト列
}

// Apply は raw に p を適用した新しいバイト列を返す。
// raw[Offset:Offset+len(Old)] が Old と一致しなければ ErrChanged を返す。
//
// **raw は書き換えない**（新しいバイト列を返す。UT-111 ケース 7）。範囲の外と
// 一致しない場合は、書き換えたものを返さない（ケース 2〜4・9）。`Old` が空で
// `Offset` が長さと等しいのは末尾への挿入であり、範囲の内側である（ケース 8）。
func (p Patch) Apply(raw []byte) ([]byte, error) {
	// Offset+len(Old) を先に足すと、極端な Offset で int があふれうる。引き算で比べる。
	if p.Offset < 0 || p.Offset > len(raw) || len(p.Old) > len(raw)-p.Offset {
		return nil, ErrChanged
	}
	end := p.Offset + len(p.Old)
	if !bytes.Equal(raw[p.Offset:end], p.Old) {
		return nil, ErrChanged
	}

	out := make([]byte, 0, len(raw)-len(p.Old)+len(p.New))
	out = append(out, raw[:p.Offset]...)
	out = append(out, p.New...)
	out = append(out, raw[end:]...)
	return out, nil
}

// Inverse は p を打ち消す Patch を返す（Old と New を入れ替える）。
//
// 位置は同じである。p を適用した後のバイト列では、Offset から New が並んでいる。
// 取り消しの履歴（IMP-108）に積んだ後で呼び出し側のスライスが書き換えられても
// 影響しないよう、写しを持つ。
func (p Patch) Inverse() Patch {
	return Patch{Offset: p.Offset, Old: bytes.Clone(p.New), New: bytes.Clone(p.Old)}
}

// rawMap は正規化後のテキストの位置を、生バイト列の位置へ移す対応表（IMP-106 の手順 1）。
//
// 対応が崩れるのは、先頭の UTF-8 BOM（3 バイト）の除去と、CRLF の LF への
// 置き換え（1 バイト減る）の 2 つだけである。単独の CR は LF へ置き換わるが長さを
// 変えない。不正なバイト列の置き換えは長さを変えるため、対応表を作らない
// （mapRaw が ErrNotEditable を返す）。
type rawMap struct {
	bom int // 除いた BOM のバイト数（0 か 3）
	// crlf は CRLF を 1 つの LF にまとめた位置（正規化後のテキストの上。昇順）。
	// normalizeNewlines は CR の位置に LF を書いて続く LF を読み飛ばすため、
	// その位置より後ろが生バイト列では 1 バイトずつ後ろへずれる。
	crlf []int
}

// toRaw は正規化後のテキストの位置 pos を、生バイト列の位置へ移す。
// pos より前にまとめた CRLF の数だけ後ろへずらす（二分探索で数える）。
func (m rawMap) toRaw(pos int) int {
	return pos + m.bom + sort.SearchInts(m.crlf, pos)
}

// mapRaw は raw を IMP-103 と同じ規則で正規化し、位置の対応表を作る（IMP-106 の手順 1）。
//
// **不正なバイト列の置き換えが起きる場合は ErrNotEditable を返す。** 置き換えは長さを
// 変え、位置を対応させられない。FR-140 はこの文書で編集モードを始めさせないが、
// 書き込みの直前に読み直した内容で改めて確かめる（その間に外部で壊されうる）。
func mapRaw(raw []byte) (text []byte, m rawMap, err error) {
	text, replaced := Normalize(raw)
	if replaced {
		return nil, rawMap{}, ErrNotEditable
	}

	body := raw
	if bytes.HasPrefix(raw, utf8BOM) {
		m.bom = len(utf8BOM)
		body = raw[len(utf8BOM):]
	}

	// 正規化（normalizeNewlines）と同じ走り方で、まとめた CRLF の位置を控える。
	j := 0 // 正規化後の位置
	for i := 0; i < len(body); i++ {
		if body[i] == '\r' && i+1 < len(body) && body[i+1] == '\n' {
			m.crlf = append(m.crlf, j)
			i++
		}
		j++
	}
	if j != len(text) {
		// 正規化の規則と対応表の作り方が食い違った。位置を信じない。
		return nil, rawMap{}, ErrNotEditable
	}
	return text, m, nil
}

// locate は raw の対応表と、正規化後のテキストの上の書き換え位置を返す（IMP-106 の手順 1・2）。
func locate(r *renderer.Renderer, raw []byte) (text []byte, m rawMap, locs renderer.Locations, err error) {
	text, m, err = mapRaw(raw)
	if err != nil {
		return nil, rawMap{}, renderer.Locations{}, err
	}
	locs, err = r.Locate(text)
	if err != nil {
		// 構文木を作れない文書では位置を決められない。書き込まない。
		return nil, rawMap{}, renderer.Locations{}, fmt.Errorf("%w: %v", ErrNotEditable, err)
	}
	return text, m, locs, nil
}

// PlanTask は raw の中で ref のチェックボックスを checked にする Patch を返す（FR-141）。
// 既にその状態なら changed は false。
//
// 置き換えるのは括弧の中の 1 バイトだけである。`x` / `X` はオン、それ以外（goldmark の
// タスクの正規表現の `\s` に当たる半角空白・タブ・改ページ）はオフとみなす。**反転では
// なく「この状態にする」で受け取る**——同じ指示が 2 度届いても元へ戻らない。
func PlanTask(r *renderer.Renderer, raw []byte, ref Ref, checked bool) (p Patch, changed bool, err error) {
	if ref.Kind != RefTask {
		return Patch{}, false, ErrBadRef
	}
	_, m, locs, err := locate(r, raw)
	if err != nil {
		return Patch{}, false, err
	}
	if ref.Index < 0 || ref.Index >= len(locs.Tasks) {
		return Patch{}, false, ErrRefNotFound
	}

	span := locs.Tasks[ref.Index]
	if span.Stop-span.Start != 1 {
		// Locate が括弧の中を求められなかった（構文木と食い違った）。
		return Patch{}, false, ErrNotEditable
	}
	off := m.toRaw(span.Start)

	// **求めた位置の前後が [ と ] で、中の 1 バイトがタスクの文字であることを確かめる。**
	// 位置の求め方が構文木と食い違ったときに、別のバイトを書き換えないための防御である
	// （リストの字下げのタブは桁に換算されうる。IMP-106）。
	if off < 1 || off+1 >= len(raw) || raw[off-1] != '[' || raw[off+1] != ']' || !isTaskByte(raw[off]) {
		return Patch{}, false, ErrNotEditable
	}

	cur := raw[off]
	on := cur == 'x' || cur == 'X'
	if on == checked {
		return Patch{}, false, nil
	}

	next := byte(' ')
	if checked {
		next = 'x'
	}
	return Patch{Offset: off, Old: []byte{cur}, New: []byte{next}}, true, nil
}

// isTaskByte はタスクの括弧の中に置ける 1 バイトかを返す（IMP-106）。
func isTaskByte(b byte) bool {
	switch b {
	case ' ', '\t', '\f', 'x', 'X':
		return true
	}
	return false
}

// CellSource は raw の中の ref のセルのソースを返す（FR-142 の編集欄の初期値）。
//
// 前後の空白を除いた内容を、記法もエスケープした縦棒の `\` も含めて書かれたとおりに
// 返す（IMP-121）。正規化後のテキストから切り出すため CR を含まない。
func CellSource(r *renderer.Renderer, raw []byte, ref Ref) (string, error) {
	text, _, locs, err := locate(r, raw)
	if err != nil {
		return "", err
	}
	cell, err := findCell(locs, ref)
	if err != nil {
		return "", err
	}
	return string(text[cell.Content.Start:cell.Content.Stop]), nil
}

// findCell は ref のセルの位置を返す。範囲の外と、ソース上に存在しないセル（補われた
// セル）は ErrRefNotFound とする（IMP-106）。panic しない。
func findCell(locs renderer.Locations, ref Ref) (*renderer.CellSpan, error) {
	if ref.Kind != RefCell {
		return nil, ErrBadRef
	}
	if ref.Index < 0 || ref.Index >= len(locs.Tables) {
		return nil, ErrRefNotFound
	}
	cells := locs.Tables[ref.Index].Cells
	if ref.Row < 0 || ref.Row >= len(cells) || ref.Col < 0 || ref.Col >= len(cells[ref.Row]) {
		return nil, ErrRefNotFound
	}
	cell := cells[ref.Row][ref.Col]
	if cell == nil {
		// **区切りを足すことになり、表の形を変える**（FR-142, UT-110 ケース 12）。
		return nil, ErrRefNotFound
	}
	return cell, nil
}
