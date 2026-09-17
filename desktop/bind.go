package desktop

import (
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/kznagamori/go_MarkView/internal/filetree"
	"github.com/kznagamori/go_MarkView/internal/mdfile"
	"github.com/kznagamori/go_MarkView/internal/ostheme"
	"github.com/kznagamori/go_MarkView/internal/session"
)

// 本ファイルはフロントエンドへ公開するメソッドを持つ（IMP-310）。
//
// **フロントエンドから任意のパスを開く汎用メソッドを定義しない**（IMP-300）。
// ファイルを開く経路は、ダイアログ・ドロップ・引数・ツリー・リンク・履歴の
// 6 つに限り、いずれも open（IMP-192）を通る。
//
// **各メソッドの入口で recover する**（IMP-022, FR-111）。goldmark 拡張や
// 想定外の入力で落ちても、アプリケーションごと終了させない。回復の関数は recover.go にある。
//
// **Wails のランタイムを ctx が nil のまま呼ばない。** ランタイムは nil を受け取ると
// log.Fatalf でプロセスを終わらせ、recover では止められない（Wails v2.15.0 の getFrontend）。

// GetInitialState は起動直後の状態をまとめて返す（IMP-310, FR-012, FR-013）。
//
// **起動時の表示対象が読み込めない場合もウィンドウは開いている。** ここでは
// 状態画面の種別（IMP-193 の表）を返し、フロントエンドは通常の状態画面と
// 同じ処理で描画する（IMP-250）。起動経路のためだけの専用画面を作らない。
func (a *App) GetInitialState() InitialStateDTO {
	defer recoverQuiet()

	a.mu.Lock()
	cfg, initial, startupErr := a.cfg, a.startup.Initial, a.startupErr
	a.mu.Unlock()

	state := InitialStateDTO{
		Config:    newConfigDTO(cfg, a.resolvedTheme()),
		StateKind: stateWelcome,
	}

	switch {
	case startupErr != nil:
		// 引数のパスが存在しない・読めない。**ウィンドウは既に開いている**
		// （FR-012）。操作案内を出したうえで、理由をステータスへ添える。
		state.Error = newErrorDTO(a.startup.Requested, startupErr)
		state.StateKind = stateKindFor(state.Error)

	case initial != "":
		dto, err := a.open(openRequest{path: initial, src: openFromArgs})
		if err != nil {
			state.Error = newErrorDTO(initial, err)
			state.StateKind = stateKindFor(state.Error)
			break
		}
		state.Document = dto
		state.StateKind = stateNone
	}

	// ツリールートは open のあとに読む。引数がファイルだった場合、そこで
	// 親ディレクトリへ移っている（FR-030）。
	state.TreeRoot = a.GetTreeRoot()

	// **画面の対象（target。IMP-190）は open が設定している。** 起動時も規則は
	// IMP-192 とまったく同じであり（IMP-193）、ここで別に設定しない。
	//
	// startupErr の経路だけは open を通らないが、引数のパスが見つからない・
	// 読めないという失敗であり、stateKindFor は必ず welcome を返す。welcome
	// では対象を持たないため、空のままで正しい（IMP-193 の表）。
	//
	// 仮にこの前提が崩れて「状態画面なのに target が空」になっても、
	// エディタで開く操作が使えないだけで、**別のファイルが開くことはない**。
	// 安全な側に倒れる（NFR-035 の 2）。

	return state
}

// OpenFileDialog は OS 標準のファイル選択ダイアログを開く（IMP-310, FR-010）。
//
// キャンセルされた場合は nil を返し、表示中の内容を変更しない。
func (a *App) OpenFileDialog() (res OpenResultDTO) {
	var path string
	defer a.recoverOpen(&path, &res)

	ctx := a.context()
	if ctx == nil {
		return OpenResultDTO{}
	}

	path, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		// 初期ディレクトリは現在のツリールート。未確定なら空文字を渡し、
		// OS の既定（カレントディレクトリ）に任せる（FR-010）。
		DefaultDirectory: a.GetTreeRoot(),
		Filters: []runtime.FileFilter{
			// フィルタ名は英語とする（UI-024）。パターンは mdfile の一覧から
			// 組み立て、拡張子の定義を 2 か所に置かない（IMP-105）。
			{DisplayName: "Markdown files", Pattern: markdownFilterPattern()},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
	// **ダイアログを出せなかった場合も「何も選ばれなかった」として扱う。**
	// 表示中の文書を状態画面で置き換える理由がない（FR-110）。
	if err != nil || path == "" {
		return OpenResultDTO{}
	}

	return a.openResult(path, openRequest{path: path, src: openFromDialog})
}

// OpenFromTree はファイルツリーで選ばれたファイルを開く（IMP-310, FR-033）。
//
// **ツリールートは変更しない。** 配布されたドキュメント群を閲覧している間に、
// ツリーが利用者の操作で移動してしまうことを防ぐ（FR-030）。
func (a *App) OpenFromTree(path string) (res OpenResultDTO) {
	defer a.recoverOpen(&path, &res)

	return a.openResult(path, openRequest{path: path, src: openFromTree})
}

// OpenConfirmed は確認画面の「Open anyway」を実行する（IMP-310, FR-016）。
//
// **直前に確認画面を出したパスに対してのみ有効とする**（IMP-314）。それ以外を
// 渡された場合は拒否する。任意のサイズのファイルを無条件に開く経路を作らない。
//
// ツリールートと履歴は確認画面を出した時点で反映済みのため、ここでは動かさない
// （openFromConfirm。IMP-192）。
//
// **拒否は「何も起きなかった」として返す**（IMP-308, IMP-314）。起きるのは、Open anyway の
// 二度押し（1 回目で本文が出た後）と、確認画面の間に確認以外の失敗で確認待ちが消えた後である。
// v1.0.0 は render-error を返しており、二度押しで出たばかりの本文が状態画面に置き換わり、Go 側は
// 文書を表示中のまま食い違っていた。確認待ちが消えた後は、F5 で確認画面を出し直せる（Reload は
// 画面の対象を読み直す）。
func (a *App) OpenConfirmed(path string) (res OpenResultDTO) {
	// 画面の対象にするのは、フロントエンドから受け取った値ではなく Go 側が持つ確認待ちのパス。
	var target string
	defer a.recoverOpen(&target, &res)

	pending := a.pendingConfirmPath()
	if pending == "" || !session.SamePath(pending, path) {
		return OpenResultDTO{}
	}
	target = pending

	return a.openResult(pending, openRequest{path: pending, src: openFromConfirm, confirmed: true})
}

// pendingConfirmPath は確認待ちのパスを返す（IMP-314）。
func (a *App) pendingConfirmPath() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.pendingConfirm
}

// FollowLink は本文中のリンクを処理する（IMP-310, FR-050, FR-053）。
//
// 判定は IMP-312 が定める順序で行う。**失敗も戻り値で伝える。** パニックを
// 回復した場合も Kind == linkError として返すため、error を返さない。
func (a *App) FollowLink(href string) (res LinkResultDTO) {
	defer a.recoverLink(href, &res)

	return a.followLink(href)
}

// HistoryBack は 1 つ前の文書へ戻る（IMP-310, FR-051）。
//
// 戻れない場合は nil を返す。フロントエンドはボタンを無効化しているが、
// ショートカットからも呼ばれるため Go 側でも端を守る。
func (a *App) HistoryBack() (res OpenResultDTO) {
	var path string
	defer a.recoverOpen(&path, &res)

	entry, ok := a.historyStep((*session.History).Back)
	if !ok {
		return OpenResultDTO{}
	}
	path = entry.Path

	return a.openHistory(entry)
}

// HistoryForward は戻る前の文書へ進む（IMP-310, FR-051）。
func (a *App) HistoryForward() (res OpenResultDTO) {
	var path string
	defer a.recoverOpen(&path, &res)

	entry, ok := a.historyStep((*session.History).Forward)
	if !ok {
		return OpenResultDTO{}
	}
	path = entry.Path

	return a.openHistory(entry)
}

// historyStep は mu の内側で履歴を 1 つ動かす（FR-051）。
//
// **錠は defer で解く。** 途中でパニックが起きても、回復（openPanicked）が錠を取れるようにする。
func (a *App) historyStep(step func(*session.History) (session.Entry, bool)) (session.Entry, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return step(a.history)
}

// openHistory は履歴のエントリを開く（FR-051, IMP-192）。
func (a *App) openHistory(entry session.Entry) OpenResultDTO {
	return a.openResult(entry.Path, openRequest{
		path:      entry.Path,
		src:       openFromHistory,
		anchor:    entry.Anchor,
		scrollTop: entry.ScrollTop,
	})
}

// Reload は画面の対象（target）を読み直す（IMP-310, FR-015）。
//
// **表示中の文書（current）を開き直さない。** 状態画面の間はその対象をもう一度開く
// （confirm-large なら確認画面、render-error なら変換をやり直す）。v1.0.0 は current を開き
// 直しており、状態画面を見ながら F5 を押すと画面に無い前の文書が表示された（BUG-012）。
//
// 画面の対象が無い（文書未表示）なら何もしない（IMP-308）。スクロール位置はフロントエンドが
// 保持している現在値を使う（IMP-321）。確認して開いた文書の同意は開く処理が引き継ぐ（FR-016）。
func (a *App) Reload() (res OpenResultDTO) {
	var target string
	defer a.recoverOpen(&target, &res)

	// **target を読むところから ioMu の内側で行う。** 錠の外で読むと、読み直すまでの間に別の
	// 文書を開く処理が済み、画面に無くなった古い対象を読み直してしまう（IMP-190）。
	// defer は後に置いたものから動くため、パニックのときは錠を解いてから recoverOpen が動く。
	a.ioMu.Lock()
	defer a.ioMu.Unlock()

	target = a.screenTarget()
	if target == "" {
		return OpenResultDTO{}
	}

	dto, err := a.openLocked(openRequest{path: target, src: openFromReload})

	return newOpenResult(target, dto, err)
}

// screenTarget は画面の対象を返す（IMP-190）。
func (a *App) screenTarget() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.target
}

// ReadDir はディレクトリの直下を読む（IMP-310, FR-032, FR-035）。
//
// 再帰しない。展開のたびに呼ばれる（FR-032 の遅延展開）。空文字を渡された
// 場合はツリールートを読む。
func (a *App) ReadDir(path string) (nodes []TreeNodeDTO, err error) {
	defer recoverBind(&err)

	if path == "" {
		path = a.GetTreeRoot()
	}
	if path == "" {
		return []TreeNodeDTO{}, nil
	}

	read, err := filetree.ReadDir(path)
	if err != nil {
		return nil, err
	}

	return newTreeNodeDTOs(read), nil
}

// GetTreeRoot はツリールートの絶対パスを返す（IMP-310, FR-030）。
// 未確定なら空文字を返す。
func (a *App) GetTreeRoot() string {
	defer recoverQuiet()

	a.mu.Lock()
	defer a.mu.Unlock()

	return a.treeRoot
}

// SetScrollTop は現在のスクロール位置を履歴へ記録する（IMP-310, IMP-311）。
//
// フロントエンドは**文書を離れる直前に 1 回だけ**呼ぶ。スクロールのたびには
// 呼ばない。
func (a *App) SetScrollTop(top int) {
	defer recoverQuiet()

	a.mu.Lock()
	defer a.mu.Unlock()

	a.history.SetScrollTop(top)
}

// UpdateConfig は設定を更新する（IMP-310, UI-110, UI-114）。
//
// ウィンドウの大きさは ConfigDTO に含まれない。保存の直前に Wails のランタイム
// から読み出す（IMP-194）。ここで書き換えないため、保存済みの値がフロントエンド
// 経由で失われることはない。
//
// 表示倍率と最大化状態は保存しないため、そもそも受け取る先がない
// （UI-111, UI-115, IMP-150）。
func (a *App) UpdateConfig(patch ConfigDTO) {
	defer recoverQuiet()

	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.Theme = patch.Theme
	a.cfg.OutlineVisible = patch.OutlineVisible
	a.cfg.FileTreeVisible = patch.FileTreeVisible
	a.cfg.OutlineWidth = patch.OutlineWidth
	a.cfg.FileTreeWidth = patch.FileTreeWidth

	// 受け取った値も範囲外なら既定値へ戻す（IMP-153）。フロントエンドを
	// 信頼できる入力元として扱わない。
	a.cfg.Normalize()

	a.scheduleSave()
}

// GetAbout はアプリケーション情報を返す（IMP-310, FR-100, FR-101）。
func (a *App) GetAbout() AboutDTO {
	defer recoverQuiet()

	// WebView の版は OS ごとに取り方が違うため、Wails との境界（このパッケージ）で解決する
	// （webview_windows.go / webview_other.go。IMP-181）。**取得に失敗した
	// ときだけ空文字になり**、Environment が当該区画ごと省く。
	return newAboutDTO(a.licenses, webviewVersion())
}

// Quit はアプリケーションを終了する（IMP-310, UI-090）。
//
// Ctrl+Q の受け口である。Alt+F4 と閉じるボタンは OS とウィンドウマネージャが
// 処理するが、Ctrl+Q はアプリケーション側で受けるほかない。
//
// **終了処理そのものは Wails に任せる。** ここで設定を保存しない。
// OnBeforeClose / OnShutdown の経路（IMP-194）を通ることで、閉じるボタンで
// 終了した場合とまったく同じ後始末になる。
func (a *App) Quit() {
	defer recoverQuiet()

	runtime.Quit(a.context())
}

// resolvedTheme は OS 設定への追従まで解決したテーマを返す（FR-071, IMP-303）。
//
// 設定に light / dark が入っていればそれを使う。空文字は「まだ利用者が
// 選んでいない」ことを表し、このときだけ OS の設定へ追従する。
//
// **Wails v2 のランタイムには OS のテーマを取得する API がない**ため、
// ostheme が OS ごとの設定を直接読む（IMP-175）。読めなかった場合は
// FR-071 の「判定できない場合は Light テーマとする」に従う。
//
// ostheme.Detect は Linux で外部コマンドを起動するが、ここへ来るのは設定に
// テーマが記録されていないときだけであり、通常は初回起動の 1 回に限られる。
func (a *App) resolvedTheme() string {
	a.mu.Lock()
	theme := a.cfg.Theme
	a.mu.Unlock()

	if theme == "light" || theme == "dark" {
		return theme
	}

	if detected := ostheme.Detect(); detected != ostheme.Unknown {
		return detected
	}

	return "light"
}

// markdownFilterPattern はダイアログのフィルタ文字列を組み立てる（FR-010）。
//
//	*.md;*.markdown;*.mdown;*.mkd
//
// 拡張子の定義は mdfile が唯一の正である（IMP-105）。ここに並べ直さない。
func markdownFilterPattern() string {
	patterns := make([]string, 0, len(mdfile.Extensions))
	for _, ext := range mdfile.Extensions {
		patterns = append(patterns, "*"+ext)
	}

	return strings.Join(patterns, ";")
}
