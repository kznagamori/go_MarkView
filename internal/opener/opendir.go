package opener

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotDir は、ディレクトリでないものをファイルマネージャで開こうとしたことを表す（NFR-035）。
var ErrNotDir = errors.New("not a directory")

// OpenDir は OS の既定のファイルマネージャでディレクトリを開く（FR-036, IMP-173）。
// dir は絶対パスで、ディレクトリであること。
//
// **ディレクトリだけを OS へ渡す。** 起動するコマンドは OpenFile と同じ既定のアプリケーションの経路
// （Windows は rundll32.exe url.dll,FileProtocolHandler、Linux は xdg-open）であり、**ファイルを渡すと
// 実行ファイルなら実行される**（NFR-035）。呼び出し側はツリーの操作の検査（IMP-135）でディレクトリと
// 確かめてから呼ぶが、**ここでも起動の直前にもう一度確かめる**（検査と起動の間のすり替わりを狭める）。
//
//   - 絶対パスでなければ ErrNotFound（実在する相対パスも拒む。作業ディレクトリで対象が変わる）
//   - os.Stat（**リンク先の実体**）で、無ければ ErrNotFound、ディレクトリでなければ ErrNotDir。
//     種類は名前で見ない（"x.exe" という名前のディレクトリは開く。UT-706 のケース 8）
//   - 渡すのは dir そのもの 1 つ（シェルを経由しない。IMP-170）
//
// Windows で explorer.exe を使わない（引数の "," を区切りとして読むなど解釈が特殊である）。
// ファイルを選んだ状態で開く手段（explorer.exe /select, など）も持たない。
func OpenDir(dir string) error {
	if dir == "" || !filepath.IsAbs(dir) {
		return fmt.Errorf("%s: %w", dir, ErrNotFound)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("%s: %w", dir, ErrNotFound)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %w", dir, ErrNotDir)
	}

	return open(dir)
}
