// Package applog は MARKVIEW_DEBUG の判定とログの初期化を担う（IMP-023）。
//
// **依存を持たない葉パッケージである**（IMP-012）。標準ライブラリ以外を
// import してはならない。この制約があるからこそ、どのパッケージから参照しても
// テストバイナリに重い依存が入らない。
//
// 既定ではログを出さない（NFR-041）。これは「自分が書かなければ満たせる」種類の
// 要求ではない。判定が散れば、そのうち 1 か所が漏れて配布物が出力を始める。
// 実際に go-webview2 が標準ロガーへ直接書いていた件を E2E-104 で検出しており
// （IMP-023）、自前のコードでも同じことが起こりうる。
// **そのため、環境変数を読むのは Enabled だけとする。**
package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
)

// envDebug はログを有効にする環境変数（NFR-041, IMP-023）。
//
// **この文字列リテラルはリポジトリ内でここだけに置く。**
// 検査は機械的に行える。
//
//	grep -rn '"MARKVIEW_DEBUG"' --include=*.go
//
// が、このファイルの 1 行だけを返すこと。テストからは envDebug を使う。
const envDebug = "MARKVIEW_DEBUG"

// enabledValue は唯一の有効値（IMP-023, UT-806）。
//
// **これ以外はすべて無効とする。** "0" や "true" まで有効にすると、
// 意図しない値で配布物が出力を始める。前後の空白も取り除かない。
const enabledValue = "1"

// Enabled は MARKVIEW_DEBUG=1 かを返す（IMP-023）。
//
// **環境変数を読むのはこの関数だけとする。** 他のどこでも os.Getenv しない。
func Enabled() bool {
	return os.Getenv(envDebug) == enabledValue
}

// New は出力先を固定したロガーを返す（IMP-023）。
//
// Enabled() が false なら io.Discard を出力先とするため、呼び出し側は
// 有効かどうかを気にせず書いてよい。ファイルには出力しない（NFR-041）。
func New() *slog.Logger {
	return newLogger(os.Stderr)
}

// newLogger は出力先を渡してロガーを組み立てる。
//
// テストからプロセスの標準エラーを奪わずに検証するために分けている
// （UT-806。標準エラーを差し替えると並行実行が壊れる）。
func newLogger(w io.Writer) *slog.Logger {
	if !Enabled() {
		w = io.Discard
	}

	return slog.New(slog.NewTextHandler(w, nil))
}

// Recovered は recover した値とスタックトレースを記録する（IMP-022, IMP-023）。
//
// **recover した直後に呼ぶ。** スタックは巻き戻ると取れないため、呼び出し側で
// debug.Stack() を持ち回らない。deferred の中で呼べば、スタックには
// runtime.gopanic とパニックした関数が含まれる。
//
// where は "renderer.Render" のような発生箇所。スタックに詳細が出るため、
// ここは粗くてよい。
func Recovered(where string, v any) {
	recovered(os.Stderr, where, v)
}

func recovered(w io.Writer, where string, v any) {
	if !Enabled() {
		return
	}

	newLogger(w).Error("recovered from panic", "where", where, "value", fmt.Sprint(v))

	// **スタックは slog の属性に入れない。** TextHandler が 1 行へ押し込み、
	// 改行が \n として引用符の中に潰れて読めなくなる。記録の直後に生のまま続ける。
	_, _ = w.Write(debug.Stack())
}
