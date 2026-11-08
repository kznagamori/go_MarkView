package document

import (
	"errors"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// red / blue は UT-120 の略記 <red> / <blue> の開きタグ（FR-147 の書く形）。
const (
	red  = `<span style="color: #d1242f">`
	blue = `<span style="color: #0969da">`
)

// textRef は text:<n> の目印を返す。
func textRef(n int) Ref { return Ref{Key: testKey, Kind: RefText, Index: n} }

// strike / color は修飾の操作を返す。
func strike() FormatStyle           { return FormatStyle{Strike: true} }
func color(name string) FormatStyle { return FormatStyle{Color: name} }

// targetOf は Ref から renderer の TextTarget を作る（UT-120 のケース 21 の解き直しに使う）。
func targetOf(ref Ref) renderer.TextTarget {
	if ref.Kind == RefCell {
		return renderer.TextTarget{Cell: true, Table: ref.Index, Row: ref.Row, Col: ref.Col}
	}
	return renderer.TextTarget{Index: ref.Index}
}

// TestPlanFormat は修飾の書き込みを検証する（UT-120。根拠: FR-143, FR-146, FR-147, FR-110 /
// IMP-106, IMP-123, IMP-124）。期待は Patch を適用した後のソースで書く（IMP-124 の例と同じ）。
//
// **ケース 2・10・11 を欠かさない。** 2 は「選択の両端にだけ差し込む」実装を、10 は「常に内側に
// 差し込む」実装を、11 は「区間ごとに判定する」実装を捕まえる。**ケース 24・25 は、操作の順で
// 入れ子の順が変わる実装を捕まえる**（色が外、取り消し線が内）。
func TestPlanFormat(t *testing.T) {
	r := renderer.New()

	tests := []struct {
		name       string
		src        string
		ref        Ref
		start, end int
		visible    string
		style      FormatStyle
		want       string
		widened    bool
	}{
		{"1 付ける", "今日は会議です", textRef(0), 3, 5, "今日は会議です", strike(), "今日は<del>会議</del>です", false},
		{"2 境界ごとに分けて差し込む", "**重要**な点", textRef(0), 1, 3, "重要な点", strike(), "**重<del>要</del>**<del>な</del>点", false},
		{"3 一部を外す", "<del>あいう</del>", textRef(0), 1, 2, "あいう", strike(), "<del>あ</del>い<del>う</del>", false},
		{"4 空のタグを作らない", "<del>あいう</del>", textRef(0), 0, 1, "あいう", strike(), "あ<del>いう</del>", false},
		{"5 手書きの取り消し線は全体を外す", "~~あいう~~", textRef(0), 1, 2, "あいう", strike(), "あいう", true},
		{"6 s", "<s>あいう</s>", textRef(0), 1, 2, "あいう", strike(), "あいう", true},
		{"6 strike", "<strike>あいう</strike>", textRef(0), 1, 2, "あいう", strike(), "あいう", true},
		{"6 チルダ1つ", "~あいう~", textRef(0), 1, 2, "あいう", strike(), "あいう", true},
		{"7 色を替える", red + "赤い文字</span>", textRef(0), 2, 4, "赤い文字", color("blue"), red + "赤い</span>" + blue + "文字</span>", false},
		{"8 色を外す", red + "赤い文字</span>", textRef(0), 0, 2, "赤い文字", color("default"), "赤い" + red + "文字</span>", false},
		{"9 インラインコードの一部は全体まで広げる", "`code` と文字", textRef(0), 2, 6, "code と文字", strike(), "<del>`code` と</del>文字", false},
		{"10 中身の全体と一致したら外側に差し込む", "日本語**重要**です", textRef(0), 3, 5, "日本語重要です", strike(), "日本語<del>**重要**</del>です", false},
		{"11 判定は選択の全体で行う", "あ<del>い</del>う", textRef(0), 0, 3, "あいう", strike(), "<del>あ</del><del>い</del><del>う</del>", false},
		{"12 すべて取り消し線なので外す", "<del>あ</del><del>い</del><del>う</del>", textRef(0), 0, 3, "あいう", strike(), "あいう", false},
		{"13 書く形を1つに固定する", "あいう", textRef(0), 0, 3, "あいう", color("red"), red + "あいう</span>", false},
		{"14 手で書いた揺れを決めた形で書き直す", `<span style="color:#D1242F">赤い</span>`, textRef(0), 0, 2, "赤い", color("blue"), blue + "赤い</span>", false},
		{"14 ほかの属性は残す", `<span title="t" style="color:#D1242F">赤い</span>`, textRef(0), 0, 2, "赤い", color("blue"), `<span title="t" style="color: #0969da">赤い</span>`, false},
		{"14 名前や値にstyleを含むほかの属性", `<span data-style="x" title="a style=b" style="color:#D1242F">赤い</span>`, textRef(0), 0, 2, "赤い", color("blue"),
			`<span data-style="x" title="a style=b" style="color: #0969da">赤い</span>`, false},
		{"15 分けられないときは入れ子で替える", red + "あ**いうえ**お</span>", textRef(0), 2, 3, "あいうえお", color("blue"), red + "あ**い" + blue + "う</span>え**お</span>", false},
		{"16 両端の空白を除く", "今日は 会議 です", textRef(0), 3, 7, "今日は 会議 です", strike(), "今日は <del>会議</del> です", false},
		{"17 絵文字は部品", ":sparkles:と", textRef(0), 0, 1, "✨と", strike(), "<del>:sparkles:</del>と", false},
		{"18 見出し", "# 見出し", textRef(0), 0, 3, "見出し", strike(), "# <del>見出し</del>", false},
		{"18 タスク", "- [ ] 買い物", textRef(0), 0, 3, "買い物", strike(), "- [ ] <del>買い物</del>", false},
		{"18 表のセル", "| a | b |\n| --- | --- |\n| 1 | 会議 |\n| x |\n", Ref{Key: testKey, Kind: RefCell, Index: 0, Row: 1, Col: 1}, 0, 2, "会議", strike(),
			"| a | b |\n| --- | --- |\n| 1 | <del>会議</del> |\n| x |\n", false},
		{"19 符号位置で数える", "😀あい", textRef(0), 2, 3, "😀あい", strike(), "😀あ<del>い</del>", false},
		{"22 手書きの取り消し線の全体をちょうど選ぶ", "~~あいう~~", textRef(0), 0, 3, "あいう", strike(), "あいう", false},
		{"23 戻りリンクの2文字は選択から外す", "本文[^1]\n\n[^1]: 注\n", textRef(1), 0, 3, "注 ↩", strike(), "本文[^1]\n\n[^1]: <del>注</del>\n", false},
		{"24 delの外側に色のspan", "<del>項目</del>", textRef(0), 0, 2, "項目", color("purple"), `<span style="color: #8250df"><del>項目</del></span>`, false},
		{"25 色のspanの内側にdel", red + "赤い</span>", textRef(0), 0, 2, "赤い", strike(), red + "<del>赤い</del></span>", false},
		{"26 角括弧の自動リンクは記号の外側", "<https://example.com> と文字", textRef(0), 0, 19, "https://example.com と文字", strike(), "<del><https://example.com></del> と文字", false},
		{"26 終わりの角括弧の自動リンク", "文字と <https://example.com>", textRef(0), 3, 22, "文字と https://example.com", strike(), "文字と <del><https://example.com></del>", false},
		{"27 中身の全体の色を外す", red + "赤い文字</span>", textRef(0), 0, 4, "赤い文字", color("default"), "赤い文字", false},
		// ケース 28: 改行で終わる区間の閉じは、次の行の前置きの後ろに置く（行の頭に置くと前置きを失う）。
		{"28 引用の中の複数行", "> a\n> **b**\n", textRef(0), 0, 3, "a\nb", strike(), "> <del>a\n> </del><del>**b**</del>\n", false},
		{"28 リストの中の複数行", "- a\n  **b**\n", textRef(0), 0, 3, "a\nb", strike(), "- <del>a\n  </del><del>**b**</del>\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(tt.src)
			op := FormatOp{Start: tt.start, End: tt.end, Visible: tt.visible, Style: tt.style}
			p, changed, widened, err := PlanFormat(r, raw, tt.ref, op)
			if err != nil || !changed {
				t.Fatalf("PlanFormat = changed %v, err %v, want 書き換える", changed, err)
			}
			if widened != tt.widened {
				t.Errorf("widened = %v, want %v", widened, tt.widened)
			}
			after, err := p.Apply(raw)
			if err != nil {
				t.Fatalf("Apply がエラーを返した: %v", err)
			}
			if string(after) != tt.want {
				t.Errorf("書き込み後 = %q, want %q", after, tt.want)
			}

			// ケース 21: 書き込み後のソースを解き直しても、見える文字が 1 文字も違わない（IMP-124 の 9）。
			text, _ := Normalize(after)
			tb, err := r.Visible(text, targetOf(tt.ref))
			if err != nil || tb == nil || string(tb.Visible) != tt.visible {
				t.Errorf("21: 書き込み後の見える文字が変わった: %v, %v, want %q", tb, err, tt.visible)
			}
		})
	}

	// ケース 1: Patch は最初の変更から最後の変更までの 1 つ（IMP-124 の 8）。
	t.Run("1 Patchの範囲", func(t *testing.T) {
		p, _, _, err := PlanFormat(r, []byte("今日は会議です"), textRef(0), FormatOp{Start: 3, End: 5, Visible: "今日は会議です", Style: strike()})
		if err != nil {
			t.Fatalf("PlanFormat がエラーを返した: %v", err)
		}
		if p.Offset != 9 || string(p.Old) != "会議" {
			t.Errorf("Offset / Old = %d / %q, want 9 / %q", p.Offset, p.Old, "会議")
		}
	})

	// ケース 20: BOM と CR の分の対応（IMP-106）。
	t.Run("20 BOMとCRLF", func(t *testing.T) {
		raw := []byte(bom + "a\r\n会議")
		p, changed, _, err := PlanFormat(r, raw, textRef(0), FormatOp{Start: 2, End: 4, Visible: "a\n会議", Style: strike()})
		if err != nil || !changed {
			t.Fatalf("PlanFormat = changed %v, err %v", changed, err)
		}
		if p.Offset != 6 || string(p.Old) != "会議" {
			t.Errorf("Offset / Old = %d / %q, want 6 / %q", p.Offset, p.Old, "会議")
		}
	})

	// ケース 23 の 2 つ目: ↩ だけを選ぶと、外した後に何も残らない。
	t.Run("23 戻りリンクだけ", func(t *testing.T) {
		_, changed, _, err := PlanFormat(r, []byte("本文[^1]\n\n[^1]: 注\n"), textRef(1), FormatOp{Start: 2, End: 3, Visible: "注 ↩", Style: strike()})
		if err != nil || changed {
			t.Errorf("PlanFormat = changed %v, err %v, want 偽, nil", changed, err)
		}
	})
}

// TestPlanFormat_Refuses は修飾を書き込まない場合を検証する（UT-121。根拠: FR-143, FR-146, FR-147, NFR-030 / IMP-124）。
//
// どのケースも Patch を返さない（changed が偽のケースはエラーを返さない）。**ケース 11 と 13 は、
// 見える文字の照合（UT-120 のケース 21）では捕まらない**——インラインの要素の並びの確かめが捕まえる。
// **ケース 1〜4 は、Go 側がフロントエンドの値を信じないことを見る**（IMP-300, NFR-030）。
func TestPlanFormat_Refuses(t *testing.T) {
	r := renderer.New()
	const plain = "今日は会議です"
	op := func(start, end int, visible string, style FormatStyle) FormatOp {
		return FormatOp{Start: start, End: end, Visible: visible, Style: style}
	}

	errTests := []struct {
		name string
		src  string
		ref  Ref
		op   FormatOp
		want error
	}{
		{"1 見える文字が1文字違う", plain, textRef(0), op(3, 5, "今日は会議でした", strike()), ErrRefNotFound},
		{"2 StartとEndが等しい", plain, textRef(0), op(3, 3, plain, strike()), ErrRefNotFound},
		{"2 StartがEndより大きい", plain, textRef(0), op(5, 3, plain, strike()), ErrRefNotFound},
		{"2 Endが見える文字の数を超える", plain, textRef(0), op(3, 8, plain, strike()), ErrRefNotFound},
		{"2 Startが負", plain, textRef(0), op(-1, 3, plain, strike()), ErrRefNotFound},
		{"3 StrikeとColorの両方", plain, textRef(0), op(3, 5, plain, FormatStyle{Strike: true, Color: "red"}), ErrRefNotFound},
		{"3 どちらも無い", plain, textRef(0), op(3, 5, plain, FormatStyle{}), ErrRefNotFound},
		{"4 一覧に無い名前", plain, textRef(0), op(3, 5, plain, color("pink")), ErrRefNotFound},
		{"4 大文字の名前", plain, textRef(0), op(3, 5, plain, color("Red")), ErrRefNotFound},
		{"4 値そのもの", plain, textRef(0), op(3, 5, plain, color("#d1242f")), ErrRefNotFound},
		{"8 範囲を超えるtext", plain, textRef(5), op(3, 5, plain, strike()), ErrRefNotFound},
		{"8 補われたセル", "| a | b |\n| --- | --- |\n| 1 | 2 |\n| x |\n", Ref{Key: testKey, Kind: RefCell, Index: 0, Row: 2, Col: 1}, op(0, 1, "", strike()), ErrRefNotFound},
		{"9 外すと入れ子が壊れる", "<del>あ**いう**え</del>", textRef(0), op(2, 3, "あいうえ", strike()), ErrCannotFormat},
		{"10 色を外すときは入れ子にできない", red + "あ**いうえ**お</span>", textRef(0), op(2, 3, "あいうえお", color("default")), ErrCannotFormat},
		{"11 内側に差し込むと太字が効かない", "日本語**重要**です", textRef(0), op(3, 4, "日本語重要です", strike()), ErrCannotFormat},
		{"12 不正なUTF-8の1バイト", "今日は\xFF会議", textRef(0), op(0, 1, "今", strike()), ErrNotEditable},
		{"12 不正なUTF-8の3バイト", "今日は\xFF\xFE\xFD会議", textRef(0), op(0, 1, "今", strike()), ErrNotEditable},
		{"13 入れ子で替えると太字が効かない", red + "あ**いう**え</span>", textRef(0), op(2, 3, "あいうえ", color("blue")), ErrCannotFormat},
		// ケース 14: 太字の記号の組み合わせが変わる（2 つの太字が 1 つの外側の太字になる）。見える文字は同じで、
		// インラインの要素の並びの確かめだけが捕まえる（11・13 は太字の記号が文字として現れ、見える文字の照合でも捕まる）。
		{"14 太字の記号の組み合わせが変わる", "**重要**と**重要**", textRef(0), op(1, 2, "重要と重要", strike()), ErrCannotFormat},
	}
	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			p, changed, _, err := PlanFormat(r, []byte(tt.src), tt.ref, tt.op)
			if !errors.Is(err, tt.want) {
				t.Errorf("エラー = %v, want %v", err, tt.want)
			}
			if changed || p.Old != nil || p.New != nil {
				t.Errorf("Patch を返した: changed %v, %+v", changed, p)
			}
		})
	}

	// changed が偽でエラーを返さないケース。
	noops := []struct {
		name string
		src  string
		op   FormatOp
	}{
		{"5 空白だけを選ぶ", "今日は 会議 です", op(3, 4, "今日は 会議 です", strike())},
		{"6 赤い文字に赤", red + "赤</span>", op(0, 1, "赤", color("red"))},
		{"7 色の無い文字にdefault", plain, op(3, 5, plain, color("default"))},
	}
	for _, tt := range noops {
		t.Run(tt.name, func(t *testing.T) {
			p, changed, _, err := PlanFormat(r, []byte(tt.src), textRef(0), tt.op)
			if err != nil || changed {
				t.Errorf("PlanFormat = changed %v, err %v, want 偽, nil", changed, err)
			}
			if p.Old != nil || p.New != nil {
				t.Errorf("Patch を返した: %+v", p)
			}
		})
	}
}
