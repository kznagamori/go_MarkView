//go:build !windows

package config

import (
	"os"
	"strconv"
)

// dirPerm は設定ディレクトリのパーミッション（IMP-152, UI-112）。
//
// Linux の /tmp は全利用者で共有されるため、他の利用者から読めないようにする。
// ファイルは os.CreateTemp が 0600 で作り、Rename でその値が引き継がれる。
const dirPerm = 0o700

// dirName は設定ディレクトリの名前を返す（UI-112）。
//
// 共有されるテンポラリ領域に置くため、名前に利用者 ID を含める。
// 他の利用者の設定を読み書きしてはならない。
func dirName() string {
	return "MarkView-" + strconv.Itoa(os.Getuid())
}
