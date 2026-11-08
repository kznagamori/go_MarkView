package mdfile

// Kind は開けるファイルの種類（1.7 の「開けるファイル」。IMP-105）。
type Kind string

const (
	KindNone     Kind = ""         // 開けない
	KindMarkdown Kind = "markdown" // FR-010
	KindPlantUML Kind = "plantuml" // FR-025
	KindMermaid  Kind = "mermaid"  // FR-026
	KindImage    Kind = "image"    // FR-027
)

// PlantUMLExtensions は図のファイル（PlantUML）の拡張子（FR-025）。
//
// **`.iuml` は含めない。** 取り込み用の断片は単体で図にならない（FR-025）。開けないファイルとして
// 扱い、リンク先なら OS の既定のアプリケーションへ渡す（FR-050）。
var PlantUMLExtensions = []string{".puml", ".pu", ".wsd", ".plantuml"}

// MermaidExtensions は図のファイル（Mermaid）の拡張子（FR-026）。
var MermaidExtensions = []string{".mmd", ".mermaid"}

// ImageType は画像の拡張子と、配信するときの Content-Type（FR-022, IMP-161, IMP-162）。
type ImageType struct {
	Ext         string
	ContentType string
}

// ImageTypes は 9 つの画像の拡張子（FR-022）。並びは FR-037 の文言の並びとする（UT-105 のケース 14）。
//
// **v1.1.0 までは assetsrv が持っていた**（allowedImageExt）。画像ファイルを開けるようになって
// （FR-027）、配信する拡張子と開ける拡張子が同じ一覧になったため、ここへ移した（IMP-105, IMP-161）。
// 2 か所に置くと、片方だけ足したときに、開けるのに描けない画像や、描けるのに開けない画像が生まれる。
var ImageTypes = []ImageType{
	{".png", "image/png"},
	{".jpg", "image/jpeg"},
	{".jpeg", "image/jpeg"},
	{".gif", "image/gif"},
	{".svg", "image/svg+xml"},
	{".webp", "image/webp"},
	{".avif", "image/avif"},
	{".bmp", "image/bmp"},
	{".ico", "image/x-icon"},
}

// KindOf は拡張子から種類を返す。比較は常に小文字化して行う。開けなければ KindNone。
//
// 最終要素の拡張子だけを見る（"a.md.txt" は開けない）。**名前全体が拡張子の文字と一致するもの
// （".md" / ".puml" / ".png" など）は KindNone とする**（UT-105 のケース 4）。
func KindOf(path string) Kind {
	ext := extOf(path)
	if ext == "" {
		return KindNone
	}

	switch {
	case contains(Extensions, ext):
		return KindMarkdown
	case contains(PlantUMLExtensions, ext):
		return KindPlantUML
	case contains(MermaidExtensions, ext):
		return KindMermaid
	}

	if _, ok := imageContentType(ext); ok {
		return KindImage
	}

	return KindNone
}

// IsDiagram は図のファイル（PlantUML か Mermaid。1.7）かを返す。
func IsDiagram(path string) bool {
	k := KindOf(path)

	return k == KindPlantUML || k == KindMermaid
}

// IsImage は画像ファイル（1.7）かを返す。
func IsImage(path string) bool {
	return KindOf(path) == KindImage
}

// IsText は文字のファイル（Markdown と図。1.7）かを返す。外部エディタで開けるもの（FR-090）。
//
// **IsOpenable との違いは画像だけである。** 画像はエディタで開かない（FR-027, FR-090）。
// エディタで開けるかの判断にだけ使う。
func IsText(path string) bool {
	k := KindOf(path)

	return k == KindMarkdown || k == KindPlantUML || k == KindMermaid
}

// IsOpenable は開けるファイル（文字のファイルと画像。1.7）かを返す。
func IsOpenable(path string) bool {
	return KindOf(path) != KindNone
}

// ImageContentType は、画像の拡張子なら配信する Content-Type を返す（IMP-161）。
//
// 名前全体が拡張子のもの（".png"）は画像としない（KindOf と同じ規則）。
func ImageContentType(path string) (string, bool) {
	ext := extOf(path)
	if ext == "" {
		return "", false
	}

	return imageContentType(ext)
}

// imageContentType は小文字の拡張子から Content-Type を引く。
func imageContentType(ext string) (string, bool) {
	for _, it := range ImageTypes {
		if it.Ext == ext {
			return it.ContentType, true
		}
	}

	return "", false
}

// contains は list に ext があるかを返す。
func contains(list []string, ext string) bool {
	for _, e := range list {
		if e == ext {
			return true
		}
	}

	return false
}
