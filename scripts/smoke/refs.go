package main

import (
	"fmt"
	"strings"
)

// 目印の照合の検査（BR-054, E2E-109 の 10, FR-061, FR-063, FR-130, FR-141,
// FR-142, MD-084, NFR-030, IMP-120, IMP-229, IMP-260, BUG-014）。
//
// ページは本番の refs.js / tablesort.js / lazy.js を呼んだ結果を、要素の種類
// ごとに文書の順で返すだけである。**どの要素がどの由来かは、ここにリテラルで
// 持つ**（smokeRefLayout）。文書の側に印を足さない——サニタイズを通る属性で
// 印を付けると、その印自体が照合の結果に影響しうる。

// 要素の由来（UT-815）。
const (
	originGFM      = "gfm"           // GFM の要素・Go 側が出した図のブロック
	originWrongKey = "raw-wrong-key" // 生 HTML で別の鍵を書いた目印
	originNoKey    = "raw-no-key"    // 目印の無い生 HTML
	originBadShape = "raw-bad-shape" // 形の崩れた目印
)

var allOrigins = []string{originGFM, originWrongKey, originNoKey, originBadShape}

// refKinds は isOwnRef の種類（IMP-260）。ページはどの要素にも 5 種類すべてで呼ぶ。
var refKinds = []string{"task", "table", "cell", "mermaid", "plantuml"}

// 図のブロックの種類（blockSpec.Kind）。code は data-source だけを持つコードブロック。
const (
	blockMermaid  = "mermaid"
	blockPlantUML = "plantuml"
	blockCode     = "code"
)

// linkSpec はリンク 1 つの由来と、GFM のリンクなら書かれたとおりのリンク先。
type linkSpec struct {
	Origin string
	Target string
}

// blockSpec は図のブロック 1 つの由来と種類。
type blockSpec struct {
	Origin string
	Kind   string
}

// refLayout は検証用文書の中の要素の並び（種類ごとの出現順）。
type refLayout struct {
	Checkboxes []string
	Tables     []string
	Cells      []string
	Links      []linkSpec
	Blocks     []blockSpec
}

// smokeRefLayout は testdata/smoke.md の並び。**testdata/smoke.md と一致させる。**
// 食い違うと「数が違う」で落ちる（UT-815 のケース 9）。
var smokeRefLayout = refLayout{
	Checkboxes: []string{
		originGFM, originGFM, // - [ ] / - [x]
		originWrongKey, originNoKey, originBadShape,
	},
	Tables: []string{
		originGFM, // 本体 2 行（ボタンが付く）
		originGFM, // 本体 1 行（ボタンが付かない）
		originWrongKey, originNoKey, originBadShape,
	},
	Cells: []string{
		// 本体 2 行の表: 見出し 2 + 本体 2 行 x 2 列
		originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
		// 本体 1 行の表: 見出し 2 + 本体 1 行 x 2 列
		originGFM, originGFM, originGFM, originGFM,
		// 生 HTML の表（1 列。見出し 1 + 本体 2 行）
		originWrongKey, originWrongKey, originWrongKey,
		originNoKey, originNoKey, originNoKey,
		originBadShape, originBadShape, originBadShape,
	},
	Links: []linkSpec{
		{originGFM, "../docs/bugs/2026-09-06-bug-008-broken-image-alt-webkitgtk.md"},
		{originGFM, "../docs/bugs/2026-09-14-bug-011-document-id-collision.md"},
		{originGFM, "./e2e/docs/design.md#api"},
		{originGFM, "https://example.com/p"},
		{Origin: originWrongKey},
		{Origin: originNoKey},
		{Origin: originBadShape},
		{originGFM, "../docs/bugs/2026-09-14-bug-014-diagram-marker-spoofing.md"},
	},
	Blocks: []blockSpec{
		// Mermaid の 7 種類
		{originGFM, blockMermaid}, {originGFM, blockMermaid}, {originGFM, blockMermaid},
		{originGFM, blockMermaid}, {originGFM, blockMermaid}, {originGFM, blockMermaid},
		{originGFM, blockMermaid},
		// PlantUML の 2 種類
		{originGFM, blockPlantUML}, {originGFM, blockPlantUML},
		// ラベルに id を書いた Mermaid
		{originGFM, blockMermaid},
		// 生 HTML で偽装したブロック
		{originWrongKey, blockPlantUML}, // !include を含む
		{originNoKey, blockMermaid},
		{originBadShape, blockMermaid},
		{originNoKey, blockCode}, // data-source だけを偽装したコードブロック
	},
}

// refReport は照合の結果（harness.js と対になる）。
type refReport struct {
	Imported     bool   `json:"imported"`     // refs.js を読めたか
	Error        string `json:"error"`        // 読めなかった理由
	SortImported bool   `json:"sortImported"` // tablesort.js を読めたか（ボタンの数を見るか）

	Checkboxes []refElement   `json:"checkboxes"`
	Tables     []refElement   `json:"tables"`
	Cells      []refElement   `json:"cells"`
	Links      []linkElement  `json:"links"`
	Blocks     []blockElement `json:"blocks"`
}

// refElement はチェックボックス・表・セル 1 つの事実。
type refElement struct {
	Own         map[string]bool `json:"own"`         // isOwnRef(element, 種類) の結果
	SortButtons int             `json:"sortButtons"` // 表: attachSortButtons の後のボタンの数
	BodyRows    int             `json:"bodyRows"`    // 表: tbody の行の数
}

// linkElement はリンク 1 つの事実。
type linkElement struct {
	Href   string  `json:"href"`
	Target *string `json:"target"` // ownLinkTarget の結果。null なら nil
}

// blockElement は図のブロック 1 つの事実（data-mermaid / data-plantuml /
// data-puml-error / data-source のどれかを持つ div.code-block）。
type blockElement struct {
	Own        map[string]bool `json:"own"`
	Source     *string         `json:"source"`     // ownSource の結果。null なら nil
	DataSource string          `json:"dataSource"` // data-source 属性の値
	Rendered   bool            `json:"rendered"`   // 図の器（.mermaid-rendered / .plantuml-rendered）ができたか
}

// failureSet は同じ問題の要素をまとめて 1 行にする。**要素ごとに 1 行を並べると、
// 照合が効いていないときに数十行になり、何が起きたかが読めなくなる。**
type failureSet struct {
	order []string
	items map[string][]string
}

func (f *failureSet) add(message string, index int) {
	if f.items == nil {
		f.items = map[string][]string{}
	}

	if _, ok := f.items[message]; !ok {
		f.order = append(f.order, message)
	}

	f.items[message] = append(f.items[message], fmt.Sprintf("#%d", index))
}

func (f *failureSet) lines() []string {
	lines := make([]string, 0, len(f.order))
	for _, message := range f.order {
		lines = append(lines, message+": "+strings.Join(f.items[message], ", "))
	}

	return lines
}

// checkRefMatching は目印の照合を判定する（UT-815）。
//
// **「対象にしないこと」と「対象にすること」の両方を見る。** 生 HTML の側だけ
// なら「すべて偽」、GFM の側だけなら「すべて真」の実装が通る（BR-054）。
func checkRefMatching(layout refLayout, got refReport) []string {
	// **読めなければ 1 件だけ報告する**（UT-811 のケース 8 と同じ）。
	if !got.Imported {
		return []string{"目印の照合: refs.js を読めない: " + got.Error}
	}

	failures := checkLayout(layout)

	var set failureSet

	// チェックボックス・表・セル。
	elementKinds := []struct {
		name    string
		kind    string
		origins []string
		got     []refElement
	}{
		{"チェックボックス", "task", layout.Checkboxes, got.Checkboxes},
		{"表", "table", layout.Tables, got.Tables},
		{"セル", "cell", layout.Cells, got.Cells},
	}

	for _, ek := range elementKinds {
		if len(ek.got) != len(ek.origins) {
			failures = append(failures,
				fmt.Sprintf("目印の照合: %sが %d 個（一覧は %d 個。testdata/smoke.md と検査の食い違い）",
					ek.name, len(ek.got), len(ek.origins)))

			continue
		}

		for i, el := range ek.got {
			checkOwn(&set, ek.name, ek.kind, ek.origins[i], el.Own, i)
		}
	}

	if len(got.Tables) == len(layout.Tables) && got.SortImported {
		for i, table := range got.Tables {
			origin := layout.Tables[i]

			switch {
			case origin != originGFM && table.SortButtons > 0:
				set.add(fmt.Sprintf("目印の照合: 生 HTML の表（%s）に並べ替えのボタンがある（FR-130）", origin), i)
			case origin == originGFM && table.BodyRows < 2 && table.SortButtons > 0:
				set.add("目印の照合: 本体が 1 行の GFM の表に並べ替えのボタンがある（FR-130 の「2 行以上」）", i)
			case origin == originGFM && table.BodyRows >= 2 && table.SortButtons == 0:
				set.add("目印の照合: 本体が 2 行以上の GFM の表に並べ替えのボタンが無い（FR-130）", i)
			}
		}
	}

	// リンク。
	if len(got.Links) != len(layout.Links) {
		failures = append(failures,
			fmt.Sprintf("目印の照合: リンクが %d 個（一覧は %d 個。testdata/smoke.md と検査の食い違い）",
				len(got.Links), len(layout.Links)))
	} else {
		for i, link := range got.Links {
			spec := layout.Links[i]

			switch {
			case spec.Origin != originGFM && link.Target != nil:
				set.add(fmt.Sprintf("目印の照合: 生 HTML のリンク（%s）に ownLinkTarget が文字列を返した"+
					"（見えているリンク先と違う文字列をコピーする。FR-063, NFR-030）", spec.Origin), i)
			case spec.Origin == originGFM && link.Target == nil:
				set.add("目印の照合: GFM のリンクで ownLinkTarget が null を返した", i)
			case spec.Origin == originGFM && *link.Target != spec.Target:
				set.add(fmt.Sprintf("目印の照合: GFM のリンクで ownLinkTarget が %q を返した（書かれたとおりの %q を期待。FR-063）",
					*link.Target, spec.Target), i)
			}
		}
	}

	// 図のブロック。
	if len(got.Blocks) != len(layout.Blocks) {
		failures = append(failures,
			fmt.Sprintf("目印の照合: 図のブロックが %d 個（一覧は %d 個。testdata/smoke.md と検査の食い違い）",
				len(got.Blocks), len(layout.Blocks)))
	} else {
		for i, block := range got.Blocks {
			checkBlock(&set, layout.Blocks[i], block, i)
		}
	}

	return append(failures, set.lines()...)
}

// checkLayout は一覧そのものが、種類ごとに 4 種類の由来をすべて含むかを見る
// （UT-815 のケース 9）。**欠けると、その由来に対する照合が一度も行われない。**
func checkLayout(layout refLayout) []string {
	links := make([]string, len(layout.Links))
	for i, link := range layout.Links {
		links[i] = link.Origin
	}

	blocks := make([]string, len(layout.Blocks))
	for i, block := range layout.Blocks {
		blocks[i] = block.Origin
	}

	var failures []string

	for _, kind := range []struct {
		name    string
		origins []string
	}{
		{"チェックボックス", layout.Checkboxes},
		{"表", layout.Tables},
		{"セル", layout.Cells},
		{"リンク", links},
		{"図のブロック", blocks},
	} {
		for _, origin := range allOrigins {
			if !containsString(kind.origins, origin) {
				failures = append(failures,
					fmt.Sprintf("目印の照合: %sの一覧に由来 %s が無い（照合が一度も試されない）", kind.name, origin))
			}
		}
	}

	return failures
}

// checkOwn は isOwnRef の結果を由来に照らして見る。
func checkOwn(set *failureSet, name, kind, origin string, own map[string]bool, index int) {
	for _, k := range refKinds {
		switch {
		case origin != originGFM && own[k]:
			set.add(fmt.Sprintf("目印の照合: 生 HTML の%s（%s）で isOwnRef(%s) が真（NFR-030）", name, origin, k), index)
		case origin == originGFM && k == kind && !own[k]:
			set.add(fmt.Sprintf("目印の照合: GFM の%sで isOwnRef(%s) が偽", name, k), index)
		case origin == originGFM && k != kind && own[k]:
			set.add(fmt.Sprintf("目印の照合: GFM の%sで種類の違う isOwnRef(%s) が真（種類を見ていない）", name, k), index)
		}
	}
}

// checkBlock は図のブロック 1 つを見る。
func checkBlock(set *failureSet, spec blockSpec, block blockElement, index int) {
	checkOwn(set, "図のブロック", spec.Kind, spec.Origin, block.Own, index)

	if spec.Origin != originGFM {
		if block.Source != nil {
			set.add(fmt.Sprintf("目印の照合: 生 HTML のブロック（%s）に ownSource が文字列を返した"+
				"（コピーボタンが見えている内容と違う文字列をコピーする。FR-061, NFR-030, BUG-014）", spec.Origin), index)
		}

		if block.Rendered {
			reason := "Go 側を経ていない図が描かれた。NFR-030, BUG-014"
			if spec.Kind == blockPlantUML {
				reason = "取り込み指令の検査を迂回して描画へ回った。MD-084, IMP-119, BUG-014"
			}

			set.add(fmt.Sprintf("目印の照合: 生 HTML で偽装したブロック（%s）に図の器ができた（%s）", spec.Origin, reason), index)
		}

		return
	}

	switch {
	case block.Source == nil:
		set.add("目印の照合: GFM の図のブロックで ownSource が null を返した", index)
	case *block.Source != block.DataSource:
		set.add("目印の照合: GFM の図のブロックで ownSource が data-source と違う文字列を返した", index)
	}

	if !block.Rendered {
		set.add("目印の照合: GFM の図のブロックに図の器ができていない", index)
	}
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}

	return false
}

// gfmBlocks は図の描画結果のうち、一覧で GFM の由来のものだけを返す。
//
// **既存の検査（Mermaid 7 種・PlantUML 2 種）を、偽装したブロックで数え違えない
// ためにある。** 偽装したブロックも data-mermaid / data-plantuml を持つため、
// ページはそれらも同じ一覧に入れて返す。
func gfmBlocks(layout refLayout, blocks []diagramBlock) []diagramBlock {
	var out []diagramBlock

	for _, block := range blocks {
		if block.Block >= 0 && block.Block < len(layout.Blocks) && layout.Blocks[block.Block].Origin == originGFM {
			out = append(out, block)
		}
	}

	return out
}
