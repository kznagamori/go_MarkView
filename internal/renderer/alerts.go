package renderer

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	gmrenderer "github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// alertKind は GitHub Alerts の種別（IMP-112, MD-040）。alertKinds の添字。
type alertKind int

const (
	alertNote alertKind = iota
	alertTip
	alertImportant
	alertWarning
	alertCaution
)

// alertKinds は種別ごとの記法・CSS クラス・表示ラベル。
//
// ラベルは GitHub と同じ英語表記に固定する（MD-040）。利用者に見える文言を
// 英語のみとする規則（UI-024）にも合う。配色は CSS 側の責務（DSP-260）。
var alertKinds = [...]struct {
	token string // [!TOKEN] の TOKEN。比較は大文字に揃えて行う
	class string // markdown-alert-<class>
	label string
}{
	alertNote:      {"NOTE", "note", "Note"},
	alertTip:       {"TIP", "tip", "Tip"},
	alertImportant: {"IMPORTANT", "important", "Important"},
	alertWarning:   {"WARNING", "warning", "Warning"},
	alertCaution:   {"CAUTION", "caution", "Caution"},
}

// kindAlert は alert ノードの種別。
var kindAlert = ast.NewNodeKind("Alert")

// alert は GitHub Alerts のブロック（IMP-112）。
//
// Blockquote を差し替えるために独自のノードにしている。goldmark の
// Blockquote は必ず <blockquote> として描画されるため、属性を付けるだけでは
// IMP-112 が定める <div> 構造にできない。
type alert struct {
	ast.BaseBlock

	kind alertKind
}

func (n *alert) Kind() ast.NodeKind { return kindAlert }

func (n *alert) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Kind": alertKinds[n.kind].token}, nil)
}

// alertExtension は Alerts の変換器と描画器を登録する（IMP-111, IMP-112）。
type alertExtension struct{}

func (alertExtension) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithASTTransformers(
		util.Prioritized(alertTransformer{}, 90),
	))
	md.Renderer().AddOptions(gmrenderer.WithNodeRenderers(
		util.Prioritized(alertRenderer{}, 500),
	))
}

// alertTransformer は Blockquote を alert ノードへ差し替える（IMP-112）。
type alertTransformer struct{}

// Transform は parser.ASTTransformer を満たす。
func (alertTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()

	// 走査しながら木を書き換えないよう、対象を集めてから差し替える。
	var quotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if q, ok := n.(*ast.Blockquote); ok && entering {
			quotes = append(quotes, q)
		}
		return ast.WalkContinue, nil
	})

	for _, q := range quotes {
		convertAlert(q, source)
	}
}

// convertAlert は引用が Alerts の記法であれば差し替える。
// 記法でない場合は何もせず、通常の引用として残す（MD-040）。
func convertAlert(q *ast.Blockquote, source []byte) {
	p, ok := q.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return
	}

	firstLine := p.Lines().At(0)
	kind, ok := parseAlertMarker(firstLine.Value(source))
	if !ok {
		return
	}

	stripMarker(p, firstLine)

	// マーカーだけの引用では段落が空になる。空の <p> を残さない。
	if p.FirstChild() == nil {
		q.RemoveChild(q, p)
	}

	a := &alert{kind: kind}
	for c := q.FirstChild(); c != nil; {
		next := c.NextSibling()
		q.RemoveChild(q, c)
		a.AppendChild(a, c)
		c = next
	}
	q.Parent().ReplaceChild(q.Parent(), q, a)
}

// parseAlertMarker は先頭行が [!種別] 形式かを調べる（IMP-112 の判定規則 2, 3）。
//
// 大文字小文字は区別しない（MD-040）。5 種のいずれにも一致しない場合は
// ok が false になり、呼び出し側は通常の引用として残す。
func parseAlertMarker(line []byte) (alertKind, bool) {
	s := string(util.TrimRightSpace(util.TrimLeftSpace(line)))

	if len(s) < len("[!]")+1 || s[:2] != "[!" || s[len(s)-1] != ']' {
		return 0, false
	}

	token := strings.ToUpper(s[2 : len(s)-1])
	for i, k := range alertKinds {
		if k.token == token {
			return alertKind(i), true
		}
	}
	return 0, false
}

// stripMarker は先頭行のマーカーに当たるインラインノードを取り除く。
//
// AST 変換器はインライン解析の後に走るため、マーカーは既に Text ノードへ
// 分かれている（`[` `!NOTE` `]` の 3 つ）。先頭行のセグメント内に収まる
// Text ノードを先頭から取り除けば、本文だけが残る。
func stripMarker(p *ast.Paragraph, firstLine text.Segment) {
	for c := p.FirstChild(); c != nil; {
		t, ok := c.(*ast.Text)
		if !ok || t.Segment.Stop > firstLine.Stop {
			break
		}
		next := c.NextSibling()
		p.RemoveChild(p, c)
		c = next
	}

	// 行の一覧からもマーカー行を落とし、ノードの内容を食い違わせない。
	p.Lines().SetSliced(1, p.Lines().Len())
}

// alertRenderer は alert ノードを描画する（IMP-112）。
type alertRenderer struct{}

func (alertRenderer) RegisterFuncs(reg gmrenderer.NodeRendererFuncRegisterer) {
	reg.Register(kindAlert, renderAlert)
}

// renderAlert は IMP-112 が固定した HTML 構造を出力する。
//
// **アイコンは出力しない。** サニタイズ（MD-072）は svg 要素を除去するため、
// ここで出したインライン SVG は必ず落ちる。アイコンはフロントエンドが後処理で
// 付与する（IMP-225, DSP-260）。サニタイズの許可リストに svg を足して
// 解決してはならない。
func renderAlert(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</div>\n")
		return ast.WalkContinue, nil
	}

	k := alertKinds[n.(*alert).kind]

	// class と label は上の表の定数であり、文書由来の文字列を含まない。
	_, _ = fmt.Fprintf(w, "<div class=\"markdown-alert markdown-alert-%s\">\n", k.class)
	_, _ = fmt.Fprintf(w, "<p class=\"markdown-alert-title\">%s</p>\n", k.label)

	return ast.WalkContinue, nil
}
