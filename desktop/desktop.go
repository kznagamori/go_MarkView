// Package desktop は Wails との境界である（IMP-011, IMP-012）。
//
// Wails にバインドする App 型と、そのバインドメソッド・境界の型（DTO）・エラーの分類・
// 監視とドロップの受け口を持つ。**Wails の API を呼んでよいのは main.go とこのパッケージ
// だけ**とし、internal/ の各パッケージは Wails に依存させない（IMP-012）。
//
// **判断を伴うロジックはここに置かず internal/session・internal/document などへ委ねる。**
// このパッケージは単体テストの対象外であり（UT-002）、ここに書いたロジックのテストには
// Wails（Linux では cgo と WebKitGTK）がリンクされてしまう。
//
// internal/ の外に置くのは、internal/ の各パッケージが Wails に依存しないという規則
// （IMP-012）を、ディレクトリの名前だけで読み取れるようにするためである。
package desktop

import "context"

// Lifecycle は Wails の起動オプションへ渡すライフサイクルの関数である（IMP-193, IMP-194）。
//
// **App のメソッドとして公開しない。** Wails は Bind に渡した値の公開メソッドをすべて
// フロントエンドから呼べるようにし、options.App の OnStartup などに渡された関数だけを
// 関数名で照合して外す。公開メソッドにすると、main.go で包んで渡しただけで照合が外れ、
// JavaScript から起動・終了の処理を呼べる経路ができる（IMP-300, IMP-310。wails generate
// module で確かめた）。非公開のメソッドを関数の値として取り出し、main.go が options.App へ渡す。
type Lifecycle struct {
	OnStartup     func(ctx context.Context)      // ウィンドウ生成の後
	OnBeforeClose func(ctx context.Context) bool // 閉じる直前（IMP-194）
	OnShutdown    func(ctx context.Context)      // ウィンドウ破棄の後（IMP-194）
}

// LifecycleOf は a のライフサイクルの関数を返す。
func LifecycleOf(a *App) Lifecycle {
	return Lifecycle{
		OnStartup:     a.onStartup,
		OnBeforeClose: a.onBeforeClose,
		OnShutdown:    a.onShutdown,
	}
}
