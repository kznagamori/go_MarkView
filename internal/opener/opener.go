// Package opener は URL とファイルを OS の既定アプリケーションへ委譲する
// （IMP-170 系）。
//
// 依存を持たない。Wails の API も呼ばない（IMP-012）。
package opener

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 番兵エラー（IMP-021）。
var (
	ErrUnsupportedScheme = errors.New("unsupported URL scheme")
	ErrNotFound          = errors.New("file not found")
)

// allowedSchemes は外部へ委譲してよい URL のスキーム（IMP-170, NFR-030）。
//
// **これ以外は拒否する。** 文書は任意の第三者から受け取りうるため、
// javascript: や file: をそのまま OS へ渡してはならない。
var allowedSchemes = map[string]bool{
	"http":   true,
	"https":  true,
	"mailto": true,
}

// runCommand は外部プロセスを起動する。
//
// **テストで差し替えられるように変数にしている**（UT-702）。実際にブラウザや
// 画像ビューアを起動するテストは書かない（UT-035）。組み立てた引数までを
// 検証する。
var runCommand = start

// OpenURL は既定ブラウザで URL を開く（IMP-170, FR-050, UI-102）。
//
// 受け付けるのは http / https / mailto のみ。スキームを持たない文字列も拒む。
func OpenURL(rawurl string) error {
	u, err := url.Parse(rawurl)
	if err != nil {
		return fmt.Errorf("%s: %w", rawurl, ErrUnsupportedScheme)
	}
	// url.Parse はスキームを小文字にして返すため ToLower は現状では
	// 効かないが、判定を url.Parse の実装詳細に頼らせない。
	if !allowedSchemes[strings.ToLower(u.Scheme)] {
		return fmt.Errorf("%s: %w", rawurl, ErrUnsupportedScheme)
	}

	return open(rawurl)
}

// OpenFile は既定のアプリケーションでファイルを開く（IMP-170, FR-053）。
//
// 存在しないファイルはここで弾く。起動コマンドは対象がなくても起動自体には
// 成功してしまい、利用者に何も伝わらないためである（FR-110）。
func OpenFile(path string) error {
	if path == "" {
		return fmt.Errorf("%s: %w", path, ErrNotFound)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, ErrNotFound)
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("%s: %w", abs, ErrNotFound)
	}

	return open(abs)
}

// open は OS の既定アプリケーションへ委譲する（IMP-170）。
//
// **引数は必ず可変長引数として渡し、シェルを経由しない。** 文字列連結で
// コマンドを組み立てると、空白や `;` を含むパスで意図しない実行が起きる。
func open(target string) error {
	name, args := openCommand(target)

	if err := runCommand(name, args...); err != nil {
		return fmt.Errorf("cannot open %s: %w", target, err)
	}
	return nil
}

// start は外部プロセスを起動して戻る。
//
// 終了は待たない。待つと、利用者がブラウザを閉じるまでこの呼び出しが返らない。
// ただし Wait を呼ばないと Unix でゾンビプロセスが残るため、別のゴルーチンで
// 回収する（NFR-020）。
func start(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()

	return nil
}
