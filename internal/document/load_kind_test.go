package document

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kznagamori/go_MarkView/internal/localurl"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// chtimes は path の更新時刻を Unix 時刻の sec 秒にする。
// 秒の単位で与える——NTFS は 100 ナノ秒単位でしか持たず、端数のあるナノ秒は丸められる（UT-210 のケース 8）。
func chtimes(t *testing.T, path string, sec int64) {
	t.Helper()

	mt := time.Unix(sec, 0)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatalf("更新時刻を変えられない: %v", err)
	}
}

// imgSrcPattern は出力の img の src を取り出す。
var imgSrcPattern = regexp.MustCompile(`<img[^>]* src="([^"]*)"`)

// imgSrc は doc.HTML のただ 1 つの img の src を返す。
func imgSrc(t *testing.T, doc *Document) string {
	t.Helper()

	m := imgSrcPattern.FindAllStringSubmatch(doc.HTML, -1)
	if len(m) != 1 {
		t.Fatalf("img が %d 個, want 1\nHTML: %s", len(m), doc.HTML)
	}
	return strings.ReplaceAll(m[0][1], "&amp;", "&")
}

// TestLoad_Image は画像ファイルの読み込みを検証する
// （UT-117 のケース 1〜10。根拠: FR-014, FR-016, FR-027, FR-140 / IMP-100, IMP-102, IMP-118, IMP-128。BUG-026）。
//
// **ケース 7〜9 が、画像の Digest を版から作る理由そのものである**（IMP-102）。Digest を零値のままに
// すると、外部で画像を書き換えても再描画されない（7 で落ちる）。中身の要約にすると、50 MB の画像を
// 読むことになる（9 で落ちる）。**ケース 5 は呼び出し側を見る**（Load が版を渡し忘れると、画像を
// 書き換えて F5 を押しても古い画像が出続ける）。拒むケースを先に書く（UT-013）。
func TestLoad_Image(t *testing.T) {
	r := renderer.New()

	// ケース 1: 画像も Markdown と同じ閾値（FR-016, FR-027）。中身は書かず大きさだけを与える。
	t.Run("1 10MiBを超える画像", func(t *testing.T) {
		p := truncateFile(t, t.TempDir(), "a.png", ConfirmThreshold+1)
		_, err := Load(r, p, LoadOptions{})
		var se *SizeError
		if !errors.Is(err, ErrNeedsConfirm) || !errors.As(err, &se) {
			t.Fatalf("エラー = %v, want ErrNeedsConfirm（*SizeError）", err)
		}
		if se.Size != ConfirmThreshold+1 || se.Limit != ConfirmThreshold {
			t.Errorf("SizeError の Size / Limit = %d / %d, want %d / %d", se.Size, se.Limit, ConfirmThreshold+1, ConfirmThreshold)
		}
	})

	// ケース 2: 1 と同じで Confirmed が真なら読み込める。
	t.Run("2 Confirmedなら読み込める", func(t *testing.T) {
		p := truncateFile(t, t.TempDir(), "a.png", ConfirmThreshold+1)
		doc := mustLoad(t, r, p, LoadOptions{Confirmed: true})
		if doc.Kind != mdfile.KindImage {
			t.Errorf("Kind = %q, want %q", doc.Kind, mdfile.KindImage)
		}
	})

	// ケース 3: 50 MiB を超える画像は Confirmed でも ErrTooLarge。
	t.Run("3 50MiBを超える画像", func(t *testing.T) {
		p := truncateFile(t, t.TempDir(), "a.png", MaxSize+1)
		if _, err := Load(r, p, LoadOptions{Confirmed: true}); !errors.Is(err, ErrTooLarge) {
			t.Errorf("エラー = %v, want ErrTooLarge", err)
		}
	})

	// ケース 4・5: 中身が画像として壊れていても読み込める（中身を読まない）。
	dir := t.TempDir()
	broken := append([]byte("not a png"), 0xFF) // 10 バイト
	img := writeBytes(t, dir, "a.png", broken)
	chtimes(t, img, 1)

	t.Run("4 壊れた画像", func(t *testing.T) {
		doc := mustLoad(t, r, img, LoadOptions{})
		if doc.Kind != mdfile.KindImage {
			t.Errorf("Kind = %q, want %q", doc.Kind, mdfile.KindImage)
		}
		if len(doc.Warnings) != 0 {
			t.Errorf("Warnings = %+v, want 空（壊れているかは画面が決める）", doc.Warnings)
		}
	})

	t.Run("5 画像のDocument", func(t *testing.T) {
		doc := mustLoad(t, r, img, LoadOptions{})
		if !strings.Contains(doc.HTML, `class="image-file"`) {
			t.Errorf("HTML に div.image-file が無い\nHTML: %s", doc.HTML)
		}
		src := imgSrc(t, doc)
		path, query, _ := strings.Cut(src, "?")
		got, ok := localurl.Decode(path)
		if !ok || filepath.Clean(got) != filepath.Clean(img) {
			t.Errorf("src が指すパス = %q（%v）, want %q", got, ok, img)
		}
		// 版: 更新時刻 1 秒（0x3b9aca00 ナノ秒）と大きさ 10 バイト。期待値は人が書いた（UT-031）。
		if query != "v=3b9aca00-a" {
			t.Errorf("src のクエリ = %q, want %q", query, "v=3b9aca00-a")
		}
		if !strings.Contains(doc.HTML, `alt="a.png"`) {
			t.Errorf("alt が a.png でない\nHTML: %s", doc.HTML)
		}
		if doc.RefKey != "" || doc.Editable() || doc.LineCount != 0 || doc.Size != 10 {
			t.Errorf("RefKey / Editable / LineCount / Size = %q / %v / %d / %d, want 空 / false / 0 / 10", doc.RefKey, doc.Editable(), doc.LineCount, doc.Size)
		}
		if len(doc.Headings) != 0 || doc.NeedsMermaid || doc.NeedsKaTeX || doc.NeedsPlantUML {
			t.Errorf("Headings / Needs* = %v / %v %v %v, want 空 / すべて false", doc.Headings, doc.NeedsMermaid, doc.NeedsKaTeX, doc.NeedsPlantUML)
		}
	})

	// ケース 6: 何も変えずに 2 回 Load すると Digest と HTML が同じ（版が同じなら URL も同じ）。
	first := mustLoad(t, r, img, LoadOptions{})
	t.Run("6 何も変えずに2回", func(t *testing.T) {
		second := mustLoad(t, r, img, LoadOptions{})
		if first.Digest != second.Digest || first.HTML != second.HTML {
			t.Errorf("2 回の Load で Digest か HTML が違う")
		}
	})

	// ケース 7: 更新時刻だけを変えると Digest も版も違う（外部の書き換えで再描画を送る）。
	t.Run("7 更新時刻だけ変える", func(t *testing.T) {
		chtimes(t, img, 2)
		defer chtimes(t, img, 1)
		doc := mustLoad(t, r, img, LoadOptions{})
		if doc.Digest == first.Digest {
			t.Error("更新時刻が変わったのに Digest が同じ")
		}
		if imgSrc(t, doc) == imgSrc(t, first) {
			t.Error("更新時刻が変わったのに src の版が同じ")
		}
	})

	// ケース 8: 大きさの違う内容に書き換え、更新時刻を戻すと Digest が違う。
	t.Run("8 大きさが違う", func(t *testing.T) {
		writeBytes(t, dir, "a.png", append(broken, 'x'))
		chtimes(t, img, 1)
		defer func() { writeBytes(t, dir, "a.png", broken); chtimes(t, img, 1) }()
		if doc := mustLoad(t, r, img, LoadOptions{}); doc.Digest == first.Digest {
			t.Error("大きさが変わったのに Digest が同じ")
		}
	})

	// ケース 9: 同じ大きさの別の内容に書き換え、更新時刻を戻すと Digest が同じ（中身を読まない）。
	t.Run("9 同じ大きさの別の内容", func(t *testing.T) {
		writeBytes(t, dir, "a.png", []byte("0123456789"))
		chtimes(t, img, 1)
		defer func() { writeBytes(t, dir, "a.png", broken); chtimes(t, img, 1) }()
		if doc := mustLoad(t, r, img, LoadOptions{}); doc.Digest != first.Digest {
			t.Error("版が同じなのに Digest が違う（版が同じなら同じ内容とみなす。IMP-102）")
		}
	})

	// ケース 10: 画像には鍵を作らず、渡されても使わない（FR-027）。
	t.Run("10 鍵を渡しても使わない", func(t *testing.T) {
		doc := mustLoad(t, r, img, LoadOptions{RefKey: testKey, ExpectDigest: first.Digest})
		if doc.RefKey != "" {
			t.Errorf("RefKey = %q, want 空", doc.RefKey)
		}
	})
}

// TestLoad_Diagrams は図のファイルの読み込みを検証する
// （UT-117 のケース 11〜14。根拠: FR-025, FR-026, FR-021, FR-140 / IMP-100, IMP-102）。
func TestLoad_Diagrams(t *testing.T) {
	r := renderer.New()
	dir := t.TempDir()

	// ケース 11: 図が 2 つの a.puml。
	t.Run("11 図が2つのPlantUML", func(t *testing.T) {
		p := writeFile(t, dir, "a.puml", "@startuml a\nA -> B\n@enduml\n\n@startuml b\nC -> D\n@enduml\n")
		doc := mustLoad(t, r, p, LoadOptions{})
		if doc.Kind != mdfile.KindPlantUML {
			t.Errorf("Kind = %q, want %q", doc.Kind, mdfile.KindPlantUML)
		}
		if n := strings.Count(doc.HTML, `data-plantuml="1"`); n != 2 {
			t.Errorf("data-plantuml のブロックが %d 個, want 2\nHTML: %s", n, doc.HTML)
		}
		if len(doc.Headings) != 2 || doc.Headings[0].Level != 2 || doc.Headings[1].Level != 2 {
			t.Errorf("Headings = %+v, want Level 2 が 2 つ", doc.Headings)
		}
		if !refKeyPattern.MatchString(doc.RefKey) || !doc.Editable() || doc.NoDiagrams {
			t.Errorf("RefKey / Editable / NoDiagrams = %q / %v / %v, want 16 進 16 文字 / true / false", doc.RefKey, doc.Editable(), doc.NoDiagrams)
		}
	})

	// ケース 12: 図の無い b.puml / 空白だけの c.mmd は NoDiagrams（renderer の結果を写す）。
	for _, f := range []struct{ name, content string }{
		{"b.puml", "Alice -> Bob\n"},
		{"c.mmd", " \n\t\n"},
	} {
		t.Run("12 図の無い"+f.name, func(t *testing.T) {
			doc := mustLoad(t, r, writeFile(t, dir, f.name, f.content), LoadOptions{})
			if !doc.NoDiagrams {
				t.Error("NoDiagrams が偽")
			}
		})
	}

	// ケース 13: 図が 1 つの a.mmd。
	t.Run("13 Mermaidファイル", func(t *testing.T) {
		doc := mustLoad(t, r, writeFile(t, dir, "a.mmd", "graph TD\nA-->B\n"), LoadOptions{})
		if doc.Kind != mdfile.KindMermaid {
			t.Errorf("Kind = %q, want %q", doc.Kind, mdfile.KindMermaid)
		}
		if n := strings.Count(doc.HTML, `data-mermaid="1"`); n != 1 {
			t.Errorf("data-mermaid のブロックが %d 個, want 1\nHTML: %s", n, doc.HTML)
		}
		if len(doc.Headings) != 0 || !doc.Editable() {
			t.Errorf("Headings / Editable = %v / %v, want 空 / true", doc.Headings, doc.Editable())
		}
	})

	// ケース 14: 不正なバイト列を含む a.puml は編集できない（Markdown と同じ。FR-021, FR-025）。
	t.Run("14 不正なバイト列のPlantUML", func(t *testing.T) {
		p := writeBytes(t, dir, "bad.puml", []byte("@startuml\nA -> \xff\n@enduml\n"))
		doc := mustLoad(t, r, p, LoadOptions{})
		if doc.Editable() {
			t.Error("Editable が真")
		}
		found := false
		for _, w := range doc.Warnings {
			found = found || w.Kind == WarnInvalidEncoding
		}
		if !found {
			t.Errorf("Warnings = %+v, want WarnInvalidEncoding を含む", doc.Warnings)
		}
	})
}

// TestLoad_TextRefs は、Load が TextRefs を Render へ渡し、Document に写すことを検証する
// （UT-116 のケース 12〜14。根拠: FR-146, FR-140 / IMP-100, IMP-102, IMP-120）。
//
// **ケース 12 も呼び出し側を見る**（4.77.0）。Render に TextRefs を直接渡すテスト（UT-217）だけでは、
// Load が渡し忘れても緑になる。渡し忘れると、修飾のボタンとメニューが淡色のままになる（IMP-265）。
func TestLoad_TextRefs(t *testing.T) {
	r := renderer.New()
	dir := t.TempDir()
	const md = "段落\n\n- [ ] タスク\n\n| a |\n| --- |\n| 1 |\n"
	p := writeFile(t, dir, "a.md", md)

	// ケース 12: TextRefs を真にして Load すると text: の目印が付き、doc.TextRefs が真。
	t.Run("12 TextRefsが真", func(t *testing.T) {
		doc := mustLoad(t, r, p, LoadOptions{TextRefs: true})
		if want := `data-ref="` + doc.RefKey + `:text:0"`; !strings.Contains(doc.HTML, want) {
			t.Errorf("HTML に %s が無い\nHTML: %s", want, doc.HTML)
		}
		if !doc.TextRefs {
			t.Error("doc.TextRefs が偽")
		}
	})

	// ケース 13: TextRefs を偽にすると text: は無く、タスクと表の目印はある。
	t.Run("13 TextRefsが偽", func(t *testing.T) {
		doc := mustLoad(t, r, p, LoadOptions{})
		if strings.Contains(doc.HTML, ":text:") || doc.TextRefs {
			t.Errorf("text: の目印がある・doc.TextRefs が真（%v）\nHTML: %s", doc.TextRefs, doc.HTML)
		}
		for _, kind := range []string{":task:0", ":table:0"} {
			if !strings.Contains(doc.HTML, doc.RefKey+kind) {
				t.Errorf("HTML に %s の目印が無い\nHTML: %s", kind, doc.HTML)
			}
		}
	})

	// ケース 14: Markdown 以外は修飾の対象外。TextRefs を真にしても目印は無く、doc.TextRefs は偽。
	for _, f := range []struct{ name, content string }{
		{"a.puml", "@startuml\nA -> B\n@enduml\n"},
		{"a.mmd", "graph TD\nA-->B\n"},
	} {
		t.Run("14 "+f.name, func(t *testing.T) {
			doc := mustLoad(t, r, writeFile(t, dir, f.name, f.content), LoadOptions{TextRefs: true})
			if strings.Contains(doc.HTML, ":text:") || doc.TextRefs {
				t.Errorf("text: の目印がある・doc.TextRefs が真（%v）\nHTML: %s", doc.TextRefs, doc.HTML)
			}
		})
	}
}
