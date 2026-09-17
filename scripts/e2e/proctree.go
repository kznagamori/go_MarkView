package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// findDescendantByExe は、pid の子孫のうち実行ファイルが exe であるプロセスを
// /proc から探す（E2E-104。Linux の xvfb-run の下で起動した MarkView）。
//
// **実行ファイルの実体のパスで照合する。** /proc/<pid>/exe はカーネルが解決した
// 実体を指すため、exe も EvalSymlinks で解決してから比べる。名前（comm）で
// 照合すると、同じ名前の別プロセスや、15 文字で切り詰められた名前と取り違える。
//
// xvfb-run は Xvfb の準備を待ってから子を起動するため、見つかるまで少し待つ。
// 見つかったものが複数あれば、pid に最も近い（浅い）ものを返す。WebKitGTK が
// 起動する子プロセスは別の実行ファイルであり、ここには掛からない。
func findDescendantByExe(pid int, exe string) (*os.Process, error) {
	want, err := resolveExe(exe)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(3 * time.Second)

	for {
		found, err := scanDescendants(pid, want)
		if err != nil {
			return nil, err
		}

		if found > 0 {
			return os.FindProcess(found)
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("pid %d の子孫に %s を実行するプロセスが無い", pid, want)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// resolveExe は照合に使う実行ファイルの実体の絶対パスを返す。
func resolveExe(exe string) (string, error) {
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(abs)
}

// scanDescendants は /proc を 1 回読み、root の子孫を浅い順にたどって、
// 実行ファイルが want のプロセスの pid を返す。無ければ 0。
func scanDescendants(root int, want string) (int, error) {
	children, err := readChildren()
	if err != nil {
		return 0, err
	}

	queue := children[root]
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]

		if exeOf(pid) == want {
			return pid, nil
		}

		queue = append(queue, children[pid]...)
	}

	return 0, nil
}

// readChildren は /proc/<pid>/stat の親 pid から、親 → 子の対応を作る。
// 子は pid の昇順に並べる（探す順を毎回同じにする）。
func readChildren() (map[int][]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("/proc を読めない: %w", err)
	}

	children := map[int][]int{}

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			// 読む間に終わったプロセス。飛ばす。
			continue
		}

		ppid, err := parentPID(string(data))
		if err != nil {
			continue
		}

		children[ppid] = append(children[ppid], pid)
	}

	for _, list := range children {
		sort.Ints(list)
	}

	return children, nil
}

// parentPID は /proc/<pid>/stat の 4 番目の欄（親 pid）を返す。
//
// 2 番目の欄（comm）は括弧で囲まれ、空白や括弧を含みうる。**最後の `)` より
// 後ろを空白で分けて読む。**
func parentPID(stat string) (int, error) {
	end := strings.LastIndexByte(stat, ')')
	if end < 0 {
		return 0, errors.New("stat の形が違う")
	}

	fields := strings.Fields(stat[end+1:])
	if len(fields) < 2 {
		return 0, errors.New("stat の欄が足りない")
	}

	// fields[0] が状態、fields[1] が親 pid。
	return strconv.Atoi(fields[1])
}

// exeOf は /proc/<pid>/exe の指す実体を返す。読めなければ空文字。
func exeOf(pid int) string {
	target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return ""
	}

	// 実行中に置き換えられた実行ファイルには " (deleted)" が付く。
	return strings.TrimSuffix(target, " (deleted)")
}
