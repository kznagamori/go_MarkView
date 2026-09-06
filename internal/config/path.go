package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// fileName は設定ファイルの名前（UI-112）。
const fileName = "config.json"

// Dir は設定ディレクトリの絶対パスを返し、なければ作る（IMP-152, UI-112）。
//
// 保存先は **OS / ユーザのテンポラリディレクトリに限る**。%APPDATA% や
// ~/.config、レジストリには書かない（NFR-033）。テンポラリに置くのは、
// 設定が消えても既定値で動く（UI-113）ことを前提にしているためである。
//
// OS ごとの差異（ディレクトリ名とパーミッション）は、ファイル名サフィックスで
// 分けた dirName / dirPerm が持つ（IMP-031）。
func Dir() (string, error) {
	dir := dirPath()

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", fmt.Errorf("cannot create the configuration directory: %w", err)
	}
	return dir, nil
}

// Path は設定ファイルの絶対パスを返す（IMP-152）。
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

// dirPath は設定ディレクトリのパスを組み立てる。作成はしない。
//
// 作成と分けているのは、保存先の決め方だけをテストできるようにするため
// である（UT-501）。作成まで行うと、テストが実際の /tmp にディレクトリを
// 残すことになる。
func dirPath() string {
	return filepath.Join(os.TempDir(), dirName())
}
