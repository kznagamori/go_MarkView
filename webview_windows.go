//go:build windows

package main

import (
	"strings"

	"github.com/wailsapp/go-webview2/webviewloader"
)

// webviewVersion は導入されている WebView2 ランタイムの版を返す（UI-100, IMP-181）。
//
// **Wails v2 の公開ランタイム（pkg/runtime）はこれを返さない**が、取得できない
// わけではない。go-webview2 は Wails 自身が WebView2 の起動に使っているパッケージ
// であり、既に依存関係へ入っている。
//
// **internal/ には置かない**（IMP-012）。go-webview2 は Wails 系の Windows 専用
// パッケージであり、internal/ へ入れると単体テストに OS 依存が持ち込まれる（UT-002）。
//
// 取得できない場合は空文字を返す。buildinfo.Environment が当該区画ごと省く
// （IMP-181）。**ここで起動を止めず、エラーも出さない**（FR-111）。
func webviewVersion() string {
	// 引数は「同梱した WebView2 を使う」ための指定である。MarkView は環境へ
	// 導入済みのランタイムを使うため空文字を渡す。ランタイムが見つからない
	// ときは ("", nil) が返る。
	v, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString("")
	if err != nil {
		return ""
	}

	// 空白だけの値を返させない。「WebView2 」とだけ書かれた区画は情報として
	// 役に立たず、IMP-181 が省くと定めているのはその状態である。
	return strings.TrimSpace(v)
}
