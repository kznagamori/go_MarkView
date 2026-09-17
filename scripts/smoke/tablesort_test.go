// 表の並べ替えの比較の判定に対するテスト（UT-814）。
package main

import (
	"strings"
	"testing"
)

// sortOK は tablesort.js が UT-814 の表どおりに返したときの結果を作る。
func sortOK() sortReport {
	results := make([]sortResult, len(sortCases))

	for i, c := range sortCases {
		res := sortResult{Fn: c.Fn, Args: c.Args}

		switch {
		case c.Fn == fnParseNumber && c.Null:
			res.Type = "object"
		case c.Fn == fnParseNumber:
			v := c.Num
			res.Type, res.Value = "number", &v
		default:
			v := float64(c.Sign)
			res.Type, res.Value = "number", &v
		}

		results[i] = res
	}

	return sortReport{Imported: true, Results: results}
}

// resultAt は入力が一致する結果を返す。
func resultAt(t *testing.T, r *sortReport, fn string, args ...string) *sortResult {
	t.Helper()

	for i := range r.Results {
		if r.Results[i].Fn == fn && strings.Join(r.Results[i].Args, "\x00") == strings.Join(args, "\x00") {
			return &r.Results[i]
		}
	}

	t.Fatalf("%s の結果が無い", describeCall(fn, args))

	return nil
}

func number(v float64) *float64 { return &v }

// TestCheckTableSort は比較の判定を検証する（UT-814 の判定のケース）。
func TestCheckTableSort(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *sortReport)
		want   []string
	}{
		{
			name: "1 すべて表どおりなら合格する",
		},
		{
			name: "2 compareCells が -5 を返しても合格する（過検出の検査。符号だけを見る）",
			mutate: func(t *testing.T, r *sortReport) {
				resultAt(t, r, fnCompareCells, "2", "10").Value = number(-5)
				resultAt(t, r, fnCompareCells, "1,234", "999").Value = number(998)
			},
		},
		{
			name: "3 1 件だけ食い違えば落ち、関数・入力・期待・実際を報告する",
			mutate: func(t *testing.T, r *sortReport) {
				resultAt(t, r, fnCompareCells, "1,234", "999").Value = number(-1)
			},
			want: []string{`compareCells("1,234", "999")`, "期待 正", "実際 -1"},
		},
		{
			name: "4 null を期待する入力で 0 を返したら落ちる（null と 0 を区別する）",
			mutate: func(t *testing.T, r *sortReport) {
				res := resultAt(t, r, fnParseNumber, "abc")
				res.Type, res.Value = "number", number(0)
			},
			want: []string{`parseNumber("abc")`, "期待 null", "実際 0"},
		},
		{
			name: "5 返った結果の件数が表の件数と違えば落ちる",
			mutate: func(t *testing.T, r *sortReport) {
				r.Results = r.Results[:len(r.Results)-1]
			},
			want: []string{"結果が 23 件"},
		},
		{
			name: "6 tablesort.js を読めなければ落ちる",
			mutate: func(t *testing.T, r *sortReport) {
				*r = sortReport{Error: "SyntaxError"}
			},
			want: []string{"tablesort.js を読めない", "SyntaxError"},
		},
		{
			name: "数を期待する入力で null を返したら落ちる",
			mutate: func(t *testing.T, r *sortReport) {
				res := resultAt(t, r, fnParseNumber, "10%")
				res.Type, res.Value = "object", nil
			},
			want: []string{`parseNumber("10%")`, "期待 10", "実際 null"},
		},
		{
			name: "compareCells が数でない値を返したら落ちる",
			mutate: func(t *testing.T, r *sortReport) {
				res := resultAt(t, r, fnCompareCells, "a", "A")
				res.Type, res.Value = "undefined", nil
			},
			want: []string{`compareCells("a", "A")`, "実際 undefined"},
		},
		{
			name: "呼び出しの順が入れ替わっていたら落ちる（ページと検査の食い違い）",
			mutate: func(t *testing.T, r *sortReport) {
				r.Results[0], r.Results[1] = r.Results[1], r.Results[0]
			},
			want: []string{"ページと検査の食い違い"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortOK()
			if tt.mutate != nil {
				tt.mutate(t, &got)
			}

			expectFailures(t, checkTableSort(got), tt.want)
		})
	}
}

// tablesort.js を読めなければ、失敗は 1 件だけ（UT-814 の判定のケース 6）。
func TestCheckTableSort_ImportFailureReportsOnce(t *testing.T) {
	if got := checkTableSort(sortReport{Error: "boom"}); len(got) != 1 {
		t.Fatalf("失敗は 1 件を期待したが %d 件:\n%s", len(got), strings.Join(got, "\n"))
	}
}

// TestSortCases_CoverUT814 は期待値の表が UT-814 の 18 行を欠かさず持つことを見る。
//
// **表の行を落とすと、その規則は一度も確かめられない。** とくに 9〜12 は、
// Intl.Collator の数値の並びだけで比べる実装で符号が逆になる入力である（UT-814）。
func TestSortCases_CoverUT814(t *testing.T) {
	// UT-814 の表の各行が持つ入力の数（5 は 3 つ、7 は 5 つ）。
	wantInputs := map[int]int{
		1: 1, 2: 1, 3: 1, 4: 1, 5: 3, 6: 1, 7: 5,
		8: 1, 9: 1, 10: 1, 11: 1, 12: 1, 13: 1, 14: 1, 15: 1, 16: 1, 17: 1, 18: 1,
	}

	gotInputs := map[int]int{}
	for _, c := range sortCases {
		gotInputs[c.Row]++

		wantFn := fnParseNumber
		if c.Row >= 8 {
			wantFn = fnCompareCells
		}

		if c.Fn != wantFn {
			t.Errorf("UT-814 の %d は %s のはずが %s", c.Row, wantFn, c.Fn)
		}
	}

	for row, want := range wantInputs {
		if gotInputs[row] != want {
			t.Errorf("UT-814 の %d の入力が %d 件（%d 件を期待）", row, gotInputs[row], want)
		}
	}

	if len(gotInputs) != len(wantInputs) {
		t.Errorf("表の行が %d 行（%d 行を期待）", len(gotInputs), len(wantInputs))
	}
}
