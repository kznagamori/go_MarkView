package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// isolateTempDir は設定の保存先をテスト専用の場所へ差し替える（UT-035）。
//
// os.TempDir() が見る環境変数は OS で異なるため、両方を設定する。
// t.Setenv はテスト終了時に元へ戻す（UT-017）。
func isolateTempDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("TMPDIR", dir) // Linux / macOS
	t.Setenv("TMP", dir)    // Windows
	t.Setenv("TEMP", dir)   // Windows
	return dir
}

// writeConfig はテスト用の設定ファイルを書く。
func writeConfig(t *testing.T, content string) {
	t.Helper()

	path, err := Path()
	if err != nil {
		t.Fatalf("保存先を解決できない: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("設定ファイルを書けない: %v", err)
	}
}

// TestDefault は既定値の実数値を固定する（UI-110）。
//
// 既定値は利用者が最初に見る状態そのものであり、うっかり変わると気づきにくい。
func TestDefault(t *testing.T) {
	d := Default()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"テーマは OS 追従（空文字）", d.Theme, ""},
		{"表示倍率", d.Zoom, 100},
		{"アウトラインは表示", d.OutlineVisible, true},
		{"ファイルツリーは非表示", d.FileTreeVisible, false},
		{"アウトライン幅", d.OutlineWidth, 240},
		{"ファイルツリー幅", d.FileTreeWidth, 260},
		{"ウィンドウ幅", d.WindowWidth, 1280},
		{"ウィンドウ高さ", d.WindowHeight, 860},
		{"最大化しない", d.WindowMaximized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("= %v, want %v", tt.got, tt.want)
			}
		})
	}
}

// TestConfig_NoWindowPosition は、保存してはならない項目が構造体にないことを
// 検証する（UT-505。根拠: UI-111, NFR-042 / IMP-150）。
//
// **将来フィールドが追加されたら落ちるテストであることに意味がある。**
// 構造体にフィールドがなければ保存も復元も起こり得ない、という構造的な保証を
// 守るためのもので、キーの集合を完全一致で比べている。
func TestConfig_NoWindowPosition(t *testing.T) {
	data, err := json.Marshal(Default())
	if err != nil {
		t.Fatalf("JSON へ書き出せない: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("JSON を読めない: %v", err)
	}

	want := map[string]bool{
		"theme": true, "zoom": true,
		"outlineVisible": true, "fileTreeVisible": true,
		"outlineWidth": true, "fileTreeWidth": true,
		"windowWidth": true, "windowHeight": true, "windowMaximized": true,
	}

	for k := range m {
		if !want[k] {
			t.Errorf("設定に想定外のキー %q がある。UI-111 / NFR-042 が禁じる項目でないか確認すること", k)
		}
	}
	for k := range want {
		if _, ok := m[k]; !ok {
			t.Errorf("設定に %q がない（UI-110）", k)
		}
	}

	// 具体的に禁じられているキーは名指しでも見る。
	for _, ng := range []string{
		"x", "y", "positionX", "positionY", "windowX", "windowY",
		"path", "file", "lastFile", "recent", "history", "treeRoot", "search",
	} {
		if _, ok := m[ng]; ok {
			t.Errorf("保存してはならないキー %q がある（UI-111, NFR-042）", ng)
		}
	}
}

// TestLoad_IgnoresForbiddenKeys は UT-505 ケース 3 を検証する。
//
// 位置を含む JSON を読み込んでも、構造体にフィールドがないため無視される。
func TestLoad_IgnoresForbiddenKeys(t *testing.T) {
	isolateTempDir(t)
	writeConfig(t, `{"zoom":150,"x":100,"y":200,"lastFile":"/secret/a.md"}`)

	c := Load()

	if c.Zoom != 150 {
		t.Errorf("Zoom = %d, want 150（既知の項目は読める）", c.Zoom)
	}

	// 読み込んだ設定を書き出しても、禁じた項目は現れない。
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, ng := range []string{`"x"`, `"y"`, "lastFile", "secret"} {
		if strings.Contains(string(data), ng) {
			t.Errorf("書き出した設定に %q が含まれている: %s", ng, data)
		}
	}
}

// TestLoad_Fallback は設定が読めない場合の既定値へのフォールバックを検証する
// （UT-502。根拠: UI-113 / IMP-151）。
//
// **Load はエラーを返さない。** テンポラリディレクトリは OS のクリーンアップで
// 消えうるため、「設定がない」は異常ではなく通常の状態である。
func TestLoad_Fallback(t *testing.T) {
	tests := []struct {
		name    string
		content string // 空文字はファイルを作らないことを示す
		create  bool
	}{
		{"存在しない", "", false},
		{"空のファイル", "", true},
		{"壊れた JSON", "{", true},
		{"閉じていない配列", "[1,2", true},
		{"null", "null", true},
		{"JSON ではない", "not json at all", true},
		{"配列", "[1,2,3]", true},
		// 有効な項目を読んだ後に型が違う項目が来る。途中まで反映した値を
		// そのまま返す実装は、ここで落ちる。
		{"途中まで有効な JSON", `{"theme":"dark","zoom":"abc"}`, true},
		{"数値", "42", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateTempDir(t)
			if tt.create {
				writeConfig(t, tt.content)
			}

			if got := Load(); got != Default() {
				t.Errorf("Load() = %+v, want %+v", got, Default())
			}
		})
	}
}

// TestLoad_Partial は部分的な設定の読み込みを検証する
// （UT-503。根拠: UI-113 / IMP-151）。
func TestLoad_Partial(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(*testing.T, Config)
	}{
		{
			name:    "一部の項目だけ",
			content: `{"theme":"dark"}`,
			check: func(t *testing.T, c Config) {
				if c.Theme != "dark" {
					t.Errorf("Theme = %q, want dark", c.Theme)
				}
				if c.Zoom != Default().Zoom {
					t.Errorf("Zoom = %d, want 既定値 %d", c.Zoom, Default().Zoom)
				}
				if c.OutlineWidth != Default().OutlineWidth {
					t.Errorf("OutlineWidth = %d, want 既定値", c.OutlineWidth)
				}
			},
		},
		{
			name:    "未知のキーを含む",
			content: `{"zoom":150,"unknownKey":"x","another":123}`,
			check: func(t *testing.T, c Config) {
				if c.Zoom != 150 {
					t.Errorf("Zoom = %d, want 150（未知のキーは無視する）", c.Zoom)
				}
			},
		},
		{
			name:    "型の違う値",
			content: `{"zoom":"abc"}`,
			check: func(t *testing.T, c Config) {
				if c != Default() {
					t.Errorf("Load() = %+v, want 既定値 %+v", c, Default())
				}
			},
		},
		{
			// 読み込みの経路でも Normalize が働くことを見る（UI-113）。
			name:    "範囲外の値は丸められる",
			content: `{"zoom":9999,"outlineWidth":-5,"theme":"blue"}`,
			check: func(t *testing.T, c Config) {
				if c.Zoom != Default().Zoom {
					t.Errorf("Zoom = %d, want 既定値 %d", c.Zoom, Default().Zoom)
				}
				if c.OutlineWidth != Default().OutlineWidth {
					t.Errorf("OutlineWidth = %d, want 既定値 %d", c.OutlineWidth, Default().OutlineWidth)
				}
				if c.Theme != Default().Theme {
					t.Errorf("Theme = %q, want 既定値 %q", c.Theme, Default().Theme)
				}
			},
		},
		{
			name:    "false を明示した項目",
			content: `{"outlineVisible":false}`,
			check: func(t *testing.T, c Config) {
				if c.OutlineVisible {
					t.Error("OutlineVisible = true, want false（明示した false が既定値に戻っている）")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateTempDir(t)
			writeConfig(t, tt.content)

			tt.check(t, Load())
		})
	}
}

// TestNormalize は範囲外の値の丸めを検証する
// （UT-504。根拠: UI-113 / IMP-153）。
//
// **範囲外・ゼロ値・負値はすべて既定値へ置き換える。最小値へ切り詰めない。**
// 保存された値が範囲外になるのはファイルが壊れた場合であり、その値を元に
// 復元しようとするより既定値から始めるほうが確実である（IMP-153）。
func TestNormalize(t *testing.T) {
	d := Default()

	tests := []struct {
		name string
		in   Config
		want Config
	}{
		// 真偽値は範囲の概念を持たないため Normalize の対象外である。
		// ゼロ値の false を既定値の true へ戻すと、利用者が閉じた
		// アウトラインが毎回開いてしまう（UT-503 の「false を明示した項目」）。
		{"ゼロ値の構造体", Config{}, zeroNormalized()},

		// UT-504 ケース 1・2: 表示倍率
		{"倍率が 0", withZoom(d, 0), d},
		{"倍率が負", withZoom(d, -100), d},
		{"倍率が上限超え", withZoom(d, 1000), d},
		{"倍率が下限未満", withZoom(d, MinZoom-1), d},

		// UT-504 ケース 3: 境界値はそのまま
		{"倍率が下限ちょうど", withZoom(d, MinZoom), withZoom(d, MinZoom)},
		{"倍率が上限ちょうど", withZoom(d, MaxZoom), withZoom(d, MaxZoom)},

		// UT-504 ケース 4・5: ペイン幅
		{"アウトライン幅が負", withOutlineWidth(d, -10), d},
		{"アウトライン幅が下限未満", withOutlineWidth(d, MinPaneWidth-1), d},
		{"アウトライン幅が下限ちょうど", withOutlineWidth(d, MinPaneWidth), withOutlineWidth(d, MinPaneWidth)},
		{"ツリー幅が下限未満", withFileTreeWidth(d, 10), d},

		// UT-504 ケース 6: ウィンドウサイズ
		{"ウィンドウ幅が下限未満", withWindowSize(d, 100, d.WindowHeight), d},
		{"ウィンドウ幅が下限ちょうど", withWindowSize(d, MinWindowW, d.WindowHeight), withWindowSize(d, MinWindowW, d.WindowHeight)},
		{"ウィンドウ高さが下限未満", withWindowSize(d, d.WindowWidth, 100), d},
		{"ウィンドウ高さが下限ちょうど", withWindowSize(d, d.WindowWidth, MinWindowH), withWindowSize(d, d.WindowWidth, MinWindowH)},

		// UT-504 ケース 7: テーマ
		{"未知のテーマ", withTheme(d, "blue"), d},
		{"テーマが light", withTheme(d, "light"), withTheme(d, "light")},
		{"テーマが dark", withTheme(d, "dark"), withTheme(d, "dark")},
		{"テーマが大文字", withTheme(d, "Dark"), d},

		// 上限のない項目は大きくてもそのまま（ペイン幅の上限は実行時の
		// ウィンドウ幅に依存するため、フロントエンド側で制限する。IMP-153）
		{"アウトライン幅が非常に大きい", withOutlineWidth(d, 100000), withOutlineWidth(d, 100000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in
			got.Normalize()

			if got != tt.want {
				t.Errorf("Normalize() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// zeroNormalized はゼロ値の Config を Normalize した結果を返す。
//
// 数値と文字列は既定値へ戻るが、真偽値はゼロ値のまま残る。
func zeroNormalized() Config {
	c := Default()
	c.OutlineVisible = false
	return c
}

func withZoom(c Config, v int) Config          { c.Zoom = v; return c }
func withTheme(c Config, v string) Config      { c.Theme = v; return c }
func withOutlineWidth(c Config, v int) Config  { c.OutlineWidth = v; return c }
func withFileTreeWidth(c Config, v int) Config { c.FileTreeWidth = v; return c }
func withWindowSize(c Config, w, h int) Config { c.WindowWidth, c.WindowHeight = w, h; return c }

// TestSave は保存と読み戻しを検証する（UT-506。根拠: UI-112 / IMP-151）。
func TestSave(t *testing.T) {
	t.Run("保存した内容を読み戻せる", func(t *testing.T) {
		isolateTempDir(t)

		want := Default()
		want.Theme = "dark"
		want.Zoom = 150
		want.FileTreeVisible = true

		if err := Save(want); err != nil {
			t.Fatalf("Save がエラーを返した: %v", err)
		}
		if got := Load(); got != want {
			t.Errorf("Load() = %+v, want %+v", got, want)
		}
	})

	t.Run("上書き保存で置き換わる", func(t *testing.T) {
		isolateTempDir(t)

		first := Default()
		first.Zoom = 150
		if err := Save(first); err != nil {
			t.Fatal(err)
		}

		second := Default()
		second.Zoom = 200
		if err := Save(second); err != nil {
			t.Fatal(err)
		}

		if got := Load(); got.Zoom != 200 {
			t.Errorf("Zoom = %d, want 200", got.Zoom)
		}
	})

	t.Run("一時ファイルを残さない", func(t *testing.T) {
		isolateTempDir(t)

		for range 100 {
			if err := Save(Default()); err != nil {
				t.Fatalf("Save がエラーを返した: %v", err)
			}
		}

		dir, err := Dir()
		if err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) != 1 || entries[0].Name() != fileName {
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Errorf("設定ディレクトリの中身 = %v, want [%s] のみ", names, fileName)
		}
	})

	t.Run("保存に失敗しても一時ファイルを残さない", func(t *testing.T) {
		// 置き換え先と同じ名前のディレクトリを作り、Rename を失敗させる。
		isolateTempDir(t)

		dir, err := Dir()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, fileName), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := Save(Default()); err == nil {
			t.Error("Save がエラーを返さない")
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Errorf("設定ディレクトリの中身 = %v, want 一時ファイルなし", names)
		}
	})

	t.Run("人が読める整形で保存する", func(t *testing.T) {
		isolateTempDir(t)

		if err := Save(Default()); err != nil {
			t.Fatal(err)
		}

		path, err := Path()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(data), "\n  \"theme\"") {
			t.Errorf("整形されていない（UI-112）: %s", data)
		}
	})
}

// TestDir は保存先の解決を検証する（UT-501。根拠: UI-112, NFR-033 / IMP-152）。
func TestDir(t *testing.T) {
	t.Run("テンポラリディレクトリの下に作る", func(t *testing.T) {
		root := isolateTempDir(t)

		dir, err := Dir()
		if err != nil {
			t.Fatalf("Dir がエラーを返した: %v", err)
		}
		if filepath.Dir(dir) != root {
			t.Errorf("Dir() = %q, want %q の直下", dir, root)
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Errorf("ディレクトリが作られていない: %v", err)
		}
	})

	t.Run("Path はディレクトリと config.json をつなぐ", func(t *testing.T) {
		isolateTempDir(t)

		dir, err := Dir()
		if err != nil {
			t.Fatal(err)
		}
		path, err := Path()
		if err != nil {
			t.Fatal(err)
		}
		if path != filepath.Join(dir, "config.json") {
			t.Errorf("Path() = %q, want %q", path, filepath.Join(dir, "config.json"))
		}
	})

	// UT-501 ケース 6: 利用者の設定領域を使わない
	//
	// 環境変数を差し替えず、実際の保存先を見る。dirPath は作成しない。
	// Windows のテンポラリは AppData\Local\Temp の下にあるため、
	// 「appdata を含まない」では判定できない。os.UserConfigDir()
	//（Windows: AppData\Roaming、Linux: ~/.config）の下にないことを見る。
	t.Run("利用者の設定領域を使わない", func(t *testing.T) {
		got := dirPath()

		if cfgDir, err := os.UserConfigDir(); err == nil {
			if strings.HasPrefix(got, cfgDir) {
				t.Errorf("保存先 %q が利用者の設定領域 %q の下にある（NFR-033）", got, cfgDir)
			}
		}

		lower := strings.ToLower(filepath.ToSlash(got))
		for _, ng := range []string{"roaming", "/.config/", "application data"} {
			if strings.Contains(lower, ng) {
				t.Errorf("保存先 %q に %q が含まれる（NFR-033）", got, ng)
			}
		}
	})
}

// TestDirName は OS ごとのディレクトリ名を検証する（UT-501。根拠: UI-112）。
func TestDirName(t *testing.T) {
	name := dirName()

	if runtime.GOOS == "windows" {
		// %TEMP% は利用者ごとに分かれているため、名前に利用者を含めない。
		if name != "MarkView" {
			t.Errorf("dirName() = %q, want MarkView", name)
		}
		return
	}

	// /tmp は共有されるため、利用者 ID を含める（UI-112）。
	want := "MarkView-" + strconv.Itoa(os.Getuid())
	if name != want {
		t.Errorf("dirName() = %q, want %q", name, want)
	}
}

// TestDirPath_DefaultTempRoot は TMPDIR 未設定時の保存先を検証する
// （UT-501 ケース 2）。
//
// 実際に作成すると /tmp にディレクトリが残るため、パスの組み立てだけを見る。
func TestDirPath_DefaultTempRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TMPDIR は Linux / macOS の変数（UT-501 ケース 2）")
	}

	t.Setenv("TMPDIR", "")

	if got := dirPath(); !strings.HasPrefix(got, "/tmp/MarkView-") {
		t.Errorf("dirPath() = %q, want /tmp/MarkView-<uid>", got)
	}
}

// TestDir_Permissions はパーミッションを検証する（UT-501 ケース 4・5）。
//
// Windows のパーミッションビットは実質的に無視されるため、Linux でのみ確認する。
func TestDir_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows ではパーミッションビットが働かない（UT-501 ケース 4・5）")
	}

	isolateTempDir(t)

	if err := Save(Default()); err != nil {
		t.Fatalf("Save がエラーを返した: %v", err)
	}

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("ディレクトリのパーミッション = %o, want 700（UI-112）", perm)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	finfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := finfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("ファイルのパーミッション = %o, want 600（UI-112）", perm)
	}
}
