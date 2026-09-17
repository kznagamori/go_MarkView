// 文書の id の名前空間の判定に対するテスト（UT-813）。
//
// **過検出のケースを必ず含める**（ケース 2・3・10）。正しい実装で毎回落ちる
// 検査は「厳しすぎる」で片づけられ、やがて外される。
package main

import (
	"strings"
	"testing"
)

// expectFailures は判定の結果を、期待した断片と突き合わせる。
// want が空なら合格（失敗 0 件）を期待する。
func expectFailures(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(want) == 0 {
		if len(got) != 0 {
			t.Fatalf("合格を期待したが %d 件の失敗:\n%s", len(got), strings.Join(got, "\n"))
		}

		return
	}

	if len(got) == 0 {
		t.Fatalf("失敗を期待したが合格した（期待した断片: %v）", want)
	}

	joined := strings.Join(got, "\n")
	for _, fragment := range want {
		if !strings.Contains(joined, fragment) {
			t.Errorf("失敗メッセージに %q が無い:\n%s", fragment, joined)
		}
	}
}

// outsideID は図の外にある XHTML の要素 1 つを作る。
func outsideID(id string) idElement {
	return idElement{ID: id, Namespace: namespaceXHTML}
}

// svgID は図の SVG の中の要素 1 つを作る。
func svgID(diagram, namespace, id string) idElement {
	return idElement{ID: id, Namespace: namespace, InSVG: true, Diagram: diagram}
}

// idsOK は修正後に期待される状態を作る。
//
// 見出しの id はすべて接頭辞付きで、決めた順番の見出しの文字は Status のまま、
// PlantUML の描画は終わっている。
func idsOK() idReport {
	headings := make([]string, statusHeadingIndex+3)
	for i := range headings {
		headings[i] = "見出し"
	}

	headings[statusHeadingIndex] = statusHeadingText

	return idReport{
		Elements: []idElement{
			outsideID("user-content-mermaidmd-080"),
			outsideID("user-content-status"),
			outsideID("user-content-目印の照合br-054-ut-815"),
		},
		Headings:     headings,
		PlantUMLDone: true,
	}
}

// TestCheckIDNamespace は AR-053 の表の判定を検証する（UT-813）。
func TestCheckIDNamespace(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*idReport)
		want   []string
	}{
		{
			name: "1 図の外の id がすべて接頭辞付きで、Status のまま、描画の後なら合格する",
		},
		{
			name: "2 PlantUML の図の器と、その SVG の中の SVG の要素の id は合格する（過検出の検査）",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements,
					idElement{ID: "plantuml-svg-3", Namespace: namespaceXHTML, Container: true, Diagram: "plantuml"},
					svgID("plantuml", namespaceSVG, "ent0001"),
					svgID("plantuml", namespaceSVG, "lnk5"),
				)
			},
		},
		{
			name: "3 Mermaid の SVG の中の SVG の要素の mermaid-svg-1 で始まる id は合格する",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements,
					svgID("mermaid", namespaceSVG, "mermaid-svg-1"),
					svgID("mermaid", namespaceSVG, "mermaid-svg-1_flowchart-v2-pointEnd"),
				)
			},
		},
		{
			name: "4 Mermaid のラベル（XHTML）の id が接頭辞付きなら合格する",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements, svgID("mermaid", namespaceXHTML, "user-content-tooltip"))
			},
		},
		{
			name: "5 Mermaid のラベル（XHTML）の id に接頭辞が無ければ落ちる（IMP-231 が抜けている）",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements, svgID("mermaid", namespaceXHTML, "tooltip"))
			},
			want: []string{"tooltip", "SANITIZE_NAMED_PROPS"},
		},
		{
			name: "6 図の外に接頭辞の無い id が 1 つあれば落ち、その id を報告する",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements, outsideID("status"))
			},
			want: []string{"図の SVG の外", "status"},
		},
		{
			name: "7 決めた順番の見出しの文字が PlantUML のログになっていたら落ちる（BUG-011 そのもの）",
			mutate: func(r *idReport) {
				r.Headings[statusHeadingIndex] = "[    12 ms] PlantUML version 1.2026.7"
			},
			want: []string{"PlantUML version", "書き換えられた"},
		},
		{
			name: "8 返った見出しの数が決めた順番に届かなければ落ちる（検証用文書と検査の食い違い）",
			mutate: func(r *idReport) {
				r.Headings = r.Headings[:statusHeadingIndex]
			},
			want: []string{"見出しが 15 個しか返っていない", "食い違い"},
		},
		{
			name: "9 PlantUML の描画が終わる前に集めた結果は落ちる",
			mutate: func(r *idReport) {
				r.PlantUMLDone = false
			},
			want: []string{"描画が終わる前"},
		},
		{
			name: "10 シーケンス図の actor0 / root-0 とフローチャートの id は合格する（過検出の検査）",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements,
					svgID("mermaid", namespaceSVG, "actor0"),
					svgID("mermaid", namespaceSVG, "root-0"),
					svgID("mermaid", namespaceSVG, "mermaid-svg-2-flowchart-A-0"),
				)
			},
		},
		{
			name: "11 SVG の中の XHTML の要素は mermaid-svg-1-x の形でも落ちる（形で除外しない）",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements, svgID("mermaid", namespaceXHTML, "mermaid-svg-1-x"))
			},
			want: []string{"mermaid-svg-1-x"},
		},
		{
			name: "12 名前空間が返っていない要素があれば落ちる",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements, idElement{ID: "user-content-x"})
			},
			want: []string{"名前空間が返っていない", "user-content-x"},
		},
		{
			name: "図の器でない要素の plantuml-svg-3 は落ちる（器であることと形の両方を見る）",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements, outsideID("plantuml-svg-3"))
			},
			want: []string{"plantuml-svg-3"},
		},
		{
			name: "図の SVG の外にある SVG の要素の id も接頭辞が要る",
			mutate: func(r *idReport) {
				r.Elements = append(r.Elements, idElement{ID: "icon", Namespace: namespaceSVG})
			},
			want: []string{"icon"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idsOK()
			if tt.mutate != nil {
				tt.mutate(&got)
			}

			expectFailures(t, checkIDNamespace(got), tt.want)
		})
	}
}

// 描画の前に集めた結果は、それだけを報告する（UT-813 のケース 9）。
//
// **描画の前の id と見出しを並べると、本当の原因（見る時点の誤り）が埋もれる。**
func TestCheckIDNamespace_BeforeRenderingReportsOnce(t *testing.T) {
	got := idsOK()
	got.PlantUMLDone = false
	got.Elements = append(got.Elements, outsideID("status"))

	if failures := checkIDNamespace(got); len(failures) != 1 {
		t.Fatalf("失敗は 1 件を期待したが %d 件:\n%s", len(failures), strings.Join(failures, "\n"))
	}
}
