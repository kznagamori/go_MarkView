// genchroma は frontend/css/chroma.css を生成する（IMP-114, DSP-013）。
//
//	go run ./scripts/genchroma
//
// **クラス名は chroma の型一覧から採り、色は DSP-013 の表から与える。**
// chroma 同梱の github / github-dark スタイルは 2015 年ごろの GitHub の
// 配色（キーワードが黒の太字、文字列が #dd1144）であり、DSP-013 が定める
// 現在の GitHub（Primer）の配色とは別物である。色までそちらから採ると
// MD-002 の「GitHub と並べて比較する」が成り立たない。
//
// 一方でクラス名を手で並べると、chroma が型を増やしたときに取りこぼす。
// 実際 DSP-013 が挙げる 11 行は代表例であり、`kc`（true / nil）や
// `si`（文字列内の式展開）のように、同じ系統でありながら表に無いクラスが
// 実際の文書に現れる。系統ごとにまとめて色を与えるのが本スクリプトの役割。
package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
)

// 出力先。リポジトリのルートから実行する前提とする。
const outPath = "frontend/css/chroma.css"

// entry は DSP-013 の表の 1 行。**ここが色の唯一の出どころ。**
//
// direct が入っている項目は --chroma-* を作らず、規則の側にその式を
// 直接書く。DSP-013 の値が既存のトークンと同じ色であり、二重に持ちたく
// ないためだが、**カスタムプロパティ経由にしてはならない理由がある。**
//
// カスタムプロパティの計算値は、宣言した要素の上で var() を解決したもの
// である。:root に --chroma-punct: var(--fg-default) と書くと、そこで
// Light の #1f2328 に確定してから継承する。テーマの切り替えは #app の
// data-theme で行うため（IMP-243）、:root は常に Light のままであり、
// Dark にしても演算子だけ黒いまま残る（2026-09-02 に実機で再現した）。
// 規則の側に var(--fg-default) と書けば、解決は使う場所で起きる。
type entry struct {
	name    string // CSS 変数名（--chroma- を除いた部分）
	light   string
	dark    string
	direct  string // 変数を作らず、規則へ直接書く式
	comment string
}

var palette = []entry{
	{name: "keyword", light: "#cf222e", dark: "#ff7b72", comment: "キーワード"},
	{name: "string", light: "#0a3069", dark: "#a5d6ff", comment: "文字列"},
	{name: "number", light: "#0550ae", dark: "#79c0ff", comment: "数値"},
	{name: "comment", direct: "var(--fg-muted)", comment: "コメント（DSP-013 の値は --fg-muted と同じ）"},
	{name: "entity", light: "#8250df", dark: "#d2a8ff", comment: "関数名・識別子"},
	{name: "tag", light: "#116329", dark: "#7ee787", comment: "タグ名"},
	{name: "attr", light: "#0550ae", dark: "#79c0ff", comment: "属性名"},
	{name: "punct", direct: "var(--fg-default)", comment: "演算子・区切り（DSP-013 の値は --fg-default と同じ）"},
	{name: "deleted-fg", light: "#82071e", dark: "#ffdcd7", comment: "diff 削除行の文字"},
	{name: "deleted-bg", light: "#ffebe9", dark: "#67060c", comment: "diff 削除行の背景"},
	{name: "inserted-fg", light: "#116329", dark: "#aff5b4", comment: "diff 追加行の文字"},
	{name: "inserted-bg", light: "#dafbe1", dark: "#033a16", comment: "diff 追加行の背景"},
}

// 系統ごとの見出し。出力の並び順もこれに従う。
var groups = []struct {
	family  string
	comment string
}{
	{"keyword", "キーワード（Keyword 系すべて）"},
	{"entity", "関数名・識別子"},
	{"tag", "タグ名"},
	{"attr", "属性名"},
	{"string", "文字列（LiteralString 系すべて）"},
	{"number", "数値（LiteralNumber 系すべて）"},
	{"comment", "コメント（Comment 系すべて）"},
	{"punct", "演算子・区切り"},
}

// familyOf はトークンの型を DSP-013 の系統へ割り当てる。
//
// **表に無い型は色を持たせない。** 空文字を返した型には規則を出さず、
// 本文の文字色（--fg-default）のまま表示する。DSP-013 が `.err` を
// 強調しないと定めているのと同じ考えで、色の付けすぎを避ける。
func familyOf(t chroma.TokenType) string {
	switch {
	case t == chroma.NameTag:
		return "tag"
	case t == chroma.NameAttribute:
		return "attr"
	case t == chroma.NameFunction, t == chroma.NameFunctionMagic, t == chroma.NameOther:
		return "entity"
	case t.Category() == chroma.Keyword:
		return "keyword"
	case t.SubCategory() == chroma.LiteralString:
		return "string"
	case t.SubCategory() == chroma.LiteralNumber:
		return "number"
	case t.Category() == chroma.Comment:
		return "comment"
	case t.Category() == chroma.Operator, t.Category() == chroma.Punctuation:
		return "punct"
	}

	return ""
}

// colorExpr は系統に与える色の式を返す。
func colorExpr(family string) string {
	for _, e := range palette {
		if e.name != family {
			continue
		}
		if e.direct != "" {
			return e.direct
		}

		return "var(--chroma-" + family + ")"
	}

	return "inherit"
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genchroma:", err)
		os.Exit(1)
	}
}

func run() error {
	var b bytes.Buffer

	writeHeader(&b)
	writePalette(&b)
	writeGroups(&b)
	writeDiff(&b)
	writeError(&b)

	return os.WriteFile(outPath, b.Bytes(), 0o644)
}

func writeHeader(b *bytes.Buffer) {
	b.WriteString(`/*
  chroma.css — シンタックスハイライトの配色（DSP-013）。

  **このファイルは scripts/genchroma が生成する。手で書き換えない。**

      go run ./scripts/genchroma

  Go 側は WithClasses(true) でクラス名だけを出力するため（IMP-114）、
  色はここで与える。インラインスタイルを出さないことで、テーマの切り替えが
  再変換なしで済む（UI-105）。

  クラス名は chroma の型一覧から、色は DSP-013 の表から採っている。
  表に挙がっていない系統には色を与えず、本文の文字色のままにする。
*/

`)
}

func writePalette(b *bytes.Buffer) {
	b.WriteString(`/* --- 配色（DSP-013）--- */

/* **トークンを参照する色をここに置かない。** :root で var() を書くと
   その場で Light の値に確定してから継承するため、Dark にしても
   切り替わらない。該当する 2 つ（コメント・演算子）は規則の側に
   直接書いている。 */
:root {
`)

	for _, e := range palette {
		if e.direct != "" {
			continue
		}

		fmt.Fprintf(b, "  /* %s */\n", e.comment)
		fmt.Fprintf(b, "  --chroma-%s: %s;\n", e.name, e.light)
	}

	b.WriteString("}\n\n[data-theme=\"dark\"] {\n")

	for _, e := range palette {
		if e.direct != "" {
			continue
		}

		fmt.Fprintf(b, "  --chroma-%s: %s;\n", e.name, e.dark)
	}

	b.WriteString("}\n")
}

func writeGroups(b *bytes.Buffer) {
	// 型 -> クラス名。系統ごとに集めて 1 つの規則にまとめる。
	byFamily := map[string][]string{}

	for t, class := range chroma.StandardTypes {
		if class == "" || t == chroma.Error {
			continue
		}

		if f := familyOf(t); f != "" {
			byFamily[f] = append(byFamily[f], class)
		}
	}

	for _, g := range groups {
		classes := byFamily[g.family]
		if len(classes) == 0 {
			continue
		}

		sort.Strings(classes)

		selectors := make([]string, len(classes))
		for i, c := range classes {
			selectors[i] = ".chroma ." + c
		}

		fmt.Fprintf(b, "\n/* %s */\n%s {\n  color: %s;\n}\n",
			g.comment, strings.Join(selectors, ",\n"), colorExpr(g.family))
	}
}

func writeDiff(b *bytes.Buffer) {
	b.WriteString(`
/* diff の削除行・追加行。**背景を敷くため行全体に効かせる** */
.chroma .gd {
  color: var(--chroma-deleted-fg);
  background-color: var(--chroma-deleted-bg);
}

.chroma .gi {
  color: var(--chroma-inserted-fg);
  background-color: var(--chroma-inserted-bg);
}
`)
}

func writeError(b *bytes.Buffer) {
	b.WriteString(`
/* 字句エラー。**強調しない**（DSP-013）。
   未対応の言語や部分的な解析失敗で本文が赤く染まるのを避ける。
   chroma 同梱のスタイルは赤字に赤の地を与えるが、それは採らない。
   規則を書かずに済ませないのは、上の系統の規則が将来 .err を巻き込んで
   色を付けたときに、ここで打ち消せるようにしておくためである。 */
.chroma .err {
  color: var(--fg-default);
  background-color: transparent;
}
`)
}
