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
	// **編集欄が開いている間は、開かずに拒み、ツリールートも変えない**（FR-140, IMP-313。4.75.0）。
	// Linux ではドロップが JavaScript を通らずにここへ届くため、フロントエンドだけでは止められない
	// （IMP-313 の IMPORTANT）。値はフロントエンドが SetEditBoxOpen で知らせたもの。
	if a.editBoxIsOpen() {
		a.emit(eventError, editBusyDTO())
		return
	}

	root := dropRoot(paths)
	if root != "" {
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
		// **ディレクトリに README が無いだけなら、ツリールートを移したところで終える。** 表示を変えず、
		// 通知もしない（FR-011 の表の 2 行目。4.89.0）。v1.0.0 から、ここで「開けないファイル」を
		// ステータスに出していた（起動時の引数でディレクトリを渡したときは出さない。E2E-214）。
		if root != "" {
			return
		}

		// 開けるファイルでもディレクトリでもない。ツリーも本文も変えず、
		// 対応していない旨をステータスへ出す（FR-011 の表の 4 行目）。
		a.emit(eventError, newErrorDTO(firstPath(paths), document.ErrNotSupported))
		return
	}

	dto, err := a.open(openRequest{path: target, src: openFromDrop})
	if err != nil {
		a.emit(eventError, newErrorDTO(target, err))
		return
	}

	a.emit(eventDocumentOpened, dto)
}

// editBoxIsOpen は、フロントエンドで編集欄が開いているかを返す（App.editBoxOpen。IMP-190, IMP-313）。
func (a *App) editBoxIsOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.editBoxOpen
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
// **判定は Go 側で行う**（IMP-300）。対象は開けるファイル（Markdown・図・画像。
// mdfile.IsOpenable。4.75.0）であり、複数渡された場合は先頭の開けるファイルを採り、
// 他は無視する。ディレクトリなら直下の README を探す（**README は Markdown のまま**。
// FR-013 と同じ探し方。IMP-193）。対象がなければ空文字を返す。
func dropTarget(paths []string) string {
	for _, p := range paths {
		if mdfile.IsOpenable(p) {
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
