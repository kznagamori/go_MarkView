package assetsrv

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/kznagamori/go_MarkView/internal/localurl"
)

// appIcon は埋め込みアイコンの代わりに使うバイト列。
var appIcon = []byte("fake png bytes")

// newHandler はテスト用のハンドラを作る。
//
// WebView は起動しない。ハンドラを直接呼ぶ（UT-038, IMP-042）。
func newHandler(t *testing.T) *Handler {
	t.Helper()

	embedded := fstest.MapFS{
		"index.html":     {Data: []byte("<html></html>")},
		"css/tokens.css": {Data: []byte(":root{}")},
		"js/main.js":     {Data: []byte("// main")},
	}
	return New(embedded, appIcon)
}

// do はリクエストを 1 件処理し、応答を返す。
func do(t *testing.T, h *Handler, method, target string) *http.Response {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec.Result()
}

// get は GET を 1 件処理する。
func get(t *testing.T, h *Handler, target string) *http.Response {
	t.Helper()
	return do(t, h, http.MethodGet, target)
}

// writeFile はテスト用のファイルを作り、そのパスを返す。
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("親ディレクトリを作れない: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("ファイルを作れない: %v", err)
	}
	return p
}

// body は応答本文を文字列で返す。
func body(t *testing.T, res *http.Response) string {
	t.Helper()

	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("本文を読めない: %v", err)
	}
	return string(data)
}

// TestServeLocal は許可された画像の配信を検証する
// （UT-601。根拠: FR-022, AR-041 / IMP-161）。
func TestServeLocal(t *testing.T) {
	dir := t.TempDir()
	h := newHandler(t)

	t.Run("存在する png を配信する", func(t *testing.T) {
		p := writeFile(t, dir, "a.png", "image bytes")

		res := get(t, h, localurl.Encode(p))
		if res.StatusCode != http.StatusOK {
			t.Fatalf("ステータス = %d, want 200", res.StatusCode)
		}
		if got := body(t, res); got != "image bytes" {
			t.Errorf("本文 = %q, want %q", got, "image bytes")
		}
	})

	// UT-601 ケース 2: FR-022 の全形式
	t.Run("対応形式", func(t *testing.T) {
		for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".avif", ".bmp", ".ico"} {
			t.Run(ext, func(t *testing.T) {
				p := writeFile(t, dir, "img"+ext, "x")

				if res := get(t, h, localurl.Encode(p)); res.StatusCode != http.StatusOK {
					t.Errorf("ステータス = %d, want 200", res.StatusCode)
				}
			})
		}
	})

	t.Run("拡張子の大文字小文字を区別しない", func(t *testing.T) {
		p := writeFile(t, dir, "upper.PNG", "x")

		if res := get(t, h, localurl.Encode(p)); res.StatusCode != http.StatusOK {
			t.Errorf("ステータス = %d, want 200", res.StatusCode)
		}
	})

	// UT-601 ケース 3・4
	t.Run("存在しないパス", func(t *testing.T) {
		if res := get(t, h, localurl.Encode(filepath.Join(dir, "nosuch.png"))); res.StatusCode != http.StatusNotFound {
			t.Errorf("ステータス = %d, want 404", res.StatusCode)
		}
	})

	t.Run("ディレクトリのパス", func(t *testing.T) {
		// 画像の拡張子を持つディレクトリ。拡張子の検査は通るため、
		// 通常ファイルかどうかの検査（IMP-161 の 5）が働くかを見る。
		p := filepath.Join(dir, "dir.png")
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}

		if res := get(t, h, localurl.Encode(p)); res.StatusCode != http.StatusNotFound {
			t.Errorf("ステータス = %d, want 404", res.StatusCode)
		}
	})
}

// TestServeLocal_Rejects は許可されない配信の拒否を検証する
// （UT-602。根拠: AR-041, NFR-031 / IMP-161）。
func TestServeLocal_Rejects(t *testing.T) {
	dir := t.TempDir()
	h := newHandler(t)

	// 実在するが画像ではないファイル。存在の有無ではなく、拡張子で拒む
	// ことを確かめるために作る。
	for _, name := range []string{"a.md", "a.go", "a.exe", "a.json", "noext", "a.txt"} {
		writeFile(t, dir, name, "secret")
	}

	tests := []struct {
		name   string
		target string
	}{
		// UT-602 ケース 1〜3
		{"Markdown", localurl.Encode(filepath.Join(dir, "a.md"))},
		{"Go のソース", localurl.Encode(filepath.Join(dir, "a.go"))},
		{"実行ファイル", localurl.Encode(filepath.Join(dir, "a.exe"))},
		{"JSON", localurl.Encode(filepath.Join(dir, "a.json"))},
		{"拡張子なし", localurl.Encode(filepath.Join(dir, "noext"))},

		// UT-602 ケース 4・5
		{"上位へたどるパス", localurl.Prefix + "..%2F..%2Fetc%2Fpasswd"},
		{"画像に見せかけた上位へのパス", localurl.Prefix + "..%2F..%2Fnosuch.png"},
		{"エスケープされていない上位へのパス", localurl.Prefix + "../../etc/passwd"},

		// 接頭辞だけ
		{"接頭辞のみ", localurl.Prefix},

		// 壊れたエスケープ（%zz など）はここでは試せない。HTTP の層で
		// リクエストとして成立せず、ハンドラまで届かないためである。
		// localurl.Decode 側の拒否は TestDecode_Rejects が見ている。
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := get(t, h, tt.target)

			if res.StatusCode != http.StatusNotFound {
				t.Errorf("ステータス = %d, want 404", res.StatusCode)
			}
			// 理由を本文に含めない（IMP-161）。
			if got := body(t, res); strings.Contains(got, "secret") {
				t.Errorf("本文に中身が漏れている: %q", got)
			}
		})
	}
}

// TestServeLocal_SymlinkExtension は検査の順序を検証する
// （UT-602 ケース 6。根拠: AR-041 / IMP-161）。
//
// **シンボリックリンクを解決してから拡張子を見る。** 順序が逆の実装は、
// `x.png` という名前で任意のファイルを配信してしまう。
func TestServeLocal_SymlinkExtension(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "secret.txt", "secret contents")

	link := filepath.Join(dir, "innocent.png")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("シンボリックリンクを作れない環境（Windows は既定で管理者権限が要る）: %v", err)
	}

	res := get(t, newHandler(t), localurl.Encode(link))

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("ステータス = %d, want 404（解決後の拡張子で判定すること）", res.StatusCode)
	}
	if got := body(t, res); strings.Contains(got, "secret contents") {
		t.Errorf("リンク先の中身が配信された: %q", got)
	}
}

// TestServeLocal_Headers は応答ヘッダを検証する
// （UT-603。根拠: AR-041 / IMP-162）。
func TestServeLocal_Headers(t *testing.T) {
	dir := t.TempDir()
	h := newHandler(t)

	tests := []struct {
		name      string
		file      string
		wantCType string
	}{
		{"png", "a.png", "image/png"},
		{"jpg", "a.jpg", "image/jpeg"},
		{"jpeg", "a.jpeg", "image/jpeg"},
		{"gif", "a.gif", "image/gif"},
		{"svg", "a.svg", "image/svg+xml"},
		{"webp", "a.webp", "image/webp"},
		{"avif", "a.avif", "image/avif"},
		{"bmp", "a.bmp", "image/bmp"},
		{"ico", "a.ico", "image/x-icon"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeFile(t, dir, tt.file, "x")
			res := get(t, h, localurl.Encode(p))

			if got := res.Header.Get("Content-Type"); got != tt.wantCType {
				t.Errorf("Content-Type = %q, want %q", got, tt.wantCType)
			}

			// UT-603 ケース 3・4: すべての /__local/ に付ける
			if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
			if got := res.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}

			// UT-603 ケース 2: SVG のスクリプト対策（AR-041）
			if got := res.Header.Get("Content-Security-Policy"); !strings.Contains(got, "sandbox") {
				t.Errorf("Content-Security-Policy = %q, want sandbox を含む", got)
			}
		})
	}
}

// TestServeAppIcon はアプリケーションアイコンの配信を検証する
// （UT-604。根拠: UI-025 / IMP-160）。
func TestServeAppIcon(t *testing.T) {
	h := newHandler(t)

	t.Run("png を配信する", func(t *testing.T) {
		res := get(t, h, AppIconPath)

		if res.StatusCode != http.StatusOK {
			t.Fatalf("ステータス = %d, want 200", res.StatusCode)
		}
		if got := res.Header.Get("Content-Type"); got != "image/png" {
			t.Errorf("Content-Type = %q, want image/png", got)
		}
		if got := body(t, res); got != string(appIcon) {
			t.Errorf("本文 = %q, want %q", got, appIcon)
		}
	})

	// UT-604 ケース 2: 配信するのは png のみ
	t.Run("ico は配信しない", func(t *testing.T) {
		if res := get(t, h, "/appicon.ico"); res.StatusCode != http.StatusNotFound {
			t.Errorf("ステータス = %d, want 404", res.StatusCode)
		}
	})

	t.Run("アイコンがなければ 404", func(t *testing.T) {
		empty := New(fstest.MapFS{}, nil)

		if res := get(t, empty, AppIconPath); res.StatusCode != http.StatusNotFound {
			t.Errorf("ステータス = %d, want 404", res.StatusCode)
		}
	})
}

// TestServeEmbedded は埋め込み資産の配信を検証する
// （UT-605。根拠: AR-020 / IMP-160）。
func TestServeEmbedded(t *testing.T) {
	h := newHandler(t)

	tests := []struct {
		name   string
		target string
		want   int
	}{
		{"index.html", "/index.html", http.StatusOK},
		{"CSS", "/css/tokens.css", http.StatusOK},
		{"JS", "/js/main.js", http.StatusOK},
		{"存在しない資産", "/nope.js", http.StatusNotFound},
		{"存在しないディレクトリ", "/nope/x.css", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if res := get(t, h, tt.target); res.StatusCode != tt.want {
				t.Errorf("ステータス = %d, want %d", res.StatusCode, tt.want)
			}
		})
	}

	// Content-Type を mime.TypeByExtension に任せると、Windows では
	// レジストリ次第で .js が text/plain になる環境がある。CSS と JS の
	// 種別を誤ると画面が壊れるため、明示した値を固定する。
	t.Run("Content-Type", func(t *testing.T) {
		for _, tt := range []struct{ target, want string }{
			{"/index.html", "text/html; charset=utf-8"},
			{"/css/tokens.css", "text/css; charset=utf-8"},
			{"/js/main.js", "text/javascript; charset=utf-8"},
		} {
			t.Run(tt.target, func(t *testing.T) {
				if got := get(t, h, tt.target).Header.Get("Content-Type"); got != tt.want {
					t.Errorf("Content-Type = %q, want %q", got, tt.want)
				}
			})
		}
	})

	t.Run("ルートは index.html を返す", func(t *testing.T) {
		res := get(t, h, "/")
		if res.StatusCode != http.StatusOK {
			t.Fatalf("ステータス = %d, want 200", res.StatusCode)
		}
		if got := body(t, res); got != "<html></html>" {
			t.Errorf("本文 = %q, want index.html の内容", got)
		}
	})

	t.Run("長くキャッシュさせる", func(t *testing.T) {
		res := get(t, h, "/index.html")

		if got := res.Header.Get("Cache-Control"); !strings.Contains(got, "max-age=") {
			t.Errorf("Cache-Control = %q, want max-age を含む（IMP-162）", got)
		}
	})
}

// TestServeLocal_Head は HEAD 要求で本文を返さないことを検証する。
func TestServeLocal_Head(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.png", "image bytes")

	res := do(t, newHandler(t), http.MethodHead, localurl.Encode(p))

	if res.StatusCode != http.StatusOK {
		t.Fatalf("ステータス = %d, want 200", res.StatusCode)
	}
	if got := body(t, res); got != "" {
		t.Errorf("本文 = %q, want 空", got)
	}
}
