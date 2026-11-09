package main

// editwork.go — generated/edit/ に置く、画像の差し替えの文書と、コミットされたファイルの写し（E2E-012 の表）。
//
//   - swap.md / swap.png / swap-blue.png: 埋め込み画像と画像ファイルの差し替え（E2E-236, E2E-394。BUG-026）
//   - work/: コミットされたファイルの写し（E2E-206）。ファイルを書き換える手順は、コミットされたファイルではなく
//     この写しに対して行う（E2E-352 / E2E-354 の work/media.md、E2E-362 の work/tables.md）
//
// **写しは -edit のたびに、コミットされたファイルから写し直す。中身をリテラルで持たない**——元を直したときに
// 食い違うため（E2E-012）。

import (
	"fmt"
	"image/color"
	"os"
	"path"
	"path/filepath"
)

// e2eDir はコミットされた検証用データの場所（work/ の写しの元）。スクリプトはリポジトリの直下で実行する。
var e2eDir = filepath.Join("testdata", "e2e")

// swapMD は swap.md（E2E-236 の手順 6〜9）。赤い swap.png を埋め込む。
const swapMD = `# Swap

赤い画像（swap.png。400 x 100）を埋め込んだ文書です（E2E-236。BUG-026）。
青い画像（swap-blue.png。500 x 100）を swap.png に上書きしてから F5 を押すと、青に変わります。

![swap](swap.png)
`

// swapFiles は swap.md と 2 つの画像を返す。**大きさを変えておく**（images.go の solidPNG）。
func swapFiles() ([]editFile, error) {
	red, err := solidPNG(400, 100, color.NRGBA{R: 0xd1, G: 0x24, B: 0x2f, A: 0xff})
	if err != nil {
		return nil, fmt.Errorf("swap.png を作れない: %w", err)
	}

	blue, err := solidPNG(500, 100, color.NRGBA{R: 0x09, G: 0x69, B: 0xda, A: 0xff})
	if err != nil {
		return nil, fmt.Errorf("swap-blue.png を作れない: %w", err)
	}

	return []editFile{
		{path: "swap.md", content: swapMD},
		{path: "swap.png", content: red},
		{path: "swap-blue.png", content: blue},
	}, nil
}

// workSources は work/ に写すもの（e2eDir からの相対パス。/ 区切り）。末尾が / のものはディレクトリの中の
// ファイルをすべて写す（サブディレクトリは無い）。
//
// media.md が埋め込む画像（docs/img/ と、11 節の images/alpha.png）を一緒に写す。tables.md は画像を持たない。
var workSources = []string{
	"media.md",
	"docs/img/",
	"images/alpha.png",
	"tables.md",
}

// workFiles は work/ の写しを返す。
func workFiles() ([]editFile, error) {
	var files []editFile

	for _, source := range workSources {
		if source[len(source)-1] != '/' {
			file, err := workFile(source)
			if err != nil {
				return nil, err
			}

			files = append(files, file)

			continue
		}

		entries, err := os.ReadDir(filepath.Join(e2eDir, filepath.FromSlash(source)))
		if err != nil {
			return nil, fmt.Errorf("写しの元を読めない (%s): %w", source, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			file, err := workFile(path.Join(source, entry.Name()))
			if err != nil {
				return nil, err
			}

			files = append(files, file)
		}
	}

	return files, nil
}

// workFile は e2eDir の source を読み、work/ の下の同じ相対パスに置く editFile を返す。
func workFile(source string) (editFile, error) {
	content, err := os.ReadFile(filepath.Join(e2eDir, filepath.FromSlash(source)))
	if err != nil {
		return editFile{}, fmt.Errorf("写しの元を読めない (%s): %w", source, err)
	}

	return editFile{path: path.Join("work", source), content: string(content)}, nil
}
