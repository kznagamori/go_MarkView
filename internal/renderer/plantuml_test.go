package renderer

import (
	"strings"
	"testing"
)

// TestRender_PlantUML は PlantUML ブロックの出力を検証する
// （UT-215。根拠: FR-024, MD-083 / IMP-119）。
//
// **境界値を先に置く**（UT-013）。`uml` と `plantuml2` を対象にしてしまう
// 実装は、対象にする側のケースだけでは検出できない。
func TestRender_PlantUML(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-215 ケース 1: uml は対象にしない（MD-083）
		{
			name:        "uml のコードブロックは PlantUML にしない",
			in:          "```uml\n@startuml\n@enduml\n```",
			notContains: []string{"data-plantuml", "plantuml-source", "data-source"},
		},
		// UT-215 ケース 2: 前方一致で拾わない
		{
			name:        "plantuml2 は PlantUML にしない",
			in:          "```plantuml2\n@startuml\n@enduml\n```",
			notContains: []string{"data-plantuml", "plantuml-source"},
		},
		// UT-215 ケース 11: Mermaid と取り違えない
		{
			name:        "mermaid のコードブロックは PlantUML にしない",
			in:          "```mermaid\ngraph TD\n```",
			contains:    []string{`data-mermaid="1"`},
			notContains: []string{"data-plantuml", "plantuml-source"},
		},
		{
			name:        "通常のコードブロック",
			in:          "```go\nx := 1\n```",
			notContains: []string{"data-plantuml", "plantuml-source", "data-source"},
		},

		// UT-215 ケース 3・6・7
		{
			name: "plantuml ブロック",
			in:   "```plantuml\n@startuml\n@enduml\n```",
			contains: []string{
				`data-lang="plantuml"`,
				`data-plantuml="1"`,
				`data-source="@startuml`,
				`<pre class="plantuml-source">@startuml`,
			},
			notContains: []string{"chroma"},
		},
		// UT-215 ケース 4: puml も同じ扱い（MD-083）
		{
			name:     "puml ブロック",
			in:       "```puml\n@startuml\n@enduml\n```",
			contains: []string{`data-plantuml="1"`, `<pre class="plantuml-source">`},
		},
		// UT-215 ケース 5: 言語名の大小を区別しない
		{
			name:     "言語名が大文字混じり",
			in:       "```PlantUML\n@startuml\n@enduml\n```",
			contains: []string{`data-plantuml="1"`},
		},
		{
			name:     "言語名がすべて大文字",
			in:       "```PUML\n@startuml\n@enduml\n```",
			contains: []string{`data-plantuml="1"`},
		},

		// UT-215 ケース 8: エスケープ
		{
			name: "記号を含むソース",
			in:   "```plantuml\nA -> B : \"q\" & <x>\n```",
			contains: []string{
				`data-source="A -&gt; B : &#34;q&#34; &amp; &lt;x&gt;"`,
				"<pre class=\"plantuml-source\">A -&gt; B : &#34;q&#34; &amp; &lt;x&gt;</pre>",
			},
		},

		// UT-090 に従って追加した境界値
		{
			name:     "空の PlantUML ブロック",
			in:       "```plantuml\n```",
			contains: []string{`data-source=""`, `<pre class="plantuml-source"></pre>`},
		},
		{
			name:        "本文中の plantuml という語",
			in:          "plantuml と puml の話",
			notContains: []string{"data-plantuml", "plantuml-source"},
		},
		{
			name:        "コードブロック内の plantuml という文字列",
			in:          "```go\nplantuml := 1\n```",
			notContains: []string{"data-plantuml", "plantuml-source"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, ng := range tt.notContains {
				if strings.Contains(got, ng) {
					t.Errorf("出力に %q が含まれている\n出力: %s", ng, got)
				}
			}
		})
	}
}

// TestRender_PlantUMLStructure は IMP-119 が固定した構造そのものを検証する。
//
// 期待値はサニタイズ後の形、つまりフロントエンドが実際に受け取る HTML である。
// 最後段のサニタイズ（IMP-116）が数値文字参照を実体へ戻すため、属性中の
// &#10; は生の改行として現れる。
//
// data-source と data-plantuml はフロントエンドの生命線である。PlantUML は
// 描画後に <pre> が SVG へ置き換わり、DOM から原文が失われる。属性名が変わると
// コピーボタン（FR-060）とテーマ切り替え時の再描画（IMP-233）が同時に壊れる。
func TestRender_PlantUMLStructure(t *testing.T) {
	const want = `<div class="code-block" data-lang="plantuml" data-plantuml="1" data-source="@startuml
Alice -&gt; Bob
@enduml">
<pre class="plantuml-source">@startuml
Alice -&gt; Bob
@enduml</pre>
</div>
`

	got := render(t, "```plantuml\n@startuml\nAlice -> Bob\n@enduml\n```")
	if got != want {
		t.Errorf("出力が構造と一致しない\n got: %q\nwant: %q", got, want)
	}
}

// TestRender_NeedsPlantUML は遅延ロードの判定を検証する
// （UT-215 ケース 9・10。根拠: MD-085 / IMP-119, NFR-013）。
//
// **同梱する資産は 5 MiB を超える**（MD-085）。誤って true になると、
// PlantUML を含まない文書を開いただけで読み込みが起きる。
func TestRender_NeedsPlantUML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"PlantUML を含まない文書", "# H\n\npara", false},
		{"Mermaid だけを含む文書", "```mermaid\ngraph TD\n```", false},
		{"uml ブロックだけを含む文書", "```uml\n@startuml\n@enduml\n```", false},
		{"plantuml ブロックを含む文書", "```plantuml\n@startuml\n@enduml\n```", true},
		{"puml ブロックを含む文書", "```puml\n@startuml\n@enduml\n```", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "")
			if err != nil {
				t.Fatalf("Render がエラーを返した: %v", err)
			}
			if res.NeedsPlantUML != tt.want {
				t.Errorf("NeedsPlantUML = %v, want %v\n出力: %s", res.NeedsPlantUML, tt.want, res.HTML)
			}
		})
	}
}

// TestRender_PlantUMLIncludeDirective は取り込み指令の検査を検証する
// （UT-216。根拠: MD-084, NFR-032 / IMP-119）。
//
// **拒むべきものと拒んではいけないものが隣り合う。** 「`!include` という
// 文字列があれば拒む」実装でも拒む側のケースはすべて通るため、
// **拒んではいけないケースを先に置く**（UT-013）。
func TestRender_PlantUMLIncludeDirective(t *testing.T) {
	tests := []struct {
		name string
		body string
		// want が true なら描画対象（data-plantuml を持つ）。
		// false なら拒否（data-puml-error="include" を持つ）。
		want bool
	}{
		// UT-216 ケース 1〜3: 拒んではいけないもの
		{"from を伴わない !theme は組み込みテーマ", "@startuml\n!theme plain\nA -> B\n@enduml", true},
		{"行頭でない !include は指令ではない", "@startuml\nAlice -> Bob : !include a\n@enduml", true},
		{"コメント行の !include", "@startuml\n' !include a.puml\n@enduml", true},
		{"指令に似た語を含む本文", "@startuml\nnote right : include this\n@enduml", true},

		// UT-216 ケース 4〜11: 拒むもの
		{"!include", "@startuml\n!include foo.puml\n@enduml", false},
		{"行頭に空白のある !include", "@startuml\n  !include foo.puml\n@enduml", false},
		{"タブで字下げした !include", "@startuml\n\t!include foo.puml\n@enduml", false},
		{"大文字の !INCLUDE", "@startuml\n!INCLUDE FOO.PUML\n@enduml", false},
		{"!includeurl", "@startuml\n!includeurl http://example.com/a\n@enduml", false},
		{"!includesub", "@startuml\n!includesub a.puml!X\n@enduml", false},
		{"!import", "@startuml\n!import a.puml\n@enduml", false},
		{"from を伴う !theme", "@startuml\n!theme x from https://example.com/\n@enduml", false},
		{"標準ライブラリ", "@startuml\n!include <C4/C4_Container>\n@enduml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, "```plantuml\n"+tt.body+"\n```")

			hasTarget := strings.Contains(got, `data-plantuml="1"`)
			hasError := strings.Contains(got, `data-puml-error="include"`)

			if hasTarget == hasError {
				t.Fatalf("描画対象と拒否の印が両立している（target=%v, error=%v）\n出力: %s",
					hasTarget, hasError, got)
			}
			if hasTarget != tt.want {
				t.Errorf("描画対象 = %v, want %v\n出力: %s", hasTarget, tt.want, got)
			}

			// 拒んだ場合も原文は残す。フロントエンドが理由とともに出す（DSP-272）。
			if !strings.Contains(got, `<pre class="plantuml-source">`) {
				t.Errorf("拒否したブロックでも原文を残すこと\n出力: %s", got)
			}
		})
	}
}

// TestRender_NeedsPlantUML_WithIncludeDirective は、拒んだブロックが
// 遅延ロードを起こさないことを検証する（UT-216 ケース 12・13。根拠: IMP-119）。
//
// **全部のブロックが拒まれた文書で 5 MiB の資産を読むのは無駄である**（NFR-013）。
func TestRender_NeedsPlantUML_WithIncludeDirective(t *testing.T) {
	const rejected = "```plantuml\n@startuml\n!include a.puml\n@enduml\n```"
	const normal = "```plantuml\n@startuml\nA -> B\n@enduml\n```"

	tests := []struct {
		name string
		in   string
		want bool
	}{
		// UT-216 ケース 12
		{"指令を含むブロックだけの文書", rejected, false},
		{"指令を含むブロックが 2 つ", rejected + "\n\n" + rejected, false},
		// UT-216 ケース 13
		{"指令を含むブロックと正常なブロック", rejected + "\n\n" + normal, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := New().Render([]byte(tt.in), "")
			if err != nil {
				t.Fatalf("Render がエラーを返した: %v", err)
			}
			if res.NeedsPlantUML != tt.want {
				t.Errorf("NeedsPlantUML = %v, want %v\n出力: %s", res.NeedsPlantUML, tt.want, res.HTML)
			}
		})
	}
}
