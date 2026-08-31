// Package ostheme は OS のテーマ設定（ダークモードの有効・無効）を読む
// （FR-071, IMP-175）。
//
// Wails に依存しない（IMP-012）。**Wails v2 のランタイムには OS のテーマを
// 取得する API がなく**（設定側の WindowSetDarkTheme などしかない）、
// FR-071 の追従を実現するには OS ごとの設定を直接読むしかない。
//
// OS へ実際に問い合わせる部分だけを ostheme_windows.go / ostheme_other.go に
// 置き、**値の解釈はこのファイルに集める**。解釈だけならどのプラットフォーム
// でもテストでき、ビルドタグ付きのテストを書かずに済む（UT-035）。
package ostheme

import "strings"

// テーマの値。空文字は「判定できなかった」を表す。
//
// **判定できない場合に Light を返さない。** 「OS が Light である」ことと
// 「OS の設定を読めなかった」ことは別であり、呼び出し側が区別できる必要が
// ある（FR-071 の既定値の適用は呼び出し側の判断である）。
const (
	Light   = "light"
	Dark    = "dark"
	Unknown = ""
)

// Detect は OS のテーマ設定を返す（FR-071）。
//
// 判定できない場合は Unknown を返す。呼び出し側は FR-071 に従って Light を
// 既定とする。
//
// **設定にテーマが記録されていないときだけ呼ばれることを前提とする。**
// Linux では外部コマンドを起動するため、起動のたびに呼ぶ処理ではない。
func Detect() string {
	return detect()
}

// fromAppsUseLightTheme は Windows のレジストリ値を解釈する（IMP-175）。
//
//	AppsUseLightTheme (REG_DWORD)  0 = ダーク / 1 = ライト
//
// 名前のとおり「ライトテーマを使うか」であり、**0 がダークである**。
func fromAppsUseLightTheme(value uint64) string {
	if value == 0 {
		return Dark
	}

	return Light
}

// fromColorScheme は gsettings の color-scheme を解釈する（IMP-175）。
//
//	'prefer-dark'   → ダーク
//	'prefer-light'  → ライト
//	'default'       → 判定しない（gtk-theme を見る）
//
// gsettings は値を引用符付き・末尾に改行を付けて返すため、両方を落とす。
func fromColorScheme(out string) string {
	switch unquote(out) {
	case "prefer-dark":
		return Dark
	case "prefer-light":
		return Light
	default:
		return Unknown
	}
}

// fromGTKTheme は gsettings の gtk-theme を解釈する（IMP-175）。
//
// color-scheme は GNOME 42 以降にしかないため、それ以前ではテーマ名で
// 判断するほかない。`Adwaita-dark` のように **-dark で終わるものだけ**を
// ダークとみなす。名前の途中に dark を含むだけのテーマまで拾うと、
// 意図しない配色で起動する。
func fromGTKTheme(out string) string {
	name := unquote(out)
	if name == "" {
		return Unknown
	}
	if strings.HasSuffix(strings.ToLower(name), "-dark") {
		return Dark
	}

	// 接尾辞がなければライトとみなす。gtk-theme が読めた時点で GNOME 系の
	// 設定は存在しており、ダークでないなら残るのはライトである。
	return Light
}

// unquote は gsettings の出力から引用符と前後の空白を落とす。
func unquote(out string) string {
	return strings.Trim(strings.TrimSpace(out), "'\"")
}
