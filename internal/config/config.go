// Package config は設定の読み書きを担う（IMP-150 系）。
//
// 依存を持たない。設定はテンポラリディレクトリにのみ置き、%APPDATA% や
// ~/.config、レジストリには書かない（UI-112, NFR-033）。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// 値の範囲（IMP-153）。
//
// **倍率の範囲と刻みはここに置かない。** 倍率は保存されず（UI-111）Normalize の
// 対象にならない。範囲と刻みは操作の上限・下限としてフロントエンドだけが持つ
// （IMP-242, FR-081）。
const (
	MinPaneWidth = 160 // UI-030, UI-040
	MinWindowW   = 640 // UI-011
	MinWindowH   = 480
)

// Config は保存する設定（IMP-150, UI-110）。
//
// **次のフィールドを定義しない。** 構造体に存在しなければ、保存も復元も
// 起こり得ない（UI-111）。
//
//   - ウィンドウ位置（X, Y）……モニタ構成が変わったときにウィンドウが画面外へ
//     配置される事故を、構造として防ぐ
//   - 表示倍率（Zoom）……セッション内の値であり、フロントエンドだけが持つ
//     （UI-115, IMP-242）
//   - 最大化状態（WindowMaximized）……起動時は常に通常状態（UI-115）
//
// 倍率と最大化状態を外したのは多重起動のためである（UI-115）。設定ファイルは
// 全インスタンスで共有され、保存は構造体まるごとの後勝ちになる。ウィンドウごとに
// 変えて使う値を保存すると、最後に終了したウィンドウの状態が以後すべての
// ウィンドウの初期値になってしまう。
//
// 同じ理由で、表示していたファイルのパス・ツリールート・履歴・検索語も持たない
// （NFR-042）。
type Config struct {
	Theme           string `json:"theme"` // "light" | "dark" | ""（OS 追従）
	OutlineVisible  bool   `json:"outlineVisible"`
	FileTreeVisible bool   `json:"fileTreeVisible"`
	OutlineWidth    int    `json:"outlineWidth"`
	FileTreeWidth   int    `json:"fileTreeWidth"`
	WindowWidth     int    `json:"windowWidth"`
	WindowHeight    int    `json:"windowHeight"`
}

// Default は UI-110 の既定値を返す（IMP-151）。
//
// Theme は空文字とし、OS のテーマ設定に追従させる判断は呼び出し側が行う
// （FR-071）。ここで light に倒すと、OS が Dark の環境でも Light で起動する。
func Default() Config {
	return Config{
		Theme:           "",
		OutlineVisible:  true,
		FileTreeVisible: false,
		OutlineWidth:    240,
		FileTreeWidth:   260,
		WindowWidth:     1280,
		WindowHeight:    860,
	}
}

// Load は設定を読み込む（IMP-151, UI-113）。
//
// **エラーを返さない。** ファイルがない・壊れている・読めない場合は Default()
// を返す。テンポラリディレクトリは OS のクリーンアップで消えうるため、
// 「設定がない」は異常ではなく通常の状態である。
func Load() Config {
	c := Default()

	path, err := Path()
	if err != nil {
		return c
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}

	// Default() を初期値とした構造体へ読み込む。JSON に現れない項目には
	// 既定値がそのまま残る（IMP-151）。
	if err := json.Unmarshal(data, &c); err != nil {
		// 途中まで書き込まれている可能性があるため、丸ごと捨てる。
		return Default()
	}

	c.Normalize()
	return c
}

// Save は設定を保存する（IMP-151, UI-112）。
//
// 失敗しても動作は継続させるため、呼び出し側はエラーを無視してよい（UI-113）。
// 同一ディレクトリの一時ファイルへ書いてから Rename で置き換えるため、
// 途中で失敗しても壊れた設定ファイルが残らない。
func Save(c Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	// 人が読める整形で出力する（UI-112）。
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot encode the configuration: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, "config-*.json")
	if err != nil {
		return fmt.Errorf("cannot create a temporary file: %w", err)
	}
	// 置き換えに成功した後は存在しないため、この Remove は何もしない。
	// 途中で失敗した場合に一時ファイルを残さないためのもの（UT-506）。
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cannot write the configuration: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cannot close the temporary file: %w", err)
	}

	if err := os.Rename(tmp.Name(), filepath.Join(dir, fileName)); err != nil {
		return fmt.Errorf("cannot replace the configuration: %w", err)
	}
	return nil
}

// Normalize は範囲外の値を既定値へ丸める（IMP-153, UI-113）。
//
// **範囲外・ゼロ値・負値はすべて既定値へ置き換える。** 最小値へ切り詰めない。
// 保存された値が範囲外になるのはファイルが壊れた場合であり、その値を元に
// 復元しようとするより、既定値から始めるほうが確実である。
func (c *Config) Normalize() {
	d := Default()

	if c.Theme != "light" && c.Theme != "dark" {
		c.Theme = d.Theme
	}
	if c.OutlineWidth < MinPaneWidth {
		c.OutlineWidth = d.OutlineWidth
	}
	if c.FileTreeWidth < MinPaneWidth {
		c.FileTreeWidth = d.FileTreeWidth
	}
	if c.WindowWidth < MinWindowW {
		c.WindowWidth = d.WindowWidth
	}
	if c.WindowHeight < MinWindowH {
		c.WindowHeight = d.WindowHeight
	}
}
