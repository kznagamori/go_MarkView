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
//  1. testdata/smoke.md を **本番の renderer** で HTML にする（**実行のたびに作った
//     目印の鍵を渡す**。IMP-102, IMP-110）
//  2. frontend/ を 127.0.0.1 で配り、そこへ本文を差し込んだページを足す
//  3. ヘッドレスブラウザでそのページを開く
//  4. ページが **本番の lazy.js** で描画し、**本番の tablesort.js と refs.js** を
//     呼んで、事実を POST で返す
//  5. 返ってきた内容を検査する（判定は Go 側。ids.go / tablesort.go / refs.go）
//
// v1.1.0 で、文書の id の名前空間（AR-053。UT-813）・表の並べ替えの比較
// （FR-130。UT-814）・目印の照合（NFR-030, BUG-014。UT-815）を足した。
// **どれも assets 部（testdata/smoke.md）でだけ判定する。**
//
// 60 秒で打ち切る（BR-054）。無限ループやハングは「失敗」ではなく「終わらない」
// という形で現れるため、待ち時間そのものを合否に含める。
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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

// mermaidLabelKind はラベルに id を書いた図（AR-053, IMP-231, UT-813）。
//
// **描けていることも見る。** 描けなければラベルの id が SVG の中に現れず、
// 文書の id の検査は何も見ないまま通る。flowchart と区別するため `graph` で書く。
const mermaidLabelKind = "graph LR"

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

	// v1.1.0 で足した検査（BR-054, E2E-109 の 8〜10）。
	IDs  idReport   `json:"ids"`
	Sort sortReport `json:"sort"`
	Refs refReport  `json:"refs"`
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
	Block  int    `json:"block"` // 図のブロックの一覧（refReport.Blocks）の何番目か。無ければ -1
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

	// **目印の鍵は、アプリと同じく実行のたびに作って渡す**（IMP-102, IMP-110）。
	// 検証用文書の偽の目印は、この鍵を知らずに書いたものとして照合される。
	refKey, err := newRefKey()
	if err != nil {
		return err
	}

	passes := []pass{
		{name: "assets", doc: *docPath, verify: verifyAssetsDoc, check: checkAssets},
		{name: "limits", doc: *limitsPath, verify: verifyLimitsDoc, check: checkLimits},
	}

	total := 0

	for _, p := range passes {
		count, err := runPass(p, *frontendDir, browser, refKey, *timeout)
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

// newRefKey は目印の鍵を作る。crypto/rand の 8 バイトを 16 進 16 文字にする（IMP-102）。
func newRefKey() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("目印の鍵を作れない: %w", err)
	}

	return hex.EncodeToString(b[:]), nil
}

// pageConfig はページが最初に受け取る値（/_config）。**期待値は渡さない。**
type pageConfig struct {
	RefKey    string          `json:"refKey"`
	SortCases []sortCaseInput `json:"sortCases"`
}

// runPass は 1 部ぶんを走らせ、失敗の件数を返す。
func runPass(p pass, frontendDir, browser, refKey string, timeout time.Duration) (int, error) {
	fmt.Printf("\n== %s : %s\n", p.name, p.doc)

	source, err := os.ReadFile(p.doc)
	if err != nil {
		return 0, fmt.Errorf("検証用文書を読めない: %w", err)
	}

	rendered, err := renderer.New().Render(source, filepath.Dir(p.doc), refKey)
	if err != nil {
		return 0, fmt.Errorf("検証用文書を変換できない: %w", err)
	}

	// **文書が検証の役に立つ形かをここで確かめる。** 誤って対象を落とした
	// 文書を渡すと、以降の検査はすべて 0 件で通ってしまう（UT-033）。
	if err := p.verify(rendered, p.doc); err != nil {
		return 0, err
	}

	config, err := json.Marshal(pageConfig{RefKey: refKey, SortCases: sortCaseInputs()})
	if err != nil {
		return 0, fmt.Errorf("ページへ渡す値を組み立てられない: %w", err)
	}

	srv, err := newServer(frontendDir, rendered.HTML, config)
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
	printResult(got, failures, p.name == "assets")

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
