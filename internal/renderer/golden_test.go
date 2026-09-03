package renderer

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
)

// update はゴールデンファイルの再生成を指示する（IMP-041）。
//
//	go test ./internal/renderer -update
//
// **再生成したら差分を必ず読む**（UT-039）。中身を確かめない再生成は、
// 誤った出力をそのまま期待値として固定する。
var update = flag.Bool("update", false, "ゴールデンファイルを再生成する")

// testdata はリポジトリ直下に置く（IMP-011, UT-018）。
const (
	showcasePath = "../../testdata/showcase.md"
	goldenPath   = "../../testdata/showcase.golden.html"
)

// goldenBaseDir はゴールデン生成時の基準ディレクトリ（AR-042）。
//
// 実際の testdata の位置を使うと、ローカル画像の URL に実行環境の絶対パスが
// 入り、ゴールデンが機械ごとに変わる。固定値を使って再現可能にする。
// 相対パスの解決そのものは UT-210 が検証している。
const goldenBaseDir = "/showcase"

// TestRender_Golden は showcase.md の変換結果を固定する
// （UT-214。根拠: MD-002, BR-053 / IMP-041）。
//
// 個別の記法は他のテストが見ている。ここで守りたいのは、**それらを 1 つの
// 文書に混ぜたときに意図しない描画変化が起きていないこと**である。
func TestRender_Golden(t *testing.T) {
	source := readShowcase(t)

	res, err := New().Render(source, goldenBaseDir)
	if err != nil {
		t.Fatalf("Render がエラーを返した: %v", err)
	}
	got := []byte(res.HTML)

	if *update {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("ゴールデンを書けない: %v", err)
		}
		t.Logf("ゴールデンを再生成した: %s（差分を必ず読むこと。UT-039）", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("ゴールデンを読めない（-update で生成する）: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("変換結果がゴールデンと一致しない。差分を確認し、"+
			"意図した変化なら -update で更新する（UT-039）\n%s", firstDiff(want, got))
	}
}

// TestRender_GoldenExpectations は showcase.md が網羅を保っていることを見る。
//
// ゴールデンは「変わっていないこと」しか言わない。**中身が薄くなっても
// 気づけない**ため、要となる性質だけを別に主張しておく。
func TestRender_GoldenExpectations(t *testing.T) {
	res, err := New().Render(readShowcase(t), goldenBaseDir)
	if err != nil {
		t.Fatalf("Render がエラーを返した: %v", err)
	}

	if !res.NeedsMermaid {
		t.Error("showcase.md に Mermaid が含まれていない（MD-080）")
	}
	if !res.NeedsKaTeX {
		t.Error("showcase.md に数式が含まれていない（MD-060）")
	}
	if !res.NeedsPlantUML {
		t.Error("showcase.md に PlantUML が含まれていない（MD-083）")
	}
	if len(res.Headings) < 10 {
		t.Errorf("見出しが %d 件しかない（MD-020, FR-040）", len(res.Headings))
	}

	// showcase.md は意図的に危険な生 HTML を含む。サニタイズ（IMP-116）が
	// 文書全体に効いていることを、ここでも直接確かめる。
	for _, ng := range []string{"<script", "onclick", "style=", "attacker-defined", "alert(1)"} {
		if strings.Contains(res.HTML, ng) {
			t.Errorf("出力に %q が残っている（MD-072, NFR-030）", ng)
		}
	}

	// 主要な記法が出力に現れていること。showcase.md から節が消えたら気づける。
	for _, want := range []string{
		"markdown-alert-note", "markdown-alert-caution",
		"math-inline", "math-block",
		"data-mermaid", "mermaid-source",
		"data-plantuml", "plantuml-source",
		"code-block", "chroma",
		"footnote-ref", "footnotes",
		"<table>", "<del>", "<kbd>", "<details>",
		`type="checkbox"`,
		"/__local/",
	} {
		if !strings.Contains(res.HTML, want) {
			t.Errorf("出力に %q がない。showcase.md の網羅が欠けている（BR-053）", want)
		}
	}
}

// readShowcase は検証用 Markdown を読む。
func readShowcase(t *testing.T) []byte {
	t.Helper()

	source, err := os.ReadFile(showcasePath)
	if err != nil {
		t.Fatalf("showcase.md を読めない: %v", err)
	}
	if bytes.ContainsRune(source, '\r') {
		t.Fatal("showcase.md に CR が含まれている。.gitattributes による LF 統一を確認すること")
	}
	return source
}

// firstDiff は最初に食い違う行を、行番号つきで返す。
//
// 出力全体を貼ると読めないため、最初の 1 か所に絞る。
func firstDiff(want, got []byte) string {
	w := strings.Split(string(want), "\n")
	g := strings.Split(string(got), "\n")

	for i := 0; i < len(w) || i < len(g); i++ {
		lw, lg := line(w, i), line(g, i)
		if lw != lg {
			return fmt.Sprintf("%d 行目\n  want: %s\n   got: %s", i+1, lw, lg)
		}
	}
	return "（行単位の差分なし。末尾の改行を確認すること）"
}

func line(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "（行なし）"
}
