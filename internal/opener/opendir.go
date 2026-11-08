package opener

import "errors"

// このファイルは v1.2.0 のディレクトリをファイルマネージャで開く処理（IMP-173。4.75.0）の宣言である。

// ErrNotDir は、ディレクトリでないものをファイルマネージャで開こうとしたことを表す（NFR-035）。
var ErrNotDir = errors.New("not a directory")

// errNotImplemented は v1.2.0 のスタブが返すエラー。実装で消す。
var errNotImplemented = errors.New("not implemented (v1.2.0)")

// OpenDir は OS の既定のファイルマネージャでディレクトリを開く（FR-036）。
// dir は絶対パスで、ディレクトリであること。
//
// TODO(v1.2.0): 未実装（テストの先行実装で置いたスタブ）。何も起動しない。
func OpenDir(dir string) error {
	return errNotImplemented
}
