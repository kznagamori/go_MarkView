package renderer

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/microcosm-cc/bluemonday"
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

	// Mermaid の描画とコピーボタンが使う（IMP-115）。
	p.AllowAttrs("data-lang", "data-mermaid", "data-source").OnElements("div")

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
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").OnElements("input")

	// 支援技術向けの役割。goldmark の脚注が出す doc-* だけを通す。
	p.AllowAttrs("role").Matching(regexp.MustCompile(`^doc-[a-z]+$`)).
		OnElements("a", "div", "li", "sup")

	return p
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
