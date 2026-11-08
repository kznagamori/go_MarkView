package mdfile

import (
	"reflect"
	"testing"
)

// TestKindOf は開けるファイルの種類の判定を検証する
// （UT-105 のケース 1〜11。根拠: FR-010, FR-025, FR-026, FR-027, FR-031 / IMP-105）。
//
// 開けないものを先に置く（UT-013）。
func TestKindOf(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Kind
	}{
		// ケース 1: 取り込み用の断片は図のファイルにしない（FR-025）。
		{"1 .iuml は開けない", "a.iuml", KindNone},

		// ケース 2: 開けない拡張子。
		{"2 .txt", "a.txt", KindNone},
		{"2 .go", "a.go", KindNone},
		{"2 .uml", "a.uml", KindNone},
		{"2 .tif", "a.tif", KindNone},

		// ケース 3: 拡張子が無く、名前が拡張子の文字。
		{"3 md", "md", KindNone},
		{"3 png", "png", KindNone},

		// ケース 4: 名前が拡張子のみ。最終要素の名前で見る。
		{"4 .md", ".md", KindNone},
		{"4 .puml", ".puml", KindNone},
		{"4 .png", ".png", KindNone},
		{"4 docs/.md", "docs/.md", KindNone},

		// ケース 5: 末尾の拡張子で判定する。
		{"5 a.md.txt", "a.md.txt", KindNone},
		{"5 a.png.txt", "a.png.txt", KindNone},

		// ケース 6: 空文字列。
		{"6 空文字列", "", KindNone},

		// ケース 7: Markdown（大文字小文字を区別しない）。
		{"7 README.md", "README.md", KindMarkdown},
		{"7 README.MD", "README.MD", KindMarkdown},
		{"7 .markdown", "a.markdown", KindMarkdown},
		{"7 .mdown", "a.mdown", KindMarkdown},
		{"7 .mkd", "a.mkd", KindMarkdown},

		// ケース 8: PlantUML。
		{"8 .puml", "a.puml", KindPlantUML},
		{"8 .pu", "a.pu", KindPlantUML},
		{"8 .wsd", "a.wsd", KindPlantUML},
		{"8 .plantuml", "a.plantuml", KindPlantUML},
		{"8 A.PUML", "A.PUML", KindPlantUML},

		// ケース 9: Mermaid。
		{"9 .mmd", "a.mmd", KindMermaid},
		{"9 .mermaid", "a.mermaid", KindMermaid},
		{"9 A.MMD", "A.MMD", KindMermaid},

		// ケース 10: 画像（FR-022 の 9 つ）。
		{"10 .png", "a.png", KindImage},
		{"10 .jpg", "a.jpg", KindImage},
		{"10 .jpeg", "a.jpeg", KindImage},
		{"10 .gif", "a.gif", KindImage},
		{"10 .svg", "a.svg", KindImage},
		{"10 .webp", "a.webp", KindImage},
		{"10 .avif", "a.avif", KindImage},
		{"10 .bmp", "a.bmp", KindImage},
		{"10 .ico", "a.ico", KindImage},
		{"10 A.PNG", "A.PNG", KindImage},

		// ケース 11: 最終要素の拡張子だけを見る。
		{"11 a.txt.md", "a.txt.md", KindMarkdown},
		{"11 docs/specs/README.md", "docs/specs/README.md", KindMarkdown},
		{"11 a.b/c.png", "a.b/c.png", KindImage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindOf(tt.in); got != tt.want {
				t.Errorf("KindOf(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestIsMarkdown は Markdown の判定を検証する（UT-105 のケース 3〜7・11。根拠: FR-013, FR-031 / IMP-105）。
//
// README の探索（FR-013）は v1.2.0 でも IsMarkdown のまま使う（IMP-105 の表）。
func TestIsMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"3 拡張子がなく名前が md", "md", false},
		{"4 名前が拡張子のみ", ".md", false},
		{"4 ディレクトリの下の名前が拡張子のみ", "docs/.md", false},
		{"5 二重拡張子は末尾で判定する", "a.md.txt", false},
		{"6 空文字列", "", false},
		{"2 .txt は対象外", "a.txt", false},
		{"10 .png は対象外", "a.png", false},
		{"8 .puml は対象外", "a.puml", false},
		{"7 README.md", "README.md", true},
		{"7 大文字小文字を区別しない", "README.MD", true},
		{"7 .markdown", "a.markdown", true},
		{"7 .mdown", "a.mdown", true},
		{"7 .mkd", "a.mkd", true},
		{"11 a.txt.md", "a.txt.md", true},
		{"11 ディレクトリを含むパス", "docs/specs/README.md", true},
		{"11 ディレクトリ名にドットを含む", "a.b/README.md", true},
		{"ドットで始まる名前でも拡張子があれば対象", ".README.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMarkdown(tt.in); got != tt.want {
				t.Errorf("IsMarkdown(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestPredicates は 5 つの判定の組み合わせを検証する（UT-105 のケース 12。根拠: FR-027, FR-090, NFR-035 / IMP-105）。
//
// IsText と IsOpenable の違いは画像だけである。画像はエディタで開かない（FR-027, FR-090）。
// Markdown と図だけで試すと、取り違えが見えない。
func TestPredicates(t *testing.T) {
	type flags struct{ markdown, diagram, image, text, openable bool }
	tests := []struct {
		in   string
		want flags
	}{
		{"a.md", flags{markdown: true, text: true, openable: true}},
		{"a.puml", flags{diagram: true, text: true, openable: true}},
		{"a.mmd", flags{diagram: true, text: true, openable: true}},
		{"a.png", flags{image: true, openable: true}},
		{"a.txt", flags{}},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := flags{
				markdown: IsMarkdown(tt.in),
				diagram:  IsDiagram(tt.in),
				image:    IsImage(tt.in),
				text:     IsText(tt.in),
				openable: IsOpenable(tt.in),
			}
			if got != tt.want {
				t.Errorf("%s: (IsMarkdown, IsDiagram, IsImage, IsText, IsOpenable) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

// TestImageContentType は画像の Content-Type を検証する（UT-105 のケース 13。根拠: FR-022, AR-041 / IMP-105, IMP-161）。
//
// 期待値は FR-022 の表から人が書いた（UT-031）。
func TestImageContentType(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"a.png", "image/png", true},
		{"a.jpg", "image/jpeg", true},
		{"a.jpeg", "image/jpeg", true},
		{"a.gif", "image/gif", true},
		{"a.svg", "image/svg+xml", true},
		{"a.webp", "image/webp", true},
		{"a.avif", "image/avif", true},
		{"a.bmp", "image/bmp", true},
		{"a.ico", "image/x-icon", true},
		{"A.PNG", "image/png", true},
		{"a.md", "", false},
		{"a.puml", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ImageContentType(tt.in)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ImageContentType(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestExtensionOrder は拡張子の一覧の並びを検証する（UT-105 のケース 14。根拠: FR-037, FR-010 / IMP-105, IMP-303, IMP-310）。
//
// ファイルツリーのフィルターの文言とファイル選択ダイアログの絞り込みは、この順で組み立てる。
// 期待値は FR-037 の表の並びから人が書いた（UT-031）。
func TestExtensionOrder(t *testing.T) {
	images := make([]string, 0, len(ImageTypes))
	for _, it := range ImageTypes {
		images = append(images, it.Ext)
	}

	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{"Extensions", Extensions, []string{".md", ".markdown", ".mdown", ".mkd"}},
		{"PlantUMLExtensions", PlantUMLExtensions, []string{".puml", ".pu", ".wsd", ".plantuml"}},
		{"MermaidExtensions", MermaidExtensions, []string{".mmd", ".mermaid"}},
		{"ImageTypes", images, []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp", ".avif", ".bmp", ".ico"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}
