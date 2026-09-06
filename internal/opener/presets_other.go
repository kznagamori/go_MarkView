//go:build !windows

package opener

import (
	"os/exec"
	"path/filepath"
)

// presetTable は Linux のプリセット表を返す（IMP-172, UI-103）。
//
// 候補はコマンド名であり、$PATH から引く。**並びは仕様書の表のとおりに
// 固定する。** 見つかったものを前へ出さない（UI-103）。
//
// **端末エディタ（vim / nano / emacs -nw）を入れない**（IMP-172）。端末を
// 持たずに起動されるため何も起きないまま終わり、利用者からは「押しても
// 無反応」に見える。引数を渡さない設計（NFR-035 の 2）のため、Other... で
// ラッパを指定して回避することもできない。
//
// **「OS の既定アプリケーション」（xdg-open）も入れない**（IMP-172）。
// .md を MarkView に関連付けている利用者では、押すたびに MarkView が増える。
// NFR-035 の 6 が拒めるのは実行ファイルの直接指定だけである。
//
// **`custom` を ID に使わない。** フロントエンドとの間で「任意指定」を表す
// 予約語である（IMP-309）。
func presetTable() []preset {
	return []preset{
		{ID: "gnome-text-editor", Name: "GNOME Text Editor", candidates: []string{"gnome-text-editor"}},
		{ID: "gedit", Name: "gedit", candidates: []string{"gedit"}},
		{ID: "kate", Name: "Kate", candidates: []string{"kate"}},
		{ID: "mousepad", Name: "Mousepad", candidates: []string{"mousepad"}},
		{ID: "vscode", Name: "Visual Studio Code", candidates: []string{"code"}},
		{ID: "gvim", Name: "gVim", candidates: []string{"gvim"}},
	}
}

// lookupPreset は候補のコマンド名を順に $PATH から引き、先に見つかったものの
// 絶対パスを返す（IMP-172）。見つからなければ空文字を返す。
//
// **絶対パスで返らなかったものは採らない。** $PATH に相対パスの項目が
// 含まれていると exec.LookPath は相対パスを返し、作業ディレクトリの内容で
// 起動対象が変わる（NFR-035 の 5）。
func lookupPreset(p preset) string {
	for _, c := range p.candidates {
		path, err := exec.LookPath(c)
		if err != nil || !filepath.IsAbs(path) {
			continue
		}

		return path
	}

	return ""
}
