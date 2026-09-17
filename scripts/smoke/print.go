package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 本ファイルは、検査した資産の版と結果の出力を持つ（BR-043, BR-054）。CI のログで、
// どの版で何を確かめたかを後から追えるようにする。判定は check.go にあり、ここでは判定しない。
//
// main.go が 400 行の目安（IMP-011）を超えたため分けた。

// printVersions は検査対象の資産の版を出す。CI のログで「どの版で通ったか」を
// 後から追えるようにするため（BR-043）。
func printVersions(frontendDir string) {
	data, err := os.ReadFile(filepath.Join(frontendDir, "vendor", "vendor.json"))
	if err != nil {
		fmt.Printf("assets  : (vendor.json を読めない: %v)\n", err)

		return
	}

	var entries []struct {
		Name      string `json:"name"`
		Version   string `json:"version"`
		BundledIn string `json:"bundledIn"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Printf("assets  : (vendor.json を解析できない: %v)\n", err)

		return
	}

	// **最上位の資産だけを出す**（BR-042, IMP-181 の Bundled と同じ絞り方）。
	// 同梱物の中に含まれるもの（Viz.js / Graphviz / Expat）は版を持たないことが
	// あり、"Graphviz " のように名前だけが並んでログが読みにくくなる。
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.BundledIn != "" {
			continue
		}

		parts = append(parts, entry.Name+" "+entry.Version)
	}

	fmt.Printf("assets  : %s\n", strings.Join(parts, ", "))
}

// printV110 は v1.1.0 で足した検査の事実を短く出す（BR-054 の 8〜10）。
func printV110(got report) {
	if len(got.IDs.Elements) > 0 || len(got.IDs.Headings) > 0 {
		svg := 0
		for _, el := range got.IDs.Elements {
			if el.InSVG && el.Namespace == namespaceSVG {
				svg++
			}
		}

		fmt.Printf("  ids      elements=%d (svg=%d) headings=%d plantumlDone=%t\n",
			len(got.IDs.Elements), svg, len(got.IDs.Headings), got.IDs.PlantUMLDone)
	}

	if got.Sort.Imported || got.Sort.Error != "" {
		fmt.Printf("  sort     imported=%t results=%d %s\n", got.Sort.Imported, len(got.Sort.Results), got.Sort.Error)
	}

	if got.Refs.Imported || got.Refs.Error != "" {
		fmt.Printf("  refs     imported=%t checkboxes=%d tables=%d cells=%d links=%d blocks=%d %s\n",
			got.Refs.Imported, len(got.Refs.Checkboxes), len(got.Refs.Tables), len(got.Refs.Cells),
			len(got.Refs.Links), len(got.Refs.Blocks), got.Refs.Error)
	}
}

func printResult(got report, failures []string, v110 bool) {
	fmt.Printf("engine  : %s\n", got.UserAgent)
	fmt.Printf("elapsed : %d ms\n\n", got.ElapsedMS)

	for _, block := range got.Mermaid {
		fmt.Printf("  mermaid  %-18s svg=%d  %d×%d\n", block.Head, block.SVG, block.Width, block.Height)
	}

	for _, block := range got.PlantUML {
		// **理由まで出す。** plantUMLKinds 以外の図は合否に効かないため、
		// 出さないと「描けなかったが理由が分からない」行になる。
		fmt.Printf("  plantuml %-22s svg=%d  %d×%d  %s\n",
			block.Head, block.SVG, block.Width, block.Height, block.Error)
	}

	for _, block := range got.PlantUMLRejected {
		fmt.Printf("  rejected %-22s svg=%d  %s\n", block.Head, block.SVG, block.Error)
	}

	if got.Math.Total > 0 {
		fmt.Printf("  katex    %d / %d rendered\n", got.Math.KaTeX, got.Math.Total)
	}

	if v110 {
		printV110(got)
	}

	if len(got.Images.Imgs) > 0 || len(got.Images.Broken) > 0 {
		fmt.Printf("  images   img=%d broken=%d legacy=%d\n", len(got.Images.Imgs), len(got.Images.Broken), got.Images.Legacy)

		for _, el := range got.Images.Broken {
			fmt.Printf("           broken <%s> %q\n", el.TagName, el.Text)
		}
	}

	fmt.Println()

	if len(failures) == 0 {
		fmt.Println("OK")

		return
	}

	fmt.Println("FAILED")
	for _, failure := range failures {
		fmt.Println("  - " + failure)
	}
}
