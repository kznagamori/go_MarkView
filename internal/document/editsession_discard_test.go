package document

import (
	"errors"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// TestEditSession_Discard は、履歴だけを捨てることを検証する
// （UT-114 ケース 19。根拠: FR-143, FR-144 / IMP-109, IMP-195）。
//
// **把握している内容まで作り直すと、古い描画のまま次の書き込みが要約の確かめを通る。**
// Discard は書き込みの前の不一致で読み直しに失敗しうるときに呼ぶ（IMP-195 の 4）。
func TestEditSession_Discard(t *testing.T) {
	r := renderer.New()
	doc, path := loadDoc(t, r, t.TempDir(), "a.md", "- [ ] a\n- [ ] b\n")

	var s EditSession
	if !s.Start(doc, true) {
		t.Fatal("前提: Start が偽")
	}
	after := commitWrite(t, &s, r, path, taskOp(doc.RefKey, 0, true))

	// 前提: 確定した書き込みは取り消せる（Discard が無ければ changed は真）
	if _, changed, err := s.Plan(r, after, Op{Kind: OpUndo}); err != nil || !changed {
		t.Fatalf("前提: 取り消しの Plan = changed %v, err %v, want 真と nil", changed, err)
	}

	before := s.Seq()
	s.Discard()

	if !s.On() {
		t.Error("On が偽（Discard は編集モードを終えない）")
	}
	if got := s.Seq(); got != before {
		t.Errorf("Seq = %d → %d, want 変わらない（編集モードの状態を変えない）", before, got)
	}

	// 履歴を捨てた
	_, changed, err := s.Plan(r, after, Op{Kind: OpUndo})
	if err != nil {
		t.Fatalf("取り消しの Plan がエラーを返した: %v", err)
	}
	if changed {
		t.Error("取り消しの changed が真（履歴を捨てていない）")
	}

	// 把握している内容は変わらない: 書き込んだ内容と同じ raw は通り、違う raw は拒む
	if _, _, err := s.Plan(r, after, taskOp(doc.RefKey, 1, true)); err != nil {
		t.Errorf("書き込んだ内容と同じ raw への Plan のエラー = %v, want nil（ErrChanged にならない）", err)
	}
	if _, _, err := s.Plan(r, []byte("- [ ] a\n- [ ] b\n- [ ] c\n"), taskOp(doc.RefKey, 1, true)); !errors.Is(err, ErrChanged) {
		t.Errorf("違う raw への Plan のエラー = %v, want ErrChanged", err)
	}
}

// TestEditSession_Deleted は削除の印を検証する
// （UT-114 ケース 20。根拠: FR-014, FR-140 / IMP-109, IMP-192）。
//
// **立ったままにならなければ、削除の後に同じ内容で戻った文書で編集モードを二度と
// 始められない**（IMP-192 は、印が立っていれば同じ内容でも再描画を送り Loaded を呼ぶ）。
func TestEditSession_Deleted(t *testing.T) {
	r := renderer.New()
	doc, _ := loadDoc(t, r, t.TempDir(), "a.md", "- [ ] a\n")

	var s EditSession
	if s.Deleted() {
		t.Error("ゼロ値の Deleted が真")
	}

	s.Removed()
	if !s.Deleted() {
		t.Error("Removed の後の Deleted が偽")
	}

	// Left は削除の印を戻さない（ケース 6 と同じ規則）
	s.Left()
	if !s.Deleted() {
		t.Error("続けて Left の後の Deleted が偽（Left が削除の印を戻した）")
	}

	// 読み込めた以上、ファイルはある
	s.Loaded(doc, true)
	if s.Deleted() {
		t.Error("続けて Loaded の後の Deleted が真")
	}
}
