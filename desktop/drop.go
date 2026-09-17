package desktop

import (
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/session"
)

// 本ファイルはファイルのドロップ（FR-011, IMP-313）を持つ。
//
// ドロップはバインドメソッドではなく、Wails のコールバック（runtime.OnFileDrop）で受け取る。

// onFileDrop はドロップされたパスを処理する（IMP-313, FR-011）。
//
// 結果は document:opened で送る。フロントエンドの呼び出しではなく OS の
// 操作で表示対象が変わるためである（IMP-320）。
func (a *App) onFileDrop(paths []string) {
	if root := dropRoot(paths); root != "" {
		a.mu.Lock()
		changed := a.setTreeRoot(root)
		newRoot := a.treeRoot
		a.mu.Unlock()

		if changed {
			a.emit(eventTreeRootChanged, newRoot)
		}
	}

	target := dropTarget(paths)
	if target == "" {
		// Markdown でもディレクトリでもない。ツリーも本文も変えず、
		// 対応していない旨をステータスへ出す（FR-011 の表の 4 行目）。
		a.emit(eventError, newErrorDTO(firstPath(paths), document.ErrNotMarkdown))
		return
	}

	dto, err := a.open(openRequest{path: target, src: openFromDrop})
	if err != nil {
		a.emit(eventError, newErrorDTO(target, err))
		return
	}

	a.emit(eventDocumentOpened, dto)
}

// firstPath は一覧の先頭を返す。空なら空文字。
func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// dropTarget はドロップされたパスから開く対象を選ぶ（IMP-313, FR-011）。
//
// **判定は Go 側で行う**（IMP-300）。複数渡された場合は先頭の Markdown を
// 採り、他は無視する。ディレクトリなら直下の README を探す。対象がなければ
// 空文字を返す。
func dropTarget(paths []string) string {
	for _, p := range paths {
		if mdfile.IsMarkdown(p) {
			return p
		}
	}

	// ディレクトリは 1 つだけ渡された場合に限って扱う。複数のディレクトリから
	// 1 つを選ぶ規則を FR-011 は定めていない。
	if root := dropRoot(paths); root != "" {
		if readme, ok := session.FindReadme(root); ok {
			return readme
		}
	}

	return ""
}

// dropRoot はドロップでツリールートにすべき場所を返す（FR-011, FR-030）。
//
// ディレクトリが 1 つだけ落とされた場合に限る。**README が見つからなくても
// ツリールートは移す。** FR-011 の表の 2 行目は「そのディレクトリを新しい
// ツリールートとし、直下に README.md があれば表示する」であり、表示できるか
// どうかとツリーの移動は別である。
//
// ファイルが落とされた場合は空文字を返す。ファイルのツリールートは open が
// 親ディレクトリとして決める（IMP-192）。
func dropRoot(paths []string) string {
	if len(paths) != 1 {
		return ""
	}

	info, err := os.Stat(paths[0])
	if err != nil || !info.IsDir() {
		return ""
	}

	return filepath.Clean(paths[0])
}
