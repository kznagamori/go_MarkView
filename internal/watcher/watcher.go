// Package watcher は表示中ファイルの変更を監視する（IMP-140 系）。
//
// internal のうち、依存を持たない葉パッケージ（applog）だけを参照する。
// Wails の API は呼ばない（IMP-012）。
package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kznagamori/go_MarkView/internal/applog"

	"github.com/fsnotify/fsnotify"
)

// debounceInterval は変更をまとめる時間（IMP-142, FR-014）。
//
// エディタの保存は複数のファイルシステムイベントに分かれる。まとめないと
// 1 回の保存で何度も再描画してしまう。
const debounceInterval = 150 * time.Millisecond

// EventKind はイベントの種別（IMP-140）。
type EventKind int

const (
	Modified EventKind = iota
	Removed
)

// Event は監視対象に起きた事象（IMP-140）。
type Event struct {
	Path string
	Kind EventKind
}

// Watcher は監視対象 1 つの変更を通知する（IMP-140）。
//
// 監視対象は常に 1 つ以下である。Watch を呼ぶたびに以前の対象を解除するため、
// ウォッチャが積み上がることはない（NFR-020）。
type Watcher struct {
	fsw    *fsnotify.Watcher
	events chan Event

	// done は Close の合図。closeOnce で 2 回目以降を無視する。
	done      chan struct{}
	closeOnce sync.Once

	mu     sync.Mutex
	dir    string // 監視中のディレクトリ（実体の親）。空なら監視していない
	target string // Watch に渡されたパスの絶対パス。Event.Path に載せる。空なら監視していない
	real   string // target のシンボリックリンクを解決した実体のパス。イベントの名前と比べる
}

// New は監視を開始する（IMP-140）。
//
// ctx のキャンセルで内部ゴルーチンを終了する（IMP-024）。呼び出し側は
// Close または ctx のどちらでも終了させられる。
func New(ctx context.Context) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("cannot start the file watcher: %w", err)
	}

	w := &Watcher{
		fsw: fsw,
		// バッファは 1 とし、取り出される前に次のイベントが出たら新しい値で置き換える
		// （IMP-140）。emit を見る。
		events: make(chan Event, 1),
		done:   make(chan struct{}),
	}

	go w.run(ctx)
	return w, nil
}

// Watch は監視対象を path 1 つに切り替える（IMP-140, IMP-141）。
//
// **監視するのは対象ファイルの親ディレクトリである。** ファイル単体を監視すると、
// エディタの「一時ファイルを作ってリネームする」保存方式で監視ハンドルが外れ、
// 2 回目以降の保存を検知できなくなる（FR-014）。
//
// **親ディレクトリは、シンボリックリンクを解決した実体のものとする**（AR-070, IMP-141）。
// リンクの置き場所を監視すると、実体への書き込み（外部のエディタ、編集モードの
// 書き込み。IMP-107 は実体を書き換える）が一度も届かない（UT-407。v1.0.0 はこうだった）。
// イベントの名前も実体のパスと比べる。解決に失敗した場合は、受け取ったパスのまま監視する。
//
// **Event.Path は Watch に渡したパスとする**（リンクを開いていればリンクのパス）。実体の
// パスを載せると、読み直しでウィンドウタイトルとステータスのパスが実体へ変わる（IMP-190）。
func (w *Watcher) Watch(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve the path to watch: %w", err)
	}
	real := abs
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		// Windows では一時ディレクトリの短い名前（8.3 形式）も長い名前へ解ける。監視する
		// ディレクトリとイベントの名前を比べる実体のパスを、同じ表記から作る。
		real = resolved
	}
	dir := filepath.Dir(real)

	w.mu.Lock()
	defer w.mu.Unlock()

	// 同じディレクトリなら、対象を差し替えるだけでよい。
	if w.dir != "" && samePath(w.dir, dir) {
		w.target, w.real = abs, real
		return nil
	}

	if w.dir != "" {
		_ = w.fsw.Remove(w.dir)
		w.dir, w.target, w.real = "", "", ""
	}

	if err := w.fsw.Add(dir); err != nil {
		return fmt.Errorf("cannot watch the directory: %w", err)
	}
	w.dir, w.target, w.real = dir, abs, real
	return nil
}

// Unwatch は監視を解除する（IMP-140）。
func (w *Watcher) Unwatch() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dir != "" {
		_ = w.fsw.Remove(w.dir)
	}
	w.dir, w.target, w.real = "", "", ""
}

// Events は通知チャネルを返す（IMP-140）。
//
// 流れるのはデバウンス後のイベントだけである。チャネルは監視の終了時に閉じる。
// バッファは 1 であり、受け手が取り出す前に次のイベントが出たら新しい値で置き換える。
func (w *Watcher) Events() <-chan Event {
	return w.events
}

// Close は監視を終了する（IMP-140, UT-406）。
//
// 2 回以上呼んでもよい。呼び出し側の終了処理が重なっても落ちないようにする。
func (w *Watcher) Close() error {
	w.closeOnce.Do(func() { close(w.done) })
	return nil
}

// run はイベントを受け取り、デバウンスして通知する（IMP-142）。
func (w *Watcher) run(ctx context.Context) {
	// 監視の終了時にチャネルを閉じる。呼び出し側は range で待てる。
	defer close(w.events)
	defer func() { _ = w.fsw.Close() }()

	// 止めた状態から始める。イベントを受けるたびに Reset で測り直す。
	timer := time.NewTimer(debounceInterval)
	timer.Stop()
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-w.done:
			return

		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			// 親ディレクトリを監視しているため、対象以外のイベントも届く。
			// ツリーの更新契機には使わず、ここで捨てる（IMP-141, FR-035）。
			if !w.matches(ev.Name) {
				continue
			}
			// 最後のイベントから debounceInterval 静かになるまで待つ。
			timer.Stop()
			timer.Reset(debounceInterval)

		case <-timer.C:
			if !w.emit() {
				return
			}

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			// 監視のエラーで終了はしない。1 つのファイルを見失うだけで、
			// 利用者は再読み込みできる（FR-111）。
			//
			// 既定では何も出ない。MARKVIEW_DEBUG=1 のときだけ記録する
			// （IMP-023, NFR-041）。判定は applog が持つ。
			applog.New().Error("watcher error", "err", err)
		}
	}
}

// emit はデバウンス後の 1 件を送出する。false を返したら run を終える。
//
// **削除と保存の区別はここで行う**（IMP-142）。エディタの「一時ファイルを作って
// リネームする」保存では、対象が一瞬消えてから同名で現れる。イベントを受けた
// 時点ではなく、静かになった時点で存在を確かめることで、保存を削除と誤認しない。
//
// **送信で止まらない**（IMP-140, IMP-024）。受け手（IMP-192 の読み直し、IMP-195 の削除）は
// ioMu を待つため、取り出しが秒単位で遅れうる。送信で止まると、このゴルーチンが fsnotify の
// イベントを読まなくなる。**fsnotify v1.10.1 の Windows 実装は、イベントの送信が詰まって
// いる間、Add / Remove の要求も処理しない**——文書を開く処理が ioMu を持ったまま Watch を
// 呼ぶと止まり、バインドメソッドがすべて止まる（UT-405 ケース 4）。
//
// 取り出されていない古い値は捨てて置き換える。監視対象は常に 1 つであり（NFR-020）、受け手が
// 必要とするのは「いまファイルがあるか」の最新の状態だけである。Modified の後の Removed も、
// その逆も、後の値だけで正しく扱える（UT-404 ケース 3）。
func (w *Watcher) emit() bool {
	w.mu.Lock()
	target := w.target
	w.mu.Unlock()

	if target == "" {
		return true
	}

	// os.Stat はシンボリックリンクを辿る。リンクを監視していれば、実体が無くなった時点で
	// Removed になる。
	kind := Modified
	if _, err := os.Stat(target); err != nil {
		kind = Removed
	}

	ev := Event{Path: target, Kind: kind}
	for {
		select {
		case <-w.done:
			return false
		default:
		}

		select {
		case w.events <- ev:
			return true
		default:
			// バッファに取り出されていない値がある。捨ててから入れ直す。受け手が同時に
			// 取り出していれば、ここでは何も取れず、次の周回で入る。
			select {
			case <-w.events:
			default:
			}
		}
	}
}

// matches はイベントのパスが監視対象かを返す。
//
// **実体の名前と比べる**（IMP-141）。監視しているのは実体のディレクトリであり、リンクと
// 同じ名前の別のファイルがそこにあっても対象ではない（UT-407 ケース 4）。編集モードの
// 一時ファイル（`.<名前>.markview-*.tmp`。IMP-107）もここで捨てる。
func (w *Watcher) matches(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.real == "" {
		return false
	}
	return samePath(name, w.real)
}

// samePath は 2 つのパスが同じ場所を指すかを判定する（IMP-025）。
//
// Windows では大文字小文字を区別せず、Linux では区別する。session にも同じ
// 判定があるが、internal 同士は依存できないためここにも置く（IMP-012）。
func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
