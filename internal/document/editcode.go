package document

import (
	"bytes"
	"reflect"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// editcode.go はコードブロックと図の書き換え（IMP-122。4.75.0）を持つ。**位置は renderer が求め**
// （Locate の FenceLocation と SplitPlantUML。IMP-121, IMP-126）、ここは生バイト列への対応・Patch の
// 組み立て・確かめだけを行う（IMP-106 と同じ分け方）。図のファイル（PlantUML・Mermaid）の書き換えは
// editdiagram.go にある（400 行の目安。IMP-011）。

// CodeSource は ref のコードブロック・図の、書き換える範囲のソースを返す（編集欄の初期値。FR-145）。
// 前置き（引用の `> ` やリストの字下げ）は除き、改行は LF にそろえる。前置きが行ごとに違えば ErrLocked。
//
//   - Markdown: 中身の各行を LF でつないだもの。**最後の行の改行を含めない**（書き戻すときに各行の後ろに
//     改行を付けるため。含めると、何も変えずに確定しただけで空の行が増える。UT-118 のケース 1・3）
//   - PlantUML ファイル: 図の Span の中身（@start… の行から @end… の行の改行の手前まで）
//   - Mermaid ファイル: ファイル全体（先頭の BOM を除き、末尾の改行を含む）
func CodeSource(r *renderer.Renderer, raw []byte, kind mdfile.Kind, ref Ref) (string, error) {
	if err := checkBlockRef(ref); err != nil {
		return "", err
	}

	switch kind {
	case mdfile.KindMarkdown:
		t, err := findFence(r, raw, ref)
		if err != nil {
			return "", err
		}
		return fenceContent(t.text, t.fence), nil
	case mdfile.KindPlantUML:
		text, _, d, err := findDiagram(raw, ref)
		if err != nil {
			return "", err
		}
		return string(text[d.Span.Start:d.Span.Stop]), nil
	case mdfile.KindMermaid:
		text, _, err := mermaidFile(raw, ref)
		if err != nil {
			return "", err
		}
		return string(text), nil
	default:
		// 画像ファイルは書き換える対象を持たない（FR-027）。
		return "", ErrNotEditable
	}
}

// PlanCode は ref のコードブロック・図の中身を text に置き換える Patch を返す（FR-145）。
//
// 新しい中身（LF にそろえたもの）が CodeSource の値と同じなら changed は偽。**Patch を当てた結果を
// 解き直して確かめ**、構造が変わるなら ErrRejected とする（書き込まない。IMP-122 の 9）。
func PlanCode(r *renderer.Renderer, raw []byte, kind mdfile.Kind, ref Ref, text string) (p Patch, changed bool, err error) {
	if err := checkBlockRef(ref); err != nil {
		return Patch{}, false, err
	}

	switch kind {
	case mdfile.KindMarkdown:
		return planFence(r, raw, ref, text)
	case mdfile.KindPlantUML:
		return planDiagram(raw, ref, text)
	case mdfile.KindMermaid:
		return planMermaidFile(raw, ref, text)
	default:
		return Patch{}, false, ErrNotEditable
	}
}

// checkBlockRef は、ref がコードブロックか図の目印であることを確かめる。ほかの種類（task / cell /
// text）は ErrBadRef。種類は合っていても文書に無い（図のファイルの code:0 など）ものは、呼び出し先が
// ErrRefNotFound にする（UT-119 のケース 11・21）。
func checkBlockRef(ref Ref) error {
	switch ref.Kind {
	case RefCode, RefMermaid, RefPlantUML:
		return nil
	}
	return ErrBadRef
}

// normalizeNewlines は入力の改行（`\r\n` / `\r` / `\n`）を LF にそろえる。
func normalizeInput(s string) string {
	return strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(s)
}

// newlineAt は raw の pos から始まる改行コードを返す。改行でなければ空文字。
func newlineAt(raw []byte, pos int) string {
	switch {
	case pos >= len(raw):
		return ""
	case raw[pos] == '\n':
		return "\n"
	case raw[pos] == '\r' && pos+1 < len(raw) && raw[pos+1] == '\n':
		return "\r\n"
	case raw[pos] == '\r':
		return "\r"
	}
	return ""
}

// lineEnding は raw の from から後ろで最初に現れる改行コードを返す。無ければ空文字。
func lineEnding(raw []byte, from int) string {
	if i := bytes.IndexAny(raw[from:], "\r\n"); i >= 0 {
		return newlineAt(raw, from+i)
	}
	return ""
}

// --- Markdown の中のブロック ---

// fenceTarget は書き換えるブロックと、書き換える前の文書の解析結果。
type fenceTarget struct {
	text  []byte                 // 正規化後のテキスト
	m     rawMap                 // 正規化後の位置から生バイト列の位置への対応
	locs  renderer.Locations     // 書き換える前の位置の一覧（確かめに使う）
	fence renderer.FenceLocation // 書き換えるブロック
}

// findFence は ref のブロックを求める（IMP-122 の 1・2）。不正なバイト列は ErrNotEditable、番号が
// 範囲を超えれば ErrRefNotFound、前置きが行ごとに違うブロックは ErrLocked。
func findFence(r *renderer.Renderer, raw []byte, ref Ref) (fenceTarget, error) {
	text, m, locs, err := locate(r, raw)
	if err != nil {
		return fenceTarget{}, err
	}
	list := fencesOf(locs, ref.Kind)
	if ref.Index < 0 || ref.Index >= len(list) {
		return fenceTarget{}, ErrRefNotFound
	}
	f := list[ref.Index]
	if f.Locked {
		return fenceTarget{}, ErrLocked
	}
	return fenceTarget{text: text, m: m, locs: locs, fence: f}, nil
}

// fencesOf は目印の種類のブロックの一覧を返す（図は種類ごとに数える。IMP-120）。
func fencesOf(locs renderer.Locations, kind RefKind) []renderer.FenceLocation {
	switch kind {
	case RefMermaid:
		return locs.Mermaids
	case RefPlantUML:
		return locs.PlantUMLs
	default:
		return locs.Codes
	}
}

// fenceContent はブロックの中身の各行を LF でつないだものを返す（最後の行の改行を含めない）。
// 中身の行が無いブロックと、空の行 1 つだけのブロックは、どちらも空文字になる。
func fenceContent(text []byte, f renderer.FenceLocation) string {
	lines := make([]string, len(f.Lines))
	for i, l := range f.Lines {
		lines[i] = string(text[l.Start:l.Stop])
	}
	return strings.Join(lines, "\n")
}

// planFence は Markdown の中のブロックの Patch を組み立てる（IMP-122 の 3〜9）。
func planFence(r *renderer.Renderer, raw []byte, ref Ref, input string) (Patch, bool, error) {
	t, err := findFence(r, raw, ref)
	if err != nil {
		return Patch{}, false, err
	}
	f, m := t.fence, t.m

	content := normalizeInput(input)
	if content == fenceContent(t.text, f) {
		return Patch{}, false, nil
	}
	lines := strings.Split(content, "\n")

	// 改行コードは開きの囲いの行の終わりのもの。開きの囲いの行に改行が無い（文書がそこで終わる）なら、
	// 中身の前に LF を 1 つ足し、最後の行には改行を付けない（4.77.0。UT-118 のケース 25）。
	bodyStart, bodyStop := m.toRaw(f.Body.Start), m.toRaw(f.Body.Stop)
	eol := lineEnding(raw[:bodyStart], m.toRaw(f.Open.Stop))
	openHasEOL := eol != ""
	if !openHasEOL {
		eol = "\n"
	}
	// 範囲の最後のバイトが改行でない（閉じが無く、最後の中身の行に改行が無い）なら、最後の行にも
	// 付けない。範囲が空なら付ける。**どちらでも、最後の行が空なら付ける**（UT-118 のケース 10・27）。
	lastEOL := openHasEOL && (bodyStop == bodyStart || raw[bodyStop-1] == '\n' || raw[bodyStop-1] == '\r')
	if lines[len(lines)-1] == "" {
		lastEOL = true
	}

	// 各行の前にブロックの前置きを、後ろにブロックの改行コードを付ける。空の行には、前置きから末尾の
	// 半角空白とタブを除いたものを付ける（引用の中の空の行は `>` だけ。IMP-122 の 4）。
	var body strings.Builder
	if !openHasEOL {
		body.WriteString("\n")
	}
	blank := strings.TrimRight(f.Prefix, " \t")
	for i, l := range lines {
		if l == "" {
			body.WriteString(blank)
		} else {
			body.WriteString(f.Prefix)
			body.WriteString(l)
		}
		if i < len(lines)-1 || lastEOL {
			body.WriteString(eol)
		}
	}

	var p Patch
	fch := t.text[f.Open.Start]
	if run := longestFenceRun(lines, fch); run < f.Open.Stop-f.Open.Start {
		// 延ばさない: 中身の行の範囲（Body）だけを置き換える。
		p = Patch{Offset: bodyStart, Old: bytes.Clone(raw[bodyStart:bodyStop]), New: []byte(body.String())}
	} else {
		// 囲いの延長（IMP-122 の 5〜7）: 開きと閉じの囲いを、最も長い並びより 1 つ長くする。言語指定と
		// 改行はそのまま残し、閉じの囲いの行の前置きは元のバイト列のまま書き戻す（4.76.0。UT-118 の 16）。
		fence := strings.Repeat(string(fch), run+1)
		openStart, openStop := m.toRaw(f.Open.Start), m.toRaw(f.Open.Stop)

		var b bytes.Buffer
		b.WriteString(fence)
		b.Write(raw[openStop:bodyStart])
		b.WriteString(body.String())
		end := bodyStop
		if f.Closed {
			closeStart, closeStop := m.toRaw(f.Close.Start), m.toRaw(f.Close.Stop)
			b.Write(raw[bodyStop:closeStart])
			b.WriteString(fence)
			end = closeStop
		}
		p = Patch{Offset: openStart, Old: bytes.Clone(raw[openStart:end]), New: b.Bytes()}
	}

	if !keepsFences(r, raw, t, ref, p, content) {
		return Patch{}, false, ErrRejected
	}
	return p, true, nil
}

// longestFenceRun は、入力の行のうち囲いを閉じうる行の、囲いの文字 ch の並びの最も長いものを返す
// （IMP-122 の 5）。数えるのは、先頭の空白を除いた後ろが ch で始まる行のうち、先頭の空白が半角空白
// 3 つまでの行と、先頭の空白にタブを含む行（引用やリストの中では前置きの後ろのタブが 4 桁にならず、
// 囲いを閉じうる。4.78.0）。**行の途中の並びは囲いを閉じない**（CommonMark）ため数えない。
func longestFenceRun(lines []string, ch byte) int {
	longest := 0
	for _, l := range lines {
		rest := strings.TrimLeft(l, " \t")
		lead := l[:len(l)-len(rest)]
		if len(lead) > 3 && !strings.Contains(lead, "\t") {
			continue
		}
		n := len(rest) - len(strings.TrimLeft(rest, string(ch)))
		longest = max(longest, n)
	}
	return longest
}

// keepsFences は Patch を当てた結果を解き直し、構造が変わらないことを確かめる（IMP-122 の 9）。
//
//   - ブロックの並び（Blocks）と、タスク・表の形・コードブロック・図・修飾できるブロックの数が変わらない
//   - 書き換えたブロックの中身（前置きを除き LF にそろえたもの）が入力と一致し、言語指定が変わらない
//   - 他のコードブロックと図の中身が変わらない（位置ではなく中身を比べる。IMP-106 のセルと同じ）
func keepsFences(r *renderer.Renderer, raw []byte, t fenceTarget, ref Ref, p Patch, content string) bool {
	text, got, ok := relocate(r, raw, p)
	if !ok || !sameStructure(t.locs, got) {
		return false
	}

	for _, kind := range []RefKind{RefCode, RefMermaid, RefPlantUML} {
		before, after := fencesOf(t.locs, kind), fencesOf(got, kind)
		for i := range before {
			want := fenceContent(t.text, before[i])
			if kind == ref.Kind && i == ref.Index {
				if after[i].Info != before[i].Info {
					return false
				}
				want = content
			}
			if fenceContent(text, after[i]) != want {
				return false
			}
		}
	}
	return true
}

// relocate は raw に p を当てた結果を正規化して解き直す。当てられない・不正なバイト列になる・
// 解けないなら ok は偽。
func relocate(r *renderer.Renderer, raw []byte, p Patch) (text []byte, locs renderer.Locations, ok bool) {
	after, err := p.Apply(raw)
	if err != nil {
		return nil, renderer.Locations{}, false
	}
	text, _, err = mapRaw(after)
	if err != nil {
		return nil, renderer.Locations{}, false
	}
	locs, err = r.Locate(text)
	if err != nil {
		return nil, renderer.Locations{}, false
	}
	return text, locs, true
}

// sameStructure は、ブロックの並び（Blocks）と、タスク・表の形（行の数と各行のセルの数と、補われた
// セルの位置）・コードブロック・図・修飾できるブロックの数が同じかを返す（IMP-122 の 9、IMP-124 の 9）。
func sameStructure(a, b renderer.Locations) bool {
	if !reflect.DeepEqual(a.Blocks, b.Blocks) || len(a.Tasks) != len(b.Tasks) || a.Texts != b.Texts ||
		len(a.Codes) != len(b.Codes) || len(a.Mermaids) != len(b.Mermaids) || len(a.PlantUMLs) != len(b.PlantUMLs) ||
		len(a.Tables) != len(b.Tables) {
		return false
	}
	for i := range a.Tables {
		ac, bc := a.Tables[i].Cells, b.Tables[i].Cells
		if len(ac) != len(bc) {
			return false
		}
		for row := range ac {
			if len(ac[row]) != len(bc[row]) {
				return false
			}
			for col := range ac[row] {
				if (ac[row][col] == nil) != (bc[row][col] == nil) {
					return false
				}
			}
		}
	}
	return true
}
