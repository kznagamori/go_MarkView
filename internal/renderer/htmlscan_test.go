package renderer

import (
	"io"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// element は出力 HTML の要素 1 つ（開始タグ・自己終了タグ）を表す。
//
// 目印（IMP-120）や id（AR-053）の検査は、**どの要素のどの属性か**を特定して
// 行う。文字列の部分一致では、別の要素の同じ名前の属性に当たって誤判定する
// （UT-217 のケース 1〜4 のような「付かないこと」の検査が特にそうである）。
type element struct {
	Tag   string
	Attrs map[string]string // 属性名 → 値（実体参照は解いた後の値）
	Text  string            // 子孫のテキストをつないだもの（空要素では空）
}

// Attr は属性の値と、属性があるかを返す。
func (e element) Attr(name string) (string, bool) {
	v, ok := e.Attrs[name]
	return v, ok
}

// voidElements は終了タグを持たない要素（HTML の空要素）。
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "source": true,
	"track": true, "wbr": true,
}

// scanElements は HTML をトークナイザで読み、要素を文書の順に返す。
//
// 構文木を組み立てる html.Parse は表や段落の中身を並べ替えるため使わない。
// 見たいのはサニタイズ後の出力そのもの（フロントエンドが受け取る並び）である。
func scanElements(t *testing.T, s string) []element {
	t.Helper()

	z := html.NewTokenizer(strings.NewReader(s))

	var (
		els  []element
		open []int // 閉じていない要素の els の添字
	)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if err := z.Err(); err != io.EOF {
				t.Fatalf("出力 HTML を読めない: %v\n出力: %s", err, s)
			}
			return els

		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			el := element{Tag: tok.Data, Attrs: map[string]string{}}
			for _, a := range tok.Attr {
				if _, dup := el.Attrs[a.Key]; !dup {
					el.Attrs[a.Key] = a.Val
				}
			}
			els = append(els, el)
			if tt == html.StartTagToken && !voidElements[tok.Data] {
				open = append(open, len(els)-1)
			}

		case html.EndTagToken:
			tok := z.Token()
			// 対応する開始タグまで閉じる。対応が無ければ何もしない。
			for i := len(open) - 1; i >= 0; i-- {
				if els[open[i]].Tag == tok.Data {
					open = open[:i]
					break
				}
			}

		case html.TextToken:
			text := string(z.Text())
			for _, i := range open {
				els[i].Text += text
			}
		}
	}
}

// byTag は tag の要素だけを文書の順に返す。
func byTag(els []element, tags ...string) []element {
	var got []element
	for _, e := range els {
		for _, tag := range tags {
			if e.Tag == tag {
				got = append(got, e)
				break
			}
		}
	}
	return got
}

// withClass は class 属性に語 class を含む要素だけを返す。
func withClass(els []element, class string) []element {
	var got []element
	for _, e := range els {
		v, _ := e.Attr("class")
		for _, c := range strings.Fields(v) {
			if c == class {
				got = append(got, e)
				break
			}
		}
	}
	return got
}

// tableRows は表の要素を行に分けて返す（[表][行][セル]）。
//
// table の開始タグで表を、tr の開始タグで行を始め、th / td をその行に足す。
// 入れ子の表は扱わない（検証用の文書に入れない）。
func tableRows(els []element) [][][]element {
	var tables [][][]element
	for _, e := range els {
		switch e.Tag {
		case "table":
			tables = append(tables, nil)
		case "tr":
			if len(tables) > 0 {
				tables[len(tables)-1] = append(tables[len(tables)-1], nil)
			}
		case "th", "td":
			if len(tables) > 0 {
				rows := tables[len(tables)-1]
				if len(rows) > 0 {
					rows[len(rows)-1] = append(rows[len(rows)-1], e)
				}
			}
		}
	}
	return tables
}
