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

// noSizeImg は固有の大きさを持たない SVG 1 枚ぶんの結果を作る（naturalWidth が 0 になりうる。UT-811 のケース 10）。
func noSizeImg() imageInfo {
	return imageInfo{Alt: noSizeImageAlt, Complete: true, NaturalWidth: 0}
}

// brokenSpan は置き換え後の要素 1 つぶんの結果を作る。
func brokenSpan(text string) brokenInfo {
	return brokenInfo{TagName: "SPAN", ClassName: "img-broken", Text: text}
}

// fixedImages は BUG-008 を修正したあとに期待される状態を作る。
//
// 読める画像が 2 枚（1 枚は固有の大きさを持たない SVG）残り、失敗した 2 枚は span へ置き換わっている。
// 片方は代替テキストを持ち、もう片方は空である（UT-811 のケース 1）。
func fixedImages() imageReport {
	return imageReport{
		Imported: true,
		Imgs:     []imageInfo{okImg(), noSizeImg()},
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
				Imgs:     []imageInfo{okImg(), noSizeImg()},
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan("")},
			},
		},
		{
			name: "4.32.0 より前の実装は落ちる（img.is-broken が残る）",
			images: imageReport{
				Imported: true,
				Legacy:   2,
				Imgs: []imageInfo{
					okImg(), noSizeImg(),
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
				Imgs:     []imageInfo{okImg(), noSizeImg()},
				Broken:   []brokenInfo{brokenSpan(""), brokenSpan("")},
			},
			want: []string{"が本文として読めない"},
		},
		{
			name: "正常な画像まで置き換えると落ちる（過検出）",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{noSizeImg()},
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan(""), brokenSpan(okImageAlt)},
			},
			want: []string{".img-broken が 3 個", "読める画像が 1 枚"},
		},
		{
			// UT-811 ケース 6: 固有の大きさを持つ読めるはずの画像（sample.png）の naturalWidth が 0
			name: "読めるはずの画像が読めていないと落ちる",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{{Alt: okImageAlt, Complete: true, NaturalWidth: 0}, noSizeImg()},
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan("")},
			},
			want: []string{"読めるはずの画像が読めていない"},
		},
		{
			name: "span 以外の要素にすると落ちる",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{okImg(), noSizeImg()},
				Broken: []brokenInfo{
					{TagName: "DIV", ClassName: "img-broken", Text: brokenImageAlt},
					brokenSpan(""),
				},
			},
			want: []string{"DIV 要素"},
		},
		{
			// UT-811 ケース 10: 大きさを持たない SVG は naturalWidth で判定しない（過検出の検査。4.80.0）
			name: "固有の大きさを持たない SVG の naturalWidth が 0 でも合格する",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{okImg(), {Alt: noSizeImageAlt, Complete: true, NaturalWidth: 0}},
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan("")},
			},
		},
		{
			// UT-811 ケース 11: naturalWidth で読み込みの失敗を判定する実装（4.80.0）
			name: "固有の大きさを持たない SVG を置き換えると落ちる",
			images: imageReport{
				Imported: true,
				Imgs:     []imageInfo{okImg()},
				Broken:   []brokenInfo{brokenSpan(brokenImageAlt), brokenSpan(""), brokenSpan(noSizeImageAlt)},
			},
			want: []string{"固有の大きさを持たない SVG が .img-broken に置き換えられた", "読める画像が 1 枚"},
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
//	toolarge     図が出ず、理由が出る（上限の 100,000 px を超える。BUG-030）
//	salt         実測では図が出た
//	ditaa        実測では理由が出た
//	nearlimit    図が出る（上限のすぐ下。高さは Windows の Chromium で 90,120 px、Linux の CI の Chrome で 79,212 px。BUG-030）
func limitsOK() report {
	const unsupported = "PlantUML could not render this diagram. It may be unsupported or larger than 100,000 px."

	return report{
		PlantUML: []diagramBlock{
			drawn("@startuml normal", 1, 112, 164, ""),
			drawn("@startuml syntaxerror", 1, 430, 160, ""),
			drawn("@startuml toolarge", 0, 0, 0, unsupported),
			drawn("@startsalt", 1, 353, 231, ""),
			drawn("@startditaa", 0, 0, 0, unsupported),
			drawn("@startuml nearlimit", 1, 413, 90120, ""),
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
				r.PlantUML[3] = drawn("@startsalt", 0, 0, 0, "PlantUML could not render this diagram. It may be unsupported or larger than 100,000 px.")
			},
		},
		{
			name: "salt も ditaa も図でよい（過検出の検査）",
			mutate: func(r *report) {
				r.PlantUML[4] = drawn("@startditaa", 1, 100, 50, "")
			},
		},
		{
			name: "上限超えのブロックに図が出たら落ちる（BUG-010 そのもの）",
			mutate: func(r *report) {
				r.PlantUML[2] = drawn("@startuml toolarge", 1, 419, 116520, "")
			},
			want: []string{"上限の 100,000 px に達していない"},
		},
		{
			name: "上限のすぐ下のブロックが理由表示になったら落ちる（上限が 4096 px のまま。BUG-030 そのもの）",
			mutate: func(r *report) {
				r.PlantUML[5] = drawn("@startuml nearlimit", 0, 0, 0, "PlantUML could not render this diagram.")
			},
			want: []string{"@startuml nearlimit", "上限が 100,000 px になっていない"},
		},
		{
			name: "上限のすぐ下のブロックの図が低いと落ちる（検証用データが小さくなった）",
			mutate: func(r *report) {
				r.PlantUML[5] = drawn("@startuml nearlimit", 1, 413, 4000, "")
			},
			want: []string{"高さ 4000"},
		},
		{
			// UT-812 ケース 15: Linux の CI の Chrome の実測（フォントの違いで Windows より 1 割ほど低い）。
			// 閾値が 80,000 のときは、これで落ちた（v1.2.0-rc.2 の前の CI）
			name: "上限のすぐ下のブロックの Linux の実測（343×79,212）は合格する",
			mutate: func(r *report) {
				r.PlantUML[5] = drawn("@startuml nearlimit", 1, 343, 79212, "")
			},
		},
		{
			// UT-812 ケース 16: 閾値ちょうどは合格する（UT-013）
			name: "上限のすぐ下のブロックの高さが閾値ちょうど（60,000）なら合格する",
			mutate: func(r *report) {
				r.PlantUML[5] = drawn("@startuml nearlimit", 1, 343, 60000, "")
			},
		},
		{
			// UT-812 ケース 17: 閾値より 1 px 低いと落ちる（UT-013）
			name: "上限のすぐ下のブロックの高さが閾値より 1 px 低い（59,999）と落ちる",
			mutate: func(r *report) {
				r.PlantUML[5] = drawn("@startuml nearlimit", 1, 343, 59999, "")
			},
			want: []string{"高さ 59999"},
		},
		{
			name: "上限のすぐ下のブロックが文書に無いと落ちる",
			mutate: func(r *report) {
				r.PlantUML = r.PlantUML[:5]
			},
			want: []string{"描画対象の PlantUML ブロックが 5 件", "@startuml nearlimit: 図が文書に見つからない"},
		},
		{
			name: "上限超えのブロックに理由が出ていないと落ちる",
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
				r.PlantUML[2] = drawn("@startuml", 0, 0, 0, "PlantUML could not render this diagram. It may be unsupported or larger than 100,000 px.")
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
