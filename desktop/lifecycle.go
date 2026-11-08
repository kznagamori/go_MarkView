package desktop

import (
	"context"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/kznagamori/go_MarkView/internal/config"
)

// 本ファイルは Wails のライフサイクルの関数（IMP-193, IMP-194）と、設定の保存（UI-114）を持つ。
// app.go が 400 行の目安（IMP-011）を超えるため分けた。**ライフサイクルの関数は公開メソッドに
// しない**（desktop.go の Lifecycle）。

// saveDebounce は設定保存を待つ時間（UI-114）。
const saveDebounce = time.Second

// onStartup は Wails がウィンドウ生成後に呼ぶ。
func (a *App) onStartup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	// ウィンドウはプライマリモニタの作業領域の中央に置く。位置は保存も
	// 復元もしない（UI-011, UI-111）。モニタ構成が変わっても画面外に
	// 出ないようにするための規定である。
	runtime.WindowCenter(ctx)

	// ドロップの受け口はバインドメソッドではなくコールバックである
	// （IMP-313）。HTML5 の DataTransfer からは絶対パスを得られない。
	runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { a.onFileDrop(paths) })

	a.startWatcher(ctx)
}

// onBeforeClose は Wails がウィンドウを閉じる直前に呼ぶ（IMP-194）。
//
// **ウィンドウの大きさはここで取り込む。** onShutdown の時点でウィンドウは
// 既に破棄されており、WindowGetSize は DPI を 0 として除算し panic する
// （2026-08-31 に実機で確認）。
//
// false を返して閉じる操作をそのまま通す。確認を挟まない（FR-110）。
func (a *App) onBeforeClose(_ context.Context) bool {
	a.captureWindowState()
	return false
}

// onShutdown は Wails が終了時に呼ぶ（IMP-194）。
//
// 順序は IMP-194 の表のとおり: 保存の予約を止める → **書き込みの途中なら上限つきで待つ** →
// 監視を閉じる → 設定を保存する。
//
// **ここで Wails のランタイムを呼ばない。** ウィンドウは既に破棄されている。
//
// **失敗しても終了を妨げない。** 設定が書けなくても、次回は既定値で起動する
// だけである（UI-113）。
func (a *App) onShutdown(_ context.Context) {
	a.mu.Lock()
	if a.saveTimer != nil {
		a.saveTimer.Stop()
		a.saveTimer = nil
	}
	w := a.watcher
	a.mu.Unlock()

	a.waitForWrites(shutdownWriteWait)

	if w != nil {
		_ = w.Close()
	}

	// 大きさは onBeforeClose で取り込み済みである。
	a.persistConfig()
}

// 終了時に書き込みの途中を待つ上限と、錠を試す間隔（IMP-194）。
const (
	shutdownWriteWait = 2 * time.Second
	shutdownWritePoll = 20 * time.Millisecond
)

// waitForWrites は、ioMu を取れるまで最大 limit だけ待つ（IMP-194, FR-143, NFR-031）。
//
// **待たずに終わると、document.Replace（IMP-107）の途中でプロセスが終わり、一時ファイルが
// 残りうる。** 上限を設けるのは、読み込みと変換（大きな文書では秒単位）で終了を止めないため
// である。Lock で待つと上限を付けられないため、TryLock を短い間隔で試す。
//
// **取れた錠は解かない。** 待った後に新しい書き込みが始まり、その途中で終わるのを防ぐ。
// この後に ioMu を取る呼び出し（監視のイベント・バインドメソッド）は、プロセスの終了まで待つ。
// 上限まで取れなかった場合は、そのまま後の処理へ進む。
func (a *App) waitForWrites(limit time.Duration) bool {
	if a.ioMu.TryLock() {
		return true
	}

	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	poll := time.NewTicker(shutdownWritePoll)
	defer poll.Stop()

	for {
		select {
		case <-deadline.C:
			return false
		case <-poll.C:
			if a.ioMu.TryLock() {
				return true
			}
		}
	}
}

// saveConfig はウィンドウの状態を取り込んでから設定を保存する（IMP-194）。
//
// **ウィンドウが生きている間にだけ呼ぶ。** 保存の予約（UI-114）から使う。
func (a *App) saveConfig() {
	a.captureWindowState()
	a.persistConfig()
}

// persistConfig は現在の設定をディスクへ書く（UI-112, IMP-152）。
//
// 履歴・表示中パス・ツリールートは保存しない（NFR-042）。構造体にフィールドが
// 存在しないため、ここで気をつける必要はない（IMP-150）。
func (a *App) persistConfig() {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()

	_ = config.Save(cfg)
}

// captureWindowState はウィンドウの大きさを設定へ取り込む（IMP-194, UI-110）。
//
// これはフロントエンドから通知されない（ConfigDTO に含まれない。IMP-303）
// ため、保存の直前に Wails のランタイムから読み出す。
//
// **最大化中のサイズは取り込まない。** 最大化中は画面いっぱいの値が返るため、
// 保存すると次回のウィンドウが画面いっぱいの大きさで開く。幅と高さは最大化
// する前の値を保つ。
//
// 最大化しているかどうかも読むが、これは**保存しないと決めるための判定**で
// あって、保存する値ではない。最大化状態そのものを保存しないため、次回は常に
// 通常状態で開く（UI-111, UI-115）。
//
// ウィンドウ位置と最大化状態は保存しない。構造体にフィールドが存在しない
// （IMP-150, UI-111）。
func (a *App) captureWindowState() {
	// ウィンドウの状態を読む API は、ウィンドウの生存に依存する。取りこぼしても
	// 保存そのものは続ける（FR-111, IMP-022）。
	defer recoverQuiet()

	ctx := a.context()
	if ctx == nil {
		return
	}

	if runtime.WindowIsMaximised(ctx) {
		return
	}

	width, height := runtime.WindowGetSize(ctx)

	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.WindowWidth, a.cfg.WindowHeight = width, height
}

// scheduleSave は設定の保存を予約する（UI-114）。
//
// **呼び出し側は mu を保持していること。**
//
// 変更から 1 秒間さらに変更がなければ書く。ペイン幅のドラッグ中は変更が
// 続くため、実際に書かれるのはドラッグを終えたあとになる。
func (a *App) scheduleSave() {
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}

	a.saveTimer = time.AfterFunc(saveDebounce, a.saveConfig)
}
