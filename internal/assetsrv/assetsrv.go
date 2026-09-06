// Package assetsrv は埋め込み資産とローカル画像を内部 HTTP 経由で配信する
// （IMP-160 系, AR-040）。
//
// WebView は file:// スキームでの任意のローカルファイル参照を制限するため、
// ローカル画像（FR-022）はこのハンドラを通して配信する。
//
// 依存するのは localurl だけである（IMP-012）。
package assetsrv

import (
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/localurl"
)

// AppIconPath はアプリケーションアイコンの配信パス（IMP-160, UI-025）。
//
// 情報ダイアログ（UI-100）が参照する。配信するのは png のみとする。
const AppIconPath = "/appicon.png"

// allowedImageExt は配信を許す拡張子と Content-Type（IMP-161, FR-022）。
//
// **画像として扱える拡張子だけを配信する。** 任意の拡張子を配信すると、
// 文書と同じディレクトリにある鍵や設定ファイルまで WebView から読めてしまう
// （AR-041, NFR-031）。
var allowedImageExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
}

// Handler は 3 系統のパスを配信する（IMP-160）。
//
//	/appicon.png              埋め込んだアプリケーションアイコン
//	/__local/<エンコード済み> ローカルの画像ファイル（IMP-161 の検査を通す）
//	それ以外の / 配下          埋め込みのアプリ資産
type Handler struct {
	embedded fs.FS
	appIcon  []byte
}

// New はハンドラを作る（IMP-160）。
func New(embedded fs.FS, appIcon []byte) *Handler {
	return &Handler{
		embedded: embedded,
		appIcon:  appIcon,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == AppIconPath:
		h.serveAppIcon(w)

	case strings.HasPrefix(r.URL.EscapedPath(), localurl.Prefix):
		h.serveLocal(w, r)

	default:
		h.serveEmbedded(w, r)
	}
}

// serveLocal はローカルの画像を配信する（IMP-161, AR-041）。
//
// **検査の順序が重要である。** シンボリックリンクを解決してから拡張子を見る。
// 順序が逆だと、`x.png` という名前で `secret.txt` を指すリンクが通ってしまう。
//
// どの検査で落ちても 404 とし、理由を本文に含めない。理由を返すと、
// ファイルの存在有無を外から確かめられる。
func (h *Handler) serveLocal(w http.ResponseWriter, r *http.Request) {
	// 1. URL からパスを取り出す。復号済みの Path ではなく、エスケープされた
	//    ままの値を渡す（localurl.Decode の契約）。
	local, ok := localurl.Decode(r.URL.EscapedPath())
	if !ok {
		notFound(w)
		return
	}

	// 2. 正規化する（IMP-025）。
	abs, err := filepath.Abs(filepath.Clean(local))
	if err != nil {
		notFound(w)
		return
	}

	// 3. シンボリックリンクを解決する。存在しないパスはここで落ちる。
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		notFound(w)
		return
	}

	// 4. **解決後のパス**の拡張子を検査する。
	ctype, ok := allowedImageExt[strings.ToLower(filepath.Ext(resolved))]
	if !ok {
		notFound(w)
		return
	}

	// 5. 通常ファイルであることを確認する。ディレクトリやデバイスファイルは拒む。
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		notFound(w)
		return
	}

	f, err := os.Open(resolved)
	if err != nil {
		notFound(w)
		return
	}
	defer func() { _ = f.Close() }()

	setLocalHeaders(w, ctype)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, f)
}

// setLocalHeaders はローカル配信の応答ヘッダを付ける（IMP-162, AR-041）。
func setLocalHeaders(w http.ResponseWriter, ctype string) {
	head := w.Header()

	head.Set("Content-Type", ctype)

	// 拡張子から決めた種別を、中身から推測し直させない（AR-041）。
	head.Set("X-Content-Type-Options", "nosniff")

	// SVG に埋め込まれたスクリプト対策（AR-041）。<img> 経由では実行され
	// ないが、WebView が直接その URL へ遷移した場合に備えて二重に防ぐ。
	head.Set("Content-Security-Policy", "sandbox")

	// 更新した画像が古いまま表示されるのを防ぐ（IMP-162）。文書は編集中に
	// 何度も読み直される。
	head.Set("Cache-Control", "no-store")
}

// serveAppIcon はアプリケーションアイコンを配信する（IMP-160, UI-025）。
func (h *Handler) serveAppIcon(w http.ResponseWriter) {
	if len(h.appIcon) == 0 {
		notFound(w)
		return
	}

	head := w.Header()
	head.Set("Content-Type", "image/png")
	head.Set("X-Content-Type-Options", "nosniff")
	head.Set("Cache-Control", cacheForever)

	_, _ = w.Write(h.appIcon)
}

// serveEmbedded は埋め込みのアプリ資産を配信する（IMP-160, AR-020）。
//
// http.FileServer を使わない。あれは /index.html を / へ 301 で書き換え、
// ディレクトリ一覧も返す。どちらもこのアプリには不要である。
func (h *Handler) serveEmbedded(w http.ResponseWriter, r *http.Request) {
	// URL のパスは常にスラッシュ区切りであるため path パッケージを使う。
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" || name == "." {
		name = "index.html"
	}

	data, err := fs.ReadFile(h.embedded, name)
	if err != nil {
		notFound(w)
		return
	}

	head := w.Header()
	head.Set("Content-Type", embeddedContentType(name))
	head.Set("X-Content-Type-Options", "nosniff")

	// 内容は実行ファイルに固定されているため、長く保持させてよい（IMP-162）。
	head.Set("Cache-Control", cacheForever)

	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

// assetContentType は埋め込み資産の Content-Type（IMP-162）。
//
// mime.TypeByExtension に任せない。Windows ではレジストリの内容に左右され、
// .js が text/plain になる環境がある。CSS と JS の種別を誤ると画面が壊れる。
var assetContentType = map[string]string{
	".html":  "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".json":  "application/json",
	".map":   "application/json",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".txt":   "text/plain; charset=utf-8",
}

// embeddedContentType は名前から Content-Type を決める。
func embeddedContentType(name string) string {
	if ctype, ok := assetContentType[strings.ToLower(path.Ext(name))]; ok {
		return ctype
	}
	return "application/octet-stream"
}

// cacheForever は変わらない資産に付けるキャッシュ指示（IMP-162）。
const cacheForever = "public, max-age=31536000"

// notFound は 404 を返す。理由は本文に含めない（IMP-161）。
func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}
