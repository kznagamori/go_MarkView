package main

import (
	"fmt"
	"regexp"
	"strings"
)

// 文書の id の名前空間の検査（BR-054, E2E-109 の 8, AR-053, BUG-011）。
//
// **ページは事実だけを返し、判定はここで行う**（BR-054）。除外をページで
// 済ませると、除外の誤りを単体テスト（UT-813）で確かめられない。

// documentIDPrefix は文書から生まれる id の接頭辞（AR-053）。
const documentIDPrefix = "user-content-"

// statusHeadingIndex は testdata/smoke.md の中で `Status` の見出しが何番目か
// （0 起点。h1〜h6 を文書の順に数える）。**testdata/smoke.md と一致させる。**
//
// **文字でも id でも探さない。** 同梱の plantuml.js は id="status" の要素の
// 文字を自分のログで書き換える（BUG-011）。文字で探すと、書き換えられた
// 見出しは「見つからない」になり、症状と検証用文書の食い違いを区別できない。
// id で探すと、接頭辞の有無で見つかり方が変わる。
const statusHeadingIndex = 15

// statusHeadingText は `Status` の見出しの文字。
const statusHeadingText = "Status"

// 名前空間（ページが要素の namespaceURI から返す値）。
const (
	namespaceXHTML = "xhtml"
	namespaceSVG   = "svg"
)

// plantUMLHolderID は PlantUML の図の器の id（IMP-233）。フロントエンドが
// 本文の中に作る要素であり、`user-content-` で始めない（AR-053 の表の「画面」）。
var plantUMLHolderID = regexp.MustCompile(`^plantuml-svg-[0-9]+$`)

// idReport は本文（#markdown）の中の id の事実（harness.js と対になる）。
type idReport struct {
	Elements     []idElement `json:"elements"`
	Headings     []string    `json:"headings"`     // h1〜h6 の文字。文書の順
	PlantUMLDone bool        `json:"plantumlDone"` // 集めた時点で PlantUML の描画が終わっていたか
}

// idElement は id を持つ要素 1 つの事実。
type idElement struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"` // xhtml / svg / それ以外の URI。取れなければ空
	InSVG     bool   `json:"inSvg"`     // 図（.mermaid-rendered / .plantuml-rendered）の SVG の中か
	Container bool   `json:"container"` // 図の器（.mermaid-rendered / .plantuml-rendered）そのものか
	Diagram   string `json:"diagram"`   // mermaid / plantuml / 空（図の外）
}

// checkIDNamespace は本文の id が AR-053 の表のとおりかを判定する（UT-813）。
//
// 規則:
//
//   - 図の SVG の外では、id は `user-content-` で始まるか、PlantUML の図の器の
//     plantuml-svg-N であること
//   - 図の SVG の中では、SVG 名前空間の要素の id は処理系のものとして判定しない。
//     それ以外（Mermaid のラベルに書き手が書いた HTML。XHTML 名前空間）の id は
//     `user-content-` で始まること
//
// **id の形（mermaid-svg- で始まるか など）では除外しない。** 処理系の id は
// その形とは限らず（シーケンス図の actor0）、形で除外すると書き手がラベルに
// 同じ形の id を書いたときに見逃す。
func checkIDNamespace(got idReport) []string {
	// **描画の前に集めた結果では何も言えない。** plantuml.js はログを出すたびに
	// #status を書き換えるため、描画の前に見ると衝突していても通る（BR-054）。
	if !got.PlantUMLDone {
		return []string{"文書の id: PlantUML の描画が終わる前に集めた結果である（描画の後に見る。BR-054）"}
	}

	var (
		failures  []string
		noNS      []string
		outside   []string
		inDiagram []string
	)

	for _, el := range got.Elements {
		switch {
		case el.Namespace == "":
			// ページと判定の食い違い。黙って合格にしない。
			noNS = append(noNS, el.ID)
		case el.InSVG && el.Namespace == namespaceSVG:
			// 処理系の id（mermaid-svg-N、actor0、ent0001 など）。判定しない。
		case el.InSVG:
			if !strings.HasPrefix(el.ID, documentIDPrefix) {
				inDiagram = append(inDiagram, fmt.Sprintf("%s（%s の図）", el.ID, el.Diagram))
			}
		case el.Container && el.Diagram == "plantuml" && plantUMLHolderID.MatchString(el.ID):
			// フロントエンドが作る図の器（IMP-233）。
		case !strings.HasPrefix(el.ID, documentIDPrefix):
			outside = append(outside, el.ID)
		}
	}

	if len(noNS) > 0 {
		failures = append(failures,
			"文書の id: 名前空間が返っていない要素がある: "+strings.Join(noNS, ", "))
	}

	if len(outside) > 0 {
		failures = append(failures,
			fmt.Sprintf("文書の id: 図の SVG の外に %s で始まらない id が %d 個（AR-053, BUG-011）: %s",
				documentIDPrefix, len(outside), strings.Join(outside, ", ")))
	}

	if len(inDiagram) > 0 {
		failures = append(failures,
			fmt.Sprintf("文書の id: 図の SVG の中の XHTML の要素に %s で始まらない id がある"+
				"（Mermaid のラベル。IMP-231 の SANITIZE_NAMED_PROPS。BUG-011）: %s",
				documentIDPrefix, strings.Join(inDiagram, ", ")))
	}

	switch {
	case len(got.Headings) <= statusHeadingIndex:
		failures = append(failures,
			fmt.Sprintf("文書の id: 見出しが %d 個しか返っていない（%d 番目の %s を期待。testdata/smoke.md と検査の食い違い）",
				len(got.Headings), statusHeadingIndex, statusHeadingText))
	case got.Headings[statusHeadingIndex] != statusHeadingText:
		failures = append(failures,
			fmt.Sprintf("文書の id: %d 番目の見出しの文字が %q になっている（%q を期待。同梱資産に書き換えられた。BUG-011）",
				statusHeadingIndex, got.Headings[statusHeadingIndex], statusHeadingText))
	}

	return failures
}
