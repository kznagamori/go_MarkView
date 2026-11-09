package document

import (
	"bytes"
	"reflect"
	"sort"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// editformatpatch.go は修飾の組み立ての後半（IMP-124 の 8・9）を持つ。差し込みと取り除きを 1 つの Patch に
// まとめ（patch）、当てた結果を解き直して確かめる（keepsFormat）。入口の PlanFormat は editformat.go、
// 差し込む位置の決め方は editformatpos.go にある（400 行の目安。IMP-011）。

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
