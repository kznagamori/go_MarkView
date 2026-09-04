// domids は、同梱資産が決め打ちで参照する DOM の id と、index.html が定義する
// id が交差しないことを検査する（BR-042, BR-043, IMP-202, IMP-230）。
//
//	go run ./scripts/domids
//
// **同梱資産はページ全体を見ている。** DOM の id はページと資産で共有された
// 名前空間であり、資産が `document.getElementById('...')` を決め打ちで呼ぶと、
// こちらが用意した要素を静かに書き換える。
//
// 実際に `plantuml.js` が `#status` を書き換え、**PlantUML を含む文書を開くと
// ステータス領域の子要素が DOM から消え、以降のすべての通知が出なくなっていた**
// （docs/bugs/2026-09-05-bug-006-status-id-collision.md）。
//
// **資産は改変できない**（BR-042。EPL-2.0 のファイル単位のコピーレフトと、
// BR-043 の自動更新で消えるため）。**避けるのはこちら側であり、そのためには
// 衝突したことに気づく必要がある。**
//
// 描画スモークテスト（BR-054）では捕まらない。あちらは index.html を配らず
// 独自のページを使うため、**衝突しようがない。**
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 既定の探索先。リポジトリのルートから実行する前提とする。
const (
	defaultVendorDir = "frontend/vendor"
	defaultHTMLPath  = "frontend/index.html"
)

// vendorIDRe は資産が決め打ちで参照する id を拾う。
//
// 引用符は資産によって異なる（minify の都合で変わる）ため両方を受ける。
// **変数を渡している呼び出しは拾わない。** それは決め打ちではなく、
// 呼び出し側が id を決めている（MarkView が渡す描画対象がこれに当たる）。
var vendorIDRe = regexp.MustCompile(`getElementById\(\s*(['"])([^'"]*)['"]\s*\)`)

// htmlIDRe は HTML が定義する id 属性を拾う。
//
// **前が空白であることを要求する。** これが無いと `data-id="x"` の `id` にも
// 当たり、存在しない id を検査対象にしてしまう。
var htmlIDRe = regexp.MustCompile(`(?:^|\s)id\s*=\s*(['"])([^'"]*)['"]`)

// errNoVendor は資産のディレクトリが無いことを表す番兵エラー（IMP-021）。
var errNoVendor = errors.New("vendor directory not found")

func main() {
	if err := run(defaultVendorDir, defaultHTMLPath); err != nil {
		fmt.Fprintln(os.Stderr, "domids:", err)
		os.Exit(1)
	}
}

func run(vendorDir, htmlPath string) error {
	found, err := vendorIDs(vendorDir)
	if err != nil {
		return err
	}
	ours, err := htmlIDs(htmlPath)
	if err != nil {
		return err
	}

	hits := intersect(found, ours)

	fmt.Printf("同梱資産が決め打ちする id: %d 件 %v\n", len(found), sortedKeys(found))
	fmt.Printf("%s の id: %d 件\n", htmlPath, len(ours))

	if len(hits) == 0 {
		fmt.Println("交差なし。OK")
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "同梱資産と衝突する id が %d 件ある。index.html 側の名前を変えること（BR-042, IMP-202）\n", len(hits))
	for _, id := range hits {
		fmt.Fprintf(&b, "  id=%q  ← %s\n", id, strings.Join(found[id], ", "))
	}
	return errors.New(strings.TrimRight(b.String(), "\n"))
}

// vendorIDs は資産の JavaScript から決め打ちの id を集める。
//
// 戻り値は id から、それを含むファイルの一覧への対応。**どのファイルが
// 原因かが分からないと、資産を差し替える判断ができない。**
//
// 走査するのは `.js` だけとする。CSS や JSON に `getElementById` の文字列が
// 現れても、それは実行されない。
func vendorIDs(dir string) (map[string][]string, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("%w: %s", errNoVendor, dir)
	}

	found := map[string][]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".js") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		rel := filepath.ToSlash(path)
		for _, m := range vendorIDRe.FindAllStringSubmatch(string(data), -1) {
			id := m[2]
			if id == "" {
				continue
			}
			// 同じファイルに何度現れても 1 件として扱う。
			if !contains(found[id], rel) {
				found[id] = append(found[id], rel)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// htmlIDs は HTML が定義する id を集める。重複は取り除く。
func htmlIDs(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var ids []string
	for _, m := range htmlIDRe.FindAllStringSubmatch(string(data), -1) {
		id := m[2]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// intersect は資産の決め打ちと自前の id の交差を返す。
func intersect(found map[string][]string, ours []string) []string {
	var hits []string
	for _, id := range ours {
		if _, ok := found[id]; ok {
			hits = append(hits, id)
		}
	}
	sort.Strings(hits)
	return hits
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
