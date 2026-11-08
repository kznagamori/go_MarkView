package document

import (
	"errors"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// このファイルは v1.2.0 のコードブロックと図の書き換え（IMP-122。4.75.0）の宣言である。
// TODO(v1.2.0): 中身は未実装（テストの先行実装で置いたスタブ）。

// errNotImplemented は v1.2.0 のスタブが返すエラー。実装で消す。
var errNotImplemented = errors.New("not implemented (v1.2.0)")

// CodeSource は ref のコードブロック・図の、書き換える範囲のソースを返す（編集欄の初期値。FR-145）。
// 前置き（引用の `> ` やリストの字下げ）は除き、改行は LF にそろえる。前置きが行ごとに違えば ErrLocked。
//
// TODO(v1.2.0): 未実装。
func CodeSource(r *renderer.Renderer, raw []byte, kind mdfile.Kind, ref Ref) (string, error) {
	return "", errNotImplemented
}

// PlanCode は ref のコードブロック・図の中身を text に置き換える Patch を返す（FR-145）。
//
// TODO(v1.2.0): 未実装。
func PlanCode(r *renderer.Renderer, raw []byte, kind mdfile.Kind, ref Ref, text string) (p Patch, changed bool, err error) {
	return Patch{}, false, errNotImplemented
}
