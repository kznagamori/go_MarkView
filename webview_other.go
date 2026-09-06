//go:build !windows

package main

/*
#cgo !webkit2_41 pkg-config: webkit2gtk-4.0
#cgo webkit2_41 pkg-config: webkit2gtk-4.1
#include "webkit2/webkit2.h"
*/
import "C"

import "fmt"

// webviewVersion は実行時にリンクされている WebKitGTK の版を返す（UI-100, IMP-181）。
//
// **Linux こそ表示する価値が大きい。** docs/troubleshooting.md の最初の項目は
// 「WebKitGTK 4.1 が入っていません」であり、利用者に「あなたの環境は何版か」を
// 見せられるのは情報ウィンドウだけである。
//
// pkg-config の指定は Wails 本体と同じ形にする。**-tags webkit2_41 を付けた
// ビルドは 4.1 に、付けないビルドは 4.0 にリンクされる**（AR-003, BR-010）。
// ここだけ別の系統へ繋ぐと、本体と違う版を表示することになる。
//
// **internal/ には置かない**（IMP-012）。cgo と OS 依存を internal/ へ持ち込むと
// 単体テストが GUI 環境を要求するようになる（UT-002）。
//
// ビルドタグは console_other.go と同じく !windows とする。対応 OS は Windows と
// Linux の 2 つだけであり（AR-001）、それ以外の環境向けにはビルドしない。
func webviewVersion() string {
	// 3 つの関数はライブラリの版を返すだけで、失敗しない。
	return fmt.Sprintf("%d.%d.%d",
		C.webkit_get_major_version(),
		C.webkit_get_minor_version(),
		C.webkit_get_micro_version(),
	)
}
