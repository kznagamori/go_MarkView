// Package filetree はファイルツリーの読み込みと絞り込みを担う（IMP-130 系）。
//
// 依存するのは mdfile だけである（IMP-012）。
package filetree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kznagamori/go_MarkView/internal/mdfile"
)

// MaxEntriesPerDir は 1 ディレクトリあたりの表示上限（FR-032, IMP-131）。
const MaxEntriesPerDir = 1000

// ErrOutsideRoot は対象がツリールートの外にあることを示す（IMP-131）。
var ErrOutsideRoot = errors.New("target is outside the tree root")

// excludedDirs は既定で表示しないディレクトリ（FR-031, IMP-132）。
var excludedDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	".git":         true,
	"target":       true,
	"dist":         true,
	"build":        true,
}

// Node はツリーの 1 要素（IMP-130）。
type Node struct {
	Name  string `json:"name"`
	Path  string `json:"path"` // 絶対パス（IMP-025）
	IsDir bool   `json:"isDir"`
	// Kind はファイルの種類（IMP-135 の Classify）。ディレクトリは空（4.75.0。UI-030 のアイコン）。
	Kind mdfile.Kind `json:"kind"`

	Children []Node `json:"children"` // 未読込のディレクトリでは nil
	Loaded   bool   `json:"loaded"`   // 子を読み込み済みか

	// Omitted は **この要素が属する一覧から件数上限で除かれた数**（FR-032）。
	// 切り詰めが起きた場合、返すすべての要素に同じ値を入れる。0 なら全件。
	//
	// 一覧に対する印を要素側に持たせているのは、ReadDir が返すのが子の並びだけで
	// あり、親を表す値を返さないためである（IMP-131）。すべてに入れるので、
	// 並べ替えても値が失われない。
	//
	// 真偽値ではなく件数を持つのは、表示が `… and N more` だからである
	// （DSP-112）。件数を返さないと、フロントエンドは N を組み立てられない。
	Omitted int `json:"omitted"`
}

// ReadDir は dir の直下のみを読み込む。再帰しない（FR-032, IMP-131）。
//
// 絞り込みと並び順は IMP-132 に従う。表示する種類のファイルを含まないディレクトリは
// 除く（FR-031）が、判定は 1 階層下までとする（IMP-133）。
//
// show はツリーに表示する種類（FR-037。IMP-134）。**件数の上限は絞り込み後の件数に対して適用する**
// （隠している画像の数で上限に掛からない。UT-303 のケース 5）。要素の種類（ディレクトリか、どの種類の
// ファイルか）は Classify と同じ規則で決め、**決められない項目（宛先の無いシンボリックリンクなど）は
// 出さず、ほかの項目は返す**（4.78.0。UT-301 のケース 12）。ツリーの操作の検査（Check）と同じ判定を
// 使い、ツリーに出ているものは開ける・展開できるようにする（FR-036）。
func ReadDir(dir string, show Show) ([]Node, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve the directory: %w", err)
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read the directory: %w", err)
	}

	nodes := make([]Node, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue // 種類を調べるまでもなく出さない（include と同じ規則）
		}
		isDir, kind, err := classifyEntry(abs, e)
		if err != nil || !shown(name, isDir, kind, show) {
			continue
		}

		path := filepath.Join(abs, name)
		if isDir && !hasShownWithin(path, show) {
			continue
		}

		// 子は読み込まない。Children は nil、Loaded は false のままとする。
		nodes = append(nodes, Node{Name: name, Path: path, IsDir: isDir, Kind: kind})
	}

	slices.SortFunc(nodes, compareNodes)

	if len(nodes) > MaxEntriesPerDir {
		omitted := len(nodes) - MaxEntriesPerDir
		nodes = nodes[:MaxEntriesPerDir]
		for i := range nodes {
			nodes[i].Omitted = omitted
		}
	}

	return nodes, nil
}

// PathTo は root から target に至る経路上のディレクトリを順に返す（IMP-131）。
//
// 表示中ファイルまでの自動展開に使う（FR-032）。target 自身は含めない。
// target が root の直下にある場合は空スライスを返す。
//
// ファイルシステムには触れない。与えられたパスは絶対パスとみなし、Clean だけを
// 行う。存在しないパスでも経路を計算できるほうが、呼び出し側で扱いやすい。
func PathTo(root, target string) ([]string, error) {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", target, ErrOutsideRoot)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s: %w", target, ErrOutsideRoot)
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 || parts[0] == "." {
		return []string{}, nil
	}

	// 最後の要素は target 自身であるため含めない。
	dirs := make([]string, 0, len(parts)-1)
	cur := filepath.Clean(root)
	for _, p := range parts[:len(parts)-1] {
		cur = filepath.Join(cur, p)
		dirs = append(dirs, cur)
	}

	return dirs, nil
}

// include は名前の規則でエントリを表示対象とするか判定する（FR-031, IMP-132。種類は見ない）。
//
// **同じ規則をツリーの操作の検査（Check の 3）でも使う。** 2 か所に書くと、ツリーに出ているのに
// 開けないもの、出ていないのに開けるものが生まれる。
func include(name string, isDir bool) bool {
	// 名前が . で始まるものは除外する。
	if strings.HasPrefix(name, ".") {
		return false
	}
	if isDir {
		return !excludedDirs[name]
	}
	return true
}

// shown は、名前の規則（include）と種類の規則で、要素をツリーに出すかを返す（IMP-132）。
// ファイルは種類を show が含むものだけ（FR-037。v1.1.0 は Markdown だけだった）。
func shown(name string, isDir bool, kind mdfile.Kind, show Show) bool {
	if !include(name, isDir) {
		return false
	}
	return isDir || show.Includes(kind)
}

// compareNodes は並び順を決める（IMP-132）。
//
// ディレクトリを先、ファイルを後とし、それぞれ名前の昇順（大文字小文字を
// 区別しない）。綴りが同じで大小だけが違う場合は、元の名前で決着させて
// 並びを安定させる。
func compareNodes(a, b Node) int {
	if a.IsDir != b.IsDir {
		if a.IsDir {
			return -1
		}
		return 1
	}

	if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}

// hasShownWithin は dir 以下に show の種類のファイルがある可能性を返す（IMP-133。4.75.0。v1.1.0 の
// hasMarkdownWithin）。**先読みの判定は、ファイルを並べる判定と同じ Show で行う**（FR-037）——画像を
// 表示していないときに、画像しか無いディレクトリを出さない（UT-307 のケース 5〜7）。
//
// 調べるのは dir の直下と、その 1 階層下まで。**全階層は走査しない**
// （FR-032）。起動時に階層全体を読むと、巨大なディレクトリで起動が遅くなる。
//
// 見つからなくても、さらに深い階層が残っている場合は true を返す。判定できない
// ディレクトリを隠すより、展開して空だと分かるほうがよい（IMP-133）。
func hasShownWithin(dir string, show Show) bool {
	subdirs, found := scanForShown(dir, show)
	if found {
		return true
	}

	for _, sub := range subdirs {
		deeper, found := scanForShown(sub, show)
		if found {
			return true
		}
		if len(deeper) > 0 {
			// さらに下に階層がある。ここでは判定できないため表示する。
			return true
		}
	}

	return false
}

// scanForShown は dir の直下を 1 度だけ読み、表示対象のサブディレクトリと、
// 表示する種類のファイルが直下にあったかを返す。種類は ReadDir と同じ規則で決める。
//
// 読めないディレクトリは「何もない」とみなす。権限のないディレクトリを
// ツリーに出しても、展開できずに終わるためである。
func scanForShown(dir string, show Show) (subdirs []string, found bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		isDir, kind, err := classifyEntry(dir, e)
		if err != nil || !shown(name, isDir, kind, show) {
			continue
		}
		if !isDir {
			return nil, true
		}
		subdirs = append(subdirs, filepath.Join(dir, name))
	}

	return subdirs, false
}
