package main

import (
	"fmt"
	"strconv"
	"strings"
)

// 表の並べ替えの比較の検査（BR-054, E2E-109 の 9, FR-130, IMP-229）。
//
// ページは本番の tablesort.js の parseNumber / compareCells を、下の表の入力で
// 呼んで返すだけである。**期待値はここに人が書いたリテラルとして持つ**（UT-031）。
// **この表を変えるときは FR-130 と IMP-229 を先に読む。** 31 章の UT-814 の表が正である。

// sortCase は UT-814 の表の 1 行のうちの 1 入力。
type sortCase struct {
	Row  int      // UT-814 の表の番号
	Fn   string   // parseNumber / compareCells
	Args []string // 入力

	// parseNumber の期待値。Null が真なら null（**0 ではない**）。
	Num  float64
	Null bool

	// compareCells の期待値の符号（-1 / 0 / 1）。**符号だけを見る。**
	Sign int
}

const (
	fnParseNumber  = "parseNumber"
	fnCompareCells = "compareCells"
)

func parseCase(row int, in string, want float64) sortCase {
	return sortCase{Row: row, Fn: fnParseNumber, Args: []string{in}, Num: want}
}

func parseNull(row int, in string) sortCase {
	return sortCase{Row: row, Fn: fnParseNumber, Args: []string{in}, Null: true}
}

func compareCase(row int, a, b string, sign int) sortCase {
	return sortCase{Row: row, Fn: fnCompareCells, Args: []string{a, b}, Sign: sign}
}

// sortCases は UT-814 の表（18 行）。**1 行に入力が複数あるものは入力ごとに分ける。**
var sortCases = []sortCase{
	parseCase(1, "1,234", 1234),
	parseCase(2, "10%", 10),
	parseCase(3, "-1.5", -1.5),
	parseCase(4, " 3 ", 3),
	parseCase(5, "+2", 2),
	parseCase(5, "1.", 1),
	parseCase(5, ".5", 0.5),
	parseCase(6, "1,2,3", 123),
	parseNull(7, "10%%"),
	parseNull(7, "%10"),
	parseNull(7, "1e3"),
	parseNull(7, "abc"),
	parseNull(7, ""),
	compareCase(8, "2", "10", -1),
	compareCase(9, "1,234", "999", 1),      // 文字の並びで比べると負になる
	compareCase(10, "-5", "-10", 1),        // 文字の並びで比べると負になる
	compareCase(11, "1.5", "1.25", 1),      // 数字の並びを整数として比べると負になる
	compareCase(12, "10", "9a", -1),        // 文字の並びで比べると正になる
	compareCase(13, "item2", "item10", -1), // 数字の並びを数の大小で比べる
	compareCase(14, "apple", "Banana", -1), // 大文字小文字を区別しない
	compareCase(15, "a", "A", 0),
	compareCase(16, "1", "a", -1), // 数を先に置く
	compareCase(17, "", "1", 1),   // 空のセルを後ろに置く
	compareCase(18, "", "", 0),
}

// sortCaseInput はページへ渡す入力（期待値は渡さない）。
type sortCaseInput struct {
	Fn   string   `json:"fn"`
	Args []string `json:"args"`
}

func sortCaseInputs() []sortCaseInput {
	inputs := make([]sortCaseInput, len(sortCases))
	for i, c := range sortCases {
		inputs[i] = sortCaseInput{Fn: c.Fn, Args: c.Args}
	}

	return inputs
}

// sortReport は tablesort.js を呼んだ結果（harness.js と対になる）。
type sortReport struct {
	Imported bool         `json:"imported"`
	Error    string       `json:"error"`
	Results  []sortResult `json:"results"`
}

// sortResult は 1 回の呼び出しの結果。
type sortResult struct {
	Fn    string   `json:"fn"`
	Args  []string `json:"args"`
	Type  string   `json:"type"`  // typeof の値。null は "object"
	Value *float64 `json:"value"` // 数でなければ nil（null / NaN / undefined など）
}

// checkTableSort は比較の関数が UT-814 の表どおりの値を返すかを判定する（UT-814）。
func checkTableSort(got sortReport) []string {
	// **読めなければ 1 件だけ報告する**（UT-811 のケース 8 と同じ）。
	if !got.Imported {
		return []string{"表の並べ替え: tablesort.js を読めない: " + got.Error}
	}

	var failures []string

	if len(got.Results) != len(sortCases) {
		failures = append(failures,
			fmt.Sprintf("表の並べ替え: 結果が %d 件（%d 件を期待。ケースの抜け）", len(got.Results), len(sortCases)))
	}

	for i, want := range sortCases {
		if i >= len(got.Results) {
			break
		}

		res := got.Results[i]
		call := describeCall(want.Fn, want.Args)

		if res.Fn != want.Fn || strings.Join(res.Args, "\x00") != strings.Join(want.Args, "\x00") {
			failures = append(failures,
				fmt.Sprintf("表の並べ替え: %d 件目の呼び出しが %s（%s を期待。ページと検査の食い違い）",
					i, describeCall(res.Fn, res.Args), call))

			continue
		}

		switch want.Fn {
		case fnParseNumber:
			switch {
			case want.Null && !(res.Type == "object" && res.Value == nil):
				// **null と 0 を区別する。** 0 を返すと空のセルが数として並ぶ。
				failures = append(failures,
					fmt.Sprintf("表の並べ替え: %s（UT-814 の %d）: 期待 null、実際 %s", call, want.Row, describeValue(res)))
			case !want.Null && !(res.Type == "number" && res.Value != nil && *res.Value == want.Num):
				failures = append(failures,
					fmt.Sprintf("表の並べ替え: %s（UT-814 の %d）: 期待 %s、実際 %s",
						call, want.Row, strconv.FormatFloat(want.Num, 'g', -1, 64), describeValue(res)))
			}

		case fnCompareCells:
			if res.Type != "number" || res.Value == nil || sign(*res.Value) != want.Sign {
				failures = append(failures,
					fmt.Sprintf("表の並べ替え: %s（UT-814 の %d）: 期待 %s、実際 %s",
						call, want.Row, describeSign(want.Sign), describeValue(res)))
			}
		}
	}

	return failures
}

func sign(v float64) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	default:
		return 0
	}
}

func describeCall(fn string, args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = strconv.Quote(arg)
	}

	return fn + "(" + strings.Join(quoted, ", ") + ")"
}

func describeSign(s int) string {
	switch {
	case s < 0:
		return "負"
	case s > 0:
		return "正"
	default:
		return "0"
	}
}

func describeValue(res sortResult) string {
	if res.Value != nil {
		return strconv.FormatFloat(*res.Value, 'g', -1, 64)
	}

	switch res.Type {
	case "object":
		return "null"
	case "number":
		// JSON は NaN / Infinity を null にする。
		return "NaN または無限大"
	default:
		return res.Type
	}
}
