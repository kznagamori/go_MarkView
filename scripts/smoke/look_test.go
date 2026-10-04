// Mermaid の図の見た目（look）の判定に対するテスト（UT-816。BUG-021）。
//
// **期待値は人が書いたリテラルにする**（UT-031）。look の値は mermaidLook を使わずに書く——
// mermaidLook を書き換えたときに、このテストも一緒に通ってしまわないようにする。
package main

import (
	"strings"
	"testing"
)

// lookBlock は描けた Mermaid のブロック 1 件ぶんを作る。
func lookBlock(head string, filtered int, looks ...string) diagramBlock {
	return diagramBlock{Head: head, SVG: 1, Width: 400, Height: 300, Looks: looks, Filtered: filtered}
}

// lookOK は IMP-231 の look（classic）で描けたときの結果を作る（12.0.0 の実測）。
//
// シーケンス図・ガント・円グラフは classic では data-look を出さない。
func lookOK() []diagramBlock {
	return []diagramBlock{
		lookBlock("flowchart TD", 0, "classic"),
		lookBlock("sequenceDiagram", 0),
		lookBlock("classDiagram", 0, "classic"),
		lookBlock("stateDiagram-v2", 0, "classic"),
		lookBlock("erDiagram", 0, "classic"),
		lookBlock("gantt", 0),
		lookBlock("pie title Sources", 0),
		lookBlock("graph LR", 0, "classic"),
	}
}

// TestCheckMermaidLook は look と影の判定を検証する（UT-816）。
func TestCheckMermaidLook(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]diagramBlock) []diagramBlock
		want   []string // 失敗メッセージに含まれるべき断片。空なら合格を期待
	}{
		{
			name: "1 すべて classic で影が無ければ合格する",
		},
		{
			name: "2 data-look を持たない図（シーケンス図など）が混ざっても合格する（過検出の検査）",
			mutate: func(b []diagramBlock) []diagramBlock {
				return append(b, lookBlock("journey", 0))
			},
		},
		{
			name: "3 look の指定が消えた（neo で描かれ、影が付く）なら落ちる（BUG-021 そのもの）",
			mutate: func(b []diagramBlock) []diagramBlock {
				b[0] = lookBlock("flowchart TD", 10, "neo")
				b[1] = lookBlock("sequenceDiagram", 6, "neo")
				return b
			},
			want: []string{
				"flowchart TD: look が neo",
				"flowchart TD: 影（filter）の付いた要素が 10 個",
				"sequenceDiagram: look が neo",
				"sequenceDiagram: 影（filter）の付いた要素が 6 個",
			},
		},
		{
			name: "4 1 つの図の中に classic と neo が混ざれば落ちる",
			mutate: func(b []diagramBlock) []diagramBlock {
				b[3] = lookBlock("stateDiagram-v2", 0, "classic", "neo")
				return b
			},
			want: []string{"stateDiagram-v2: look が neo"},
		},
		{
			name: "5 classic のままでも影が付けば落ちる（DSP-270）",
			mutate: func(b []diagramBlock) []diagramBlock {
				b[2] = lookBlock("classDiagram", 2, "classic")
				return b
			},
			want: []string{"classDiagram: 影（filter）の付いた要素が 2 個"},
		},
		{
			name: "6 handDrawn でも落ちる（影が無くても look を見る）",
			mutate: func(b []diagramBlock) []diagramBlock {
				b[7] = lookBlock("graph LR", 0, "handDrawn")
				return b
			},
			want: []string{"graph LR: look が handDrawn"},
		},
		{
			name: "7 描けた図のどれも data-look を持たなければ落ちる（何も見ずに通らない）",
			mutate: func(b []diagramBlock) []diagramBlock {
				for i := range b {
					b[i].Looks = nil
				}
				return b
			},
			want: []string{"data-look を持つ Mermaid の図が 1 つも無い"},
		},
		{
			name: "8 描けていないブロックは数えない（checkMermaid が落とす。同じ原因を 2 度数えない）",
			mutate: func(b []diagramBlock) []diagramBlock {
				return append(b,
					diagramBlock{Head: "flowchart LR", SVG: 0, Looks: []string{"neo"}, Filtered: 3},
					diagramBlock{Head: "pie", SVG: 1, Error: "Parse error", Looks: []string{"neo"}},
				)
			},
		},
		{
			name: "9 図が 1 つも描けていなければ、data-look の欠如では落とさない（checkMermaid が落とす）",
			mutate: func([]diagramBlock) []diagramBlock {
				return []diagramBlock{{Head: "flowchart TD", SVG: 0}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := lookOK()
			if tt.mutate != nil {
				blocks = tt.mutate(blocks)
			}

			got := checkMermaidLook(blocks)

			if len(tt.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("合格を期待したが失敗した:\n%s", strings.Join(got, "\n"))
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("失敗は %d 件を期待したが %d 件:\n%s", len(tt.want), len(got), strings.Join(got, "\n"))
			}

			joined := strings.Join(got, "\n")
			for _, fragment := range tt.want {
				if !strings.Contains(joined, fragment) {
					t.Errorf("失敗に %q が含まれない:\n%s", fragment, joined)
				}
			}
		})
	}
}
