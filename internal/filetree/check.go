package filetree

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
)

// このファイルは v1.2.0 の表示する種類（IMP-134）と、種類の判定とツリーの操作の検査（IMP-135）を持つ。

// Show はツリーに表示する種類（FR-037）。Markdown は常に表示する（外せない）。
type Show struct {
	Diagrams bool // 図のファイル（PlantUML・Mermaid）
	Images   bool // 画像ファイル
}

// Includes は種類 k のファイルを表示するかを返す。KindNone は常に偽、KindMarkdown は常に真（IMP-134）。
func (s Show) Includes(k mdfile.Kind) bool {
	switch k {
	case mdfile.KindMarkdown:
		return true
	case mdfile.KindPlantUML, mdfile.KindMermaid:
		return s.Diagrams
	case mdfile.KindImage:
		return s.Images
	default:
		return false
	}
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
// **フロントエンドが渡すパスは、形が正しくても信じない**（NFR-030, NFR-035）。検査の順序は IMP-135 の表の
// とおりで、どこで落ちても呼び出し側は何も起動しない。ファイルシステムに触れるのは 4（Classify）だけで、
// 1〜3 はパスの文字列だけで決める。2 と 3 は**ツリー上のパス**（シンボリックリンクを解決しない）で見る——
// ツリールートの外を指すリンクも、ツリーに出ているものなら受け付ける（4.74.0 の利用者の判断）。
func Check(root, path string, show Show, want Want, allowRoot bool) (string, error) {
	// 1. root が空でなく、path が絶対パス。
	if root == "" || !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s: %w", path, ErrNotInTree)
	}

	// 2. ツリー上のパスがツリールートの中（ルートそのものは allowRoot のときだけ）。
	p := filepath.Clean(path)
	rel, err := filepath.Rel(filepath.Clean(root), p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s: %w", p, ErrNotInTree)
	}

	if rel == "." {
		if !allowRoot {
			return "", fmt.Errorf("%s: %w", p, ErrNotInTree)
		}
	} else {
		// 3. ルートからの相対パスの各部分が、ツリーに表示する名前の規則に当てはまる（ルート自身の名前と
		//    それより上には当てはめない）。最後の部分もディレクトリとして当てはめる（4.78.0。UT-308 の 24）。
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if !include(part, true) {
				return "", fmt.Errorf("%s: %w", p, ErrNotInTree)
			}
		}
	}

	// 4. 種類を決める。存在しなければ ErrMissing（not-found を出す）、ほかの失敗はそのまま包む
	//    （権限が無い等。IMP-196 が permission にする。UT-308 の 14）。
	isDir, kind, err := Classify(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("%s: %w", p, ErrMissing)
	case err != nil:
		return "", fmt.Errorf("%s: %w", p, err)
	}

	// 5. 種類が want に合い、ファイルならその種類を show が含む（フィルターで隠した種類を受け付けない）。
	if !wants(want, isDir, kind, show) {
		return "", fmt.Errorf("%s: %w", p, ErrNotInTree)
	}
	return p, nil
}

// wants は、種類が操作の求めるものに合うかを返す（IMP-135 の表）。ファイルは show が含む種類に限る。
func wants(want Want, isDir bool, kind mdfile.Kind, show Show) bool {
	file := !isDir && show.Includes(kind)
	switch want {
	case WantOpenable:
		return file
	case WantText:
		// 画像はエディタで開かない（FR-027, FR-090）。
		return file && kind != mdfile.KindImage
	case WantDir:
		return isDir
	case WantAny:
		return isDir || file
	}
	return false
}

// Classify は path の種類を、ツリーの表示（IMP-131）と同じ規則で決める（IMP-135）。
//
//   - ディレクトリかファイルかは os.Stat（シンボリックリンクを解決した実体）で決める
//   - **名前付きパイプ・デバイス・ソケットは種類なし**とする（読むと止まる・デバイスを読む。IMP-102 の
//     document の判定と同じ。4.87.0 の利用者の判断）。**`Mode().IsRegular()` では判定しない**——Go 1.23
//     から、Windows の再解析ポイント（OneDrive のクラウドのファイルなど）は ModeIrregular になり、
//     v1.1.0 で出ていたファイルがツリーから消える
//   - ファイルの種類はツリー上の名前の拡張子（mdfile.KindOf）。**シンボリックリンクなら、実体
//     （filepath.EvalSymlinks）の拡張子の種類が同じときに限る**（違えば種類なし。UT-308 の 19）
//
// 調べられなければ（宛先の無いリンクは fs.ErrNotExist、権限が無い等）そのエラーを返す。
func Classify(path string) (isDir bool, kind mdfile.Kind, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, mdfile.KindNone, err
	}
	if info.IsDir() {
		return true, mdfile.KindNone, nil
	}
	if special(info.Mode()) {
		return false, mdfile.KindNone, nil
	}

	kind = mdfile.KindOf(path)
	if kind == mdfile.KindNone {
		return false, kind, nil
	}
	if li, err := os.Lstat(path); err == nil && li.Mode()&fs.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return false, mdfile.KindNone, err
		}
		if mdfile.KindOf(real) != kind {
			return false, mdfile.KindNone, nil
		}
	}
	return false, kind, nil
}

// special は、ディレクトリでもファイルでもないもの（名前付きパイプ・デバイス・ソケット）か（Classify）。
func special(m fs.FileMode) bool {
	return m&(fs.ModeNamedPipe|fs.ModeDevice|fs.ModeCharDevice|fs.ModeSocket) != 0
}

// classifyEntry は、ディレクトリの読み込みで得た要素の種類を Classify と同じ規則で決める。
// シンボリックリンクでない通常のファイルとディレクトリは、要素の型だけで決まる（os.Stat を呼ばない。
// 1000 件のディレクトリで 1000 回の Stat を避ける）。それ以外（シンボリックリンク・再解析ポイント・
// パイプなど）は Classify に任せる。
func classifyEntry(dir string, e fs.DirEntry) (isDir bool, kind mdfile.Kind, err error) {
	switch t := e.Type(); {
	case t.IsDir():
		return true, mdfile.KindNone, nil
	case t.IsRegular():
		return false, mdfile.KindOf(e.Name()), nil
	default:
		return Classify(filepath.Join(dir, e.Name()))
	}
}
