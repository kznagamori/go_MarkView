package opener

// openCommand は Windows で既定アプリケーションへ委譲するコマンドを返す
// （IMP-170）。
//
// cmd /c start は使わない。クォートの扱いが引数の内容によって変わり、
// 空白や & を含むパスで誤動作しうるためである。
func openCommand(target string) (name string, args []string) {
	return "rundll32.exe", []string{"url.dll,FileProtocolHandler", target}
}
