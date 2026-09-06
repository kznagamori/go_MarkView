package ostheme

import "testing"

// TestFromAppsUseLightTheme はレジストリ値の解釈を検証する
// （UT-703 ケース 1〜3。根拠: FR-071 / IMP-175）。
//
// **名前と意味が逆であることを取り違えないための検査でもある。**
// AppsUseLightTheme は「ライトを使うか」であり、0 がダークである。
func TestFromAppsUseLightTheme(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
		want  string
	}{
		// 境界値を先に（UT-013）
		{"0 はダーク", 0, "dark"},
		{"1 はライト", 1, "light"},

		// 想定外の値。Windows は 0 / 1 しか書かないが、書き換えられていても
		// 落ちず、ライト側へ倒す
		{"2 はライト", 2, "light"},
		{"大きな値はライト", 4294967295, "light"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromAppsUseLightTheme(tt.value); got != tt.want {
				t.Errorf("fromAppsUseLightTheme(%d) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// TestFromColorScheme は gsettings の color-scheme の解釈を検証する
// （UT-703 ケース 4〜8。根拠: FR-071 / IMP-175）。
//
// gsettings は値を引用符付きで、末尾に改行を付けて返す。**実際の出力の形の
// まま**入力に与える。整形済みの文字列で試すと、引用符と改行を落とす処理が
// 検証されない。
func TestFromColorScheme(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		// 異常系・判定できない側を先に（UT-013）
		{"空文字", "", ""},
		{"default は判定しない", "'default'\n", ""},
		{"未知の値", "'prefer-sepia'\n", ""},
		{"引用符がない", "prefer-dark\n", "dark"},

		// gsettings の実際の出力
		{"prefer-dark", "'prefer-dark'\n", "dark"},
		{"prefer-light", "'prefer-light'\n", "light"},
		{"改行がない", "'prefer-dark'", "dark"},
		{"二重引用符", "\"prefer-dark\"\n", "dark"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromColorScheme(tt.out); got != tt.want {
				t.Errorf("fromColorScheme(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

// TestFromGTKTheme は gsettings の gtk-theme の解釈を検証する
// （UT-703 ケース 9〜13。根拠: FR-071 / IMP-175）。
func TestFromGTKTheme(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		// 判定できない場合（UT-013）
		{"空文字", "", ""},
		{"引用符だけ", "''\n", ""},

		// **接尾辞だけを見る。** 名前の途中に dark を含むテーマを拾わない
		{"darkmatter は途中に dark があるだけ", "'darkmatter'\n", "light"},
		{"Darkly", "'Darkly'\n", "light"},

		// 接尾辞が -dark のもの
		{"Adwaita-dark", "'Adwaita-dark'\n", "dark"},
		{"Yaru-dark", "'Yaru-dark'\n", "dark"},
		{"大文字の -DARK", "'Adwaita-DARK'\n", "dark"},

		// 接尾辞がなければライト
		{"Adwaita", "'Adwaita'\n", "light"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromGTKTheme(tt.out); got != tt.want {
				t.Errorf("fromGTKTheme(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

// TestDetect_ReturnsKnownValue は Detect が定義済みの値だけを返すことを
// 検証する（UT-703 ケース 14。根拠: FR-071 / IMP-175）。
//
// **実行環境の OS 設定に依存する値そのものは検査できない**（UT-035）。
// 検査できるのは「Light / Dark / 判定不能 のいずれかであること」までで、
// これは呼び出し側が 3 通りだけを扱えばよいことの裏付けになる。
func TestDetect_ReturnsKnownValue(t *testing.T) {
	got := Detect()

	if got != Light && got != Dark && got != Unknown {
		t.Errorf("Detect() = %q, want one of %q / %q / %q", got, Light, Dark, Unknown)
	}
}
