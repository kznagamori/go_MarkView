// 見える文字の照合の判定に対するテスト（UT-817）。
//
// **ケース 2 が、この検査を置いた理由そのものである**（BR-054）。画面側と Go 側の数え方が 1 文字でも
// 違うと、そのブロックの修飾は書き込む前の照合で止まり、黙って効かない（FR-146）。**ケース 6 を欠くと、
// 両側とも数え漏らした検査が、照合するものが 0 件のまま毎回通る。**
package main

import (
	"errors"
	"strings"
	"testing"
)

// visibleGo は Go 側の結果の例（段落・見出し・リストの項目・表のセル。UT-817 のケース 1）。
func visibleGo() goVisible {
	return goVisible{Texts: map[string]string{
		"text:0":     "見出し",
		"text:1":     "段落の￼文字",
		"text:2":     "項目",
		"cell:0:0:0": "a",
		"cell:0:1:0": "1",
	}}
}

// visiblePage は Go 側と同じ結果を返したページの例。
func visiblePage() visibleReport {
	return visibleReport{Imported: true, Blocks: []visibleItem{
		{"text:0", "見出し"},
		{"text:1", "段落の￼文字"},
		{"text:2", "項目"},
		{"cell:0:0:0", "a"},
		{"cell:0:1:0", "1"},
	}}
}

// TestCheckVisibleText は見える文字の照合の判定を検証する（UT-817。根拠: FR-146, FR-147, BR-054,
// NFR-061, NFR-030 / IMP-123, IMP-266）。
func TestCheckVisibleText(t *testing.T) {
	tests := []struct {
		name   string
		want   func() goVisible
		mutate func(*visibleReport)
		expect []string // 失敗メッセージに含まれるべき断片。空なら合格を期待
	}{
		{
			name: "1 同じ目印の集合で、どのブロックの文字も同じなら合格する",
		},
		{
			name: "2 1 つのブロックで 1 文字だけ違えば落ちる",
			mutate: func(r *visibleReport) {
				// 画面が U+FFFC の代わりに画像の代替テキストを数えた。
				r.Blocks[1].Text = "段落のalt文字"
			},
			expect: []string{"text:1 が食い違う（最初に違う位置 3）", `Go "段落の￼文字"`, `画面 "段落のalt文字"`},
		},
		{
			name: "3 Go 側にある目印がページの結果に無ければ落ちる",
			mutate: func(r *visibleReport) {
				r.Blocks = r.Blocks[1:]
			},
			expect: []string{"Go 側にある text:0 が画面の結果に無い"},
		},
		{
			name: "4 ページの結果にある目印が Go 側に無ければ落ちる",
			mutate: func(r *visibleReport) {
				r.Blocks = append(r.Blocks, visibleItem{"text:9", "余分"})
			},
			expect: []string{"画面の結果にある text:9 が Go 側に無い"},
		},
		{
			name: "5 ページの結果に同じ目印が 2 回あれば落ちる",
			mutate: func(r *visibleReport) {
				r.Blocks = append(r.Blocks, visibleItem{"text:0", "見出し"})
			},
			expect: []string{"画面の結果に text:0 が 2 回ある"},
		},
		{
			name: "6 text の結果が両側とも 0 件なら落ちる",
			want: func() goVisible {
				return goVisible{Texts: map[string]string{"cell:0:0:0": "a"}}
			},
			mutate: func(r *visibleReport) {
				r.Blocks = []visibleItem{{"cell:0:0:0", "a"}}
			},
			expect: []string{"照合した text のブロックが 0 件"},
		},
		{
			name: "6 cell の結果が両側とも 0 件なら落ちる",
			want: func() goVisible {
				return goVisible{Texts: map[string]string{"text:0": "見出し"}}
			},
			mutate: func(r *visibleReport) {
				r.Blocks = []visibleItem{{"text:0", "見出し"}}
			},
			expect: []string{"照合した cell のブロックが 0 件"},
		},
		{
			name: "8 2 つのブロックで違えば 2 件報告する",
			mutate: func(r *visibleReport) {
				r.Blocks[0].Text = "見出"
				r.Blocks[3].Text = "b"
			},
			expect: []string{"text:0 が食い違う", "cell:0:0:0 が食い違う"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := visibleGo()
			if tt.want != nil {
				want = tt.want()
			}
			got := visiblePage()
			if tt.mutate != nil {
				tt.mutate(&got)
			}

			failures := checkVisibleText(want, got)
			expectFailures(t, failures, tt.expect)

			// ケース 8: 違うブロックごとに 1 件（1 つ目で止めない）。
			if strings.HasPrefix(tt.name, "8 ") && len(failures) != 2 {
				t.Errorf("失敗が %d 件（2 件を期待）:\n%s", len(failures), strings.Join(failures, "\n"))
			}
		})
	}
}

// TestCheckVisibleText_ImportFailureReportsOnce は visible.js を読めないとき、失敗が 1 件だけで
// 他の判定へ進まないことを見る（UT-817 のケース 7。UT-811 のケース 8 と同じ）。
func TestCheckVisibleText_ImportFailureReportsOnce(t *testing.T) {
	got := checkVisibleText(visibleGo(), visibleReport{Error: "SyntaxError"})
	if len(got) != 1 || !strings.Contains(got[0], "visible.js を読めない") {
		t.Fatalf("visible.js を読めない旨の失敗 1 件を期待したが %d 件:\n%s", len(got), strings.Join(got, "\n"))
	}

	// Go 側が失敗したときも 1 件だけ（照合するものが無い）。
	got = checkVisibleText(goVisible{Err: errors.New("boom")}, visiblePage())
	if len(got) != 1 || !strings.Contains(got[0], "VisibleTexts が失敗した") {
		t.Fatalf("Go 側の失敗 1 件を期待したが %d 件:\n%s", len(got), strings.Join(got, "\n"))
	}
}
