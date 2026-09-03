// vendorupdate は同梱資産（Mermaid / KaTeX / PlantUML）を最新安定版へ更新する
// （BR-043 の手順 1〜3）。
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
// # 一部だけ更新しない
//
// 3 つのうち 1 つでも取れなければ、**どれも更新しない**。半端に新しい
// 組み合わせを作らないためで、リリースノート（BR-051）に書く内容も
// 「更新した / しなかった」の 2 通りで済む。
//
// # 中止を意味する 2 つの理由
//
// `license-rejected` と `notice-mismatch` は、**取得の失敗とは扱いが違う**。
// 前者は NFR-051 が許容しないライセンスの版を掴んだこと、後者は同梱物の
// 構成が変わったことを指し、**どちらもリリースを中止する**（BR-043）。
// 終了コードは 0 のままとし、reason で呼び出し側へ伝える。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	// preserve は入れ替えのときに**残す**階層（BR-042）。
	//
	// PlantUML の licenses/ には Viz.js / Graphviz / Expat の全文が入って
	// おり、**自動更新はこれを取りに行かない**（人が 1 度取得してコミットする）。
	// install は入れ替えの前にディレクトリごと消すため、ここに挙げておかないと
	// **CI が回った瞬間に全文が消える。** NFR-051（著作権表示と全文の保持）に
	// 直接触れる。
	preserve []string
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
	{
		name: plantUMLName,
		pkg:  "@plantuml/core",
		dir:  plantUMLDir,
		// **emoji.js と openiconic.js は取らない**（AR-020）。同梱するのは
		// 描画に要る 2 本だけである。
		files: map[string]string{
			"plantuml.js":   "plantuml.js",
			"viz-global.js": "viz-global.js",
			"LICENSE":       "LICENSE",
		},
		preserve: []string{"licenses"},
	},
}

// PlantUML は告知の照合（BR-043）で名前を直に使うため、定数で持つ。
const (
	plantUMLName = "PlantUML"
	plantUMLDir  = "plantuml"
)

// allowedSPDX は NFR-051 が許容するライセンス種別。
//
// **増やすのは意図的な行為とする。** 一覧に無いものが現れたら、その資産を
// 更新せずコミット済みの版でリリースを続ける（BR-043）。判断を人へ戻す。
// `@plantuml/core` は 1.2026.5 以前が GPL-3.0-or-later であり、**この検査が
// 無いと上流のライセンス変更が黙って入る。**
var allowedSPDX = map[string]bool{
	"mit":          true,
	"bsd-2-clause": true,
	"bsd-3-clause": true,
	"apache-2.0":   true,
	"isc":          true,
	"ofl-1.1":      true,
	"epl-2.0":      true,
}

// result は 1 件の資産の取得結果。
type result struct {
	name   string
	before string
	after  string
	source string
	spdx   string // npm が申告するライセンス種別（BR-043, NFR-051）
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

	// **ライセンスを確かめる**（BR-043, NFR-051）。許容しない版を掴んだら
	// 更新せずに続ける。取得の失敗とは扱いが違い、中止を意味する。
	if bad := rejected(results); len(bad) > 0 {
		fmt.Printf("\n許容しないライセンスの版を掴んだ: %s\n", strings.Join(bad, ", "))
		fmt.Println("更新しない。リリースを中止する（BR-043, NFR-051）")

		return emit(false, "license-rejected", results)
	}

	if unchanged(results) {
		fmt.Println("\nどれも最新版だった。更新しない")

		return emit(false, "already-latest", results)
	}

	// **告知を照合する**（BR-043）。入れ替えの前に行う。ここで中止すれば
	// リポジトリの状態は変わらない。
	current, err = collate(*dir, filepath.Join(staging, plantUMLDir, "viz-global.js"), current)
	if err != nil {
		fmt.Println("\n告知と記録が食い違う。リリースを中止する（BR-043）")
		fmt.Println(" ", err)

		return emit(false, "notice-mismatch", results)
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

		// **消す前に、残すものを staging へ写す**（BR-042）。
		for _, keep := range a.preserve {
			if err := copyTree(filepath.Join(target, keep), filepath.Join(staging, a.dir, keep)); err != nil {
				return fmt.Errorf("%s/%s を残せない: %w", a.dir, keep, err)
			}
		}

		// **消してから置く。** 版によってフォントの構成が変わりうるため、
		// 上書きだけでは古いファイルが残る。
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("%s を消せない: %w", target, err)
		}

		if err := moveDir(filepath.Join(staging, a.dir), target); err != nil {
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
				// 取得したライセンス種別も書き戻す（BR-043 の手順 3）。
				if got.spdx != "" {
					e.SPDX = got.spdx
				}
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
	Name      string `json:"name"`
	Version   string `json:"version"`
	SPDX      string `json:"spdx"`
	License   string `json:"license"`
	Source    string `json:"source"`
	Fetched   string `json:"fetched"`
	BundledIn string `json:"bundledIn,omitempty"`
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

// rejected は NFR-051 が許容しないライセンスの資産名を返す（BR-043）。
//
// **申告が空のものも許容しない。** 分からないものを通すと、検査を置いた
// 意味がなくなる。
func rejected(results []result) []string {
	var bad []string

	for _, got := range results {
		if !allowedSPDX[strings.ToLower(strings.TrimSpace(got.spdx))] {
			bad = append(bad, fmt.Sprintf("%s %s (%s)", got.name, got.after, orDash(got.spdx)))
		}
	}

	return bad
}

// moveDir は staging の階層を本番へ移す。
//
// **os.Rename はファイルシステムをまたげない。** Windows で TEMP が
// リポジトリと別のドライブにあると必ず失敗する（実測: C: と P:。2026-09-03）。
// しかもこの関数は移す前に本番を消しており、**失敗すると資産が消えたまま残る。**
// 落ちる余地を作らないよう、rename がだめなら複製へ落とす。
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	return copyTree(src, dst)
}

// copyTree は src の階層を dst へ複製する（BR-042 の preserve）。
//
// src が無い場合は何もしない。初回の取得ではまだ置かれていないため。
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	if !info.IsDir() {
		return copyFile(src, dst)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	for _, e := range entries {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // 読むだけ

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck

		return err
	}

	return out.Close()
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
