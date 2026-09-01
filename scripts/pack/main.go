// pack は配布アーカイブを作る（BR-020, BR-021）。
//
//	go run ./scripts/pack -exe build/bin/MarkView.exe -os windows -arch amd64 -version v1.0.0
//	go run ./scripts/pack -exe build/bin/MarkView    -os linux   -arch amd64 -version v1.0.0
//
// **ランナーの tar / Compress-Archive を使わない。** 中身の要求（BR-021）は
// 「実行ファイルと最小限の文書のみ、階層を作らない、実行ビットが立っている」
// であり、これは道具によって結果が変わる。zip はそもそも権限を持たず、
// Windows で作った tar は所有者やモードがホストの都合を引きずる。E2E-101 が
// これらを 1 件ずつ検査する以上、**作る側も同じ規則で固定する**。
//
// 生成物の名前は BR-020 の表に従う。
//
//	MarkView-<version>-windows-amd64.zip
//	MarkView-<version>-linux-amd64.tar.gz
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// member はアーカイブへ入れる 1 件。
type member struct {
	name   string
	source string
	mode   int64
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pack:", err)
		os.Exit(1)
	}
}

func run() error {
	exe := flag.String("exe", "", "ビルドした実行ファイル")
	goos := flag.String("os", "", "windows か linux")
	arch := flag.String("arch", "", "amd64 か arm64")
	version := flag.String("version", "", "タグ名（例: v1.0.0）")
	out := flag.String("out", "dist", "出力先ディレクトリ")
	license := flag.String("license", "LICENSE", "同梱するライセンス")
	readme := flag.String("readme", "README.md", "同梱する README")
	flag.Parse()

	if *exe == "" || *goos == "" || *arch == "" || *version == "" {
		return errors.New("-exe / -os / -arch / -version はすべて要る")
	}

	if *goos != "windows" && *goos != "linux" {
		return fmt.Errorf("扱わない OS: %s", *goos)
	}

	// 実行ファイルの名前は OS で決まる（BR-020）。渡された名前は使わない。
	exeName := "MarkView"
	if *goos == "windows" {
		exeName = "MarkView.exe"
	}

	members := []member{
		{name: exeName, source: *exe, mode: 0o755},
		{name: "LICENSE", source: *license, mode: 0o644},
		{name: "README.md", source: *readme, mode: 0o644},
	}

	for _, m := range members {
		if _, err := os.Stat(m.source); err != nil {
			return fmt.Errorf("%s が無い: %w", m.name, err)
		}
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return fmt.Errorf("出力先を作れない: %w", err)
	}

	name := fmt.Sprintf("MarkView-%s-%s-%s", *version, *goos, *arch)

	var path string

	var err error

	if *goos == "windows" {
		path = filepath.Join(*out, name+".zip")
		err = writeZip(path, members)
	} else {
		path = filepath.Join(*out, name+".tar.gz")
		err = writeTarGz(path, members)
	}

	if err != nil {
		return err
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	fmt.Printf("%s  %d バイト (%.1f MiB)\n", path, info.Size(), float64(info.Size())/(1<<20))

	// ワークフローが後段へ渡せるようにしておく。
	if output := os.Getenv("GITHUB_OUTPUT"); output != "" {
		return appendOutput(output, fmt.Sprintf("archive=%s\nsize=%d\n", path, info.Size()))
	}

	return nil
}

// writeZip は Windows 向けの zip を書く。
//
// **ディレクトリのエントリを作らない**（BR-021）。名前に区切りを入れないため、
// 展開すると 3 つのファイルが並ぶだけになる。
func writeZip(path string, members []member) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("作成できない (%s): %w", path, err)
	}
	defer file.Close() //nolint:errcheck // 下で Close する

	archive := zip.NewWriter(file)

	for _, m := range members {
		header := &zip.FileHeader{
			Name:     m.name,
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		}
		// zip の権限は Windows では使われないが、Linux で展開したときに
		// 実行ビットが残るよう入れておく。
		header.SetMode(os.FileMode(m.mode))

		entry, err := archive.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("%s を追加できない: %w", m.name, err)
		}

		if err := copyInto(entry, m.source); err != nil {
			return err
		}
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("zip を閉じられない: %w", err)
	}

	return file.Close()
}

// writeTarGz は Linux 向けの tar.gz を書く。
//
// **モードと所有者を固定する。** ビルドしたホストのユーザ名や uid が
// 配布物に残らないようにする。
func writeTarGz(path string, members []member) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("作成できない (%s): %w", path, err)
	}
	defer file.Close() //nolint:errcheck // 下で Close する

	gz := gzip.NewWriter(file)
	archive := tar.NewWriter(gz)

	now := time.Now().UTC()

	for _, m := range members {
		info, err := os.Stat(m.source)
		if err != nil {
			return err
		}

		header := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     m.name,
			Size:     info.Size(),
			Mode:     m.mode,
			ModTime:  now,
			Uid:      0,
			Gid:      0,
			Uname:    "root",
			Gname:    "root",
			Format:   tar.FormatGNU,
		}

		if err := archive.WriteHeader(header); err != nil {
			return fmt.Errorf("%s を追加できない: %w", m.name, err)
		}

		if err := copyInto(archive, m.source); err != nil {
			return err
		}
	}

	if err := archive.Close(); err != nil {
		return fmt.Errorf("tar を閉じられない: %w", err)
	}

	if err := gz.Close(); err != nil {
		return fmt.Errorf("gzip を閉じられない: %w", err)
	}

	return file.Close()
}

func copyInto(w io.Writer, source string) error {
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("読めない (%s): %w", source, err)
	}
	defer file.Close() //nolint:errcheck // 読むだけ

	if _, err := io.Copy(w, file); err != nil {
		return fmt.Errorf("書き込めない (%s): %w", source, err)
	}

	return nil
}

func appendOutput(path, text string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	if _, err := file.WriteString(text); err != nil {
		file.Close() //nolint:errcheck

		return err
	}

	return file.Close()
}
