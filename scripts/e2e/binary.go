package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func runBinary(args []string) (*report, error) {
	flags := flag.NewFlagSet("binary", flag.ExitOnError)
	archive := flags.String("archive", "", "配布アーカイブ（展開してから実行する）")
	exe := flags.String("exe", "", "展開済みの実行ファイル")
	version := flags.String("version", "", "タグ名（例: v1.0.0-rc.1）")
	data := flags.String("data", filepath.Join("testdata", "e2e"), "検証用データ（E2E-012）")
	alive := flags.Duration("alive", 5*time.Second, "起動後に生存を確かめるまでの時間")
	timeout := flags.Duration("timeout", 30*time.Second, "CLI 実行の打ち切り")
	foreign := flags.Bool("allow-foreign", false, "他 OS の成果物でも中断せず、実行を伴う確認を飛ばす")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if *version == "" {
		return nil, errors.New("-version が要る（例: -version v1.0.0-rc.1）")
	}

	if (*archive == "") == (*exe == "") {
		return nil, errors.New("-archive か -exe のどちらか一方を指定する")
	}

	// **展開してから実行する**（E2E-011）。配布物そのものを動かす。
	if *archive != "" {
		dir, err := os.MkdirTemp("", "markview-e2e-")
		if err != nil {
			return nil, fmt.Errorf("展開先を作れない: %w", err)
		}
		defer os.RemoveAll(dir) //nolint:errcheck // 一時ディレクトリ

		found, err := extract(*archive, dir)
		if err != nil {
			return nil, err
		}

		*exe = found
	}

	absData, err := filepath.Abs(*data)
	if err != nil {
		return nil, fmt.Errorf("検証用データのパスを解決できない: %w", err)
	}

	if _, err := os.Stat(filepath.Join(absData, "README.md")); err != nil {
		return nil, fmt.Errorf("検証用データが無い（E2E-012）: %w", err)
	}

	fmt.Printf("実行ファイル : %s\n", *exe)
	fmt.Printf("検証用データ : %s\n", absData)
	fmt.Printf("タグ         : %s\n", *version)

	if display := displayWrapper(); display != "" {
		fmt.Printf("表示         : %s\n", display)
	}

	// **対象 OS の上でしか実行できない**（E2E-010）。取り違えたまま走らせると
	// 実行を伴う確認がすべて失敗し、原因が「配布物が壊れている」のか
	// 「ジョブの割り当てを誤っている」のか読み取れなくなる。既定では中断する。
	format := executableFormat(*exe)
	native := (format == "elf" && runtime.GOOS == "linux") ||
		(format == "pe" && runtime.GOOS == "windows")

	if !native && !*foreign {
		return nil, fmt.Errorf("%s は %s 形式であり、この OS（%s）では実行できない。"+
			"対象 OS のランナーで実行するか、-allow-foreign を付けて実行を伴う確認を飛ばす",
			filepath.Base(*exe), format, runtime.GOOS)
	}

	result := &report{}

	if native {
		checkVersionOutput(result, *exe, *version, *timeout)
		checkHelpOutput(result, *exe, *timeout)
		checkStartup(result, *exe, absData, *alive)
		checkBadArguments(result, *exe, absData, *alive, *timeout)
	} else {
		skipRunChecks(result, fmt.Sprintf("%s 形式を %s では実行できない", format, runtime.GOOS))
	}

	checkDependencies(result, *exe)

	return result, nil
}

// executableFormat は実行ファイルの形式を返す（"elf" / "pe" / "unknown"）。
func executableFormat(exe string) string {
	file, err := os.Open(exe)
	if err != nil {
		return "unknown"
	}
	defer file.Close() //nolint:errcheck // 読むだけ

	head := make([]byte, 4)
	if _, err := io.ReadFull(file, head); err != nil {
		return "unknown"
	}

	switch {
	case string(head) == "\x7fELF":
		return "elf"
	case string(head[:2]) == "MZ":
		return "pe"
	}

	return "unknown"
}

// skipRunChecks は実行を伴う確認をまとめて飛ばす。
//
// **黙って減らさない。** 表の行はそのまま出し、飛ばした理由を残す。
func skipRunChecks(result *report, why string) {
	rows := []struct {
		id     string
		number int
		name   string
	}{
		{"E2E-102", 1, "--version の終了"},
		{"E2E-102", 2, "--version の内容"},
		{"E2E-102", 3, "バージョンがタグと一致"},
		{"E2E-102", 4, "-v が同じ出力"},
		{"E2E-103", 1, "--help の終了"},
		{"E2E-103", 2, "--help の内容"},
		{"E2E-103", 3, "--help が英語"},
		{"E2E-103", 4, "-h が同じ出力"},
		{"E2E-104", 1, "引数なしで起動"},
		{"E2E-104", 2, "終了させたときの状態"},
		{"E2E-104", 3, "README.md を指定して起動"},
		{"E2E-104", 4, "docs/ を指定して起動"},
		{"E2E-104", 5, "標準エラーに自前のログが無い"},
		{"E2E-105", 1, "存在しないパス"},
		{"E2E-105", 2, "デバイスファイル"},
		{"E2E-105", 3, "権限のないファイル"},
		{"E2E-105", 4, "引数を 2 つ指定"},
		{"E2E-105", 5, "未知のオプション"},
	}

	for _, row := range rows {
		result.skip(row.id, row.number, row.name, why)
	}
}

// checkVersionOutput は --version を確かめる（E2E-102）。
//
// **ウィンドウを開かないことは「終了すること」で確かめる。** ウィンドウを
// 開いてしまえばプロセスは終わらず、打ち切りに掛かる。
func checkVersionOutput(result *report, exe, version string, timeout time.Duration) {
	long := run(exe, "", timeout, "--version")

	result.verify("E2E-102", 1, "--version の終了", long.code == 0 && !long.timedOut,
		long.describe()+"（終了したのでウィンドウは開いていない）")

	hasAll := strings.Contains(long.stdout, "commit:") && strings.Contains(long.stdout, "built:")
	result.verify("E2E-102", 2, "--version の内容", hasAll, firstLine(long.stdout))

	// **ldflags の埋め込み漏れはここでしか捕まらない**（BR-030）。
	want := "MarkView " + version
	result.verify("E2E-102", 3, "バージョンがタグと一致", strings.HasPrefix(long.stdout, want+"\n"),
		fmt.Sprintf("%q（期待 %q）", firstLine(long.stdout), want))

	short := run(exe, "", timeout, "-v")
	result.verify("E2E-102", 4, "-v が同じ出力", short.stdout == long.stdout && short.code == long.code,
		short.describe())
}

// checkHelpOutput は --help を確かめる（E2E-103）。
func checkHelpOutput(result *report, exe string, timeout time.Duration) {
	long := run(exe, "", timeout, "--help")

	result.verify("E2E-103", 1, "--help の終了", long.code == 0 && !long.timedOut,
		long.describe()+"（終了したのでウィンドウは開いていない）")

	hasAll := strings.Contains(long.stdout, "Usage:") &&
		strings.Contains(long.stdout, "--version") &&
		strings.Contains(long.stdout, "--help")
	result.verify("E2E-103", 2, "--help の内容", hasAll, firstLine(long.stdout))

	// UI-024: 利用者に見えるものは英語。ASCII だけかで機械的に見る。
	result.verify("E2E-103", 3, "--help が英語", isASCII(long.stdout),
		fmt.Sprintf("%d 文字 / ASCII のみ=%t", len(long.stdout), isASCII(long.stdout)))

	short := run(exe, "", timeout, "-h")
	result.verify("E2E-103", 4, "-h が同じ出力", short.stdout == long.stdout && short.code == long.code,
		short.describe())
}

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
}

// goLogPrefix は標準 log パッケージの既定の接頭辞（"2026/09/02 02:47:44 "）。
var goLogPrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

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
func looksLikeOurLog(line string) bool {
	return goLogPrefix.MatchString(line) ||
		strings.Contains(line, "level=") ||
		strings.Contains(strings.ToLower(line), "markview")
}

// checkBadArguments は不正な引数での起動を確かめる（E2E-105）。
//
// 判定は「起動から一定時間後に生存している、または終了コードが 0」。
func checkBadArguments(result *report, exe, data string, alive, timeout time.Duration) {
	nosuch := filepath.Join(data, "nosuchfile.md")

	device := "/dev/null"
	if runtime.GOOS == "windows" {
		device = "NUL"
	}

	survives := func(number int, name string, args ...string) {
		got := launch(exe, data, alive, args...)
		result.verify("E2E-105", number, name, got.alive || got.exitedCleanly, got.detail)
	}

	survives(1, "存在しないパス", nosuch)
	survives(2, "デバイスファイル", device)

	// ケース 3: 読み取り権限のないファイル（Linux）。
	if runtime.GOOS == "windows" {
		result.skip("E2E-105", 3, "権限のないファイル", "Windows では chmod で再現できない")
	} else if os.Geteuid() == 0 {
		result.skip("E2E-105", 3, "権限のないファイル", "root は権限に関わらず読めてしまう")
	} else if path, err := unreadableFile(); err != nil {
		result.fail("E2E-105", 3, "権限のないファイル", "用意できない: %v", err)
	} else {
		defer os.Remove(path) //nolint:errcheck // 一時ファイル
		survives(3, "権限のないファイル", path)
	}

	survives(4, "引数を 2 つ指定", "README.md", "docs")

	// ケース 5: 未知のオプション（FR-012）。
	got := run(exe, data, timeout, "--nosuch")

	ok := got.code == 2 && !got.timedOut &&
		strings.Contains(got.stderr, "Usage:") && got.stdout == ""
	result.verify("E2E-105", 5, "未知のオプション", ok,
		fmt.Sprintf("%s / stderr=%q / stdout %d 文字",
			got.describe(), firstLine(got.stderr), len(got.stdout)))
}

// unreadableFile は読み取り権限のないファイルを作る（E2E-105 ケース 3）。
func unreadableFile() (string, error) {
	file, err := os.CreateTemp("", "markview-unreadable-*.md")
	if err != nil {
		return "", err
	}

	path := file.Name()

	if _, err := file.WriteString("# 読めないはず\n"); err != nil {
		file.Close() //nolint:errcheck

		return "", err
	}

	if err := file.Close(); err != nil {
		return "", err
	}

	if err := os.Chmod(path, 0o000); err != nil {
		return "", err
	}

	return path, nil
}

// displayWrapper は GUI を起動するときに前に置くコマンドを返す。
//
// **CI には画面が無い。** Linux では xvfb-run で仮想ディスプレイを用意する
// （E2E-104 の実行方法）。既に DISPLAY があるならそのまま使う。
func displayWrapper() string {
	if runtime.GOOS == "windows" {
		return ""
	}

	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		return ""
	}

	path, err := exec.LookPath("xvfb-run")
	if err != nil {
		return ""
	}

	return path
}

// stop はプロセスを終わらせる。
//
// Windows では **taskkill を /F なしで呼ぶ**。ウィンドウへ WM_CLOSE が
// 送られ、閉じるボタンと同じ経路で終わる。Kill で潰すと「正常に終了できるか」
// を確かめたことにならない。
//
// **/T は付けない。** 木をたどって子まで閉じようとするが、WebView2 の
// 子プロセス（msedgewebview2.exe）はウィンドウを持たないため /F なしでは
// 閉じられず、taskkill 全体が 128 で失敗する。MarkView 自身のウィンドウを
// 閉じれば子は道連れに終わる（2026-09-02 に実機で確認）。
func stop(process *os.Process) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", strconv.Itoa(process.Pid)).Run()
	}

	return process.Signal(syscall.SIGTERM)
}
