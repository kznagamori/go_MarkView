package desktop

// 本ファイルは編集モード（FR-140〜FR-144）のバインドメソッドの戻り値を定める（IMP-316）。
// 処理の中身は editmode.go（IMP-195）にある。外部エディタ（dto_editor.go）と混ぜない。
//
// **表示の更新はこれらの戻り値ではなく document:changed（IMP-320）で届く**（AR-061）。
// フロントエンドは、戻り値とイベントのどちらが先に届いても成り立つように書く（IMP-195）。
//
// **目印の値（ref）はフロントエンドが HTML から読んだ文字列をそのまま受け取り、Go 側で解く**
// （document.ParseRef）。番号へ分解して受け取らない——鍵の照合を省く経路を作らない。

// EditModeDTO は SetEditMode の結果（IMP-316）。
//
// **失敗を伝える欄を持たない。** 開始できない場合も、回復したパニックも失敗としない
// （IMP-195, IMP-310）。On が結果としての状態を表す。
type EditModeDTO struct {
	On  bool   `json:"on"`  // 結果として編集モードか
	Seq uint64 `json:"seq"` // 編集モードの状態の版（IMP-109。DocumentDTO.EditSeq と同じ系列）
}

// EditResultDTO は書き込み・取り消し・やり直しの結果（IMP-316）。
//
// Changed が偽で Stale も Error も無いのは、「内容が変わらなかった」または「取り消すものが
// 無かった」ことを表す（FR-142, FR-144）。フロントエンドは見た目を戻すだけにし、通知しない。
type EditResultDTO struct {
	Changed bool      `json:"changed"` // ファイルを書き換えたか
	Stale   bool      `json:"stale"`   // 指示を作った描画が古かった（FR-143）。通知しない
	Error   *ErrorDTO `json:"error"`   // edit-conflict / edit-failed（IMP-195 の 3〜6）。成功と Stale では null
}

// CellSourceDTO はセルの編集欄に入れるソース（FR-142, IMP-316）。
type CellSourceDTO struct {
	Text  string    `json:"text"`
	Stale bool      `json:"stale"` // 読めない・古い指示・編集できないセル。通知しない（IMP-195）
	Error *ErrorDTO `json:"error"` // edit-conflict のときだけ
}
