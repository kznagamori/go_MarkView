package document

import "github.com/kznagamori/go_MarkView/internal/renderer"

// このファイルは v1.2.0 の修飾の書き込み（IMP-124。4.75.0）の宣言である。
// TODO(v1.2.0): 中身は未実装（テストの先行実装で置いたスタブ）。

// FormatStyle は修飾の操作。Strike と Color のどちらか一方だけを使う。
type FormatStyle struct {
	Strike bool   // 取り消し線のトグル（FR-146）
	Color  string // 文字色の名前（FR-147。renderer.TextColors の名前か "default"）
}

// FormatOp はフロントエンドから届く修飾の指示（FR-146, FR-147。IMP-316）。
type FormatOp struct {
	Start, End int    // ブロックの見える文字の中の、Unicode の符号位置での位置（半開区間）
	Visible    string // そのブロックの見える文字の全体（照合用。IMP-123, IMP-266）
	Style      FormatStyle
}

// PlanFormat は ref のブロックの選んだ文字に修飾を付ける・外す Patch を返す。
// widened は、手書きの取り消し線を全体で外した（FR-146。IMP-195 が format-widened の警告を出す）。
//
// TODO(v1.2.0): 未実装。
func PlanFormat(r *renderer.Renderer, raw []byte, ref Ref, op FormatOp) (p Patch, changed, widened bool, err error) {
	return Patch{}, false, false, errNotImplemented
}
