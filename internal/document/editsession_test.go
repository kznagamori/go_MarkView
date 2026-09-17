package document

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// loadDoc は content のファイルを dir に作り、document.Load で読んだ文書とパスを返す。
func loadDoc(t *testing.T, r *renderer.Renderer, dir, name, content string) (*Document, string) {
	t.Helper()

	p := writeFile(t, dir, name, content)
	doc, err := Load(r, p, LoadOptions{})
	if err != nil {
		t.Fatalf("前提: Load がエラーを返した: %v", err)
	}
	return doc, p
}

// reload は path を読み直す。
func reload(t *testing.T, r *renderer.Renderer, path string, opts LoadOptions) *Document {
	t.Helper()

	doc, err := Load(r, path, opts)
	if err != nil {
		t.Fatalf("前提: 読み直しの Load がエラーを返した: %v", err)
	}
	return doc
}

// readRaw は path の生バイト列を返す。
func readRaw(t *testing.T, path string) []byte {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("前提: ファイルを読めない: %v", err)
	}
	return raw
}

// taskOp は doc の描画で作った目印でタスクを書き換える指示を返す。
func taskOp(key string, index int, checked bool) Op {
	return Op{Kind: OpTask, Ref: fmt.Sprintf("%s:task:%d", key, index), Checked: checked}
}

// cellOp は doc の描画で作った目印でセルを書き換える指示を返す。
func cellOp(key string, table, row, col int, text string) Op {
	return Op{Kind: OpCell, Ref: fmt.Sprintf("%s:cell:%d:%d:%d", key, table, row, col), Text: text}
}

// commitWrite は IMP-195 の手順（読み込み → Plan → 書き込み → Commit）を
// テストの側で行い、書き込んだ内容を返す。書き込みは os.WriteFile で済ませる
// （Replace は UT-112 が見る）。
func commitWrite(t *testing.T, s *EditSession, r *renderer.Renderer, path string, op Op) []byte {
	t.Helper()

	raw := readRaw(t, path)
	p, changed, err := s.Plan(r, raw, op)
	if err != nil {
		t.Fatalf("前提: Plan がエラーを返した: %v", err)
	}
	if !changed {
		t.Fatalf("前提: Plan の changed が偽: %+v", op)
	}

	after := splice(t, raw, p)
	if err := os.WriteFile(path, after, 0o644); err != nil {
		t.Fatalf("前提: 書き込めない: %v", err)
	}
	s.Commit(op, p, after)
	return after
}

// digestOf は書き込んだ内容の要約（LoadOptions.ExpectDigest に渡す値）を返す。
func digestOf(b []byte) [sha256.Size]byte {
	return sha256.Sum256(b)
}

// TestEditSession_Start は編集モードの開始を検証する
// （UT-114 ケース 1〜8・17。根拠: FR-014, FR-140, FR-144 / IMP-109）。
//
// **開始できないケースを先に書く**（UT-013）。
func TestEditSession_Start(t *testing.T) {
	r := renderer.New()
	dir := t.TempDir()
	doc, _ := loadDoc(t, r, dir, "a.md", "- [ ] a\n")

	t.Run("1: ゼロ値", func(t *testing.T) {
		var s EditSession
		if s.On() {
			t.Error("On が真")
		}
	})

	t.Run("2: Start(nil, true)", func(t *testing.T) {
		var s EditSession
		if s.Start(nil, true) {
			t.Error("Start が真")
		}
		if s.On() {
			t.Error("On が真")
		}
	})

	t.Run("3: 状態画面（showing が偽）", func(t *testing.T) {
		var s EditSession
		if s.Start(doc, false) {
			t.Error("Start が真")
		}
	})

	t.Run("4: 不正なバイト列を含む文書", func(t *testing.T) {
		invalid, _ := loadDoc(t, r, dir, "invalid.md", "- [ ] a\xFF\n")
		var s EditSession
		if s.Start(invalid, true) {
			t.Error("Start が真（FR-140 の表）")
		}
	})

	t.Run("5: Removed の後に Start", func(t *testing.T) {
		var s EditSession
		s.Removed()
		if s.Start(doc, true) {
			t.Error("Start が真")
		}
	})

	// ケース 6: Left は削除の印を戻さない
	t.Run("6: Removed → Left → Start", func(t *testing.T) {
		var s EditSession
		s.Removed()
		s.Left()
		if s.Start(doc, true) {
			t.Error("Start が真（Left が削除の印を戻した）")
		}
	})

	// ケース 7: 読み込めた以上、ファイルはある
	t.Run("7: Removed → Loaded(doc, true) → Start", func(t *testing.T) {
		var s EditSession
		s.Removed()
		s.Loaded(doc, true)
		if !s.Start(doc, true) {
			t.Error("Start が偽")
		}
	})

	t.Run("8: 正常な文書で Start", func(t *testing.T) {
		var s EditSession
		can := s.CanStart(doc, true)
		started := s.Start(doc, true)

		if !started {
			t.Error("Start が偽")
		}
		if !s.On() {
			t.Error("On が偽")
		}
		if can != started {
			t.Errorf("CanStart = %v と Start = %v が一致しない", can, started)
		}
	})

	t.Run("17: 編集モードの間に Removed", func(t *testing.T) {
		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		s.Removed()

		if s.On() {
			t.Error("On が真")
		}
		if s.Start(doc, true) {
			t.Error("続く Start が真（ケース 5 と同じ）")
		}
	})
}

// TestEditSession_AfterLoad は読み込みの後の扱いを検証する
// （UT-114 ケース 9〜14・18。根拠: FR-014, FR-140, FR-144 / IMP-109）。
//
// **「編集モードを保つか」と「履歴を保つか」は別の軸である**（ケース 12 と 13）。
func TestEditSession_AfterLoad(t *testing.T) {
	r := renderer.New()

	t.Run("9: 編集モードの間に別の文書で Loaded(same=false)", func(t *testing.T) {
		dir := t.TempDir()
		doc, _ := loadDoc(t, r, dir, "a.md", "- [ ] a\n")
		other, _ := loadDoc(t, r, dir, "b.md", "- [ ] b\n")

		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		s.Loaded(other, false)
		if s.On() {
			t.Error("On が真")
		}
	})

	t.Run("10: 編集モードの間に Left", func(t *testing.T) {
		doc, _ := loadDoc(t, r, t.TempDir(), "a.md", "- [ ] a\n")

		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		s.Left()
		if s.On() {
			t.Error("On が真")
		}
	})

	t.Run("11: 読み直したら不正なバイト列を含んでいた", func(t *testing.T) {
		dir := t.TempDir()
		doc, path := loadDoc(t, r, dir, "a.md", "- [ ] a\n")

		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		if err := os.WriteFile(path, []byte("- [ ] a\xFF\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		s.Loaded(reload(t, r, path, LoadOptions{}), true)
		if s.On() {
			t.Error("On が真")
		}
	})

	t.Run("12: 書き込みを確定した後、同じ内容で Loaded(same=true)", func(t *testing.T) {
		doc, path := loadDoc(t, r, t.TempDir(), "a.md", "- [ ] a\n")

		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		after := commitWrite(t, &s, r, path, taskOp(doc.RefKey, 0, true))
		s.Loaded(reload(t, r, path, LoadOptions{RefKey: doc.RefKey, ExpectDigest: digestOf(after)}), true)

		if !s.On() {
			t.Error("On が偽")
		}
		// 取り消せる
		_, changed, err := s.Plan(r, readRaw(t, path), Op{Kind: OpUndo})
		if err != nil {
			t.Fatalf("取り消しの Plan がエラーを返した: %v", err)
		}
		if !changed {
			t.Error("取り消しの changed が偽（履歴を捨てた）")
		}
	})

	// ケース 13: 落とすと、外部の変更の上から古い状態を書き戻す（FR-144）
	t.Run("13: 書き込みを確定した後、外部で書き換えて Loaded(same=true)", func(t *testing.T) {
		doc, path := loadDoc(t, r, t.TempDir(), "a.md", "- [ ] a\n")

		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		commitWrite(t, &s, r, path, taskOp(doc.RefKey, 0, true))

		if err := os.WriteFile(path, []byte("- [ ] a\n- [ ] b\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		external := reload(t, r, path, LoadOptions{})
		s.Loaded(external, true)

		if !s.On() {
			t.Error("On が偽")
		}
		raw := readRaw(t, path)
		_, changed, err := s.Plan(r, raw, Op{Kind: OpUndo})
		if err != nil {
			t.Errorf("取り消しの Plan がエラーを返した: %v", err)
		}
		if changed {
			t.Error("取り消しの changed が真（取り消すものが無いはず）")
		}
		if _, _, err := s.Plan(r, raw, taskOp(external.RefKey, 1, true)); err != nil {
			t.Errorf("新しい内容に対する書き込みの Plan がエラーを返した: %v（ErrChanged にならない）", err)
		}
	})

	t.Run("14: Stop", func(t *testing.T) {
		doc, _ := loadDoc(t, r, t.TempDir(), "a.md", "- [ ] a\n")

		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		s.Stop()
		if s.On() {
			t.Error("On が真")
		}
	})

	t.Run("18: 書き込みを確定した後、編集モードの間にもう一度 Start", func(t *testing.T) {
		doc, path := loadDoc(t, r, t.TempDir(), "a.md", "- [ ] a\n")

		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		commitWrite(t, &s, r, path, taskOp(doc.RefKey, 0, true))

		if !s.Start(doc, true) {
			t.Error("Start が偽")
		}
		if !s.On() {
			t.Error("On が偽")
		}
		_, changed, err := s.Plan(r, readRaw(t, path), Op{Kind: OpUndo})
		if err != nil {
			t.Fatalf("取り消しの Plan がエラーを返した: %v", err)
		}
		if !changed {
			t.Error("取り消しの changed が偽（履歴を捨てた）")
		}
	})
}

// TestEditSession_Seq は状態の通し番号を検証する
// （UT-114 ケース 15・16。根拠: FR-140 / IMP-109）。
func TestEditSession_Seq(t *testing.T) {
	r := renderer.New()
	dir := t.TempDir()
	doc, _ := loadDoc(t, r, dir, "a.md", "- [ ] a\n")
	other, _ := loadDoc(t, r, dir, "b.md", "- [ ] b\n")

	// ケース 15: 呼ぶたびに大きくなる
	t.Run("15: 状態を変えうる呼び出しのたびに増える", func(t *testing.T) {
		var s EditSession

		steps := []struct {
			name string
			call func()
		}{
			{"編集モードを始めた Start", func() {
				if !s.Start(doc, true) {
					t.Fatal("前提: Start が偽")
				}
			}},
			{"編集モードの間の Loaded", func() { s.Loaded(doc, true) }},
			{"Stop", func() { s.Stop() }},
			{"編集モードでない間の Loaded", func() { s.Loaded(other, false) }},
			{"Left", func() { s.Left() }},
			{"Removed", func() { s.Removed() }},
		}

		for _, step := range steps {
			before := s.Seq()
			step.call()
			if after := s.Seq(); after <= before {
				t.Errorf("%s の前後で Seq = %d → %d, want 大きくなる", step.name, before, after)
			}
		}
	})

	// ケース 16: 変わらない（IMP-109）
	t.Run("16: 偽を返した Start", func(t *testing.T) {
		var s EditSession
		before := s.Seq()
		if s.Start(nil, true) {
			t.Fatal("前提: Start が真")
		}
		if after := s.Seq(); after != before {
			t.Errorf("Seq = %d → %d, want 変わらない", before, after)
		}
	})

	t.Run("16: 編集モードの間の Start", func(t *testing.T) {
		var s EditSession
		if !s.Start(doc, true) {
			t.Fatal("前提: Start が偽")
		}
		before := s.Seq()
		if !s.Start(doc, true) {
			t.Error("編集モードの間の Start が偽")
		}
		if after := s.Seq(); after != before {
			t.Errorf("Seq = %d → %d, want 変わらない", before, after)
		}
	})
}
