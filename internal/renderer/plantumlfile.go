package renderer

// このファイルは v1.2.0 の PlantUML ファイルの変換（IMP-126。4.75.0）の宣言である。
// TODO(v1.2.0): 中身は未実装（テストの先行実装で置いたスタブ）。

// Diagram は PlantUML ファイルの中の図 1 つ（MD-086）。
type Diagram struct {
	Name   string // 見出し（MD-086 の 3）
	Span   Span   // @start の行の頭から、@end の行の終わり（改行の手前）まで。@end が無ければファイルの終わりまで
	Closed bool   // @end の行があったか
}

// SplitPlantUML はファイルの中身（正規化後）を図に切り分ける（MD-086）。
//
// TODO(v1.2.0): 未実装。
func SplitPlantUML(text []byte) []Diagram {
	return nil
}

// RenderPlantUMLFile は PlantUML ファイルを図ごとの見出しと図のブロックへ変換する（IMP-126）。
//
// TODO(v1.2.0): 未実装。
func (r *Renderer) RenderPlantUMLFile(source []byte, opts Options) (Result, error) {
	return Result{Headings: []Heading{}}, errNotImplemented
}
