package opener

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// 番兵エラー（IMP-021, IMP-171）。
//
// エディタのパスが絶対でない場合と、MarkView 自身が指定された場合を
// 呼び出し側が区別できるようにする。どちらも NFR-035 が禁じる指定である。
var (
	ErrNotAbsolute = errors.New("editor path must be absolute")
	ErrSelf        = errors.New("MarkView cannot be used as an editor")
)

// Editor は既知のエディタ 1 件（IMP-172, UI-103）。
//
// **Path をフロントエンドへ渡さない。** 画面に出すのは ID と Name だけで
// あり、実行ファイルのパスは Go の中で生まれて Go の中で消費される
// （IMP-309, NFR-035 の 3）。
type Editor struct {
	ID   string // "vscode" 等。設定には保存しない（UI-116）
	Name string // 画面に出す表示名。英語（UI-024）
	Path string // 見つかった絶対パス。見つからなければ空
}

// preset はプリセット表の 1 行（IMP-172）。
//
// candidates の意味は OS で異なる。Windows は探す絶対パスの候補、Linux は
// $PATH から引くコマンド名である。**先に見つかったものを採る。**
type preset struct {
	ID         string
	Name       string
	candidates []string
}

// lookup はプリセット 1 件の実行ファイルを探し、絶対パスを返す。
// 見つからなければ空文字を返す。
//
// **テストで差し替えられるように変数にしている**（UT-705）。実際にエディタが
// インストールされているかどうかに依存するテストは書かない（UT-035）。
// runCommand（IMP-170）と同じ流儀である。
var lookup = lookupPreset

// lookupPreset と presetTable の実装は OS ごとに分かれている
// （presets_windows.go / presets_other.go）。Windows は既知パスの os.Stat、
// Linux は exec.LookPath による。**どちらもレジストリを読まない**（NFR-033）。

// Editors は既知のエディタを定義順に返す（IMP-172, UI-103）。
//
// **見つからなかったものも Path が空の状態で含める。** 一覧から消すと
// 「なぜ自分のエディタが出ないのか」が分からない（UI-103）。
// **見つかったものを前へ並べ替えない。** 並びが環境や起動のたびに変わると、
// 位置で覚えられなくなる。
func Editors() []Editor {
	table := presetTable()

	editors := make([]Editor, 0, len(table))
	for _, p := range table {
		editors = append(editors, Editor{
			ID:   p.ID,
			Name: p.Name,
			Path: lookup(p),
		})
	}

	return editors
}

// OpenWith は指定した実行ファイルでファイルを開く（FR-090, IMP-171）。
//
// editor / path はいずれも絶対パスであること。**渡す引数は path 1 つだけ**と
// する（NFR-035 の 2）。行番号・起動オプション・コマンドテンプレートのいずれも
// 持たない。**一度足すと外せない。**
//
// path は呼び出し側（app.go）が保持する「画面がいま対象にしているファイル」
// である（IMP-190 の target）。**「表示中の文書」ではない。** 状態画面を出して
// いる間、表示中の文書は前に開いていたものが残っており、そちらを渡すと画面と
// 違うファイルが開く。
//
// 検査に落ちた場合、**プロセスは 1 つも起動しない。**
func OpenWith(editor, path string) error {
	// 1. editor が絶対パスである（NFR-035 の 5）。
	//    相対パスやコマンド名を許すと、$PATH や作業ディレクトリの内容で
	//    起動されるプログラムが変わる。
	if !filepath.IsAbs(editor) {
		return fmt.Errorf("%s: %w", editor, ErrNotAbsolute)
	}

	// 2. editor が存在する（UI-116）。
	//    設定に残っていても、その後にアンインストールされていることがある。
	if !isRegularFile(editor) {
		return fmt.Errorf("%s: %w", editor, ErrNotFound)
	}

	// 3. editor が MarkView 自身でない（NFR-035 の 6）。
	//    許すと、押すたびにウィンドウが増える。
	if isSelf(editor) {
		return fmt.Errorf("%s: %w", editor, ErrSelf)
	}

	// 4. path が絶対パスで、存在する。
	if !filepath.IsAbs(path) || !isRegularFile(path) {
		return fmt.Errorf("%s: %w", path, ErrNotFound)
	}

	// 5. 起動する。**引数は path 1 つだけ**であり、シェルを経由しない
	//    （NFR-035 の 2 と 4）。終了は待たない（IMP-171）。
	if err := runCommand(editor, path); err != nil {
		return fmt.Errorf("cannot open %s: %w", path, err)
	}

	return nil
}

// isRegularFile は通常ファイルとして存在するかを返す。
//
// ディレクトリやデバイスファイルを「存在する」とみなさない。os.Stat は
// シンボリックリンクを辿るため、リンク経由でも実体が通常ファイルなら真を返す。
func isRegularFile(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular()
}

// isSelf は指定された実行ファイルが MarkView 自身かを返す（NFR-035 の 6）。
//
// **os.Executable() が失敗した場合は false を返す。** 比較できないことを理由に
// 起動を止めると、正当なエディタまで開けなくなる。ここで防いでいるのは
// 「押すたびにウィンドウが増える」事故であり、安全性の最後の砦ではない
// （設定に絶対パス以外を保持しないこと（IMP-153）と対で効く）。
func isSelf(editor string) bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}

	return samePath(editor, self)
}

// samePath は 2 つのパスが同じ実行ファイルを指すかを返す。
//
// **シンボリックリンクを解決してから比べる**（IMP-171）。解決しないと、
// 自身へのリンクやジャンクションを指定して検査をすり抜けられる。
//
// 大文字小文字の扱いは IMP-025 に従い、Windows では区別しない。
// **`session.SamePath` を呼ばない。** internal/ 同士の依存は 2 系統に
// 限られており（IMP-012）、opener からは呼べないためである。
func samePath(a, b string) bool {
	a, b = resolveLinks(a), resolveLinks(b)

	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

// resolveLinks はシンボリックリンクを解決する。解決できなければ元の値を返す。
func resolveLinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}

	return path
}
