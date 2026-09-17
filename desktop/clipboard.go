package desktop

import (
	"errors"
	"fmt"
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// 本ファイルはクリップボードのバインドメソッドを持つ（IMP-310, AR-062）。
//
// **失敗は Go の error で返す**（IMP-308）。種別は書き込みと読み込みで 1 つずつしかなく、
// Kind を伴う分岐が要らない（IMP-315）。

// CopyToClipboard はテキストをクリップボードへ書く（IMP-310, FR-061, FR-063, AR-062）。
//
// WebView の Clipboard API は権限や環境によって使えないことがあるため、
// Go 側を経由する（AR-062）。
func (a *App) CopyToClipboard(text string) (err error) {
	defer recoverBind(&err)

	ctx := a.context()
	if ctx == nil {
		return errClipboard
	}

	if err := runtime.ClipboardSetText(ctx, text); err != nil {
		return fmt.Errorf("%w: %v", errClipboard, err)
	}
	return nil
}

// ReadClipboard はクリップボードのテキストを読む（IMP-310, FR-063, AR-062）。
//
// 右クリックメニューの Paste が使う。**WebView の navigator.clipboard.readText は読み取りに
// 権限を求めるため使わない**（AR-062）。
//
// **テキスト以外（画像など）しか入っていなければ空文字を返し、失敗にしない**（IMP-310）。
// Wails v2.15.0 の Windows の実装は、テキストの形式が無いと空文字と「エラー番号 0」
// （syscall.Errno(0)。IsClipboardFormatAvailable が 0 を返しただけで、失敗ではない）を返す。
// Linux の実装はエラーを返さない。
func (a *App) ReadClipboard() (text string, err error) {
	defer recoverBind(&err)

	ctx := a.context()
	if ctx == nil {
		return "", errPaste
	}

	text, err = runtime.ClipboardGetText(ctx)
	if err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && errno == 0 {
			return "", nil
		}
		return "", fmt.Errorf("%w: %v", errPaste, err)
	}
	return text, nil
}
