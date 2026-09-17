package document

import (
	"crypto/sha256"
	"errors"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// 編集モードの状態と、書き込んでよいかの判断をすべてここに置く（IMP-109）。
//
// **ファイルにも Wails にも触れず、錠も持たない。** バインドメソッドの側（IMP-195）は、
// 錠を取り、ファイルを読み書きし、イベントを送るだけにする。判断を desktop に置くと
// 単体テストの対象外になり（UT-002, IMP-012）、書き込みの安全性を担う部分が検証されない。

// ErrStale は、指示を作った描画の後に再描画が起きていたことを表す（FR-143）。通知しない。
var ErrStale = errors.New("edit instruction is stale")

// OpKind は書き込みの種類（IMP-109）。
type OpKind int

const (
	OpTask OpKind = iota // SetTask（FR-141）
	OpCell               // SetCell（FR-142）
	OpUndo               // UndoEdit（FR-144）
	OpRedo               // RedoEdit（FR-144）
)

// Op はフロントエンドから届いた 1 回の書き込みの指示。
type Op struct {
	Kind    OpKind
	Ref     string // OpTask / OpCell: data-ref の値そのもの（IMP-316）
	Checked bool   // OpTask
	Text    string // OpCell
}

// EditSession は編集モードの状態（FR-140）。ゼロ値は「編集モードでない」。
type EditSession struct {
	on      bool
	removed bool     // 監視が表示中のファイルの削除を送った。次に読み込めたら偽に戻す
	log     *EditLog // on の間だけ非 nil（IMP-108）
	seq     uint64   // 状態を変えうる呼び出しのたびに増やす（IMP-302 の EditSeq）
}

// On は編集モードかを返す。
func (s *EditSession) On() bool {
	return s.on
}

// Seq は状態を変えうる呼び出しの通し番号を返す。
//
// フロントエンドは、先に届いた新しい値を後から届いた古い値で上書きしない（IMP-260）。
func (s *EditSession) Seq() uint64 {
	return s.seq
}

// CanStart は doc で編集モードを始められるかを返す（DocumentDTO.Editable）。
// showing は、画面が doc を表示していること（状態画面・文書未表示でない。IMP-190 の showing）。
//
// **Start はこの関数だけで判断する**——DocumentDTO.Editable とボタンの淡色（UI-021）と、
// 開始の可否を 1 つの式に揃える（FR-140 の表）。
func (s *EditSession) CanStart(doc *Document, showing bool) bool {
	return doc != nil && showing && doc.Editable() && !s.removed
}

// Start は CanStart が真なら編集モードを始めて真を返す。偽なら何も変えない。
//
// **既に編集モードの間に呼んでも、何も変えずに真を返す**（履歴を保つ。ボタンの二重押しで
// 履歴が消えないようにする。UT-114 ケース 18）。
func (s *EditSession) Start(doc *Document, showing bool) bool {
	if s.on {
		return true
	}
	if !s.CanStart(doc, showing) {
		return false
	}
	s.on = true
	s.log = NewEditLog(doc.Digest)
	s.seq++
	return true
}

// Stop は編集モードを終え、履歴を捨てる。
func (s *EditSession) Stop() {
	s.end()
	s.seq++
}

// Loaded は文書を開く処理（IMP-192）で読み込みに成功したときに呼ぶ。same は SameDocument。
//
// 読み込みの後の扱い（IMP-109 の表。上から順に当てはめる）:
//
//	on が偽                                   変えない（始めない）
//	same が偽（文書の切り替え）                終える
//	same が真で doc.Editable() が偽           終える（読み直したら不正なバイト列を含んでいた）
//	same が真で Digest が Known と一致しない   保つ。履歴だけを捨てて新しい Digest で作り直す
//	same が真で一致する                       保つ（履歴も保つ）
//
// **「編集モードを保つか」と「履歴を保つか」は別の軸である**（UT-114 ケース 12 と 13）。
// 外部で変更された後に古い履歴を残すと、外部の変更の上から古い状態を書き戻す（FR-144）。
//
// 読み込めた以上ファイルはあるため、削除の印を戻す。
func (s *EditSession) Loaded(doc *Document, same bool) {
	s.removed = false
	s.seq++

	switch {
	case !s.on:
	case !same, doc == nil, !doc.Editable():
		s.end()
	case doc.Digest != s.log.Known():
		s.log = NewEditLog(doc.Digest)
	}
}

// Left は状態画面を出したときに呼ぶ（文書の切り替えとして終える。IMP-192）。
//
// **削除の印は戻さない**（UT-114 ケース 6）。削除の後に状態画面へ移っただけで編集モードを
// 始められてはならない。
func (s *EditSession) Left() {
	s.end()
	s.seq++
}

// Removed は監視が表示中のファイルの削除を送ったときに呼ぶ（IMP-195, IMP-320）。
//
// 編集モードを終え、履歴を捨て、削除の印を立てる（FR-140 の表の「削除された」）。
// 次に読み込めるまで開始できない。
func (s *EditSession) Removed() {
	s.end()
	s.removed = true
	s.seq++
}

// Deleted は削除の印が立っているか（Removed の後、まだ読み込めていないか）を返す（IMP-192）。
func (s *EditSession) Deleted() bool {
	return s.removed
}

// Discard は取り消し・やり直しの履歴だけを捨てる。把握している内容（Known）は変えない（IMP-195 の 4）。
//
// **seq を増やさない**（編集モードの状態を変えない）。**把握している内容を作り直さない**——
// 作り直すと、読み直しに失敗して描画が古いまま（鍵も古いまま）なのに、次の書き込みが
// Plan の要約の確かめを通ってしまう。次に読み込めたときに Loaded が作り直す。
func (s *EditSession) Discard() {
	if s.log != nil {
		s.log = NewEditLog(s.log.Known())
	}
}

// Check は、指示がいまの描画に対して有効かを確かめる（IMP-195 の 1, 2）。ファイルを読む前に呼ぶ。
//
//  1. 編集モードで、doc があり、画面が doc を表示している
//  2. OpTask / OpCell なら、ParseRef で解け、鍵が doc.RefKey と一致し、種類が Op の種類と一致する
//     （取り消し・やり直しは鍵を持たないため省く）
//
// どちらかに当てはまらなければ ErrStale とする。指示を作った描画の後に、文書の切り替えか、
// 自分の書き込み以外による再描画が起きている（FR-143）。**生 HTML で偽装した目印も、鍵が
// 合わないためここで止まる**（NFR-030, UT-115 ケース 3）。種類の食い違う指示（セルの目印で
// タスクを書き換える）も拒む（ケース 19）。
func (s *EditSession) Check(doc *Document, showing bool, op Op) error {
	if !s.on || doc == nil || !showing {
		return ErrStale
	}
	switch op.Kind {
	case OpUndo, OpRedo:
		return nil
	case OpTask, OpCell:
		ref, err := ParseRef(op.Ref)
		if err != nil || ref.Key != doc.RefKey || ref.Kind != refKindOf(op.Kind) {
			return ErrStale
		}
		return nil
	default:
		return ErrStale
	}
}

// Plan は読んだ生バイト列 raw に対する Patch を作る（IMP-195 の 4, 5）。
//
//  4. **raw の SHA-256 が Known と一致する**こと。違えば ErrChanged（書き込まない。FR-143）
//  5. OpTask は PlanTask、OpCell は PlanCell、OpUndo は Undo、OpRedo は Redo
//
// PlanTask / PlanCell の ErrBadRef / ErrRefNotFound は ErrStale に写す。**ErrNotEditable は
// そのまま返す**——表の形の確かめ（IMP-106）で拒んだのは古い指示ではなく、黙って戻すと
// 確定した入力が通知も無く消える（FR-142, FR-110。IMP-195 が edit-failed で通知する）。
// 変わらない・履歴が空なら changed は偽。
func (s *EditSession) Plan(r *renderer.Renderer, raw []byte, op Op) (p Patch, changed bool, err error) {
	if !s.on || s.log == nil {
		return Patch{}, false, ErrStale
	}
	if sha256.Sum256(raw) != s.log.Known() {
		return Patch{}, false, ErrChanged
	}

	switch op.Kind {
	case OpTask, OpCell:
		ref, err := ParseRef(op.Ref)
		if err != nil || ref.Kind != refKindOf(op.Kind) {
			return Patch{}, false, ErrStale
		}
		if op.Kind == OpTask {
			p, changed, err = PlanTask(r, raw, ref, op.Checked)
		} else {
			p, changed, err = PlanCell(r, raw, ref, op.Text)
		}
		if err != nil {
			return Patch{}, false, staleOr(err)
		}
		return p, changed, nil

	case OpUndo:
		p, ok := s.log.Undo()
		return p, ok, nil

	case OpRedo:
		p, ok := s.log.Redo()
		return p, ok, nil

	default:
		return Patch{}, false, ErrStale
	}
}

// Commit は書き込みに成功した後に呼び、履歴を確定する（IMP-195 の 7）。after は書き込んだ内容。
//
// **書き込みに失敗したときは呼ばない**（IMP-108 の「取り出す」と「確定する」を分ける）。
func (s *EditSession) Commit(op Op, p Patch, after []byte) {
	if s.log == nil {
		return
	}
	h := sha256.Sum256(after)
	switch op.Kind {
	case OpTask, OpCell:
		s.log.Record(p, h)
	case OpUndo:
		s.log.CommitUndo(h)
	case OpRedo:
		s.log.CommitRedo(h)
	}
}

// CellSource は raw の中の ref のセルのソースを返す（GetCellSource。IMP-195）。
//
// Plan の 4 と同じ確かめを行う（一致しなければ ErrChanged）。ErrBadRef / ErrRefNotFound /
// ErrNotEditable は ErrStale とする（編集欄が開かないだけで済み、書き込みではないため
// 通知しない）。呼ぶ前に Check（OpCell）を通す。
func (s *EditSession) CellSource(r *renderer.Renderer, raw []byte, ref string) (string, error) {
	if !s.on || s.log == nil {
		return "", ErrStale
	}
	if sha256.Sum256(raw) != s.log.Known() {
		return "", ErrChanged
	}
	parsed, err := ParseRef(ref)
	if err != nil || parsed.Kind != RefCell {
		return "", ErrStale
	}
	src, err := CellSource(r, raw, parsed)
	if err != nil {
		return "", ErrStale
	}
	return src, nil
}

// end は編集モードを終え、履歴を捨てる（seq は呼び出し側が進める）。
func (s *EditSession) end() {
	s.on = false
	s.log = nil
}

// refKindOf は書き込みの種類に対応する目印の種類を返す。
func refKindOf(k OpKind) RefKind {
	if k == OpCell {
		return RefCell
	}
	return RefTask
}

// staleOr は書き換え位置の特定のエラーを、Plan が返すエラーへ写す（IMP-109 の表の 5）。
func staleOr(err error) error {
	switch {
	case errors.Is(err, ErrBadRef), errors.Is(err, ErrRefNotFound):
		return ErrStale
	default:
		return err
	}
}
