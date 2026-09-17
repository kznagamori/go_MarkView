package renderer

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"golang.org/x/net/html"
)

// allowedElements は MD-072 が許可する要素。**ハードコードし、設定で緩めない。**
//
// 任意の第三者から受け取った Markdown を開くという利用形態を前提とした
// 必須の安全対策である（NFR-030）。ここにない要素は黙って取り除かれる。
//
// svg を**追加してはならない**。Alerts のアイコンは Go 側で出さず、
// フロントエンドが後処理で付与する（IMP-112, IMP-225）。
var allowedElements = []string{
	"a", "b", "i", "strong", "em", "u", "s", "del", "ins", "mark", "small",
	"sub", "sup", "br", "hr", "p", "div", "span", "blockquote", "pre", "code",
	"kbd", "samp", "var",
	"h1", "h2", "h3", "h4", "h5", "h6",
	"ul", "ol", "li", "dl", "dt", "dd",
	"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption",
	"colgroup", "col",
	"img", "picture", "source",
	"details", "summary", "figure", "figcaption",
	"abbr", "cite", "q", "time", "ruby", "rt", "rp",
}

// ownClasses は本アプリと goldmark が出力するクラス名（IMP-116）。
//
// chroma のトークンクラスは数が多く更新もされるため、ここには並べず
// chroma.StandardTypes から組み立てる（classPattern）。
var ownClasses = []string{
	"code-block",
	"markdown-alert",
	"markdown-alert-title",
	"markdown-alert-(?:note|tip|important|warning|caution)",
	"math-inline",
	"math-block",
	"mermaid-source",
	"plantuml-source",

	// goldmark の脚注拡張が出すクラス（MD-050）。
	"footnotes",
	"footnote-ref",
	"footnote-backref",
}

// Policy は MD-072 の許可リストを実装した bluemonday ポリシーを返す（IMP-116）。
//
// 変換パイプラインの最後段に固定で置き、迂回経路を作らない（AR-031）。
// html.WithUnsafe()（IMP-111）とこのサニタイズは対で意味を持つ。
// **片方だけを変更してはならない。**
func Policy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	p.AllowElements(allowedElements...)

	// URL は http / https / mailto と相対パス（#アンカー、/__local/ を含む）のみ。
	// javascript: や vbscript: は通らない（MD-072）。
	p.RequireParseableURLs(true)
	p.AllowRelativeURLs(true)
	p.AllowURLSchemes("http", "https", "mailto")

	// data: は画像に限って許可する（MD-072）。
	//
	// bluemonday の AllowDataURIImages は使わない。**あれは image/svg+xml も
	// 許可する。** MD-072 はインライン SVG を除去対象としており、data URI 経由で
	// SVG を持ち込めるのは同じ規定の抜け道になる。
	p.AllowURLSchemeWithCustomPolicy("data", allowDataImage)

	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowAttrs("src", "alt", "title").OnElements("img")
	// abbr は title がなければ意味を持たない（MD-072 の許可要素）。
	// 値は平文であり、ツールチップとして表示されるだけである。
	p.AllowAttrs("title").OnElements("abbr")
	p.AllowAttrs("width", "height").Matching(regexp.MustCompile(`^[0-9]+$`)).OnElements("img")

	// クラスは許可した語の並びだけを通す。任意のクラス名は通さない。
	p.AllowAttrs("class").Matching(classPattern()).
		OnElements("a", "p", "div", "span", "pre", "code", "ol", "li", "sup")

	// 見出しアンカー（MD-021）と脚注の相互リンク（MD-050）に使う。
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^\S+$`)).
		OnElements("h1", "h2", "h3", "h4", "h5", "h6", "li", "sup", "div")

	// Mermaid・PlantUML の描画とコピーボタンが使う（IMP-115, IMP-119）。
	//
	// **data-puml-error を落としてはならない。** これが消えると、取り込み指令で
	// 拒んだブロックが描画対象と区別できなくなり、フロントエンドが外部を取りに
	// 行く図を処理系へ渡してしまう（MD-084, NFR-032）。
	p.AllowAttrs("data-lang", "data-mermaid", "data-source",
		"data-plantuml", "data-puml-error").OnElements("div")

	// 表の桁揃え（MD-024）。style 属性を許可せずに済ませるため、goldmark には
	// align 属性で出力させている（IMP-111）。
	p.AllowAttrs("align").Matching(regexp.MustCompile(`^(?:left|center|right)$`)).
		OnElements("th", "td")

	// タスクリスト（MD-022）のチェックボックス。**MD-072 の許可要素には input が
	// 含まれていない**が、含めないとタスクリストが描画されない。読み取り専用の
	// チェックボックスに限れば、スクリプトを伴わず操作もできない。
	//
	// bluemonday では属性を許可した要素がそのまま許可要素になるため、
	// allowedElements には足さず、例外をこの 1 か所に閉じ込めている。
	// type を持たない input や type="text" の input は通らない。
	//
	// **属性を 1 つでも許可した要素は、その属性だけで残る**（bluemonday v1.0.27）。
	// `<input disabled>` は type が落ちて文字の入力欄として残るため、type="checkbox"
	// を持たない input はサニタイズの後に取り除く（sanitizeAfter。IMP-116）。
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").OnElements("input")

	// 編集・並べ替え・図の目印（IMP-120）。値の形を正規表現で縛り、要素を限る。
	//
	// **形を縛っても偽装は防げない**（書き手は同じ形を書ける）。防ぐのは変換ごとの
	// 鍵であり、照合はフロントエンドの refs.js と Go 側の EditSession が行う
	// （IMP-260, IMP-109）。形を縛るのは、目印以外の用途で属性を通さないためである。
	p.AllowAttrs("data-ref").Matching(editRefPattern).OnElements("input", "table", "th", "td")
	p.AllowAttrs("data-ref").Matching(diagramRefPattern).OnElements("div")
	p.AllowAttrs("data-link").Matching(linkRefPattern).OnElements("a")

	// 支援技術向けの役割。goldmark の脚注が出す doc-* だけを通す。
	p.AllowAttrs("role").Matching(regexp.MustCompile(`^doc-[a-z]+$`)).
		OnElements("a", "div", "li", "sup")

	return p
}

// 目印の値の形（IMP-116, IMP-120）。数は符号なしの 10 進数だけを受け付ける
// （document.ParseRef も同じ形に揃える。UT-108 ケース 5）。
var (
	editRefPattern    = regexp.MustCompile(`^[0-9a-f]{16}:(?:task:[0-9]+|table:[0-9]+|cell:[0-9]+:[0-9]+:[0-9]+)$`)
	diagramRefPattern = regexp.MustCompile(`^[0-9a-f]{16}:(?:mermaid|plantuml):[0-9]+$`)
	linkRefPattern    = regexp.MustCompile(`^[0-9a-f]{16}:`)
)

// sanitizeAfter はサニタイズの後に、属性指定だけでは閉じ込められない 2 つを直す
// （IMP-116, AR-053）。
//
//   - 生 HTML の id 属性に DocumentIDPrefix を付ける（既に付いている値には付けない）
//   - type="checkbox" を持たない input を取り除く
//
// **1 回の走査で行い、開始タグと自己終了タグの両方を見る。** bluemonday は
// `<div id="x"/>` を自己終了タグのまま出し、ブラウザはこれを開始タグとして扱う。
// 開始タグだけを見ると、接頭辞の無い id や入力欄が本文に残る（UT-219 ケース 13、
// UT-209 ケース 23）。
//
// **直さないトークンは Raw() のバイト列をそのまま書く。** 出力を変えない
// （ゴールデンテストに目印と接頭辞以外の差分を出さない。UT-214）。
//
// hasRawHTML が偽なら走査を省く。生 HTML が無ければ、出力の id と input はすべて
// 自分で出したものである（IMP-116。NFR-011）。**数を数えて省いてはならない**——
// bluemonday は object などを中身ごと捨てるため、id や `<input` の数は書き手が
// 合わせられる（UT-219 ケース 12、UT-209 ケース 24）。
func sanitizeAfter(sanitized []byte, hasRawHTML bool) []byte {
	if !hasRawHTML {
		return sanitized
	}

	var out bytes.Buffer
	out.Grow(len(sanitized) + 64)

	z := html.NewTokenizer(bytes.NewReader(sanitized))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			// 入力はメモリ上のバイト列であり、終わり（io.EOF）以外では止まらない。
			break
		}

		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			out.Write(z.Raw())
			continue
		}

		// Raw() は次の Next() で無効になるため、Token() を取る前に写しておく。
		raw := append([]byte(nil), z.Raw()...)
		tok := z.Token()

		if tok.Data == "input" && !isCheckboxInput(tok) {
			// 文字の入力欄として本文に出さない。input は空要素であり、
			// 対になる終了タグは無い。
			continue
		}

		if !prefixRawID(&tok) {
			out.Write(raw)
			continue
		}
		// 組み立て直すタグだけ、属性値を HTML の属性値としてエスケープし直す
		// （Token.String が行う）。トークンの種類（末尾の /> の有無）は保つ。
		out.WriteString(tok.String())
	}

	return out.Bytes()
}

// isCheckboxInput は input のトークンが type="checkbox" を持つかを返す。
// サニタイズを通った後の値であり、type は checkbox 以外が既に落ちている。
func isCheckboxInput(tok html.Token) bool {
	for _, a := range tok.Attr {
		if a.Key == "type" && a.Val == "checkbox" {
			return true
		}
	}
	return false
}

// prefixRawID は id 属性が DocumentIDPrefix で始まらなければ付け、書き換えたかを返す。
//
// **「既に付いている値には付けない」はこの走査に必須の規則である。** 見出しと
// 脚注の id は出力の時点で接頭辞が付いており（IMP-117, IMP-111）、付け直すと
// 二重になる。
func prefixRawID(tok *html.Token) bool {
	changed := false
	for i, a := range tok.Attr {
		if a.Key == "id" && !strings.HasPrefix(a.Val, DocumentIDPrefix) {
			tok.Attr[i].Val = DocumentIDPrefix + a.Val
			changed = true
		}
	}
	return changed
}

// hasRawHTMLKey は、構文木に生 HTML があったかを parser.Context へ置く鍵。
var hasRawHTMLKey = parser.NewContextKey()

// rawHTMLTransformer は構文木に生 HTML（HTMLBlock / RawHTML）があるかを調べ、
// Context に置く（sanitizeAfter の走査を省いてよいかの判断。IMP-116）。
type rawHTMLTransformer struct{}

func (rawHTMLTransformer) Transform(doc *ast.Document, _ text.Reader, pc parser.Context) {
	found := false
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.Kind() {
		case ast.KindHTMLBlock, ast.KindRawHTML:
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	pc.Set(hasRawHTMLKey, found)
}

// classPattern は class 属性に許可する値の正規表現を組み立てる（IMP-116）。
//
// クラスは空白区切りで複数指定できるため、許可した語の並びとして検査する。
// chroma のトークンクラス（`k` `s2` `nf` など）は接頭辞を持たず、数も多い。
// 値を書き写すと chroma の更新で取りこぼし、コードが無色になる。そのため
// chroma.StandardTypes から組み立て、一覧の維持を不要にしている。
func classPattern() *regexp.Regexp {
	words := make([]string, len(ownClasses))
	copy(words, ownClasses)

	var chromaClasses []string
	for _, cls := range chroma.StandardTypes {
		if cls != "" {
			chromaClasses = append(chromaClasses, regexp.QuoteMeta(cls))
		}
	}
	sort.Strings(chromaClasses) // 生成結果を実行ごとに変えない
	words = append(words, chromaClasses...)

	word := "(?:" + strings.Join(words, "|") + ")"
	return regexp.MustCompile("^" + word + "(?: " + word + ")*$")
}

// dataImagePrefix は data: URL のうち許可する種別（MD-072）。
// **svg+xml を加えてはならない。** 生 HTML の svg を除去している意味がなくなる。
var dataImagePrefix = regexp.MustCompile(`^image/(?:gif|jpeg|png|webp);base64,`)

// allowDataImage は data: URL が許可された画像かどうかを判定する。
//
// 種別の申告を信じるだけでなく base64 として復号できることも確かめる。
// 復号できない値は画像として読めず、置いておく理由がない。
func allowDataImage(u *url.URL) bool {
	if u.RawQuery != "" || u.Fragment != "" {
		return false
	}

	prefix := dataImagePrefix.FindString(u.Opaque)
	if prefix == "" {
		return false
	}

	_, err := base64.StdEncoding.DecodeString(u.Opaque[len(prefix):])
	return err == nil
}
