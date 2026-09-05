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

// 画像の検査に使う値（FR-022, IMP-226, DSP-123）。**testdata/smoke.md と
// 一致させる。** 片方だけを変えると、検査は 0 件で通るのではなく
// 「見つからない」で落ちる（そう作ってある）。
const (
	okImageAlt      = "読める画像"
	brokenImageAlt  = "この画像は読み込みに失敗します"
	smokeImageCount = 3 // 読める 1 枚 + 失敗する 2 枚
)

// plantuml-limits.md のブロック名（E2E-012, BUG-010）。**testdata と一致させる。**
//
// **名前で見分ける。** どのブロックも先頭行が `@startuml` のままでは区別できず、
// 「6 節が理由表示になっているか」を機械で確かめられない。
var (
	limitsDrawn = []string{
		"@startuml normal",
		"@startuml syntaxerror",
		"@startuml toolarge",
		"@startsalt",
		"@startditaa",
	}
	limitsRejected = []string{
		"@startuml include",
		"@startuml includeurl",
		"@startuml stdlib",
	}
)

// report はページから返る結果（harness.js と対になる）。
type report struct {
	Mermaid          []diagramBlock `json:"mermaid"`
	PlantUML         []diagramBlock `json:"plantuml"`
	PlantUMLRejected []diagramBlock `json:"plantumlRejected"`
	Math             mathResult     `json:"math"`
	Images           imageReport    `json:"images"`
	Errors           []string       `json:"errors"`
	Console          []string       `json:"console"`
	ElapsedMS        int            `json:"elapsedMs"`
	UserAgent        string         `json:"userAgent"`
	Fatal            string         `json:"fatal"`
}

// imageReport は読み込みに失敗した画像の扱い（IMP-226, DSP-123）。
type imageReport struct {
	Imgs     []imageInfo  `json:"imgs"`
	Broken   []brokenInfo `json:"broken"`
	Legacy   int          `json:"legacy"` // img.is-broken の数。4.32.0 より前のフック
	Imported bool         `json:"imported"`
	Error    string       `json:"error"`
}

type imageInfo struct {
	Alt          string `json:"alt"`
	ClassName    string `json:"className"`
	Complete     bool   `json:"complete"`
	NaturalWidth int    `json:"naturalWidth"`
}

type brokenInfo struct {
	TagName   string `json:"tagName"`
	ClassName string `json:"className"`
	Text      string `json:"text"`
}

// pass は 1 部ぶんの検証。**文書ごとに問いが違うため、検査も分ける。**
//
//	assets  同梱資産が描けるか（BR-054）
//	limits  描けないものが理由とともに示されるか（E2E-012, MD-083, MD-084）
type pass struct {
	name   string
	doc    string
	verify func(renderer.Result, string) error
	check  func(renderer.Result, report) []string
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
	docPath := flag.String("doc", filepath.Join("testdata", "smoke.md"), "同梱資産の検証用 Markdown")
	limitsPath := flag.String("limits", filepath.Join("testdata", "e2e", "plantuml-limits.md"),
		"PlantUML の制限の検証用 Markdown（E2E-012）")
	frontendDir := flag.String("frontend", "frontend", "配信するフロントエンドのディレクトリ")
	browserPath := flag.String("browser", "", "使用するブラウザ（未指定なら自動で探す）")
	timeout := flag.Duration("timeout", 60*time.Second, "1 部あたりの打ち切り時間（BR-054）")
	flag.Parse()

	printVersions(*frontendDir)

	browser, err := findBrowser(*browserPath)
	if err != nil {
		return err
	}

	fmt.Printf("browser : %s\n", browser)

	passes := []pass{
		{name: "assets", doc: *docPath, verify: verifyAssetsDoc, check: checkAssets},
		{name: "limits", doc: *limitsPath, verify: verifyLimitsDoc, check: checkLimits},
	}

	total := 0

	for _, p := range passes {
		count, err := runPass(p, *frontendDir, browser, *timeout)
		if err != nil {
			return err
		}

		total += count
	}

	if total > 0 {
		return fmt.Errorf("描画スモークテストに失敗した（%d 件）", total)
	}

	return nil
}

// runPass は 1 部ぶんを走らせ、失敗の件数を返す。
func runPass(p pass, frontendDir, browser string, timeout time.Duration) (int, error) {
	fmt.Printf("\n== %s : %s\n", p.name, p.doc)

	source, err := os.ReadFile(p.doc)
	if err != nil {
		return 0, fmt.Errorf("検証用文書を読めない: %w", err)
	}

	rendered, err := renderer.New().Render(source, filepath.Dir(p.doc))
	if err != nil {
		return 0, fmt.Errorf("検証用文書を変換できない: %w", err)
	}

	// **文書が検証の役に立つ形かをここで確かめる。** 誤って対象を落とした
	// 文書を渡すと、以降の検査はすべて 0 件で通ってしまう（UT-033）。
	if err := p.verify(rendered, p.doc); err != nil {
		return 0, err
	}

	srv, err := newServer(frontendDir, rendered.HTML)
	if err != nil {
		return 0, err
	}

	url, err := srv.start()
	if err != nil {
		return 0, err
	}

	fmt.Printf("page    : %s\n\n", url)

	got, err := collect(srv, browser, url, timeout)
	if err != nil {
		return 0, err
	}

	failures := p.check(rendered, got)
	printResult(got, failures)

	return len(failures), nil
}

// verifyAssetsDoc は同梱資産の検証用文書が要件を満たすかを見る。
func verifyAssetsDoc(rendered renderer.Result, path string) error {
	if !rendered.NeedsMermaid || !rendered.NeedsKaTeX {
		return fmt.Errorf("%s に Mermaid（%t）と数式（%t）の両方が必要", path, rendered.NeedsMermaid, rendered.NeedsKaTeX)
	}

	if got := countImages(rendered.HTML); got != smokeImageCount {
		return fmt.Errorf("%s の画像が %d 枚（%d 枚を期待。IMP-226 の検査に要る）", path, got, smokeImageCount)
	}

	if !strings.Contains(rendered.HTML, `alt="`+brokenImageAlt+`"`) {
		return fmt.Errorf("%s に代替テキスト %q の画像が見つからない（IMP-226 の検査に要る）", path, brokenImageAlt)
	}

	return nil
}

// verifyLimitsDoc は PlantUML の制限の検証用文書が要件を満たすかを見る。
func verifyLimitsDoc(rendered renderer.Result, path string) error {
	if !rendered.NeedsPlantUML {
		return fmt.Errorf("%s に PlantUML のブロックが必要", path)
	}

	if got := strings.Count(rendered.HTML, `data-puml-error=`); got != len(limitsRejected) {
		return fmt.Errorf("%s の拒否ブロックが %d 件（%d 件を期待。MD-084, IMP-119）", path, got, len(limitsRejected))
	}

	return nil
}

func countImages(html string) int {
	return strings.Count(html, "<img ")
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
func checkAssets(rendered renderer.Result, got report) []string {
	failures := checkCommon(got)
	failures = append(failures, checkMermaid(got)...)
	failures = append(failures, checkPlantUML(got)...)
	failures = append(failures, checkMath(rendered, got)...)

	return append(failures, checkBrokenImages(countImages(rendered.HTML), got)...)
}

// checkLimits は描けないものが理由とともに示されるかを検査する
// （MD-083, MD-084, DSP-272, BUG-010）。
func checkLimits(_ renderer.Result, got report) []string {
	return append(checkCommon(got), checkPlantUMLLimits(got)...)
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

	for _, block := range got.PlantUMLRejected {
		fmt.Printf("  rejected %-22s svg=%d  %s\n", block.Head, block.SVG, block.Error)
	}

	if got.Math.Total > 0 {
		fmt.Printf("  katex    %d / %d rendered\n", got.Math.KaTeX, got.Math.Total)
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
