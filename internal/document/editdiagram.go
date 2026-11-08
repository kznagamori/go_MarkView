package document

import (
	"bytes"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// editdiagram.go は図のファイル（PlantUML ファイル・Mermaid ファイル）の書き換え（IMP-122 の
// 「PlantUML ファイルの図」「Mermaid ファイル」）を持つ。入口の CodeSource / PlanCode と Markdown の中の
// ブロックは editcode.go にある。
// findDiagram は PlantUML ファイルの ref の図を求める（IMP-122 の「PlantUML ファイルの図」、IMP-126）。
// 切り分けは正規化後のテキストで行い、位置は m で生バイト列へ戻す。
func findDiagram(raw []byte, ref Ref) (text []byte, m rawMap, d renderer.Diagram, err error) {
	text, m, err = mapRaw(raw)
	if err != nil {
		return nil, rawMap{}, renderer.Diagram{}, err
	}
	diagrams := renderer.SplitPlantUML(text)
	if ref.Kind != RefPlantUML || ref.Index < 0 || ref.Index >= len(diagrams) {
		return nil, rawMap{}, renderer.Diagram{}, ErrRefNotFound
	}
	return text, m, diagrams[ref.Index], nil
}

// planDiagram は PlantUML ファイルの図の Patch を組み立てる。
//
//   - `@start…` と `@end…` の行も書き換えられる（図の名前を直せるように。FR-145）。前置きは無い
//   - **図の範囲が `@end…` の行の改行の手前で終わる（Closed）なら、入力の末尾の改行をすべて除く**
//     （図の外に空行を足さない。4.77.0。UT-119 のケース 23・24）。`@end…` の無い図は入力をそのまま書く
//   - 入力の改行は、ファイルの最初の改行コード（無ければ LF）にそろえる
//   - **確かめ**: 分け直して図の数が変わらず、書き換えた図の原文が書いた内容と一致し（両方の末尾の
//     改行を除いて比べる。4.78.0。UT-119 のケース 26・27）、他の図の原文が変わらない。違えば ErrRejected
//     （`@enduml` を消して 2 つの図がつながる、などを拒む。UT-119 のケース 4・5）
func planDiagram(raw []byte, ref Ref, input string) (Patch, bool, error) {
	text, m, d, err := findDiagram(raw, ref)
	if err != nil {
		return Patch{}, false, err
	}

	content := normalizeInput(input)
	if d.Closed {
		content = strings.TrimRight(content, "\n")
	}
	if content == string(text[d.Span.Start:d.Span.Stop]) {
		return Patch{}, false, nil
	}

	start, stop := m.toRaw(d.Span.Start), m.toRaw(d.Span.Stop)
	p := Patch{Offset: start, Old: bytes.Clone(raw[start:stop]), New: []byte(withNewlines(content, fileNewline(raw)))}

	if !keepsDiagrams(text, raw, ref, p, content) {
		return Patch{}, false, ErrRejected
	}
	return p, true, nil
}

// keepsDiagrams は planDiagram の確かめ（IMP-122）。
func keepsDiagrams(text, raw []byte, ref Ref, p Patch, content string) bool {
	after, err := p.Apply(raw)
	if err != nil {
		return false
	}
	afterText, _, err := mapRaw(after)
	if err != nil {
		return false
	}
	before, got := renderer.SplitPlantUML(text), renderer.SplitPlantUML(afterText)
	if len(before) != len(got) {
		return false
	}
	for i := range before {
		b := string(text[before[i].Span.Start:before[i].Span.Stop])
		g := string(afterText[got[i].Span.Start:got[i].Span.Stop])
		if i == ref.Index {
			b, g = strings.TrimRight(content, "\n"), strings.TrimRight(g, "\n")
		}
		if b != g {
			return false
		}
	}
	return true
}

// mermaidFile は Mermaid ファイルの、書き換える範囲（ファイル全体。先頭の BOM を除く）を返す
// （IMP-122 の「Mermaid ファイル」）。目印は mermaid:0 だけ。
func mermaidFile(raw []byte, ref Ref) (text []byte, m rawMap, err error) {
	text, m, err = mapRaw(raw)
	if err != nil {
		return nil, rawMap{}, err
	}
	if ref.Kind != RefMermaid || ref.Index != 0 {
		return nil, rawMap{}, ErrRefNotFound
	}
	return text, m, nil
}

// planMermaidFile は Mermaid ファイルの Patch を組み立てる。
//
//   - 書き換える範囲はファイル全体（先頭の BOM を除く）。入力の改行はファイルの最初の改行コード
//     （無ければ LF）にそろえる
//   - **確かめ**: 入力が空白だけなら ErrRejected（図が無くなる。MD-087 の 3）。空白だけかの判定は描画
//     （renderer の RenderMermaidFile が NoDiagrams にする規則。各行が半角空白とタブだけ）と同じにする。
//     **Mermaid の構文は解かない**（AR-031）
func planMermaidFile(raw []byte, ref Ref, input string) (Patch, bool, error) {
	text, m, err := mermaidFile(raw, ref)
	if err != nil {
		return Patch{}, false, err
	}

	content := normalizeInput(input)
	if strings.Trim(content, " \t\n") == "" {
		return Patch{}, false, ErrRejected
	}
	if content == string(text) {
		return Patch{}, false, nil
	}

	start := m.toRaw(0)
	return Patch{Offset: start, Old: bytes.Clone(raw[start:]), New: []byte(withNewlines(content, fileNewline(raw)))}, true, nil
}

// fileNewline はファイルの最初の改行コードを返す。無ければ LF（IMP-122）。
func fileNewline(raw []byte) string {
	if eol := lineEnding(raw, 0); eol != "" {
		return eol
	}
	return "\n"
}

// withNewlines は LF にそろえた s の改行を eol にする。
func withNewlines(s, eol string) string {
	if eol == "\n" {
		return s
	}
	return strings.ReplaceAll(s, "\n", eol)
}
