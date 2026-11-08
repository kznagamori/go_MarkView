package renderer

import (
	"strings"
	"testing"
)

// renderMermaidFile は Mermaid ファイルを変換する。失敗した時点で打ち切る。
func renderMermaidFile(t *testing.T, source string, opts Options) (Result, []element) {
	t.Helper()

	res, err := New().RenderMermaidFile([]byte(source), opts)
	if err != nil {
		t.Fatalf("RenderMermaidFile(%q) がエラーを返した: %v", source, err)
	}
	return res, scanElements(t, res.HTML)
}

// TestRenderMermaidFile は Mermaid ファイルの変換を検証する
// （UT-225。根拠: FR-026, FR-040, MD-082, MD-087, AR-031, FR-146, NFR-013 / IMP-115, IMP-127）。
//
// **Go 側は Mermaid の構文を解かない**（AR-031）。図が 2 つある・MultiMarkdown の文書である、
// などは処理系が構文エラーとして示す（ケース 5）。
func TestRenderMermaidFile(t *testing.T) {
	opts := Options{RefKey: testRefKey}

	// ケース 4: 空白だけのファイル / 空のファイル（境界を先に置く。UT-013）。
	for _, src := range []string{" \n\t\n", ""} {
		t.Run("4 空白だけ "+strings.ReplaceAll(src, "\n", "⏎"), func(t *testing.T) {
			res, _ := renderMermaidFile(t, src, opts)
			if res.HTML != "" || !res.NoDiagrams || res.NeedsMermaid {
				t.Errorf("HTML / NoDiagrams / NeedsMermaid = %q / %v / %v, want 空 / true / false", res.HTML, res.NoDiagrams, res.NeedsMermaid)
			}
		})
	}

	// ケース 1: ファイル全体を 1 つの図とし、Markdown の中の Mermaid のブロックと同じ形にする。
	t.Run("1 基本", func(t *testing.T) {
		const src = "graph TD\nA-->B\n"
		res, els := renderMermaidFile(t, src, opts)
		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "div.code-block", blocks, 1, res.HTML)
		wantAttr(t, "図", blocks[0], "data-lang", "mermaid", res.HTML)
		wantAttr(t, "図", blocks[0], "data-mermaid", "1", res.HTML)
		wantAttr(t, "図", blocks[0], "data-ref", ref("mermaid:0"), res.HTML)
		wantAttr(t, "図", blocks[0], "data-source", src, res.HTML)
		if !res.NeedsMermaid || res.NoDiagrams || len(res.Headings) != 0 {
			t.Errorf("NeedsMermaid / NoDiagrams / Headings = %v / %v / %v, want true / false / 空", res.NeedsMermaid, res.NoDiagrams, res.Headings)
		}
		if res.Headings == nil {
			t.Error("Headings が nil。空スライスを返す規約（UT-203 ケース 5）に反する")
		}
	})

	// ケース 2・3: 空白とタブだけの先頭の行を除く（MD-087 の 2）。字下げされた最初の行は残す。
	for _, tt := range []struct{ name, src, want string }{
		{"2 先頭の空行と空白だけの行", "\n  \n\tgraph TD\n", "\tgraph TD\n"},
		{"3 先頭に空行のあるFrontMatter付き", "\n---\ntitle: t\n---\ngraph TD\n", "---\ntitle: t\n---\ngraph TD\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, els := renderMermaidFile(t, tt.src, opts)
			blocks := withClass(byTag(els, "div"), "code-block")
			wantCount(t, "div.code-block", blocks, 1, res.HTML)
			wantAttr(t, "図", blocks[0], "data-source", tt.want, res.HTML)
		})
	}

	// ケース 5: 図が 2 つ書かれたファイル / MultiMarkdown の文書でも、ブロックは 1 つ（構文を解かない）。
	for _, tt := range []struct{ name, src string }{
		{"5 図が2つ", "graph TD\nA-->B\n\ngraph LR\nC-->D\n"},
		{"5 MultiMarkdown", "# 見出し\n\n```mermaid\ngraph TD\n```\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, els := renderMermaidFile(t, tt.src, opts)
			wantCount(t, "div.code-block", withClass(byTag(els, "div"), "code-block"), 1, res.HTML)
			if n := len(byTag(els, "h1")); n != 0 {
				t.Errorf("Markdown として解いた（h1 が %d 個）\n出力: %s", n, res.HTML)
			}
		})
	}

	// ケース 6: 修飾の目印は付けない / 鍵が空なら data-ref が無い。
	t.Run("6 TextRefs", func(t *testing.T) {
		_, els := renderMermaidFile(t, "graph TD\n", Options{RefKey: testRefKey, TextRefs: true})
		if got := refsOf(els, "text"); len(got) != 0 {
			t.Errorf("text: の目印がある: %v", got)
		}
	})
	t.Run("6 鍵が空", func(t *testing.T) {
		res, els := renderMermaidFile(t, "graph TD\n", Options{})
		if len(withClass(byTag(els, "div"), "code-block")) != 1 {
			t.Fatalf("図のブロックが無い（検査の前提が崩れている）\n出力: %s", res.HTML)
		}
		for _, e := range els {
			wantNoAttr(t, "<"+e.Tag+">", e, "data-ref", res.HTML)
		}
	})

	// ケース 7: 原文の < > & は正しくエスケープされる。
	t.Run("7 エスケープ", func(t *testing.T) {
		const src = "graph TD\nA[\"<b>&amp;\"]-->B\n"
		res, els := renderMermaidFile(t, src, opts)
		blocks := withClass(byTag(els, "div"), "code-block")
		wantCount(t, "div.code-block", blocks, 1, res.HTML)
		wantAttr(t, "図", blocks[0], "data-source", src, res.HTML)
		if n := len(byTag(els, "b")); n != 0 {
			t.Errorf("原文の <b> が要素になった\n出力: %s", res.HTML)
		}
	})

	// ケース 8: UT-213 の異常入力で panic しない。
	for _, tt := range robustInputs() {
		t.Run("8 "+tt.name, func(t *testing.T) {
			withinLimit(t, func() { _, _ = New().RenderMermaidFile([]byte(tt.in), opts) })
		})
	}
}
