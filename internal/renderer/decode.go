package renderer

import (
	"strconv"
	"unicode/utf8"

	"github.com/yuin/goldmark/util"
)

// decodeText は文字のノードの綴りを、見える文字に解く（IMP-117 の 1, IMP-123）。1 文字（符号位置）
// ごとに、元の綴りの上の区間（base を足した位置）を返す。
//
// **goldmark の文字のノード（ast.Text）は、エスケープ（`\*`）と文字参照（`&amp;`）を書かれたとおりの
// 綴りで持ち、描画のときに解く**（goldmark v1.8.5 で実測。1 つの文字のノードの区間の中に生のまま
// 入っている）。見出しの文字（アウトライン。IMP-117）も、見える文字（修飾。IMP-123）も、画面に出る
// 文字と同じにするため、ここで解く。
//
// **エスケープと文字参照は 1 回の走査で解く**（描画の Writer と同じ）。2 段に分けると、`\&amp;`（
// エスケープした & と、文字の amp;）を取り違える。
//
//   - バックスラッシュと ASCII の約物の 2 バイトは、その約物 1 文字（区間は 2 バイト）
//   - 文字参照（`&名前;` / `&#10 進;` / `&#x16 進;`）は、解いた文字（区間は参照の全体。解いた文字が
//     2 つ以上でも、どれも同じ区間）。**解き方は描画（goldmark の Writer）と同じ規則**にする
//     （decodeNumeric。4.86.0）
//   - NUL は U+FFFD（描画の Writer が置き換える）
//   - それ以外は UTF-8 の 1 符号位置
//
// インラインコードの中（IsRaw の文字）には使わない。そこは書かれたとおりに表示される。
func decodeText(b []byte, base int) ([]rune, []Span) {
	runes := make([]rune, 0, len(b))
	spans := make([]Span, 0, len(b))

	for i := 0; i < len(b); {
		switch {
		case b[i] == '\\' && i+1 < len(b) && util.IsPunct(b[i+1]):
			runes = append(runes, rune(b[i+1]))
			spans = append(spans, Span{Start: base + i, Stop: base + i + 2})
			i += 2
			continue

		case b[i] == '&':
			if decoded, n := decodeReference(b[i:]); n > 0 {
				for _, r := range decoded {
					runes = append(runes, r)
					spans = append(spans, Span{Start: base + i, Stop: base + i + n})
				}
				i += n
				continue
			}
		}

		r, size := utf8.DecodeRune(b[i:])
		if r == 0 {
			r = utf8.RuneError
		}
		runes = append(runes, r)
		spans = append(spans, Span{Start: base + i, Stop: base + i + size})
		i += size
	}

	return runes, spans
}

// decodeReference は b の先頭の文字参照を解き、解いた文字と参照の長さを返す。参照でなければ長さ 0。
// 名前の参照は描画と同じ一覧（util.LookUpHTML5EntityByName）で引く。
func decodeReference(b []byte) (string, int) {
	if len(b) < 3 || b[0] != '&' {
		return "", 0
	}
	if b[1] == '#' {
		return decodeNumeric(b)
	}

	i := 1
	for i < len(b) && util.IsAlphaNumeric(b[i]) {
		i++
	}
	if i == 1 || i >= len(b) || b[i] != ';' {
		return "", 0
	}
	entity, ok := util.LookUpHTML5EntityByName(string(b[1:i]))
	if !ok {
		return "", 0
	}

	return string(entity.Characters), i + 1
}

// decodeNumeric は b の先頭の数値の文字参照（`&#…;` / `&#x…;`）を解く。**規則は描画（goldmark
// v1.8.5 の html.defaultWriter.Write）と同じにする**: 10 進は 1〜7 桁を基数 10 で、16 進は 1〜6 桁で
// 読み、0 と符号位置にならない値は U+FFFD。桁が多すぎる参照は解かない（書かれたとおりに出る）。
//
// **goldmark の util.ResolveNumericReferences は使わない。** 10 進を基数 0 で読む（先頭の 0 で 8 進に
// なる。`&#065;` が `5`）うえ、16 進の桁数の上限が無く、描画と違う文字になる（独立したレビューの指摘。
// 見出しのスラッグ（MD-021）と見える文字（IMP-123）が画面と食い違っていた）。
//
// CR（`&#13;`）は LF にする。描画は CR をそのまま書くが、ブラウザが HTML を読むときに LF へそろえる
// （画面の文字は LF）。**CR の直後に LF が来る形（行末の `&#13;`、`&#13;&#10;`）は、ブラウザが 2 つを
// LF 1 つにまとめるため一致しない**（IMP-123。そのブロックの修飾は古い指示として断られる）。
func decodeNumeric(b []byte) (string, int) {
	i, base, maxDigits := 2, 10, 7
	if i < len(b) && (b[i] == 'x' || b[i] == 'X') {
		i, base, maxDigits = 3, 16, 6
	}
	start := i
	for i < len(b) && isDigitOf(b[i], base) {
		i++
	}
	if i == start || i-start > maxDigits || i >= len(b) || b[i] != ';' {
		return "", 0
	}

	v, err := strconv.ParseUint(string(b[start:i]), base, 32)
	if err != nil {
		return "", 0
	}
	r := util.ToValidRune(rune(v))
	if r == '\r' {
		r = '\n'
	}

	return string(r), i + 1
}

// isDigitOf は c が base（10 か 16）の数字かを返す。
func isDigitOf(c byte, base int) bool {
	if base == 16 {
		return util.IsHexDecimal(c)
	}

	return util.IsNumeric(c)
}

// decodedString は decodeText の文字だけを文字列で返す。
func decodedString(b []byte) string {
	runes, _ := decodeText(b, 0)

	return string(runes)
}
