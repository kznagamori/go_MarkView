package desktop

import "github.com/kznagamori/go_MarkView/internal/session"

// 本ファイルは表示履歴の戻る・進む（FR-051, IMP-191, IMP-310）と、history:changed（IMP-323）を持つ。
// bind.go が 400 行の目安（IMP-011）を超えるため分けた。
//
// **判断（何を取り除き、位置をどこに置くか）は internal/session の History が持つ**（IMP-012,
// IMP-191）。ここは錠を取り、開く処理の結果で Step と Drop のどちらを呼ぶかを選び、知らせるだけにする。

// HistoryBack は 1 つ前の文書へ戻る（IMP-310, FR-051）。
//
// 戻れない場合は何も起きなかったものとして返す（IMP-308）。フロントエンドはボタンを淡色にしているが
// （UI-021, IMP-323）、ショートカットとマウスのサイドボタンからも呼ばれるため Go 側でも端を守る。
func (a *App) HistoryBack() OpenResultDTO {
	return a.historyMove(-1)
}

// HistoryForward は戻る前の文書へ進む（IMP-310, FR-051）。
func (a *App) HistoryForward() OpenResultDTO {
	return a.historyMove(+1)
}

// historyMove は IMP-191 の「戻る・進むの手順」を行う（BUG-023）。
//
//  1. Peek(delta) で移動先の項目を得る。無ければ何もしない（履歴の端。IMP-308）
//  2. 項目を開く（openFromHistory。IMP-192）
//  3. 開けた（文書を表示した）、または状態画面になったなら Step(delta)
//  4. 表示を変えない失敗（not-found / permission / not-supported）なら Drop(delta) で移動先の項目を
//     取り除き、位置は動かさない。理由は戻り値の Error でステータス領域に出す（FR-110）
//
// **1〜4 は 1 つの ioMu の中で行う**（IMP-191）。2 回続けて押したとき、2 回目が 1 回目の Step より
// 前の位置から移動先を求めると、同じ項目を 2 回開いて位置だけが 2 つ進む。**ioMu を持ったまま開くため
// openLocked の系統（openOutcome）を呼ぶ**——open を呼ぶと sync.Mutex の再入で止まる（IMP-190）。
//
// **v1.1.0 は開く前に位置を動かし（Back / Forward）、開けなくても戻さなかった。** 画面は前の文書の
// まま位置だけが消えた文書を指し、その間に別の文書を開くと、見ていた文書が履歴から消えて戻れなく
// なった（BUG-023）。
func (a *App) historyMove(delta int) (res OpenResultDTO) {
	var path string
	defer a.recoverOpen(&path, &res)

	// defer は後に置いたものから動く。パニックのときは ioMu を解いてから recoverOpen が動く
	// （回復は ioMu を取る。openPanicked）。
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	// 1.
	entry, ok := a.peekHistory(delta)
	if !ok {
		return OpenResultDTO{}
	}
	path = entry.Path

	// 2.
	dto, outcome, err := a.openOutcome(openRequest{
		path:      entry.Path,
		src:       openFromHistory,
		anchor:    entry.Anchor,
		scrollTop: entry.ScrollTop,
	})

	// 3, 4. **開けなかったときだけ取り除く。** 状態画面は「開けなかった」に入れない——画面がその
	// 文書を対象にしている（UI-013。BUG-024 で状態画面も履歴に積むことにした）。
	a.changeHistory(func(h *session.History) {
		if outcome == outcomeKept {
			h.Drop(delta)
			return
		}
		h.Step(delta)
	})

	return newOpenResult(entry.Path, dto, err)
}

// peekHistory は mu の内側で移動先の項目を得る（IMP-191 の 1）。
//
// **錠は defer で解く。** 途中でパニックが起きても、回復（openPanicked）が錠を取れるようにする。
func (a *App) peekHistory(delta int) (session.Entry, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.history.Peek(delta)
}

// changeHistory は mu の内側で履歴を変え、変えた**後の**値を history:changed で送る（IMP-323）。
//
// 値は錠の内側で読み、送出は錠を解いた後で行う（IMP-192 の Wails の呼び出しと同じ）。change が
// 何もしない関数なら、いまの値を送り直すだけになる（回復の後に写しをそろえる。IMP-310）。
func (a *App) changeHistory(change func(*session.History)) {
	h := a.changeHistoryLocked(change)
	a.emit(eventHistoryChanged, h)
}

// changeHistoryLocked は changeHistory の錠の内側。**錠は defer で解く**（peekHistory と同じ理由）。
func (a *App) changeHistoryLocked(change func(*session.History)) HistoryDTO {
	a.mu.Lock()
	defer a.mu.Unlock()

	change(a.history)
	return a.nextHistoryDTO()
}

// nextHistoryDTO は、送る history:changed の値を作る（IMP-323）。**呼び出し側は mu を保持していること。**
//
// 送るたびに版（historySeq）を 1 増やす。**到着順は決まっていない**ため、フロントエンドはそれまでに
// 受け取った版より小さいものを捨てる。
func (a *App) nextHistoryDTO() HistoryDTO {
	a.historySeq++
	return a.historyDTO()
}

// currentHistory は mu の内側でいまの履歴の値を読む（InitialStateDTO.History。IMP-303）。
func (a *App) currentHistory() HistoryDTO {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.historyDTO()
}

// historyDTO はいまの履歴の値を版を増やさずに返す（起動時の InitialStateDTO.History。IMP-303）。
// **呼び出し側は mu を保持していること。**
func (a *App) historyDTO() HistoryDTO {
	return HistoryDTO{
		CanBack:    a.history.CanBack(),
		CanForward: a.history.CanForward(),
		Seq:        a.historySeq,
	}
}
