// Package buildinfo は、ビルド時に実行ファイルへ埋め込まれた情報を提供する。
//
// ここで公開する値は、コマンドラインの --version（FR-012）と
// アプリケーション情報ダイアログ（FR-100）の双方から参照される。
package buildinfo

// ビルド時に ldflags の -X で上書きする変数（IMP-180、BR-030）。
//
// 既定値は開発ビルド用である。リリース成果物では 3 つとも必ず上書きされ、
// Version が "dev" のまま配布される事故は E2E-102 のケース 3 で検出する。
//
// 上書きは以下の形で行う（BR-010 の -ldflags に含める）。
//
//	-X github.com/kznagamori/go_MarkView/internal/buildinfo.Version=v1.0.0
//	-X github.com/kznagamori/go_MarkView/internal/buildinfo.Commit=a1b2c3d
//	-X github.com/kznagamori/go_MarkView/internal/buildinfo.BuildTime=2026-08-31T12:00:00Z
//
// -X は文字列変数にしか値を入れられないため、3 つとも string のままとする。
// 時刻を time.Time で保持しない理由もこれにあたる。
var (
	// Version は Git タグ名（v1.2.3 形式）。タグがない場合は "dev" のままとする。
	Version = "dev"

	// Commit は短縮コミットハッシュ（7 桁）。git rev-parse --short HEAD の結果。
	Commit = "unknown"

	// BuildTime は RFC 3339 形式（UTC）のビルド日時。
	BuildTime = "unknown"
)
