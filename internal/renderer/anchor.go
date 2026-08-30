package renderer

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// headingsKey は変換 1 回分の見出し一覧を parser.Context へ置くための鍵。
//
// Renderer は複数のゴルーチンから同時に使われる（IMP-024）。AST 変換器
// そのものは状態を持たせず、結果は Render ごとに作る Context 側へ置く。
// 変換器にスライスを持たせると、同時に走る変換どうしが干渉する。
var headingsKey = parser.NewContextKey()

// headingTransformer は見出しへ ID を付与し、一覧を集める（IMP-117, FR-040）。
//
// goldmark の parser.WithAutoHeadingID() は使わない。GitHub 互換のスラッグ
// 規則（MD-021）と生成結果が異なり、GitHub の README に書かれた `#見出し`
// 形式のリンクが MarkView で外れてしまうためである（IMP-111）。
type headingTransformer struct{}

// Transform は parser.ASTTransformer を満たす。
func (headingTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	s := newSlugger()
	// 空スライスへの変換は Render が行う（UT-203 ケース 5）。ここでは追加した分だけを持つ。
	var headings []Heading

	// ast.Walk はここでは失敗しない。walker がエラーを返さないため。
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		h, ok := n.(*ast.Heading)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}

		plain := plainText(h, source)
		id := s.Slug(plain)
		if id != "" {
			h.SetAttributeString("id", []byte(id))
		}
		headings = append(headings, Heading{Level: h.Level, Text: plain, ID: id})

		// 見出しの中に見出しは現れない。子を辿る必要はない。
		return ast.WalkSkipChildren, nil
	})

	pc.Set(headingsKey, headings)
}

// plainText はノードからインライン記法を除いたプレーンテキストを組み立てる
// （IMP-117 の 1）。得られた文字列は Heading.Text とスラッグの両方に使う。
//
// 強調・リンク・コードスパンは、いずれも内側に文字列ノードを持つため、
// それらを拾えば記法だけが落ちる。自動リンクは子を持たないため、
// 表示されるラベルを直接取る。
func plainText(n ast.Node, source []byte) string {
	var b strings.Builder

	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := c.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(source))
		case *ast.String:
			b.Write(v.Value)
		case *ast.AutoLink:
			b.Write(v.Label(source))
		}
		return ast.WalkContinue, nil
	})

	return b.String()
}

// slugger は 1 文書分の見出しアンカーを生成する（IMP-117）。
//
// 重複の連番は文書ごとに数え直す必要があるため、Renderer ではなく
// 変換 1 回ごとに作る。
type slugger struct {
	used map[string]int
}

func newSlugger() *slugger {
	return &slugger{used: make(map[string]int)}
}

// Slug は GitHub 互換のアンカー文字列を返す（MD-021）。
//
// 同じ slugger の中で重複した場合、2 つ目以降に -1, -2 … を付ける。
// 規則を適用して何も残らない見出し（空、記号のみ）にはアンカーを与えず、
// 空文字を返す。連番も付けない。id="" や id="-1" は利用者が辿れる
// アンカーにならず、UT-202 ケース 10 が許す「空文字」に寄せている。
func (s *slugger) Slug(text string) string {
	base := slugify(text)
	if base == "" {
		return ""
	}

	n := s.used[base]
	s.used[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, n)
}

// slugify は MD-021 の 2〜4 を適用する。
//
// 空白は連続していても 1 つの - にまとめる。ただし先頭と末尾では - を
// 書かない。GitHub の見出しリンクに前後のハイフンは現れないためである。
func slugify(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	pendingSpace := false
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsSpace(r):
			// MD-021 の 3。実際に - を書くのは、次に残る文字が来たとき。
			pendingSpace = true

		case isASCIIAlnum(r) || r == '-' || r == '_' || r > unicode.MaxASCII:
			if pendingSpace && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingSpace = false
			b.WriteRune(r)

		default:
			// MD-021 の 4。英数字・-・_・非 ASCII 以外の記号は取り除く。
		}
	}

	return b.String()
}

// isASCIIAlnum は ASCII の英数字かどうかを返す。
// 呼び出し時点で小文字化されているが、規則そのものは大文字も含めて書く。
func isASCIIAlnum(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}
