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

// Back は 1 つ前の文書へ戻る（FR-051）。
//
// 戻れない場合は ok が false になる。空の履歴で呼んでも落ちない。
func (h *History) Back() (Entry, bool) {
	if h.index <= 0 {
		return Entry{}, false
	}

	h.index--
	return h.entries[h.index], true
}

// Forward は戻る前の文書へ進む（FR-051）。
//
// 進めない場合は ok が false になる。
func (h *History) Forward() (Entry, bool) {
	if h.index < 0 || h.index >= len(h.entries)-1 {
		return Entry{}, false
	}

	h.index++
	return h.entries[h.index], true
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
