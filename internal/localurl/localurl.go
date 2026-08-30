// Package localurl は、内部アセットサーバがローカルファイルを配信するための
// URL を組み立て、また解く（AR-040）。
//
// **依存を持たない葉パッケージである**（IMP-012）。組み立てる側（renderer の
// IMP-118）と解く側（assetsrv の IMP-161）は互いに依存できないが、両者の規則は
// 必ず一致していなければならない。食い違えば画像がすべて 404 になる。
// 逆変換の対を 1 か所に置くことで、片方だけが変わる事故を防いでいる。
//
// **このパッケージに依存を追加してはならない。** 依存を持たないことが、
// IMP-012 の依存規則の例外として認められている唯一の根拠である（mdfile と同じ）。
package localurl

import (
	"net/url"
	"path/filepath"
	"strings"
)

// Prefix はローカルファイル配信のパス接頭辞（AR-040, IMP-160）。
const Prefix = "/__local/"

// Encode は絶対パスから配信用の URL パスを組み立てる。
//
// パス全体を 1 つのセグメントとしてエスケープする。区切りの / も %2F になるため、
// 空白・`#`・`%`・非 ASCII を含むパスでも URL として壊れず、`/__local//docs/...`
// のような二重スラッシュも生じない。
func Encode(absPath string) string {
	return Prefix + url.PathEscape(filepath.ToSlash(absPath))
}

// Decode は配信用の URL パスから絶対パスを取り出す。
//
// 受け取るのは **エスケープされたままのパス**、つまり Encode の出力そのものである。
// net/http で扱う場合は `r.URL.Path`（復号済み）ではなく `r.URL.EscapedPath()` を渡す。
//
// 接頭辞が違う、エスケープが壊れている、中身が空の場合は ok が false になる。
// 呼び出し側はこの時点で 404 を返してよい（IMP-161）。
func Decode(urlPath string) (absPath string, ok bool) {
	rest, ok := strings.CutPrefix(urlPath, Prefix)
	if !ok || rest == "" {
		return "", false
	}

	decoded, err := url.PathUnescape(rest)
	if err != nil || decoded == "" {
		return "", false
	}

	return filepath.FromSlash(decoded), true
}
