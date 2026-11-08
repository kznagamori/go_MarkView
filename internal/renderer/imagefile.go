package renderer

import (
	"html"
	"path/filepath"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/localurl"
)

// RenderImageFile は画像ファイルを 1 枚だけ表示する HTML を返す（IMP-128, FR-027）。
// version は localurl.Version で作ったファイルの版（IMP-118）。空なら版を付けない。
//
//   - URL は localurl.Encode（埋め込み画像と同じ。IMP-118）で組み立て、**版をクエリで付ける**。
//     外部で画像を書き換えたら、監視の再描画（FR-014）で版の変わった URL が出て、WebView が取り直す
//     （BUG-026）
//   - alt はファイル名（ベース名）。読み込みに失敗したら、埋め込み画像と同じ壊れた画像の表示に
//     代替テキストとして出る（IMP-226, DSP-123）
//   - **中身は読まない**（IMP-102）。寸法も求めない（フロントエンドが naturalWidth で見る。IMP-228）
//   - 見出し・目印・遅延ロードは持たない
//   - **出力は IMP-116 のサニタイズを通す**（Go が組み立てた HTML でも迂回経路を作らない。AR-031）
//
// 画像ファイルかどうかの判断は DocumentDTO.kind で行い、class="image-file" の有無では行わない
// （IMP-228。生 HTML でも同じ class を書ける）。出力の形:
//
//	<div class="image-file"><img src="/__local/<エスケープ済み絶対パス>?v=<版>" alt="<ファイル名>"></div>
func (r *Renderer) RenderImageFile(path, version string) (result Result, err error) {
	defer recoverRender(&result, &err)

	src := localurl.Encode(path)
	if version != "" {
		src += "?v=" + version
	}

	var b strings.Builder
	b.WriteString(`<div class="image-file"><img src="`)
	b.WriteString(html.EscapeString(src))
	b.WriteString(`" alt="`)
	b.WriteString(html.EscapeString(filepath.Base(path)))
	b.WriteString(`"></div>`)

	// 生 HTML は無い（自分で組み立てた）ため、サニタイズの後の走査は省く。
	sanitized := sanitizeAfter(r.policy.SanitizeBytes([]byte(b.String())), false)

	return Result{HTML: string(sanitized), Headings: []Heading{}}, nil
}
