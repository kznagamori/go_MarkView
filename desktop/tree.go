package desktop

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/kznagamori/go_MarkView/internal/applog"
	"github.com/kznagamori/go_MarkView/internal/filetree"
	"github.com/kznagamori/go_MarkView/internal/opener"
	"github.com/kznagamori/go_MarkView/internal/session"
)

// 本ファイルはファイルツリーの操作のバインドメソッドを持つ（IMP-196, IMP-310, FR-030, FR-032,
// FR-033, FR-036, FR-037）。
//
// **ツリーからパスを受け取る操作は、すべて filetree.Check（IMP-135）を通す。** フロントエンドが渡す
// 値は、形が正しくても信じない（NFR-030, NFR-035）。**検査の判断は filetree が持ち、ここは錠を取り、
// 検査を呼び、結果を開く処理・起動・クリップボード・ツリールートの変更へ渡すだけにする**（IMP-012）。
//
// ツリールートと表示する種類（show）は mu の内側で読み、検査・読み込み・起動は mu の外で行う
// （IMP-196）。どれも ioMu を取らない——文書を開く OpenFromTree だけが open を通して取る。

// OpenFromTree はファイルツリーで選ばれたファイルを開く（IMP-310, IMP-196, FR-033, FR-036 の Open）。
//
// **開く前に検査を通す**（WantOpenable。4.75.0）。v1.1.0 は検査せず、document.Load が Markdown 以外を
// 拒むことで足りていた。開けるファイルが 3 種類になり、**ツリーに出していない種類（フィルターで隠した
// 画像など）をツリーの経路で開かない**ためである。**ツリールートは変更しない**（FR-030）。
//
// 検査の失敗: ErrNotInTree は何も起きなかったもの、ErrMissing は not-found、それ以外は permission
// （ステータスに出す。表示も確認待ちも変えない）。回復したパニックは文書を開くメソッドと同じに扱う。
//
// **回復で画面の対象にするのは、検査を通ったパスだけ**（IMP-300 の 3）。検査の前に落ちたら対象は
// 分からないものとする——フロントエンドから届いた値をそのまま target にすると、その後の F5 と
// 「エディタで開く」が検査を通っていないパスを使う（IMP-190）。
func (a *App) OpenFromTree(path string) (res OpenResultDTO) {
	var target string
	defer a.recoverOpen(&target, &res)

	checked, err := a.checkTree(path, filetree.WantOpenable, false)
	if err != nil {
		return OpenResultDTO{Error: newTreeCheckErrorDTO(path, err)}
	}
	target = checked

	return a.openResult(checked, openRequest{path: checked, src: openFromTree})
}

// ReadDir はディレクトリの直下を読む（IMP-310, IMP-196, FR-032, FR-035, FR-037）。
//
// 再帰しない。展開のたびに呼ばれる（FR-032 の遅延展開）。**キャッシュを持たない**（IMP-310）。
// **空文字はツリールートを読む**（検査の前に置き換える。ツリールートが未確定なら空の一覧）。
//
// **展開も検査を通す**（WantDir。ツリールートそのものも可）。ツリーに出していないディレクトリ
// （.git など）の中身をフロントエンドへ返さない（FR-036）。種別を返さない（IMP-308）ため、検査の
// 失敗は Go の error として返し、**ErrMissing の文言は File not found: <path>**、ほかの失敗は
// Cannot access: <path> とする（フロントエンドはそのままステータスに出し、展開しない。4.76.0）。
// 表示する種類は App.cfg のツリーのフィルターから作る（IMP-134）。
func (a *App) ReadDir(path string) (nodes []TreeNodeDTO, err error) {
	defer recoverBind(&err)

	if path == "" {
		path = a.GetTreeRoot()
	}
	if path == "" {
		return []TreeNodeDTO{}, nil
	}

	checked, err := a.checkTree(path, filetree.WantDir, true)
	switch {
	case errors.Is(err, filetree.ErrNotInTree):
		return []TreeNodeDTO{}, err
	case err != nil:
		return []TreeNodeDTO{}, errors.New(newTreeCheckErrorDTO(path, err).Message)
	}

	read, err := filetree.ReadDir(checked, a.treeShow())
	if err != nil {
		return nil, err
	}

	return newTreeNodeDTOs(read), nil
}

// SetTreeRoot はツリーのディレクトリをツリールートにする（IMP-310, IMP-196, FR-030, FR-036）。
//
// 検査は WantDir（**ツリールートそのものは受け付けない**）。変えた結果は戻り値で返し、
// **tree:root-changed は送らない**（呼び出したフロントエンドが戻り値で更新する。AR-061）。
func (a *App) SetTreeRoot(path string) (res TreeActionDTO) {
	defer recoverTreeAction(&res)

	checked, err := a.checkTree(path, filetree.WantDir, false)
	if err != nil {
		return TreeActionDTO{Error: newTreeCheckErrorDTO(path, err)}
	}

	return a.moveTreeRoot(checked)
}

// SetTreeRootUp はツリールートの親をツリールートにする（IMP-310, IMP-196, FR-036 の
// Set parent as tree root）。
//
// **パスを受け取らない。** いまのツリールートの親を Go 側で求める。**ファイルシステムの根とドライブの
// 根（filepath.Dir が自分自身を返す）では何もしない**（メニューの項目は淡色。UI-032）。親が存在しない・
// ディレクトリでないときも何もしない。
func (a *App) SetTreeRootUp() (res TreeActionDTO) {
	defer recoverTreeAction(&res)

	parent, ok := parentOf(a.GetTreeRoot())
	if !ok {
		return TreeActionDTO{}
	}
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return TreeActionDTO{}
	}

	return a.moveTreeRoot(parent)
}

// CanSetTreeRootUp は、ツリールートに親があるか（根でないか）を返す（IMP-310, IMP-196, UI-032）。
//
// メニューの Set parent as tree root の淡色に使う（IMP-270）。**フロントエンドはパスから根かどうかを
// 判断しない**（IMP-300 の 2）。
func (a *App) CanSetTreeRootUp() (ok bool) {
	defer recoverQuiet()

	_, ok = parentOf(a.GetTreeRoot())
	return ok
}

// OpenInFileManager はツリーのディレクトリを OS の既定のファイルマネージャで開く（IMP-310, IMP-196,
// IMP-173, FR-036, NFR-035）。
//
// 検査は WantDir（ツリールートそのものも可）。**ファイルマネージャで開くのはディレクトリだけ**——
// opener.OpenDir も起動の直前にもう一度確かめる（IMP-173）。起動の失敗は open-failed。
func (a *App) OpenInFileManager(path string) (res TreeActionDTO) {
	defer recoverTreeAction(&res)

	checked, err := a.checkTree(path, filetree.WantDir, true)
	if err != nil {
		return TreeActionDTO{Error: newTreeCheckErrorDTO(path, err)}
	}

	return openDirResult(checked)
}

// OpenContainingFolder はツリーのファイルの親ディレクトリをファイルマネージャで開く（IMP-310,
// IMP-196, FR-036 の Open containing folder）。
//
// 検査は WantOpenable（ファイル）。親はツリールートそのものでもよい。**ファイルを選んだ状態には
// しない**（IMP-173）。
func (a *App) OpenContainingFolder(path string) (res TreeActionDTO) {
	defer recoverTreeAction(&res)

	checked, err := a.checkTree(path, filetree.WantOpenable, false)
	if err != nil {
		return TreeActionDTO{Error: newTreeCheckErrorDTO(path, err)}
	}

	return openDirResult(filepath.Dir(checked))
}

// CopyTreePath はツリーの項目の絶対パスをクリップボードへ格納する（IMP-310, IMP-196, FR-036 の
// Copy path）。
//
// 検査は WantAny（ツリールートそのものも可）。パスの文字列はフロントエンドも持っているが、**ツリーから
// パスを受け取る操作はすべて同じ検査を通す**（ファイルが消えていれば not-found を出す）。失敗は
// clipboard、**成功は知らせない**（UI-060 の「情報を出すのはエディタの起動だけ」）。
func (a *App) CopyTreePath(path string) (res TreeActionDTO) {
	defer recoverTreeAction(&res)

	checked, err := a.checkTree(path, filetree.WantAny, true)
	if err != nil {
		return TreeActionDTO{Error: newTreeCheckErrorDTO(path, err)}
	}

	if err := a.writeClipboard(checked); err != nil {
		return TreeActionDTO{Error: &ErrorDTO{Kind: errKindClipboard, Message: "Failed to copy."}}
	}
	return TreeActionDTO{}
}

// ListEditorsForTree はツリーのファイルをエディタで開くための一覧を返す（IMP-310, IMP-196, FR-036 の
// Open in editor..., FR-091）。
//
// 検査は WantText（**画像はここで拒まれる**。エディタで開くのは文字のファイルだけ。FR-036）。通った
// ファイルを editorTarget に控え、ListEditors と同じ一覧に開くファイルの名前（FileName）を載せる。
// **検査を通らなければ一覧を空で返す**（フロントエンドはウィンドウを出さない）。ErrMissing は not-found、
// ほかの失敗は permission を Error に載せる。**控えはどちらの場合も作り直す**（前の控えを残さない）。
// 回復したパニックはエディタのメソッドと同じく editor-failed とする（IMP-310）。
func (a *App) ListEditorsForTree(path string) (res EditorListDTO) {
	defer recoverEditorList(&res)

	checked, err := a.checkTree(path, filetree.WantText, false)
	if err != nil {
		list := a.listEditorsFor("")
		list.Error = newTreeCheckErrorDTO(path, err)
		return list
	}

	return a.listEditorsFor(checked)
}

// checkTree はいまのツリールートと表示する種類で filetree.Check を呼ぶ（IMP-135, IMP-196）。
//
// ツリールートと表示する種類は mu の内側で読み、検査（ファイルシステムに触れる）は mu の外で行う。
func (a *App) checkTree(path string, want filetree.Want, allowRoot bool) (string, error) {
	a.mu.Lock()
	root, show := a.treeRoot, treeShowOf(a.cfg.TreeShowDiagrams, a.cfg.TreeShowImages)
	a.mu.Unlock()

	return filetree.Check(root, path, show, want, allowRoot)
}

// treeShow は App.cfg のツリーのフィルターから表示する種類を作る（IMP-134, IMP-196）。
func (a *App) treeShow() filetree.Show {
	a.mu.Lock()
	defer a.mu.Unlock()

	return treeShowOf(a.cfg.TreeShowDiagrams, a.cfg.TreeShowImages)
}

// treeShowOf は設定のフィルター 2 つを filetree.Show へ写す（FR-037）。Markdown は常に表示する。
func treeShowOf(diagrams, images bool) filetree.Show {
	return filetree.Show{Diagrams: diagrams, Images: images}
}

// moveTreeRoot はツリールートを root へ変え、TreeActionDTO を返す（IMP-196, FR-030）。
//
// **表示中の文書・表示履歴・編集モード・監視・target は変えない**（文書の切り替えではない。FR-030）。
// 比較は session.SamePath で行い、同じなら何もしない（setTreeRoot）。戻り値に新しいルートと、
// **画面の対象の表示用パス**（session.DisplayPath(新しいルート, target)。FR-052）を載せる。
// **パスをフロントエンドで相対化させない**（IMP-300 の 2）。ツリールートは保存しない（NFR-042）。
func (a *App) moveTreeRoot(root string) TreeActionDTO {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.setTreeRoot(root) {
		return TreeActionDTO{}
	}

	res := TreeActionDTO{Root: a.treeRoot}
	if a.target != "" {
		res.DisplayPath, res.OutsideTree = session.DisplayPath(a.treeRoot, a.target)
	}
	return res
}

// parentOf はディレクトリの親を返す。根（filepath.Dir が自分自身を返す）と空文字では ok が偽
// （IMP-196, UI-032）。ファイルシステムに触れない。
func parentOf(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}

	dir = filepath.Clean(dir)
	parent := filepath.Dir(dir)
	if parent == dir {
		return "", false
	}
	return parent, true
}

// openDirResult はディレクトリをファイルマネージャで開き、TreeActionDTO を返す（IMP-173, IMP-196）。
//
// **起動の失敗（opener.ErrNotFound / opener.ErrNotDir / 起動できない）は open-failed**（IMP-315）。
// Path には開こうとしたディレクトリの絶対パスを入れる（実行ファイルのパスは入らない）。
func openDirResult(dir string) TreeActionDTO {
	if err := opener.OpenDir(dir); err != nil {
		return TreeActionDTO{Error: newOpenFailedDTO(dir, err)}
	}
	return TreeActionDTO{}
}

// recoverTreeAction は、ツリーの操作で回復したパニックを通知せず、「何もしなかった」として返す
// （IMP-022, IMP-196, IMP-310）。
func recoverTreeAction(res *TreeActionDTO) {
	if r := recover(); r != nil {
		applog.Recovered("app.tree", r)
		*res = TreeActionDTO{}
	}
}
