package filetree

import (
	"errors"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
)

// このファイルは v1.2.0 の表示する種類（IMP-134）とツリーの操作の検査（IMP-135）の宣言である。
// TODO(v1.2.0): 中身は未実装（テストの先行実装で置いたスタブ）。

// errNotImplemented は v1.2.0 のスタブが返すエラー。実装で消す。
var errNotImplemented = errors.New("not implemented (v1.2.0)")

// Show はツリーに表示する種類（FR-037）。Markdown は常に表示する（外せない）。
type Show struct {
	Diagrams bool // 図のファイル（PlantUML・Mermaid）
	Images   bool // 画像ファイル
}

// Includes は種類 k のファイルを表示するかを返す。KindNone は常に偽、KindMarkdown は常に真。
//
// TODO(v1.2.0): 未実装。
func (s Show) Includes(k mdfile.Kind) bool {
	return false
}

// Want は操作が求める種類（FR-036）。
type Want int

const (
	WantOpenable Want = iota // 開く（FR-033、メニューの Open）。開けるファイル
	WantText                 // Open in editor...。文字のファイル（画像を含めない）
	WantDir                  // 展開・Open in file manager・Set as tree root。ディレクトリ
	WantAny                  // Copy path。表示しているファイルかディレクトリ
)

var (
	ErrNotInTree = errors.New("path is not shown in the tree") // 何もしない（通知しない）
	ErrMissing   = errors.New("path no longer exists")         // not-found を出す（FR-036）
)

// Check は path がいまのツリーに出ているもので、want の種類であるかを確かめ、
// filepath.Clean したパスを返す。allowRoot が真なら、ツリールートそのものも受け付ける（IMP-135）。
//
// TODO(v1.2.0): 未実装。
func Check(root, path string, show Show, want Want, allowRoot bool) (string, error) {
	return "", errNotImplemented
}

// Classify は path の種類を、ツリーの表示（IMP-131）と同じ規則で決める（IMP-135）。
//
// TODO(v1.2.0): 未実装。
func Classify(path string) (isDir bool, kind mdfile.Kind, err error) {
	return false, mdfile.KindNone, errNotImplemented
}
