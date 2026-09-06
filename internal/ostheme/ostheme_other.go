//go:build !windows

package ostheme

import (
	"context"
	"os/exec"
	"time"
)

// gsettings の応答を待つ上限（IMP-175）。
//
// 起動経路で呼ばれるため、応答しない環境で待ち続けさせない。GNOME が
// 入っていない環境では実行そのものが失敗し、待ち時間は発生しない。
const detectTimeout = 500 * time.Millisecond

const interfaceSchema = "org.gnome.desktop.interface"

// detect は gsettings から OS のテーマを読む（FR-071, IMP-175）。
//
// color-scheme を先に見る。GNOME 42 以降はこちらが正であり、gtk-theme は
// 配色と一致しないことがある。読めない場合だけ gtk-theme へ落とす。
func detect() string {
	if theme := fromColorScheme(gsettings("color-scheme")); theme != Unknown {
		return theme
	}

	return fromGTKTheme(gsettings("gtk-theme"))
}

// gsettings は設定値を 1 つ読む。失敗した場合は空文字を返す。
//
// 引数は exec.Command の可変長引数として渡し、シェルを経由しない
// （IMP-170 と同じ理由）。
func gsettings(key string) string {
	ctx, cancel := context.WithTimeout(context.Background(), detectTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "gsettings", "get", interfaceSchema, key).Output()
	if err != nil {
		return ""
	}

	return string(out)
}
