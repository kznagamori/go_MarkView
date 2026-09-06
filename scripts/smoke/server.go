package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kznagamori/go_MarkView/internal/localurl"
)

// 検証用ページ一式。実行ファイルへ埋め込み、作業ディレクトリに依存させない。
//
//go:embed harness.html harness.js stub-app.js stub-runtime.js
var assets embed.FS

// server は frontend/ をそのまま配りつつ、検証用のページと結果の受け口を
// 足したものを提供する。
//
// **file:// を使わない。** ES モジュール（frontend/js/ は module である）は
// file:// では同一生成元の扱いにならず、import がすべて失敗する。HTTP で
// 配れば本番の WebView と同じ経路になり、相対パス（vendor/ 以下）の解決も
// index.html と揃う。
type server struct {
	page   []byte       // 変換済みの本文を埋めた harness.html
	files  http.Handler // frontend/ のファイル
	result chan report  // ページから返ってきた結果
}

func newServer(frontendDir, body string) (*server, error) {
	// **Windows ではレジストリの内容が拡張子の判定に混ざる。** .js が
	// text/plain になっている環境があり、そうなるとブラウザが厳格な
	// MIME 検査でモジュールを拒む。ここで上書きしておく。
	for ext, typ := range map[string]string{
		".js":    "text/javascript; charset=utf-8",
		".css":   "text/css; charset=utf-8",
		".json":  "application/json",
		".svg":   "image/svg+xml",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
	} {
		if err := mime.AddExtensionType(ext, typ); err != nil {
			return nil, fmt.Errorf("MIME タイプを登録できない (%s): %w", ext, err)
		}
	}

	tpl, err := template.ParseFS(assets, "harness.html")
	if err != nil {
		return nil, fmt.Errorf("検証用ページを読めない: %w", err)
	}

	var page strings.Builder
	// 本文は renderer がサニタイズ済みで返したもの（IMP-116）。
	if err := tpl.Execute(&page, struct{ Body template.HTML }{template.HTML(body)}); err != nil {
		return nil, fmt.Errorf("検証用ページを組み立てられない: %w", err)
	}

	return &server{
		page:  []byte(page.String()),
		files: http.FileServer(http.Dir(frontendDir)),
		// 1 つ分の余裕を持たせ、受け取り側が待つ前に届いても取りこぼさない。
		result: make(chan report, 1),
	}, nil
}

// start は 127.0.0.1 の空きポートで待ち受け、検証用ページの URL を返す。
//
// **ポートを固定しない。** 開発機で他のプロセスと衝突すると、原因が
// 「資産が壊れている」のか「ポートが埋まっている」のか区別できなくなる。
func (s *server) start() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("待ち受けを開始できない: %w", err)
	}

	go http.Serve(listener, s) //nolint:errcheck // 停止はプロセスの終了で行う

	return fmt.Sprintf("http://%s/_smoke.html", listener.Addr().String()), nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/_smoke.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(s.page) //nolint:errcheck

		return

	case "/_smoke.js":
		s.serveAsset(w, "harness.js")

		return

	case "/_result":
		s.receive(w, r)

		return
	}

	// **ローカル画像を配る**（AR-040, IMP-160）。renderer は画像の src を
	// /__local/<絶対パスをエスケープしたもの> へ書き換えるため（IMP-118）、
	// ここを配らないと検証用文書の画像がすべて 404 になる。
	//
	// **本物の assetsrv は使わない。** あちらは MIME の絞り込みや SVG の
	// 扱い（NFR-031）を持つが、ここで問うのは「読める画像は読め、無い画像は
	// 読めない」ことだけである。**判定を増やすと、配信側の不具合と
	// フロントエンドの不具合を切り分けられなくなる。**
	if strings.HasPrefix(r.URL.EscapedPath(), localurl.Prefix) {
		s.serveLocal(w, r)

		return
	}

	// Wails のバインディングは本物を配らず代役で済ませる（stub-app.js の注記）。
	if strings.HasPrefix(r.URL.Path, "/wailsjs/") {
		switch r.URL.Path {
		case "/wailsjs/runtime/runtime.js":
			s.serveAsset(w, "stub-runtime.js")
		default:
			s.serveAsset(w, "stub-app.js")
		}

		return
	}

	s.files.ServeHTTP(w, r)
}

// serveLocal は /__local/ のパスを解いてファイルを返す。
//
// **無いファイルは 404 のままにする。** 検証用文書はわざと失敗する画像を
// 含んでおり（testdata/smoke.md）、ここで代わりの画像を返してしまうと
// IMP-226 の検査が成立しない。
func (s *server) serveLocal(w http.ResponseWriter, r *http.Request) {
	path, ok := localurl.Decode(r.URL.EscapedPath())
	if !ok {
		http.NotFound(w, r)

		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)

		return
	}

	http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(data))
}

func (s *server) serveAsset(w http.ResponseWriter, name string) {
	data, err := assets.ReadFile(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Write(data) //nolint:errcheck
}

func (s *server) receive(w http.ResponseWriter, r *http.Request) {
	var got report
	if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	w.WriteHeader(http.StatusNoContent)

	select {
	case s.result <- got:
	default: // 2 通目は捨てる。最初の 1 通で判断する。
	}
}
