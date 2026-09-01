// vendorupdate は同梱資産（Mermaid / KaTeX）を最新安定版へ更新する
// （BR-043 の手順 1〜2）。
//
//	go run ./scripts/vendorupdate                 # frontend/vendor/ を更新する
//	go run ./scripts/vendorupdate -dir /tmp/试し   # 別の場所へ出して確かめる
//
// **リリース CI がプレリリースタグで呼ぶ。** 開発者が明示的に更新しなくても、
// rc を打つたびに新しい Mermaid が取り込まれる。ライセンスの再生成（BR-040）
// と描画スモークテスト（BR-054）、コミットとタグの付け替えはワークフローの
// 担当であり、このスクリプトは**ファイルと vendor.json を書き換えるところまで**。
//
// # 取得に失敗しても止めない
//
// BR-043 は「取得に失敗した場合、リリースを中止しない」と定めている。
// ネットワークやレジストリの都合でリリースが止まると、利用者へ新しい版を
// 届けられなくなる。**失敗しても終了コードは 0** とし、更新したかどうかを
// 標準出力と GITHUB_OUTPUT で伝える。呼び出し側が判断する。
//
// # 片方だけ更新しない
//
// 2 つのうち 1 つでも取れなければ、**どちらも更新しない**。半端に新しい
// 組み合わせを作らないためで、リリースノート（BR-051）に書く内容も
// 「更新した / しなかった」の 2 通りで済む。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// asset は同梱資産 1 件の取得方法（BR-042）。
type asset struct {
	name string // vendor.json の name。**既存の値と一致させる**
	pkg  string // npm のパッケージ名
	dir  string // frontend/vendor/ の下のディレクトリ名

	// files は tarball 内のパス（package/ を除く）と、置き先の対応。
	files map[string]string

	// tree は丸ごと取り出す階層。KaTeX のフォントに使う。
	tree map[string]string
}

var assets = []asset{
	{
		name: "Mermaid",
		pkg:  "mermaid",
		dir:  "mermaid",
		files: map[string]string{
			"dist/mermaid.min.js": "mermaid.min.js",
			"LICENSE":             "LICENSE",
		},
	},
	{
		name: "KaTeX",
		pkg:  "katex",
		dir:  "katex",
		files: map[string]string{
			"dist/katex.min.js":  "katex.min.js",
			"dist/katex.min.css": "katex.min.css",
			"LICENSE":            "LICENSE",
		},
		// **フォントを選別しない。** CSS が参照するものが 1 つでも欠けると
		// 404 になる。合計でも 1 MB 程度に収まる。
		tree: map[string]string{"dist/fonts/": "fonts/"},
	},
}

// result は 1 件の資産の取得結果。
type result struct {
	name   string
	before string
	after  string
	source string
	err    error
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "vendorupdate:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", filepath.Join("frontend", "vendor"), "同梱資産の置き場")
	timeout := flag.Duration("timeout", 3*time.Minute, "取得の打ち切り")
	flag.Parse()

	manifest := filepath.Join(*dir, "vendor.json")

	current, err := readManifest(manifest)
	if err != nil {
		// **これは想定外**。リポジトリに必ずある（BR-042）。
		return err
	}

	// 取得は一時ディレクトリへ行い、**全部そろってから置き換える**。
	staging, err := os.MkdirTemp("", "markview-vendor-")
	if err != nil {
		return fmt.Errorf("作業ディレクトリを作れない: %w", err)
	}
	defer os.RemoveAll(staging) //nolint:errcheck // 一時ディレクトリ

	results := make([]result, 0, len(assets))
	ok := true

	for _, a := range assets {
		got := download(a, staging, *timeout, versionOf(current, a.name))
		results = append(results, got)

		if got.err != nil {
			ok = false
		}
	}

	report(results)

	if !ok {
		fmt.Println("\n更新しない。コミット済みの版でビルドを続ける（BR-043）")

		return emit(false, "fetch-failed", results)
	}

	if unchanged(results) {
		fmt.Println("\nどちらも最新版だった。更新しない")

		return emit(false, "already-latest", results)
	}

	if err := install(*dir, staging, results, current); err != nil {
		// ここまで来ての失敗は書き込み側の問題であり、握りつぶさない。
		return err
	}

	fmt.Println("\n更新した")

	return emit(true, "updated", results)
}

// install は staging の内容を本番へ移し、vendor.json を書き換える。
func install(dir, staging string, results []result, current []entry) error {
	for _, a := range assets {
		target := filepath.Join(dir, a.dir)

		// **消してから置く。** 版によってフォントの構成が変わりうるため、
		// 上書きだけでは古いファイルが残る。
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("%s を消せない: %w", target, err)
		}

		if err := os.Rename(filepath.Join(staging, a.dir), target); err != nil {
			return fmt.Errorf("%s へ移せない: %w", target, err)
		}
	}

	fetched := time.Now().UTC().Format("2006-01-02")

	updated := make([]entry, 0, len(current))

	for _, e := range current {
		for _, got := range results {
			if got.name == e.Name {
				e.Version = got.after
				e.Source = got.source
				e.Fetched = fetched
			}
		}

		updated = append(updated, e)
	}

	return writeManifest(filepath.Join(dir, "vendor.json"), updated)
}

func report(results []result) {
	fmt.Printf("%-10s %-12s %-12s %s\n", "資産", "現在", "最新", "状態")

	for _, got := range results {
		state := "更新できる"
		latest := got.after

		switch {
		case got.err != nil:
			state = "取得できない: " + got.err.Error()
			latest = "" // 分からないものを分かったように書かない
		case got.before == got.after:
			state = "最新"
		}

		fmt.Printf("%-10s %-12s %-12s %s\n", got.name, got.before, orDash(latest), state)
	}
}

func unchanged(results []result) bool {
	for _, got := range results {
		if got.before != got.after {
			return false
		}
	}

	return true
}

// emit は結果をワークフローへ渡す（BR-051 のリリースノートに使う）。
//
// **「更新しなかった」を 2 通りに分ける。** BR-051 は取得に失敗した場合に
// その旨をリリースノートへ書くことを求めており、「既に最新だった」と
// 同じ扱いにはできない。
func emit(updated bool, reason string, results []result) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return nil
	}

	text := fmt.Sprintf("updated=%t\nreason=%s\n", updated, reason)

	for _, got := range results {
		key := lower(got.name)
		text += fmt.Sprintf("%s_before=%s\n", key, got.before)
		text += fmt.Sprintf("%s_after=%s\n", key, orDash(got.after))
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("GITHUB_OUTPUT へ書けない: %w", err)
	}

	if _, err := file.WriteString(text); err != nil {
		file.Close() //nolint:errcheck

		return err
	}

	return file.Close()
}

// entry は vendor.json の 1 件（IMP-181 の VendorEntry と同じ形）。
//
// **buildinfo を import しない。** あちらは埋め込み済みの JSON を読む側で
// あり、書く側がその都合に縛られる理由がない（IMP-012）。
type entry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Fetched string `json:"fetched"`
}

func readManifest(path string) ([]entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vendor.json を読めない: %w", err)
	}

	var entries []entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("vendor.json を解析できない: %w", err)
	}

	return entries, nil
}

func writeManifest(path string, entries []entry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func versionOf(entries []entry, name string) string {
	for _, e := range entries {
		if e.Name == name {
			return e.Version
		}
	}

	return ""
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}

	return s
}

func lower(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}

	return string(out)
}
