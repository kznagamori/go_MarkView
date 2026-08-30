// MarkView は Markdown の閲覧に特化した軽量デスクトップアプリケーションである。
//
// 本ファイルはエントリポイントであり、担うのは以下の 3 つに限る。
//
//   - コマンドラインオプションの解析（FR-012）
//   - 実行環境に依存する値（カレントディレクトリ、実行ファイルの位置）の取得
//   - Wails の起動
//
// 表示対象とツリールートを決める判断は internal/session が持つ（IMP-193）。
// ここに判断を書くと package main のテストとなり、テストバイナリに Wails
// （Linux では cgo と WebKitGTK）がリンクされてしまう（IMP-012, UT-002）。
package main

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"github.com/kznagamori/go_MarkView/internal/buildinfo"
	"github.com/kznagamori/go_MarkView/internal/session"
)

// 埋め込み資産（IMP-030）。all: を付け、_ や . で始まるファイルも含める。
//
//go:embed all:frontend
var frontendFS embed.FS

// アプリケーションアイコン（IMP-032, UI-025）。
//
// Linux ではウィンドウとタスクバーのアイコンとして Wails のオプションへ渡す。
// Windows は実行ファイルのリソースから OS が解決するため、icon.ico は
// 埋め込まない。二重に持つとサイズを無駄に増やす（NFR-021）。
//
//go:embed assets/icon.png
var appIconPNG []byte

// exit code。FR-012 が定める。
const (
	exitOK         = 0
	exitRunError   = 1 // ウィンドウを起動できなかった
	exitUsageError = 2 // 未知のオプション（FR-012）
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run は main の本体。終了コードを返す。
//
// 引数と出力先を受け取る形にしているのは、動作を追いやすくするためである。
// ただし本関数は package main に属し、単体テストの対象外とする（UT-002）。
// オプションの扱いは E2E-102 / E2E-103 / E2E-105 が検証する。
func run(args []string, stdout, stderr io.Writer) int {
	action, positional := parseArgs(args)

	switch action {
	case actionVersion:
		printVersion(stdout)
		return exitOK
	case actionHelp:
		printUsage(stdout)
		return exitOK
	case actionUnknownOption:
		// 使用方法は標準エラーへ出す。存在しないパス（起動して案内を出す）と
		// 異なり、オプションの指定誤りは利用者が気づくべき入力の誤りである
		// ため、黙って無視せず終了コードで知らせる（FR-012）。
		printUsage(stderr)
		return exitUsageError
	}

	startup, err := resolveStartup(positional)
	if err != nil {
		// 起動対象が読めなくてもウィンドウは必ず開く（FR-012）。
		// エラーは状態画面 welcome とステータス表示へ渡す（IMP-193）。
		_ = err // TODO(T3-12): InitialStateDTO の Error に載せる（IMP-303）
	}

	if err := launch(startup); err != nil {
		fmt.Fprintf(stderr, "MarkView: %v\n", err)
		return exitRunError
	}
	return exitOK
}

// launch は Wails のウィンドウを起動する。戻るのはウィンドウを閉じた後。
//
// ウィンドウのサイズと最小サイズは UI-011 が定める値。位置は設定に保存せず、
// 常にプライマリモニタの中央に置く（UI-111。中央寄せは App.onStartup）。
//
// TODO(T3-12): 設定（UI-110）を読み、保存されたウィンドウサイズと
// 最大化状態を反映する。テーマの解決結果も渡す（IMP-303）。
func launch(startup session.Startup) error {
	app := NewApp(startup)

	return wails.Run(&options.App{
		Title:     "MarkView", // 文書未表示時のタイトル（UI-013）
		Width:     1280,
		Height:    860,
		MinWidth:  640,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: frontendFS,
			// TODO(T3-5): assetsrv.Handler を Middleware / Handler として
			// 組み込み、ローカル画像と /appicon.png を配信する（IMP-160）。
		},
		OnStartup: app.onStartup,
		Bind:      []any{app},
		// Windows は実行ファイルのリソースからアイコンを取るため指定不要。
		// Linux は埋め込んだ PNG を明示的に渡す（IMP-032）。
		Linux: &linux.Options{Icon: appIconPNG},
	})
}

// action は解析結果として選ばれる動作。
type action int

const (
	actionRun action = iota
	actionVersion
	actionHelp
	actionUnknownOption
)

// parseArgs はコマンドライン引数を解析する（FR-012）。
//
// 判定できるオプションが現れた時点でそれを採る。したがって
// "--version --nosuch" はバージョンを表示し、"--nosuch --version" は
// 使用方法をエラー出力へ出す。先に書かれたほうが効く、という単純な規則とする。
//
// 位置引数は 2 つ目以降を無視する（E2E-105 のケース 4）。
func parseArgs(args []string) (action, []string) {
	var positional []string

	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
			continue
		}
		switch a {
		case "-v", "--version":
			return actionVersion, nil
		case "-h", "--help":
			return actionHelp, nil
		default:
			return actionUnknownOption, nil
		}
	}
	return actionRun, positional
}

// resolveStartup は実行環境から cwd と exeDir を求め、session へ渡す。
//
// os.Getwd と os.Executable の呼び出しをここで行うのは、session を
// 実行環境から切り離してテストできるようにするためである（IMP-193, UT-035）。
func resolveStartup(positional []string) (session.Startup, error) {
	cwd, err := os.Getwd()
	if err != nil {
		// カレントディレクトリが失われている（削除された等）。
		// 起動は続けるため、空文字のまま session へ渡す。
		cwd = ""
	}

	return session.ResolveStartup(positional, cwd, executableDir())
}

// executableDir は実行ファイルが置かれたディレクトリを返す。
// シンボリックリンクは解決してから求める（FR-013）。
// 求められない場合は空文字を返し、探索の対象から外す。
func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// printVersion はバージョン情報を出力する（FR-012, BR-030, E2E-102）。
// 文言はすべて英語とする（UI-024）。
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "MarkView %s\n", buildinfo.Version)
	fmt.Fprintf(w, "commit: %s\n", buildinfo.Commit)
	fmt.Fprintf(w, "built:  %s\n", buildinfo.BuildTime)
}

// printUsage は使用方法を出力する（FR-012, E2E-103）。
// 文言はすべて英語とする（UI-024）。
func printUsage(w io.Writer) {
	fmt.Fprint(w, `MarkView - a lightweight Markdown viewer.

Usage:
  MarkView [options] [path]

Arguments:
  path            A Markdown file to open, or a directory to browse.
                  When omitted, MarkView looks for README.md in the current
                  directory, then in the directory of the executable.

Options:
  -v, --version   Print version information and exit.
  -h, --help      Print this help and exit.
`)
}
