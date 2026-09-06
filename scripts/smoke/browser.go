package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ErrNoBrowser はヘッドレスブラウザが見つからないことを表す（IMP-021）。
var ErrNoBrowser = errors.New("no Chromium-based browser found")

// findBrowser は使用するブラウザの実行ファイルを探す。
//
// **この依存は CI と開発機の中に閉じている**（BR-054）。配布物には含まれず、
// BR-001 の「Node.js を要求しない」にも触れない。Chromium 系を使うのは、
// Windows では WebView2（Edge と同じエンジン）、Linux では WebKitGTK が
// 実行環境であり（AR-003）、前者とは同じ土俵で確かめられるためである。
//
// 探索順は明示指定 → 環境変数 → OS ごとの既定。
func findBrowser(explicit string) (string, error) {
	if explicit != "" {
		return resolve(explicit)
	}

	if fromEnv := os.Getenv("MARKVIEW_SMOKE_BROWSER"); fromEnv != "" {
		return resolve(fromEnv)
	}

	for _, candidate := range candidates() {
		if found, err := resolve(candidate); err == nil {
			return found, nil
		}
	}

	return "", fmt.Errorf("%w: MARKVIEW_SMOKE_BROWSER で指定できる", ErrNoBrowser)
}

// resolve は絶対パスならその存在を、名前なら PATH を確かめる。
func resolve(name string) (string, error) {
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err != nil {
			return "", fmt.Errorf("%w: %s", ErrNoBrowser, name)
		}

		return name, nil
	}

	found, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNoBrowser, name)
	}

	return found, nil
}

func candidates() []string {
	if runtime.GOOS != "windows" {
		return []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
			"microsoft-edge",
			"microsoft-edge-stable",
		}
	}

	// Edge は Windows 11 に必ず入っている。WebView2 と同じエンジンであり、
	// **本番に最も近い**ため先に見る。
	list := []string{}
	for _, base := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")} {
		if base == "" {
			continue
		}

		list = append(list,
			filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
		)
	}

	return append(list, "msedge.exe", "chrome.exe")
}

// launch はブラウザをヘッドレスで起動する。終了は ctx の取り消しで行う。
//
// 標準エラーは捨てずに buffer へ集める。失敗したときに「ブラウザが何か
// 言っていたか」を出せるようにするためで、成功時は表示しない。
func launch(ctx context.Context, browser, url string, stderr *safeBuffer) (*exec.Cmd, func(), error) {
	profile, err := os.MkdirTemp("", "markview-smoke-")
	if err != nil {
		return nil, nil, fmt.Errorf("一時プロファイルを作れない: %w", err)
	}

	cleanup := func() { os.RemoveAll(profile) } //nolint:errcheck

	// --no-sandbox は CI（コンテナ・root 実行）で必要になる。開いているのは
	// 自分で立てた 127.0.0.1 のページだけであり、外部の内容は読み込まない。
	args := []string{
		"--headless=new",
		"--disable-gpu",
		"--no-sandbox",
		"--disable-dev-shm-usage",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-background-networking",
		"--disable-component-update",
		"--window-size=1280,900",
		"--user-data-dir=" + profile,
		url,
	}

	cmd := exec.CommandContext(ctx, browser, args...)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("ブラウザを起動できない (%s): %w", browser, err)
	}

	return cmd, cleanup, nil
}
