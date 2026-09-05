// 描画スモークテストの**判定**に対するテスト（UT-811, UT-812）。
//
// **判定にテストが要る理由**（UT-002）。判定が甘くなっても、スモークテストの
// 出力は「OK」のままであり、何も起きない。**通ったことが証拠にならない検査を
// CI に置かない。**
//
// ここで検査するのは `checkBrokenImages` と `checkPlantUMLLimits` の 2 つで
// ある。どちらも report を受け取る純関数であり、ブラウザを起動しない
// （UT-038）。**過検出のケースを必ず含める**——衝突していないのに落ちる誤りは
// 「検査が厳しすぎる」で片づけられ、やがて外される。
package main

import (
	"strings"
	"testing"
)

// okImg は「読める画像」1 枚ぶんの結果を作る。
func okImg() imageInfo {
	return imageInfo{Alt: okImageAlt, Complete: true, NaturalWidth: 1600}
}

// brokenSpan は置き換え後の要素 1 つぶんの結果を作る。
func brokenSpan(text string) brokenInfo {
	return brokenInfo{TagName: "SPAN", ClassName: "img-broken", Text: text}
}

// fixedImages は BUG-008 を修正したあとに期待される状態を作る。
//
// 読める画像が 1 枚残り、失敗した 2 枚は span へ置き換わっている。
// 片方は代替テキストを持ち、もう片方は空である。
func fixedImages() imageReport {
	return imageReport{
		Imported: true,
		Imgs:     []imageInfo{okImg()},
		Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan("")},
	}
}

// TestCheckBrokenImages は FR-022 / IMP-226 / DSP-123 の検査を検証する（UT-811）。
func TestCheckBrokenImages(t *testing.T) {
	tests := []struct {
		name   string
		images imageReport
		want   []string // 失敗メッセージに含まれるべき断片。空なら合格を期待
	}{
		{
			name:   "修正後の状態は合格する",
			images: fixedImages(),
		},
		{
			name: "代替テキストが空の画像だけでも合格する（過検出の検査）",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{okImg()},
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan("")},
			},
		},
		{
			name: "4.32.0 より前の実装は落ちる（img.is-broken が残る）",
			images: imageReport{
				Imported: true,
				Legacy:   2,
				Imgs: []imageInfo{
					okImg(),
					{Alt: brokenImageAlt, ClassName: "is-broken", Complete: true},
					{Alt: "", ClassName: "is-broken", Complete: true},
				},
			},
			want: []string{"img.is-broken が 2 個残っている", "代替テキスト", "img が残っている"},
		},
		{
			name: "枠はできたが代替テキストを描いていないと落ちる",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{okImg()},
				Broken:   []brokenInfo{brokenSpan(""), brokenSpan("")},
			},
			want: []string{"が本文として読めない"},
		},
		{
			name: "正常な画像まで置き換えると落ちる（過検出）",
			images: imageReport{
				Imported: true,
				Imgs:     nil,
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan(""), brokenSpan(okImageAlt)},
			},
			want: []string{".img-broken が 3 個", "読める画像が 0 枚"},
		},
		{
			name: "読めるはずの画像が読めていないと落ちる",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{{Alt: okImageAlt, Complete: true, NaturalWidth: 0}},
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan("")},
			},
			want: []string{"読めるはずの画像が読めていない"},
		},
		{
			name: "span 以外の要素にすると落ちる",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{okImg()},
				Broken: []brokenInfo{
					{TagName: "DIV", ClassName: "img-broken", Text: brokenImageAlt},
					brokenSpan(""),
				},
			},
			want: []string{"DIV 要素"},
		},
		{
			name:   "viewer.js を読めないときは、それだけを報告する",
			images: imageReport{Imported: false, Error: "SyntaxError"},
			want:   []string{"markBrokenImages を読めない"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkBrokenImages(smokeImageCount, report{Images: tt.images})

			if len(tt.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("合格を期待したが %d 件の失敗:\n%s", len(got), strings.Join(got, "\n"))
				}

				return
			}

			if len(got) == 0 {
				t.Fatalf("失敗を期待したが合格した（期待した断片: %v）", tt.want)
			}

			joined := strings.Join(got, "\n")
			for _, fragment := range tt.want {
				if !strings.Contains(joined, fragment) {
					t.Errorf("失敗メッセージに %q が無い:\n%s", fragment, joined)
				}
			}
		})
	}
}

// viewer.js を読めない場合は、他の判定へ進まないこと（UT-811）。
//
// **進むと、画像の情報が空であることを理由に大量の失敗が並び、
// 本当の原因（モジュールが読めない）が埋もれる。**
func TestCheckBrokenImages_ImportFailureReportsOnce(t *testing.T) {
	got := checkBrokenImages(smokeImageCount, report{Images: imageReport{Imported: false, Error: "boom"}})

	if len(got) != 1 {
		t.Fatalf("失敗は 1 件を期待したが %d 件:\n%s", len(got), strings.Join(got, "\n"))
	}
}

// drawn は plantuml-limits.md の描画対象ブロック 1 件ぶんを作る。
func drawn(head string, svg, width, height int, reason string) diagramBlock {
	return diagramBlock{Head: head, SVG: svg, Width: width, Height: height, Error: reason}
}

// limitsOK は BUG-010 の修正後に期待される状態を作る（実測に基づく）。
//
//	normal       図が出る
//	syntaxerror  エラー図が出る（FR-024。**失敗ではない**）
//	toolarge     図が出ず、理由が出る（4096 px の制限）
//	salt         実測では図が出た
//	ditaa        実測では理由が出た
func limitsOK() report {
	return report{
		PlantUML: []diagramBlock{
			drawn("@startuml normal", 1, 112, 164, ""),
			drawn("@startuml syntaxerror", 1, 430, 160, ""),
			drawn("@startuml toolarge", 0, 0, 0, "PlantUML could not render this diagram."),
			drawn("@startsalt", 1, 353, 231, ""),
			drawn("@startditaa", 0, 0, 0, "PlantUML could not render this diagram."),
		},
		PlantUMLRejected: []diagramBlock{
			drawn("@startuml include", 0, 0, 0, "Include directives are not supported."),
			drawn("@startuml includeurl", 0, 0, 0, "Include directives are not supported."),
			drawn("@startuml stdlib", 0, 0, 0, "Include directives are not supported."),
		},
	}
}

// TestCheckPlantUMLLimits は MD-083 / MD-084 / DSP-272 の検査を検証する（UT-812）。
func TestCheckPlantUMLLimits(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*report)
		want   []string
	}{
		{
			name: "実測どおりの状態は合格する",
		},
		{
			name: "salt も ditaa も理由表示でよい（過検出の検査）",
			mutate: func(r *report) {
				r.PlantUML[3] = drawn("@startsalt", 0, 0, 0, "PlantUML could not render this diagram.")
			},
		},
		{
			name: "salt も ditaa も図でよい（過検出の検査）",
			mutate: func(r *report) {
				r.PlantUML[4] = drawn("@startditaa", 1, 100, 50, "")
			},
		},
		{
			name: "4096 px 超えのブロックに図が出たら落ちる（BUG-010 そのもの）",
			mutate: func(r *report) {
				r.PlantUML[2] = drawn("@startuml toolarge", 1, 430, 560, "")
			},
			want: []string{"4096 px の制限に達していない"},
		},
		{
			name: "4096 px 超えのブロックに理由が出ていないと落ちる",
			mutate: func(r *report) {
				r.PlantUML[2] = drawn("@startuml toolarge", 0, 0, 0, "")
			},
			want: []string{"理由が出ていない"},
		},
		{
			name: "正常な図が描けていないと落ちる",
			mutate: func(r *report) {
				r.PlantUML[0] = drawn("@startuml normal", 0, 0, 0, "failed")
			},
			want: []string{"@startuml normal"},
		},
		{
			name: "構文エラーが図にならないと落ちる（FR-024）",
			mutate: func(r *report) {
				r.PlantUML[1] = drawn("@startuml syntaxerror", 0, 0, 0, "reason")
			},
			want: []string{"エラー図が 1 個出るのを期待"},
		},
		{
			name: "salt が図も理由も出さないと落ちる",
			mutate: func(r *report) {
				r.PlantUML[3] = drawn("@startsalt", 0, 0, 0, "")
			},
			want: []string{"図も理由も出ていない"},
		},
		{
			name: "取り込み指令のブロックに図が出たら落ちる（IMP-119）",
			mutate: func(r *report) {
				r.PlantUMLRejected[0] = drawn("@startuml include", 1, 100, 100, "")
			},
			want: []string{"描画対象から外れていない"},
		},
		{
			name: "取り込み指令のブロックが足りないと落ちる",
			mutate: func(r *report) {
				r.PlantUMLRejected = r.PlantUMLRejected[:2]
			},
			want: []string{"拒んだ PlantUML ブロックが 2 件", "stdlib"},
		},
		{
			name: "ブロック名が変わると落ちる（testdata と検査の食い違い）",
			mutate: func(r *report) {
				r.PlantUML[2] = drawn("@startuml", 0, 0, 0, "PlantUML could not render this diagram.")
			},
			want: []string{"@startuml toolarge: 図が文書に見つからない"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := limitsOK()
			if tt.mutate != nil {
				tt.mutate(&got)
			}

			failures := checkPlantUMLLimits(got)

			if len(tt.want) == 0 {
				if len(failures) != 0 {
					t.Fatalf("合格を期待したが %d 件の失敗:\n%s", len(failures), strings.Join(failures, "\n"))
				}

				return
			}

			if len(failures) == 0 {
				t.Fatalf("失敗を期待したが合格した（期待した断片: %v）", tt.want)
			}

			joined := strings.Join(failures, "\n")
			for _, fragment := range tt.want {
				if !strings.Contains(joined, fragment) {
					t.Errorf("失敗メッセージに %q が無い:\n%s", fragment, joined)
				}
			}
		})
	}
}

// TestCountImages は変換結果から画像の枚数を数えられることを見る（UT-811）。
//
// **この数が検証用文書の形を保証している**（verifyAssetsDoc）。
// 画像を減らした文書を渡すと、以降の検査が 0 件で通ってしまう。
func TestCountImages(t *testing.T) {
	tests := []struct {
		name string
		html string
		want int
	}{
		{"画像なし", `<p>text</p>`, 0},
		{"1 枚", `<p><img src="/__local/a" alt="a"></p>`, 1},
		{"3 枚", `<img src="a" alt=""><img src="b" alt=""><img src="c" alt="">`, 3},
		{"属性のない img は数えない", `<p><img></p>`, 0},
		{"文字列 img は数えない", `<p>img タグの話</p>`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countImages(tt.html); got != tt.want {
				t.Errorf("countImages = %d, want %d", got, tt.want)
			}
		})
	}
}
