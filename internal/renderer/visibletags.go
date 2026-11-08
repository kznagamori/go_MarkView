package renderer

// このファイルは、見える文字の走査（visiblebuild.go）のうち、生 HTML のタグの対と、ソースの上の
// 記号の位置を求める小さな関数を持つ（IMP-123）。visiblebuild.go が 400 行の目安（IMP-011）を超えた
// ため分けた。

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"golang.org/x/net/html"
)

// rawTag は生 HTML のタグ 1 つ（対を組むために控える）。
type rawTag struct {
	name    string
	closing bool
	style   string
	at      int  // 見える文字の上の位置
	span    Span // ソースの上の区間（前処理の後の本文の上）
}

// visitRawHTML は生 HTML のタグを 0 文字として扱い、対を組むために控える（IMP-123）。
// `<img>` は 1 文字の部品とする。
func (b *textBuilder) visitRawHTML(v *ast.RawHTML) ext {
	var raw []byte
	span := Span{Start: -1}
	for i := 0; i < v.Segments.Len(); i++ {
		s := v.Segments.At(i)
		raw = append(raw, s.Value(b.src)...)
		if span.Start < 0 {
			span.Start = s.Start
		}
		span.Stop = s.Stop
	}
	if span.Start < 0 {
		return ext{}
	}
	if span.Stop > b.cur {
		b.cur = span.Stop
	}
	e := ext{start: span.Start, stop: span.Stop, ok: true}

	z := html.NewTokenizer(bytes.NewReader(raw))
	tt := z.Next()
	if tt != html.StartTagToken && tt != html.EndTagToken && tt != html.SelfClosingTagToken {
		return e
	}
	tok := z.Token()
	if tok.Data == "img" && tt != html.EndTagToken {
		start := len(b.tb.Visible)
		b.add(objectChar, span)
		b.atom(start)
		return e
	}
	tag := rawTag{name: tok.Data, closing: tt == html.EndTagToken, at: len(b.tb.Visible), span: span}
	for _, a := range tok.Attr {
		if a.Key == "style" {
			tag.style = a.Val
		}
	}
	if tt != html.SelfClosingTagToken {
		b.tags = append(b.tags, tag)
	}

	return e
}

// pairTags は、同じブロックの中で開きと閉じがそろったタグだけを対にしてインラインの要素にする
// （IMP-123）。そろわないタグは対にしない（0 文字のまま）。
func (b *textBuilder) pairTags() {
	var stack []rawTag
	for _, t := range b.tags {
		if !t.closing {
			stack = append(stack, t)
			continue
		}
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].name != t.name {
				continue
			}
			open := stack[i]
			stack = append(stack[:i], stack[i+1:]...)
			kind, color := tagKind(open)
			b.tb.Inlines = append(b.tb.Inlines, InlineSpan{
				Kind:  kind,
				Text:  Span{Start: open.at, Stop: t.at},
				Open:  Span{Start: open.span.Start + b.shift, Stop: open.span.Stop + b.shift},
				Close: Span{Start: t.span.Start + b.shift, Stop: t.span.Stop + b.shift},
				Color: color,
			})
			break
		}
	}
}

// tagKind は生 HTML のタグの対の種類を返す。色は IMP-116 が通すものと同じ判定（ColorOfStyle）で読む。
func tagKind(t rawTag) (kind, color string) {
	switch strings.ToLower(t.name) {
	case "del":
		return "del", ""
	case "s":
		return "s", ""
	case "strike":
		return "strike-tag", ""
	case "span":
		if name, ok := ColorOfStyle(t.style); ok {
			return "color", name
		}
	}
	return "html", ""
}

// decodeTextRaw は綴りをそのまま 1 符号位置ずつ数える（解かない）。
func decodeTextRaw(b []byte, base int) ([]rune, []Span) {
	runes := make([]rune, 0, len(b))
	spans := make([]Span, 0, len(b))
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		runes = append(runes, r)
		spans = append(spans, Span{Start: base + i, Stop: base + i + size})
		i += size
	}
	return runes, spans
}
