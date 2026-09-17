package document

import (
	"errors"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// judgeDoc は UT-115 の多くのケースで使う文書。
// タスク 0 はオフ、タスク 1 はオン。表 0 の本体 2 行目はセルが 1 つしかない（2 列目が補われる）。
const judgeDoc = "- [ ] a\n- [x] b\n\n| h1 | h2 |\n| --- | --- |\n| 1 | 2 |\n| x |\n"

// otherKey は doc.RefKey と違う鍵を返す（UT-115 ケース 3）。
func otherKey(doc *Document) string {
	if doc.RefKey == "fedcba9876543210" {
		return "0123456789abcdef"
	}
	return "fedcba9876543210"
}

// startedSession は judgeDoc を読み、編集モードを始めたセッションを返す。
func startedSession(t *testing.T, r *renderer.Renderer, content string) (*EditSession, *Document, string) {
	t.Helper()

	doc, path := loadDoc(t, r, t.TempDir(), "a.md", content)
	s := &EditSession{}
	if !s.Start(doc, true) {
		t.Fatal("前提: Start が偽")
	}
	return s, doc, path
}

// TestEditSession_Check は指示の有効性の確かめを検証する
// （UT-115 ケース 1〜6・19。根拠: FR-141, FR-142, FR-143, NFR-030 / IMP-109）。
//
// **拒むケースを先に書く**（UT-013）。
func TestEditSession_Check(t *testing.T) {
	r := renderer.New()

	t.Run("1: 編集モードでない", func(t *testing.T) {
		doc, _ := loadDoc(t, r, t.TempDir(), "a.md", judgeDoc)
		var s EditSession
		if err := s.Check(doc, true, taskOp(doc.RefKey, 0, true)); !errors.Is(err, ErrStale) {
			t.Errorf("Check のエラー = %v, want ErrStale", err)
		}
	})

	t.Run("2: showing が偽", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		if err := s.Check(doc, false, taskOp(doc.RefKey, 0, true)); !errors.Is(err, ErrStale) {
			t.Errorf("Check のエラー = %v, want ErrStale", err)
		}
	})

	// ケース 3: 生 HTML で偽装した目印も、ここで止まる（NFR-030）
	t.Run("3: 鍵が doc.RefKey と違う目印", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		if err := s.Check(doc, true, taskOp(otherKey(doc), 0, true)); !errors.Is(err, ErrStale) {
			t.Errorf("Check のエラー = %v, want ErrStale", err)
		}
	})

	t.Run("4: 形の崩れた目印", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		if err := s.Check(doc, true, Op{Kind: OpTask, Ref: "bad", Checked: true}); !errors.Is(err, ErrStale) {
			t.Errorf("Check のエラー = %v, want ErrStale", err)
		}
	})

	// ケース 19: 別の要素を書き換えない（IMP-109 の 2）
	t.Run("19: OpTask にセルの目印", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		op := Op{Kind: OpTask, Ref: doc.RefKey + ":cell:0:1:0", Checked: true}
		if err := s.Check(doc, true, op); !errors.Is(err, ErrStale) {
			t.Errorf("Check のエラー = %v, want ErrStale", err)
		}
	})

	t.Run("19: OpCell にタスクの目印", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		op := Op{Kind: OpCell, Ref: doc.RefKey + ":task:0", Text: "9"}
		if err := s.Check(doc, true, op); !errors.Is(err, ErrStale) {
			t.Errorf("Check のエラー = %v, want ErrStale", err)
		}
	})

	t.Run("5: 鍵の合う目印", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		if err := s.Check(doc, true, taskOp(doc.RefKey, 0, true)); err != nil {
			t.Errorf("Check のエラー = %v, want nil", err)
		}
		if err := s.Check(doc, true, cellOp(doc.RefKey, 0, 1, 0, "9")); err != nil {
			t.Errorf("セルの Check のエラー = %v, want nil", err)
		}
	})

	t.Run("6: 取り消し（目印を持たない）", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		if err := s.Check(doc, true, Op{Kind: OpUndo}); err != nil {
			t.Errorf("Check のエラー = %v, want nil", err)
		}
	})
}

// TestEditSession_Plan は書き込みの判断を検証する
// （UT-115 ケース 7〜12・14〜18。根拠: FR-141, FR-142, FR-143, FR-144 / IMP-109）。
func TestEditSession_Plan(t *testing.T) {
	r := renderer.New()

	t.Run("7: raw が把握している内容と違う", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		p, _, err := s.Plan(r, []byte("- [ ] changed outside\n"), taskOp(doc.RefKey, 0, true))
		if !errors.Is(err, ErrChanged) {
			t.Errorf("Plan のエラー = %v, want ErrChanged", err)
		}
		if p.Old != nil || p.New != nil {
			t.Errorf("Patch を返した: %+v", p)
		}
	})

	t.Run("8: 番号がタスクの数を超える", func(t *testing.T) {
		s, doc, path := startedSession(t, r, judgeDoc)
		if _, _, err := s.Plan(r, readRaw(t, path), taskOp(doc.RefKey, 2, true)); !errors.Is(err, ErrStale) {
			t.Errorf("Plan のエラー = %v, want ErrStale", err)
		}
	})

	t.Run("8: 補われたセル", func(t *testing.T) {
		s, doc, path := startedSession(t, r, judgeDoc)
		if _, _, err := s.Plan(r, readRaw(t, path), cellOp(doc.RefKey, 0, 2, 1, "y")); !errors.Is(err, ErrStale) {
			t.Errorf("Plan のエラー = %v, want ErrStale", err)
		}
	})

	t.Run("9: 既にその状態のチェックボックス", func(t *testing.T) {
		s, doc, path := startedSession(t, r, judgeDoc)
		_, changed, err := s.Plan(r, readRaw(t, path), taskOp(doc.RefKey, 1, true))
		if err != nil {
			t.Fatalf("Plan のエラー = %v, want nil", err)
		}
		if changed {
			t.Error("changed が真")
		}
	})

	t.Run("10: 履歴が空の取り消し", func(t *testing.T) {
		s, _, path := startedSession(t, r, judgeDoc)
		_, changed, err := s.Plan(r, readRaw(t, path), Op{Kind: OpUndo})
		if err != nil {
			t.Fatalf("Plan のエラー = %v, want nil", err)
		}
		if changed {
			t.Error("changed が真")
		}
	})

	// ケース 11: 把握している内容が進む
	t.Run("11: Commit の後、書き込んだ内容に対して次の Plan", func(t *testing.T) {
		s, doc, path := startedSession(t, r, judgeDoc)
		after := commitWrite(t, s, r, path, taskOp(doc.RefKey, 0, true))

		if _, _, err := s.Plan(r, after, taskOp(doc.RefKey, 1, false)); err != nil {
			t.Errorf("次の Plan のエラー = %v, want nil（ErrChanged にならない）", err)
		}
	})

	// ケース 12: 書き込みに失敗した場合
	t.Run("12: Plan の後に Commit を呼ばない", func(t *testing.T) {
		s, doc, path := startedSession(t, r, judgeDoc)
		raw := readRaw(t, path)
		if _, _, err := s.Plan(r, raw, taskOp(doc.RefKey, 0, true)); err != nil {
			t.Fatalf("前提: Plan がエラーを返した: %v", err)
		}

		if _, _, err := s.Plan(r, raw, taskOp(doc.RefKey, 0, true)); err != nil {
			t.Errorf("元の raw に対する次の Plan のエラー = %v, want nil", err)
		}
		_, changed, err := s.Plan(r, raw, Op{Kind: OpUndo})
		if err != nil {
			t.Fatalf("取り消しの Plan がエラーを返した: %v", err)
		}
		if changed {
			t.Error("取り消しの changed が真（確定していない書き込みが履歴に入った）")
		}
	})

	// ケース 14: 前の書き込みでずれた位置を持ち回らない
	t.Run("14: 同じ行のセルを長く書き換えた後、その右のセル", func(t *testing.T) {
		const table = "| a | b |\n| --- | --- |\n| 1 | 2 |\n"
		s, doc, path := startedSession(t, r, table)
		after := commitWrite(t, s, r, path, cellOp(doc.RefKey, 0, 1, 0, "12345"))

		p, _, err := s.Plan(r, after, cellOp(doc.RefKey, 0, 1, 1, "9"))
		if err != nil {
			t.Fatalf("右のセルの Plan がエラーを返した: %v", err)
		}
		const want = "| a | b |\n| --- | --- |\n| 12345 | 9 |\n"
		if got := string(splice(t, after, p)); got != want {
			t.Errorf("書き換えた結果 = %q, want %q", got, want)
		}
	})

	// ケース 15: 逆順に戻り、最初の内容と 1 バイトも違わない
	t.Run("15: 2 回書き込み、取り消しと確定を 2 回", func(t *testing.T) {
		const original = "- [ ] a\n- [ ] b\n"
		s, doc, path := startedSession(t, r, original)
		commitWrite(t, s, r, path, taskOp(doc.RefKey, 0, true))
		commitWrite(t, s, r, path, taskOp(doc.RefKey, 1, true))

		wants := []string{"- [x] a\n- [ ] b\n", original}
		for i, want := range wants {
			got := string(commitWrite(t, s, r, path, Op{Kind: OpUndo}))
			if got != want {
				t.Errorf("%d 回目の取り消しの結果 = %q, want %q", i+1, got, want)
			}
		}
	})

	t.Run("16: 取り消した後に新しい書き込みを確定する", func(t *testing.T) {
		s, doc, path := startedSession(t, r, "- [ ] a\n- [ ] b\n")
		commitWrite(t, s, r, path, taskOp(doc.RefKey, 0, true))
		commitWrite(t, s, r, path, Op{Kind: OpUndo})
		commitWrite(t, s, r, path, taskOp(doc.RefKey, 1, true))

		_, changed, err := s.Plan(r, readRaw(t, path), Op{Kind: OpRedo})
		if err != nil {
			t.Fatalf("やり直しの Plan がエラーを返した: %v", err)
		}
		if changed {
			t.Error("やり直しの changed が真（やり直しの履歴を捨てていない）")
		}
	})

	t.Run("17: CellSource で raw が把握している内容と違う", func(t *testing.T) {
		s, doc, _ := startedSession(t, r, judgeDoc)
		_, err := s.CellSource(r, []byte("| h1 | h2 |\n| --- | --- |\n| 9 | 2 |\n"), doc.RefKey+":cell:0:1:0")
		if !errors.Is(err, ErrChanged) {
			t.Errorf("CellSource のエラー = %v, want ErrChanged", err)
		}
	})

	t.Run("18: CellSource で補われたセル", func(t *testing.T) {
		s, doc, path := startedSession(t, r, judgeDoc)
		_, err := s.CellSource(r, readRaw(t, path), doc.RefKey+":cell:0:2:1")
		if !errors.Is(err, ErrStale) {
			t.Errorf("CellSource のエラー = %v, want ErrStale", err)
		}
	})

	// 17・18 の対。常にエラーを返す実装を捕まえる（31.1 の「境界を追加してよい」）
	t.Run("17・18 の対: CellSource で存在するセル", func(t *testing.T) {
		s, doc, path := startedSession(t, r, judgeDoc)
		got, err := s.CellSource(r, readRaw(t, path), doc.RefKey+":cell:0:1:0")
		if err != nil {
			t.Fatalf("CellSource がエラーを返した: %v", err)
		}
		if got != "1" {
			t.Errorf("CellSource = %q, want %q", got, "1")
		}
	})
}

// TestEditSession_KeyCarryOver は、書き込みの直後の読み直しで鍵を引き継ぐかを検証する
// （UT-115 ケース 13・20。根拠: FR-143 / IMP-102, IMP-109）。
//
// **ケース 3・13・20 は組で意味を持つ。** 3 と 20 だけなら「書き込みの後の指示をすべて
// 拒む」実装で通り、13 だけなら「何も拒まない」実装で通る。
func TestEditSession_KeyCarryOver(t *testing.T) {
	r := renderer.New()
	const original = "- [ ] a\n- [ ] b\n"

	t.Run("13: 同じ描画の目印で 2 回続けて書き込む（鍵を引き継いで読み直す）", func(t *testing.T) {
		s, doc, path := startedSession(t, r, original)
		// 1 回目の前の描画で作った目印を、2 回目にも使う。
		op1, op2 := taskOp(doc.RefKey, 0, true), taskOp(doc.RefKey, 1, true)

		if err := s.Check(doc, true, op1); err != nil {
			t.Fatalf("前提: 1 回目の Check がエラーを返した: %v", err)
		}
		after1 := commitWrite(t, s, r, path, op1)

		doc2 := reload(t, r, path, LoadOptions{RefKey: doc.RefKey, ExpectDigest: digestOf(after1)})
		s.Loaded(doc2, true)

		if err := s.Check(doc2, true, op2); err != nil {
			t.Fatalf("2 回目の Check のエラー = %v, want nil（ErrStale にならない）", err)
		}
		p, _, err := s.Plan(r, readRaw(t, path), op2)
		if err != nil {
			t.Fatalf("2 回目の Plan がエラーを返した: %v", err)
		}
		// 1 回目の後の内容で位置を求め直す
		if p.Offset != 11 {
			t.Errorf("2 回目の Offset = %d, want 11", p.Offset)
		}
		const want = "- [x] a\n- [x] b\n"
		if got := string(splice(t, after1, p)); got != want {
			t.Errorf("2 回目の結果 = %q, want %q", got, want)
		}
	})

	t.Run("20: 読み直しを鍵なし（新しい鍵）で行う", func(t *testing.T) {
		s, doc, path := startedSession(t, r, original)
		op1, op2 := taskOp(doc.RefKey, 0, true), taskOp(doc.RefKey, 1, true)

		if err := s.Check(doc, true, op1); err != nil {
			t.Fatalf("前提: 1 回目の Check がエラーを返した: %v", err)
		}
		commitWrite(t, s, r, path, op1)

		doc2 := reload(t, r, path, LoadOptions{})
		s.Loaded(doc2, true)

		if err := s.Check(doc2, true, op2); !errors.Is(err, ErrStale) {
			t.Errorf("2 回目の Check のエラー = %v, want ErrStale（外部の再読み込みと同じ扱い）", err)
		}
	})
}
