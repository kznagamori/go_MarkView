package renderer

// このファイルは文字色（FR-147）の span の style の扱いを持つ（IMP-116, IMP-123, IMP-124）。
//
// 色を付けてよいのは、`span` の `style` が「一覧の色（TextColors の Light の値）の color の宣言
// 1 つだけ」のときに限る。**判定はこの 1 つの関数（ColorOfStyle）で行い**、サニタイズの前後の走査
// （IMP-116）と、見える文字の対応付け（IMP-123）と、書き込み（IMP-124）が同じ関数を使う。判定が
// 2 か所に分かれると、画面で色が付くものと Go が色として読むものが食い違い、`Default` を選んでも
// 色が外れない文書ができる。

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// ColorOfStyle は style 属性の値が「一覧の色の color の宣言 1 つだけ」なら、その色の名前を返す
// （IMP-116, IMP-123, IMP-124）。
//
//   - 値は TextColors の Light の値に限る（Dark の値・一覧に無い値は色ではない）
//   - 大文字と小文字、コロンの前後と全体の前後の空白の揺れは許す（bluemonday は `color:#D1242F`
//     を `color: #d1242f` に整える。MD-072）
//   - **末尾の `;` は 1 つだけ許す。`!important` は含めない**（4.77.0）
//   - **`color` 以外の宣言を含むもの・`color` が 2 つあるものは色ではない**（MD-072。4.77.0。
//     利用者の判断）
func ColorOfStyle(style string) (string, bool) {
	s := strings.TrimSpace(style)
	s = strings.TrimSpace(strings.TrimSuffix(s, ";"))

	prop, val, found := strings.Cut(s, ":")
	if !found || !strings.EqualFold(strings.TrimSpace(prop), "color") {
		return "", false
	}

	val = strings.TrimSpace(val)
	for _, c := range TextColors {
		if strings.EqualFold(val, c.Light) {
			return c.Name, true
		}
	}

	return "", false
}

// isTextColorValue は、bluemonday が宣言ごとに渡す color の値（小文字にされたもの）が一覧の色かを返す
// （サニタイズのポリシーが使う。IMP-116）。**判定は ColorOfStyle に任せる**（色の判定を 2 か所に
// 書かない。片方だけ変えると、画面で色が付くものと Go が色と読むものが食い違う）。
func isTextColorValue(v string) bool {
	_, ok := ColorOfStyle("color: " + v)

	return ok
}

// dropUncoloredStyles は、サニタイズの前に、`span` の `style` のうち「一覧の色の color の宣言
// 1 つだけ」でないものを取り除く（IMP-116）。
//
// **bluemonday は宣言ごとに絞り、許可した宣言だけを残す**（v1.0.27 の sanitizeStyles。`!important`
// も黙って外す）。そのままでは `color: #d1242f; font-weight: bold` が赤で描かれ、Go が色と読むもの
// （ColorOfStyle）と食い違う。だから、bluemonday に渡す前に style ごと落とす。
//
// `span` 以外の要素の style は、ポリシーが通さないため、ここでは見ない。直さないトークンは Raw() の
// バイト列をそのまま書く（出力を変えない）。生 HTML が無い文書では呼ばない（style は生 HTML から
// しか来ない）。
func dropUncoloredStyles(converted []byte) []byte {
	var out bytes.Buffer
	out.Grow(len(converted))

	z := html.NewTokenizer(bytes.NewReader(converted))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			out.Write(z.Raw())
			continue
		}

		raw := append([]byte(nil), z.Raw()...)
		tok := z.Token()

		if tok.Data != "span" || !dropStyleUnlessColor(&tok) {
			out.Write(raw)
			continue
		}
		out.WriteString(tok.String())
	}

	return out.Bytes()
}

// dropStyleUnlessColor は、style が色の宣言 1 つだけでなければ style 属性を取り除き、取り除いたかを返す。
func dropStyleUnlessColor(tok *html.Token) bool {
	for i, a := range tok.Attr {
		if a.Key != "style" {
			continue
		}
		if _, ok := ColorOfStyle(a.Val); ok {
			return false
		}
		tok.Attr = append(tok.Attr[:i], tok.Attr[i+1:]...)
		return true
	}

	return false
}

// colorToClass は、サニタイズの後の `span` の style を外し、色なら class に `md-color-<名前>` を足す
// （IMP-116。sanitizeAfter が 1 回の走査の中で呼ぶ）。書き換えたかを返す。
//
// **色は CSS が与える**——Light は一覧の Light の値、Dark は一覧の Dark の値で描く（FR-147, DSP-010,
// DSP-121）。**インラインの style のままでは、Dark で別の色に描けない**（CSS より style が勝つ）。
// `md-color-*` は class の許可リストに入れない（生 HTML で直接書いた class は落ちる）。色が付くのは
// 許可した style を書いた span だけになり、画面で色が付くものと Go が色として読むものが一致する。
func colorToClass(tok *html.Token) bool {
	if tok.Data != "span" {
		return false
	}

	style := -1
	for i, a := range tok.Attr {
		if a.Key == "style" {
			style = i
			break
		}
	}
	if style < 0 {
		return false
	}

	name, ok := ColorOfStyle(tok.Attr[style].Val)
	tok.Attr = append(tok.Attr[:style], tok.Attr[style+1:]...)
	if !ok {
		return true
	}

	cls := "md-color-" + name
	for i, a := range tok.Attr {
		if a.Key == "class" {
			tok.Attr[i].Val = strings.TrimSpace(a.Val + " " + cls)
			return true
		}
	}
	tok.Attr = append(tok.Attr, html.Attribute{Key: "class", Val: cls})

	return true
}
