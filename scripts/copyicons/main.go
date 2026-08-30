// copyicons は assets/ のアイコンを、Wails が参照する build/ 配下へ複製する（BR-013）。
//
// Wails v2 はアイコンの参照先を build/ 配下に固定しており、任意のパスを指定する
// 設定を持たない。そのため「原本を assets/ に置き、ビルドの前に複製する」構成を採る。
// assets/ を唯一の正とし、build/ 配下は常にそこから作られた複製として扱う。
//
// 実行はローカルビルドとリリース CI の両方で行う必要がある（BR-013）。
// 片方でしか行わないと、手元と配布物でアイコンが食い違う。忘れようがないよう、
// wails.json の preBuildHooks から自動で呼ばれるようにしてある。
//
// 使い方:
//
//	go run github.com/kznagamori/go_MarkView/scripts/copyicons
//
// Wails のビルドフックはカレントディレクトリを build/bin にして実行するため、
// 本プログラムは go.mod を上へ辿ってリポジトリの根を自分で見つける。
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// 複製の対象。icon.icns は macOS 用であり、本バージョンの対象外のため含めない（BR-013）。
var targets = []struct {
	src string
	dst string
}{
	{src: "assets/icon.png", dst: "build/appicon.png"},      // 共通（Linux のウィンドウ・タスクバー）
	{src: "assets/icon.ico", dst: "build/windows/icon.ico"}, // Windows の実行ファイル用リソース
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "copyicons: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}

	for _, t := range targets {
		src := filepath.Join(root, filepath.FromSlash(t.src))
		dst := filepath.Join(root, filepath.FromSlash(t.dst))

		changed, err := copyIfDifferent(src, dst)
		if err != nil {
			return err
		}
		if changed {
			fmt.Printf("copyicons: %s -> %s\n", t.src, t.dst)
		}
	}
	return nil
}

// moduleRoot はカレントディレクトリから上へ辿り、go.mod のあるディレクトリを返す。
//
// 呼び出し元のカレントディレクトリに依存しないようにするための処理である。
// Wails のビルドフックは build/bin から実行される。
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine the working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above the working directory")
		}
		dir = parent
	}
}

// copyIfDifferent は src を dst へ複製する。内容が同じなら何もしない。
// 戻り値は実際に書き込んだかどうか。
//
// 原本が欠けている場合はエラーとする。既定のアイコンのまま配布される事故を
// 防ぐため、黙って読み飛ばさない（UI-025）。
func copyIfDifferent(src, dst string) (bool, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return false, fmt.Errorf("cannot read the source icon: %w", err)
	}

	if current, err := os.ReadFile(dst); err == nil && bytes.Equal(current, data) {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, fmt.Errorf("cannot create the destination directory: %w", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return false, fmt.Errorf("cannot write the destination icon: %w", err)
	}
	return true, nil
}
