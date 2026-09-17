package document

import (
	"bytes"
	"crypto/sha256"
)

// MaxUndo は取り消せる回数の上限（FR-144）。
const MaxUndo = 100

// EditLog は編集モードの 1 回分の書き込みの記録（IMP-108）。編集モードを始めたときに作り、
// 終えたときに捨てる（FR-140）。
//
// **ファイルに触れない純粋なデータ構造である。** 履歴に文書全体の写しを持たない。
// Patch は書き換えた箇所の前後の文字列だけを持ち、内容の一致は要約で確かめる
// （FR-144, NFR-020）。50 MB の文書でも、1 回の記録はセルの文字列程度の大きさで済む。
//
// `Undo` が返す Patch を適用できるのは、ファイルの内容が `Known()` と一致しているとき
// だけである。位置は直前の書き込みの後の内容に対するものであり、他者の変更の上に適用
// すると別の箇所を壊す。一致を確かめるのは呼び出し側（IMP-109 の Plan）である。
type EditLog struct {
	known [sha256.Size]byte // 把握している内容の要約（FR-143）
	undo  []Patch           // 古い順。末尾が直前の書き込み
	redo  []Patch           // 取り消した書き込み。末尾が直前に取り消したもの
}

// NewEditLog は把握している内容の要約 known から履歴を作る。
func NewEditLog(known [sha256.Size]byte) *EditLog {
	return &EditLog{known: known}
}

// Known は把握している内容の要約を返す。
func (l *EditLog) Known() [sha256.Size]byte {
	return l.known
}

// Record は書き込みを記録する。known を after にし、redo を捨て、undo が MaxUndo を超えたら先頭を捨てる。
//
// 新しい書き込みの後では、取り消した書き込みをやり直せない（FR-144, UT-113 ケース 5）。
func (l *EditLog) Record(p Patch, after [sha256.Size]byte) {
	l.known = after
	l.redo = nil
	l.undo = append(l.undo, clonePatch(p))
	if len(l.undo) > MaxUndo {
		// 古いものから捨てる（UT-113 ケース 7）。先頭を詰めて、捨てた Patch を配列に残さない。
		copy(l.undo, l.undo[1:])
		l.undo[len(l.undo)-1] = Patch{}
		l.undo = l.undo[:len(l.undo)-1]
	}
}

// Undo は取り消しに使う Patch（直前の書き込みの Inverse）を返す。無ければ ok は偽。
// 返しただけでは履歴を動かさない。書き込みに成功してから CommitUndo を呼ぶ。
//
// **「取り出す」と「確定する」を分ける。** 書き込みに失敗した取り消しで履歴が動くと、
// 次の Ctrl+Z が 1 つ先を取り消してしまう（UT-113 ケース 4）。
func (l *EditLog) Undo() (p Patch, ok bool) {
	if len(l.undo) == 0 {
		return Patch{}, false
	}
	return l.undo[len(l.undo)-1].Inverse(), true
}

// CommitUndo は known を after にし、undo の末尾を redo へ移す。
func (l *EditLog) CommitUndo(after [sha256.Size]byte) {
	l.known = after
	if len(l.undo) == 0 {
		return
	}
	last := l.undo[len(l.undo)-1]
	l.undo[len(l.undo)-1] = Patch{}
	l.undo = l.undo[:len(l.undo)-1]
	l.redo = append(l.redo, last)
}

// Redo は Undo の逆向き。直前に取り消した書き込みを、もう一度当てる Patch を返す。
func (l *EditLog) Redo() (p Patch, ok bool) {
	if len(l.redo) == 0 {
		return Patch{}, false
	}
	return clonePatch(l.redo[len(l.redo)-1]), true
}

// CommitRedo は CommitUndo の逆向き。known を after にし、redo の末尾を undo へ戻す。
func (l *EditLog) CommitRedo(after [sha256.Size]byte) {
	l.known = after
	if len(l.redo) == 0 {
		return
	}
	last := l.redo[len(l.redo)-1]
	l.redo[len(l.redo)-1] = Patch{}
	l.redo = l.redo[:len(l.redo)-1]
	l.undo = append(l.undo, last)
}

// clonePatch は p の写しを返す。履歴に積んだ後で呼び出し側のスライスが書き換えられても、
// 取り消しの内容が変わらないようにする。
func clonePatch(p Patch) Patch {
	return Patch{Offset: p.Offset, Old: bytes.Clone(p.Old), New: bytes.Clone(p.New)}
}
