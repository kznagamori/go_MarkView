package document

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// codeBasic は UT-118 の基本の文書（LF。中身の行の範囲 Body は [6, 10)）。
const codeBasic = "```go\na\nb\n```\n"

// bom は UTF-8 の BOM。
const bom = "\xEF\xBB\xBF"

// invalidUTF8 は UT-107 のケース 7 の 2 つの並び（1 バイト / 長さの変わらない 3 バイト）。
var invalidUTF8 = []string{"\xFF", "\xFF\xFE\xFD"}

// codeRef は種類 kind の n 番目の目印（鍵は testKey）を返す。
func codeRef(kind RefKind, n int) Ref { return Ref{Key: testKey, Kind: kind, Index: n} }

// wantPatch は PlanCode の結果が want と一致することを確かめる。
func wantPatch(t *testing.T, got Patch, changed bool, err error, want Patch) {
	t.Helper()

	if err != nil || !changed {
		t.Fatalf("PlanCode = changed %v, err %v, want 書き換える", changed, err)
	}
	if got.Offset != want.Offset || string(got.Old) != string(want.Old) || string(got.New) != string(want.New) {
		t.Errorf("Patch = {%d %q %q}, want {%d %q %q}", got.Offset, got.Old, got.New, want.Offset, want.Old, want.New)
	}
}

// checkStructure は UT-118 のケース 23・24 を確かめる: Patch を適用した結果を正規化して Locate で
// 解き直し、ブロックの並びと数が変わらず、書き換えたブロックの中身が入力と一致し、言語指定と他の
// ブロックの中身が変わらないこと。Old の範囲の外のバイト列が 1 バイトも変わらないこと。
func checkStructure(t *testing.T, r *renderer.Renderer, raw []byte, kind RefKind, n int, input string, p Patch) {
	t.Helper()

	after, err := p.Apply(raw)
	if err != nil {
		t.Fatalf("Apply がエラーを返した: %v", err)
	}
	// ケース 24: Old の範囲の外のバイト列は 1 バイトも変わらない（BOM・CR・末尾の改行を含む）。
	if !bytes.Equal(after[:p.Offset], raw[:p.Offset]) || !bytes.Equal(after[p.Offset+len(p.New):], raw[p.Offset+len(p.Old):]) {
		t.Errorf("Old の範囲の外が変わった\n前: %q\n後: %q", raw, after)
	}

	locsOf := func(b []byte) renderer.Locations {
		text, _ := Normalize(b)
		locs, err := r.Locate(text)
		if err != nil {
			t.Fatalf("Locate がエラーを返した: %v", err)
		}
		return locs
	}
	textOf := func(b []byte) string { text, _ := Normalize(b); return string(text) }
	before, got := locsOf(raw), locsOf(after)

	// ケース 23: ブロックの並びと、タスク・表・コードブロック・図・修飾できるブロックの数が変わらない。
	if !reflect.DeepEqual(before.Blocks, got.Blocks) {
		t.Errorf("Blocks が変わった: %v → %v", before.Blocks, got.Blocks)
	}
	if len(before.Tasks) != len(got.Tasks) || len(before.Tables) != len(got.Tables) ||
		len(before.Codes) != len(got.Codes) || len(before.Mermaids) != len(got.Mermaids) ||
		len(before.PlantUMLs) != len(got.PlantUMLs) || before.Texts != got.Texts {
		t.Errorf("ブロックの数が変わった: %+v → %+v", before, got)
	}
	fences := func(l renderer.Locations) []renderer.FenceLocation {
		switch kind {
		case RefMermaid:
			return l.Mermaids
		case RefPlantUML:
			return l.PlantUMLs
		default:
			return l.Codes
		}
	}
	bf, af := fences(before), fences(got)
	if n >= len(af) || n >= len(bf) {
		t.Fatalf("書き換えたブロック %d が無い: 前 %d 個 / 後 %d 個", n, len(bf), len(af))
	}
	// 書き換えたブロックの中身（前置きを除き LF にそろえたもの）が入力と一致し、言語指定が変わらない。
	content := func(s string, f renderer.FenceLocation) string {
		var lines []string
		for _, l := range f.Lines {
			lines = append(lines, s[l.Start:l.Stop])
		}
		return strings.Join(lines, "\n")
	}
	wantInput := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(input)
	if got := content(textOf(after), af[n]); got != wantInput {
		t.Errorf("書き換えたブロックの中身 = %q, want %q", got, wantInput)
	}
	if af[n].Info != bf[n].Info {
		t.Errorf("言語指定が変わった: %q → %q", bf[n].Info, af[n].Info)
	}
	// 他のブロックの中身が変わらない（位置ではなく中身を比べる）。
	for i := range bf {
		if i != n && content(textOf(raw), bf[i]) != content(textOf(after), af[i]) {
			t.Errorf("%d 番目のブロックの中身が変わった", i)
		}
	}
}

// TestCodeSource_Markdown は Markdown の中のブロックの編集欄の値を検証する
// （UT-118 のケース 1・6・9・11・20〜22・25・30・31。根拠: FR-021, FR-145 / IMP-106, IMP-121, IMP-122）。
func TestCodeSource_Markdown(t *testing.T) {
	r := renderer.New()

	// 拒むケースを先に置く（UT-013）。
	t.Run("20 前置きが行ごとに違うブロック", func(t *testing.T) {
		for _, src := range []string{"> ```\n> a\n>b\n> ```\n", "1. ```\n\tz\n   ```\n"} {
			if _, err := CodeSource(r, []byte(src), mdfile.KindMarkdown, codeRef(RefCode, 0)); !errors.Is(err, ErrLocked) {
				t.Errorf("CodeSource(%q) のエラー = %v, want ErrLocked", src, err)
			}
		}
	})
	t.Run("30 リストの中で閉じも中身も無いブロック", func(t *testing.T) {
		if _, err := CodeSource(r, []byte("- ```"), mdfile.KindMarkdown, codeRef(RefCode, 0)); !errors.Is(err, ErrLocked) {
			t.Errorf("エラー = %v, want ErrLocked（前置きを決められない）", err)
		}
	})
	t.Run("21 番号がコードブロックの数以上", func(t *testing.T) {
		if _, err := CodeSource(r, []byte(codeBasic), mdfile.KindMarkdown, codeRef(RefCode, 1)); !errors.Is(err, ErrRefNotFound) {
			t.Errorf("エラー = %v, want ErrRefNotFound", err)
		}
	})
	t.Run("22 不正なUTF-8", func(t *testing.T) {
		for _, bad := range invalidUTF8 {
			src := "```go\na" + bad + "\n```\n"
			if _, err := CodeSource(r, []byte(src), mdfile.KindMarkdown, codeRef(RefCode, 0)); !errors.Is(err, ErrNotEditable) {
				t.Errorf("CodeSource(%q) のエラー = %v, want ErrNotEditable", src, err)
			}
		}
	})

	tests := []struct {
		name string
		src  string
		want string
	}{
		{"1 基本の文書（最後の改行を含めない）", codeBasic, "a\nb"},
		{"6 CRLFの文書", "```go\r\na\r\nb\r\n```\r\n", "a\nb"},
		{"9 空のブロック", "```\n```\n", ""},
		{"11 引用の中のブロック", "> ```\n> a\n>\n> b\n> ```\n", "a\n\nb"},
		{"25 開きの囲いの行で終わる文書", "```go", ""},
		{"31 中身の1行目が空の行の引用の中", "> ```\n>\n> a\n> ```\n", "\na"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CodeSource(r, []byte(tt.src), mdfile.KindMarkdown, codeRef(RefCode, 0))
			if err != nil || got != tt.want {
				t.Errorf("CodeSource(%q) = %q, %v, want %q", tt.src, got, err, tt.want)
			}
		})
	}

	// ケース 19: 字下げ形式と数式は数えない。
	t.Run("19 字下げ形式と数式の後", func(t *testing.T) {
		const src = "para\n\n    x\n\n```math\na\n```\n\n```go\ny\n```\n"
		if got, err := CodeSource(r, []byte(src), mdfile.KindMarkdown, codeRef(RefCode, 0)); err != nil || got != "y" {
			t.Errorf("CodeSource = %q, %v, want %q", got, err, "y")
		}
	})
}

// TestPlanCode_Markdown は Markdown の中のブロックの書き換えを検証する
// （UT-118。根拠: FR-021, FR-143, FR-145 / IMP-106, IMP-121, IMP-122）。
//
// **ケース 3 と 4 は対で意味を持つ。** 編集欄の値に最後の改行を含める実装では、何も変えずに Save
// しただけで空の行が増え（3 で落ちる）、入力の末尾の改行を捨てる実装では利用者の空の行が消える
// （4 で落ちる）。**ケース 16 は 4.76.0 で直した手順そのものである**（閉じの囲いの前置きの書き戻し）。
func TestPlanCode_Markdown(t *testing.T) {
	r := renderer.New()
	plan := func(src string, kind RefKind, n int, text string) (Patch, bool, error) {
		return PlanCode(r, []byte(src), mdfile.KindMarkdown, codeRef(kind, n), text)
	}

	// 拒む・書かないケースを先に置く（UT-013）。
	t.Run("3 同じ内容", func(t *testing.T) {
		if _, changed, err := plan(codeBasic, RefCode, 0, "a\nb"); err != nil || changed {
			t.Errorf("PlanCode = changed %v, err %v, want 偽, nil", changed, err)
		}
	})
	t.Run("9 空のブロックに空", func(t *testing.T) {
		if _, changed, err := plan("```\n```\n", RefCode, 0, ""); err != nil || changed {
			t.Errorf("PlanCode = changed %v, err %v, want 偽, nil", changed, err)
		}
	})
	t.Run("18 図は種類ごとに数える", func(t *testing.T) {
		if _, _, err := plan("```mermaid\ngraph TD\n```\n", RefCode, 0, "x"); !errors.Is(err, ErrRefNotFound) {
			t.Errorf("code:0 のエラー = %v, want ErrRefNotFound", err)
		}
	})
	t.Run("20 前置きが行ごとに違うブロック", func(t *testing.T) {
		for _, src := range []string{"> ```\n> a\n>b\n> ```\n", "1. ```\n\tz\n   ```\n"} {
			if _, _, err := plan(src, RefCode, 0, "x"); !errors.Is(err, ErrLocked) {
				t.Errorf("PlanCode(%q) のエラー = %v, want ErrLocked", src, err)
			}
		}
	})
	t.Run("21 番号がコードブロックの数以上", func(t *testing.T) {
		if _, _, err := plan(codeBasic, RefCode, 3, "x"); !errors.Is(err, ErrRefNotFound) {
			t.Errorf("エラー = %v, want ErrRefNotFound", err)
		}
	})
	t.Run("22 不正なUTF-8", func(t *testing.T) {
		for _, bad := range invalidUTF8 {
			if _, _, err := plan("```go\na"+bad+"\n```\n", RefCode, 0, "x"); !errors.Is(err, ErrNotEditable) {
				t.Errorf("エラー = %v, want ErrNotEditable", err)
			}
		}
	})

	// 書き換えるケース。check が真なら UT-118 のケース 23・24 の確かめも行う。
	tests := []struct {
		name  string
		src   string
		kind  RefKind
		text  string
		want  Patch
		check bool
	}{
		{"2 基本", codeBasic, RefCode, "x", Patch{6, []byte("a\nb\n"), []byte("x\n")}, true},
		{"4 入力の末尾の改行は最後の空の行として残る", codeBasic, RefCode, "a\nb\n", Patch{6, []byte("a\nb\n"), []byte("a\nb\n\n")}, false},
		{"5 入力がCRLF", codeBasic, RefCode, "x\r\ny", Patch{6, []byte("a\nb\n"), []byte("x\ny\n")}, false},
		{"6 CRLFの文書", "```go\r\na\r\nb\r\n```\r\n", RefCode, "x\ny", Patch{7, []byte("a\r\nb\r\n"), []byte("x\r\ny\r\n")}, true},
		{"7 BOMの分だけ後ろ", bom + codeBasic, RefCode, "x", Patch{9, []byte("a\nb\n"), []byte("x\n")}, false},
		{"8 TOMLのFrontMatterの後", "+++\na = 1\n+++\n```\nx\n```\n", RefCode, "y", Patch{18, []byte("x\n"), []byte("y\n")}, false},
		{"9 空のブロックに書く", "```\n```\n", RefCode, "x", Patch{4, nil, []byte("x\n")}, false},
		{"10 文書の終わりで閉じたブロック", "```\na", RefCode, "x\ny", Patch{4, []byte("a"), []byte("x\ny")}, true},
		{"11 引用の中", "> ```\n> a\n>\n> b\n> ```\n", RefCode, "x\n\ny", Patch{6, []byte("> a\n>\n> b\n"), []byte("> x\n>\n> y\n")}, true},
		{"12 リストの中", "- ```\n  a\n  ```\n", RefCode, "x\n\ny", Patch{6, []byte("  a\n"), []byte("  x\n\n  y\n")}, true},
		{"13 囲いを延ばす", codeBasic, RefCode, "x\n```\ny", Patch{0, []byte("```go\na\nb\n```"), []byte("````go\nx\n```\ny\n````")}, true},
		{"14 半角空白3つと4つの並び", codeBasic, RefCode, "   ````", Patch{0, []byte("```go\na\nb\n```"), []byte("`````go\n   ````\n`````")}, false},
		{"14 半角空白4つと3つの並びは延ばさない", codeBasic, RefCode, "    ```", Patch{6, []byte("a\nb\n"), []byte("    ```\n")}, false},
		{"14 行の途中の並びは延ばさない", codeBasic, RefCode, "a ``` b", Patch{6, []byte("a\nb\n"), []byte("a ``` b\n")}, false},
		{"14 囲いより短い並びは延ばさない", codeBasic, RefCode, "``", Patch{6, []byte("a\nb\n"), []byte("``\n")}, false},
		{"15 チルダの囲いにバッククォート", "~~~\na\n~~~\n", RefCode, "```", Patch{4, []byte("a\n"), []byte("```\n")}, false},
		{"15 チルダの囲いにチルダ4つ", "~~~\na\n~~~\n", RefCode, "~~~~", Patch{0, []byte("~~~\na\n~~~"), []byte("~~~~~\n~~~~\n~~~~~")}, false},
		{"16 引用の中で囲いを延ばす", "> ```\n> a\n> ```\n", RefCode, "```", Patch{2, []byte("```\n> a\n> ```"), []byte("````\n> ```\n> ````")}, true},
		{"17 閉じが無いので開きだけ延ばす", "```\na", RefCode, "```", Patch{0, []byte("```\na"), []byte("````\n```")}, true},
		{"18 Mermaidのブロック", "```mermaid\ngraph TD\n```\n", RefMermaid, "graph LR", Patch{11, []byte("graph TD\n"), []byte("graph LR\n")}, false},
		{"25 開きの囲いの行で終わる文書", "```go", RefCode, "x\ny", Patch{5, nil, []byte("\nx\ny")}, true},
		{"26 CRだけの文書", "```\ra\r```\r", RefCode, "x\ny", Patch{4, []byte("a\r"), []byte("x\ry\r")}, true},
		{"27 閉じの無いブロックに末尾の改行", "```\na", RefCode, "x\ny\n", Patch{4, []byte("a"), []byte("x\ny\n\n")}, true},
		{"27 開きの囲いの行で終わる文書に末尾の改行", "```go", RefCode, "x\ny\n", Patch{5, nil, []byte("\nx\ny\n\n")}, true},
		{"28 開きの囲いの行の改行コードにそろえる", "x\r\n\n```\na\n```\n", RefCode, "b\nc", Patch{8, []byte("a\n"), []byte("b\nc\n")}, true},
		{"29 引用の中でタブを含む並び", "> ```\n> a\n> ```\n", RefCode, "\t```", Patch{2, []byte("```\n> a\n> ```"), []byte("````\n> \t```\n> ````")}, true},
		{"29 最上位でタブを含む並び", codeBasic, RefCode, "\t```", Patch{0, []byte("```go\na\nb\n```"), []byte("````go\n\t```\n````")}, true},
		{"30 リストの中の空のブロック", "- ```\n  ```\n", RefCode, "x", Patch{6, nil, []byte("  x\n")}, true},
		{"31 中身の1行目が空の行の引用の中", "> ```\n>\n> a\n> ```\n", RefCode, "\nb", Patch{6, []byte(">\n> a\n"), []byte(">\n> b\n")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, changed, err := plan(tt.src, tt.kind, 0, tt.text)
			wantPatch(t, p, changed, err, tt.want)
			if tt.check && err == nil {
				checkStructure(t, r, []byte(tt.src), tt.kind, 0, tt.text, p)
			}
		})
	}
}

// plantUMLFile は UT-119 の基本の PlantUML ファイル（図 0 は [0, 26)、図 1 は [28, 54)）。
const plantUMLFile = "@startuml a\nA -> B\n@enduml\n\n@startuml b\nC -> D\n@enduml\n"

// diagram1 は plantUMLFile の図 1 のソース（@end… の行の改行を含まない）。
const diagram1 = "@startuml b\nC -> D\n@enduml"

// TestCodeSource_DiagramFiles は図のファイルの編集欄の値を検証する
// （UT-119 のケース 1・7・9・10・13・17・21・22。根拠: FR-025, FR-026, FR-145, MD-086, MD-087 / IMP-122, IMP-126）。
func TestCodeSource_DiagramFiles(t *testing.T) {
	r := renderer.New()

	// 拒むケースを先に置く（UT-013）。
	t.Run("11 PlantUMLファイルの範囲外と種類違い", func(t *testing.T) {
		for _, ref := range []Ref{codeRef(RefPlantUML, 2), codeRef(RefMermaid, 0), codeRef(RefCode, 0)} {
			if _, err := CodeSource(r, []byte(plantUMLFile), mdfile.KindPlantUML, ref); !errors.Is(err, ErrRefNotFound) {
				t.Errorf("CodeSource(%+v) のエラー = %v, want ErrRefNotFound", ref, err)
			}
		}
	})
	t.Run("21 Mermaidファイルの範囲外と種類違い", func(t *testing.T) {
		for _, ref := range []Ref{codeRef(RefMermaid, 1), codeRef(RefPlantUML, 0)} {
			if _, err := CodeSource(r, []byte("graph TD\n"), mdfile.KindMermaid, ref); !errors.Is(err, ErrRefNotFound) {
				t.Errorf("CodeSource(%+v) のエラー = %v, want ErrRefNotFound", ref, err)
			}
		}
	})
	t.Run("22 不正なUTF-8", func(t *testing.T) {
		for _, bad := range invalidUTF8 {
			if _, err := CodeSource(r, []byte("@startuml\nA"+bad+"\n@enduml\n"), mdfile.KindPlantUML, codeRef(RefPlantUML, 0)); !errors.Is(err, ErrNotEditable) {
				t.Errorf("PlantUML のエラー = %v, want ErrNotEditable", err)
			}
			if _, err := CodeSource(r, []byte("graph TD\nA"+bad+"\n"), mdfile.KindMermaid, codeRef(RefMermaid, 0)); !errors.Is(err, ErrNotEditable) {
				t.Errorf("Mermaid のエラー = %v, want ErrNotEditable", err)
			}
		}
	})

	tests := []struct {
		name string
		src  string
		kind mdfile.Kind
		ref  Ref
		want string
	}{
		{"1 PlantUMLの図1", plantUMLFile, mdfile.KindPlantUML, codeRef(RefPlantUML, 1), diagram1},
		{"7 CRLFのPlantUML", strings.ReplaceAll(plantUMLFile, "\n", "\r\n"), mdfile.KindPlantUML, codeRef(RefPlantUML, 1), diagram1},
		{"9 BOMを含まない", bom + plantUMLFile, mdfile.KindPlantUML, codeRef(RefPlantUML, 0), "@startuml a\nA -> B\n@enduml"},
		{"10 @endの無い図はファイルの終わりまで", "@startuml\nA -> B\n", mdfile.KindPlantUML, codeRef(RefPlantUML, 0), "@startuml\nA -> B\n"},
		{"13 Mermaidファイルの全体", "graph TD\r\nA-->B\r\n", mdfile.KindMermaid, codeRef(RefMermaid, 0), "graph TD\nA-->B\n"},
		{"17 先頭の空行も含む", "\n\ngraph TD\n", mdfile.KindMermaid, codeRef(RefMermaid, 0), "\n\ngraph TD\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CodeSource(r, []byte(tt.src), tt.kind, tt.ref)
			if err != nil || got != tt.want {
				t.Errorf("CodeSource = %q, %v, want %q", got, err, tt.want)
			}
		})
	}
}

// TestPlanCode_DiagramFiles は図のファイルの書き換えを検証する
// （UT-119。根拠: FR-025, FR-026, FR-143, FR-145, MD-086, MD-087 / IMP-122, IMP-126）。
//
// **ケース 4 が、PlantUML ファイルで書き込む内容を確かめる理由そのものである**（FR-145）。
// @enduml を消すと 2 つの図が 1 つにつながり、図 1 の目印が消える。
func TestPlanCode_DiagramFiles(t *testing.T) {
	r := renderer.New()
	puml := func(src string, n int, text string) (Patch, bool, error) {
		return PlanCode(r, []byte(src), mdfile.KindPlantUML, codeRef(RefPlantUML, n), text)
	}
	mmd := func(src, text string) (Patch, bool, error) {
		return PlanCode(r, []byte(src), mdfile.KindMermaid, codeRef(RefMermaid, 0), text)
	}

	// 拒む・書かないケースを先に置く（UT-013）。
	t.Run("4 @endumlを消す", func(t *testing.T) {
		if _, _, err := puml(plantUMLFile, 0, "@startuml a\nA -> B"); !errors.Is(err, ErrRejected) {
			t.Errorf("エラー = %v, want ErrRejected（図の数が 1 つになる）", err)
		}
	})
	t.Run("5 図を足す", func(t *testing.T) {
		if _, _, err := puml(plantUMLFile, 0, "@startuml a\nA -> B\n@enduml\n@startuml z\nX -> Y\n@enduml"); !errors.Is(err, ErrRejected) {
			t.Errorf("エラー = %v, want ErrRejected（図の数が 3 つになる）", err)
		}
	})
	t.Run("6 同じ内容", func(t *testing.T) {
		if _, changed, err := puml(plantUMLFile, 1, diagram1); err != nil || changed {
			t.Errorf("changed %v, err %v, want 偽, nil", changed, err)
		}
	})
	t.Run("24 同じ内容の末尾に改行", func(t *testing.T) {
		if _, changed, err := puml(plantUMLFile, 1, diagram1+"\n"); err != nil || changed {
			t.Errorf("changed %v, err %v, want 偽, nil", changed, err)
		}
	})
	t.Run("11 範囲外と種類違い", func(t *testing.T) {
		for _, ref := range []Ref{codeRef(RefPlantUML, 2), codeRef(RefMermaid, 0), codeRef(RefCode, 0)} {
			if _, _, err := PlanCode(r, []byte(plantUMLFile), mdfile.KindPlantUML, ref, "x"); !errors.Is(err, ErrRefNotFound) {
				t.Errorf("PlanCode(%+v) のエラー = %v, want ErrRefNotFound", ref, err)
			}
		}
	})
	t.Run("15 Mermaidファイルの図が無くなる", func(t *testing.T) {
		for _, text := range []string{" \n\t\n", ""} {
			if _, _, err := mmd("graph TD\r\nA-->B\r\n", text); !errors.Is(err, ErrRejected) {
				t.Errorf("PlanCode(%q) のエラー = %v, want ErrRejected", text, err)
			}
		}
	})
	t.Run("16 Mermaidファイルの同じ内容", func(t *testing.T) {
		if _, changed, err := mmd("graph TD\r\nA-->B\r\n", "graph TD\nA-->B\n"); err != nil || changed {
			t.Errorf("changed %v, err %v, want 偽, nil", changed, err)
		}
	})
	t.Run("21 Mermaidファイルの範囲外と種類違い", func(t *testing.T) {
		for _, ref := range []Ref{codeRef(RefMermaid, 1), codeRef(RefPlantUML, 0)} {
			if _, _, err := PlanCode(r, []byte("graph TD\n"), mdfile.KindMermaid, ref, "graph LR"); !errors.Is(err, ErrRefNotFound) {
				t.Errorf("PlanCode(%+v) のエラー = %v, want ErrRefNotFound", ref, err)
			}
		}
	})
	t.Run("22 不正なUTF-8", func(t *testing.T) {
		for _, bad := range invalidUTF8 {
			if _, _, err := puml("@startuml\nA"+bad+"\n@enduml\n", 0, "@startuml\nB\n@enduml"); !errors.Is(err, ErrNotEditable) {
				t.Errorf("PlantUML のエラー = %v, want ErrNotEditable", err)
			}
			if _, _, err := mmd("graph TD\nA"+bad+"\n", "graph LR\n"); !errors.Is(err, ErrNotEditable) {
				t.Errorf("Mermaid のエラー = %v, want ErrNotEditable", err)
			}
		}
	})

	// 書き換えるケース。
	tests := []struct {
		name string
		do   func() (Patch, bool, error)
		want Patch
	}{
		{"2 図1を書き換える", func() (Patch, bool, error) { return puml(plantUMLFile, 1, "@startuml b\nC -> E\n@enduml") },
			Patch{28, []byte(diagram1), []byte("@startuml b\nC -> E\n@enduml")}},
		{"3 図の名前を直す", func() (Patch, bool, error) { return puml(plantUMLFile, 1, "@startuml c\nC -> D\n@enduml") },
			Patch{28, []byte(diagram1), []byte("@startuml c\nC -> D\n@enduml")}},
		{"7 CRLFのファイルにLFの入力", func() (Patch, bool, error) {
			return puml(strings.ReplaceAll(plantUMLFile, "\n", "\r\n"), 1, "@startuml b\nC -> E\n@enduml")
		}, Patch{32, []byte("@startuml b\r\nC -> D\r\n@enduml"), []byte("@startuml b\r\nC -> E\r\n@enduml")}},
		{"8 LFのファイルにCRLFの入力", func() (Patch, bool, error) { return puml(plantUMLFile, 1, "@startuml b\r\nC -> E\r\n@enduml") },
			Patch{28, []byte(diagram1), []byte("@startuml b\nC -> E\n@enduml")}},
		{"9 BOMの分だけ後ろ", func() (Patch, bool, error) { return puml(bom+plantUMLFile, 0, "@startuml a\nA -> C\n@enduml") },
			Patch{3, []byte("@startuml a\nA -> B\n@enduml"), []byte("@startuml a\nA -> C\n@enduml")}},
		{"14 Mermaidファイルの全体", func() (Patch, bool, error) { return mmd("graph TD\r\nA-->B\r\n", "graph LR\nA-->B\n") },
			Patch{0, []byte("graph TD\r\nA-->B\r\n"), []byte("graph LR\r\nA-->B\r\n")}},
		{"18 MermaidファイルのBOMの分だけ後ろ", func() (Patch, bool, error) { return mmd(bom+"graph TD\n", "graph LR\n") },
			Patch{3, []byte("graph TD\n"), []byte("graph LR\n")}},
		{"19 改行コードの混ざったファイル", func() (Patch, bool, error) { return mmd("a\r\nb\nc", "a\nB\nc") },
			Patch{0, []byte("a\r\nb\nc"), []byte("a\r\nB\r\nc")}},
		{"20 改行の無いファイル", func() (Patch, bool, error) { return mmd("graph TD", "graph LR\nA") },
			Patch{0, []byte("graph TD"), []byte("graph LR\nA")}},
		{"23 入力の末尾の改行をすべて除く", func() (Patch, bool, error) { return puml(plantUMLFile, 1, "@startuml b\nC -> E\n@enduml\n\n") },
			Patch{28, []byte(diagram1), []byte("@startuml b\nC -> E\n@enduml")}},
		{"25 @endの無い図は入力をそのまま", func() (Patch, bool, error) { return puml("@startuml\nA -> B\n", 0, "@startuml\nA -> C\n") },
			Patch{0, []byte("@startuml\nA -> B\n"), []byte("@startuml\nA -> C\n")}},
		{"26 @endumlを足し入力の末尾に改行", func() (Patch, bool, error) {
			return puml("@startuml\nA -> B\n", 0, "@startuml\nA -> B\n@enduml\n")
		}, Patch{0, []byte("@startuml\nA -> B\n"), []byte("@startuml\nA -> B\n@enduml\n")}},
		{"27 最後の図の@endumlを消す", func() (Patch, bool, error) { return puml(plantUMLFile, 1, "@startuml b\nC -> E") },
			Patch{28, []byte(diagram1), []byte("@startuml b\nC -> E")}},
		{"27 最後に改行の無いファイルで同じこと", func() (Patch, bool, error) {
			return puml(strings.TrimSuffix(plantUMLFile, "\n"), 1, "@startuml b\nC -> E")
		}, Patch{28, []byte(diagram1), []byte("@startuml b\nC -> E")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, changed, err := tt.do()
			wantPatch(t, p, changed, err, tt.want)
		})
	}

	// ケース 12: 2 の Patch を適用しても、図 0 と図の外の行（空行）は 1 バイトも変わらない。
	t.Run("12 図の外の行は変えない", func(t *testing.T) {
		p, _, err := puml(plantUMLFile, 1, "@startuml b\nC -> E\n@enduml")
		if err != nil {
			t.Fatalf("PlanCode がエラーを返した: %v", err)
		}
		after, err := p.Apply([]byte(plantUMLFile))
		if err != nil {
			t.Fatalf("Apply がエラーを返した: %v", err)
		}
		if !bytes.HasPrefix(after, []byte(plantUMLFile[:28])) || !bytes.HasSuffix(after, []byte(plantUMLFile[54:])) {
			t.Errorf("図 1 の外が変わった: %q", after)
		}
	})
}
