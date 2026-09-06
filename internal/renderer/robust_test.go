package renderer

import (
	"strings"
	"testing"
	"time"
)

// renderWithin は制限時間つきで変換する（UT-019）。
//
// UT-213 は「panic しないこと」と「時間内に完了すること」の両方を求める。
// time.Sleep で待つのではなく、チャネルと select で待ち合わせる。
func renderWithin(t *testing.T, source string, limit time.Duration) (Result, error) {
	t.Helper()

	type outcome struct {
		res Result
		err error
	}
	done := make(chan outcome, 1)

	go func() {
		res, err := New().Render([]byte(source), "")
		done <- outcome{res, err}
	}()

	select {
	case o := <-done:
		return o.res, o.err
	case <-time.After(limit):
		t.Fatalf("変換が %s 以内に終わらなかった", limit)
		return Result{}, nil
	}
}

// renderLimit は UT-213 の時間予算を返す。
//
// 最も重いケース（1000 段のネスト＝約 1 MB の入力）で実測 1.3 秒。
// 変換時間は入力量におおむね比例し、二乗で増える箇所はない。予算はその
// 数倍を取り、CI の負荷で揺れても落ちないようにする（UT-019）。
func renderLimit() time.Duration {
	if raceEnabled {
		return 90 * time.Second
	}
	return 10 * time.Second
}

// TestRender_DoesNotCrash は異常な入力で落ちないことを検証する
// （UT-213。根拠: FR-111, NFR-050 / IMP-022）。
//
// **この観点に限り「panic しないこと」自体が仕様である**（UT-032 の例外）。
// Render はパニックを recover してエラーへ変えるため、ここではエラーも
// 返らないこと（＝そもそも落ちていないこと）まで見る。
func TestRender_DoesNotCrash(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		// UT-213 ケース 6: 空入力
		{"空入力", ""},
		{"空白のみ", "   \n\n   "},

		// UT-213 ケース 1: 深い入れ子
		{"1000 段のネストしたリスト", nestedList(1000)},
		{"1000 段の入れ子引用", strings.Repeat("> ", 1000) + "x"},

		// UT-213 ケース 2: 閉じていない記法
		{"閉じていない強調", strings.Repeat("*", 10000)},
		{"閉じていないリンク", strings.Repeat("[", 10000)},
		{"閉じていない画像", strings.Repeat("![", 5000)},
		{"閉じていないコードフェンス", "```go\nx"},
		{"閉じていない数式", "$" + strings.Repeat("a", 10000)},
		{"閉じていない HTML", strings.Repeat("<div>", 5000)},

		// UT-213 ケース 3: 不正な UTF-8（正規化前の生バイト）
		{"不正な UTF-8", "a\xffb\xfe\xfec"},
		{"不正なバイトのみ", "\xff\xfe\xfd"},

		// UT-213 ケース 4: 極端に長い 1 行
		{"1 MB の 1 行", strings.Repeat("a", 1<<20)},

		// UT-213 ケース 5: 巨大な表
		{"1000 列の表", wideTable(1000)},

		// UT-090 に従って追加
		{"見出しだけが 10000 個", strings.Repeat("# H\n", 10000)},
		{"NUL バイトを含む", "a\x00b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// パニックがここまで漏れれば、テストはその時点で失敗する。
			res, err := renderWithin(t, tt.in, renderLimit())
			if err != nil {
				t.Errorf("Render がエラーを返した: %v", err)
			}
			if res.Headings == nil {
				t.Error("Headings が nil。空スライスを返す規約（UT-203 ケース 5）に反する")
			}
		})
	}
}

// nestedList は depth 段の入れ子リストを組み立てる。
func nestedList(depth int) string {
	var b strings.Builder
	for i := range depth {
		b.WriteString(strings.Repeat("  ", i))
		b.WriteString("- x\n")
	}
	return b.String()
}

// wideTable は cols 列の表を組み立てる。
func wideTable(cols int) string {
	var b strings.Builder
	b.WriteString("|" + strings.Repeat(" a |", cols) + "\n")
	b.WriteString("|" + strings.Repeat(" --- |", cols) + "\n")
	b.WriteString("|" + strings.Repeat(" 1 |", cols) + "\n")
	return b.String()
}

// TestRecoverRender はパニックがエラーへ変換されることを検証する
// （UT-213 の前提。根拠: FR-111 / IMP-022）。
//
// Render を外から panic させる手段がないため、遮断そのものを直接見る。
func TestRecoverRender(t *testing.T) {
	res, err := func() (res Result, err error) {
		defer recoverRender(&res, &err)

		res = Result{HTML: "途中まで組み立てた結果"}
		panic("boom")
	}()

	if err == nil {
		t.Fatal("パニックがエラーに変換されていない")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("エラーにパニックの内容が含まれていない: %v", err)
	}
	if res.HTML != "" {
		t.Errorf("途中の結果が返っている: %q", res.HTML)
	}
}

// TestRecoverRender_NoPanic は、パニックがない場合に結果を壊さないことを
// 検証する（recover を無条件にエラー化していないかの確認）。
func TestRecoverRender_NoPanic(t *testing.T) {
	res, err := func() (res Result, err error) {
		defer recoverRender(&res, &err)

		res = Result{HTML: "ok"}
		return res, nil
	}()

	if err != nil {
		t.Errorf("エラーが返った: %v", err)
	}
	if res.HTML != "ok" {
		t.Errorf("結果が壊れている: %q", res.HTML)
	}
}

// TestRender_LazyLoadFlags は遅延ロードのフラグの組み合わせを検証する
// （UT-212。根拠: AR-021, NFR-013 / IMP-113, IMP-115）。
//
// **誤って true になると遅延ロードの効果が失われる。** ケース 1 と 5 が要。
func TestRender_LazyLoadFlags(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantMermaid  bool
		wantKaTeX    bool
		wantPlantUML bool
	}{
		// UT-212 ケース 1・6・7: 立ってはいけない場合
		{"プレーンな Markdown", "# H\n\npara と `code`", false, false, false},
		{"コードブロック内の mermaid という文字列", "```go\nmermaid := 1\n```", false, false, false},
		{"コードブロック内の plantuml という文字列", "```go\nplantuml := 1\n```", false, false, false},
		{"本文中の mermaid という語", "mermaid と katex の話", false, false, false},
		{"本文中の plantuml という語", "plantuml と puml の話", false, false, false},
		{"通貨表記", "$100 と $200", false, false, false},

		// UT-212 ケース 2〜5
		{"Mermaid のみ", "```mermaid\ngraph TD\n```", true, false, false},
		{"数式のみ", "$a+b$", false, true, false},
		{"ブロック数式のみ", "$$a+b$$", false, true, false},
		{"math コードブロックのみ", "```math\na+b\n```", false, true, false},
		{"PlantUML のみ", "```plantuml\n@startuml\n@enduml\n```", false, false, true},
		{"puml のみ", "```puml\n@startuml\n@enduml\n```", false, false, true},
		{"Mermaid と数式", "```mermaid\ngraph TD\n```\n\n$a+b$", true, true, false},
		{
			"3 つとも",
			"```mermaid\ngraph TD\n```\n\n$a+b$\n\n```plantuml\n@startuml\n@enduml\n```",
			true, true, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "")
			if err != nil {
				t.Fatalf("Render がエラーを返した: %v", err)
			}
			if res.NeedsMermaid != tt.wantMermaid {
				t.Errorf("NeedsMermaid = %v, want %v", res.NeedsMermaid, tt.wantMermaid)
			}
			if res.NeedsKaTeX != tt.wantKaTeX {
				t.Errorf("NeedsKaTeX = %v, want %v", res.NeedsKaTeX, tt.wantKaTeX)
			}
			if res.NeedsPlantUML != tt.wantPlantUML {
				t.Errorf("NeedsPlantUML = %v, want %v", res.NeedsPlantUML, tt.wantPlantUML)
			}
		})
	}
}
