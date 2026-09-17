package desktop

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

// App は Wails にバインドする唯一の型である（IMP-190）。判断を伴うロジックを置かない（desktop.go）。
//
// **状態の変更はすべて mu で保護する**（IMP-024）。バインドメソッドは複数の
// ゴルーチンから同時に呼ばれうる。**edit だけは例外で、ioMu で保護する**（IMP-190）。
//
// **錠を取る順序は常に ioMu → mu とする。** 逆にするとデッドロックする（IMP-024）。
// ioMu は文書を開く処理（IMP-192）・編集モードの切り替えと書き込み（IMP-195）・削除の
// 受け取り（IMP-195）の全体を包む。**sync.Mutex は再入できない**——ioMu を持ったまま
// 文書を開くときは、錠を取らない openLocked を呼ぶ（IMP-192）。
type App struct {
	ctx context.Context
	mu  sync.Mutex

	renderer *renderer.Renderer
	watcher  *watcher.Watcher
	cfg      config.Config
	licenses string // OSS ライセンス一覧（FR-101）。main.go が埋め込んだものを受け取る

	treeRoot string             // ツリールートの絶対パス（FR-030）
	current  *document.Document // 表示中の文書。未表示なら nil
	history  *session.History   // 表示履歴（IMP-191）

	// target は画面がいま対象にしているファイルの絶対パス（IMP-190）。
	//
	// 本文を表示していればその文書、状態画面を出していればその対象。文書
	// 未表示（welcome）なら空。「エディタで開く」（FR-090）と**再読み込み**（FR-015,
	// IMP-310）が使う。**状態画面の間の `F5` は、current ではなくこの対象を読み直す**
	// （v1.0.0 は current を読み直していた。BUG-012）。
	//
	// **current と取り違えない。** current は「読み込みと変換に成功した文書」
	// であり、状態画面（confirm-large / too-large / render-error）を出して
	// いる間は**前に開いていた文書のまま残る**。一方 target は画面が示して
	// いる対象であり、**ウィンドウタイトル（UI-013）と常に一致する。**
	//
	// ここで current を渡すと、利用者が `big.md — too large` の画面を見ながら
	// 押したのに前の文書がエディタで開く。**エラーも出ないため気づけない**
	// （NFR-035 の 2）。
	//
	// **更新するのは開く処理（openLocked の commit）だけである**（IMP-192）。判定は
	// classifyOpen に 1 つだけ置き、ウィンドウタイトルも同じ値から決める。
	target string

	// showing は、画面が current を表示しているか（IMP-190, IMP-192）。
	//
	// 状態画面（confirm-large / too-large / render-error）と文書未表示では偽。
	// **target と current.Path を比べて代わりにしない。** 表示中の文書を読み直したら
	// 50 MB を超えていた場合、状態画面の対象は current と同じファイルになる。
	//
	// 使う場面は 3 つある。DocumentDTO.SameDocument（直前が状態画面なら同じファイルでも
	// 偽）、監視のイベントの照合（状態画面の間のイベントは読み直さずに捨てる。BUG-012）、
	// 編集モードの判断（EditSession.CanStart / Check。IMP-109）。
	showing bool

	// currentConfirmed は、current を確認画面の Open anyway で開いたか、その同意を
	// 引き継いで読み直したか（FR-016, IMP-192）。
	//
	// 同じファイルの読み直し（監視・再読み込み・書き込みの後）で LoadOptions.Confirmed を
	// 渡すために持つ。**持たないと、確認して開いた 10 MB 超の文書が保存や F5 のたびに
	// 確認画面へ戻り、書き込みの直後の読み直しで編集モードが黙って終わる**（FR-140, FR-143）。
	currentConfirmed bool

	// pendingConfirm は確認画面を表示中のファイル（FR-016）。
	//
	// OpenConfirmed が受け付ける対象をこの 1 つに限定するために保持する
	// （IMP-314）。任意のサイズのファイルを無条件に開く経路を作らない。
	pendingConfirm string

	// pendingEditor は BrowseEditor で選ばれた「確定前の候補」（IMP-310）。
	//
	// OpenInEditor("custom") が受け付ける対象をこの 1 つに限定するために
	// 保持する。pendingConfirm（IMP-314）とまったく同じ考え方であり、
	// **フロントエンドから任意の実行ファイルを起動する経路を作らない**
	// ためのものである（IMP-300 の 3, NFR-035）。
	//
	// **ListEditors が捨てる。** 押すたびに選択ウィンドウを出す設計であり
	// （UI-103）、初期選択は設定に保存されたエディタから決まる。Browse した
	// まま閉じた候補が次に開いたときも残っていると、「閉じた場合は何も
	// 保存しない」（FR-091）が破れて見える。
	pendingEditor string

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

	// ioMu は表示中ファイルの読み直しと書き込みを 1 つずつ順に行うための錠（FR-143, IMP-190）。
	//
	// **mu だけでは、読み直しの途中に書き込みが割り込み、自分の書き込みを外部の変更と
	// 取り違える**（FR-143, FR-144）。読み込みと変換は mu の外で行う（IMP-192）が、ioMu の
	// 内側で行う。
	ioMu sync.Mutex

	// edit は編集モードの状態と判断（IMP-109）。Go 側が正（IMP-300 の 4）。
	//
	// **ioMu の内側でだけ触る。** 触る経路（開く処理・編集モードのバインド・削除の受け取り）は
	// すべて ioMu を取るため、mu で包まない（Plan の構文解析の間 mu を持ち続けない）。
	edit document.EditSession
}

// NewApp は App を生成する（IMP-193）。
//
// startup と cfg は main.go が解決した結果を渡す。cfg をここで読まないのは、
// ウィンドウの初期サイズ（UI-011）が Wails の起動オプションとして必要であり、
// main.go が先に持っていなければならないためである。licenses も main.go が渡す——
// go:embed はパッケージのディレクトリより上を参照できない（IMP-030）。
//
// **ファイルパス・履歴・ツリールートをディスクへ書き出す経路を持たない**
// （NFR-042）。config.Config にそれらのフィールドが存在しないことで構造的に
// 保証している（IMP-150）。
func NewApp(startup session.Startup, startupErr error, cfg config.Config, licenses string) *App {
	return &App{
		startup:    startup,
		startupErr: startupErr,
		cfg:        cfg,
		licenses:   licenses,
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
// 順序は IMP-194 の表のとおり: 保存の予約を止める → **書き込みの途中なら上限つきで待つ** →
// 監視を閉じる → 設定を保存する。
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

	a.waitForWrites(shutdownWriteWait)

	if w != nil {
		_ = w.Close()
	}

	// 大きさは onBeforeClose で取り込み済みである。
	a.persistConfig()
}

// 終了時に書き込みの途中を待つ上限と、錠を試す間隔（IMP-194）。
const (
	shutdownWriteWait = 2 * time.Second
	shutdownWritePoll = 20 * time.Millisecond
)

// waitForWrites は、ioMu を取れるまで最大 limit だけ待つ（IMP-194, FR-143, NFR-031）。
//
// **待たずに終わると、document.Replace（IMP-107）の途中でプロセスが終わり、一時ファイルが
// 残りうる。** 上限を設けるのは、読み込みと変換（大きな文書では秒単位）で終了を止めないため
// である。Lock で待つと上限を付けられないため、TryLock を短い間隔で試す。
//
// **取れた錠は解かない。** 待った後に新しい書き込みが始まり、その途中で終わるのを防ぐ。
// この後に ioMu を取る呼び出し（監視のイベント・バインドメソッド）は、プロセスの終了まで待つ。
// 上限まで取れなかった場合は、そのまま後の処理へ進む。
func (a *App) waitForWrites(limit time.Duration) bool {
	if a.ioMu.TryLock() {
		return true
	}

	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	poll := time.NewTicker(shutdownWritePoll)
	defer poll.Stop()

	for {
		select {
		case <-deadline.C:
			return false
		case <-poll.C:
			if a.ioMu.TryLock() {
				return true
			}
		}
	}
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
