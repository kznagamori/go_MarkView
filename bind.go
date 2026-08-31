package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
// 想定外の入力で落ちても、アプリケーションごと終了させない。

// errNotPending は確認待ちでないパスに OpenConfirmed が呼ばれたことを表す
// （IMP-314, FR-016）。
var errNotPending = errors.New("no pending confirmation for this path")

// recoverBind はパニックをエラーへ変える（IMP-022）。
//
// 戻り値にエラーを持つメソッドで使う。利用者には状態画面の render-error
// として見える（IMP-315）。
func recoverBind(err *error) {
	r := recover()
	if r == nil {
		return
	}

	// TODO(課題 3): IMP-023 の NewLogger() の置き場所が決まったら、
	// 開発モードでスタックトレースを標準エラーへ出す。
	*err = fmt.Errorf("%w: %v", errPanic, r)
}

// recoverOpen はパニックを OpenResultDTO の Error へ変える（IMP-022, IMP-308）。
//
// 分類できないため render-error になり、利用者には状態画面として見える
// （IMP-315）。
func recoverOpen(path string, res *OpenResultDTO) {
	r := recover()
	if r == nil {
		return
	}

	*res = OpenResultDTO{Error: newErrorDTO(path, fmt.Errorf("%w: %v", errPanic, r))}
}

// recoverLink はパニックを LinkResultDTO の Error へ変える（IMP-022, IMP-305）。
func recoverLink(href string, res *LinkResultDTO) {
	r := recover()
	if r == nil {
		return
	}

	*res = LinkResultDTO{Kind: linkError, Error: newErrorDTO(href, fmt.Errorf("%w: %v", errPanic, r))}
}

// recoverQuiet はパニックを握りつぶす（IMP-022）。
//
// エラーを返せないメソッドで使う。設定の更新やスクロール位置の記録が
// 失敗しても、利用者の操作を妨げる理由はない。
func recoverQuiet() {
	_ = recover()
}

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

	return state
}

// OpenFileDialog は OS 標準のファイル選択ダイアログを開く（IMP-310, FR-010）。
//
// キャンセルされた場合は nil を返し、表示中の内容を変更しない。
func (a *App) OpenFileDialog() (res OpenResultDTO) {
	defer recoverOpen("", &res)

	path, err := runtime.OpenFileDialog(a.context(), runtime.OpenDialogOptions{
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
	defer recoverOpen(path, &res)

	return a.openResult(path, openRequest{path: path, src: openFromTree})
}

// OpenConfirmed は確認画面の「Open anyway」を実行する（IMP-310, FR-016）。
//
// **直前に確認画面を出したパスに対してのみ有効とする**（IMP-314）。それ以外を
// 渡された場合は拒否する。任意のサイズのファイルを無条件に開く経路を作らない。
//
// ツリールートと履歴は確認画面を出した時点で反映済みのため、ここでは動かさない
// （openFromConfirm。IMP-192）。
func (a *App) OpenConfirmed(path string) (res OpenResultDTO) {
	defer recoverOpen(path, &res)

	a.mu.Lock()
	pending := a.pendingConfirm
	a.mu.Unlock()

	if pending == "" || !session.SamePath(pending, path) {
		return newOpenResult(path, nil, fmt.Errorf("%s: %w", path, errNotPending))
	}

	return a.openResult(pending, openRequest{path: pending, src: openFromConfirm, confirmed: true})
}

// FollowLink は本文中のリンクを処理する（IMP-310, FR-050, FR-053）。
//
// 判定は IMP-312 が定める順序で行う。**失敗も戻り値で伝える。** パニックを
// 回復した場合も Kind == linkError として返すため、error を返さない。
func (a *App) FollowLink(href string) (res LinkResultDTO) {
	defer recoverLink(href, &res)

	return a.followLink(href)
}

// HistoryBack は 1 つ前の文書へ戻る（IMP-310, FR-051）。
//
// 戻れない場合は nil を返す。フロントエンドはボタンを無効化しているが、
// ショートカットからも呼ばれるため Go 側でも端を守る。
func (a *App) HistoryBack() (res OpenResultDTO) {
	defer recoverOpen("", &res)

	a.mu.Lock()
	entry, ok := a.history.Back()
	a.mu.Unlock()

	if !ok {
		return OpenResultDTO{}
	}

	return a.openHistory(entry)
}

// HistoryForward は戻る前の文書へ進む（IMP-310, FR-051）。
func (a *App) HistoryForward() (res OpenResultDTO) {
	defer recoverOpen("", &res)

	a.mu.Lock()
	entry, ok := a.history.Forward()
	a.mu.Unlock()

	if !ok {
		return OpenResultDTO{}
	}

	return a.openHistory(entry)
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

// Reload は表示中のファイルを読み直す（IMP-310, FR-015）。
//
// 表示中の文書がない場合は nil を返す。スクロール位置はフロントエンドが
// 保持している現在値を使う（IMP-321）。
func (a *App) Reload() (res OpenResultDTO) {
	defer recoverOpen("", &res)

	a.mu.Lock()
	current := a.current
	a.mu.Unlock()

	if current == nil {
		return OpenResultDTO{}
	}

	return a.openResult(current.Path, openRequest{path: current.Path, src: openFromReload})
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
// ウィンドウの大きさと最大化状態は ConfigDTO に含まれない。これらは保存の
// 直前に Wails のランタイムから読み出す（IMP-194）。ここで書き換えないため、
// 保存済みの値がフロントエンド経由で失われることはない。
func (a *App) UpdateConfig(patch ConfigDTO) {
	defer recoverQuiet()

	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.Theme = patch.Theme
	a.cfg.Zoom = patch.Zoom
	a.cfg.OutlineVisible = patch.OutlineVisible
	a.cfg.FileTreeVisible = patch.FileTreeVisible
	a.cfg.OutlineWidth = patch.OutlineWidth
	a.cfg.FileTreeWidth = patch.FileTreeWidth

	// 受け取った値も範囲外なら既定値へ戻す（IMP-153）。フロントエンドを
	// 信頼できる入力元として扱わない。
	a.cfg.Normalize()

	a.scheduleSave()
}

// CopyToClipboard はテキストをクリップボードへ書く（IMP-310, FR-061, AR-062）。
//
// WebView の Clipboard API は権限や環境によって使えないことがあるため、
// Go 側を経由する（AR-062）。
func (a *App) CopyToClipboard(text string) (err error) {
	defer recoverBind(&err)

	if err := runtime.ClipboardSetText(a.context(), text); err != nil {
		return fmt.Errorf("%w: %v", errClipboard, err)
	}
	return nil
}

// GetAbout はアプリケーション情報を返す（IMP-310, FR-100, FR-101）。
func (a *App) GetAbout() AboutDTO {
	defer recoverQuiet()

	// TODO(T6-2): licenses/THIRD_PARTY.md を go:embed して渡す
	// （IMP-030, BR-040）。生成スクリプトができるまでは空欄とする。
	//
	// WebView のバージョンは Wails v2 のランタイムが返さないため空文字を
	// 渡す。Environment は当該区画ごと省く（IMP-181）。
	return newAboutDTO("", "")
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

// dropTarget はドロップされたパスから開く対象を選ぶ（IMP-313, FR-011）。
//
// **判定は Go 側で行う**（IMP-300）。複数渡された場合は先頭の Markdown を
// 採り、他は無視する。ディレクトリなら直下の README を探す。対象がなければ
// 空文字を返す。
func dropTarget(paths []string) string {
	for _, p := range paths {
		if mdfile.IsMarkdown(p) {
			return p
		}
	}

	// ディレクトリは 1 つだけ渡された場合に限って扱う。複数のディレクトリから
	// 1 つを選ぶ規則を FR-011 は定めていない。
	if root := dropRoot(paths); root != "" {
		if readme, ok := session.FindReadme(root); ok {
			return readme
		}
	}

	return ""
}

// dropRoot はドロップでツリールートにすべき場所を返す（FR-011, FR-030）。
//
// ディレクトリが 1 つだけ落とされた場合に限る。**README が見つからなくても
// ツリールートは移す。** FR-011 の表の 2 行目は「そのディレクトリを新しい
// ツリールートとし、直下に README.md があれば表示する」であり、表示できるか
// どうかとツリーの移動は別である。
//
// ファイルが落とされた場合は空文字を返す。ファイルのツリールートは open が
// 親ディレクトリとして決める（IMP-192）。
func dropRoot(paths []string) string {
	if len(paths) != 1 {
		return ""
	}

	info, err := os.Stat(paths[0])
	if err != nil || !info.IsDir() {
		return ""
	}

	return filepath.Clean(paths[0])
}
