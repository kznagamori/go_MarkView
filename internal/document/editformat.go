package document

import (
	"bytes"
	"reflect"
	"sort"
	"unicode"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// editformat.go は修飾の書き込み（IMP-124。4.75.0）を持つ。**書くのは `<del>` と決めた色の `<span>` を
// 差し込む・取り除くことと、手書きの取り消し線の記号・タグを取り除くことだけで、既存の記法を組み立て
// 直さない**（2.14 の IMPORTANT の 4）。見える文字とソースの対応は renderer が求める（IMP-123）。
// 差し込む位置の決め方は editformatpos.go にある。

// FormatStyle は修飾の操作。Strike と Color のどちらか一方だけを使う。
type FormatStyle struct {
	Strike bool   // 取り消し線のトグル（FR-146）
	Color  string // 文字色の名前（FR-147。renderer.TextColors の名前か "default"）
}

// FormatOp はフロントエンドから届く修飾の指示（FR-146, FR-147。IMP-316）。
type FormatOp struct {
	Start, End int    // ブロックの見える文字の中の、Unicode の符号位置での位置（半開区間）
	Visible    string // そのブロックの見える文字の全体（照合用。IMP-123, IMP-266）
	Style      FormatStyle
}

// colorDefault は「色を外す」を表す名前（FR-147）。
const colorDefault = "default"

// PlanFormat は ref のブロックの選んだ文字に修飾を付ける・外す Patch を返す。
// widened は、手書きの取り消し線を全体で外した（FR-146。IMP-195 が format-widened の警告を出す）。
//
// 手順は IMP-124 の 1〜9 の順に固定する。フロントエンドの値は信じない（IMP-300, NFR-030）——見える文字が
// 1 文字でも違う・位置が範囲の外・操作が 2 つの形のどちらでもない指示は ErrRefNotFound（IMP-109 が
// ErrStale にして通知しない）。分けられない・確かめで構造が変わるなら ErrCannotFormat（書き込まない）。
func PlanFormat(r *renderer.Renderer, raw []byte, ref Ref, op FormatOp) (p Patch, changed, widened bool, err error) {
	target, err := textTarget(ref)
	if err != nil {
		return Patch{}, false, false, err
	}
	// 1. 正規化と位置の対応表。不正なバイト列は ErrNotEditable。
	text, m, err := mapRaw(raw)
	if err != nil {
		return Patch{}, false, false, err
	}
	// 2. 操作の形と、見える文字の照合。
	color, ok := styleColor(op.Style)
	if !ok {
		return Patch{}, false, false, ErrRefNotFound
	}
	tb, err := r.Visible(text, target)
	if err != nil || string(tb.Visible) != op.Visible || op.Start < 0 || op.Start >= op.End || op.End > len(tb.Visible) {
		return Patch{}, false, false, ErrRefNotFound
	}
	// 3. 選択を整える。空になれば何もしない。
	sel, ok := selection(tb, op.Start, op.End)
	if !ok {
		return Patch{}, false, false, nil
	}

	// 4〜7. 区間ごとに差し込み・取り除きを決める。
	f := newFormatter(tb, text, raw, m, sel)
	if op.Style.Strike {
		err = f.strike()
	} else {
		err = f.recolor(color)
	}
	if err != nil {
		return Patch{}, false, false, err
	}

	// 8. 1 つの Patch にまとめる。何も変えないなら changed は偽。
	p, ok = f.patch()
	if !ok {
		return Patch{}, false, false, ErrCannotFormat
	}
	if bytes.Equal(p.Old, p.New) {
		return Patch{}, false, false, nil
	}

	// 9. 解き直して確かめる。
	if !keepsFormat(r, raw, text, target, f, p, op.Style.Strike, color) {
		return Patch{}, false, false, ErrCannotFormat
	}
	return p, true, f.widened, nil
}

// textTarget は修飾の目印（text:<n> と表のセル）を renderer の指定に写す。ほかの種類は ErrBadRef。
func textTarget(ref Ref) (renderer.TextTarget, error) {
	switch ref.Kind {
	case RefText:
		return renderer.TextTarget{Index: ref.Index}, nil
	case RefCell:
		return renderer.TextTarget{Cell: true, Table: ref.Index, Row: ref.Row, Col: ref.Col}, nil
	}
	return renderer.TextTarget{}, ErrBadRef
}

// styleColor は操作の形を確かめ、文字色の操作なら色の名前（default は空文字）を返す。
// 受け付けるのは「Strike が真で Color が空」と「Strike が偽で Color が TextColors の名前か default」の
// 2 つの形だけである（IMP-124 の 2。4.76.0。UT-121 のケース 3・4）。
func styleColor(s FormatStyle) (string, bool) {
	if s.Strike {
		return "", s.Color == ""
	}
	if s.Color == colorDefault {
		return "", true
	}
	if colorValue(s.Color) != "" {
		return s.Color, true
	}
	return "", false
}

// colorValue は色の名前のファイルに書く値（Light）を返す。一覧に無ければ空文字。
func colorValue(name string) string {
	for _, c := range renderer.TextColors {
		if c.Name == name {
			return c.Light
		}
	}
	return ""
}

// colorOpen は文字色の開きタグを決めた形で返す（コロンの後に半角空白 1 つ、小文字の 16 進。FR-147）。
func colorOpen(name string) []byte {
	return []byte(`<span style="color: ` + colorValue(name) + `">`)
}

// styleAttr は開きタグの style 属性（名前から値の終わりまで）の区間を返す。色を替えるときに値だけを
// 書き直す（IMP-124 の 6）。**属性を頭から順に読む**——正規表現で `style=` を探すと、`data-style="x"` や
// ほかの属性の値の中の `style=` に当たる（P24 の独立したレビューの指摘 2。UT-120 のケース 14）。
func styleAttr(tag []byte) (start, end int, ok bool) {
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
	i := 1 // '<' の後ろ
	for i < len(tag) && !isSpace(tag[i]) && tag[i] != '>' && tag[i] != '/' {
		i++ // タグの名前
	}
	for i < len(tag) {
		for i < len(tag) && (isSpace(tag[i]) || tag[i] == '/') {
			i++
		}
		if i >= len(tag) || tag[i] == '>' {
			break
		}
		nameStart := i
		for i < len(tag) && !isSpace(tag[i]) && tag[i] != '=' && tag[i] != '>' && tag[i] != '/' {
			i++
		}
		name := tag[nameStart:i]
		j := i
		for j < len(tag) && isSpace(tag[j]) {
			j++
		}
		if j < len(tag) && tag[j] == '=' {
			j++
			for j < len(tag) && isSpace(tag[j]) {
				j++
			}
			switch {
			case j < len(tag) && (tag[j] == '"' || tag[j] == '\''):
				if k := bytes.IndexByte(tag[j+1:], tag[j]); k >= 0 {
					j += k + 2
				} else {
					j = len(tag)
				}
			default:
				for j < len(tag) && !isSpace(tag[j]) && tag[j] != '>' {
					j++
				}
			}
			i = j
		}
		if bytes.EqualFold(name, []byte("style")) {
			return nameStart, i, true
		}
	}
	return 0, 0, false
}

// selection は選択を整える（IMP-124 の 3）。両端の空白を除き、脚注の戻りリンク（ソースに無い 2 文字）に
// かかっていたら広げずに選択から外して改めて空白を除き、ほかの部品（Atoms）の一部にかかっていたら
// その全体まで広げる。空になれば ok は偽（4.78.0。UT-120 のケース 23）。
func selection(tb *renderer.TextBlock, start, end int) (renderer.Span, bool) {
	sel := trimSpace(tb.Visible, renderer.Span{Start: start, Stop: end})
	for _, a := range tb.Atoms {
		if !isBacklink(tb, a) || !overlaps(a, sel) {
			continue
		}
		if a.Start <= sel.Start {
			sel.Start = max(sel.Start, a.Stop)
		} else {
			sel.Stop = min(sel.Stop, a.Start)
		}
	}
	sel = trimSpace(tb.Visible, sel)
	if sel.Start >= sel.Stop {
		return renderer.Span{}, false
	}
	for _, a := range tb.Atoms {
		if !isBacklink(tb, a) && overlaps(a, sel) {
			sel.Start, sel.Stop = min(sel.Start, a.Start), max(sel.Stop, a.Stop)
		}
	}
	return sel, true
}

// trimSpace は区間の両端の空白（改行と U+00A0 を含む）を除く。
func trimSpace(vis []rune, s renderer.Span) renderer.Span {
	for s.Start < s.Stop && unicode.IsSpace(vis[s.Start]) {
		s.Start++
	}
	for s.Stop > s.Start && unicode.IsSpace(vis[s.Stop-1]) {
		s.Stop--
	}
	return s
}

// isBacklink は部品が脚注の戻りリンク（ソース上の区間がすべて長さ 0。IMP-123）かを返す。
func isBacklink(tb *renderer.TextBlock, a renderer.Span) bool {
	for i := a.Start; i < a.Stop; i++ {
		if tb.Source[i].Start != tb.Source[i].Stop {
			return false
		}
	}
	return a.Stop > a.Start
}

func contains(outer, inner renderer.Span) bool {
	return outer.Start <= inner.Start && inner.Stop <= outer.Stop
}

func overlaps(a, b renderer.Span) bool {
	return a.Start < b.Stop && b.Start < a.Stop
}

// isAtomKind は丸ごとにだけ囲める要素（インラインコードと画像）か。区間の境界を作らない。
func isAtomKind(kind string) bool { return kind == "code" || kind == "image" }

// strikeKinds は取り消し線の要素（MarkView の形の del と、手書きの記号・タグ）。
var strikeKinds = map[string]bool{"strike": true, "del": true, "s": true, "strike-tag": true}

// throughKinds は、区間がその中身の全体と一致したら外側に差し込む要素（IMP-124 の 4）。太字・斜体・
// 取り消し線（MarkView の形の del を含む）・リンク。**色の span は含めない**——取り消し線は色の span の
// 内側に入り、色の span は取り消し線の外側に入る（「色が外、取り消し線が内」。4.80.0。UT-120 の 24・25）。
var throughKinds = map[string]bool{
	"strong": true, "emphasis": true, "link": true,
	"strike": true, "del": true, "s": true, "strike-tag": true,
}

// struckAt は見える文字 i が取り消し線の中か。
func struckAt(tb *renderer.TextBlock, i int) bool {
	for _, e := range tb.Inlines {
		if strikeKinds[e.Kind] && e.Text.Start <= i && i < e.Text.Stop {
			return true
		}
	}
	return false
}

// colorIndexAt は見える文字 i を囲む最も内側の色の span の添字を返す。無ければ -1。
func colorIndexAt(tb *renderer.TextBlock, i int) int {
	found := -1
	for k, e := range tb.Inlines {
		if e.Kind == "color" && e.Text.Start <= i && i < e.Text.Stop {
			found = k // 外側から順に並ぶので、最後に当たったものが最も内側
		}
	}
	return found
}

// colorAt は見える文字 i の色の名前を返す（色が無ければ空文字）。
func colorAt(tb *renderer.TextBlock, i int) string {
	if k := colorIndexAt(tb, i); k >= 0 {
		return tb.Inlines[k].Color
	}
	return ""
}

// inlineShape は確かめで比べるインラインの要素（種類と中身の区間）。
type inlineShape struct {
	Kind string
	Text renderer.Span
}

// shapeOf はインラインの要素の並びを返す。MarkView の形の del と色の span と、skip の添字（外した
// 手書きの取り消し線）は除く（IMP-124 の 9。4.78.0）。
func shapeOf(tb *renderer.TextBlock, skip map[int]bool) []inlineShape {
	out := []inlineShape{}
	for k, e := range tb.Inlines {
		if e.Kind == "del" || e.Kind == "color" || skip[k] {
			continue
		}
		out = append(out, inlineShape{Kind: e.Kind, Text: e.Text})
	}
	return out
}

// keepsFormat は Patch を当てた結果を解き直して確かめる（IMP-124 の 9）。
//
//   - ブロックの並びと、タスク・表・コードブロック・図・修飾できるブロックの数が変わらない
//   - そのブロックの見える文字が 1 文字も変わらない
//   - インラインの要素の並び（MarkView の形の del と色の span を除く）とハード改行の数が変わらない
//     ——日本語に接した太字が効かなくなるのを捕まえる（見える文字は同じままのため 2 番目では捕まらない）
//   - 選んだ文字の取り消し線と色が意図どおりで、選んでいない文字は変わらない。**手書きの取り消し線を
//     全体で外したときは、外れた範囲の全体を選んだ文字とみなし、外したものを並びの比べから除く**
func keepsFormat(r *renderer.Renderer, raw, text []byte, target renderer.TextTarget, f *formatter, p Patch, strike bool, color string) bool {
	before, err := r.Locate(text)
	if err != nil {
		return false
	}
	afterText, got, ok := relocate(r, raw, p)
	if !ok || !sameStructure(before, got) {
		return false
	}
	after, err := r.Visible(afterText, target)
	if err != nil || string(after.Visible) != string(f.tb.Visible) || after.HardBreaks != f.tb.HardBreaks {
		return false
	}
	if !reflect.DeepEqual(shapeOf(f.tb, f.removed), shapeOf(after, nil)) {
		return false
	}

	for i := range f.tb.Visible {
		wantStruck, wantColor := struckAt(f.tb, i), colorAt(f.tb, i)
		if f.chosen(i) {
			if strike {
				wantStruck = f.add
			} else {
				wantColor = color
			}
		}
		if struckAt(after, i) != wantStruck || colorAt(after, i) != wantColor {
			return false
		}
	}
	return true
}

// insertion は生バイト列の位置 pos への差し込み。同じ位置では閉じタグ（rank 0）を開きタグ（rank 1）より
// 先に置く（前の区間の閉じと次の区間の開き、分けたタグの閉じと新しいタグの開き）。
type insertion struct {
	pos, rank, seq int
	text           []byte
}

const (
	rankClose = 0
	rankOpen  = 1
)

// patch は差し込みと取り除きを、最初の変更から最後の変更までの 1 つの Patch にまとめる（IMP-124 の 8）。
// 取り除く区間が重なる・差し込む位置が取り除く区間の内側にあるなら ok は偽（組み立てが崩れた）。
func (f *formatter) patch() (Patch, bool) {
	if len(f.ins) == 0 && len(f.dels) == 0 {
		return Patch{}, true
	}
	ins := append([]insertion(nil), f.ins...)
	sort.Slice(ins, func(i, j int) bool {
		a, b := ins[i], ins[j]
		if a.pos != b.pos {
			return a.pos < b.pos
		}
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		return a.seq < b.seq
	})
	dels := append([]renderer.Span(nil), f.dels...)
	sort.Slice(dels, func(i, j int) bool { return dels[i].Start < dels[j].Start })

	first, last := len(f.raw), 0
	for _, d := range dels {
		first, last = min(first, d.Start), max(last, d.Stop)
	}
	for i, d := range dels {
		if i > 0 && d.Start < dels[i-1].Stop {
			return Patch{}, false
		}
		for _, in := range ins {
			if d.Start < in.pos && in.pos < d.Stop {
				return Patch{}, false
			}
		}
	}
	for _, in := range ins {
		first, last = min(first, in.pos), max(last, in.pos)
	}

	var out bytes.Buffer
	ii, di := 0, 0
	for cur := first; ; {
		for ii < len(ins) && ins[ii].pos == cur {
			out.Write(ins[ii].text)
			ii++
		}
		if cur >= last {
			break
		}
		if di < len(dels) && dels[di].Start == cur {
			cur = dels[di].Stop
			di++
			continue
		}
		next := last
		if ii < len(ins) {
			next = min(next, ins[ii].pos)
		}
		if di < len(dels) {
			next = min(next, dels[di].Start)
		}
		out.Write(f.raw[cur:next])
		cur = next
	}
	return Patch{Offset: first, Old: bytes.Clone(f.raw[first:last]), New: out.Bytes()}, true
}
