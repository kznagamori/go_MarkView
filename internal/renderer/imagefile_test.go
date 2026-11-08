package renderer

import (
	"path/filepath"
	"strings"
	"testing"
)

// renderImageFile は画像ファイルを変換し、div.image-file の中の img を返す。
func renderImageFile(t *testing.T, path, version string) (Result, element) {
	t.Helper()

	res, err := New().RenderImageFile(path, version)
	if err != nil {
		t.Fatalf("RenderImageFile(%q, %q) がエラーを返した: %v", path, version, err)
	}
	els := scanElements(t, res.HTML)
	divs := withClass(byTag(els, "div"), "image-file")
	if len(divs) != 1 {
		t.Fatalf("div.image-file が %d 個, want 1\n出力: %s", len(divs), res.HTML)
	}
	imgs := byTag(els, "img")
	if len(imgs) != 1 {
		t.Fatalf("img が %d 個, want 1\n出力: %s", len(imgs), res.HTML)
	}
	return res, imgs[0]
}

// splitSrc は img の src を、パスの部分とクエリに分ける。
func splitSrc(t *testing.T, img element) (path, query string) {
	t.Helper()

	src, _ := img.Attr("src")
	if !strings.HasPrefix(src, "/__local/") {
		t.Fatalf("src = %q, /__local/ で始まらない", src)
	}
	p, q, _ := strings.Cut(src, "?")
	return localPathOf(t, p), q
}

// TestRenderImageFile は画像ファイルの変換を検証する
// （UT-226。根拠: FR-027, AR-040, AR-041, FR-040, FR-146 / IMP-116, IMP-118, IMP-128）。
//
// 画像のファイルは読まない（パスと版だけで HTML を組み立てる）ため、ファイルを置かずに試す。
func TestRenderImageFile(t *testing.T) {
	dir := t.TempDir()

	// ケース 1: 渡された版をそのまま付ける。alt はベース名。
	t.Run("1 基本", func(t *testing.T) {
		path := filepath.Join(dir, "a.png")
		_, img := renderImageFile(t, path, "3b9aca00-3")
		p, q := splitSrc(t, img)
		if p != filepath.ToSlash(path) {
			t.Errorf("src が指すパス = %q, want %q", p, filepath.ToSlash(path))
		}
		if q != "v=3b9aca00-3" {
			t.Errorf("src のクエリ = %q, want %q", q, "v=3b9aca00-3")
		}
		if alt, _ := img.Attr("alt"); alt != "a.png" {
			t.Errorf("alt = %q, want %q", alt, "a.png")
		}
	})

	// ケース 2: alt は属性値としてエスケープされ、解くと元の名前に戻る。
	t.Run("2 altのエスケープ", func(t *testing.T) {
		const name = `a"b<&.png`
		_, img := renderImageFile(t, filepath.Join(dir, name), "1-1")
		if alt, _ := img.Attr("alt"); alt != name {
			t.Errorf("alt = %q, want %q", alt, name)
		}
	})

	// ケース 3: 空白・#・非 ASCII を含む名前でも、src を解くと元のパス（# で切れない）。
	t.Run("3 空白と#と非ASCII", func(t *testing.T) {
		path := filepath.Join(dir, "a b#図.png")
		_, img := renderImageFile(t, path, "1-1")
		if p, _ := splitSrc(t, img); p != filepath.ToSlash(path) {
			t.Errorf("src が指すパス = %q, want %q", p, filepath.ToSlash(path))
		}
	})

	// ケース 4: 見出し・遅延ロード・目印は無い。
	t.Run("4 出力", func(t *testing.T) {
		res, _ := renderImageFile(t, filepath.Join(dir, "a.png"), "1-1")
		if len(res.Headings) != 0 || res.Headings == nil {
			t.Errorf("Headings = %#v, want 空スライス", res.Headings)
		}
		if res.NeedsMermaid || res.NeedsKaTeX || res.NeedsPlantUML {
			t.Errorf("Needs* = %v / %v / %v, want すべて false", res.NeedsMermaid, res.NeedsKaTeX, res.NeedsPlantUML)
		}
		if strings.Contains(res.HTML, "data-ref") {
			t.Errorf("data-ref がある\n出力: %s", res.HTML)
		}
	})

	// ケース 5: サニタイズを通った後も class="image-file" と /__local/ の src が残る（迂回経路を作らない）。
	// renderImageFile が div.image-file と img を見つけること自体がこの確認である。class の値も見る。
	t.Run("5 サニタイズ", func(t *testing.T) {
		res, err := New().RenderImageFile(filepath.Join(dir, "a.png"), "1-1")
		if err != nil {
			t.Fatalf("RenderImageFile がエラーを返した: %v", err)
		}
		divs := withClass(byTag(scanElements(t, res.HTML), "div"), "image-file")
		if len(divs) != 1 {
			t.Fatalf("div.image-file が残っていない\n出力: %s", res.HTML)
		}
		if cls, _ := divs[0].Attr("class"); cls != "image-file" {
			t.Errorf("class = %q, want %q", cls, "image-file")
		}
		if !strings.Contains(res.HTML, `src="/__local/`) {
			t.Errorf("/__local/ の src が残っていない\n出力: %s", res.HTML)
		}
	})
}
