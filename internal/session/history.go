package session

// MaxHistory は表示履歴の保持上限（FR-051, IMP-191）。
const MaxHistory = 50

// Entry は履歴 1 件（IMP-191）。
type Entry struct {
	Path      string // 絶対パス（IMP-025）
	ScrollTop int    // フロントエンドから受け取るスクロール位置
	Anchor    string // アンカー付きリンクで開いた場合の見出し ID
}

// History は表示履歴を保持する（FR-051, IMP-191）。
//
// ブラウザと同じスタック構造とし、戻った状態から新しい文書を開くと前方履歴を
// 破棄する。**メモリ上のみに保持し、ディスクへは一切書かない**（NFR-042）。
// プロセスの終了とともに消える。
type History struct {
	entries []Entry
	index   int // 現在位置。空のときは -1
}

// NewHistory は空の履歴を作る。
func NewHistory() *History {
	return &History{index: -1}
}

// Push は履歴に 1 件積む（FR-051）。
//
// **現在位置より後ろ（前方履歴）は破棄する。** 戻ってから別の文書を開いたとき、
// 進む先が元の枝に残っているとブラウザと挙動が食い違う。
//
// 呼び出す前に SetScrollTop で現在位置の位置を記録しておくと、戻ったときに
// 元の位置へ復元できる（IMP-191）。
func (h *History) Push(e Entry) {
	if h.index >= 0 && h.index < len(h.entries)-1 {
		h.entries = h.entries[:h.index+1]
	}
	h.entries = append(h.entries, e)

	if len(h.entries) > MaxHistory {
		// 先頭から捨てる。元の配列を握り続けないよう作り直す。
		h.entries = append([]Entry(nil), h.entries[len(h.entries)-MaxHistory:]...)
	}

	h.index = len(h.entries) - 1
}

// Peek は位置を動かさずに、delta（-1: 戻る / +1: 進む）先の項目を返す（IMP-191。4.75.0。BUG-023）。
// 無ければ（履歴の端・空の履歴）ok は偽。
//
// **戻る・進むは Peek → 開く → Step / Drop の順に分ける。** v1.1.0 の Back / Forward は開く前に位置を
// 動かし、開けなくても戻さなかった。画面は前の文書のまま位置だけが消えた文書を指し、その間に別の文書を
// 開くと、見ていた文書が履歴から消えて戻れなくなった（BUG-023。UT-804 のケース 2・12）。
func (h *History) Peek(delta int) (Entry, bool) {
	i, ok := h.at(delta)
	if !ok {
		return Entry{}, false
	}
	return h.entries[i], true
}

// Step は位置を delta 動かす。開けた後（状態画面を含む）に呼ぶ（IMP-191）。範囲外なら何もしない。
func (h *History) Step(delta int) {
	if i, ok := h.at(delta); ok {
		h.index = i
	}
}

// Drop は delta 先の項目を取り除く。位置が指す文書は変えない。開けなかった後に呼ぶ（IMP-191。4.76.0）。
//
// **何を取り除き、位置をどこに置くかを 1 つの呼び出しにまとめる**（desktop に添字の計算を置かない。
// IMP-012。BUG-023 の調査報告）。位置より前（delta が負）を取り除いたら、位置を 1 つ前へずらして同じ文書を
// 指す（UT-804 のケース 8）。位置より後ろなら位置は変わらない（ケース 9）。delta が 0（位置そのもの）なら、
// 位置は 1 つ前（先頭なら 0）を指す（App はこの使い方をしない境界。ケース 10）。空の履歴・範囲外の delta
// では何もしない（ケース 1・11）。
func (h *History) Drop(delta int) {
	i, ok := h.at(delta)
	if !ok {
		return
	}
	h.entries = append(h.entries[:i], h.entries[i+1:]...)
	if delta < 0 || (delta == 0 && h.index > 0) {
		h.index--
	}
	if len(h.entries) == 0 {
		h.index = -1
	}
}

// CanBack は戻れるかを返す（FR-051 のボタンの淡色。IMP-323）。
func (h *History) CanBack() bool {
	_, ok := h.at(-1)
	return ok
}

// CanForward は進めるかを返す。
func (h *History) CanForward() bool {
	_, ok := h.at(+1)
	return ok
}

// at は位置から delta 先の添字を返す。履歴が空か、範囲の外なら ok は偽。
func (h *History) at(delta int) (int, bool) {
	i := h.index + delta
	if h.index < 0 || i < 0 || i >= len(h.entries) {
		return 0, false
	}
	return i, true
}

// SetScrollTop は現在位置のスクロール位置を更新する（IMP-191, FR-050）。
//
// 履歴が空のときは何もしない。
func (h *History) SetScrollTop(top int) {
	if h.index < 0 {
		return
	}
	h.entries[h.index].ScrollTop = top
}
