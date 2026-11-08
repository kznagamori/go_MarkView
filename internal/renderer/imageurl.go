package renderer

// このファイルは画像の URL の書き換え（IMP-118）と、Markdown の中の参照の解決（AR-042）を持つ。
// renderer.go が 400 行の目安（IMP-011）を超えたため分けた（v1.2.0）。

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/localurl"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// baseDirKey は変換 1 回分の基準ディレクトリを parser.Context へ置く鍵（IMP-118）。
var baseDirKey = parser.NewContextKey()

// remoteSchemes は書き換えずにそのまま渡す URL の接頭辞（IMP-118, MD-071）。
var remoteSchemes = []string{"http://", "https://", "data:"}

// imageTransformer は画像の src を配信用 URL へ書き換える（IMP-118）。
//
// リンク（a href）は書き換えない。クリック時にフロントエンドが捕捉して Go 側へ
// 渡すため、元の値のまま保持する必要がある（AR-060, IMP-223）。
type imageTransformer struct{}

func (imageTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	// 基準ディレクトリを置くのは Render だけである。Locate と Visible は同じパーサーを通るが、
	// URL を出さないため書き換えない——**画像ごとの os.Stat を呼ばない**（IMP-118）。書き換えて
	// も位置の数え方は変わらない。
	baseDir, ok := pc.Get(baseDirKey).(string)
	if !ok {
		return
	}

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		img, ok := n.(*ast.Image)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}

		img.Destination = []byte(rewriteImageURL(string(img.Destination), baseDir))
		return ast.WalkContinue, nil
	})
}

// rewriteImageURL は画像の src を WebView から取得できる形へ変換する（IMP-118）。
//
//	http:// https://   → そのまま（MD-071）
//	data:              → そのまま
//	絶対パス・相対パス → /__local/<エスケープ済み絶対パス>?v=<版>（AR-040）
//
// 相対パスは baseDir（表示中ファイルのディレクトリ）を基準に解決する。
// ツリールートを基準にしてはならない（AR-042）。
//
// 上記以外のスキーム（file: や javascript: 等）はローカルパスとして扱う。
// 結果は存在しないパスとなり配信されないため、素通しするより安全である。
//
// **ローカルの画像にはファイルの版を付ける**（BUG-026。IMP-118）。WebView2 も WebKitGTK も、
// ページの中で一度読んだ画像を、Cache-Control: no-store でも同じ URL なら使い回す。MarkView は
// ページを遷移しないため、URL がパスだけで決まると、画像を書き換えて F5 を押してもウィンドウを
// 閉じるまで古い画像が出続けた。**版が変われば URL が変わって取り直され、変わらなければ取り直さない**
// （再描画でちらつかない）。版は localurl.Version で作る（画像ファイルの表示と同じ関数。IMP-128）。
// os.Stat に失敗したら付けない（404 になるだけである）。配信側はクエリを見ない（IMP-161）。
func rewriteImageURL(src, baseDir string) string {
	if src == "" {
		return src
	}

	for _, scheme := range remoteSchemes {
		if len(src) >= len(scheme) && strings.EqualFold(src[:len(scheme)], scheme) {
			return src
		}
	}

	path := ResolveRef(src, baseDir)
	u := localurl.Encode(path)

	if fi, err := os.Stat(path); err == nil {
		u += "?v=" + localurl.Version(fi)
	}

	return u
}

// ResolveRef は Markdown 内の参照をローカルの絶対パスへ解決する（AR-042）。
//
// **画像（IMP-118）とリンク遷移（IMP-312）が同じ規則を使う。** 別々に書くと、
// [x](./a.png) が開くファイルと ![x](./a.png) が表示するファイルが食い違いうる。
//
// 基準は baseDir、すなわち表示中ファイルのディレクトリである。ツリールートを
// 基準にしてはならない（AR-042）。スキームの判定は呼び出し側が済ませておく。
func ResolveRef(ref, baseDir string) string {
	// Markdown の宛先は URL であり、空白などは %20 で書かれうる。
	// ファイルパスへ戻してから解決する。復号できない場合は元の文字列を使う。
	path := ref
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}

	// 先頭が / のパスは OS を問わず絶対として扱う。Markdown の URL は POSIX
	// 形式で書かれるが、Windows の filepath.IsAbs はドライブレターを要求し、
	// /abs/a.png を相対と判定してしまう。
	if !strings.HasPrefix(path, "/") && !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	return filepath.Clean(path)
}
