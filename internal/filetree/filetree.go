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
// 絞り込みと並び順は IMP-132 に従う。Markdown を含まないディレクトリは
// 除く（FR-031）が、判定は 1 階層下までとする（IMP-133）。
func ReadDir(dir string) ([]Node, error) {
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
		name, isDir := e.Name(), e.IsDir()
		if !include(name, isDir) {
			continue
		}

		path := filepath.Join(abs, name)
		if isDir && !hasMarkdownWithin(path) {
			continue
		}

		// 子は読み込まない。Children は nil、Loaded は false のままとする。
		nodes = append(nodes, Node{Name: name, Path: path, IsDir: isDir})
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

// include はエントリを表示対象とするか判定する（FR-031, IMP-132）。
func include(name string, isDir bool) bool {
	// 名前が . で始まるものは除外する。
	if strings.HasPrefix(name, ".") {
		return false
	}
	if isDir {
		return !excludedDirs[name]
	}

	// ファイルは Markdown のみ（IMP-105）。
	return mdfile.IsMarkdown(name)
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

// hasMarkdownWithin は dir 以下に Markdown がある可能性を返す（IMP-133）。
//
// 調べるのは dir の直下と、その 1 階層下まで。**全階層は走査しない**
// （FR-032）。起動時に階層全体を読むと、巨大なディレクトリで起動が遅くなる。
//
// 見つからなくても、さらに深い階層が残っている場合は true を返す。判定できない
// ディレクトリを隠すより、展開して空だと分かるほうがよい（IMP-133）。
func hasMarkdownWithin(dir string) bool {
	subdirs, found := scanForMarkdown(dir)
	if found {
		return true
	}

	for _, sub := range subdirs {
		deeper, found := scanForMarkdown(sub)
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

// scanForMarkdown は dir の直下を 1 度だけ読み、表示対象のサブディレクトリと、
// Markdown ファイルが直下にあったかを返す。
//
// 読めないディレクトリは「何もない」とみなす。権限のないディレクトリを
// ツリーに出しても、展開できずに終わるためである。
func scanForMarkdown(dir string) (subdirs []string, foundMarkdown bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, false
	}

	for _, e := range entries {
		name, isDir := e.Name(), e.IsDir()
		if !include(name, isDir) {
			continue
		}
		if !isDir {
			return nil, true
		}
		subdirs = append(subdirs, filepath.Join(dir, name))
	}

	return subdirs, false
}
