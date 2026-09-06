//go:build !windows

package main

// attachConsole は Windows 以外では何もしない。
//
// GUI サブシステムという区分は Windows 固有のものであり、Linux では
// 実行ファイルを起動したシェルの標準出力がそのまま繋がる。
// 実装の対応は console_windows.go にある。
func attachConsole() {}
