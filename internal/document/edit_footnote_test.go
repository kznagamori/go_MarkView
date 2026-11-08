package document

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// renderedRefs は、鍵 key で描いた source の出力から、種類 kind の目印を描画される順に返す。
func renderedRefs(t *testing.T, r *renderer.Renderer, source, kind string) []string {
	t.Helper()

	res, err := r.Render([]byte(source), renderer.Options{RefKey: testKey})
	if err != nil {
		t.Fatalf("前提: Render がエラーを返した: %v", err)
	}
	re := regexp.MustCompile(`data-ref="(` + testKey + `:` + kind + `:[0-9:]+)"`)
	var refs []string
	for _, m := range re.FindAllStringSubmatch(res.HTML, -1) {
		refs = append(refs, m[1])
	}
	return refs
}

// lineOf は raw の pos を含む行を返す（改行を含まない）。
func lineOf(raw []byte, pos int) string {
	s := string(raw)
	start := strings.LastIndexByte(s[:pos], '\n') + 1
	end := strings.IndexByte(s[pos:], '\n')
	if end < 0 {
		return s[start:]
	}
	return s[start : pos+end]
}

// footnoteTaskForms は UT-109 のケース 13 の 3 つの形（BUG-027 の 1.1。3 つは別々の文書。
// 脚注の中の字下げは半角空白 4 つ）。order は描画される順のタスクの文字（UT-217 のケース 27 と同じ）。
var footnoteTaskForms = []struct {
	name  string
	src   string
	order []string
}{
	{
		name:  "形1_定義が本文より前",
		src:   "本文[^1]\n\n[^1]: 脚注:\n\n    - [ ] 脚注のタスク\n\n- [ ] 本文のタスク\n",
		order: []string{"本文のタスク", "脚注のタスク"},
	},
	{
		name:  "形2_参照されていない定義",
		src:   "[^z]: 使っていない脚注:\n\n    - [ ] 見えないタスク\n\n- [ ] 本文のタスク A\n- [ ] 本文のタスク B\n",
		order: []string{"本文のタスク A", "本文のタスク B"},
	},
	{
		name:  "形3_定義と参照の順が違う",
		src:   "本文[^a]と[^b]\n\n[^b]: B:\n\n    - [ ] 脚注 b のタスク\n\n[^a]: A:\n\n    - [ ] 脚注 a のタスク\n",
		order: []string{"脚注 a のタスク", "脚注 b のタスク"},
	},
}

// TestPlanTask_Footnotes は、Render の目印で PlanTask を呼ぶと画面で押した項目が書き換わることを
// 検証する（UT-109 のケース 13。根拠: FR-141, MD-022 / IMP-106, IMP-120, IMP-121。BUG-027）。
//
// **Render の番号と PlanTask の位置を同じ入力で突き合わせる。** 脚注を末尾に参照の順で書いた文書
// （よくある書き方）では食い違わないため、3 つの形を欠かさない。
func TestPlanTask_Footnotes(t *testing.T) {
	r := renderer.New()

	for _, f := range footnoteTaskForms {
		t.Run(f.name, func(t *testing.T) {
			raw := []byte(f.src)
			refs := renderedRefs(t, r, f.src, "task")
			if len(refs) != len(f.order) {
				t.Fatalf("描画されたタスクが %d 個, want %d（参照されていない脚注は描かれない）", len(refs), len(f.order))
			}
			// 描画される順の i 番目のチェックボックスを押すと、order[i] の行の括弧が書き換わる。
			for i, s := range refs {
				ref, err := ParseRef(s)
				if err != nil {
					t.Fatalf("目印 %q を解けない: %v", s, err)
				}
				p, changed, err := PlanTask(r, raw, ref, true)
				if err != nil || !changed {
					t.Fatalf("PlanTask(%q) = changed %v, err %v", s, changed, err)
				}
				if line := lineOf(raw, p.Offset); !strings.HasSuffix(line, f.order[i]) {
					t.Errorf("画面の %q を押した（%s）のに書き換わる行が %q", f.order[i], s, line)
				}
			}
		})
	}

	// 形 2: 参照されていない脚注のタスクは描かれず、目印も無い。task:2 は ErrRefNotFound。
	t.Run("形2_task2は無い", func(t *testing.T) {
		_, _, err := PlanTask(r, []byte(footnoteTaskForms[1].src), Ref{Key: testKey, Kind: RefTask, Index: 2}, true)
		if !errors.Is(err, ErrRefNotFound) {
			t.Errorf("task:2 のエラー = %v, want ErrRefNotFound", err)
		}
	})
}

// footnoteTableDoc は UT-110 のケース 23 の文書（UT-109 のケース 13 の形 1 で、タスクの代わりに
// 1 列の表）。脚注の表のセルは「脚注のセル」、本文の表の本体の 1 行目は「本文のセル」。
const footnoteTableDoc = "本文[^1]\n\n[^1]: 脚注:\n\n" +
	"    | h |\n    | --- |\n    | 脚注のセル |\n\n" +
	"| h |\n| --- |\n| 本文のセル |\n"

// TestPlanCell_Footnotes は、Render が本文のセルに付けた目印で、本文の表のセルを読み書きすることを
// 検証する（UT-110 のケース 23。根拠: FR-142, MD-024 / IMP-106, IMP-120, IMP-121。BUG-027）。
func TestPlanCell_Footnotes(t *testing.T) {
	r := renderer.New()
	raw := []byte(footnoteTableDoc)

	// 前提: 本文の表が先に描かれ、そのセルの目印が cell:0:1:0 である。
	res, err := r.Render(raw, renderer.Options{RefKey: testKey})
	if err != nil {
		t.Fatalf("前提: Render がエラーを返した: %v", err)
	}
	m := regexp.MustCompile(`<td data-ref="(` + testKey + `:cell:[0-9:]+)">本文のセル</td>`).FindStringSubmatch(res.HTML)
	if m == nil {
		t.Fatalf("本文のセルの目印が見つからない\n出力: %s", res.HTML)
	}
	if want := testKey + ":cell:0:1:0"; m[1] != want {
		t.Errorf("本文のセルの目印 = %q, want %q（描画される順で本文の表が 0 番）", m[1], want)
	}

	ref, err := ParseRef(m[1])
	if err != nil {
		t.Fatalf("目印 %q を解けない: %v", m[1], err)
	}
	if got, err := CellSource(r, raw, ref); err != nil || got != "本文のセル" {
		t.Errorf("CellSource = %q, %v, want %q", got, err, "本文のセル")
	}
	p, changed, err := PlanCell(r, raw, ref, "新しいセル")
	if err != nil || !changed {
		t.Fatalf("PlanCell = changed %v, err %v", changed, err)
	}
	if string(p.Old) != "本文のセル" {
		t.Errorf("Old = %q, want %q（本文の表のセルを書き換える）", p.Old, "本文のセル")
	}
}
