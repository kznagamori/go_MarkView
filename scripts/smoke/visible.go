package main

import (
	"fmt"
	"sort"
	"strings"
)

// 見える文字の照合の検査（BR-054, E2E-109 の 12, FR-146, FR-147, IMP-123, IMP-266。UT-817）。
//
// ページは、鍵の合う text と cell の目印を持つブロック（refs.js の isOwnRef）のそれぞれについて、
// 本番の visible.js の visibleText を呼んだ結果を返すだけである。Go 側は同じ文書の
// renderer.VisibleTexts を求めて突き合わせる。**ページは事実だけを返し、判定は Go 側で行う。**
// **Go 側の結果をページへ渡さない**（渡すと照合の意味が無くなる。IMP-266）。

// visibleReport はページから返る見える文字（harness.js と対になる）。
type visibleReport struct {
	Imported bool          `json:"imported"` // visible.js を読めたか
	Error    string        `json:"error"`    // 読めなかった理由
	Blocks   []visibleItem `json:"blocks"`
}

// visibleItem は 1 つのブロックの見える文字。
type visibleItem struct {
	Ref  string `json:"ref"`  // 目印から鍵を除いた値（"text:3" / "cell:0:1:2"）
	Text string `json:"text"` // visibleText の結果
}

// goVisible は Go 側の見える文字（renderer.VisibleTexts の結果）。
type goVisible struct {
	Texts map[string]string
	Err   error
}

// checkVisibleText は見える文字の照合を判定する（UT-817）。**空なら合格。**
//
// **照合したブロックが text と cell の種類ごとに 0 件なら失敗とする**（ケース 6）。両側とも 0 件なら
// 食い違いが出ず、何も見ないまま通る。食い違いはブロックごとに 1 件ずつ報告する（ケース 8）。
func checkVisibleText(want goVisible, got visibleReport) []string {
	// **読めなければ 1 件だけ報告する**（UT-811 のケース 8 と同じ。ケース 7）。
	if !got.Imported {
		return []string{"見える文字の照合: visible.js を読めない: " + got.Error}
	}
	if want.Err != nil {
		return []string{"見える文字の照合: Go 側の renderer.VisibleTexts が失敗した: " + want.Err.Error()}
	}

	var failures []string

	page := map[string]string{}
	for _, b := range got.Blocks {
		if _, dup := page[b.Ref]; dup {
			// ケース 5: 生 HTML で偽装した目印を、鍵を照合せずに数えた（NFR-030）。
			failures = append(failures,
				fmt.Sprintf("見える文字の照合: 画面の結果に %s が 2 回ある（鍵を照合せずに数えていないか。NFR-030）", b.Ref))
			continue
		}
		page[b.Ref] = b.Text
	}

	compared := map[string]int{"text": 0, "cell": 0}

	for _, ref := range sortedKeys(want.Texts) {
		text, ok := page[ref]
		if !ok {
			// ケース 3: 画面が数え漏らした、または目印を描かなかった。
			failures = append(failures,
				fmt.Sprintf("見える文字の照合: Go 側にある %s が画面の結果に無い（Go %q）", ref, want.Texts[ref]))
			continue
		}

		kind, _, _ := strings.Cut(ref, ":")
		compared[kind]++

		if text != want.Texts[ref] {
			// ケース 2・8: ブロックごとに 1 件。最初に違う位置（符号位置）を添える。
			failures = append(failures,
				fmt.Sprintf("見える文字の照合: %s が食い違う（最初に違う位置 %d）。Go %q / 画面 %q",
					ref, firstRuneDiff(want.Texts[ref], text), want.Texts[ref], text))
		}
	}

	for _, ref := range sortedKeys(page) {
		if _, ok := want.Texts[ref]; !ok {
			// ケース 4。
			failures = append(failures,
				fmt.Sprintf("見える文字の照合: 画面の結果にある %s が Go 側に無い（画面 %q）", ref, page[ref]))
		}
	}

	// ケース 6: 種類ごとに数える。
	for _, kind := range []string{"text", "cell"} {
		if compared[kind] == 0 {
			failures = append(failures,
				fmt.Sprintf("見える文字の照合: 照合した %s のブロックが 0 件（検証用文書と検査の食い違い。何も見ないまま通る）", kind))
		}
	}

	return failures
}

// firstRuneDiff は 2 つの文字列で最初に違う符号位置を返す。
func firstRuneDiff(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n := min(len(ra), len(rb))
	for i := range n {
		if ra[i] != rb[i] {
			return i
		}
	}
	return n
}

// sortedKeys は map の鍵を昇順で返す（報告の順を毎回同じにする）。
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
