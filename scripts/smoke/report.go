package main

// ページ（harness.js / collect.js）から返る結果の型（BR-054）。**ページは事実だけを返し、判定は Go 側で行う。**
// main.go から分けた（400 行の目安。IMP-011。v1.2.0 で見える文字の照合の結果が増えたため）。

// report はページから返る結果（harness.js と対になる）。
type report struct {
	Mermaid          []diagramBlock `json:"mermaid"`
	PlantUML         []diagramBlock `json:"plantuml"`
	PlantUMLRejected []diagramBlock `json:"plantumlRejected"`
	Math             mathResult     `json:"math"`
	Images           imageReport    `json:"images"`
	Errors           []string       `json:"errors"`
	Console          []string       `json:"console"`
	ElapsedMS        int            `json:"elapsedMs"`
	UserAgent        string         `json:"userAgent"`
	Fatal            string         `json:"fatal"`

	// v1.1.0 で足した検査（BR-054, E2E-109 の 8〜10）。
	IDs  idReport   `json:"ids"`
	Sort sortReport `json:"sort"`
	Refs refReport  `json:"refs"`

	// v1.2.0 で足した検査（BR-054, E2E-109 の 12。UT-817）。
	Visible visibleReport `json:"visible"`
}

// imageReport は読み込みに失敗した画像の扱い（IMP-226, DSP-123）。
type imageReport struct {
	Imgs     []imageInfo  `json:"imgs"`
	Broken   []brokenInfo `json:"broken"`
	Legacy   int          `json:"legacy"` // img.is-broken の数。4.32.0 より前のフック
	Imported bool         `json:"imported"`
	Error    string       `json:"error"`
}

type imageInfo struct {
	Alt          string `json:"alt"`
	ClassName    string `json:"className"`
	Complete     bool   `json:"complete"`
	NaturalWidth int    `json:"naturalWidth"`
}

type brokenInfo struct {
	TagName   string `json:"tagName"`
	ClassName string `json:"className"`
	Text      string `json:"text"`
}

type diagramBlock struct {
	Index  int    `json:"index"`
	Block  int    `json:"block"` // 図のブロックの一覧（refReport.Blocks）の何番目か。無ければ -1
	Head   string `json:"head"`
	SVG    int    `json:"svg"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Error  string `json:"error"`

	// Mermaid だけが返す（collect.js の collectLook。UT-816）。
	Looks    []string `json:"looks"`    // SVG の中の data-look の値（重複を除く）
	Filtered int      `json:"filtered"` // 計算値の filter が none でない要素の数（影）
}

type mathResult struct {
	Total  int      `json:"total"`
	KaTeX  int      `json:"katex"`
	Failed []string `json:"failed"`
}
