package document

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// refKeyPattern は目印の鍵の形（IMP-102: crypto/rand の 8 バイトを 16 進 16 文字にしたもの）。
var refKeyPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// writeBytes はバイト列のファイルを作り、そのパスを返す。
func writeBytes(t *testing.T, dir, name string, b []byte) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("テストファイルを作れない: %v", err)
	}
	return p
}

// mustLoad は Load を呼び、失敗したらテストを止める。
func mustLoad(t *testing.T, r *renderer.Renderer, path string, opts LoadOptions) *Document {
	t.Helper()

	doc, err := Load(r, path, opts)
	if err != nil {
		t.Fatalf("Load がエラーを返した: %v", err)
	}
	return doc
}

// TestLoad_DigestAndKey は読み込みの要約と鍵を検証する
// （UT-116。根拠: FR-140, FR-143, NFR-030 / IMP-100, IMP-102, IMP-120）。
func TestLoad_DigestAndKey(t *testing.T) {
	r := renderer.New()
	const tasks = "- [ ] a\n- [x] b\n"

	t.Run("1: 同じファイルを 2 回 Load", func(t *testing.T) {
		p := writeFile(t, t.TempDir(), "a.md", "# a\n")
		first := mustLoad(t, r, p, LoadOptions{})
		second := mustLoad(t, r, p, LoadOptions{})

		if first.Digest != second.Digest {
			t.Error("Digest が違う")
		}
		// ゼロ値どうしの一致で通らないようにする（要約を求めていない実装を捕まえる）。
		if first.Digest == ([sha256.Size]byte{}) {
			t.Error("Digest がゼロ値")
		}
		if first.RefKey == second.RefKey {
			t.Errorf("RefKey が同じ（%q）。Load のたびに作り直していない", first.RefKey)
		}
	})

	t.Run("2: RefKey の形", func(t *testing.T) {
		doc := mustLoad(t, r, writeFile(t, t.TempDir(), "a.md", "# a\n"), LoadOptions{})
		if !refKeyPattern.MatchString(doc.RefKey) {
			t.Errorf("RefKey = %q, want 16 文字の 0-9a-f", doc.RefKey)
		}
	})

	t.Run("3: 要約の一致する ExpectDigest と RefKey を渡す", func(t *testing.T) {
		raw := []byte("# a\n")
		p := writeBytes(t, t.TempDir(), "a.md", raw)
		doc := mustLoad(t, r, p, LoadOptions{RefKey: testKey, ExpectDigest: sha256.Sum256(raw)})

		if doc.RefKey != testKey {
			t.Errorf("RefKey = %q, want %q（渡した値）", doc.RefKey, testKey)
		}
	})

	// ケース 4・5: 正規化した後の内容で要約を取る誤りを捕まえる
	t.Run("4: BOM の有無だけが違う", func(t *testing.T) {
		dir := t.TempDir()
		plain := mustLoad(t, r, writeBytes(t, dir, "plain.md", []byte("# a\n")), LoadOptions{})
		bom := mustLoad(t, r, writeBytes(t, dir, "bom.md", []byte("\xEF\xBB\xBF# a\n")), LoadOptions{})

		if plain.Digest == bom.Digest {
			t.Error("Digest が同じ（正規化の前のバイト列の要約でない）")
		}
	})

	t.Run("5: 改行コードだけが違う", func(t *testing.T) {
		dir := t.TempDir()
		lf := mustLoad(t, r, writeBytes(t, dir, "lf.md", []byte("a\nb\n")), LoadOptions{})
		crlf := mustLoad(t, r, writeBytes(t, dir, "crlf.md", []byte("a\r\nb\r\n")), LoadOptions{})

		if lf.Digest == crlf.Digest {
			t.Error("Digest が同じ（正規化の前のバイト列の要約でない）")
		}
	})

	t.Run("6: 不正な UTF-8 を含むファイル", func(t *testing.T) {
		doc := mustLoad(t, r, writeBytes(t, t.TempDir(), "invalid.md", []byte("a\xFFb\n")), LoadOptions{})
		if doc.Editable() {
			t.Error("Editable が真")
		}
	})

	t.Run("7: 正常なファイルと空のファイル", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"normal.md", "empty.md"} {
			content := "# a\n"
			if name == "empty.md" {
				content = ""
			}
			doc := mustLoad(t, r, writeFile(t, dir, name, content), LoadOptions{})
			if !doc.Editable() {
				t.Errorf("%s: Editable が偽", name)
			}
		}
	})

	// ケース 8・9: 呼び出し側を見る（BUG-005 と同じ形の見落としを避ける）
	t.Run("8: Load が自分の鍵を Render へ渡している", func(t *testing.T) {
		doc := mustLoad(t, r, writeFile(t, t.TempDir(), "tasks.md", tasks), LoadOptions{})
		if doc.RefKey == "" {
			t.Fatal("RefKey が空")
		}
		want := `data-ref="` + doc.RefKey + `:task:0"`
		if !strings.Contains(doc.HTML, want) {
			t.Errorf("HTML に %s が無い: %q", want, doc.HTML)
		}
	})

	t.Run("9: 渡した鍵が HTML の目印に使われる", func(t *testing.T) {
		raw := []byte(tasks)
		p := writeBytes(t, t.TempDir(), "tasks.md", raw)
		doc := mustLoad(t, r, p, LoadOptions{RefKey: testKey, ExpectDigest: sha256.Sum256(raw)})

		want := `data-ref="` + testKey + `:task:0"`
		if !strings.Contains(doc.HTML, want) {
			t.Errorf("HTML に %s が無い: %q", want, doc.HTML)
		}
	})

	// ケース 10: 鍵の引き継ぎの安全弁（IMP-102）。書き込んだ後、読み直す前に外部で書き換えられた
	t.Run("10: ExpectDigest がファイルと違う内容の要約", func(t *testing.T) {
		p := writeFile(t, t.TempDir(), "tasks.md", tasks)
		doc := mustLoad(t, r, p, LoadOptions{RefKey: testKey, ExpectDigest: sha256.Sum256([]byte("- [x] a\n- [x] b\n"))})

		if doc.RefKey == testKey {
			t.Error("RefKey が渡した値のまま（要約が一致しないのに鍵を引き継いだ）")
		}
		if !refKeyPattern.MatchString(doc.RefKey) {
			t.Errorf("RefKey = %q, want 16 文字の 0-9a-f の新しい鍵", doc.RefKey)
		}
		want := `data-ref="` + doc.RefKey + `:task:0"`
		if !strings.Contains(doc.HTML, want) {
			t.Errorf("HTML に %s が無い（目印も新しい鍵であること）: %q", want, doc.HTML)
		}
	})

	t.Run("11: RefKey だけを渡し、ExpectDigest を渡さない", func(t *testing.T) {
		p := writeFile(t, t.TempDir(), "tasks.md", tasks)
		doc := mustLoad(t, r, p, LoadOptions{RefKey: testKey})

		if doc.RefKey == testKey {
			t.Error("RefKey が渡した値のまま（一致を確かめずに鍵を使った）")
		}
		if !refKeyPattern.MatchString(doc.RefKey) {
			t.Errorf("RefKey = %q, want 16 文字の 0-9a-f の新しい鍵", doc.RefKey)
		}
	})
}
