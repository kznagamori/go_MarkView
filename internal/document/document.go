package document

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/localurl"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// 番兵エラー（IMP-021）。呼び出し側は errors.Is で判定する。
//
// 文言は開発者向けの英語とする。利用者に見せる文言は app.go が組み立てる
// （IMP-315, UI-024）。
//
// ErrNotSupported は v1.1.0 の「Markdown でない」の番兵を改名したもの（4.75.0。IMP-102）。開けるファイルが
// Markdown・図のファイル・画像ファイルの 3 種類になり、「Markdown でない」では理由を表せなくなった。
var (
	ErrNotFound     = errors.New("file not found")
	ErrPermission   = errors.New("permission denied")
	ErrNotSupported = errors.New("not a supported file")
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
	Path          string             // 絶対パス（IMP-025）
	Kind          mdfile.Kind        // 種類（IMP-105）。変換の分け方と編集の可否を決める（4.75.0）
	Size          int64              // ファイルの実バイト数
	HTML          string             // サニタイズ済みの本文 HTML
	Headings      []renderer.Heading // アウトライン（FR-040）
	LineCount     int                // 総行数（UI-060 の表示に使う）
	NeedsMermaid  bool               // Mermaid の遅延ロード判定（AR-021）
	NeedsKaTeX    bool               // KaTeX の遅延ロード判定（AR-021）
	NeedsPlantUML bool               // PlantUML の遅延ロード判定（AR-021, MD-085）
	Warnings      []Warning          // 描画は継続するが利用者に伝える事象
	Digest        [sha256.Size]byte  // 読み込んだ生バイト列の要約（FR-143 の「把握している内容」）
	RefKey        string             // この描画の目印の鍵（IMP-120）。描画のたびに作り直す
	TextRefs      bool               // 修飾の目印（text:<n>）を付けて描画したか（IMP-120。編集モードの間だけ真）
	NoDiagrams    bool               // 図のファイルで、図が 1 つも無い（FR-025, FR-026。IMP-126, IMP-127）
}

// Editable は編集モードを開始できる文書かを返す（IMP-100, FR-140 の表）。
// 不正なバイト列を置き換えた文書は、表示とファイルの内容が一致しないため偽とする。
// 画像ファイルも偽とする（書き換える対象を持たない。FR-027）。
//
// 置き換えたかどうかは、Load が付ける WarnInvalidEncoding で判断する。同じ事実を
// 別のフィールドにも持つと、片方だけが食い違いうる。
func (d *Document) Editable() bool {
	if d == nil || d.Kind == mdfile.KindImage {
		return false
	}
	for _, w := range d.Warnings {
		if w.Kind == WarnInvalidEncoding {
			return false
		}
	}
	return true
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
	// FR-016 の「Open anyway」と、確認して描画した文書を同じファイルとして
	// 読み直すとき（IMP-192 の currentConfirmed）に渡す。
	Confirmed bool

	// RefKey が空でなく、読んだ生バイト列の要約が ExpectDigest と一致すれば、
	// 新しい鍵を作らずにこの値を使う（IMP-120）。
	// 編集モードの書き込みの直後の読み直しだけが渡す（IMP-195 の 8, FR-143）。
	RefKey       string
	ExpectDigest [sha256.Size]byte // RefKey を使ってよい内容の要約（書き込んだ内容の SHA-256）

	// TextRefs が true なら、修飾の目印（text:<n>）も付けて描画する（IMP-120, renderer.Options）。
	// 編集モードの間の、同じ文書の読み直しだけが渡す（IMP-192）。
	// Markdown 以外では見ない（修飾の対象外。FR-146）。
	TextRefs bool
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
// 返しうるエラー: ErrNotFound / ErrPermission / ErrNotSupported /
// ErrTooLarge / ErrNeedsConfirm / 変換エラー。
// サイズ超過は *SizeError でラップして返す。
//
// **検査の順序は IMP-102 が固定している。** 拡張子 → 存在と権限 → サイズ →
// 読み込み → 正規化 → 変換の順で、途中で失敗したら以降は行わない。
// 画像ファイルは中身を読まず、サイズの判定の後で版から HTML を作る（loadImage）。
func Load(r *renderer.Renderer, path string, opts LoadOptions) (*Document, error) {
	kind := mdfile.KindOf(path)
	if kind == mdfile.KindNone {
		return nil, fmt.Errorf("%s: %w", path, ErrNotSupported)
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
	if notFile(info) {
		// 開ける拡張子の名前のディレクトリ（x.md / x.png など）や FIFO。種類を問わず開けない（UT-106 のケース 5〜7）。
		return nil, fmt.Errorf("%s: %w", abs, ErrNotSupported)
	}

	// 大きさの規則は画像にも当てはめる（FR-016, FR-027）。画像は os.Stat の大きさで判定する。
	size := info.Size()
	if limit, err := checkSize(size, opts.Confirmed); err != nil {
		return nil, &SizeError{Path: abs, Size: size, Limit: limit, Err: err}
	}

	if kind == mdfile.KindImage {
		return loadImage(r, abs, info)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, classifyError(abs, err)
	}

	// 要約と鍵は読んだ直後、正規化と変換の前に決める（IMP-102 の図）。
	//
	// **要約は正規化の前の生バイト列で取る**（FR-143, FR-144）。正規化の後で取ると、
	// 外部のエディタが改行コードや BOM だけを変えて保存したときに「変わっていない」と
	// 判断し、CRLF の文書へ LF の位置で書き込む（UT-116 ケース 4・5）。内容の写しは
	// 保持しない。
	digest := sha256.Sum256(raw)
	key, err := refKeyFor(opts, digest)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}

	text, replaced := Normalize(raw)

	// 相対パスの基準は表示中ファイルのディレクトリとする（AR-042）。
	// ツリールートを基準にしてはならない。
	// **自分の鍵を渡す。** 渡さないと目印が付かず、編集・並べ替え・リンクのコピーが
	// 働かない（IMP-110, UT-116 ケース 8・9）。
	// **修飾の目印は Markdown だけ**（FR-146 の対象外。IMP-102）。渡した値を Document にも写す。
	ropts := renderer.Options{
		BaseDir:  filepath.Dir(abs),
		RefKey:   key,
		TextRefs: opts.TextRefs && kind == mdfile.KindMarkdown,
	}
	res, err := render(r, kind, text, ropts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}

	doc := &Document{
		Path:          abs,
		Kind:          kind,
		Size:          size,
		HTML:          res.HTML,
		Headings:      res.Headings,
		LineCount:     CountLines(text),
		NeedsMermaid:  res.NeedsMermaid,
		NeedsKaTeX:    res.NeedsKaTeX,
		NeedsPlantUML: res.NeedsPlantUML,
		Digest:        digest,
		RefKey:        key,
		TextRefs:      ropts.TextRefs,
		NoDiagrams:    res.NoDiagrams,

		// 警告がなくても nil ではなく空スライスを返す。JSON で [] になり、
		// フロントエンドが null を場合分けせずに済む（Headings と同じ扱い）。
		Warnings: []Warning{},
	}

	if replaced {
		doc.Warnings = append(doc.Warnings, Warning{Kind: WarnInvalidEncoding})
	}

	return doc, nil
}

// notFile は、開ける拡張子の名前でも文書として読めないもの（ディレクトリ・名前付きパイプ・デバイス・
// ソケット）かを返す（IMP-102。4.87.0。利用者の判断）。FIFO を読むと書き手が現れるまで戻らず、デバイスは
// 中身を読んでしまう（Linux）。**`Mode().IsRegular()` では判定しない**——Go 1.23 から、Windows の再解析
// ポイント（シンボリックリンクと重複除去以外。OneDrive のクラウドのファイルなど）は `ModeIrregular` になり、
// v1.1.0 で開けた文書が開けなくなる。
func notFile(info fs.FileInfo) bool {
	return info.IsDir() || info.Mode()&(fs.ModeNamedPipe|fs.ModeDevice|fs.ModeCharDevice|fs.ModeSocket) != 0
}

// render は種類ごとの変換を呼ぶ（IMP-102 の「種類ごとの変換」の表）。画像は loadImage が扱う。
func render(r *renderer.Renderer, kind mdfile.Kind, text []byte, opts renderer.Options) (renderer.Result, error) {
	switch kind {
	case mdfile.KindPlantUML:
		return r.RenderPlantUMLFile(text, opts)
	case mdfile.KindMermaid:
		return r.RenderMermaidFile(text, opts)
	default:
		return r.Render(text, opts)
	}
}

// loadImage は画像ファイルの Document を組み立てる（IMP-102, IMP-128, FR-027）。
//
//   - **中身を読まない。** 版（localurl.Version。更新時刻と大きさ）を URL に付け、外部で書き換えられたら
//     URL が変わって WebView が取り直す（BUG-026）
//   - **Digest は版の文字列の SHA-256**（版が同じなら同じ内容とみなす）。零値のままにすると、監視の
//     読み直しの「同じ内容なら送らない」（IMP-192）で、外部の書き換えが画面に届かない（UT-117 のケース 7）
//   - 鍵は作らない（編集しない）。LoadOptions.RefKey も見ない（UT-117 のケース 10）
func loadImage(r *renderer.Renderer, abs string, info os.FileInfo) (*Document, error) {
	version := localurl.Version(info)
	res, err := r.RenderImageFile(abs, version)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", abs, err)
	}

	return &Document{
		Path:     abs,
		Kind:     mdfile.KindImage,
		Size:     info.Size(),
		HTML:     res.HTML,
		Headings: res.Headings,
		Digest:   sha256.Sum256([]byte(version)),
		Warnings: []Warning{},
	}, nil
}

// refKeyLen は鍵の元にする乱数のバイト数（IMP-102。16 進で 16 文字になる）。
const refKeyLen = 8

// refKeyFor はこの描画の目印の鍵を決める（IMP-102, IMP-120）。
//
// **鍵は Load のたびに作り直す。** 同じファイルを読み直しても値が変わり、鍵が変わる
// こと自体が「その描画の後に再描画が起きた」ことの印になる（FR-143）。書き手は変換より
// 前に文書を書くため鍵を知りようがなく、生 HTML で目印を偽装できない（NFR-030）。
//
// 例外は opts.RefKey が空でなく、**読んだ生バイト列の要約が opts.ExpectDigest と一致する**
// ときで、その値を使う。編集モードの書き込みの直後の読み直し（IMP-195 の 8）だけが渡す。
// **一致しなければ新しい鍵を作る**——書き込みと読み直しの間に外部の書き込みが入ると、
// 古い描画で作った指示が通り、別のチェックボックスを書き換える（UT-116 ケース 10）。
// ExpectDigest を渡さない（ゼロ値）場合も、生バイト列の要約とは一致しないため作り直す
// （一致を確かめずに鍵を使わない。ケース 11）。
func refKeyFor(opts LoadOptions, digest [sha256.Size]byte) (string, error) {
	if opts.RefKey != "" && digest == opts.ExpectDigest {
		return opts.RefKey, nil
	}

	var b [refKeyLen]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("cannot generate a reference key: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
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
