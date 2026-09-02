package main

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/kznagamori/go_MarkView/internal/config"
	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/renderer"
	"github.com/kznagamori/go_MarkView/internal/session"
	"github.com/kznagamori/go_MarkView/internal/watcher"
)

// saveDebounce は設定保存を待つ時間（UI-114）。
const saveDebounce = time.Second

// Go からフロントエンドへ送るイベント名（IMP-320）。
//
// 一方向の通知であり、フロントエンドは起動時に 1 度だけ購読する（IMP-322）。
const (
	eventDocumentOpened  = "document:opened"  // 呼び出し以外で表示対象が変わった（FR-011）
	eventDocumentChanged = "document:changed" // 表示中ファイルの更新を検知した（FR-014）
	eventDocumentRemoved = "document:removed" // 表示中ファイルが削除された（FR-014）
	eventTreeRootChanged = "tree:root-changed"
	eventError           = "error" // 非同期処理で発生したエラー（FR-110）
)

// App は Wails にバインドする唯一の型である（IMP-190）。
//
// Wails の API を呼べるのは main.go と本ファイル群（app.go / open.go）だけとし、
// internal/ の各パッケージは Wails に依存させない（IMP-012）。また、判断を伴う
// ロジックはここに置かず internal/session などへ委ねる。ここに書いたロジックは
// package main のテストとなり、テストバイナリに Wails（Linux では cgo と
// WebKitGTK）がリンクされてしまうためである（UT-002）。
//
// **状態の変更はすべて mu で保護する**（IMP-024）。バインドメソッドは複数の
// ゴルーチンから同時に呼ばれうる。
type App struct {
	ctx context.Context
	mu  sync.Mutex

	renderer *renderer.Renderer
	watcher  *watcher.Watcher
	cfg      config.Config

	treeRoot string             // ツリールートの絶対パス（FR-030）
	current  *document.Document // 表示中の文書。未表示なら nil
	history  *session.History   // 表示履歴（IMP-191）

	// pendingConfirm は確認画面を表示中のファイル（FR-016）。
	//
	// OpenConfirmed が受け付ける対象をこの 1 つに限定するために保持する
	// （IMP-314）。任意のサイズのファイルを無条件に開く経路を作らない。
	pendingConfirm string

	// pendingSource は確認画面を出したときの経路（IMP-192, IMP-314）。
	//
	// OpenConfirmed で開き直すとき、ツリールートと履歴を二重に動かさない
	// ために保持する。確認の時点でどちらも反映済みである。
	pendingSource openSource

	// startup は起動時に決定した表示対象とツリールート（IMP-193）。
	startup session.Startup

	// startupErr は起動時の対象解決で起きた失敗（IMP-193 の表）。
	//
	// **ウィンドウは必ず開く**（FR-012）。ここに残しておき、フロントエンドが
	// GetInitialState を呼んだ時点でステータス表示へ渡す。
	startupErr error

	// saveTimer は設定保存のデバウンス（UI-114）。
	//
	// 変更のたびに書くと、ペイン幅のドラッグ中に何十回も書き込むことになる。
	saveTimer *time.Timer
}

// NewApp は App を生成する（IMP-193）。
//
// startup と cfg は main.go が解決した結果を渡す。cfg をここで読まないのは、
// ウィンドウの初期サイズ（UI-011）が Wails の起動オプションとして必要であり、
// main.go が先に持っていなければならないためである。
//
// **ファイルパス・履歴・ツリールートをディスクへ書き出す経路を持たない**
// （NFR-042）。config.Config にそれらのフィールドが存在しないことで構造的に
// 保証している（IMP-150）。
func NewApp(startup session.Startup, startupErr error, cfg config.Config) *App {
	return &App{
		startup:    startup,
		startupErr: startupErr,
		cfg:        cfg,
		renderer:   renderer.New(),
		history:    session.NewHistory(),
		treeRoot:   startup.TreeRoot,
	}
}

// onStartup は Wails がウィンドウ生成後に呼ぶ。
func (a *App) onStartup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	// ウィンドウはプライマリモニタの作業領域の中央に置く。位置は保存も
	// 復元もしない（UI-011, UI-111）。モニタ構成が変わっても画面外に
	// 出ないようにするための規定である。
	runtime.WindowCenter(ctx)

	// ドロップの受け口はバインドメソッドではなくコールバックである
	// （IMP-313）。HTML5 の DataTransfer からは絶対パスを得られない。
	runtime.OnFileDrop(ctx, func(_, _ int, paths []string) { a.onFileDrop(paths) })

	a.startWatcher(ctx)
}

// onBeforeClose は Wails がウィンドウを閉じる直前に呼ぶ（IMP-194）。
//
// **ウィンドウの大きさはここで取り込む。** onShutdown の時点でウィンドウは
// 既に破棄されており、WindowGetSize は DPI を 0 として除算し panic する
// （2026-08-31 に実機で確認）。
//
// false を返して閉じる操作をそのまま通す。確認を挟まない（FR-110）。
func (a *App) onBeforeClose(_ context.Context) bool {
	a.captureWindowState()
	return false
}

// onShutdown は Wails が終了時に呼ぶ（IMP-194）。
//
// **ここで Wails のランタイムを呼ばない。** ウィンドウは既に破棄されている。
//
// **失敗しても終了を妨げない。** 設定が書けなくても、次回は既定値で起動する
// だけである（UI-113）。
func (a *App) onShutdown(_ context.Context) {
	a.mu.Lock()
	if a.saveTimer != nil {
		a.saveTimer.Stop()
		a.saveTimer = nil
	}
	w := a.watcher
	a.mu.Unlock()

	if w != nil {
		_ = w.Close()
	}

	// 大きさは onBeforeClose で取り込み済みである。
	a.persistConfig()
}

// saveConfig はウィンドウの状態を取り込んでから設定を保存する（IMP-194）。
//
// **ウィンドウが生きている間にだけ呼ぶ。** 保存の予約（UI-114）から使う。
func (a *App) saveConfig() {
	a.captureWindowState()
	a.persistConfig()
}

// persistConfig は現在の設定をディスクへ書く（UI-112, IMP-152）。
//
// 履歴・表示中パス・ツリールートは保存しない（NFR-042）。構造体にフィールドが
// 存在しないため、ここで気をつける必要はない（IMP-150）。
func (a *App) persistConfig() {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()

	_ = config.Save(cfg)
}

// captureWindowState はウィンドウの大きさを設定へ取り込む（IMP-194, UI-110）。
//
// これはフロントエンドから通知されない（ConfigDTO に含まれない。IMP-303）
// ため、保存の直前に Wails のランタイムから読み出す。
//
// **最大化中のサイズは取り込まない。** 最大化中は画面いっぱいの値が返るため、
// 保存すると次回のウィンドウが画面いっぱいの大きさで開く。幅と高さは最大化
// する前の値を保つ。
//
// 最大化しているかどうかも読むが、これは**保存しないと決めるための判定**で
// あって、保存する値ではない。最大化状態そのものを保存しないため、次回は常に
// 通常状態で開く（UI-111, UI-115）。
//
// ウィンドウ位置と最大化状態は保存しない。構造体にフィールドが存在しない
// （IMP-150, UI-111）。
func (a *App) captureWindowState() {
	// ウィンドウの状態を読む API は、ウィンドウの生存に依存する。取りこぼしても
	// 保存そのものは続ける（FR-111, IMP-022）。
	defer recoverQuiet()

	ctx := a.context()
	if ctx == nil {
		return
	}

	if runtime.WindowIsMaximised(ctx) {
		return
	}

	width, height := runtime.WindowGetSize(ctx)

	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.WindowWidth, a.cfg.WindowHeight = width, height
}

// scheduleSave は設定の保存を予約する（UI-114）。
//
// **呼び出し側は mu を保持していること。**
//
// 変更から 1 秒間さらに変更がなければ書く。ペイン幅のドラッグ中は変更が
// 続くため、実際に書かれるのはドラッグを終えたあとになる。
func (a *App) scheduleSave() {
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}

	a.saveTimer = time.AfterFunc(saveDebounce, a.saveConfig)
}

// startWatcher はファイル監視を開始する（FR-014, IMP-140, IMP-024）。
//
// 監視は ctx の Done で終わる。ゴルーチンをアプリのライフサイクルへ紐付け、
// 終了時に必ず止める（NFR-020）。
//
// 監視を作れなくても起動は続ける。自動更新が効かなくなるだけで、利用者は
// 再読み込みできる（FR-015, FR-111）。
func (a *App) startWatcher(ctx context.Context) {
	w, err := watcher.New(ctx)
	if err != nil {
		return
	}

	a.mu.Lock()
	a.watcher = w
	current := a.current
	a.mu.Unlock()

	// onStartup より前に文書を開いていた場合、その時点では watcher が
	// なく監視を張れていない。ここで追いつかせる。
	if current != nil {
		_ = w.Watch(current.Path)
	}

	go a.consumeWatchEvents(w)
}

// consumeWatchEvents は監視イベントをフロントエンドへ流す（FR-014, IMP-320）。
//
// チャネルは監視の終了時に閉じるため、range で待てる（IMP-140）。
func (a *App) consumeWatchEvents(w *watcher.Watcher) {
	for ev := range w.Events() {
		switch ev.Kind {
		case watcher.Modified:
			a.onCurrentModified(ev.Path)
		case watcher.Removed:
			a.onCurrentRemoved(ev.Path)
		}
	}
}

// onCurrentModified は表示中ファイルの更新を反映する（FR-014, IMP-321）。
//
// **スクロール位置はフロントエンドが持つ現在値を使う**（keep）。再描画に
// 失敗した場合はエラーだけを送り、直前の描画結果は残す（FR-110）。
func (a *App) onCurrentModified(path string) {
	dto, err := a.open(openRequest{path: path, src: openFromReload})
	if err != nil {
		a.emit(eventError, newErrorDTO(path, err))
		return
	}

	a.emit(eventDocumentChanged, dto)
}

// onCurrentRemoved は表示中ファイルの削除を伝える（FR-014, FR-110）。
//
// **直前の描画結果は保持する。** 本文を消すと、編集の途中でファイルが一瞬
// 消えるような保存方式のたびに画面が空になる。
func (a *App) onCurrentRemoved(path string) {
	a.emit(eventDocumentRemoved, removedErrorDTO(path))
}

// onFileDrop はドロップされたパスを処理する（IMP-313, FR-011）。
//
// 結果は document:opened で送る。フロントエンドの呼び出しではなく OS の
// 操作で表示対象が変わるためである（IMP-320）。
func (a *App) onFileDrop(paths []string) {
	if root := dropRoot(paths); root != "" {
		a.mu.Lock()
		changed := a.setTreeRoot(root)
		newRoot := a.treeRoot
		a.mu.Unlock()

		if changed {
			a.emit(eventTreeRootChanged, newRoot)
		}
	}

	target := dropTarget(paths)
	if target == "" {
		// Markdown でもディレクトリでもない。ツリーも本文も変えず、
		// 対応していない旨をステータスへ出す（FR-011 の表の 4 行目）。
		a.emit(eventError, newErrorDTO(firstPath(paths), document.ErrNotMarkdown))
		return
	}

	dto, err := a.open(openRequest{path: target, src: openFromDrop})
	if err != nil {
		a.emit(eventError, newErrorDTO(target, err))
		return
	}

	a.emit(eventDocumentOpened, dto)
}

// firstPath は一覧の先頭を返す。空なら空文字。
func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// emit はフロントエンドへイベントを送る（IMP-320）。
//
// ウィンドウが生成される前（ctx が nil）は何もしない。購読側がいない時点で
// 送っても届かず、落ちる理由もない（FR-111）。
func (a *App) emit(name string, payload any) {
	ctx := a.context()
	if ctx == nil {
		return
	}

	runtime.EventsEmit(ctx, name, payload)
}

// setWindowTitle はウィンドウタイトルを更新する（UI-013）。
//
// **フロントエンドからは変えられない。** WebView の document.title を
// ネイティブのウィンドウタイトルへ反映する仕組みを Wails v2 は持たないため、
// 表示対象が変わるたびに Go 側から呼ぶ。
//
// name が空ならアプリケーション名だけにする（文書未表示時。UI-013）。
func (a *App) setWindowTitle(name string) {
	ctx := a.context()
	if ctx == nil {
		return
	}

	title := AppTitle
	if name != "" {
		title = name + " - " + AppTitle
	}

	runtime.WindowSetTitle(ctx, title)
}

// context は Wails のコンテキストを返す。ウィンドウ生成前は nil。
func (a *App) context() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.ctx
}

// setTreeRoot はツリールートを変更し、変わったときだけ true を返す
// （FR-030）。
//
// **呼び出し側は mu を保持していること。**
//
// 比較は session.SamePath で行う。Windows では大文字小文字が違うだけの
// パスを別のルートとみなさない（IMP-025）。単純な != で比べると、同じ場所を
// 指しているのに tree:root-changed を送り、ツリーが無用に組み直される。
func (a *App) setTreeRoot(root string) bool {
	root = filepath.Clean(root)
	if session.SamePath(a.treeRoot, root) {
		return false
	}

	a.treeRoot = root
	return true
}

// watchCurrent は監視対象を表示中の文書へ切り替える（FR-014, IMP-140）。
//
// **呼び出し側は mu を保持していること。**
//
// 監視は常に 1 つ以下とする。Watch が切り替えまで面倒を見るため、ここで
// Unwatch を挟まない。失敗しても開く操作は成功とする。自動更新が効かなく
// なるだけで、利用者は再読み込みできる（FR-015, FR-111）。
func (a *App) watchCurrent(path string) {
	// TODO(T3-11): onStartup で watcher を生成したら常に非 nil になる。
	if a.watcher == nil {
		return
	}
	_ = a.watcher.Watch(path)
}
