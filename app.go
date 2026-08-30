package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/kznagamori/go_MarkView/internal/session"
)

// App は Wails にバインドする唯一の型である（IMP-190）。
//
// Wails の API を呼べるのは main.go と本ファイルだけとし、internal/ の各
// パッケージは Wails に依存させない（IMP-012）。また、判断を伴うロジックは
// ここに置かず internal/session などへ委ねる。ここに書いたロジックは
// package main のテストとなり、テストバイナリに Wails（Linux では cgo と
// WebKitGTK）がリンクされてしまうためである（UT-002）。
//
// TODO(T3-10): IMP-190 が定める状態（renderer, watcher, cfg, treeRoot,
// current, history, pendingConfirm）と排他制御を持たせる。
// TODO(T3-11): IMP-310 のバインドメソッド 13 種と IMP-320 のイベント 5 種。
type App struct {
	ctx context.Context

	// startup は起動時に決定した表示対象とツリールート（IMP-193）。
	// T1-6 の時点では保持するだけで、まだ描画に使っていない。
	startup session.Startup
}

// NewApp は App を生成する。startup は main.go が解決した結果を渡す。
func NewApp(startup session.Startup) *App {
	return &App{startup: startup}
}

// onStartup は Wails がウィンドウ生成後に呼ぶ。
func (a *App) onStartup(ctx context.Context) {
	a.ctx = ctx

	// ウィンドウはプライマリモニタの作業領域の中央に置く。位置は保存も
	// 復元もしない（UI-011, UI-111）。モニタ構成が変わっても画面外に
	// 出ないようにするための規定である。
	runtime.WindowCenter(ctx)
}

// TODO(T3-12): onShutdown で watcher の停止と config.Save を行う（IMP-194）。
