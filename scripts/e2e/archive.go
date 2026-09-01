package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// BR-060 / NFR-021 の閾値。**超えても失敗させない**（E2E-107）。
const (
	maxExeSize     int64 = 25 << 20 // 25 MiB
	maxArchiveSize int64 = 12 << 20 // 12 MiB
)

// target は BR-020 が定める 4 種の成果物。
type target struct {
	goos string
	arch string
	ext  string
}

var targets = []target{
	{"windows", "amd64", ".zip"},
	{"windows", "arm64", ".zip"},
	{"linux", "amd64", ".tar.gz"},
	{"linux", "arm64", ".tar.gz"},
}

func (t target) label() string { return t.goos + "/" + t.arch }

func (t target) archive(version string) string {
	return fmt.Sprintf("MarkView-%s-%s-%s%s", version, t.goos, t.arch, t.ext)
}

// exeName は BR-020 が定める実行ファイル名。
func (t target) exeName() string {
	if t.goos == "windows" {
		return "MarkView.exe"
	}

	return "MarkView"
}

// entry はアーカイブの中身 1 件。
type entry struct {
	name string
	size int64
	mode fs.FileMode
	dir  bool
}

func runArchives(args []string) (*report, error) {
	flags := flag.NewFlagSet("archives", flag.ExitOnError)
	dir := flags.String("dir", "dist", "配布物のあるディレクトリ")
	version := flags.String("version", "", "タグ名（例: v1.0.0-rc.1）")
	sums := flags.String("checksums", "", "チェックサムの一覧（既定は <dir>/checksums.txt）")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	if *version == "" {
		return nil, errors.New("-version が要る（例: -version v1.0.0-rc.1）")
	}

	if *sums == "" {
		*sums = filepath.Join(*dir, "checksums.txt")
	}

	fmt.Printf("配布物 : %s\n", *dir)
	fmt.Printf("タグ   : %s\n", *version)

	result := &report{}

	for _, t := range targets {
		checkArchive(result, filepath.Join(*dir, t.archive(*version)), t)
	}

	checkChecksums(result, *dir, *sums, *version)

	return result, nil
}

// checkArchive は 1 つのアーカイブを検査する（E2E-101, E2E-107）。
func checkArchive(result *report, path string, t target) {
	name := t.label()

	info, err := os.Stat(path)
	if err != nil {
		result.fail("E2E-101", 1, name+" の展開", "アーカイブが無い: %v", err)

		return
	}

	entries, err := readArchive(path, t)
	if err != nil {
		result.fail("E2E-101", 1, name+" の展開", "%v", err)

		return
	}

	result.verify("E2E-101", 1, name+" の展開", true, fmt.Sprintf("%d 件", len(entries)))

	// ケース 2 / 5: 3 点のみで、設定ファイルやランタイムを含まない。
	want := map[string]bool{t.exeName(): true, "LICENSE": true, "README.md": true}

	var got, extra, missing []string

	for _, e := range entries {
		got = append(got, e.name)
		if !want[e.name] {
			extra = append(extra, e.name)
		}
	}

	for n := range want {
		if !contains(got, n) {
			missing = append(missing, n)
		}
	}

	sort.Strings(got)
	sort.Strings(extra)
	sort.Strings(missing)

	result.verify("E2E-101", 2, name+" の内容", len(extra) == 0 && len(missing) == 0,
		describeContents(got, extra, missing))

	// ケース 3: ディレクトリ階層を作らない（BR-021）。
	var nested []string

	for _, e := range entries {
		if e.dir || strings.Contains(e.name, "/") {
			nested = append(nested, e.name)
		}
	}

	result.verify("E2E-101", 3, name+" の階層", len(nested) == 0,
		emptyOr(nested, "階層なし", "階層がある: "))

	// ケース 4: 実行可能ビット（Linux のみ）。
	if t.goos == "windows" {
		result.skip("E2E-101", 4, name+" の実行ビット", "zip は権限を持たない（Windows 対象）")
	} else {
		exe := find(entries, t.exeName())
		result.verify("E2E-101", 4, name+" の実行ビット", exe != nil && exe.mode&0o111 != 0,
			modeOf(exe))
	}

	// ケース 5 は 2 と同じ判定になるため、観点だけを別に記録する。
	result.verify("E2E-101", 5, name+" の同梱物", len(extra) == 0,
		emptyOr(extra, "設定ファイル・ランタイムなし", "余分: "))

	// E2E-107: 記録に留める（BR-060）。**表の # の順に並べる。**
	if exe := find(entries, t.exeName()); exe != nil {
		checkSize(result, name+" の実行ファイル", 1, exe.size, maxExeSize)
	}

	checkSize(result, name+" のアーカイブ", 2, info.Size(), maxArchiveSize)
}

// checkSize は閾値を超えても失敗させない（BR-060, E2E-107）。
//
// 資産の更新（BR-043）でリリースが機械的に止まると、利用者に新しい版を
// 届けられなくなる。担保は「計測を常に見えるようにすること」で行う。
func checkSize(result *report, name string, number int, size, limit int64) {
	detail := fmt.Sprintf("%d バイト (%.1f MiB / 上限 %.0f MiB)",
		size, float64(size)/(1<<20), float64(limit)/(1<<20))

	if size > limit {
		result.warn("E2E-107", number, name, detail+" **超過**")

		return
	}

	result.note("E2E-107", number, name, detail)
}

// checkChecksums は checksums.txt を検査する（E2E-108）。
func checkChecksums(result *report, dir, sums, version string) {
	data, err := os.ReadFile(sums)
	if err != nil {
		result.fail("E2E-108", 1, "checksums.txt の存在", "%v", err)
		result.fail("E2E-108", 2, "記載値と実ファイル", "検証できない")
		result.fail("E2E-108", 3, "成果物の数", "検証できない")

		return
	}

	// sha256sum の出力形式（<16 進>  <ファイル名>）を読む。
	listed := map[string]string{}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}

		// GNU coreutils はバイナリモードで名前の前に * を置く。
		listed[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}

	var missing, mismatch []string

	for _, t := range targets {
		name := t.archive(version)

		want, ok := listed[name]
		if !ok {
			missing = append(missing, name)

			continue
		}

		got, err := sha256Of(filepath.Join(dir, name))
		if err != nil {
			mismatch = append(mismatch, name+": "+err.Error())

			continue
		}

		if got != want {
			mismatch = append(mismatch, fmt.Sprintf("%s: %s != %s", name, got[:16], want[:16]))
		}
	}

	result.verify("E2E-108", 1, "全成果物の記載", len(missing) == 0,
		emptyOr(missing, fmt.Sprintf("%d 件すべて記載", len(targets)), "記載が無い: "))

	result.verify("E2E-108", 2, "記載値と実ファイル", len(mismatch) == 0,
		emptyOr(mismatch, "すべて一致", "不一致: "))

	// ケース 3: 4 種が揃っている。
	var absent []string

	for _, t := range targets {
		if _, err := os.Stat(filepath.Join(dir, t.archive(version))); err != nil {
			absent = append(absent, t.label())
		}
	}

	result.verify("E2E-108", 3, "成果物の数", len(absent) == 0,
		emptyOr(absent, fmt.Sprintf("%d 種すべて揃っている", len(targets)), "無い: "))
}

func readArchive(file string, t target) ([]entry, error) {
	if t.ext == ".zip" {
		return readZip(file)
	}

	return readTarGz(file)
}

func readZip(file string) ([]entry, error) {
	reader, err := zip.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("zip を開けない: %w", err)
	}
	defer reader.Close() //nolint:errcheck // 読むだけ

	entries := make([]entry, 0, len(reader.File))

	for _, f := range reader.File {
		name := path.Clean(strings.TrimPrefix(filepath.ToSlash(f.Name), "./"))
		entries = append(entries, entry{
			name: name,
			size: int64(f.UncompressedSize64),
			mode: f.Mode(),
			dir:  f.FileInfo().IsDir(),
		})
	}

	return entries, nil
}

func readTarGz(file string) ([]entry, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("tar.gz を開けない: %w", err)
	}
	defer f.Close() //nolint:errcheck // 読むだけ

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip を展開できない: %w", err)
	}
	defer gz.Close() //nolint:errcheck // 読むだけ

	var entries []entry

	reader := tar.NewReader(gz)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("tar を読めない: %w", err)
		}

		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		entries = append(entries, entry{
			name: name,
			size: header.Size,
			mode: header.FileInfo().Mode(),
			dir:  header.Typeflag == tar.TypeDir,
		})
	}

	return entries, nil
}

func sha256Of(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // 読むだけ

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(sum.Sum(nil)), nil
}

func find(entries []entry, name string) *entry {
	for i := range entries {
		if entries[i].name == name {
			return &entries[i]
		}
	}

	return nil
}

func contains(list []string, want string) bool {
	for _, got := range list {
		if got == want {
			return true
		}
	}

	return false
}

func emptyOr(list []string, whenEmpty, prefix string) string {
	if len(list) == 0 {
		return whenEmpty
	}

	return prefix + strings.Join(list, ", ")
}

func describeContents(got, extra, missing []string) string {
	detail := strings.Join(got, ", ")

	if len(extra) > 0 {
		detail += " / 余分: " + strings.Join(extra, ", ")
	}

	if len(missing) > 0 {
		detail += " / 不足: " + strings.Join(missing, ", ")
	}

	return detail
}

func modeOf(e *entry) string {
	if e == nil {
		return "実行ファイルが無い"
	}

	return e.mode.String()
}
