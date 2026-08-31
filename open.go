package main

import (
	"errors"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/session"
)

// 本ファイルは文書を開く共通処理を持つ（IMP-192）。
//
// **FR-010 / FR-011 / FR-012 / FR-033 / FR-050 / FR-051 のすべてが、この 1 つの
// 内部処理を通る**（AR-060）。フロントエンドから任意のパスを開く汎用 API を
// 作らないのと同じ理由で、Go 側にも開く経路を 1 つしか置かない（IMP-300）。

// openSource は文書を開いた経路（IMP-192）。
type openSource int

const (
	openFromDialog  openSource = iota // ファイル選択ダイアログ（FR-010）
	openFromDrop                      // ドラッグ＆ドロップ（FR-011）
	openFromArgs                      // コマンドライン引数（FR-012）
	openFromTree                      // ファイルツリーからの選択（FR-033）
	openFromLink                      // 文書内リンク（FR-050）
	openFromHistory                   // 履歴移動（FR-051）
	openFromReload                    // 再読み込み・更新検知（FR-014, FR-015）
	openFromConfirm                   // 確認画面の Open anyway（FR-016）
)

// openRequest は open への指示。
//
// IMP-192 は open(path, src, opts) の 3 引数で定めるが、アンカーと復元位置を
// 渡す必要があるため構造体にした。位置引数が 5 つ並ぶと、呼び出し側で
// どの値がどの経路のためのものか読み取れなくなる。
type openRequest struct {
	path string
	src  openSource

	// anchor はアンカー付きリンク（./a.md#sec）を踏んだときの見出し ID。
	anchor string

	// scrollTop は openFromHistory で復元するスクロール位置（FR-051）。
	scrollTop int

	// confirmed は FR-016 の「Open anyway」。10 MB 超でも描画する。
	confirmed bool
}

// open は文書を開く唯一の内部処理（IMP-192）。
//
// 経路による差異は IMP-192 の表に挙がった 3 つ（ツリールート・履歴・
// スクロール）だけとする。**この表以外の差異を持ち込まない。** 分岐が増えると、
// リンク遷移でツリールートが動かないという FR-030 の不変条件を壊しやすくなる。
func (a *App) open(req openRequest) (*DocumentDTO, error) {
	// 読み込みと変換はロックの外で行う。10 MB 近い文書では時間がかかり、
	// その間ほかのバインドメソッドを止める理由がない。renderer は状態を
	// 持たず、同時に呼んでよい（IMP-024）。
	opts := document.LoadOptions{Confirmed: req.confirmed}

	doc, err := document.Load(a.renderer, req.path, opts)
	if err != nil {
		newRoot := a.commitPending(req, err)

		// 状態画面に対象ファイル名が出る場合は、タイトルもそれに合わせる。
		// 「描画したかどうか」ではなく「いま何を開こうとしているか」を示す
		// （UI-013）。読めなかっただけの場合は操作案内に戻るため触らない。
		if stateKindFor(newErrorDTO(req.path, err)) != stateWelcome {
			a.setWindowTitle(filepath.Base(req.path))
		}
		if newRoot != "" {
			a.emit(eventTreeRootChanged, newRoot)
		}
		return nil, err
	}

	dto, newRoot := a.commitOpen(doc, req)

	// Wails の呼び出しはロックを解いてから行う（IMP-024）。
	a.setWindowTitle(dto.Name)
	if newRoot != "" {
		a.emit(eventTreeRootChanged, newRoot)
	}

	return dto, nil
}

// commitOpen は読み込んだ文書を状態へ反映し、DTO を組み立てる（IMP-192）。
//
// 戻り値の newRoot は、ツリールートが変わった場合のみ非空である。イベントの
// 送出は呼び出し側がロックの外で行う。
func (a *App) commitOpen(doc *document.Document, req openRequest) (dto *DocumentDTO, newRoot string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// ツリールートは、ファイルの出どころが利用者の明示的な指定である経路
	// でのみ変える（FR-030）。
	if changesTreeRoot(req.src) && a.setTreeRoot(filepath.Dir(doc.Path)) {
		newRoot = a.treeRoot
	}

	if pushesHistory(req.src) {
		a.history.Push(session.Entry{Path: doc.Path, Anchor: req.anchor})
	}

	a.current = doc
	a.watchCurrent(doc.Path)

	// 開けたので確認待ちは解消した。覚えたままにすると、まったく別の文書を
	// 表示している状態で OpenConfirmed が通ってしまう（IMP-314）。
	a.pendingConfirm = ""

	// 表示用パスは都度算出する。保持すると、ツリールートが変わったときに
	// 古い値が残る（IMP-025）。
	display, outside := session.DisplayPath(a.treeRoot, doc.Path)

	return newDocumentDTO(doc, display, outside, scrollFor(req)), newRoot
}

// commitPending は読み込みに失敗した場合の状態を反映する（IMP-314, FR-016）。
//
// 確認待ち（10 MB 超）のときは、**描画しないままツリールートと履歴だけを
// 対象へ移す**。FR-016 は「確認画面を表示した時点でタイトルとパス表示を対象の
// ものに更新し、履歴に積む。Alt+← で直前の文書へ戻れること」を求めており、
// 履歴に積まないと戻る先が 1 つずれる。適用する規則は成功時と同じ表に従う。
//
// **監視は張らない。** 描画を始めていないファイルは FR-014 の対象外である。
// 同じ理由で current も差し替えない。まだその文書を表示していない。
//
// 確認以外の失敗では覚えていた値を捨てる。残したままにすると、確認画面を
// 閉じたあとの操作で OpenConfirmed が通ってしまう。
func (a *App) commitPending(req openRequest, err error) (newRoot string) {
	var sizeErr *document.SizeError

	a.mu.Lock()
	defer a.mu.Unlock()

	if !errors.As(err, &sizeErr) || !errors.Is(sizeErr.Err, document.ErrNeedsConfirm) {
		a.pendingConfirm = ""
		return ""
	}

	a.pendingConfirm = sizeErr.Path
	a.pendingSource = req.src

	if changesTreeRoot(req.src) && a.setTreeRoot(filepath.Dir(sizeErr.Path)) {
		newRoot = a.treeRoot
	}
	if pushesHistory(req.src) {
		a.history.Push(session.Entry{Path: sizeErr.Path, Anchor: req.anchor})
	}

	return newRoot
}

// changesTreeRoot はツリールートを変更する経路かを返す（IMP-192, FR-030）。
//
// **ツリーからの選択とリンク遷移では変更しない。** ドキュメント群を配布した
// ときに、利用者の操作でツリーが意図せず移動することを防ぐ（FR-030, FR-052）。
// openResult は open を呼び、結果を OpenResultDTO へ写す（IMP-192, IMP-308）。
//
// バインドメソッドはこちらを使う。open そのものは Go 側の内部処理であり、
// イベント送出（app.go）からも呼ばれるため error のまま残す。
func (a *App) openResult(path string, req openRequest) OpenResultDTO {
	dto, err := a.open(req)

	return newOpenResult(path, dto, err)
}

func changesTreeRoot(src openSource) bool {
	switch src {
	case openFromDialog, openFromDrop, openFromArgs:
		return true
	default:
		// openFromConfirm はここに含めない。確認画面を出した時点で
		// 変更済みである（commitPending）。
		return false
	}
}

// pushesHistory は履歴に積む経路かを返す（IMP-192, FR-051）。
//
// 履歴移動そのものと再読み込みでは積まない。積むと、戻るたびに履歴が伸びて
// 戻れなくなる。
func pushesHistory(src openSource) bool {
	switch src {
	case openFromHistory, openFromReload, openFromConfirm:
		// openFromConfirm は確認画面を出した時点で積み済みである
		// （commitPending）。ここで積むと同じ文書が 2 つ並ぶ。
		return false
	default:
		return true
	}
}

// scrollFor は描画後のスクロール指示を決める（IMP-192, IMP-302）。
//
// スクロールの扱いは経路に依存するため Go 側が決め、フロントエンドに経路を
// 意識させない。
func scrollFor(req openRequest) ScrollDTO {
	switch req.src {
	case openFromHistory:
		// 記録された位置を復元する（FR-051）。アンカーで開いた文書でも、
		// 離れる直前に記録した実際の位置のほうが正確である（IMP-311）。
		return ScrollDTO{Mode: scrollRestore, Top: req.scrollTop}

	case openFromReload:
		// 位置の出どころはフロントエンドが持つ現在値である。Go 側は
		// Top を設定しない（FR-014, IMP-321）。
		return ScrollDTO{Mode: scrollKeep}

	default:
		if req.anchor != "" {
			return ScrollDTO{Mode: scrollAnchor, Anchor: req.anchor}
		}
		return ScrollDTO{Mode: scrollTop}
	}
}
