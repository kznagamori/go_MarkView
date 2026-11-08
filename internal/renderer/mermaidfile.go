package renderer

// このファイルは v1.2.0 の Mermaid ファイルの変換（IMP-127。4.75.0）の宣言である。

// RenderMermaidFile はファイル全体を 1 つの図のブロックへ変換する（IMP-127）。
//
// TODO(v1.2.0): 未実装（テストの先行実装で置いたスタブ）。
func (r *Renderer) RenderMermaidFile(source []byte, opts Options) (Result, error) {
	return Result{Headings: []Heading{}}, errNotImplemented
}
