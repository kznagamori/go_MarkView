package renderer

import (
	"bytes"
	"fmt"
	"html"
	"strings"
)

// Diagram は PlantUML ファイルの中の図 1 つ（MD-086）。
type Diagram struct {
	Name   string // 見出し（MD-086 の 3）
	Span   Span   // @start の行の頭から、@end の行の終わり（改行の手前）まで。@end が無ければファイルの終わりまで
	Closed bool   // @end の行があったか
}

// SplitPlantUML はファイルの中身（正規化後）を図に切り分ける（MD-086, IMP-126）。
//
//  1. 行頭の空白・タブを除いた後ろが `@start<種類>` で始まる行で図が始まる（大文字小文字を区別しない。
//     種類は英字と数字の並び）。**同じ種類の `@end<種類>` で始まる行で終わる。** 図の中に別の
//     `@start…` があっても、その図の `@end…` まで図とする
//  2. 図の外の行は無視する
//  3. `@end…` が無いまま終わった図は、ファイルの終わりまでを 1 つの図とする（Closed が偽。MD-086 の 4）
//
// Span は図の `@start…` の行の頭（行頭の空白を含む）から、`@end…` の行の終わり（改行の手前）まで。
// 見出しの決め方は diagramName。
func SplitPlantUML(text []byte) []Diagram {
	var (
		diagrams []Diagram
		open     bool   // 図の中か
		kind     string // 開いている図の種類（小文字）
		start    int    // 開いている図の @start の行の頭
		name     string // @start の行の名前
		title    string // 図の中の最初の title の行の文字
	)

	for pos := 0; pos < len(text); {
		end := len(text)
		next := len(text)
		if i := bytes.IndexByte(text[pos:], '\n'); i >= 0 {
			end, next = pos+i, pos+i+1
		}
		line := text[pos:end]
		body := bytes.TrimLeft(line, " \t")

		switch {
		case !open:
			if k, rest, ok := cutStart(body); ok {
				open, kind, start = true, k, pos
				name, title = startName(rest), ""
			}
		case isEndOf(body, kind):
			diagrams = append(diagrams, Diagram{
				Name:   diagramName(name, title, len(diagrams)+1),
				Span:   Span{Start: start, Stop: end},
				Closed: true,
			})
			open = false
		case title == "":
			title = titleOf(body)
		}

		pos = next
	}

	if open {
		diagrams = append(diagrams, Diagram{
			Name:   diagramName(name, title, len(diagrams)+1),
			Span:   Span{Start: start, Stop: len(text)},
			Closed: false,
		})
	}

	return diagrams
}

// cutStart は行（行頭の空白を除いたもの）が `@start<種類>` で始まるなら、小文字の種類と後ろを返す。
func cutStart(body []byte) (kind string, rest []byte, ok bool) {
	const prefix = "@start"
	if len(body) < len(prefix) || !strings.EqualFold(string(body[:len(prefix)]), prefix) {
		return "", nil, false
	}

	n := alnumPrefix(body[len(prefix):])
	if n == 0 {
		return "", nil, false
	}

	return strings.ToLower(string(body[len(prefix) : len(prefix)+n])), body[len(prefix)+n:], true
}

// isEndOf は行（行頭の空白を除いたもの）が、種類 kind の `@end<種類>` で始まるかを返す
// （大文字小文字を区別しない。MD-086 の 1 の「で始まる行」のとおり、後ろに何が続いてもよい）。
func isEndOf(body []byte, kind string) bool {
	word := "@end" + kind

	return len(body) >= len(word) && strings.EqualFold(string(body[:len(word)]), word)
}

// alnumPrefix は b の先頭から続く ASCII の英字と数字の長さを返す。
func alnumPrefix(b []byte) int {
	n := 0
	for n < len(b) && isASCIIAlnum(rune(b[n])) {
		n++
	}

	return n
}

// startName は `@start<種類>` の後ろから図の名前を取り出す（MD-086 の 3）。
//
//   - `@startuml name` は空白の後ろ（前後の空白を除く）
//   - `@startuml(id=name)` は id の値（前後の空白を除く）
func startName(rest []byte) string {
	s := string(rest)
	if strings.HasPrefix(s, "(") {
		inner, _, _ := strings.Cut(s[1:], ")")
		for _, field := range strings.Split(inner, ",") {
			key, val, found := strings.Cut(field, "=")
			if found && strings.EqualFold(strings.TrimSpace(key), "id") {
				return strings.TrimSpace(val)
			}
		}

		return ""
	}

	return strings.TrimSpace(s)
}

// titleOf は図の中の行（行頭の空白を除いたもの）が `title <文字>` の形なら、その文字を返す
// （大文字小文字を区別しない）。**複数行の `title` … `end title` の形は使わない**（文字が無い）。
func titleOf(body []byte) string {
	const word = "title"
	if len(body) <= len(word) || !strings.EqualFold(string(body[:len(word)]), word) {
		return ""
	}
	if c := body[len(word)]; c != ' ' && c != '\t' {
		return ""
	}

	return strings.TrimSpace(string(body[len(word):]))
}

// diagramName は図の見出しを決める（MD-086 の 3）: 名前 → 図の中の最初の title の行 →
// `Diagram <n>`（n は 1 起点の図の番号）。
func diagramName(name, title string, n int) string {
	switch {
	case name != "":
		return name
	case title != "":
		return title
	}

	return fmt.Sprintf("Diagram %d", n)
}

// RenderPlantUMLFile は PlantUML ファイルを図ごとの見出しと図のブロックへ変換する（IMP-126, FR-025）。
//
//   - 図ごとに、h2 の見出しと図のブロックを上から並べる。**図のブロックの形は Markdown の中の
//     PlantUML と同じ**（writePlantUMLBlock を共有する）
//   - **data-source は図の Span の中身**（`@start…` と `@end…` の行を含む）。**処理系へは図ごとに渡す**
//     ——処理系は 1 回の描画で先頭の図しか描かない（MD-086。実測）
//   - **取り込み指令は図ごとに検査する**（hasIncludeDirective。IMP-119）。拒んだ図は data-plantuml を
//     付けず data-puml-error="include"（見出しは付ける）。NeedsPlantUML は描く図があるときだけ
//   - 目印は `<鍵>:plantuml:<n>`（n は図の番号。0 起点）。**拒んだ図も数える**（書き換えの番号。IMP-122）
//   - 見出しは h2 で、アンカーは IMP-117 の slugger（同じ名前には -1 …、`user-content-` を付ける）。
//     **見出しの文字は HTML としてエスケープし、Markdown として解かない**
//   - **図が 1 つも無いファイル**は、ソースの全体を等幅のコードブロックで出し（目印と data-plantuml は
//     付けない）、NoDiagrams を真にする。`No diagrams found.` の文言はフロントエンドが出す（IMP-290）
//   - 修飾の目印（text）は付けない（FR-146 の対象外。TextRefs は見ない）
//   - 出力は IMP-116 のサニタイズを通す。パニックは recover してエラーとして返す（IMP-022）
func (r *Renderer) RenderPlantUMLFile(source []byte, opts Options) (result Result, err error) {
	defer recoverRender(&result, &err)

	diagrams := SplitPlantUML(source)

	var b bytes.Buffer
	headings := []Heading{}
	needs := false

	if len(diagrams) == 0 {
		b.WriteString(`<div class="code-block" data-lang="plantuml"><pre><code>`)
		b.WriteString(html.EscapeString(string(source)))
		b.WriteString("</code></pre></div>\n")

		sanitized := sanitizeAfter(r.policy.SanitizeBytes(b.Bytes()), false)
		return Result{HTML: string(sanitized), Headings: headings, NoDiagrams: true}, nil
	}

	s := newSlugger()
	for i, d := range diagrams {
		// 見出し。接頭辞は id 属性へ書くときに付ける（IMP-117 の 6）。スラッグが空なら id を出さない。
		var id string
		if slug := s.Slug(d.Name); slug != "" {
			id = DocumentIDPrefix + slug
		}
		headings = append(headings, Heading{Level: 2, Text: d.Name, ID: id})

		if id != "" {
			b.WriteString(`<h2 id="` + html.EscapeString(id) + `">`)
		} else {
			b.WriteString("<h2>")
		}
		b.WriteString(html.EscapeString(d.Name))
		b.WriteString("</h2>\n")

		src := source[d.Span.Start:d.Span.Stop]
		rejected := hasIncludeDirective(string(src))
		if !rejected {
			needs = true
		}
		writePlantUMLBlock(&b, src, diagramRef(opts.RefKey, "plantuml", i), rejected, false)
	}

	// 生 HTML は無い（自分で組み立てた）ため、サニタイズの後の走査は省く。
	sanitized := sanitizeAfter(r.policy.SanitizeBytes(b.Bytes()), false)

	return Result{HTML: string(sanitized), Headings: headings, NeedsPlantUML: needs}, nil
}
