//go:build !windows

package opener

// openCommand は Linux で既定アプリケーションへ委譲するコマンドを返す
// （IMP-170）。
//
// xdg-open が存在しない環境ではプロセスの起動に失敗し、その旨が
// 呼び出し側へ返る。ステータス表示で利用者へ伝える（FR-110）。
func openCommand(target string) (name string, args []string) {
	return "xdg-open", []string{target}
}
