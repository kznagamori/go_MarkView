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
	dir    string // 監視中の親ディレクトリ。空なら監視していない
	target string // 監視対象の絶対パス。空なら監視していない
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
		fsw:    fsw,
		events: make(chan Event),
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
func (w *Watcher) Watch(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot resolve the path to watch: %w", err)
	}
	dir := filepath.Dir(abs)

	w.mu.Lock()
	defer w.mu.Unlock()

	// 同じディレクトリなら、対象を差し替えるだけでよい。
	if w.dir != "" && samePath(w.dir, dir) {
		w.target = abs
		return nil
	}

	if w.dir != "" {
		_ = w.fsw.Remove(w.dir)
		w.dir, w.target = "", ""
	}

	if err := w.fsw.Add(dir); err != nil {
		return fmt.Errorf("cannot watch the directory: %w", err)
	}
	w.dir, w.target = dir, abs
	return nil
}

// Unwatch は監視を解除する（IMP-140）。
func (w *Watcher) Unwatch() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.dir != "" {
		_ = w.fsw.Remove(w.dir)
	}
	w.dir, w.target = "", ""
}

// Events は通知チャネルを返す（IMP-140）。
//
// 流れるのはデバウンス後のイベントだけである。チャネルは監視の終了時に閉じる。
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
func (w *Watcher) emit() bool {
	w.mu.Lock()
	target := w.target
	w.mu.Unlock()

	if target == "" {
		return true
	}

	kind := Modified
	if _, err := os.Stat(target); err != nil {
		kind = Removed
	}

	select {
	case w.events <- Event{Path: target, Kind: kind}:
		return true
	case <-w.done:
		return false
	}
}

// matches はイベントのパスが監視対象かを返す。
func (w *Watcher) matches(name string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.target == "" {
		return false
	}
	return samePath(name, w.target)
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
