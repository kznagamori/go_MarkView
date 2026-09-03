package main

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/kznagamori/go_MarkView/internal/document"
	"github.com/kznagamori/go_MarkView/internal/opener"
)

// 本ファイルは Go の番兵エラーを ErrorDTO へ写す（IMP-315）。
//
// **文言の組み立てはフロントエンドで行う。** Go 側は Kind と要素（パス・
// サイズ）を渡し、フロントエンドが strings.js の文言を選ぶ（IMP-290）。
// Message は未知の Kind を受け取ったときのフォールバックであり、UI に出る
// 文言の正はここではない。

// errPanic は回復したパニックを表す（IMP-022, FR-111）。
//
// バインドメソッドの入口で recover し、これに置き換えて返す。利用者には
// 状態画面（UI-052）の render-error として見える。
var errPanic = errors.New("recovered from panic")

// errClipboard はクリップボードへの書き込みに失敗したことを表す
// （FR-061, AR-062）。
var errClipboard = errors.New("cannot write to the clipboard")

// errLinkNotFound はリンク先が見つからないことを表す（FR-050）。
var errLinkNotFound = errors.New("link target not found")

// errNoTarget は画面が対象にしているファイルが無いことを表す
// （IMP-310, FR-090）。
//
// 文書未表示（welcome）ではボタンが淡色になっており（UI-021）通常は起こらない
// が、**防御的に扱う。** ここで target の代わりに current を使ってしまうと、
// 状態画面を見ながら押したときに前の文書が開く（IMP-190）。
var errNoTarget = errors.New("no file is currently targeted")

// errUnknownEditor は指定された ID のエディタを解決できないことを表す
// （IMP-310, FR-091）。
//
// 一覧に無い ID、見つからなかったプリセット、`Browse` していない `custom` が
// これに当たる。**フロントエンドから任意の実行ファイルを起動する経路を作らない**
// ための拒否であり（IMP-300 の 3, NFR-035）、通常の操作では起こらない。
var errUnknownEditor = errors.New("the selected editor is not available")

// newErrorDTO はエラーを ErrorDTO へ写す（IMP-315）。
//
// path は対象のパス（リンクの場合は href）。エラー値そのものからは取り出せない
// ため、呼び出し側が渡す。番兵エラーは `fmt.Errorf("%s: %w", path, err)` の形で
// 包まれており、メッセージからパスを切り出す実装は包み方に依存してしまう。
//
// **分類できないエラーは render-error とする。** 想定外の失敗を「見つからない」
// と伝えると、利用者は存在するファイルを探し続けることになる。
func newErrorDTO(path string, err error) *ErrorDTO {
	if err == nil {
		return nil
	}

	// サイズ超過は判断に必要な数値を伴う（IMP-102, UI-052）。
	var sizeErr *document.SizeError
	if errors.As(err, &sizeErr) {
		return newSizeErrorDTO(sizeErr)
	}

	kind, message := classifyError(path, err)

	return &ErrorDTO{Kind: kind, Message: message, Path: path}
}

// newSizeErrorDTO はサイズ超過を写す（IMP-315, FR-016）。
func newSizeErrorDTO(sizeErr *document.SizeError) *ErrorDTO {
	dto := &ErrorDTO{
		Path:  sizeErr.Path,
		Size:  sizeErr.Size,
		Limit: sizeErr.Limit,
	}

	if errors.Is(sizeErr.Err, document.ErrNeedsConfirm) {
		dto.Kind = errKindNeedsConfirm
		dto.Message = "This file is large."
		return dto
	}

	dto.Kind = errKindTooLarge
	dto.Message = fmt.Sprintf("File is too large (%s / limit %s)",
		formatSize(sizeErr.Size), formatSize(sizeErr.Limit))

	return dto
}

// classifyError は番兵エラーを Kind とフォールバック文言へ写す（IMP-315）。
func classifyError(path string, err error) (kind, message string) {
	switch {
	case errors.Is(err, document.ErrNotFound):
		return errKindNotFound, fmt.Sprintf("File not found: %s", path)

	case errors.Is(err, document.ErrPermission):
		return errKindPermission, fmt.Sprintf("Cannot access: %s", path)

	case errors.Is(err, document.ErrNotMarkdown):
		return errKindNotMarkdown, fmt.Sprintf("Not a Markdown file: %s", path)

	case errors.Is(err, errLinkNotFound):
		return errKindLinkNotFound, fmt.Sprintf("Link target not found: %s", path)

	case errors.Is(err, errClipboard):
		return errKindClipboard, "Failed to copy."

	// 起動時のパス解決（session.ResolveStartup）は os.Stat のエラーを
	// そのまま包んで返す（IMP-193）。document の番兵には包み直されないため、
	// ここで拾わないと render-error に落ちて状態画面が出てしまう。
	case errors.Is(err, fs.ErrNotExist):
		return errKindNotFound, fmt.Sprintf("File not found: %s", path)

	case errors.Is(err, fs.ErrPermission):
		return errKindPermission, fmt.Sprintf("Cannot access: %s", path)

	default:
		// 変換エラーと回復したパニックがここに来る（IMP-022）。
		return errKindRenderError, "Failed to render this document."
	}
}

// newEditorErrorDTO はエディタの起動失敗を ErrorDTO へ写す（IMP-315）。
//
// **classifyError を使い回さない。** opener.ErrNotFound は「エディタの実行
// ファイルが無い」であって「文書が無い」ではない。同じ not-found として
// 伝えると、利用者は開けなかった Markdown を探し始めることになる。
// エディタの失敗はどれも「起動できなかった」の一言で足りる（UI-060）。
//
// **ErrorDTO.Path を設定しない。** ここに載せられる対象は実行ファイルのパス
// しかなく、それはフロントエンドへ渡してはならない（NFR-035 の 3,
// IMP-309）。文言（IMP-315）もパスを含まない。
func newEditorErrorDTO(err error) *ErrorDTO {
	if err == nil {
		return nil
	}

	// MarkView 自身の指定だけは区別して伝える。「起動できません」とだけ
	// 返すと、利用者は原因が分からず同じ操作を繰り返す（NFR-035 の 6）。
	if errors.Is(err, opener.ErrSelf) {
		return &ErrorDTO{
			Kind:    errKindEditorSelf,
			Message: "MarkView cannot be used as an editor.",
		}
	}

	// 絶対パスでない・実行ファイルが無い・プロセスを起動できない・対象が
	// 無い（errNoTarget）・回復したパニックは、すべてここへ落ちる。
	return &ErrorDTO{
		Kind:    errKindEditorFailed,
		Message: "Failed to start the editor.",
	}
}

// removedErrorDTO は監視対象が削除されたことを伝える（IMP-315, FR-014）。
//
// エラー値を経由しない。watcher は「削除された」という事実を Event として
// 返すのであって、失敗を返すわけではない（IMP-142）。
func removedErrorDTO(path string) *ErrorDTO {
	return &ErrorDTO{
		Kind:    errKindRemoved,
		Message: fmt.Sprintf("File was deleted: %s", path),
		Path:    path,
	}
}

// stateKindFor は起動時のエラーに対応する状態画面を返す（IMP-193, IMP-303）。
//
// **存在しない・読めないパスでも welcome とする。** 起動できなかったことを
// 専用の画面で伝えるより、通常の操作案内を出したうえでステータスに理由を
// 添えるほうが、利用者は次に何をすればよいか分かる（FR-012）。
func stateKindFor(dto *ErrorDTO) string {
	if dto == nil {
		return stateWelcome
	}

	switch dto.Kind {
	case errKindNeedsConfirm:
		return stateConfirmLarge
	case errKindTooLarge:
		return stateTooLarge
	case errKindRenderError:
		return stateRenderError
	default:
		return stateWelcome
	}
}

// formatSize はバイト数を MB 表記にする（UI-052）。
//
// フォールバック文言でしか使わない。UI に出る表記は strings.js が組み立てる
// （IMP-290）。
func formatSize(bytes int64) string {
	const mib = 1 << 20
	return fmt.Sprintf("%.1f MB", float64(bytes)/mib)
}
