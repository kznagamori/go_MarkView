package desktop

// 本ファイルは、文書を開いた経路（IMP-192）と、経路によって変わるもの（ツリールート・履歴・
// スクロール・DocumentDTO.Trigger）を持つ。
//
// **経路による差異は IMP-192 の表に挙がったものだけとする。** 分岐を open.go の本体に
// 散らさず、ここに集めて表と突き合わせられるようにする。

// openSource は文書を開いた経路（IMP-192）。
type openSource int

const (
	openFromDialog  openSource = iota // ファイル選択ダイアログ（FR-010）
	openFromDrop                      // ドラッグ＆ドロップ（FR-011）
	openFromArgs                      // コマンドライン引数（FR-012）
	openFromTree                      // ファイルツリーからの選択（FR-033）
	openFromLink                      // 文書内リンク（FR-050）
	openFromHistory                   // 履歴移動（FR-051）
	openFromReload                    // 再読み込み・更新検知・書き込みの前後の読み直し（FR-014, FR-015, IMP-195）
	openFromConfirm                   // 確認画面の Open anyway（FR-016）
)

// triggerOf は DocumentDTO.Trigger を決める（IMP-192 の呼び出し元の表, IMP-302）。
//
// 呼び出し元が明示していればその値とする。空なら、手動の再読み込みの経路（openFromReload）は
// reload、それ以外は open とする。**監視のイベント（watch）と書き込みの直後の読み直し（edit）は
// 呼び出し元が必ず明示する**——どちらも経路は openFromReload であり、経路からは区別できない。
func triggerOf(req openRequest) string {
	switch {
	case req.trigger != "":
		return req.trigger
	case req.src == openFromReload:
		return triggerReload
	default:
		return triggerOpen
	}
}

// changesTreeRoot はツリールートを変更する経路かを返す（IMP-192, FR-030）。
//
// **ツリーからの選択とリンク遷移では変更しない。** ドキュメント群を配布した
// ときに、利用者の操作でツリーが意図せず移動することを防ぐ（FR-030, FR-052）。
func changesTreeRoot(src openSource) bool {
	switch src {
	case openFromDialog, openFromDrop, openFromArgs:
		return true
	default:
		// openFromConfirm はここに含めない。確認画面を出した時点で
		// 変更済みである（commitPending）。
		return false
	}
}

// pushesHistory は履歴に積む経路かを返す（IMP-192, FR-051）。
//
// 履歴移動そのものと再読み込みでは積まない。積むと、戻るたびに履歴が伸びて
// 戻れなくなる。
func pushesHistory(src openSource) bool {
	switch src {
	case openFromHistory, openFromReload, openFromConfirm:
		// openFromConfirm は確認画面を出した時点で積み済みである
		// （commitPending）。ここで積むと同じ文書が 2 つ並ぶ。
		return false
	default:
		return true
	}
}

// scrollFor は描画後のスクロール指示を決める（IMP-192, IMP-302）。
//
// スクロールの扱いは経路に依存するため Go 側が決め、フロントエンドに経路を
// 意識させない。
func scrollFor(req openRequest) ScrollDTO {
	switch req.src {
	case openFromHistory:
		// 記録された位置を復元する（FR-051）。アンカーで開いた文書でも、
		// 離れる直前に記録した実際の位置のほうが正確である（IMP-311）。
		return ScrollDTO{Mode: scrollRestore, Top: req.scrollTop}

	case openFromReload:
		// 位置の出どころはフロントエンドが持つ現在値である。Go 側は
		// Top を設定しない（FR-014, IMP-321）。
		return ScrollDTO{Mode: scrollKeep}

	default:
		if req.anchor != "" {
			return ScrollDTO{Mode: scrollAnchor, Anchor: req.anchor}
		}
		return ScrollDTO{Mode: scrollTop}
	}
}
