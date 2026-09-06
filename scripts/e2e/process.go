package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// runResult は 1 回の実行の結果。
type runResult struct {
	code     int
	stdout   string
	stderr   string
	elapsed  time.Duration
	timedOut bool
}

func (r runResult) describe() string {
	if r.timedOut {
		return fmt.Sprintf("打ち切り (%.1f 秒)", r.elapsed.Seconds())
	}

	return fmt.Sprintf("exit=%d (%.2f 秒)", r.code, r.elapsed.Seconds())
}

// run は終了するまで待つ実行（--version / --help / 未知のオプション）。
//
// **打ち切りに掛かったら、それはウィンドウを開いたということ**である
// （E2E-102, E2E-103, E2E-105）。CLI として応答すべき経路で GUI が
// 立ち上がる不具合は、この形で現れる。
func run(exe, dir string, timeout time.Duration, args ...string) runResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	err := cmd.Run()
	elapsed := time.Since(started)

	result := runResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		elapsed:  elapsed,
		timedOut: errors.Is(ctx.Err(), context.DeadlineExceeded),
	}

	if cmd.ProcessState != nil {
		result.code = cmd.ProcessState.ExitCode()
	} else if err != nil {
		result.code = -1
	}

	return result
}

// launchResult は GUI を起動したときの結果。
type launchResult struct {
	alive         bool // 一定時間後も生存していた
	exitedCleanly bool // 早く終わったが終了コードは 0
	stopped       bool // 終了要求に応じて止まった
	detail        string
	stopDetail    string
	stderr        string
}

// launch はウィンドウを開く起動を行い、生存を確かめてから終わらせる
// （E2E-104, E2E-105）。
//
// **固定時間の sleep で描画完了を待たない**（E2E-020）。待つのは
// 「一定時間後も生きているか」という一点だけで、これは描画の完了とは
// 無関係に判定できる。
func launch(exe, dir string, alive time.Duration, args ...string) launchResult {
	name := exe
	full := args

	// CI には画面が無い。Linux では xvfb-run を前に置く（E2E-104）。
	if wrapper := displayWrapper(); wrapper != "" {
		name = wrapper
		full = append([]string{"-a", exe}, args...)
	}

	cmd := exec.Command(name, full...)
	cmd.Dir = dir

	var stderr lockedBuffer

	cmd.Stderr = &stderr

	started := time.Now()

	if err := cmd.Start(); err != nil {
		return launchResult{detail: fmt.Sprintf("起動できない: %v", err)}
	}

	done := make(chan error, 1)

	go func() { done <- cmd.Wait() }()

	select {
	case <-done:
		// 生存を確かめる前に終わった。異常終了の疑い。
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}

		return launchResult{
			exitedCleanly: code == 0,
			detail: fmt.Sprintf("%.1f 秒で終了 exit=%d%s",
				time.Since(started).Seconds(), code, quoted(stderr.String())),
			stderr: stderr.String(),
		}

	case <-time.After(alive):
	}

	result := launchResult{
		alive:  true,
		detail: fmt.Sprintf("%.0f 秒後も生存", alive.Seconds()),
	}

	// 終了要求を出して、応じるかを見る（E2E-104 ケース 2）。
	if err := stop(cmd.Process); err != nil {
		result.stopDetail = fmt.Sprintf("終了要求を出せない: %v", err)

		_ = cmd.Process.Kill()
		<-done

		result.stderr = stderr.String()

		return result
	}

	select {
	case err := <-done:
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}

		// **終了コードは 0 か、シグナルによる終了なら合格**（E2E-104）。
		// SIGTERM で終わると Go は exit=-1 を返す。
		result.stopped = code == 0 || code == -1 || err == nil
		result.stopDetail = fmt.Sprintf("終了要求から %.1f 秒で終了 exit=%d",
			time.Since(started).Seconds()-alive.Seconds(), code)

	case <-time.After(10 * time.Second):
		result.stopDetail = "終了要求に 10 秒応じない。強制終了した"

		_ = cmd.Process.Kill()
		<-done
	}

	result.stderr = stderr.String()

	return result
}

func quoted(s string) string {
	if s == "" {
		return ""
	}

	return " / stderr=" + firstLine(s)
}

// lockedBuffer は子プロセスの出力を受ける。書くのは別のゴルーチンである。
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}
