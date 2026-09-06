package opener

import (
	"os"
	"path/filepath"
)

// presetTable は Windows のプリセット表を返す（IMP-172, UI-103）。
//
// **並びは仕様書の表のとおりに固定する。** 見つかったものを前へ出さない
// （UI-103）。位置で覚えられなくなるためである。
//
// **レジストリを読まない**（NFR-033）。インストール先は環境変数からの既知の
// 相対位置だけで探す。ここに無い場所へ入れている利用者は `Other...` で
// 実行ファイルを選べる（UI-103）。
//
// **`custom` を ID に使わない。** フロントエンドとの間で「任意指定」を表す
// 予約語である（IMP-309）。
func presetTable() []preset {
	return []preset{
		{
			ID:   "notepad",
			Name: "Notepad",
			candidates: paths(
				under("WINDIR", `system32\notepad.exe`),
			),
		},
		{
			ID:   "vscode",
			Name: "Visual Studio Code",
			candidates: paths(
				under("LOCALAPPDATA", `Programs\Microsoft VS Code\Code.exe`),
				under("PROGRAMFILES", `Microsoft VS Code\Code.exe`),
			),
		},
		{
			ID:   "notepadpp",
			Name: "Notepad++",
			candidates: paths(
				under("PROGRAMFILES", `Notepad++\notepad++.exe`),
				under("PROGRAMFILES(X86)", `Notepad++\notepad++.exe`),
			),
		},
		{
			ID:   "hidemaru",
			Name: "Hidemaru",
			candidates: paths(
				under("PROGRAMFILES", `Hidemaru\Hidemaru.exe`),
				under("PROGRAMFILES(X86)", `Hidemaru\Hidemaru.exe`),
			),
		},
		{
			ID:   "sakura",
			Name: "sakura editor",
			candidates: paths(
				under("PROGRAMFILES(X86)", `sakura\sakura.exe`),
				under("PROGRAMFILES", `sakura\sakura.exe`),
			),
		},
	}
}

// lookupPreset は候補の絶対パスを順に調べ、先に見つかったものを返す
// （IMP-172）。見つからなければ空文字を返す。
//
// ディレクトリを「見つかった」とみなさないよう isRegularFile を用いる。
func lookupPreset(p preset) string {
	for _, c := range p.candidates {
		if isRegularFile(c) {
			return c
		}
	}

	return ""
}

// under は環境変数 env が指すディレクトリからの相対位置を絶対パスにする。
//
// **env が空の環境では空文字を返す。** そのまま filepath.Join すると相対パスに
// なり、作業ディレクトリの内容で起動対象が変わる（NFR-035 の 5）。
// 64 ビット版 Windows でも PROGRAMFILES(X86) が無い構成はありうる。
func under(env, rel string) string {
	base := os.Getenv(env)
	if base == "" {
		return ""
	}

	return filepath.Join(base, rel)
}

// paths は空文字を落として候補の並びを作る。
func paths(list ...string) []string {
	out := make([]string, 0, len(list))
	for _, p := range list {
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}
