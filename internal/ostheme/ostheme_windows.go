package ostheme

import "golang.org/x/sys/windows/registry"

// Windows のテーマ設定の在り処（IMP-175）。
//
// SystemUsesLightTheme（タスクバー等）ではなく AppsUseLightTheme を見る。
// アプリケーションの配色に対応するのはこちらである。
const (
	personalizeKey    = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	appsUseLightValue = "AppsUseLightTheme"
)

// detect はレジストリから OS のテーマを読む（FR-071, IMP-175）。
//
// キーも値も存在しないことがある（古い Windows、ポリシーによる削除）。
// その場合は Unknown を返し、呼び出し側の既定に委ねる。
func detect() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if err != nil {
		return Unknown
	}
	defer key.Close()

	value, _, err := key.GetIntegerValue(appsUseLightValue)
	if err != nil {
		return Unknown
	}

	return fromAppsUseLightTheme(value)
}
