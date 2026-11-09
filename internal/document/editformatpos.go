package document

import (
	"bytes"
	"sort"
	"unicode"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// editformatpos.go は修飾の差し込み・取り除きの位置の決め方（IMP-124 の 4〜7）を持つ。入口の PlanFormat は
// editformat.go、Patch へのまとめと解き直しての確かめは editformatpatch.go にある（400 行の目安。IMP-011）。
//
// 位置はすべて正規化後のテキストの上で求め、記録するときに生バイト列の位置へ移す（IMP-106 の対応表）。

// formatter は 1 回の修飾の組み立て。
type formatter struct {
	tb   *renderer.TextBlock
	text []byte // 正規化後のテキスト
	raw  []byte
	m    rawMap
	sel  renderer.Span // 整えた選択（見える文字の上）

	ins     []insertion
	dels    []renderer.Span // 生バイト列の上の取り除く区間
	add     bool            // 取り消し線を付けるか（外すなら偽）
	widened bool            // 手書きの取り消し線を、選んだ範囲より広く外した
	removed map[int]bool    // 外した手書きの取り消し線（Inlines の添字）
}

func newFormatter(tb *renderer.TextBlock, text, raw []byte, m rawMap, sel renderer.Span) *formatter {
	return &formatter{tb: tb, text: text, raw: raw, m: m, sel: sel, removed: map[int]bool{}}
}

// chosen は見える文字 i が「選んだ文字」か（確かめに使う）。手書きの取り消し線を全体で外したときは、
// 外れた範囲の全体を選んだ文字とみなす（IMP-124 の 9。4.78.0）。
func (f *formatter) chosen(i int) bool {
	if f.sel.Start <= i && i < f.sel.Stop {
		return true
	}
	for k := range f.removed {
		if t := f.tb.Inlines[k].Text; t.Start <= i && i < t.Stop {
			return true
		}
	}
	return false
}

// insert は正規化後の位置 pos への差し込みを記録する。
func (f *formatter) insert(pos int, text []byte, rank int) {
	f.ins = append(f.ins, insertion{pos: f.m.toRaw(pos), rank: rank, seq: len(f.ins), text: text})
}

// remove は正規化後の区間 s の取り除きを記録する（同じ区間は 1 度だけ）。
func (f *formatter) remove(s renderer.Span) {
	r := renderer.Span{Start: f.m.toRaw(s.Start), Stop: f.m.toRaw(s.Stop)}
	for _, d := range f.dels {
		if d == r {
			return
		}
	}
	f.dels = append(f.dels, r)
}

// rawOf は正規化後の区間 s の、生バイト列のままのバイト列を返す（動かすタグを書いたとおりに書き戻す）。
func (f *formatter) rawOf(s renderer.Span) []byte {
	return bytes.Clone(f.raw[f.m.toRaw(s.Start):f.m.toRaw(s.Stop)])
}

// segments は選択を、インラインの要素の境界で区切った区間の並びを返す（IMP-124 の 4）。境界を作るのは
// 丸ごとにだけ囲める要素（インラインコードと画像）以外の要素の、中身の頭と終わりである。
func (f *formatter) segments() []renderer.Span {
	cuts := []int{f.sel.Start, f.sel.Stop}
	for _, e := range f.tb.Inlines {
		if isAtomKind(e.Kind) {
			continue
		}
		for _, b := range []int{e.Text.Start, e.Text.Stop} {
			if f.sel.Start < b && b < f.sel.Stop {
				cuts = append(cuts, b)
			}
		}
	}
	sort.Ints(cuts)
	var out []renderer.Span
	for i := 1; i < len(cuts); i++ {
		if cuts[i] > cuts[i-1] {
			out = append(out, renderer.Span{Start: cuts[i-1], Stop: cuts[i]})
		}
	}
	return out
}

// container は区間 s を囲む要素のうち、差し込みの外側の限りになるもの（Inlines の添字。無ければ -1）を
// 返す。内側から見て、中身が s と一致する throughKinds の要素は越え（その外側に差し込む）、それ以外の
// 最初の要素で止まる。丸ごとにだけ囲める要素は区間の内側にしか無いので見ない。
func (f *formatter) container(s renderer.Span) int {
	for k := len(f.tb.Inlines) - 1; k >= 0; k-- {
		e := f.tb.Inlines[k]
		if isAtomKind(e.Kind) || e.Text.Start == e.Text.Stop || !contains(e.Text, s) {
			continue
		}
		if e.Text == s && throughKinds[e.Kind] {
			continue
		}
		return k
	}
	return -1
}

// positions は区間 s の両端の差し込み位置（正規化後のテキストの上）を返す。c は外側の限りの要素
// （container。-1 はブロック）。c の内側の要素のうち、s の頭から始まり s に収まるものの開きの前、s の
// 終わりで終わり s に収まるものの閉じの後ろまで広げる（中身の全体と一致する要素の外側、インライン
// コードと画像の記号の外側）。角括弧の自動リンク（`<https://…>`）は `<` と `>` の外側にする。
func (f *formatter) positions(s renderer.Span, c int) (left, right int) {
	tb := f.tb
	left = tb.Source[s.Start].Start
	right = tb.Source[s.Stop-1].Stop
	for k, e := range tb.Inlines {
		if k <= c || e.Text.Start == e.Text.Stop || (c >= 0 && !contains(tb.Inlines[c].Text, e.Text)) {
			continue
		}
		if e.Text.Start == s.Start && e.Text.Stop <= s.Stop {
			left = min(left, e.Open.Start)
		}
		if e.Text.Stop == s.Stop && e.Text.Start >= s.Start {
			right = max(right, e.Close.Stop)
		}
	}
	for _, a := range tb.Atoms {
		if !f.angleAutolink(a) {
			continue
		}
		if a.Start == s.Start {
			left = min(left, tb.Source[a.Start].Start-1)
		}
		if a.Stop == s.Stop {
			right = max(right, tb.Source[a.Stop-1].Stop+1)
		}
	}
	// 改行で終わる区間の閉じは、次の行の前置き（引用の `> ` やリストの字下げ）の後ろに置く。改行の直後
	// （行の頭）に置くと、その行が前置きを失って遅延継続行になり、`>` が文字になる（P24 の独立した
	// レビューの指摘 1。UT-120 のケース 28）。改行の手前に置くと、改行が取り消し線の外に出る。
	if q := tb.Source[s.Stop-1].Stop; tb.Visible[s.Stop-1] == '\n' && right == q && s.Stop < len(tb.Visible) {
		right = f.lineContent(q, s.Stop)
	}
	return left, right
}

// lineContent は、改行の直後の位置 q から、次の行のインラインの中身が始まる位置（前置きの後ろ）を返す。
// next は次の行の最初の見える文字。その文字か、そこから始まる要素の記号・タグか、角括弧の自動リンクの
// `<` のうち、q 以降で最も前の位置とする。
func (f *formatter) lineContent(q, next int) int {
	tb := f.tb
	best := tb.Source[next].Start
	for _, e := range tb.Inlines {
		for _, p := range []int{e.Open.Start, e.Close.Start} {
			if q <= p && p < best {
				best = p
			}
		}
	}
	for _, a := range tb.Atoms {
		if a.Start == next && f.angleAutolink(a) {
			best = min(best, tb.Source[a.Start].Start-1)
		}
	}
	return max(best, q)
}

// angleAutolink は部品が角括弧の自動リンクか（中の文字ごとに区間を持ち、インラインの要素として並ばず、
// 前後が `<` と `>`）。renderer は自動リンクの中の文字だけに区間を付ける（IMP-123）。
func (f *formatter) angleAutolink(a renderer.Span) bool {
	tb := f.tb
	if a.Stop-a.Start < 2 || tb.Source[a.Start] == tb.Source[a.Stop-1] {
		return false
	}
	for _, e := range tb.Inlines {
		if e.Text == a {
			return false
		}
	}
	start, stop := tb.Source[a.Start].Start, tb.Source[a.Stop-1].Stop
	return start > 0 && stop < len(f.text) && f.text[start-1] == '<' && f.text[stop] == '>'
}

// wrap は区間 s を open と closing で囲む（取り消し線と色を付ける。IMP-124 の 5・6）。
func (f *formatter) wrap(s renderer.Span, open, closing []byte) {
	left, right := f.positions(s, f.container(s))
	f.insert(left, open, rankOpen)
	f.insert(right, closing, rankClose)
}

// crosses は、要素 t の中で区間 s を分けると、t の内側の別の要素の境界をまたぐかを返す（IMP-124 の 7。
// `<del>あ**いう**え</del>` の「う」だけを外すと、`</del>` を `**` の内側に置くことになる）。
func (f *formatter) crosses(t int, s renderer.Span) bool {
	outer := f.tb.Inlines[t]
	for k, e := range f.tb.Inlines {
		if k <= t || e.Text.Start == e.Text.Stop || !contains(outer.Text, e.Text) {
			continue
		}
		if overlaps(e.Text, s) && !contains(s, e.Text) {
			return true
		}
	}
	return false
}

// split は要素 t（MarkView の形の del か色の span）の中身から区間 s を外へ出す。s の前で閉じ、後ろで開き
// 直す。**s が中身の頭や終わりに接しているときは、空のタグを作らないよう、元のタグを取り除いて s の反対
// 側に書き戻す**（IMP-124 の 5・6。UT-120 のケース 4・8）。書き戻すタグは書かれたとおりのバイト列とする。
// 差し込んだ両端の位置を返す（色を替えるときに、そこへ新しい色を差し込む）。
func (f *formatter) split(t int, s renderer.Span) (left, right int) {
	e := f.tb.Inlines[t]
	left, right = f.positions(s, t)
	open, closing := f.rawOf(e.Open), f.rawOf(e.Close)
	switch {
	case s.Start == e.Text.Start:
		f.remove(e.Open)
		f.insert(right, open, rankOpen)
	case s.Stop == e.Text.Stop:
		f.remove(e.Close)
		f.insert(left, closing, rankClose)
	default:
		f.insert(left, closing, rankClose)
		f.insert(right, open, rankOpen)
	}
	return left, right
}

// strike は取り消し線を付ける・外す（IMP-124 の 5）。**選択の文字（空白を除く）がすべて取り消し線なら
// 外し、そうでなければ付ける。判定は選択の全体で行い、差し込みは区間ごとに行う**（4.76.0。UT-120 の 11）。
func (f *formatter) strike() error {
	tb, sel := f.tb, f.sel
	f.add = false
	for i := sel.Start; i < sel.Stop; i++ {
		if !unicode.IsSpace(tb.Visible[i]) && !struckAt(tb, i) {
			f.add = true
			break
		}
	}

	if f.add {
		// 付ける: 取り消し線でない区間の両端に <del> と </del>。既に取り消し線の区間は何もしない。
		for _, s := range f.segments() {
			if !struckAt(tb, s.Start) {
				f.wrap(s, []byte("<del>"), []byte("</del>"))
			}
		}
		return nil
	}

	for k, e := range tb.Inlines {
		if !strikeKinds[e.Kind] || !overlaps(e.Text, sel) {
			continue
		}
		if e.Kind != "del" {
			// 手書き（~~ / ~ / <s> / <strike>）: 全体の記号・タグを取り除く。記法を書き換えない。
			f.remove(e.Open)
			f.remove(e.Close)
			f.removed[k] = true
			f.widened = f.widened || !contains(sel, e.Text)
			continue
		}
		// MarkView の形: 中身の全体ならタグを取り除き、一部なら分ける。
		in := renderer.Span{Start: max(sel.Start, e.Text.Start), Stop: min(sel.Stop, e.Text.Stop)}
		if in == e.Text {
			f.remove(e.Open)
			f.remove(e.Close)
			continue
		}
		if f.crosses(k, in) {
			return ErrCannotFormat
		}
		f.split(k, in)
	}
	return nil
}

// recolor は文字色を付ける・替える・外す（IMP-124 の 6・7）。color が空なら外す（default）。
// 最も内側の色の span が同じ区間の並びごとに扱う。
func (f *formatter) recolor(color string) error {
	segs := f.segments()
	for i := 0; i < len(segs); {
		t := colorIndexAt(f.tb, segs[i].Start)
		j := i + 1
		for j < len(segs) && colorIndexAt(f.tb, segs[j].Start) == t {
			j++
		}
		if err := f.recolorRun(t, segs[i:j], color); err != nil {
			return err
		}
		i = j
	}
	return nil
}

// recolorRun は、最も内側の色の span が t（無ければ -1）である区間の並びの色を color にする。
func (f *formatter) recolorRun(t int, segs []renderer.Span, color string) error {
	open, closing := colorOpen(color), []byte("</span>")
	if t < 0 {
		// 色の無い文字: 付ける（default なら何もしない）。
		if color != "" {
			for _, s := range segs {
				f.wrap(s, open, closing)
			}
		}
		return nil
	}

	e := f.tb.Inlines[t]
	if e.Color == color {
		return nil // 同じ色（FR-143 の「内容が変わらない書き込みはしない」）
	}
	run := renderer.Span{Start: segs[0].Start, Stop: segs[len(segs)-1].Stop}
	switch {
	case run == e.Text && color == "":
		// 中身の全体の色を外す: タグを取り除く。
		f.remove(e.Open)
		f.remove(e.Close)
	case run == e.Text:
		// 中身の全体の色を替える: style の値を決めた形で書き直す（手で書いた揺れも。UT-120 のケース 14）。
		tag := f.rawOf(e.Open)
		if start, end, ok := styleAttr(tag); ok {
			tag = append(append(append([]byte{}, tag[:start]...), `style="color: `+colorValue(color)+`"`...), tag[end:]...)
		} else {
			tag = open
		}
		f.remove(e.Open)
		f.insert(e.Open.Start, tag, rankOpen)
	case f.crosses(t, run):
		// 分けられない: 色を外すときは書き込まない。替えるときだけ、区間を新しい span で囲む入れ子で
		// 替える（内側の色が勝つ。UT-120 のケース 15、UT-121 のケース 10）。
		if color == "" {
			return ErrCannotFormat
		}
		for _, s := range segs {
			f.wrap(s, open, closing)
		}
	default:
		left, right := f.split(t, run)
		if color != "" {
			f.insert(left, open, rankOpen)
			f.insert(right, closing, rankClose)
		}
	}
	return nil
}
