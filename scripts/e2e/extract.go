package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// extract はアーカイブを展開し、実行ファイルのパスを返す（E2E-011）。
//
// **配布物そのものを動かす。** 手元でビルドしたものや wails dev の起動を
// 検証の対象にしない。展開の経路まで含めて確かめる意味もある。
//
// アーカイブは BR-021 により階層を持たない。念のため、名前に区切りを
// 含むものは弾く。悪意のあるアーカイブを想定した防御ではなく、
// **前提が崩れたことに気づくため**である。
func extract(archive, dest string) (string, error) {
	var (
		names []string
		err   error
	)

	switch {
	case strings.HasSuffix(archive, ".zip"):
		names, err = extractZip(archive, dest)
	case strings.HasSuffix(archive, ".tar.gz"):
		names, err = extractTarGz(archive, dest)
	default:
		return "", fmt.Errorf("扱えない形式: %s", archive)
	}

	if err != nil {
		return "", err
	}

	for _, name := range names {
		if name == "MarkView" || name == "MarkView.exe" {
			return filepath.Join(dest, name), nil
		}
	}

	return "", fmt.Errorf("%s に実行ファイルが無い（%s）", archive, strings.Join(names, ", "))
}

func extractZip(archive, dest string) ([]string, error) {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return nil, fmt.Errorf("zip を開けない: %w", err)
	}
	defer reader.Close() //nolint:errcheck // 読むだけ

	var names []string

	for _, file := range reader.File {
		name, err := safeName(file.Name)
		if err != nil {
			return nil, err
		}

		if file.FileInfo().IsDir() {
			continue
		}

		source, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("%s を開けない: %w", name, err)
		}

		if err := writeOut(filepath.Join(dest, name), source, file.Mode()); err != nil {
			source.Close() //nolint:errcheck

			return nil, err
		}

		source.Close() //nolint:errcheck

		names = append(names, name)
	}

	return names, nil
}

func extractTarGz(archive, dest string) ([]string, error) {
	file, err := os.Open(archive)
	if err != nil {
		return nil, fmt.Errorf("tar.gz を開けない: %w", err)
	}
	defer file.Close() //nolint:errcheck // 読むだけ

	gz, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("gzip を展開できない: %w", err)
	}
	defer gz.Close() //nolint:errcheck // 読むだけ

	var names []string

	reader := tar.NewReader(gz)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("tar を読めない: %w", err)
		}

		name, err := safeName(header.Name)
		if err != nil {
			return nil, err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		if err := writeOut(filepath.Join(dest, name), reader, header.FileInfo().Mode()); err != nil {
			return nil, err
		}

		names = append(names, name)
	}

	return names, nil
}

// safeName は階層を持たない名前だけを通す（BR-021）。
func safeName(raw string) (string, error) {
	name := path.Clean(strings.TrimPrefix(filepath.ToSlash(raw), "./"))

	if name == "." || name == "" {
		return "", fmt.Errorf("名前が空のエントリがある")
	}

	if strings.Contains(name, "/") || filepath.IsAbs(name) {
		return "", fmt.Errorf("階層を含むエントリがある: %s", raw)
	}

	return name, nil
}

func writeOut(path string, source io.Reader, mode os.FileMode) error {
	// 実行ビットを保つ（E2E-101 ケース 4）。Windows では無視される。
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return fmt.Errorf("書き出せない (%s): %w", path, err)
	}

	if _, err := io.Copy(out, source); err != nil {
		out.Close() //nolint:errcheck

		return fmt.Errorf("書き出せない (%s): %w", path, err)
	}

	return out.Close()
}
