// 同梱資産の更新でリリースを止める判定に対するテスト（UT-818）。
//
// **リリースを止める判定**である（license-rejected / notice-mismatch。BR-043）。判定が甘くなっても
// 出力は「更新した」のままで、何も起きない（30 章 UT-002 の NOTE）。取得・展開・書き込みは対象外とし、
// 判定だけを呼ぶ（4.78.0。利用者の判断）。告知の照合は、小さな告知・vendor.json・全文を t.TempDir() に
// 置いて行う（通信しない。IMP-042）。**止めるケースと止めないケースの両方を置く**（UT-033）。
package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestRejected はライセンスの判定を検証する（UT-818 のケース 1〜4。根拠: BR-043, NFR-051）。
//
// **一覧を広げるのは、NFR-051 を改めてからにする。** ケース 3 は一覧そのものを固定する（表の健全性）。
func TestRejected(t *testing.T) {
	// ケース 1: GPL-3.0-or-later（@plantuml/core の 1.2026.5 以前）は拒む。
	t.Run("1 GPL", func(t *testing.T) {
		got := rejected([]result{{name: "@plantuml/core", after: "1.2026.5", spdx: "GPL-3.0-or-later"}})
		if len(got) != 1 || !strings.Contains(got[0], "@plantuml/core") {
			t.Errorf("rejected = %q, want @plantuml/core の 1 件", got)
		}
	})

	// ケース 2: 申告が空 / 空白だけは拒む（分からないものを通さない）。
	for _, spdx := range []string{"", "   "} {
		t.Run("2 申告が空 "+strings.ReplaceAll(spdx, " ", "_"), func(t *testing.T) {
			if got := rejected([]result{{name: "x", after: "1.0.0", spdx: spdx}}); len(got) != 1 {
				t.Errorf("rejected = %q, want 1 件", got)
			}
		})
	}

	// ケース 3: NFR-051 の一覧はどれも拒まない（SPDX の識別子は大文字小文字を区別しない）。
	t.Run("3 許容する一覧", func(t *testing.T) {
		var results []result
		for _, spdx := range []string{"MIT", "BSD-2-Clause", "BSD-3-Clause", "Apache-2.0", "ISC", "OFL-1.1", "EPL-2.0", "mit"} {
			results = append(results, result{name: spdx, after: "1.0.0", spdx: spdx})
		}
		if got := rejected(results); len(got) != 0 {
			t.Errorf("rejected = %q, want 空", got)
		}
	})

	// ケース 4: 3 つの資産のうち 1 つだけ GPL なら、その 1 つの名前だけを返す。
	t.Run("4 1つだけGPL", func(t *testing.T) {
		got := rejected([]result{
			{name: "mermaid", after: "12.1.0", spdx: "MIT"},
			{name: "@plantuml/core", after: "1.2026.5", spdx: "GPL-3.0-or-later"},
			{name: "katex", after: "0.19.0", spdx: "MIT"},
		})
		if len(got) != 1 || !strings.Contains(got[0], "@plantuml/core") || strings.Contains(got[0], "mermaid") {
			t.Errorf("rejected = %q, want @plantuml/core の 1 件だけ", got)
		}
	})
}

// noticeFixture は告知の照合の入力（viz-global.js の先頭・vendor.json の記録・全文）を作る。
type noticeFixture struct {
	head     string  // viz-global.js の先頭（告知）
	entries  []entry // vendor.json の記録
	fullText string  // Viz.js のライセンス全文
}

// okNotice は正しい組み合わせ（UT-818 のケース 5・7）を返す。
//
// 告知の 1 行目が配布物そのもの（Viz.js）で、contains の下には Graphviz と Expat の 2 件だけを並べる。
// 全文の著作権表示は年を持ち、告知は年を書かない（BR-043 の IMPORTANT の 2 つの落とし穴）。
func okNotice() noticeFixture {
	return noticeFixture{
		head: "/*!\nViz.js 3.24.0\nCopyright (c) Michael Daines\n\n" +
			"This distribution contains other software in object code form:\n" +
			"Graphviz https://www.graphviz.org\nExpat https://libexpat.github.io\n*/\n(function(){})();\n",
		entries: []entry{
			{Name: plantUMLName, Version: "1.2026.8", SPDX: "MIT", License: "plantuml/LICENSE"},
			{Name: "Viz.js", Version: "3.24.0", SPDX: "MIT", License: "plantuml/LICENSE-vizjs", BundledIn: plantUMLName},
			{Name: "Graphviz", Version: "-", SPDX: "EPL-2.0", License: "plantuml/LICENSE-graphviz", BundledIn: plantUMLName},
			{Name: "Expat", Version: "-", SPDX: "MIT", License: "plantuml/LICENSE-expat", BundledIn: plantUMLName},
		},
		fullText: "MIT License\n\nCopyright (c) 2014-2018 Michael Daines\n\nPermission is hereby granted...\n",
	}
}

// runCollate は fixture を t.TempDir() に置いて collate を呼ぶ。
func runCollate(t *testing.T, f noticeFixture) ([]entry, error) {
	t.Helper()

	dir := t.TempDir()
	viz := filepath.Join(dir, "plantuml", "viz-global.js")
	if err := os.MkdirAll(filepath.Dir(viz), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(viz, []byte(f.head), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plantuml", "LICENSE-vizjs"), []byte(f.fullText), 0o644); err != nil {
		t.Fatal(err)
	}
	return collate(dir, viz, f.entries)
}

// TestCollate は viz-global.js の告知の照合を検証する（UT-818 のケース 5〜9。根拠: BR-042, BR-043, NFR-051）。
//
// **ケース 5 と 7 は、正しい組み合わせでリリースを止めないことを見る**（BR-043 の IMPORTANT の 2 つの
// 落とし穴）。1・6・8 だけなら「常に止める」実装が通り、5・7 だけなら「常に通す」実装が通る。
func TestCollate(t *testing.T) {
	// ケース 5・7: 1 行目の配布物も数え、年を除いた権利者の名前で探す。
	t.Run("5・7 正しい組み合わせは通る", func(t *testing.T) {
		got, err := runCollate(t, okNotice())
		if err != nil {
			t.Fatalf("collate がエラーを返した（正しい組み合わせでリリースを止めた）: %v", err)
		}
		if !reflect.DeepEqual(got, okNotice().entries) {
			t.Errorf("collate の記録 = %+v, want 変わらない", got)
		}
	})

	// ケース 6: 告知に 4 つ目のプロジェクトがある / vendor.json に告知に無いものがある（照合 1）。
	t.Run("6 告知に4つ目のプロジェクト", func(t *testing.T) {
		f := okNotice()
		f.head = strings.Replace(f.head, "Expat https://libexpat.github.io\n", "Expat https://libexpat.github.io\nZlib https://zlib.net\n", 1)
		if _, err := runCollate(t, f); err == nil {
			t.Error("collate がエラーを返さなかった（notice-mismatch を期待）")
		}
	})
	t.Run("6 vendor.jsonに告知に無いもの", func(t *testing.T) {
		f := okNotice()
		f.entries = append(f.entries, entry{Name: "Zlib", Version: "-", SPDX: "Zlib", BundledIn: plantUMLName})
		if _, err := runCollate(t, f); err == nil {
			t.Error("collate がエラーを返さなかった（notice-mismatch を期待）")
		}
	})

	// ケース 8: 告知の権利者の名前が全文に無い（照合 2）。
	t.Run("8 権利者が全文に無い", func(t *testing.T) {
		f := okNotice()
		f.fullText = "MIT License\n\nCopyright (c) 2014-2018 Someone Else\n"
		if _, err := runCollate(t, f); err == nil {
			t.Error("collate がエラーを返さなかった（notice-mismatch を期待）")
		}
	})

	// ケース 9: Viz.js の版が告知と vendor.json で違えば、失敗にせず告知の版を書き戻す（照合 3）。
	t.Run("9 版の違いは書き戻す", func(t *testing.T) {
		f := okNotice()
		f.entries[1].Version = "3.23.0"
		got, err := runCollate(t, f)
		if err != nil {
			t.Fatalf("collate がエラーを返した（版の違いは失敗にしない）: %v", err)
		}
		if e := findEntry(got, "Viz.js"); e == nil || e.Version != "3.24.0" {
			t.Errorf("Viz.js の記録 = %+v, want 版 3.24.0（告知の版）", e)
		}
	})
}
