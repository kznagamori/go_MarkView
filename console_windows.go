package main

import (
	"os"
	"syscall"
)

// ATTACH_PARENT_PROCESS。AttachConsole に渡すと、親プロセスのコンソールへ繋ぐ。
const attachParentProcess = ^uintptr(0) // (DWORD)-1

// attachConsole は、親プロセスのコンソールへ標準出力と標準エラーを繋ぎ直す。
//
// Wails は Windows 向けに **GUI サブシステム**の実行ファイルを作る。GUI
// サブシステムのプロセスはコンソールから起動されてもコンソールに接続されず、
// GetStdHandle が NULL を返す。この状態で標準出力へ書いても行き先がないため、
// 利用者がターミナルで `MarkView --version` と打っても何も表示されない。
// FR-012 が求める「標準出力に出力して終了する」を満たすために繋ぎ直す。
//
// 呼ぶのは --version / --help / 未知のオプションを処理する直前に限る。
// 通常の起動（ウィンドウを開く経路）では呼ばない。コンソールから起動された
// アプリがコンソールを掴み続けると、ターミナルを閉じられなくなるためである。
func attachConsole() {
	if stdoutConnected() {
		// すでに有効な出力先がある。リダイレクトやパイプで起動された場合が
		// これにあたる。ここで CONOUT$ に繋ぎ替えると、リダイレクト先ではなく
		// コンソールへ出てしまうので何もしない。
		return
	}

	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if r, _, _ := proc.Call(attachParentProcess); r == 0 {
		// 親にコンソールがない（エクスプローラから起動された等）。
		// 出力先がないだけで異常ではないため、そのまま続ける。
		return
	}

	// AttachConsole は Go の os.Stdout を差し替えないため、自分で開き直す。
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
}

// stdoutConnected は標準出力に有効なハンドルがあるかを返す。
func stdoutConnected() bool {
	h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return false
	}
	return h != 0 && h != syscall.InvalidHandle
}
