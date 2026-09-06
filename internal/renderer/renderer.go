// Package renderer は Markdown を HTML へ変換する（IMP-110 系）。
//
// 変換とシンタックスハイライトは必ずこの Go 側で行う。フロントエンドに
// Markdown パーサやハイライタを置かない（AR-031）。
//
// internal のうち、依存を持たない葉パッケージ（localurl / applog）だけを
// 参照する（IMP-012）。
package renderer

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/applog"
	"github.com/kznagamori/go_MarkView/internal/localurl"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Heading は本文中の見出し 1 つを表す（IMP-110, FR-040）。
//
// JSON のタグはフロントエンドへ渡す形（IMP-302）に合わせる。
type Heading struct {
	Level int    `json:"level"` // 1..6
	Text  string `json:"text"`  // インライン記法を除去したプレーンテキスト
	ID    string `json:"id"`    // 見出しアンカー（MD-021）
}

// Result は 1 回の変換結果を表す（IMP-110）。
type Result struct {
	HTML          string
	Headings      []Heading
	NeedsMermaid  bool
	NeedsKaTeX    bool
	NeedsPlantUML bool // AR-021, MD-085。IMP-119 が立てる
}

// Renderer は goldmark のパイプラインを保持する（IMP-110）。
//
// 状態を持たないため、複数のゴルーチンから同時に Render を呼んでよい
// （IMP-024）。goldmark の Convert はゴルーチンセーフである。
type Renderer struct {
	md     goldmark.Markdown
	policy *bluemonday.Policy
}

// New は変換器を作る（IMP-111）。
func New() *Renderer {
	return &Renderer{
		policy: Policy(),
		md: goldmark.New(
			goldmark.WithExtensions(
				// GFM の内訳を個別に登録する。表の桁揃えを style 属性ではなく
				// align 属性で出させるためで、これにより出力から style を一掃でき、
				// サニタイズ（IMP-116）で style を許可せずに済む（MD-024, MD-072）。
				extension.NewTable(
					extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute),
				),
				extension.Strikethrough, // 打消し線（MD-024）
				extension.TaskList,      // タスクリスト（MD-022）
				extension.Linkify,       // 裸の URL の自動リンク（MD-070）
				// 脚注（MD-050）。戻りリンクの記号は goldmark 既定の
				// U+21A9 + U+FE0E（異体字セレクタ付き）ではなく、GitHub と
				// 同じ U+21A9 単独にする。セレクタが付くと白黒の記号として
				// 描かれ、GitHub の色付きの矢印と見た目が変わる（MD-002）。
				extension.NewFootnote(
					extension.WithFootnoteBacklinkHTML("&#x21a9;"),
				),
				emoji.Emoji, // 絵文字ショートコード（MD-051）
				meta.Meta,   // YAML Front Matter（MD-073）

				alertExtension{}, // GitHub Alerts（IMP-112, MD-040）

				mathExtension{}, // 数式の保護（IMP-113, MD-060）

				mermaidExtension{}, // Mermaid ブロックの取り出し（IMP-115, MD-080）

				plantUMLExtension{}, // PlantUML ブロックの取り出し（IMP-119, MD-083）

				// ハイライトは Mermaid・PlantUML・数式の後に置く。いずれも先に
				// 専用ノードへ差し替わり、chroma には渡らない（IMP-114, IMP-115, IMP-119）。
				newHighlighting(),
			),

			goldmark.WithParserOptions(
				// 見出し ID に parser.WithAutoHeadingID() は使わない。GitHub
				// 互換のスラッグ規則（MD-021）と生成結果が異なるためである。
				// 独自の AST 変換で付与する（IMP-111, IMP-117）。
				parser.WithASTTransformers(
					util.Prioritized(headingTransformer{}, 100),
					util.Prioritized(imageTransformer{}, 95), // 画像 URL の書き換え（IMP-118）
				),
			),

			goldmark.WithRendererOptions(
				// 生 HTML を通す。goldmark 段階で落とすと <details> のような
				// 許可要素（MD-072）まで失われるため。安全性の担保は後段の
				// サニタイズ（IMP-116）へ一元化する。この 2 つは必ず対で扱う。
				html.WithUnsafe(),

				// インデント形式のコードブロックもラッパで包む（IMP-115）。
				// 既定の描画器（優先度 1000）より小さい値で上書きする。
				gmrenderer.WithNodeRenderers(
					util.Prioritized(codeBlockRenderer{}, 500),
				),
			),
		),
	}
}

// Render は Markdown を変換する（IMP-110）。
//
// baseDir は相対パス解決の基準ディレクトリ（表示中ファイルのディレクトリ。
// AR-042）。source は正規化済みの UTF-8 テキストであることを前提とする
// （IMP-103）。
func (r *Renderer) Render(source []byte, baseDir string) (result Result, err error) {
	// 変換中のパニックをエラーへ変える（IMP-022, FR-111）。
	defer recoverRender(&result, &err)

	source = normalizeFrontMatter(source)

	// Context は変換ごとに作る。見出し一覧の受け渡しに使い、同時に走る
	// 変換どうしが干渉しないようにしている（IMP-024, IMP-117）。
	pc := parser.NewContext()
	// 画像 URL の書き換えに使う（IMP-118）。AST 変換器は 1 度しか登録され
	// ないため、変換ごとに変わる値は Context 経由で渡す。
	pc.Set(baseDirKey, baseDir)

	var buf bytes.Buffer
	if err := r.md.Convert(source, &buf, parser.WithContext(pc)); err != nil {
		return Result{}, fmt.Errorf("cannot convert markdown: %w", err)
	}

	headings, _ := pc.Get(headingsKey).([]Heading)
	if headings == nil {
		// 見出しのない文書でも nil ではなく空スライスを返す（UT-203 ケース 5）。
		// JSON では [] になり、フロントエンドが null を場合分けせずに済む。
		headings = []Heading{}
	}

	needsKaTeX, _ := pc.Get(needsKaTeXKey).(bool)
	needsMermaid, _ := pc.Get(needsMermaidKey).(bool)
	needsPlantUML, _ := pc.Get(needsPlantUMLKey).(bool)

	return Result{
		// サニタイズは変換パイプラインの最後段に固定で置く（IMP-116, AR-031）。
		HTML:          string(r.policy.SanitizeBytes(buf.Bytes())),
		Headings:      headings,
		NeedsKaTeX:    needsKaTeX,
		NeedsMermaid:  needsMermaid,
		NeedsPlantUML: needsPlantUML,
	}, nil
}

// Front Matter の区切り行（MD-073, IMP-111）。
const (
	yamlFence = "---"
	tomlFence = "+++"
)

// normalizeFrontMatter は Front Matter の扱いを MD-073 の規定に揃える。
//
// 規則を次に固定する。**開きの区切りが 1 行目にあり、かつ対応する閉じの
// 区切りがある場合にのみ Front Matter とみなす。閉じがない場合は本文として
// 描画する**（UT-211 ケース 5）。閉じのないものまで Front Matter とみなすと、
// 水平線で始まるだけの文書や書きかけの文書が丸ごと消えて何も表示されなく
// なり、GitHub の表示（MD-002）とも食い違うためである。
//
//   - TOML（+++）は goldmark-meta が扱わないため、閉じが揃うときにここで
//     取り除く。
//   - YAML（---）は閉じが揃うときは meta.Meta（IMP-111）に任せる。揃わない
//     ときだけ先頭に空行を 1 つ加え、Front Matter と認識させない。meta.Meta
//     が見るのは 1 行目だけであり、Markdown では先頭の空行は描画に影響しない。
//     この 1 行がないと、閉じのない YAML で文書全体が失われる。
func normalizeFrontMatter(source []byte) []byte {
	if body, ok := cutFrontMatter(source, tomlFence); ok {
		return body
	}

	if firstLineIs(source, yamlFence) {
		if _, ok := cutFrontMatter(source, yamlFence); !ok {
			return append([]byte{'\n'}, source...)
		}
	}

	return source
}

// cutFrontMatter は開きと閉じが揃った Front Matter を取り除いた本文を返す。
// 1 行目が区切りでない場合、または閉じが見つからない場合は ok が false になる。
func cutFrontMatter(source []byte, fence string) (body []byte, ok bool) {
	first, rest, hasNewline := bytes.Cut(source, []byte("\n"))
	if string(first) != fence || !hasNewline {
		return nil, false
	}

	// 閉じの区切りは、行全体が区切りと一致する行に限る。
	for offset := 0; offset < len(rest); {
		line, next := rest[offset:], len(rest)
		if end := bytes.IndexByte(rest[offset:], '\n'); end >= 0 {
			line, next = rest[offset:offset+end], offset+end+1
		}
		if string(line) == fence {
			return rest[next:], true
		}
		offset = next
	}

	return nil, false
}

// firstLineIs は 1 行目が fence と完全に一致するかを返す。
func firstLineIs(source []byte, fence string) bool {
	first, _, _ := bytes.Cut(source, []byte("\n"))
	return string(first) == fence
}

// recoverRender は変換中のパニックをエラーへ変換する（IMP-022, FR-111）。
//
// 「どの異常系でも異常終了しない」を実装レベルで保証する 2 か所のうちの 1 つ
// （もう 1 つは app.go の各バインドメソッド）。goldmark の拡張や chroma が
// 想定外の入力で落ちても、その文書が開けないだけで済ませる。
//
// スタックトレースは開発モードでのみ標準エラーへ出す（IMP-023, NFR-041）。
// 環境変数の判定は applog が持つ。ここで os.Getenv しない。
func recoverRender(result *Result, err *error) {
	v := recover()
	if v == nil {
		return
	}

	applog.Recovered("renderer.Render", v)

	// 途中まで組み立てた結果は返さない。呼び出し側は状態画面を出す（UI-052）。
	*result = Result{}
	*err = fmt.Errorf("markdown rendering failed: %v", v)
}

// baseDirKey は変換 1 回分の基準ディレクトリを parser.Context へ置く鍵（IMP-118）。
var baseDirKey = parser.NewContextKey()

// remoteSchemes は書き換えずにそのまま渡す URL の接頭辞（IMP-118, MD-071）。
var remoteSchemes = []string{"http://", "https://", "data:"}

// imageTransformer は画像の src を配信用 URL へ書き換える（IMP-118）。
//
// リンク（a href）は書き換えない。クリック時にフロントエンドが捕捉して Go 側へ
// 渡すため、元の値のまま保持する必要がある（AR-060, IMP-223）。
type imageTransformer struct{}

func (imageTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	baseDir, _ := pc.Get(baseDirKey).(string)

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		img, ok := n.(*ast.Image)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}

		img.Destination = []byte(rewriteImageURL(string(img.Destination), baseDir))
		return ast.WalkContinue, nil
	})
}

// rewriteImageURL は画像の src を WebView から取得できる形へ変換する（IMP-118）。
//
//	http:// https://   → そのまま（MD-071）
//	data:              → そのまま
//	絶対パス・相対パス → /__local/<エスケープ済み絶対パス>（AR-040）
//
// 相対パスは baseDir（表示中ファイルのディレクトリ）を基準に解決する。
// ツリールートを基準にしてはならない（AR-042）。
//
// 上記以外のスキーム（file: や javascript: 等）はローカルパスとして扱う。
// 結果は存在しないパスとなり配信されないため、素通しするより安全である。
func rewriteImageURL(src, baseDir string) string {
	if src == "" {
		return src
	}

	for _, scheme := range remoteSchemes {
		if len(src) >= len(scheme) && strings.EqualFold(src[:len(scheme)], scheme) {
			return src
		}
	}

	return localurl.Encode(ResolveRef(src, baseDir))
}

// ResolveRef は Markdown 内の参照をローカルの絶対パスへ解決する（AR-042）。
//
// **画像（IMP-118）とリンク遷移（IMP-312）が同じ規則を使う。** 別々に書くと、
// [x](./a.png) が開くファイルと ![x](./a.png) が表示するファイルが食い違いうる。
//
// 基準は baseDir、すなわち表示中ファイルのディレクトリである。ツリールートを
// 基準にしてはならない（AR-042）。スキームの判定は呼び出し側が済ませておく。
func ResolveRef(ref, baseDir string) string {
	// Markdown の宛先は URL であり、空白などは %20 で書かれうる。
	// ファイルパスへ戻してから解決する。復号できない場合は元の文字列を使う。
	path := ref
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}

	// 先頭が / のパスは OS を問わず絶対として扱う。Markdown の URL は POSIX
	// 形式で書かれるが、Windows の filepath.IsAbs はドライブレターを要求し、
	// /abs/a.png を相対と判定してしまう。
	if !strings.HasPrefix(path, "/") && !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	return filepath.Clean(path)
}
