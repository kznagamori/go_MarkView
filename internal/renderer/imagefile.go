package renderer

// このファイルは v1.2.0 の画像ファイルの変換（IMP-128。4.75.0）の宣言である。

// RenderImageFile は画像ファイルを 1 枚だけ表示する HTML を返す（IMP-128）。
// version は localurl.Version で作ったファイルの版（IMP-118）。
//
// TODO(v1.2.0): 未実装（テストの先行実装で置いたスタブ）。
func (r *Renderer) RenderImageFile(path, version string) (Result, error) {
	return Result{Headings: []Heading{}}, errNotImplemented
}
