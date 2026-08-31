// Package session は、Wails に依存しない起動時の判断・表示履歴・パス算出を担う。
//
// これらを app.go ではなくここへ置くのは、app.go のロジックが package main の
// テストとなり、テストバイナリに Wails（Linux では cgo と WebKitGTK）が
// リンクされてしまうためである（IMP-012, UT-002）。
package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
)

// Startup は起動時に決定した表示対象とツリールートを表す（IMP-193）。
type Startup struct {
	// TreeRoot はファイルツリーの起点となるディレクトリの絶対パス。
	// 表示対象が決まらない場合でも必ず埋まる。
	TreeRoot string

	// Initial は最初に表示する文書の絶対パス。表示対象がなければ空文字。
	Initial string

	// Requested は引数で指定されたパスの絶対形。**解決に失敗した場合も残す。**
	// ウィンドウは必ず開くため（FR-012）、どのパスが開けなかったのかを
	// 伝える必要がある。引数がない場合は空文字。
	Requested string
}

// ResolveStartup は起動時の表示対象とツリールートを決定する（FR-012, FR-013, IMP-193）。
//
// args はオプションを取り除いた位置引数のみを受け取る。--version / --help の
// 処理は main.go が Wails の起動前に済ませる（IMP-193 の 1）。
// 引数が 2 つ以上ある場合、2 つ目以降は無視する（E2E-105 のケース 4）。
//
// cwd と exeDir を引数で受け取るのは、実行環境に依存する値を関数内で直接
// 取得しないためである（UT-035）。os.Getwd と os.Executable の呼び出しと、
// 実行ファイルのシンボリックリンク解決は main.go 側で行う（FR-013, IMP-193）。
//
// 指定されたパスが存在しない・読めない場合はエラーを返すが、そのときも
// TreeRoot は埋めて返す。ウィンドウは必ず開く必要があるためである（FR-012）。
// 呼び出し側はエラーを errors.Is(err, fs.ErrNotExist) 等で判別し、
// 状態画面 welcome とステータス表示に振り分ける（IMP-193 の表）。
func ResolveStartup(args []string, cwd, exeDir string) (Startup, error) {
	cwd = filepath.Clean(cwd)

	if len(args) > 0 && args[0] != "" {
		return resolveFromArg(args[0], cwd)
	}
	return resolveWithoutArg(cwd, exeDir), nil
}

// resolveFromArg は引数でパスが指定された場合の解決を行う（FR-012、IMP-193 の 2）。
func resolveFromArg(arg, cwd string) (Startup, error) {
	target := arg
	if !filepath.IsAbs(target) {
		// process のカレントディレクトリではなく、渡された cwd を基準にする。
		// filepath.Abs を使うと実行環境に依存してしまう（UT-035）。
		target = filepath.Join(cwd, target)
	}
	target = filepath.Clean(target)

	info, err := os.Stat(target)
	if err != nil {
		// 起動は継続させるため、ツリールートだけは決めて返す（FR-012）。
		return Startup{TreeRoot: cwd, Requested: target},
			fmt.Errorf("cannot open %s: %w", target, err)
	}

	if info.IsDir() {
		st := Startup{TreeRoot: target, Requested: target}
		if readme, ok := FindReadme(target); ok {
			st.Initial = readme
		}
		return st, nil
	}

	// ファイルの場合、Markdown かどうかはここで判定しない。
	// 拡張子が対象外であることは document.Load が ErrNotMarkdown として
	// 報告する（IMP-102）。判定を 2 箇所に分散させないため、ここでは通す。
	return Startup{TreeRoot: filepath.Dir(target), Initial: target, Requested: target}, nil
}

// resolveWithoutArg は引数がない場合の探索を行う（FR-013、IMP-193 の 3〜4）。
//
// カレントディレクトリを先に見るのは、PATH に置いた MarkView をシェルから
// 使う場合に、意図しない場所の README が開くのを避けるためである。
// 実行ファイルの場所を次に見るのは、「実行ファイルと README を同じフォルダに
// 入れて配布し、受領者はダブルクリックするだけ」という UC-01 を、
// カレントがホームや / になる環境でも成立させるためである（FR-013 の TIP）。
func resolveWithoutArg(cwd, exeDir string) Startup {
	if readme, ok := FindReadme(cwd); ok {
		return Startup{TreeRoot: cwd, Initial: readme}
	}

	// カレントと実行ファイルの場所が同一なら、探索は 1 回で済ませる（FR-013）。
	if exeDir != "" && !SamePath(cwd, exeDir) {
		exeDir = filepath.Clean(exeDir)
		if readme, ok := FindReadme(exeDir); ok {
			return Startup{TreeRoot: exeDir, Initial: readme}
		}
	}

	// どちらにも見つからない。ツリールートはカレントとし、表示対象なしとする。
	return Startup{TreeRoot: cwd}
}

// FindReadme はディレクトリ直下から README を探す（FR-013, IMP-193）。
//
// 完全一致 "README.md" を優先し、次に大文字小文字を無視した一致のうち
// 名前の昇順で先頭のものを返す。見つかった場合は dir を含む絶対パスを返す。
//
// dir が読めない場合は「見つからなかった」として扱う。起動を失敗させない
// ためであり、読めない理由の通知は呼び出し側の責務ではない（FR-013 の 3）。
func FindReadme(dir string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, e.Name())
	}

	name, ok := pickReadme(names)
	if !ok {
		return "", false
	}
	return filepath.Join(dir, name), true
}

// pickReadme は名前の一覧から README を 1 つ選ぶ（FR-013）。
//
// FindReadme から切り出しているのは、選択規則をファイルシステムなしで
// 検証できるようにするためである。大文字小文字だけが異なる複数の README は
// Windows のファイルシステムでは同時に作れず、実ファイルでは検証できない
// （UT-802 のケース 3 と 4 が Linux 限定である理由）。
func pickReadme(names []string) (string, bool) {
	var candidates []string
	for _, n := range names {
		if !mdfile.IsMarkdown(n) {
			continue
		}
		stem := strings.TrimSuffix(n, filepath.Ext(n))
		if !strings.EqualFold(stem, "README") {
			continue
		}
		candidates = append(candidates, n)
	}
	if len(candidates) == 0 {
		return "", false
	}

	// 完全一致を最優先する。大小の違う複数が並んだときに、
	// 綴りが正規のものを選ぶため（FR-013）。
	for _, n := range candidates {
		if n == "README.md" {
			return n, true
		}
	}

	// 次いで名前の昇順で先頭。比較はバイト順とし、ロケールに依存させない。
	sort.Strings(candidates)
	return candidates[0], true
}
