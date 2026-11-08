package mdfile

// このファイルは v1.2.0 の開けるファイルの種類（IMP-105。4.75.0）の宣言である。
// TODO(v1.2.0): 中身は未実装（テストの先行実装で置いたスタブ）。
// 一覧はゼロ値のままにしてあり、UT-105 のケース 13・14 は実装するまで赤い。

// Kind は開けるファイルの種類（1.7 の「開けるファイル」）。
type Kind string

const (
	KindNone     Kind = ""         // 開けない
	KindMarkdown Kind = "markdown" // FR-010
	KindPlantUML Kind = "plantuml" // FR-025
	KindMermaid  Kind = "mermaid"  // FR-026
	KindImage    Kind = "image"    // FR-027
)

// PlantUMLExtensions は図のファイル（PlantUML）の拡張子（FR-025）。
// TODO(v1.2.0): `.puml` `.pu` `.wsd` `.plantuml`（IMP-105）。
var PlantUMLExtensions []string

// MermaidExtensions は図のファイル（Mermaid）の拡張子（FR-026）。
// TODO(v1.2.0): `.mmd` `.mermaid`（IMP-105）。
var MermaidExtensions []string

// ImageType は画像の拡張子と、配信するときの Content-Type（FR-022, IMP-161, IMP-162）。
type ImageType struct {
	Ext         string
	ContentType string
}

// ImageTypes は 9 つの画像の拡張子（FR-022）。並びは FR-037 の文言の並びとする。
// TODO(v1.2.0): IMP-105 の 9 つ（assetsrv の allowedImageExt から移す。IMP-161）。
var ImageTypes []ImageType

// KindOf は拡張子から種類を返す。比較は常に小文字化して行う。開けなければ KindNone。
// TODO(v1.2.0): 未実装。
func KindOf(path string) Kind {
	return KindNone
}

// IsDiagram は図のファイル（PlantUML か Mermaid。1.7）かを返す。
// TODO(v1.2.0): 未実装。
func IsDiagram(path string) bool {
	return false
}

// IsImage は画像ファイル（1.7）かを返す。
// TODO(v1.2.0): 未実装。
func IsImage(path string) bool {
	return false
}

// IsText は文字のファイル（Markdown と図。1.7）かを返す。外部エディタで開けるもの（FR-090）。
// TODO(v1.2.0): 未実装。
func IsText(path string) bool {
	return false
}

// IsOpenable は開けるファイル（文字のファイルと画像。1.7）かを返す。
// TODO(v1.2.0): 未実装。
func IsOpenable(path string) bool {
	return false
}

// ImageContentType は、画像の拡張子なら配信する Content-Type を返す（IMP-161）。
// TODO(v1.2.0): 未実装。
func ImageContentType(path string) (string, bool) {
	return "", false
}
