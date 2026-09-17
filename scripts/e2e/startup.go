package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// 本ファイルは、起動と終了の確認（E2E-104）を持つ。WebView のデータの置き場所（ケース 6。
// AR-004, NFR-033）と、標準エラーに自前のログが出ていないか（ケース 5。NFR-041）の判定を含む。
//
// binary.go が 400 行の目安（IMP-011）を超えたため分けた。

// checkStartup は起動と終了を確かめる（E2E-104）。
//
// **これがリリース前に唯一「実行ファイルが動く」ことを機械的に確かめる手段**
// である。WebView2 / WebKitGTK の初期化に失敗すればここで落ちる。
func checkStartup(result *report, exe, data string, alive time.Duration) {
	cases := []struct {
		number int
		name   string
		args   []string
	}{
		{1, "引数なしで起動", nil},
		{3, "README.md を指定して起動", []string{"README.md"}},
		{4, "docs/ を指定して起動", []string{"docs"}},
	}

	// ケース 6 は起動の前後を比べる必要があるため、先に現状を控える。
	before := snapshotAppDataDir(exe)

	var stderrs []string

	for i, c := range cases {
		got := launch(exe, data, alive, c.args...)

		result.verify("E2E-104", c.number, c.name, got.alive, got.detail)

		// ケース 2（終了させても異常終了しない）は最初の起動で代表させる。
		if i == 0 {
			result.verify("E2E-104", 2, "終了させたときの状態", got.alive && got.stopped, got.stopDetail)
		}

		stderrs = append(stderrs, got.stderr)
	}

	// ケース 5: MarkView 自身のログが出ていないこと（NFR-041, IMP-023）。
	// **OS 側のライブラリが出す警告は対象外**であるため、行ごとに見分ける。
	var ours []string

	othersCount := 0

	for _, text := range stderrs {
		for _, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if looksLikeOurLog(line) {
				ours = append(ours, line)

				continue
			}

			othersCount++
		}
	}

	result.verify("E2E-104", 5, "標準エラーに自前のログが無い", len(ours) == 0,
		emptyOr(ours, fmt.Sprintf("自前のログ 0 行 / OS 側 %d 行", othersCount), "自前のログ: "))

	checkWebviewDataPath(result, exe, before)
}

// webviewDataState は %APPDATA%\<実行ファイル名>\EBWebView の状態（E2E-104 ケース 6）。
type webviewDataState struct {
	path    string
	exists  bool
	modTime time.Time
}

// snapshotAppDataDir は既定の WebView2 データ領域の現状を控える。
//
// **見るのは親ではなく `EBWebView` である。** WebView2 が書くのはその配下で
// あり、親ディレクトリの更新時刻は最初の作成時から動かない。
func snapshotAppDataDir(exe string) webviewDataState {
	appData := os.Getenv("AppData")
	if runtime.GOOS != "windows" || appData == "" {
		return webviewDataState{}
	}

	// go-webview2 の既定値は %APPDATA%\<実行ファイル名>（IMP-193）。
	path := filepath.Join(appData, filepath.Base(exe), "EBWebView")

	info, err := os.Stat(path)
	if err != nil {
		return webviewDataState{path: path}
	}

	return webviewDataState{path: path, exists: true, modTime: info.ModTime()}
}

// checkWebviewDataPath は WebView2 のデータ領域が %APPDATA% に作られて
// いないことを確かめる（E2E-104 ケース 6。AR-004, NFR-033, IMP-193）。
//
// **`WebviewUserDataPath` の指定漏れを機械的に検出する唯一の手段である。**
// 指定を落としても画面は何も壊れず、%APPDATA% に数十 MB が書かれるだけで
// あるため、人が見て気づくことはまずない。
//
// 既にある古いディレクトリで落ちないよう、**存在の有無ではなく起動の前後で
// 変化したか**を見る。開発機には修正前の残骸があることがある。
func checkWebviewDataPath(result *report, exe string, before webviewDataState) {
	const (
		id   = "E2E-104"
		num  = 6
		name = "WebView のデータが %APPDATA% に無い"
	)

	if runtime.GOOS != "windows" {
		result.skip(id, num, name, "Linux には指定手段が無く、既定に従う（AR-004）")

		return
	}

	if before.path == "" {
		result.skip(id, num, name, "AppData 環境変数が無い")

		return
	}

	after := snapshotAppDataDir(exe)

	// 指定が効いていれば、テンポラリ側に作られる（AR-004）。
	wanted := filepath.Join(os.TempDir(), "MarkView", "webview2")
	_, tempErr := os.Stat(wanted)
	inTemp := tempErr == nil

	switch {
	case !before.exists && after.exists:
		result.verify(id, num, name, false,
			"起動で "+after.path+" が作られた。WebviewUserDataPath の指定漏れ（IMP-193）")
	case before.exists && !after.modTime.Equal(before.modTime):
		result.verify(id, num, name, false,
			"起動で "+after.path+" が更新された。WebviewUserDataPath の指定漏れ（IMP-193）")
	case !inTemp:
		result.verify(id, num, name, false,
			wanted+" が作られていない。WebView2 のデータ領域の位置を確認すること")
	default:
		result.verify(id, num, name, true, "データ領域は "+wanted+" 配下"+staleNote(before.exists))
	}
}

// staleNote は %APPDATA% 側に古い残骸がある場合の但し書きを返す。
func staleNote(stale bool) string {
	if stale {
		return "（%APPDATA% 側に残骸があるが、この起動では触られていない）"
	}

	return ""
}

// goLogPrefix は標準 log パッケージの既定の接頭辞（"2026/09/02 02:47:44 "）。
var goLogPrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

// glibLogLine は GLib の既定のログハンドラが出す行の形（E2E-104 ケース 5）。
//
//	(MarkView:12345): Gtk-WARNING **: 10:11:12.345: …
//	(MarkView:12345): GLib-GObject-CRITICAL **: …
//	** (MarkView:12345): WARNING **: …（ドメインの無いもの、致命的なもの）
//
// **括弧の中の名前はプログラム名であり、GLib が付ける。** MarkView の中で
// GTK / WebKitGTK が警告すると、アプリ名を含んだ行になる。形を「行頭の括弧に
// `名前:pid`、続けて `ドメイン-レベル **:`」に限り、それ以外の行は拾わない。
// ` **` は警告以上（ERROR / CRITICAL / WARNING）にだけ付き、DEBUG などには
// 付かない（GLib の gmessages.c）ため、無くても当てる。
var glibLogLine = regexp.MustCompile(
	`^(?:\*\* )?\([^()\s]+:\d+\): (?:[A-Za-z0-9_.-]+-)?(?:ERROR|CRITICAL|WARNING|Message|INFO|DEBUG)(?: \*\*)?: `)

// looksLikeOurLog は MarkView 側の出力らしい行かを判定する（IMP-023）。
//
// 既定ではログを出さない（NFR-041）。**OS 側のライブラリが出す警告は対象外**
// であるため（E2E-104 ケース 5）、見分けがつく形を 3 つ挙げる。
//
//   - Go の標準 log の日時。GTK も WebKit もこの形では出さない。
//     依存ライブラリが log.Printf を直接呼ぶ場合もここに掛かる
//   - log/slog のテキスト形式（level=）
//   - アプリ名
//
// 1 つ目は実際に効いた。go-webview2 が起動のたびに 1 行出しており、
// この判定を入れたことで E2E-104 が失敗し、main.go を直すことになった。
//
// **GLib の既定のログの形の行は、アプリ名を含んでいても OS 側として扱う**
// （E2E-104 ケース 5）。GLib はプログラム名を行頭の括弧に入れるため、アプリ名で
// 判定すると **GTK が 1 行警告しただけでリリースが止まる。** 先にこの形を除く。
func looksLikeOurLog(line string) bool {
	if glibLogLine.MatchString(line) {
		return false
	}

	return goLogPrefix.MatchString(line) ||
		strings.Contains(line, "level=") ||
		strings.Contains(strings.ToLower(line), "markview")
}
