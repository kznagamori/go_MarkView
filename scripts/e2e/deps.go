package main

import (
	"debug/elf"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// Ubuntu 24.04 が備える glibc（E2E-106 ケース 4）。
//
// これを超える版を要求していると、宣言している対象環境で動かない。
// ランナーを固定してビルドする理由もここにある（BR-050）。
const (
	ubuntuGlibcMajor = 2
	ubuntuGlibcMinor = 39
)

// 開発用・テスト用のライブラリらしさ（E2E-106 ケース 3）。
//
// 配布物がこれらに依存していれば、ビルド環境の何かが漏れている。
var suspiciousLibraries = []string{"gtest", "gmock", "asan", "ubsan", "tsan", "valgrind", "-dev.so"}

// checkDependencies は Linux の依存ライブラリを確かめる（E2E-106）。
func checkDependencies(result *report, exe string) {
	names := []string{"webkit2gtk の版", "not found の有無", "想定外のライブラリ", "glibc の要求版"}

	file, err := elf.Open(exe)
	if err != nil {
		// ELF でなければ Windows の成果物。この確認は Linux 向け（AR-003）。
		for i, name := range names {
			result.skip("E2E-106", i+1, name, "ELF ではない（Windows の成果物）")
		}

		return
	}
	defer file.Close() //nolint:errcheck // 読むだけ

	libraries, err := file.ImportedLibraries()
	if err != nil {
		for i, name := range names {
			result.fail("E2E-106", i+1, name, "DT_NEEDED を読めない: %v", err)
		}

		return
	}

	sort.Strings(libraries)

	// ケース 1: **-tags webkit2_41 の指定漏れはここでしか捕まらない**（BR-010）。
	// 4.0 系にリンクされると Ubuntu 24.04 で起動できない配布物ができる。
	has41 := anyHasPrefix(libraries, "libwebkit2gtk-4.1")
	has40 := anyHasPrefix(libraries, "libwebkit2gtk-4.0")

	result.verify("E2E-106", 1, names[0], has41 && !has40,
		fmt.Sprintf("4.1=%t / 4.0=%t / 依存 %d 件", has41, has40, len(libraries)))

	checkLoader(result, exe, names[1])

	// ケース 3: 想定外のライブラリ。
	var suspicious []string

	for _, library := range libraries {
		lower := strings.ToLower(library)
		for _, mark := range suspiciousLibraries {
			if strings.Contains(lower, mark) {
				suspicious = append(suspicious, library)
			}
		}
	}

	listed := strings.Join(libraries, " ")
	if listed == "" {
		listed = "依存ライブラリなし（静的リンク）"
	}

	result.verify("E2E-106", 3, names[2], len(suspicious) == 0,
		emptyOr(suspicious, listed, "想定外: "))

	checkGlibc(result, file, names[3])
}

// checkLoader は ldd を実行し、解決できない依存が無いことを見る（E2E-106 ケース 2）。
//
// **ここだけは実際にローダへ聞く必要がある。** ELF の DT_NEEDED を読んでも
// 「その名前のライブラリが実行環境に在るか」は分からない。
func checkLoader(result *report, exe, name string) {
	if runtime.GOOS != "linux" {
		result.skip("E2E-106", 2, name, "ldd は Linux でしか実行できない")

		return
	}

	output, err := exec.Command("ldd", exe).CombinedOutput()
	if err != nil && len(output) == 0 {
		result.skip("E2E-106", 2, name, "ldd を実行できない: "+err.Error())

		return
	}

	var unresolved []string

	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "not found") {
			unresolved = append(unresolved, strings.TrimSpace(line))
		}
	}

	result.verify("E2E-106", 2, name, len(unresolved) == 0,
		emptyOr(unresolved, "not found は 0 件", "解決できない: "))
}

// checkGlibc は要求している glibc の最大版を見る（E2E-106 ケース 4）。
func checkGlibc(result *report, file *elf.File, name string) {
	symbols, err := file.ImportedSymbols()

	// 動的シンボルが無いのは静的リンクということ。失敗ではない。
	// **その場合でも webkit2gtk が要る**ので、そちらはケース 1 が見ている。
	if errors.Is(err, elf.ErrNoSymbols) {
		result.note("E2E-106", 4, name, "動的シンボルが無い（静的リンク）")

		return
	}

	if err != nil {
		result.fail("E2E-106", 4, name, "シンボルを読めない: %v", err)

		return
	}

	major, minor := 0, 0
	found := ""

	for _, symbol := range symbols {
		gotMajor, gotMinor, ok := parseGlibcVersion(symbol.Version)
		if !ok {
			continue
		}

		if gotMajor > major || (gotMajor == major && gotMinor > minor) {
			major, minor = gotMajor, gotMinor
			found = symbol.Version
		}
	}

	if found == "" {
		result.note("E2E-106", 4, name, "glibc のバージョン付きシンボルが無い（静的リンク？）")

		return
	}

	ok := major < ubuntuGlibcMajor ||
		(major == ubuntuGlibcMajor && minor <= ubuntuGlibcMinor)

	result.verify("E2E-106", 4, name, ok,
		fmt.Sprintf("要求 %s / Ubuntu 24.04 は GLIBC_%d.%d", found, ubuntuGlibcMajor, ubuntuGlibcMinor))
}

// parseGlibcVersion は "GLIBC_2.34" を (2, 34) にする。
func parseGlibcVersion(version string) (major, minor int, ok bool) {
	const prefix = "GLIBC_"

	if !strings.HasPrefix(version, prefix) {
		return 0, 0, false
	}

	parts := strings.SplitN(strings.TrimPrefix(version, prefix), ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}

	return major, minor, true
}

func anyHasPrefix(list []string, prefix string) bool {
	for _, got := range list {
		if strings.HasPrefix(got, prefix) {
			return true
		}
	}

	return false
}
