package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// registry は npm のレジストリ。**公式配布物だけを取る**（BR-042）。
const registry = "https://registry.npmjs.org"

// tarball の上限。壊れた応答や誤った URL で無限に読み続けないための歯止め。
const maxTarball = 200 << 20 // 200 MiB

// packument は /<pkg>/latest の応答のうち、必要な部分だけ。
//
// **latest は dist-tag であり、定義上は最新の安定版である。** プレリリース版は
// ここに現れない（BR-043 の「Mermaid / KaTeX 側のプレリリース版は対象外」）。
type packument struct {
	Version string `json:"version"`
	License string `json:"license"` // NFR-051 の判定に使う（BR-043）
	Dist    struct {
		Tarball string `json:"tarball"`
	} `json:"dist"`
}

// download は 1 件の資産を staging へ取り出す。
//
// 失敗は result.err に入れて返す。**エラーで止めない**（BR-043）。
func download(a asset, staging string, timeout time.Duration, before string) result {
	got := result{name: a.name, before: before, after: before}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	latest, err := fetchLatest(ctx, a.pkg)
	if err != nil {
		got.err = err

		return got
	}

	got.after = latest.Version
	got.source = latest.Dist.Tarball
	got.spdx = latest.License

	dest := filepath.Join(staging, a.dir)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		got.err = err

		return got
	}

	if err := extract(ctx, latest.Dist.Tarball, a, dest); err != nil {
		got.err = err

		return got
	}

	return got
}

func fetchLatest(ctx context.Context, pkg string) (packument, error) {
	var latest packument

	body, err := get(ctx, registry+"/"+pkg+"/latest")
	if err != nil {
		return latest, err
	}
	defer body.Close() //nolint:errcheck // 読むだけ

	if err := json.NewDecoder(body).Decode(&latest); err != nil {
		return latest, fmt.Errorf("%s のメタデータを解析できない: %w", pkg, err)
	}

	if latest.Version == "" || latest.Dist.Tarball == "" {
		return latest, fmt.Errorf("%s のメタデータに版か tarball が無い", pkg)
	}

	return latest, nil
}

// extract は tarball から必要なファイルだけを取り出す。
//
// **要るものが 1 つでも欠けたら失敗にする。** 版が上がって配置が変わった
// ことに気づかないまま、欠けた資産で配布するのが最悪の結末である。
func extract(ctx context.Context, url string, a asset, dest string) error {
	body, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close() //nolint:errcheck // 読むだけ

	gz, err := gzip.NewReader(io.LimitReader(body, maxTarball))
	if err != nil {
		return fmt.Errorf("%s を展開できない: %w", a.pkg, err)
	}
	defer gz.Close() //nolint:errcheck // 読むだけ

	seen := map[string]bool{}
	trees := map[string]int{}

	reader := tar.NewReader(gz)

	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("%s の tar を読めない: %w", a.pkg, err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		// npm の tarball は中身をすべて package/ の下に置く。
		name := strings.TrimPrefix(path.Clean(header.Name), "package/")

		if target, ok := a.files[name]; ok {
			if err := save(reader, dest, target); err != nil {
				return err
			}

			seen[name] = true

			continue
		}

		for prefix, into := range a.tree {
			if !strings.HasPrefix(name, prefix) {
				continue
			}

			target := into + strings.TrimPrefix(name, prefix)
			if err := save(reader, dest, target); err != nil {
				return err
			}

			trees[prefix]++
		}
	}

	for name := range a.files {
		if !seen[name] {
			return fmt.Errorf("%s の tarball に %s が無い（配置が変わった可能性）", a.pkg, name)
		}
	}

	for prefix := range a.tree {
		if trees[prefix] == 0 {
			return fmt.Errorf("%s の tarball に %s が無い（配置が変わった可能性）", a.pkg, prefix)
		}
	}

	return nil
}

// save は tar の 1 件を書き出す。
func save(reader io.Reader, dest, name string) error {
	// tarball の内容を信用して書き先を決めない。
	clean := path.Clean(name)
	if strings.HasPrefix(clean, "..") || path.IsAbs(clean) {
		return fmt.Errorf("置き先として扱えない名前: %s", name)
	}

	target := filepath.Join(dest, filepath.FromSlash(clean))

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	file, err := os.Create(target)
	if err != nil {
		return err
	}

	written, err := io.Copy(file, reader)
	if err != nil {
		file.Close() //nolint:errcheck

		return err
	}

	if err := file.Close(); err != nil {
		return err
	}

	if written == 0 {
		return fmt.Errorf("%s が空である", name)
	}

	return nil
}

func get(ctx context.Context, url string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("取得できない (%s): %w", url, err)
	}

	if response.StatusCode != http.StatusOK {
		response.Body.Close() //nolint:errcheck

		return nil, fmt.Errorf("取得できない (%s): HTTP %d", url, response.StatusCode)
	}

	return response.Body, nil
}
