package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// 本ファイルは、Go モジュールのライセンスの収集を持つ（BR-040）。対象の OS とビルドタグごとに
// 依存を列挙し、モジュールのディレクトリからライセンスの全文を読む。同梱資産の側は main.go にある。
//
// main.go が 400 行の目安（IMP-011）を超えたため分けた。

// collectModules は実行ファイルに入る Go モジュールを集める。
func collectModules() ([]entry, error) {
	// パスをキーにして和集合を採る。
	dirs := map[string][2]string{} // path -> {version, dir}

	for _, t := range targets {
		lines, err := listDeps(t.goos, t.tags)
		if err != nil {
			return nil, err
		}

		for _, line := range lines {
			parts := strings.Split(line, "\t")
			if len(parts) != 3 {
				continue
			}

			path, version, dir := parts[0], parts[1], parts[2]

			// メインモジュールはバージョンを持たない。自分自身は載せない。
			if version == "" {
				continue
			}
			if dir == "" {
				return nil, fmt.Errorf("%s のモジュールが展開されていない。go mod download を実行する", path)
			}

			dirs[path] = [2]string{version, dir}
		}
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("依存モジュールが 1 つも見つからない")
	}

	paths := make([]string, 0, len(dirs))
	for p := range dirs {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	out := make([]entry, 0, len(paths))
	for _, p := range paths {
		version, dir := dirs[p][0], dirs[p][1]

		text, err := readLicense(dir)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}

		out = append(out, entry{name: p, version: version, kind: detect(text), text: text})
	}

	return out, nil
}

// listDeps は 1 つのプラットフォームについて依存モジュールを列挙する。
func listDeps(goos, tags string) ([]string, error) {
	args := []string{"list", "-deps", "-f", "{{if .Module}}{{.Module.Path}}\t{{.Module.Version}}\t{{.Module.Dir}}{{end}}"}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, ".")

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "GOOS="+goos)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list (GOOS=%s): %v: %s", goos, err, strings.TrimSpace(stderr.String()))
	}

	var lines []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	return lines, nil
}

// readLicense はモジュールのディレクトリからライセンス全文を読む。
//
// **見つからなければエラーにする。** 黙って省くと、表示されないまま
// 配布してしまう（FR-101 は MUST）。
//
// 主たるライセンスに続けて、同じディレクトリにある NOTICE・PATENTS・
// 副次的なライセンスも併記する。**Apache-2.0 は NOTICE の再頒布を求めており**
// （gopkg.in/yaml.v2）、golang.org/x/* の PATENTS も許諾の一部である。
// 主たる 1 つだけを載せると、条件を満たさないまま配布することになる。
func readLicense(dir string) (string, error) {
	var primary string
	var primaryName string

	for _, name := range licenseNames {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			primary = normalize(string(data))
			primaryName = name

			break
		}
	}

	if primaryName == "" {
		return "", fmt.Errorf("ライセンスファイルが見つからない: %s", dir)
	}

	extras, err := extraFiles(dir, primaryName)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(primary)

	for _, name := range extras {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}

		fmt.Fprintf(&b, "\n\n----- %s -----\n\n%s", name, normalize(string(data)))
	}

	return b.String(), nil
}

// extraFiles は併記すべき副次的なファイルの名前を返す。
func extraFiles(dir, primary string) ([]string, error) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, item := range items {
		if item.IsDir() {
			continue
		}

		name := item.Name()
		if strings.EqualFold(name, primary) {
			continue
		}

		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "NOTICE") || strings.HasPrefix(upper, "PATENTS") ||
			strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "LICENCE") ||
			strings.HasPrefix(upper, "COPYING") {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names, nil
}
