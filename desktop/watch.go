package desktop

import (
	"context"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/session"
	"github.com/kznagamori/go_MarkView/internal/watcher"
)

// 本ファイルは、表示中ファイルの監視（FR-014, IMP-140）のイベントを受け取る。
//
// **受け取ったイベントは、画面がいま表示している文書のものかを確かめてから扱う**
// （IMP-192 の「イベントの照合」）。監視のイベントは ioMu を待つ間に古くなりうる。

// startWatcher はファイル監視を開始する（FR-014, IMP-140, IMP-024）。
//
// 監視は ctx の Done で終わる。ゴルーチンをアプリのライフサイクルへ紐付け、
// 終了時に必ず止める（NFR-020）。
//
// 監視を作れなくても起動は続ける。自動更新が効かなくなるだけで、利用者は
// 再読み込みできる（FR-015, FR-111）。
func (a *App) startWatcher(ctx context.Context) {
	w, err := watcher.New(ctx)
	if err != nil {
		return
	}

	// **開く処理と同じ ioMu の内側で置く。** 錠の外で watcher を置いてから current を読むと、
	// その間に別の文書を開く処理が監視を張り、ここで古い文書へ張り直してしまう（IMP-192）。
	// onStartup は Wails がゴルーチンで呼ぶため、ここで待っても UI スレッドは止まらない。
	a.ioMu.Lock()
	a.mu.Lock()
	a.watcher = w
	current, showing := a.current, a.showing
	a.mu.Unlock()

	// onStartup より前に文書を開いていた場合、その時点では watcher が
	// なく監視を張れていない。ここで追いつかせる。**状態画面を出していれば張らない**
	// （FR-016 の「確認画面を表示しているだけの状態では監視しない」。BUG-012）。
	if current != nil && showing {
		_ = w.Watch(current.Path)
	}
	a.ioMu.Unlock()

	go a.consumeWatchEvents(w)
}

// consumeWatchEvents は監視イベントをフロントエンドへ流す（FR-014, IMP-320）。
//
// チャネルは監視の終了時に閉じるため、range で待てる（IMP-140）。
func (a *App) consumeWatchEvents(w *watcher.Watcher) {
	for ev := range w.Events() {
		switch ev.Kind {
		case watcher.Modified:
			a.onCurrentModified(ev.Path)
		case watcher.Removed:
			a.onCurrentRemoved(ev.Path)
		}
	}
}

// onCurrentModified は表示中ファイルの更新を反映する（FR-014, IMP-321）。
//
// **スクロール位置はフロントエンドが持つ現在値を使う**（keep）。表示を変えない失敗
// （読めなかった）では、エラーだけを送り直前の描画結果を残す（FR-110）。**状態画面になる
// 失敗（しきい値をまたいだ・変換に失敗した）も error イベントで送る**——Go 側は既に状態画面へ
// 移っており（IMP-192）、フロントエンドは種別を見て状態画面を出す（IMP-320）。
func (a *App) onCurrentModified(path string) {
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	current, ok := a.watchedCurrent(path)
	if !ok {
		return
	}

	dto, err := a.openLocked(openRequest{path: current.Path, src: openFromReload, trigger: triggerWatch})
	switch {
	case err != nil:
		a.emit(eventError, newErrorDTO(current.Path, err))
	case dto != nil:
		a.emit(eventDocumentChanged, dto)
	default:
		// 読み直した内容が表示中のものと同じだった（IMP-321）。書き込みの直後に Go 側が
		// 送った document:changed の後から届くイベントであり、同じ再描画を繰り返さない。
	}
}

// onCurrentRemoved は表示中ファイルの削除を伝える（FR-014, FR-110, IMP-195）。
//
// **直前の描画結果は保持する。** 本文を消すと、編集の途中でファイルが一瞬
// 消えるような保存方式のたびに画面が空になる。
//
// **編集モードも同時に終え、削除の印を立てる**（IMP-109 の Removed。次に読み込めるまで開始
// できない）。
func (a *App) onCurrentRemoved(path string) {
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	current, ok := a.watchedCurrent(path)
	if !ok {
		return
	}

	a.edit.Removed()
	a.emit(eventDocumentRemoved, removedErrorDTO(current.Path))
}

// watchedCurrent は、監視のイベントが画面の表示している文書のものかを確かめ、そうなら
// その文書を返す（IMP-192 の「イベントの照合」, IMP-195）。**呼び出し元は ioMu を持っていること。**
//
// **ioMu を取った後で確かめる。** ioMu を待つ間に別の文書を開く処理が済んでいると、古い
// イベントが後から届く。確かめないと、表示中の B に対して A を開き直して B の表示が A に戻る。
// 削除なら、B に削除の印を立てて B で編集を始められなくなる。
//
// **状態画面の間（showing が偽）のイベントは捨てる。** v1.0.0 はイベントのパスをそのまま
// 開き直しており、確認画面が前の文書の表示に置き換わっていた（BUG-012）。
func (a *App) watchedCurrent(path string) (*document.Document, bool) {
	a.mu.Lock()
	current, showing := a.current, a.showing
	a.mu.Unlock()

	// SameFile はシンボリックリンクを解くためにファイルシステムに触れる。mu の外で呼ぶ。
	if !showing || current == nil || !session.SameFile(path, current.Path) {
		return nil, false
	}

	return current, true
}

// switchWatch は監視を結果に合わせる（FR-014, FR-016, IMP-140, IMP-192）。
//
// **mu の外で呼ぶ**（ioMu の内側ではある）。監視は常に 1 つ以下とする。Watch が切り替えまで
// 面倒を見るため、成功時に Unwatch を挟まない。
//
// **状態画面を出したら、前の文書の監視を外す。** v1.0.0 は外しておらず、確認画面の間に前の
// 文書が外部で更新されると、確認画面が前の文書の表示に置き換わっていた（BUG-012）。
//
// Watch に失敗しても開く操作は成功とする。自動更新が効かなくなるだけで、利用者は再読み込み
// できる（FR-015, FR-111）。
func switchWatch(w *watcher.Watcher, outcome openOutcome, target string) {
	// 起動の直後、onStartup が監視を作る前に開いた場合は nil である。startWatcher が追いつかせる。
	if w == nil {
		return
	}

	switch outcome {
	case outcomeShown:
		_ = w.Watch(target)
	case outcomeState:
		w.Unwatch()
	}
}
