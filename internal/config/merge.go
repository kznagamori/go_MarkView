package config

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
)

// 本ファイルは、複数のインスタンスが 1 つの設定ファイルを共有するときの書き方（UI-115）と、フロントエンド
// から受け取ったテーマの扱い（FR-071, IMP-303）を持つ。どちらも BUG-031 の修正である（4.97.0。
// docs/bugs/2026-11-12-bug-031-theme-reset.md）。
//
// v1.2.0-rc.1 までは、保存のたびにインスタンスの設定を構造体まるごと書いていた。そのため、テーマに触れて
// いないインスタンスも、起動時に読んだ古いテーマを書き戻した。

// Read は設定ファイルを読む（UI-115, IMP-151）。
//
// **Load と違い、ファイルが無い・読めない・壊れているときはエラーを返す。** 保存の直前に、他のインスタンスが
// 書いた値を土台にするために使う（SaveChanges）。土台が無いときに既定値を土台にすると、このインスタンスが
// 起動時に読んだ値（変えていない項目）を既定値で書き潰してしまう。
//
// 読めた値は Load と同じく、欠けた項目を既定値で埋め（IMP-151）、範囲外の値を丸める（IMP-153）。
func Read() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	c := Default()
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("cannot decode the configuration: %w", err)
	}

	c.Normalize()
	return c, nil
}

// Merge は保存する設定を組み立てる（UI-115, IMP-151）。
//
//   - disk: 保存の直前に読み直したファイルの値（他のインスタンスが書いたものを含む）
//   - base: このインスタンスが前回ファイルとそろえた値（起動時に読んだ値か、前回保存した値）
//   - cur:  このインスタンスのいまの値
//
// **base から変わった項目だけを cur から取り、他の項目は disk の値を残す。** 同じ項目を複数のインスタンスで
// 変えたときは、後から保存したものが残る。
//
// 項目はリフレクションで並べる。Config に項目を足したときに、ここへの書き足しを忘れて「その項目だけ保存
// されない」状態を作らないためである（UT-507 が項目の数を固定する）。Config の項目はすべて比較できる型
// （文字列・真偽値・整数）である。
func Merge(disk, base, cur Config) Config {
	out := disk

	o := reflect.ValueOf(&out).Elem()
	b := reflect.ValueOf(base)
	c := reflect.ValueOf(cur)
	for i := range c.NumField() {
		if !c.Field(i).Equal(b.Field(i)) {
			o.Field(i).Set(c.Field(i))
		}
	}

	return out
}

// SaveChanges は、このインスタンスが base から変えた項目だけをファイルへ書く（UI-114, UI-115, IMP-151, IMP-194）。
//
// 保存の直前にファイルを読み直し（Read）、変えていない項目はファイルの値を残す（Merge）。**読み直した値を
// 画面や App の状態へ反映しない**——実行中に他のインスタンスの変更へ追従しないこと（UI-115）は変えない。
//
// ファイルが無い・読めない・壊れているときは cur をそのまま書く（UI-113 の「次回の保存時に上書きする」）。
//
// **読み直してから書くまでの間に他のインスタンスが書くと、その書き込みは失われる。** 錠は持たない。保存は
// 変更から 1 秒後と終了時にしか起きず、間は数ミリ秒であり、失われても設定は壊れない（一時ファイル＋リネーム。
// UI-112）。
//
// 呼び出し側は、成功したら base を cur に置き換える。失敗したら base を残し、次の保存で書き直す。
func SaveChanges(base, cur Config) error {
	out := cur
	if disk, err := Read(); err == nil {
		out = Merge(disk, base, cur)
	}
	return Save(out)
}

// ThemeFromPatch は、フロントエンドから受け取ったテーマ（UpdateConfig）を設定へ当てた結果を返す
// （FR-071, IMP-303。BUG-031）。
//
// "light" / "dark" は、このウィンドウで利用者が選んだ値として受け取る。**それ以外（空文字）は「このウィンドウ
// ではまだ選んでいない」であり、current（起動時に読んだ保存値）を変えない。** フロントエンドはテーマを
// 切り替えるまで空文字を送る（IMP-210 の state.themeExplicit）。受け取った空文字で保存値を書き換えると、
// 保存したテーマのあるウィンドウでペインを開閉しただけで、テーマが OS の設定への追従へ戻る。
//
// 設定が無い初回起動（current が空文字）では空文字のまま残り、OS の設定への追従を保つ（FR-071）。テーマを
// OS の設定への追従へ戻す操作は無い（FR-070 は Light と Dark の切り替えだけ）。
func ThemeFromPatch(current, patch string) string {
	if patch == "light" || patch == "dark" {
		return patch
	}
	return current
}
