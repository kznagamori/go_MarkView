package config

// dirPerm は設定ディレクトリのパーミッション（IMP-152）。
//
// Windows では ACL で保護され、パーミッションビットは実質的に無視される。
// %TEMP% は利用者ごとに分かれており、他の利用者からは見えない。
const dirPerm = 0o755

// dirName は設定ディレクトリの名前を返す（UI-112）。
//
// %TEMP% が利用者ごとに分かれているため、名前に利用者を含めない。
func dirName() string {
	return "MarkView"
}
