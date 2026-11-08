package main

// 描画スモークの検証用文書（testdata/smoke.md）の要素の並び（BR-054, UT-815）。
//
// **どの要素がどの由来かを、種類ごとの出現順でリテラルとして持つ。** 文書の側に印を足さない——
// サニタイズを通る属性で印を付けると、その印自体が照合の結果に影響しうる。**testdata/smoke.md の
// 要素を足したり消したりしたら、ここも直す。** 食い違うと「数が違う」で落ちる（UT-815 のケース 9）。
// refs.go から分けた（400 行の目安。IMP-011。v1.2.0 で修飾できるブロックの一覧が増えたため）。

// smokeRefLayout は testdata/smoke.md の並び。**testdata/smoke.md と一致させる。**
// 食い違うと「数が違う」で落ちる（UT-815 のケース 9）。
var smokeRefLayout = refLayout{
	Checkboxes: []string{
		originGFM, originGFM, // - [ ] / - [x]
		originWrongKey, originNoKey, originBadShape,
	},
	Tables: []string{
		originGFM, // 本体 2 行（ボタンが付く）
		originGFM, // 本体 1 行（ボタンが付かない）
		originWrongKey, originNoKey, originBadShape,
	},
	Cells: []string{
		// 本体 2 行の表: 見出し 2 + 本体 2 行 x 2 列
		originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
		// 本体 1 行の表: 見出し 2 + 本体 1 行 x 2 列
		originGFM, originGFM, originGFM, originGFM,
		// 生 HTML の表（1 列。見出し 1 + 本体 2 行）
		originWrongKey, originWrongKey, originWrongKey,
		originNoKey, originNoKey, originNoKey,
		originBadShape, originBadShape, originBadShape,
	},
	Links: []linkSpec{
		{originGFM, "../docs/bugs/2026-09-06-bug-008-broken-image-alt-webkitgtk.md"},
		{originGFM, "../docs/bugs/2026-09-14-bug-011-document-id-collision.md"},
		{originGFM, "./e2e/docs/design.md#api"},
		{originGFM, "https://example.com/p"},
		{Origin: originWrongKey},
		{Origin: originNoKey},
		{Origin: originBadShape},
		{originGFM, "../docs/bugs/2026-09-14-bug-014-diagram-marker-spoofing.md"},
		// v1.2.0 の見える文字の照合の節（脚注の参照・裸の www・脚注の戻りリンク）
		{Origin: originGFMNone},
		{originGFM, "www.example.com/q"},
		{Origin: originGFMNone},
	},
	Blocks: []blockSpec{
		// Mermaid の 7 種類
		{originGFM, blockMermaid}, {originGFM, blockMermaid}, {originGFM, blockMermaid},
		{originGFM, blockMermaid}, {originGFM, blockMermaid}, {originGFM, blockMermaid},
		{originGFM, blockMermaid},
		// PlantUML の 2 種類
		{originGFM, blockPlantUML}, {originGFM, blockPlantUML},
		// ラベルに id を書いた Mermaid
		{originGFM, blockMermaid},
		// 生 HTML で偽装したブロック
		{originWrongKey, blockPlantUML}, // !include を含む
		{originNoKey, blockMermaid},
		{originBadShape, blockMermaid},
		{originNoKey, blockCode}, // data-source だけを偽装したコードブロック
	},
	Codes: []string{
		originGFM,     // ```go
		originGFMNone, // 字下げ形式のコードブロック（目印を持たない）
		originWrongKey, originNoKey, originBadShape,
	},
	Texts: smokeTextLayout,
}

// smokeTextLayout は testdata/smoke.md の p / h1〜h6 / li の並び（88 個）。**testdata/smoke.md と一致させる。**
//
// 見出しと Markdown の段落・詰まったリストの項目は gfm。Alerts の見出しの p と、広いリストと脚注の li は
// gfm-none（目印は中の p に付く）。生 HTML のブロックの p（偽の目印の節）は目印を持たないので raw-no-key。
var smokeTextLayout = []string{
	// 0〜6: 冒頭（見出し・段落 2・IMPORTANT 2 つ）
	originGFM, originGFM, originGFM, originGFMNone, originGFM, originGFMNone, originGFM,
	// 7〜15: Mermaid の見出し（h2・h3 x 7）と PlantUML の h2
	originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
	// 16〜19: PlantUML の IMPORTANT 2 つ
	originGFMNone, originGFM, originGFMNone, originGFM,
	// 20〜25: PlantUML の h3 x 2、数式の h2 と段落 3
	originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
	// 26〜29: 画像の h2・段落・IMPORTANT
	originGFM, originGFM, originGFMNone, originGFM,
	// 30〜37: 画像の説明と画像の段落 x 4
	originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
	// 38〜41: 文書の id の h2・段落・IMPORTANT
	originGFM, originGFM, originGFMNone, originGFM,
	// 42〜46: Status の h3 と段落、ラベルの h3 と段落、目印の照合の h2
	originGFM, originGFM, originGFM, originGFM, originGFM,
	// 47〜49: 目印の照合の IMPORTANT（本文の段落 2 つ）
	originGFMNone, originGFM, originGFM,
	// 50〜55: GFM の表・タスク・リンクの節（h3・段落 2・タスクの li 2・リンクの段落）
	originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
	// 56〜63: 偽の目印の節（h3・段落と、生 HTML の p x 6）
	originGFM, originGFM,
	originNoKey, originNoKey, originNoKey, originNoKey, originNoKey, originNoKey,
	// 64〜69: 偽装した図の節、コードブロックと修飾の目印の節
	originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
	// 70〜73: 広いリスト（li は gfm-none、中の p は gfm）
	originGFMNone, originGFM, originGFMNone, originGFM,
	// 74〜76: 生 HTML で偽装した修飾の目印
	originWrongKey, originNoKey, originBadShape,
	// 77〜85: 見える文字の照合の節（h2・段落 6・入れ子のリストの li 2）
	originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM, originGFM,
	// 86・87: 脚注（li は gfm-none、中の p は gfm）
	originGFMNone, originGFM,
}
