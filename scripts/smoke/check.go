package main

import (
	"fmt"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// 本ファイルは、ページが返した事実の判定を持つ（BR-054, E2E-109）。**空なら合格**とする。
//
// **判定は純関数にする**（UT-038）。ブラウザを起動する部分と混ぜると、判定そのものを
// 単体テスト（UT-811, UT-812。main_test.go）で試せなくなる。文書の id・表の並べ替え・
// 目印の照合の判定は ids.go / tablesort.go / refs.go にある。
//
// main.go が 400 行の目安（IMP-011）を超えたため分けた。

// checkCommon はどちらの部でも見るものを検査する。**空なら合格。**
func checkCommon(got report) []string {
	var failures []string

	if got.Fatal != "" {
		failures = append(failures, "描画が例外で止まった: "+got.Fatal)
	}

	for _, message := range got.Errors {
		failures = append(failures, "JavaScript のエラー: "+message)
	}

	// **console.error も失敗として扱う。** Mermaid は図の解析に失敗しても
	// 例外を投げずにここへ流すことがあり、見逃すと「SVG は出たが中身は
	// エラー表示」という状態を通してしまう。
	for _, message := range got.Console {
		failures = append(failures, "console.error: "+message)
	}

	return failures
}

// checkAssets は同梱資産が描けるかを検査する（BR-054, E2E-109）。
//
// **図の検査は GFM の由来のブロックだけで行う。** 検証用文書は生 HTML で偽装した
// 図のブロックも含み（BUG-014）、それらも data-mermaid / data-plantuml を持つ。
// 数えると 7 種類の検査が数え違える。偽装したブロックは目印の照合（UT-815）が見る。
func checkAssets(rendered renderer.Result, got report) []string {
	drawn := got
	drawn.Mermaid = gfmBlocks(smokeRefLayout, got.Mermaid)
	drawn.PlantUML = gfmBlocks(smokeRefLayout, got.PlantUML)

	failures := checkCommon(got)
	failures = append(failures, checkMermaid(drawn)...)
	failures = append(failures, checkPlantUML(drawn)...)
	failures = append(failures, checkMath(rendered, got)...)
	failures = append(failures, checkBrokenImages(countImages(rendered.HTML), got)...)
	failures = append(failures, checkIDNamespace(got.IDs)...)
	failures = append(failures, checkTableSort(got.Sort)...)

	return append(failures, checkRefMatching(smokeRefLayout, got.Refs)...)
}

// checkLimits は描けないものが理由とともに示されるかを検査する
// （MD-083, MD-084, DSP-272, BUG-010）。
func checkLimits(_ renderer.Result, got report) []string {
	return append(checkCommon(got), checkPlantUMLLimits(got)...)
}

func checkMermaid(got report) []string {
	var failures []string

	kinds := append(append([]string{}, mermaidKinds...), mermaidLabelKind)

	if len(got.Mermaid) != len(kinds) {
		failures = append(failures, fmt.Sprintf("Mermaid のブロックが %d 件（%d 件を期待）", len(got.Mermaid), len(kinds)))
	}

	for _, kind := range kinds {
		block, ok := findBlock(got.Mermaid, kind)

		switch {
		case !ok:
			failures = append(failures, kind+": 図が文書に見つからない")
		case block.Error != "":
			failures = append(failures, kind+": "+block.Error)
		case block.SVG != 1:
			failures = append(failures, fmt.Sprintf("%s: SVG が %d 個（1 個を期待）", kind, block.SVG))
		case block.Width <= 0 || block.Height <= 0:
			// 大きさのない SVG は「描けた」とは言えない。
			failures = append(failures, fmt.Sprintf("%s: SVG の寸法が %d×%d", kind, block.Width, block.Height))
		}
	}

	return failures
}

// checkPlantUML は PlantUML の描画結果を検査する（BR-054, E2E-109）。
//
// 検査の内容は Mermaid と同じである。**SVG の有無だけでは足りない**——
// 失敗しても大きさのない SVG が残ることがあるため、寸法まで見る。
func checkPlantUML(got report) []string {
	var failures []string

	if len(got.PlantUML) != len(plantUMLKinds) {
		failures = append(failures, fmt.Sprintf("PlantUML のブロックが %d 件（%d 件を期待）", len(got.PlantUML), len(plantUMLKinds)))
	}

	for _, kind := range plantUMLKinds {
		block, ok := findBlock(got.PlantUML, kind)

		switch {
		case !ok:
			failures = append(failures, kind+": 図が文書に見つからない")
		case block.Error != "":
			failures = append(failures, kind+": "+block.Error)
		case block.SVG != 1:
			failures = append(failures, fmt.Sprintf("%s: SVG が %d 個（1 個を期待）", kind, block.SVG))
		case block.Width <= 0 || block.Height <= 0:
			failures = append(failures, fmt.Sprintf("%s: SVG の寸法が %d×%d", kind, block.Width, block.Height))
		}
	}

	return failures
}

func checkMath(rendered renderer.Result, got report) []string {
	var failures []string

	// **Go が出した数と、ブラウザが見つけた数を突き合わせる。** 片側だけを
	// 数えると、変換が数式を落としたときに 0 件どうしで一致してしまう。
	inline := strings.Count(rendered.HTML, `class="math-inline"`)
	block := strings.Count(rendered.HTML, `class="math-block"`)

	if inline == 0 || block == 0 {
		failures = append(failures, fmt.Sprintf("変換結果にインライン数式が %d 件、ブロック数式が %d 件（どちらも 1 件以上を期待）", inline, block))
	}

	if got.Math.Total != inline+block {
		failures = append(failures, fmt.Sprintf("数式の要素が %d 個（変換結果は %d 個）", got.Math.Total, inline+block))
	}

	if got.Math.KaTeX != got.Math.Total {
		failures = append(failures, fmt.Sprintf("KaTeX が描いたのは %d / %d 個", got.Math.KaTeX, got.Math.Total))
	}

	for _, source := range got.Math.Failed {
		failures = append(failures, "KaTeX が解釈できない: "+source)
	}

	return failures
}

// checkBrokenImages は読み込みに失敗した画像の扱いを検査する
// （FR-022, IMP-226, DSP-123, BUG-008）。
//
// **見るのは「枠が出たか」ではなく「代替テキストが本文として読めるか」である。**
// 枠は CSS がこちらで描くためどのエンジンでも出るが、中身は 4.32.0 より前は
// ブラウザ既定に委ねていた。**そこが Linux で空になっていた。**
//
// **エンジン差そのものはここでは見えない。** 本テストは Chromium 系でしか
// 走らない（BR-054, NFR-061）。見ているのは「自前で描いているか」だけであり、
// **「Linux でも同じに見えるか」は手動テスト（E2E-236）が受け持つ。**
func checkBrokenImages(htmlImages int, got report) []string {
	var failures []string

	if !got.Images.Imported {
		return append(failures, "viewer.js の markBrokenImages を読めない: "+got.Images.Error)
	}

	// 読める 1 枚を除いた残りが、失敗する画像である。
	wantBroken := htmlImages - 1

	// **旧いフックが残っていたら、修正が入っていない。**
	if got.Images.Legacy > 0 {
		failures = append(failures,
			fmt.Sprintf("img.is-broken が %d 個残っている（4.32.0 より前のフック。BUG-008）", got.Images.Legacy))
	}

	if len(got.Images.Broken) != wantBroken {
		failures = append(failures,
			fmt.Sprintf(".img-broken が %d 個（%d 個を期待）", len(got.Images.Broken), wantBroken))
	}

	// **残っている img は「読める画像」1 枚だけ**（過検出の検査）。
	// 正常な画像まで置き換えていたら、ここで落ちる。
	readable := 0

	for _, img := range got.Images.Imgs {
		switch {
		case img.Alt != okImageAlt:
			failures = append(failures,
				fmt.Sprintf("img が残っている: alt=%q（失敗した画像は置き換える。IMP-226）", img.Alt))
		case img.NaturalWidth <= 0:
			failures = append(failures,
				fmt.Sprintf("読めるはずの画像が読めていない: alt=%q", img.Alt))
		default:
			readable++
		}
	}

	if readable != 1 {
		failures = append(failures, fmt.Sprintf("読める画像が %d 枚（1 枚を期待）", readable))
	}

	// **代替テキストが本文として読めること。ここが BUG-008 の本体である。**
	found := false

	for _, el := range got.Images.Broken {
		if el.TagName != "SPAN" {
			failures = append(failures,
				fmt.Sprintf(".img-broken が %s 要素（SPAN を期待。IMP-226）", el.TagName))
		}

		if el.Text == brokenImageAlt {
			found = true
		}
	}

	if !found {
		failures = append(failures,
			fmt.Sprintf("代替テキスト %q が本文として読めない（ブラウザ既定に委ねていないか。BUG-008, NFR-061）", brokenImageAlt))
	}

	return failures
}

// checkPlantUMLLimits は「描けないものが理由とともに示されるか」を検査する
// （MD-083, MD-084, DSP-272, BUG-010）。
//
// **判定は「SVG が得られたか」だけで行う**（DSP-272）。図種別ごとの判定を
// 持ち込まない。**ただし 6 節（4096 px 超え）だけは期待値を確定できる**
// ——大きさは検証用データ側で決められるためである。
func checkPlantUMLLimits(got report) []string {
	var failures []string

	if len(got.PlantUML) != len(limitsDrawn) {
		failures = append(failures,
			fmt.Sprintf("描画対象の PlantUML ブロックが %d 件（%d 件を期待）", len(got.PlantUML), len(limitsDrawn)))
	}

	if len(got.PlantUMLRejected) != len(limitsRejected) {
		failures = append(failures,
			fmt.Sprintf("拒んだ PlantUML ブロックが %d 件（%d 件を期待。MD-084）", len(got.PlantUMLRejected), len(limitsRejected)))
	}

	// 1 節: 正常な図。**他の失敗が波及していないことを、ここで見る。**
	if block, ok := findBlock(got.PlantUML, "@startuml normal"); !ok {
		failures = append(failures, "@startuml normal: 図が文書に見つからない")
	} else if block.SVG != 1 || block.Width <= 0 || block.Height <= 0 {
		failures = append(failures,
			fmt.Sprintf("@startuml normal: svg=%d %d×%d（1 個・寸法ありを期待）", block.SVG, block.Width, block.Height))
	}

	// 2 節: 構文エラー。**PlantUML はエラーを「図」で返す**（FR-024, DSP-272）。
	if block, ok := findBlock(got.PlantUML, "@startuml syntaxerror"); !ok {
		failures = append(failures, "@startuml syntaxerror: 図が文書に見つからない")
	} else if block.SVG != 1 {
		failures = append(failures,
			fmt.Sprintf("@startuml syntaxerror: svg=%d（エラー図が 1 個出るのを期待。FR-024）", block.SVG))
	}

	// 6 節: 4096 px 超え。**ここが BUG-010 の本体である。**
	if block, ok := findBlock(got.PlantUML, "@startuml toolarge"); !ok {
		failures = append(failures, "@startuml toolarge: 図が文書に見つからない")
	} else {
		if block.SVG != 0 {
			failures = append(failures,
				"@startuml toolarge: 図が出てしまった（4096 px の制限に達していない。BUG-010）")
		}

		if block.Error == "" {
			failures = append(failures, "@startuml toolarge: 理由が出ていない（DSP-272）")
		}
	}

	// 7・8 節: salt / ditaa。**どちらでもよいが、空にはならない**（DSP-272）。
	// **ここを断定して書かない。** 返し方は同梱資産の版で変わりうるため、
	// 断定すると BR-043 の自動更新のたびに誤って落ちる。
	for _, kind := range []string{"@startsalt", "@startditaa"} {
		block, ok := findBlock(got.PlantUML, kind)
		if !ok {
			failures = append(failures, kind+": ブロックが文書に見つからない")

			continue
		}

		if block.SVG == 0 && block.Error == "" {
			failures = append(failures, kind+": 図も理由も出ていない（空になっている。DSP-272）")
		}
	}

	// 3〜5 節: 取り込み指令。**Go 側が弾いたブロックは、理由だけが出る**（MD-084）。
	for _, kind := range limitsRejected {
		block, ok := findBlock(got.PlantUMLRejected, kind)

		switch {
		case !ok:
			failures = append(failures, kind+": 拒否されたブロックが見つからない（MD-084, IMP-119）")
		case block.SVG != 0:
			failures = append(failures, kind+": 図が出てしまった（描画対象から外れていない。IMP-119）")
		case block.Error == "":
			failures = append(failures, kind+": 理由が出ていない（DSP-272）")
		}
	}

	return failures
}

func findBlock(blocks []diagramBlock, kind string) (diagramBlock, bool) {
	for _, block := range blocks {
		if strings.HasPrefix(block.Head, kind) {
			return block, true
		}
	}

	return diagramBlock{}, false
}
