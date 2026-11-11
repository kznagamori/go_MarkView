package config

import (
	"os"
	"reflect"
	"testing"
)

// other は「他のインスタンスが書いた」ことにする設定。どの項目も Default() と違う値にしてある。
func other() Config {
	return Config{
		Theme:            "dark",
		OutlineVisible:   false,
		FileTreeVisible:  true,
		OutlineWidth:     300,
		FileTreeWidth:    320,
		WindowWidth:      1000,
		WindowHeight:     700,
		Editor:           absEditor(),
		TreeShowDiagrams: false,
		TreeShowImages:   true,
	}
}

// absEditor は OS ごとの絶対パスのエディタ（Normalize で消えない値）を返す。
func absEditor() string {
	if os.PathSeparator == '\\' {
		return `C:\Tools\editor.exe`
	}
	return "/opt/tools/editor"
}

// TestMerge は、変えた項目だけを書き換えることを検証する（UT-507 ケース 1〜3。根拠: UI-115, FR-070 / IMP-151。BUG-031）。
//
// base はこのインスタンスが起動時に読んだ値（ここでは Default()）、disk は保存の直前のファイル（他の
// インスタンスが other() を書いた）とする。
func TestMerge(t *testing.T) {
	base := Default()
	disk := other()

	tests := []struct {
		name string
		cur  Config
		want Config
	}{
		{
			// ケース 1: 何も変えていない → ファイルの値をすべて残す
			name: "何も変えていない",
			cur:  Default(),
			want: other(),
		},
		{
			// ケース 2: テーマに触れていないインスタンスが、他のインスタンスが選んだテーマを書き戻さない
			// （BUG-031 の戻り方 2。Light を選ぶ前から開いていたウィンドウを、選んだ後に閉じる）
			name: "テーマに触れていない",
			cur:  withOutlineWidth(Default(), 280),
			want: withOutlineWidth(other(), 280),
		},
		{
			// ケース 3: 同じ項目を両方で変えた → 後から保存したこのインスタンスの値が残る
			name: "同じ項目を両方で変えた",
			cur:  withTheme(Default(), "light"),
			want: withTheme(other(), "light"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Merge(disk, base, tt.cur); got != tt.want {
				t.Errorf("Merge =\n %+v\nwant\n %+v", got, tt.want)
			}
		})
	}
}

// TestMerge_EachField は、どの項目も、変えたらこのインスタンスの値が書かれることを検証する
// （UT-507 ケース 4。根拠: UI-115, UI-110 / IMP-151）。
//
// ファイルの値と起動時に読んだ値を Default() にそろえ、1 つの項目だけを既定値と違うリテラルにする。
// 書き写しを忘れた項目は既定値のまま残って落ちる。**変えていない項目がファイルの値を残すことは、
// TestMerge のケース 1 が全項目について見る**（other() はどの項目も Default() と違う）。
//
// **項目の数を固定する。** Config に項目を足したら落ちる——表に行を足し、保存の仕方（UI-115）を考える。
func TestMerge_EachField(t *testing.T) {
	tests := []struct {
		name string
		set  func(c *Config)
	}{
		{"テーマ", func(c *Config) { c.Theme = "light" }},
		{"アウトラインの表示", func(c *Config) { c.OutlineVisible = false }},
		{"ファイルツリーの表示", func(c *Config) { c.FileTreeVisible = true }},
		{"アウトラインの幅", func(c *Config) { c.OutlineWidth = 250 }},
		{"ファイルツリーの幅", func(c *Config) { c.FileTreeWidth = 270 }},
		{"ウィンドウの幅", func(c *Config) { c.WindowWidth = 1100 }},
		{"ウィンドウの高さ", func(c *Config) { c.WindowHeight = 800 }},
		{"エディタ", func(c *Config) { c.Editor = absEditor() }},
		{"ツリーに図を出すか", func(c *Config) { c.TreeShowDiagrams = false }},
		{"ツリーに画像を出すか", func(c *Config) { c.TreeShowImages = true }},
	}

	if n := reflect.TypeFor[Config]().NumField(); n != len(tests) {
		t.Fatalf("Config の項目は %d 個、表は %d 行。項目を足したら表に行を足す（UI-115）", n, len(tests))
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cur := Default()
			tt.set(&cur)
			if cur == Default() {
				t.Fatal("表の値が既定値と同じ（検査にならない）")
			}

			if got := Merge(Default(), Default(), cur); got != cur {
				t.Errorf("Merge =\n %+v\nwant\n %+v", got, cur)
			}
		})
	}
}

// readFile は保存先の設定を Read で読む（テストの確かめ用）。
func readFile(t *testing.T) Config {
	t.Helper()

	c, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return c
}

// TestSaveChanges は、保存の直前に読み直し、変えた項目だけを書くことを検証する
// （UT-507 ケース 5〜8。根拠: UI-115, UI-113, UI-114 / IMP-151, IMP-194。BUG-031）。
func TestSaveChanges(t *testing.T) {
	t.Run("ケース 5: ファイルが無い → このインスタンスの値をそのまま書く", func(t *testing.T) {
		isolateTempDir(t)
		cur := withOutlineWidth(Default(), 280)

		if err := SaveChanges(Default(), cur); err != nil {
			t.Fatalf("SaveChanges: %v", err)
		}
		if got := readFile(t); got != cur {
			t.Errorf("= %+v, want %+v", got, cur)
		}
	})

	t.Run("ケース 6: ファイルが壊れている → このインスタンスの値をそのまま書く（UI-113）", func(t *testing.T) {
		isolateTempDir(t)
		writeConfig(t, "{")
		cur := withOutlineWidth(Default(), 280)

		if err := SaveChanges(Default(), cur); err != nil {
			t.Fatalf("SaveChanges: %v", err)
		}
		if got := readFile(t); got != cur {
			t.Errorf("= %+v, want %+v", got, cur)
		}
	})

	t.Run("ケース 7: Light を選ぶ前から開いていたウィンドウを、選んだ後に閉じる（BUG-031 の戻り方 2）", func(t *testing.T) {
		isolateTempDir(t)
		writeConfig(t, `{"theme":""}`)
		// A と B は同じ設定で起動した（どちらも base は Default()）。
		// B でテーマを Light にして保存する
		if err := SaveChanges(Default(), withTheme(Default(), "light")); err != nil {
			t.Fatalf("B の保存: %v", err)
		}
		// A はテーマに触れず、アウトラインを閉じて終了する
		a := Default()
		a.OutlineVisible = false
		if err := SaveChanges(Default(), a); err != nil {
			t.Fatalf("A の保存: %v", err)
		}

		got := readFile(t)
		if got.Theme != "light" || got.OutlineVisible {
			t.Errorf("theme = %q, outlineVisible = %v; want \"light\", false", got.Theme, got.OutlineVisible)
		}
	})

	t.Run("ケース 8: 前回の保存の後に変えていない項目は、他のインスタンスが後から書いた値を残す", func(t *testing.T) {
		isolateTempDir(t)
		// A がテーマを Light にして保存した（A の base は Light になる）
		a := withTheme(Default(), "light")
		if err := SaveChanges(Default(), a); err != nil {
			t.Fatalf("A の 1 回目の保存: %v", err)
		}
		// その後に B が Dark を選んで保存した
		writeConfig(t, `{"theme":"dark"}`)
		// A が何も変えずに終了する（終了時の保存。UI-114）
		if err := SaveChanges(a, a); err != nil {
			t.Fatalf("A の終了時の保存: %v", err)
		}

		if got := readFile(t).Theme; got != "dark" {
			t.Errorf("theme = %q, want \"dark\"（A は前回の保存から変えていない）", got)
		}
	})
}

// TestRead は、保存の直前の読み直しを検証する（UT-507 ケース 9〜11。根拠: UI-113, UI-115 / IMP-151）。
//
// **Load と違い、土台にできないときはエラーを返す。** 既定値を土台にすると、変えていない項目を既定値で
// 書き潰す。
func TestRead(t *testing.T) {
	t.Run("ケース 9: ファイルが無い → エラー", func(t *testing.T) {
		isolateTempDir(t)
		if _, err := Read(); err == nil {
			t.Error("エラーにならない")
		}
	})

	t.Run("ケース 10: 壊れている → エラー", func(t *testing.T) {
		isolateTempDir(t)
		writeConfig(t, "{")
		if _, err := Read(); err == nil {
			t.Error("エラーにならない")
		}
	})

	t.Run("ケース 11: 一部の項目と範囲外の値 → 欠けた項目は既定値、範囲外は既定値へ丸める（Load と同じ）", func(t *testing.T) {
		isolateTempDir(t)
		writeConfig(t, `{"theme":"dark","outlineWidth":10}`)
		want := withTheme(Default(), "dark")
		if got := readFile(t); got != want {
			t.Errorf("= %+v, want %+v", got, want)
		}
	})
}

// TestThemeFromPatch は、フロントエンドから受け取ったテーマの当て方を検証する
// （UT-508。根拠: FR-071, FR-070, UI-110 / IMP-303, IMP-151。BUG-031）。
func TestThemeFromPatch(t *testing.T) {
	tests := []struct {
		name           string
		current, patch string
		want           string
	}{
		// ケース 1・2: 保存したテーマのあるウィンドウで、テーマに触れずにペインを開閉した（BUG-031 の戻り方 1）
		{"ケース 1: light に空文字 → light のまま", "light", "", "light"},
		{"ケース 2: dark に空文字 → dark のまま", "dark", "", "dark"},
		// ケース 3: 設定が無い初回起動で、テーマに触れずにペインを開閉した → OS の設定への追従を保つ（FR-071）
		{"ケース 3: 空文字に空文字 → 空文字のまま", "", "", ""},
		// ケース 4〜6: このウィンドウで利用者が選んだ
		{"ケース 4: 空文字に light → light", "", "light", "light"},
		{"ケース 5: light に dark → dark", "light", "dark", "dark"},
		{"ケース 6: dark に light → light", "dark", "light", "light"},
		// ケース 7: 一覧に無い値は受け取らない
		{"ケース 7: light に blue → light のまま", "light", "blue", "light"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ThemeFromPatch(tt.current, tt.patch); got != tt.want {
				t.Errorf("ThemeFromPatch(%q, %q) = %q, want %q", tt.current, tt.patch, got, tt.want)
			}
		})
	}
}
