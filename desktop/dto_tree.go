package desktop

import "github.com/kznagamori/go_MarkView/internal/filetree"

// 本ファイルはファイルツリーの操作と履歴の境界の型を定める（IMP-304, IMP-317, IMP-323）。
// dto.go が 400 行の目安（IMP-011）を超えるため分けた。ツリーの操作のバインドメソッドは
// tree.go（IMP-196）、戻る・進むと history:changed の送出は history.go（IMP-191, IMP-323）にある。

// TreeNodeDTO はファイルツリーの 1 要素（IMP-304）。
//
// 子ノードは含めない。展開のたびに ReadDir を呼ぶ（FR-032 の遅延展開）。
type TreeNodeDTO struct {
	Name  string `json:"name"`
	Path  string `json:"path"` // 絶対パス（IMP-025）
	IsDir bool   `json:"isDir"`

	// Kind はファイルの種類（"markdown" | "plantuml" | "mermaid" | "image"）。ディレクトリは空。
	// フロントエンドはこれでアイコンを決める（UI-030, DSP-112）。**拡張子をフロントエンドで
	// 判定しない**（IMP-300 の 2）。
	Kind string `json:"kind"`

	// HasChild は展開可能かを示す（FR-032 の先読み結果）。
	HasChild bool `json:"hasChild"`

	// Omitted は **その要素が属する一覧から件数上限で除かれた数**
	// （IMP-130, FR-032）。切り詰めが起きた場合、返すすべての要素に同じ値が入る。
	// フロントエンドは先頭の要素を見て、末尾に `… and N more` を出す（DSP-112）。
	Omitted int `json:"omitted"`
}

// newTreeNodeDTOs は filetree の結果を DTO の並びへ写す（IMP-304）。
//
// HasChild はディレクトリかどうかと一致する。ReadDir が表示している種類を含まない
// ディレクトリを既に除いており（FR-031, IMP-133）、残ったディレクトリはすべて展開する
// 価値があるためである。**先読みの判定を DTO 側でやり直さない。**
func newTreeNodeDTOs(nodes []filetree.Node) []TreeNodeDTO {
	dtos := make([]TreeNodeDTO, 0, len(nodes))

	for _, n := range nodes {
		dtos = append(dtos, TreeNodeDTO{
			Name:     n.Name,
			Path:     n.Path,
			IsDir:    n.IsDir,
			Kind:     string(n.Kind),
			HasChild: n.IsDir,
			Omitted:  n.Omitted,
		})
	}

	return dtos
}

// TreeActionDTO はファイルツリーの操作のうち、文書を開かないものの結果（IMP-317, FR-036）。
//
// **すべて空なら「何もしなかった」**（IMP-135 の ErrNotInTree など）。
//
// Root が空でなければ、フロントエンドはツリーを新しいルートで読み直し、ステータス領域の
// パス（DisplayPath / OutsideTree）を差し替える（FR-030, FR-052。IMP-270）。表示中の文書・
// 履歴・編集モードは変わっていない。**表示用パスは Go 側が求める**（IMP-300 の 2）。
type TreeActionDTO struct {
	Root        string    `json:"root"`        // ツリールートを変えたときの新しいルート。変えなければ空
	DisplayPath string    `json:"displayPath"` // 同上のとき、画面の対象の表示用パス（FR-052）。対象が無ければ空
	OutsideTree bool      `json:"outsideTree"` // 同上のとき、画面の対象がツリールートの外か
	Error       *ErrorDTO `json:"error"`       // not-found / permission / open-failed / clipboard（IMP-315）
}

// HistoryDTO は戻れるか・進めるか（FR-051, IMP-323）。
//
// history:changed のペイロードであり、起動時の値は InitialStateDTO.History で渡す。
// **到着順は決まっていない。** フロントエンドは、それまでに受け取った Seq より小さいものを
// 捨てる（EditModeDTO と同じ考え方。IMP-316）。**フロントエンドで履歴を数え直さない**（FR-051）。
type HistoryDTO struct {
	CanBack    bool   `json:"canBack"`    // session.History.CanBack()
	CanForward bool   `json:"canForward"` // session.History.CanForward()
	Seq        uint64 `json:"seq"`        // App.historySeq。送るたびに 1 増やす
}
