package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// noticeHead は viz-global.js の先頭から読む量。
//
// 本体は 1.4 MiB あり、しかも改行の少ない minify 済みである。告知は先頭の
// 8 行に収まるため、全部を読み込まない。
const noticeHead = 4 << 10

// containsMarker は同梱物の並びが始まる行（BR-043）。
const containsMarker = "This distribution contains other software in object code form:"

// notice は viz-global.js 先頭の告知（BR-042, BR-043）。
//
//	/*!
//	Viz.js 3.24.0
//	Copyright (c) Michael Daines
//
//	This distribution contains other software in object code form:
//	Graphviz https://www.graphviz.org
//	Expat https://libexpat.github.io
//	*/
type notice struct {
	name     string   // 配布物そのものの名前。"Viz.js"
	version  string   // "3.24.0"
	holder   string   // 著作権者。"Michael Daines"
	contains []string // 同梱されているもの。["Graphviz", "Expat"]
}

// projects は告知が名指しするプロジェクトを返す（BR-043 の照合 1）。
//
// **1 行目の配布物そのものを数に含める。** 告知は Viz.js を「配布物」として
// 1 行目に書き、`contains` の下には Graphviz と Expat の 2 件しか並べない。
// 1 行目を数え忘れると、**正しい告知に対して notice-mismatch を出して
// リリースを止めてしまう。**
func (n notice) projects() []string {
	all := append([]string{n.name}, n.contains...)
	sort.Strings(all)

	return all
}

// readNotice は viz-global.js の先頭の告知を読む。
//
// **この告知が `bundledIn` の一覧の正である**（BR-042）。3 件の全文は
// バンドルの中に入っておらず、tarball から取り出すこともできないため、
// 同梱の構成が変わったことはここでしか捕まえられない。
func readNotice(path string) (notice, error) {
	var got notice

	file, err := os.Open(path)
	if err != nil {
		return got, fmt.Errorf("viz-global.js を開けない: %w", err)
	}
	defer file.Close() //nolint:errcheck // 読むだけ

	buf := make([]byte, noticeHead)
	read, err := io.ReadFull(file, buf)
	if err != nil && read == 0 {
		return got, fmt.Errorf("viz-global.js を読めない: %w", err)
	}

	lines := strings.Split(string(buf[:read]), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "/*!" {
		return got, fmt.Errorf("viz-global.js の先頭が告知で始まっていない（BR-042 の改変禁止に触れていないか）")
	}

	inContains := false

	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)

		if line == "*/" {
			break
		}

		switch {
		case line == "":

		case line == containsMarker:
			inContains = true

		case inContains:
			// "Graphviz https://www.graphviz.org" の形。名前だけを取る。
			got.contains = append(got.contains, strings.Fields(line)[0])

		case strings.HasPrefix(line, "Copyright"):
			got.holder = copyrightHolder(line)

		case got.name == "":
			// "Viz.js 3.24.0"
			if fields := strings.Fields(line); len(fields) >= 2 {
				got.name, got.version = fields[0], fields[1]
			}
		}
	}

	if got.name == "" || got.version == "" || got.holder == "" || len(got.contains) == 0 {
		return got, fmt.Errorf("viz-global.js の告知を読み取れない（形が変わった可能性）")
	}

	return got, nil
}

// copyrightHolder は著作権表示から権利者の名前だけを取り出す（BR-043 の照合 2）。
//
// **年を含めて比べてはならない。** 告知は `Copyright (c) Michael Daines` と
// 年を書かないが、全文のほうは `Copyright (c) 2014-2018 Michael Daines` と
// 年を持つ。行そのものを部分文字列として探すと、**正しい組み合わせに対して
// 毎回リリースが止まる**（実測で確認。2026-09-03）。
//
// 捕まえたいのは「上流で権利者が変わった」「置いてある全文が別物である」で
// あり、それは権利者の名前で判定できる。年の更新は権利者の変更ではない。
func copyrightHolder(line string) string {
	rest := strings.TrimPrefix(line, "Copyright")
	rest = strings.TrimSpace(rest)
	rest = strings.TrimPrefix(rest, "(c)")
	rest = strings.TrimPrefix(rest, "(C)")
	rest = strings.TrimPrefix(rest, "©")
	rest = strings.TrimSpace(rest)

	// 先頭に並ぶ年・年範囲・区切りを落とす。
	fields := strings.Fields(rest)
	for len(fields) > 0 && isYearish(fields[0]) {
		fields = fields[1:]
	}

	return strings.Join(fields, " ")
}

// isYearish は年・年範囲として読める語かどうかを返す（2014、2014-2018、2014, など）。
func isYearish(s string) bool {
	s = strings.TrimSuffix(s, ",")
	if s == "" {
		return false
	}

	for _, r := range s {
		if (r < '0' || r > '9') && r != '-' {
			return false
		}
	}

	return strings.ContainsAny(s, "0123456789")
}

// collate は告知と vendor.json を突き合わせる（BR-043）。
//
//	1  告知が名指しするものが bundledIn の一覧と過不足なく一致する → 中止
//	2  告知の著作権者が license の指す全文に含まれる               → 中止
//	3  版が告知と一致する                                          → 書き戻す
//
// **この照合が、3 件の全文を「取りに行かない」ことの代償を埋めている**
// （BR-042）。全文を毎回取得しない代わりに、同梱の構成が変わったことは必ず
// 検出する。逆に、同じ 3 件のまま上流が全文だけを差し替えた場合は捕まえられない。
func collate(vendorDir, vizPath string, entries []entry) (updated []entry, err error) {
	got, err := readNotice(vizPath)
	if err != nil {
		return nil, err
	}

	// ---- 1. 同梱の構成
	recorded := bundledNames(entries, plantUMLName)
	named := got.projects()

	if !sameStrings(recorded, named) {
		return nil, fmt.Errorf(
			"告知と vendor.json の同梱物が食い違う（告知: %s / 記録: %s）"+
				"。新しい同梱物のライセンスを表示しないまま配布することになる（NFR-051, FR-101）",
			strings.Join(named, ", "), strings.Join(recorded, ", "))
	}

	// ---- 2. 著作権者
	self := findEntry(entries, got.name)
	if self == nil {
		return nil, fmt.Errorf("vendor.json に %s の記録が無い", got.name)
	}

	text, err := os.ReadFile(filepath.Join(vendorDir, filepath.FromSlash(self.License)))
	if err != nil {
		return nil, fmt.Errorf("%s のライセンス全文を読めない: %w", got.name, err)
	}

	if !strings.Contains(string(text), got.holder) {
		return nil, fmt.Errorf(
			"告知の著作権者 %q が %s に見当たらない"+
				"。上流で権利者が変わったか、置いてある全文が別物である",
			got.holder, self.License)
	}

	// ---- 3. 版は書き戻す（失敗にしない）
	updated = make([]entry, 0, len(entries))
	for _, e := range entries {
		if e.Name == got.name && e.Version != got.version {
			fmt.Printf("  告知に合わせて %s の版を %s -> %s へ直す\n", e.Name, orDash(e.Version), got.version)
			e.Version = got.version
		}

		updated = append(updated, e)
	}

	return updated, nil
}

// bundledNames は bundledIn が host の記録の名前を返す。
func bundledNames(entries []entry, host string) []string {
	var names []string

	for _, e := range entries {
		if e.BundledIn == host {
			names = append(names, e.Name)
		}
	}

	sort.Strings(names)

	return names
}

func findEntry(entries []entry, name string) *entry {
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}

	return nil
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
