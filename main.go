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
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"github.com/kznagamori/go_MarkView/internal/assetsrv"
	"github.com/kznagamori/go_MarkView/internal/buildinfo"
	"github.com/kznagamori/go_MarkView/internal/config"
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

// OSS ライセンス一覧（FR-101, BR-040, IMP-030）。
//
// **実行時に外部から取得しない。** ビルド時に埋め込んだものだけを表示する
// （FR-101）。中身は scripts/genlicenses が生成する。ファイルが無いと
// ビルドが失敗するため、生成漏れのまま配布されることはない。
//
//go:embed licenses/THIRD_PARTY.md
var thirdPartyLicenses string

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

	if action != actionRun {
		// ウィンドウを開かずに標準出力・標準エラーへ書く経路に入る前に、
		// 親プロセスのコンソールへ繋ぎ直す（Windows のみ実体がある）。
		// Wails が作る Windows の実行ファイルは GUI サブシステムであり、
		// これを行わないとターミナルに何も表示されない。
		attachConsole()
		stdout, stderr = os.Stdout, os.Stderr
	}

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

	// 起動対象が読めなくてもウィンドウは必ず開く（FR-012）。エラーは
	// App が保持し、GetInitialState で状態画面とステータスへ渡す（IMP-193）。
	startup, startupErr := resolveStartup(positional)

	if err := launch(startup, startupErr); err != nil {
		fmt.Fprintf(stderr, "MarkView: %v\n", err)
		return exitRunError
	}
	return exitOK
}

// launch は Wails のウィンドウを起動する。戻るのはウィンドウを閉じた後。
//
// 設定はここで読む。ウィンドウの初期サイズが Wails の起動オプションとして
// 必要であり、App より先に必要になるためである（UI-011, UI-110, IMP-193）。
// **位置は保存も復元もしない。** 常にプライマリモニタの中央に置く
// （UI-111。中央寄せは App.onStartup）。
func launch(startup session.Startup, startupErr error) error {
	silenceDefaultLogger()

	// 設定がない・壊れている場合も既定値で起動する。Load はエラーを
	// 返さない（UI-113, IMP-151）。
	cfg := config.Load()

	buildinfo.SetVendorJSON(readVendorJSON())
	app := NewApp(startup, startupErr, cfg)

	return wails.Run(&options.App{
		Title:     AppTitle, // 文書未表示時のタイトル（UI-013）
		Width:     cfg.WindowWidth,
		Height:    cfg.WindowHeight,
		MinWidth:  640, // UI-011
		MinHeight: 480,

		// 最大化状態は復元する。位置は復元しない（UI-110, UI-111）。
		WindowStartState: startState(cfg),

		AssetServer: &assetserver.Options{
			// Assets を渡さず Handler だけを使う。埋め込み資産の
			// Content-Type を自前の表で決めるためである（IMP-160, IMP-162）。
			// Wails は /wails/runtime.js の配信と index.html への
			// スクリプト挿入を、この Handler より前で行う。
			Handler: assetsrv.New(frontendAssets(), appIconPNG),
		},

		// **ファイルドロップは既定で無効である**（IMP-245, FR-011）。
		// これを渡さないと runtime.OnFileDrop のコールバックが呼ばれない。
		// 受け口となる要素は CSS の --wails-drop-target で決まるため、
		// #app に指定してウィンドウ全体を対象にする（UI-070）。
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},

		OnStartup: app.onStartup,

		// ウィンドウの大きさは閉じる直前に取り込む。OnShutdown では
		// ウィンドウが既に破棄されており、読み出すと落ちる（IMP-194）。
		OnBeforeClose: app.onBeforeClose,
		OnShutdown:    app.onShutdown,

		Bind: []any{app},

		// Windows は実行ファイルのリソースからアイコンを取るため指定不要。
		// Linux は埋め込んだ PNG を明示的に渡す（IMP-032）。
		Linux: &linux.Options{Icon: appIconPNG},
	})
}

// startState は起動時のウィンドウ状態を返す（UI-110）。
func startState(cfg config.Config) options.WindowStartState {
	if cfg.WindowMaximized {
		return options.Maximised
	}
	return options.Normal
}

// frontendAssets は埋め込みの frontend/ を根とした FS を返す。
//
// go:embed は frontend/ を含んだ形で保持するため、そのままでは
// assetsrv が index.html を引けない。
func frontendAssets() fs.FS {
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		// go:embed の対象が変わらない限り起こらない。起きた場合も
		// ウィンドウは開き、資産が 404 になるだけとする（FR-111）。
		return frontendFS
	}
	return sub
}

// readVendorJSON は同梱資産の情報を読む（IMP-181, BR-042）。
//
// go:embed はパッケージのディレクトリより上を参照できないため、main.go が
// 読んで buildinfo へ渡す。**読めなくてもエラーにしない。** 情報ダイアログの
// Bundled 行が空になるだけで、文書の閲覧は続けられる（FR-111）。
//
// TODO(T6-1): frontend/vendor/ を用意するまでは常に読めない。
func readVendorJSON() []byte {
	data, err := frontendFS.ReadFile("frontend/vendor/vendor.json")
	if err != nil {
		return nil
	}
	return data
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

// silenceDefaultLogger は標準ロガーの出力先を捨てる（NFR-041, IMP-023）。
//
// **既定ではログを出さない**という要求は、自分が書かなければ満たせるもの
// ではない。**依存ライブラリが標準の log パッケージへ直接書く。**
// go-webview2 は環境の初期化に成功したことを log.Printf で報告しており
// （v1.0.22 の pkg/edge/chromium.go:318）、そのままだと配布物が起動のたびに
// 標準エラーへ 1 行出す。2026-09-02 に E2E-104 のケース 5 で検出した。
//
// MarkView 自身は log/slog を使う（IMP-023）。標準ロガーを黙らせても
// 自前のログには影響しない。MARKVIEW_DEBUG=1 のときは調査の助けになるため
// そのまま出す。
func silenceDefaultLogger() {
	if os.Getenv("MARKVIEW_DEBUG") == "1" {
		return
	}

	log.SetOutput(io.Discard)
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
