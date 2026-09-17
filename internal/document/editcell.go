package document

import (
	"bytes"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// editcell.go はセルの書き換え（IMP-106 の「セル」）を持つ。位置の対応と Patch は edit.go にある。

// cellTrimSet は前後から除く空白（goldmark の util.IsSpace のうち改行を除いたもの）。
//
// goldmark の表の解析はセルの前後の ASCII の空白だけを除く。Unicode の空白（全角空白
// など）まで除くと、書かれた内容と比べたときに「変わっていない」を誤る。
const cellTrimSet = " \t\v\f"

// PlanCell は raw の中で ref のセルを text に置き換える Patch を返す（FR-142）。
//
// 置き換えるのはセルの内容（前後の空白を除いた範囲。renderer.CellSpan の Content）で、
// 区切りの `|` とその内側の前後の空白は残す。整えた結果が元の内容と同じなら changed は偽。
//
// **組み立てた Patch を当てた結果をもう一度解き直し、表の形が変わらないことを確かめる。**
// 確かめられなければ Patch を返さず ErrNotEditable とする（書き込まない）。エスケープの
// 規則はパーサの実装に依存し、手で書いた規則だけでは形が変わらないことを保証できない
// （IMP-106。4.43.0 で `\` の個数を数える規則が goldmark と食い違っていた）。
func PlanCell(r *renderer.Renderer, raw []byte, ref Ref, text string) (p Patch, changed bool, err error) {
	src, m, locs, err := locate(r, raw)
	if err != nil {
		return Patch{}, false, err
	}
	cell, err := findCell(locs, ref)
	if err != nil {
		return Patch{}, false, err
	}

	content := formatCell(text)

	start, stop := m.toRaw(cell.Content.Start), m.toRaw(cell.Content.Stop)
	if cell.Content.Start == cell.Content.Stop {
		// **空のセルは、区切りの間の空白の 1 文字目の直後へ入れる。** 空白が無ければ前の
		// 区切りの直後へ入れる。`|  |` は `| x |` になり、空白の数は変わらない（UT-110 ケース 10・11）。
		at := cell.Between.Start
		if cell.Between.Stop > cell.Between.Start {
			at++
		}
		start = m.toRaw(at)
		stop = start
	}
	old := raw[start:stop]

	// **新しい内容が `\` で終わり、書く位置の直後が区切りの `|`（間に空白が無い）なら、
	// 末尾に半角空白を 1 つ足す。** そのままでは `\` が区切りの `|` をエスケープし、
	// 隣のセルと結合して表の形が変わる（UT-110 ケース 20）。空白があれば何も足さない（ケース 6）。
	written := content
	if strings.HasSuffix(content, `\`) && stop < len(raw) && raw[stop] == '|' {
		written += " "
	}

	if written == string(old) {
		return Patch{}, false, nil
	}

	p = Patch{Offset: start, Old: bytes.Clone(old), New: []byte(written)}
	if !keepsTableShape(r, src, raw, locs, ref, p, content) {
		return Patch{}, false, ErrNotEditable
	}
	return p, true, nil
}

// formatCell は入力をセルの内容へ整える（IMP-106 の「セル」の 1〜3）。
//
//  1. 改行（`\r\n` / `\r` / `\n`）を半角空白 1 つへ置き換える（FR-142）
//  2. 前後の空白を除く
//  3. **直前の 1 文字が `\` でない `|`** を `\|` にする。**`\` の個数は数えない**——
//     goldmark v1.8.5 の表の解析は、`|` の直前の 1 文字だけを見て区切りかどうかを決める
//     （UT-110 ケース 5）
func formatCell(text string) string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Trim(text, cellTrimSet)

	var b strings.Builder
	b.Grow(len(text) + 4)
	for i := 0; i < len(text); i++ {
		if text[i] == '|' && (i == 0 || text[i-1] != '\\') {
			b.WriteByte('\\')
		}
		b.WriteByte(text[i])
	}
	return b.String()
}

// keepsTableShape は Patch を当てた結果を解き直し、表の形が変わらないことを確かめる
// （IMP-106 の「セル」の最後の項）。
//
//   - 文書の中の表の数とタスクの数が変わらない（見出し行の書き換えで表として解析され
//     なくなると、後ろの表とセルの番号がずれる）
//   - その表の行数と、各行のセルの数と、補われたセルの位置が変わらない
//   - 書き換えたセル以外のセルについて、Content の区間が指すバイト列が変わらない
//     （区間の位置そのものは、書き換えたセルより後ろでずれる。比べるのは中身である）
//   - 書き換えたセルの Content の区間が指すバイト列が、整えた文字列（足した空白を除く）と一致する
func keepsTableShape(r *renderer.Renderer, before, raw []byte, locs renderer.Locations, ref Ref, p Patch, content string) bool {
	after, err := p.Apply(raw)
	if err != nil {
		return false
	}
	afterText, _, err := mapRaw(after)
	if err != nil {
		return false
	}
	got, err := r.Locate(afterText)
	if err != nil {
		return false
	}

	if len(got.Tables) != len(locs.Tables) || len(got.Tasks) != len(locs.Tasks) {
		return false
	}

	bc, gc := locs.Tables[ref.Index].Cells, got.Tables[ref.Index].Cells
	if len(gc) != len(bc) {
		return false
	}
	for row := range bc {
		if len(gc[row]) != len(bc[row]) {
			return false
		}
		for col := range bc[row] {
			b, g := bc[row][col], gc[row][col]
			if (b == nil) != (g == nil) {
				return false
			}
			if b == nil {
				continue
			}
			gotText := string(afterText[g.Content.Start:g.Content.Stop])
			if row == ref.Row && col == ref.Col {
				if gotText != content {
					return false
				}
				continue
			}
			if gotText != string(before[b.Content.Start:b.Content.Stop]) {
				return false
			}
		}
	}
	return true
}
