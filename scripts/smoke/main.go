// smoke は同梱した資産（Mermaid / KaTeX）が実際に描画できることを確かめる
// （BR-054, E2E-109）。
//
//	go run ./scripts/smoke
//	go run ./scripts/smoke -browser "C:\Program Files\Google\Chrome\Application\chrome.exe"
//
// **これは「表示が正しいか」の検査ではない。** BR-043 により、リリースのたびに
// Mermaid / KaTeX が最新安定版へ自動更新される。そのとき破壊的変更が入っていて
// も、Go 側のテスト（`go test ./...`）は資産に触れないため何も起きない。
// この隙間を埋めるのが本スクリプトであり、問うているのは 1 つだけである
// ——「更新した資産を、今のフロントエンドのコードで描けるか」。
//
// 進め方は次のとおり。
//
//  1. testdata/smoke.md を **本番の renderer** で HTML にする
//  2. frontend/ を 127.0.0.1 で配り、そこへ本文を差し込んだページを足す
//  3. ヘッドレスブラウザでそのページを開く
//  4. ページが **本番の lazy.js** で描画し、結果を POST で返す
//  5. 返ってきた内容を検査する
//
// 60 秒で打ち切る（BR-054）。無限ループやハングは「失敗」ではなく「終わらない」
// という形で現れるため、待ち時間そのものを合否に含める。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// mermaidKinds は BR-054 が挙げる 7 種類。**減らさない。**
//
// 判定は data-source の 1 行目の前方一致で行う。stateDiagram-v2 のように
// 版数が付く記法があるため、完全一致にはしない。
var mermaidKinds = []string{
	"flowchart",
	"sequenceDiagram",
	"classDiagram",
	"stateDiagram",
	"erDiagram",
	"gantt",
	"pie",
}

// plantUMLKinds は BR-054 が挙げる 2 種類。**両方を見る。**
//
// Graphviz を要する図（class）と要さない図（sequence）を分けているのは、
// viz-global.js の読み込みに失敗しても **要さない図だけは描けてしまう**
// ためである（IMP-233 の 4）。片方だけを見ると、この壊れ方を見落とす。
//
// 判定は data-source の 1 行目の前方一致で行う。検証用文書は図に名前を
// 付けており（@startuml sequence）、種別をそこで見分ける。
var plantUMLKinds = []string{
	"@startuml sequence",
	"@startuml class",
}

// report はページから返る結果（harness.js と対になる）。
type report struct {
	Mermaid   []diagramBlock `json:"mermaid"`
	PlantUML  []diagramBlock `json:"plantuml"`
	Math      mathResult     `json:"math"`
	Errors    []string       `json:"errors"`
	Console   []string       `json:"console"`
	ElapsedMS int            `json:"elapsedMs"`
	UserAgent string         `json:"userAgent"`
	Fatal     string         `json:"fatal"`
}

type diagramBlock struct {
	Index  int    `json:"index"`
	Head   string `json:"head"`
	SVG    int    `json:"svg"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Error  string `json:"error"`
}

type mathResult struct {
	Total  int      `json:"total"`
	KaTeX  int      `json:"katex"`
	Failed []string `json:"failed"`
}

// safeBuffer はブラウザの標準エラーを受ける。別のゴルーチンが書くため保護する。
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "smoke:", err)
		os.Exit(1)
	}
}

func run() error {
	docPath := flag.String("doc", filepath.Join("testdata", "smoke.md"), "検証用の Markdown")
	frontendDir := flag.String("frontend", "frontend", "配信するフロントエンドのディレクトリ")
	browserPath := flag.String("browser", "", "使用するブラウザ（未指定なら自動で探す）")
	timeout := flag.Duration("timeout", 60*time.Second, "打ち切りまでの時間（BR-054）")
	flag.Parse()

	source, err := os.ReadFile(*docPath)
	if err != nil {
		return fmt.Errorf("検証用文書を読めない: %w", err)
	}

	rendered, err := renderer.New().Render(source, filepath.Dir(*docPath))
	if err != nil {
		return fmt.Errorf("検証用文書を変換できない: %w", err)
	}

	// **文書が検証の役に立つ形かをここで確かめる。** 誤って Mermaid や数式を
	// 落とした文書を渡すと、以降の検査はすべて 0 件で通ってしまう（UT-033）。
	if !rendered.NeedsMermaid || !rendered.NeedsKaTeX {
		return fmt.Errorf("%s に Mermaid（%t）と数式（%t）の両方が必要", *docPath, rendered.NeedsMermaid, rendered.NeedsKaTeX)
	}

	printVersions(*frontendDir)

	srv, err := newServer(*frontendDir, rendered.HTML)
	if err != nil {
		return err
	}

	url, err := srv.start()
	if err != nil {
		return err
	}

	browser, err := findBrowser(*browserPath)
	if err != nil {
		return err
	}

	fmt.Printf("browser : %s\n", browser)
	fmt.Printf("page    : %s\n\n", url)

	got, err := collect(srv, browser, url, *timeout)
	if err != nil {
		return err
	}

	failures := check(rendered, got)
	printResult(got, failures)

	if len(failures) > 0 {
		return fmt.Errorf("描画スモークテストに失敗した（%d 件）", len(failures))
	}

	return nil
}

// collect はブラウザを起動し、ページからの結果を待つ。
func collect(srv *server, browser, url string, timeout time.Duration) (report, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var stderr safeBuffer

	cmd, cleanup, err := launch(ctx, browser, url, &stderr)
	if err != nil {
		return report{}, err
	}
	defer cleanup()

	select {
	case got := <-srv.result:
		// 結果は届いた。ブラウザはもう要らない。
		cancel()
		cmd.Wait() //nolint:errcheck // 取り消しによる終了なので状態は見ない

		return got, nil

	case <-ctx.Done():
		cmd.Wait() //nolint:errcheck

		return report{}, fmt.Errorf("%s 以内に結果が返らなかった（BR-054）\n--- ブラウザの出力 ---\n%s", timeout, stderr.String())
	}
}

// check は結果を検査し、失敗の一覧を返す。**空なら合格。**
func check(rendered renderer.Result, got report) []string {
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

	failures = append(failures, checkMermaid(got)...)
	failures = append(failures, checkPlantUML(got)...)

	return append(failures, checkMath(rendered, got)...)
}

func checkMermaid(got report) []string {
	var failures []string

	if len(got.Mermaid) != len(mermaidKinds) {
		failures = append(failures, fmt.Sprintf("Mermaid のブロックが %d 件（%d 件を期待）", len(got.Mermaid), len(mermaidKinds)))
	}

	for _, kind := range mermaidKinds {
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

func findBlock(blocks []diagramBlock, kind string) (diagramBlock, bool) {
	for _, block := range blocks {
		if strings.HasPrefix(block.Head, kind) {
			return block, true
		}
	}

	return diagramBlock{}, false
}

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

func printResult(got report, failures []string) {
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

	fmt.Printf("  katex    %d / %d rendered\n\n", got.Math.KaTeX, got.Math.Total)

	if len(failures) == 0 {
		fmt.Println("OK")

		return
	}

	fmt.Println("FAILED")
	for _, failure := range failures {
		fmt.Println("  - " + failure)
	}
}
