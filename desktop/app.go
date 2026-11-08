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

// Go からフロントエンドへ送るイベント名（IMP-320）。
//
// 一方向の通知であり、フロントエンドは起動時に 1 度だけ購読する（IMP-322）。
const (
	eventDocumentOpened  = "document:opened"  // 呼び出し以外で表示対象が変わった（FR-011）
	eventDocumentChanged = "document:changed" // 表示中ファイルの更新を検知した（FR-014）
	eventDocumentRemoved = "document:removed" // 表示中ファイルが削除された（FR-014）
	eventTreeRootChanged = "tree:root-changed"
	eventHistoryChanged  = "history:changed" // 表示履歴が変わった（積む・戻る・進む・取り除く。FR-051, IMP-323）
	eventError           = "error"           // 非同期処理で発生したエラー（FR-110）
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

	// editorTarget はエディタ選択ウィンドウを開いた時点で控えた、エディタで開くファイル（IMP-190,
	// FR-091。4.75.0）。
	//
	// ListEditors なら target（**文字のファイルのときだけ**。画像・文書未表示なら空）、ListEditorsForTree
	// なら IMP-135 で確かめたファイル。**OpenInEditor はこれを開き**、起動の成否に関わらず終わったら
	// 捨てる。ウィンドウを閉じたら（CloseEditor）も捨てる。フロントエンドへ渡さない（ベース名だけを
	// EditorListDTO.FileName に載せる）。v1.1.0 は押した時点の target を開いており、**ウィンドウを
	// 開いている間にドロップで文書が切り替わると、ウィンドウに出ている名前と違うファイルが開いた**。
	editorTarget string

	// editBoxOpen は、フロントエンドで編集欄（セル・コードブロック）が開いているか（SetEditBoxOpen。
	// IMP-190, IMP-313。4.75.0）。
	//
	// **ドロップを拒むためにだけ使う**（FR-140）。Linux ではドロップが JavaScript を通らずに Go へ届く
	// ため、フロントエンドだけでは止められない。**書き込みの判断には使わない**（IMP-109 が正）。
	editBoxOpen bool

	// pendingSource は確認画面を出したときの経路（IMP-192, IMP-314）。
	//
	// OpenConfirmed で開き直すとき、ツリールートと履歴を二重に動かさない
	// ために保持する。確認の時点でどちらも反映済みである。
	pendingSource openSource

	// historySeq は history:changed（IMP-323）の版。送るたびに 1 増やす（nextHistoryDTO）。
	//
	// イベントとバインドメソッドの戻り値は到着順が決まっていない。フロントエンドはそれまでに受け取った
	// 版より小さいものを捨てる（EditModeDTO.Seq と同じ考え方。IMP-316）。
	historySeq uint64

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
