package renderer

import "errors"

// このファイルは v1.2.0 の見える文字の対応付け（IMP-123。4.75.0）の宣言である。
// TODO(v1.2.0): 中身は未実装（テストの先行実装で置いたスタブ）。

// errNotImplemented は v1.2.0 のスタブが返すエラー。実装で消す。
var errNotImplemented = errors.New("not implemented (v1.2.0)")

// TextTarget は修飾するブロックの指定（document の Ref から作る）。
type TextTarget struct {
	Cell            bool // 表のセルなら真（Table / Row / Col を使う）
	Index           int  // text:<n> の n
	Table, Row, Col int
}

// TextBlock は 1 つのブロックの見える文字と、ソースとの対応。
type TextBlock struct {
	Visible    []rune       // 見える文字（IMP-123 の表の数え方）
	Source     []Span       // 見える文字 1 つずつの、ソース上の区間（正規化後のテキストの上）
	Atoms      []Span       // 丸ごとにしか囲めない部品の、見える文字の上の区間
	Inlines    []InlineSpan // インラインの要素（入れ子の外側から順）
	HardBreaks int          // ハード改行の数（IMP-124 の 9）
}

// InlineSpan はインラインの要素 1 つ。
type InlineSpan struct {
	// Kind は "strong" / "emphasis" / "strike"（記号）/ "link" / "code" / "image" /
	// "del" / "s" / "strike-tag" / "color"（生 HTML のタグの対）/ "html"（その他のタグの対）。
	Kind  string
	Text  Span   // 見える文字の上の区間（中身）
	Open  Span   // ソース上の開きの記号・タグの区間
	Close Span   // ソース上の閉じの記号・タグの区間
	Color string // Kind が "color" のときの色の名前（TextColors）
}

// TextColors は文字色の一覧（FR-147）。名前・ファイルに書く値（Light）・Dark で描く値の順。
//
// TODO(v1.2.0): FR-147 の 7 色（IMP-123）。テストの先行実装ではゼロ値のまま（UT-221 は赤）。
var TextColors []struct{ Name, Light, Dark string }

// Visible は修飾するブロックの見える文字と、ソースとの対応を返す（IMP-123）。
//
// TODO(v1.2.0): 未実装。
func (r *Renderer) Visible(source []byte, target TextTarget) (*TextBlock, error) {
	return nil, errNotImplemented
}

// VisibleTexts は、修飾できるすべてのブロック（text と cell）の見える文字を、目印から鍵を除いた値
// （"text:3" / "cell:0:1:2"）ごとに返す（描画スモークの照合。BR-054, IMP-123）。
//
// TODO(v1.2.0): 未実装。
func (r *Renderer) VisibleTexts(source []byte) (map[string]string, error) {
	return nil, errNotImplemented
}
