package document

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// 番兵エラー（IMP-021）。呼び出し側は errors.Is で判定する。
//
// 文言は開発者向けの英語とする。利用者に見せる文言は app.go が組み立てる
// （IMP-315, UI-024）。
var (
	ErrNotFound     = errors.New("file not found")
	ErrPermission   = errors.New("permission denied")
	ErrNotMarkdown  = errors.New("not a markdown file")
	ErrTooLarge     = errors.New("file exceeds the maximum size")
	ErrNeedsConfirm = errors.New("file requires confirmation before rendering")
)

// サイズ閾値（IMP-101, FR-016）。
//
// 仕様上の「10 MB」「50 MB」は 2 進接頭辞として解釈する。
// **閾値ちょうどは超過ではない。**
const (
	ConfirmThreshold int64 = 10 << 20 // 10 MiB = 10,485,760 バイト
	MaxSize          int64 = 50 << 20 // 50 MiB = 52,428,800 バイト
)

// Document は表示対象の 1 文書を表す（IMP-100）。
type Document struct {
	Path         string             // 絶対パス（IMP-025）
	Size         int64              // ファイルの実バイト数
	HTML         string             // サニタイズ済みの本文 HTML
	Headings     []renderer.Heading // アウトライン（FR-040）
	LineCount    int                // 総行数（UI-060 の表示に使う）
	NeedsMermaid bool               // Mermaid の遅延ロード判定（AR-021）
	NeedsKaTeX   bool               // KaTeX の遅延ロード判定（AR-021）
	Warnings     []Warning          // 描画は継続するが利用者に伝える事象
}

// WarningKind は警告の種別（IMP-100）。
type WarningKind int

const (
	WarnInvalidEncoding WarningKind = iota // 不正な UTF-8 を置換した（FR-021）
	WarnTruncatedTree                      // ツリーの件数上限に達した（FR-032）
)

// Warning は FR-110 のうち「描画を継続する」事象を表す（IMP-100）。
//
// 利用者に見せる文言は app.go が Kind から組み立てる（IMP-315）。
// Detail は種別だけでは足りない場合の補足（件数など）に使う。
type Warning struct {
	Kind   WarningKind
	Detail string
}

// LoadOptions は読み込みの指示（IMP-102）。
type LoadOptions struct {
	// Confirmed が true の場合、ConfirmThreshold を超えていても描画する。
	// FR-016 の「Open anyway」に対応する。
	Confirmed bool
}

// SizeError はサイズ超過を、判断に必要な数値とともに伝える（IMP-102）。
//
// 呼び出し側は状態画面（UI-052）に実サイズと閾値を表示できる。
type SizeError struct {
	Path  string
	Size  int64
	Limit int64
	Err   error // ErrTooLarge または ErrNeedsConfirm
}

func (e *SizeError) Error() string {
	return fmt.Sprintf("%s: %v (%d bytes, limit %d bytes)", e.Path, e.Err, e.Size, e.Limit)
}

func (e *SizeError) Unwrap() error { return e.Err }

// Load はファイルを読み込み、変換して Document を返す（IMP-102）。
//
// 返しうるエラー: ErrNotFound / ErrPermission / ErrNotMarkdown /
// ErrTooLarge / ErrNeedsConfirm / 変換エラー。
// サイズ超過は *SizeError でラップして返す。
//
// **検査の順序は IMP-102 が固定している。** 拡張子 → 存在と権限 → サイズ →
// 読み込み → 正規化 → 変換の順で、途中で失敗したら以降は行わない。
func Load(r *renderer.Renderer, path string, opts LoadOptions) (*Document, error) {
	if !mdfile.IsMarkdown(path) {
		return nil, fmt.Errorf("%s: %w", path, ErrNotMarkdown)
	}

	// 内部で保持するパスは常に絶対パスとする（IMP-025）。
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, ErrNotFound)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, classifyError(abs, err)
	}
	if info.IsDir() {
		// 拡張子が Markdown のディレクトリ。読める文書ではない。
		return nil, fmt.Errorf("%s: %w", abs, ErrNotMarkdown)
	}

	size := info.Size()
	if limit, err := checkSize(size, opts.Confirmed); err != nil {
		return nil, &SizeError{Path: abs, Size: size, Limit: limit, Err: err}
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, classifyError(abs, err)
	}

	text, replaced := Normalize(raw)

	// 相対パスの基準は表示中ファイルのディレクトリとする（AR-042）。
	// ツリールートを基準にしてはならない。
	res, err := r.Render(text, filepath.Dir(abs))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}

	doc := &Document{
		Path:         abs,
		Size:         size,
		HTML:         res.HTML,
		Headings:     res.Headings,
		LineCount:    CountLines(text),
		NeedsMermaid: res.NeedsMermaid,
		NeedsKaTeX:   res.NeedsKaTeX,

		// 警告がなくても nil ではなく空スライスを返す。JSON で [] になり、
		// フロントエンドが null を場合分けせずに済む（Headings と同じ扱い）。
		Warnings: []Warning{},
	}

	if replaced {
		doc.Warnings = append(doc.Warnings, Warning{Kind: WarnInvalidEncoding})
	}

	return doc, nil
}

// checkSize はサイズ閾値を判定する（IMP-101, FR-016）。
//
// 超過していなければ err は nil。limit は err に対応する閾値であり、
// 呼び出し側が SizeError に載せる。
//
// **閾値ちょうどは超過ではない。** また ErrTooLarge は Confirmed でも覆らない。
// この 2 点がこの機能で最も間違えやすい（UT-104）。
func checkSize(size int64, confirmed bool) (limit int64, err error) {
	switch {
	case size > MaxSize:
		return MaxSize, ErrTooLarge
	case size > ConfirmThreshold && !confirmed:
		return ConfirmThreshold, ErrNeedsConfirm
	default:
		return 0, nil
	}
}

// classifyError は os のエラーを番兵エラーへ分類する（IMP-021, FR-110）。
//
// 分類できないものは包んでそのまま返す。呼び出し側は errors.Is で判別できず、
// 汎用のエラー表示になる（IMP-315）。
func classifyError(path string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s: %w", path, ErrNotFound)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("%s: %w", path, ErrPermission)
	default:
		return fmt.Errorf("%s: %w", path, err)
	}
}
