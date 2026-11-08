package document

import (
	"crypto/sha256"
	"errors"
	"os"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// codeDoc は UT-115 のケース 21・22・24 で使う、コードブロックと図を含む Markdown。
const codeDoc = "```go\nx\n```\n\n```mermaid\ngraph TD\n```\n\n```plantuml\n@startuml\nA -> B\n@enduml\n```\n\n段落\n\n- [ ] a\n\n| h1 | h2 |\n| --- | --- |\n| 1 | 2 |\n"

// startedFile は name のファイルを content で作って読み、編集モードを始めたセッションを返す。
func startedFile(t *testing.T, r *renderer.Renderer, name, content string, opts LoadOptions) (*EditSession, *Document, string) {
	t.Helper()

	p := writeFile(t, t.TempDir(), name, content)
	doc, err := Load(r, p, opts)
	if err != nil {
		t.Fatalf("前提: Load がエラーを返した: %v", err)
	}
	s := &EditSession{}
	if !s.Start(doc, true) {
		t.Fatal("前提: Start が偽")
	}
	return s, doc, p
}

// planCommit は IMP-195 の手順（読み込み → Plan → 書き込み → Commit）を行い、書き込んだ内容を返す。
func planCommit(t *testing.T, s *EditSession, r *renderer.Renderer, path string, kind mdfile.Kind, op Op) []byte {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("前提: 読めない: %v", err)
	}
	p, changed, _, err := s.Plan(r, raw, kind, op)
	if err != nil || !changed {
		t.Fatalf("前提: Plan = changed %v, err %v", changed, err)
	}
	after, err := p.Apply(raw)
	if err != nil {
		t.Fatalf("前提: Apply がエラーを返した: %v", err)
	}
	if err := os.WriteFile(path, after, 0o644); err != nil {
		t.Fatalf("前提: 書けない: %v", err)
	}
	s.Commit(op, p, after)
	return after
}

// TestEditSession_StartKinds は開けるファイルの種類ごとの開始の可否を検証する
// （UT-114 のケース 21。根拠: FR-027, FR-140, FR-145 / IMP-100, IMP-109）。
func TestEditSession_StartKinds(t *testing.T) {
	r := renderer.New()
	dir := t.TempDir()

	// 画像ファイルは編集モードを始められない（書き換える対象を持たない。FR-027）。
	t.Run("21 画像ファイル", func(t *testing.T) {
		doc, err := Load(r, writeBytes(t, dir, "a.png", []byte("not a png")), LoadOptions{})
		if err != nil {
			t.Fatalf("前提: Load がエラーを返した: %v", err)
		}
		var s EditSession
		if s.CanStart(doc, true) || s.Start(doc, true) {
			t.Error("画像ファイルで CanStart / Start が真")
		}
	})

	// PlantUML ファイル・Mermaid ファイルは始められる（図の書き換え。FR-145）。
	for _, f := range []struct{ name, content string }{
		{"a.puml", "@startuml\nA -> B\n@enduml\n"},
		{"a.mmd", "graph TD\nA-->B\n"},
	} {
		t.Run("21 "+f.name, func(t *testing.T) {
			doc, err := Load(r, writeFile(t, dir, f.name, f.content), LoadOptions{})
			if err != nil {
				t.Fatalf("前提: Load がエラーを返した: %v", err)
			}
			var s EditSession
			if !s.CanStart(doc, true) || !s.Start(doc, true) {
				t.Error("図のファイルで CanStart / Start が偽")
			}
		})
	}
}

// TestEditSession_CheckV120 は v1.2.0 の指示と目印の種類の対応を検証する
// （UT-115 のケース 21〜25。根拠: FR-145, FR-146, NFR-030 / IMP-109 の 2）。
func TestEditSession_CheckV120(t *testing.T) {
	r := renderer.New()

	// ケース 22・24・25（拒む側）を先に置く（UT-013）。
	t.Run("22 OpCodeに種類の違う目印", func(t *testing.T) {
		s, doc, _ := startedFile(t, r, "a.md", codeDoc, LoadOptions{TextRefs: true})
		for _, op := range []Op{
			{Kind: OpCode, Ref: doc.RefKey + ":task:0", Text: "y"},
			{Kind: OpCode, Ref: doc.RefKey + ":text:0", Text: "y"},
			{Kind: OpCell, Ref: doc.RefKey + ":code:0", Text: "y"},
		} {
			if err := s.Check(doc, true, op); !errors.Is(err, ErrStale) {
				t.Errorf("Check(%+v) のエラー = %v, want ErrStale", op, err)
			}
		}
	})

	t.Run("24 OpFormatに種類の違う目印", func(t *testing.T) {
		s, doc, _ := startedFile(t, r, "a.md", codeDoc, LoadOptions{TextRefs: true})
		for _, ref := range []string{":code:0", ":task:0"} {
			op := Op{Kind: OpFormat, Ref: doc.RefKey + ref, Format: FormatOp{Start: 0, End: 1, Visible: "段落", Style: FormatStyle{Strike: true}}}
			if err := s.Check(doc, true, op); !errors.Is(err, ErrStale) {
				t.Errorf("Check(%s) のエラー = %v, want ErrStale", ref, err)
			}
		}
	})

	// ケース 25: 修飾の目印を付けた Markdown の描画に対する指示だけを受け付ける。セルの目印でも TextRefs を見る。
	t.Run("25 TextRefsが偽の文書", func(t *testing.T) {
		s, doc, _ := startedFile(t, r, "a.md", codeDoc, LoadOptions{})
		for _, ref := range []string{":text:0", ":cell:0:1:0"} {
			op := Op{Kind: OpFormat, Ref: doc.RefKey + ref, Format: FormatOp{Start: 0, End: 1, Visible: "1", Style: FormatStyle{Strike: true}}}
			if err := s.Check(doc, true, op); !errors.Is(err, ErrStale) {
				t.Errorf("Check(%s) のエラー = %v, want ErrStale", ref, err)
			}
		}
	})
	t.Run("25 Kindが図のファイル", func(t *testing.T) {
		s, md, _ := startedFile(t, r, "a.md", codeDoc, LoadOptions{TextRefs: true})
		// Load した PlantUML ファイルは TextRefs が常に偽であり、Kind の確かめを見分けられない
		// ため、Document を直接組み立てる（4.78.0）。
		doc := &Document{Path: md.Path, Kind: mdfile.KindPlantUML, TextRefs: true, RefKey: md.RefKey, Digest: md.Digest}
		op := Op{Kind: OpFormat, Ref: doc.RefKey + ":text:0", Format: FormatOp{Start: 0, End: 1, Visible: "段落", Style: FormatStyle{Strike: true}}}
		if err := s.Check(doc, true, op); !errors.Is(err, ErrStale) {
			t.Errorf("Check のエラー = %v, want ErrStale", err)
		}
	})

	// ケース 21: OpCode に鍵の合う code: / mermaid: / plantuml: の目印。
	t.Run("21 OpCodeの目印", func(t *testing.T) {
		s, doc, _ := startedFile(t, r, "a.md", codeDoc, LoadOptions{})
		for _, ref := range []string{":code:0", ":mermaid:0", ":plantuml:0"} {
			if err := s.Check(doc, true, Op{Kind: OpCode, Ref: doc.RefKey + ref, Text: "y"}); err != nil {
				t.Errorf("Check(%s) のエラー = %v, want nil", ref, err)
			}
		}
	})

	// ケース 23: OpFormat に text: / cell: の目印（表のセルも修飾できる。FR-146）。
	t.Run("23 OpFormatの目印", func(t *testing.T) {
		s, doc, _ := startedFile(t, r, "a.md", codeDoc, LoadOptions{TextRefs: true})
		for _, ref := range []string{":text:0", ":cell:0:1:0"} {
			op := Op{Kind: OpFormat, Ref: doc.RefKey + ref, Format: FormatOp{Start: 0, End: 1, Visible: "段落", Style: FormatStyle{Strike: true}}}
			if err := s.Check(doc, true, op); err != nil {
				t.Errorf("Check(%s) のエラー = %v, want nil", ref, err)
			}
		}
	})
}

// TestEditSession_PlanV120 は v1.2.0 の書き込みの判断を検証する
// （UT-115 のケース 26〜31。根拠: FR-143, FR-144, FR-145, FR-146, FR-110 / IMP-109 の 5）。
//
// **ケース 26 と 27 は対で意味を持つ**——26 だけなら「どれも写さない」実装が、27 だけなら
// 「どれも ErrStale に写す」実装が通る（4.77.0）。黙って戻すと、確定した入力が通知も無く消える。
func TestEditSession_PlanV120(t *testing.T) {
	r := renderer.New()

	plan := func(t *testing.T, s *EditSession, path string, kind mdfile.Kind, op Op) (Patch, bool, bool, error) {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("前提: 読めない: %v", err)
		}
		return s.Plan(r, raw, kind, op)
	}

	// ケース 26: 拒んだ書き込みは ErrStale にしない。
	t.Run("26 前置きが行ごとに違うブロック", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "> ```\n> a\n>b\n> ```\n", LoadOptions{})
		_, _, _, err := plan(t, s, p, mdfile.KindMarkdown, Op{Kind: OpCode, Ref: doc.RefKey + ":code:0", Text: "x"})
		if !errors.Is(err, ErrLocked) {
			t.Errorf("Plan のエラー = %v, want ErrLocked", err)
		}
	})
	t.Run("26 PlantUMLファイルの@endumlを消す", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.puml", "@startuml a\nA -> B\n@enduml\n\n@startuml b\nC -> D\n@enduml\n", LoadOptions{})
		_, _, _, err := plan(t, s, p, mdfile.KindPlantUML, Op{Kind: OpCode, Ref: doc.RefKey + ":plantuml:0", Text: "@startuml a\nA -> B"})
		if !errors.Is(err, ErrRejected) {
			t.Errorf("Plan のエラー = %v, want ErrRejected", err)
		}
	})
	t.Run("26 Mermaidファイルを空白だけにする", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.mmd", "graph TD\r\nA-->B\r\n", LoadOptions{})
		_, _, _, err := plan(t, s, p, mdfile.KindMermaid, Op{Kind: OpCode, Ref: doc.RefKey + ":mermaid:0", Text: " \n\t\n"})
		if !errors.Is(err, ErrRejected) {
			t.Errorf("Plan のエラー = %v, want ErrRejected", err)
		}
	})
	t.Run("26 分けられない修飾", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "<del>あ**いう**え</del>\n", LoadOptions{TextRefs: true})
		op := Op{Kind: OpFormat, Ref: doc.RefKey + ":text:0", Format: FormatOp{Start: 2, End: 3, Visible: "あいうえ", Style: FormatStyle{Strike: true}}}
		if _, _, _, err := plan(t, s, p, mdfile.KindMarkdown, op); !errors.Is(err, ErrCannotFormat) {
			t.Errorf("Plan のエラー = %v, want ErrCannotFormat", err)
		}
	})

	// ケース 27: 見える文字の食い違い・Style の形の違いは古い指示（ErrStale。通知しない）。
	t.Run("27 見える文字が1文字違う", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "今日は会議です\n", LoadOptions{TextRefs: true})
		op := Op{Kind: OpFormat, Ref: doc.RefKey + ":text:0", Format: FormatOp{Start: 3, End: 5, Visible: "今日は会議でした", Style: FormatStyle{Strike: true}}}
		if _, _, _, err := plan(t, s, p, mdfile.KindMarkdown, op); !errors.Is(err, ErrStale) {
			t.Errorf("Plan のエラー = %v, want ErrStale", err)
		}
	})
	t.Run("27 Styleの形が違う", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "今日は会議です\n", LoadOptions{TextRefs: true})
		op := Op{Kind: OpFormat, Ref: doc.RefKey + ":text:0", Format: FormatOp{Start: 3, End: 5, Visible: "今日は会議です", Style: FormatStyle{Strike: true, Color: "red"}}}
		if _, _, _, err := plan(t, s, p, mdfile.KindMarkdown, op); !errors.Is(err, ErrStale) {
			t.Errorf("Plan のエラー = %v, want ErrStale", err)
		}
	})

	// ケース 28: 種類の無い目印・範囲を超える番号は ErrStale。
	t.Run("28 PlantUMLファイルにmermaidの目印", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.puml", "@startuml\nA -> B\n@enduml\n", LoadOptions{})
		if _, _, _, err := plan(t, s, p, mdfile.KindPlantUML, Op{Kind: OpCode, Ref: doc.RefKey + ":mermaid:0", Text: "graph TD"}); !errors.Is(err, ErrStale) {
			t.Errorf("Plan のエラー = %v, want ErrStale", err)
		}
	})
	t.Run("28 範囲を超えるcode", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", codeDoc, LoadOptions{})
		if _, _, _, err := plan(t, s, p, mdfile.KindMarkdown, Op{Kind: OpCode, Ref: doc.RefKey + ":code:5", Text: "y"}); !errors.Is(err, ErrStale) {
			t.Errorf("Plan のエラー = %v, want ErrStale", err)
		}
	})

	// ケース 29: 手書きの取り消し線の一部を外すと、全体が外れ widened が真。
	t.Run("29 手書きの取り消し線", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "~~あいう~~\n", LoadOptions{TextRefs: true})
		op := Op{Kind: OpFormat, Ref: doc.RefKey + ":text:0", Format: FormatOp{Start: 1, End: 2, Visible: "あいう", Style: FormatStyle{Strike: true}}}
		_, changed, widened, err := plan(t, s, p, mdfile.KindMarkdown, op)
		if err != nil || !changed || !widened {
			t.Errorf("Plan = changed %v, widened %v, err %v, want true, true, nil", changed, widened, err)
		}
	})

	// ケース 30: 1 回の操作が 1 回の取り消し（FR-144, FR-146）。
	t.Run("30 OpCodeの取り消し", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "```go\na\n```\n", LoadOptions{})
		before := readRaw(t, p)
		after := planCommit(t, s, r, p, mdfile.KindMarkdown, Op{Kind: OpCode, Ref: doc.RefKey + ":code:0", Text: "b"})
		undo, changed, _, err := s.Plan(r, after, mdfile.KindMarkdown, Op{Kind: OpUndo})
		if err != nil || !changed {
			t.Fatalf("取り消しの Plan = changed %v, err %v", changed, err)
		}
		if got, err := undo.Apply(after); err != nil || string(got) != string(before) {
			t.Errorf("取り消した結果 = %q, %v, want %q", got, err, before)
		}
	})
	t.Run("30 OpFormatの取り消し", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "今日は会議です\n", LoadOptions{TextRefs: true})
		before := readRaw(t, p)
		op := Op{Kind: OpFormat, Ref: doc.RefKey + ":text:0", Format: FormatOp{Start: 3, End: 5, Visible: "今日は会議です", Style: FormatStyle{Strike: true}}}
		after := planCommit(t, s, r, p, mdfile.KindMarkdown, op)
		undo, changed, _, err := s.Plan(r, after, mdfile.KindMarkdown, Op{Kind: OpUndo})
		if err != nil || !changed {
			t.Fatalf("取り消しの Plan = changed %v, err %v", changed, err)
		}
		if got, err := undo.Apply(after); err != nil || string(got) != string(before) {
			t.Errorf("取り消した結果 = %q, %v, want %q", got, err, before)
		}
	})

	// ケース 31: CodeSource の確かめ。ErrChanged / ErrLocked（ErrStale にしない）/ ErrStale。
	t.Run("31 CodeSourceでrawが違う", func(t *testing.T) {
		s, doc, _ := startedFile(t, r, "a.md", "```go\na\n```\n", LoadOptions{})
		if _, err := s.CodeSource(r, []byte("```go\nb\n```\n"), mdfile.KindMarkdown, doc.RefKey+":code:0"); !errors.Is(err, ErrChanged) {
			t.Errorf("CodeSource のエラー = %v, want ErrChanged", err)
		}
	})
	t.Run("31 CodeSourceで前置きが行ごとに違う", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "> ```\n> a\n>b\n> ```\n", LoadOptions{})
		if _, err := s.CodeSource(r, readRaw(t, p), mdfile.KindMarkdown, doc.RefKey+":code:0"); !errors.Is(err, ErrLocked) {
			t.Errorf("CodeSource のエラー = %v, want ErrLocked", err)
		}
	})
	t.Run("31 CodeSourceで範囲を超える番号", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "```go\na\n```\n", LoadOptions{})
		if _, err := s.CodeSource(r, readRaw(t, p), mdfile.KindMarkdown, doc.RefKey+":code:5"); !errors.Is(err, ErrStale) {
			t.Errorf("CodeSource のエラー = %v, want ErrStale", err)
		}
	})
	// 31 の対: 正常な CodeSource は中身を返す（ErrChanged の確かめが通ることも見る）。
	t.Run("31 の対 CodeSourceで存在するブロック", func(t *testing.T) {
		s, doc, p := startedFile(t, r, "a.md", "```go\na\n```\n", LoadOptions{})
		raw := readRaw(t, p)
		if sha256.Sum256(raw) != doc.Digest {
			t.Fatal("前提: 読み直した内容が Load と違う")
		}
		if got, err := s.CodeSource(r, raw, mdfile.KindMarkdown, doc.RefKey+":code:0"); err != nil || got != "a" {
			t.Errorf("CodeSource = %q, %v, want %q", got, err, "a")
		}
	})
}
