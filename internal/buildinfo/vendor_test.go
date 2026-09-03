package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

// setVendorJSON は vendor.json を差し替え、テスト終了時に戻す（UT-017）。
func setVendorJSON(t *testing.T, data string) {
	t.Helper()

	original := vendorJSON
	SetVendorJSON([]byte(data))
	t.Cleanup(func() { vendorJSON = original })
}

// TestVendors は vendor.json の解析を検証する
// （UT-801。根拠: BR-042 / IMP-181）。
func TestVendors(t *testing.T) {
	// UT-801 ケース 1: 名称・バージョン・spdx・license が読める（BR-042）
	t.Run("正常な vendor.json", func(t *testing.T) {
		setVendorJSON(t, `[
			{"name":"mermaid","version":"11.4.1","spdx":"MIT","license":"mermaid/LICENSE",
			 "source":"https://example.com/mermaid","fetched":"2026-08-30"},
			{"name":"katex","version":"0.16.11","spdx":"MIT","license":"katex/LICENSE",
			 "source":"https://example.com/katex","fetched":"2026-08-30"}
		]`)

		got := Vendors()
		if len(got) != 2 {
			t.Fatalf("件数 = %d, want 2", len(got))
		}

		if got[0].Name != "mermaid" || got[0].Version != "11.4.1" {
			t.Errorf("1 件目 = %+v, want mermaid 11.4.1", got[0])
		}
		if got[0].SPDX != "MIT" || got[0].License != "mermaid/LICENSE" {
			t.Errorf("1 件目の spdx / license = %+v", got[0])
		}
		if got[0].Source != "https://example.com/mermaid" || got[0].Fetched != "2026-08-30" {
			t.Errorf("1 件目の取得元・取得日 = %+v", got[0])
		}
		if got[1].Name != "katex" || got[1].Version != "0.16.11" {
			t.Errorf("2 件目 = %+v, want katex 0.16.11", got[1])
		}
	})

	// UT-801 ケース 4: 版が空でも落とさない（BR-042）
	//
	// **ここが要である。** 「版が無いものは不完全だから捨てる」実装は Bundled 行
	// では正しく見えるが、ライセンス一覧から Graphviz が消える（BR-040, NFR-051）。
	// 版は再頒布の条件ではない。条件は名称・ライセンス種別・全文の 3 つである。
	t.Run("版が空のエントリを落とさない", func(t *testing.T) {
		setVendorJSON(t, `[
			{"name":"Graphviz","version":"","spdx":"EPL-2.0",
			 "license":"plantuml/licenses/graphviz-EPL-2.0.txt","bundledIn":"PlantUML"}
		]`)

		got := Vendors()
		if len(got) != 1 {
			t.Fatalf("件数 = %d, want 1（版が空でも落とさない）", len(got))
		}
		if got[0].Name != "Graphviz" || got[0].SPDX != "EPL-2.0" {
			t.Errorf("エントリ = %+v", got[0])
		}
		if got[0].License == "" {
			t.Error("license が空。全文の位置を失うとライセンス一覧を作れない")
		}
	})

	// UT-801 ケース 2・3: 壊れていても落とさない
	t.Run("壊れた入力", func(t *testing.T) {
		tests := []struct {
			name string
			data string
		}{
			{"閉じていない配列", `[{"name":"a"`},
			{"JSON ではない", "not json"},
			{"空文字", ""},
			{"オブジェクト（配列ではない）", `{"name":"a"}`},
			{"数値", "42"},
			{"null", "null"},
			{"要素の型が違う", `["a","b"]`},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				setVendorJSON(t, tt.data)

				// panic しないことが要求そのものである（FR-111）。
				got := Vendors()

				if got == nil {
					t.Error("nil が返った。常に非 nil の空スライスを返すこと")
				}
				if len(got) != 0 {
					t.Errorf("件数 = %d, want 0（%+v）", len(got), got)
				}
			})
		}
	})

	t.Run("0 件の配列", func(t *testing.T) {
		setVendorJSON(t, "[]")

		got := Vendors()
		if got == nil {
			t.Error("nil が返った。常に非 nil の空スライスを返すこと")
		}
		if len(got) != 0 {
			t.Errorf("件数 = %d, want 0", len(got))
		}
	})

	t.Run("未設定でも落ちない", func(t *testing.T) {
		original := vendorJSON
		vendorJSON = nil
		t.Cleanup(func() { vendorJSON = original })

		if got := Vendors(); len(got) != 0 || got == nil {
			t.Errorf("Vendors() = %+v, want 非 nil の空スライス", got)
		}
	})

	t.Run("未知のキーは無視する", func(t *testing.T) {
		setVendorJSON(t, `[{"name":"a","version":"1","unknown":"x"}]`)

		got := Vendors()
		if len(got) != 1 || got[0].Name != "a" {
			t.Errorf("Vendors() = %+v, want 1 件", got)
		}
	})
}

// TestBundled は Bundled 行に出す資産の絞り込みを検証する
// （UT-801 ケース 5〜7。根拠: BR-042, UI-100 / IMP-181）。
//
// 同梱物の中に含まれるもの（Viz.js / Graphviz / Expat）は bundledIn を持ち、
// Bundled 行には出さない。**記録からは外さない**——ライセンス表示（FR-101）に
// 全文とともに現れる必要がある。絞り込みをここ 1 か所に置くのは、
// フロントエンドで絞ると絞り方が 2 つに分かれるためである（IMP-306）。
func TestBundled(t *testing.T) {
	const mixed = `[
		{"name":"Mermaid","version":"11.4.1","spdx":"MIT","license":"mermaid/LICENSE"},
		{"name":"KaTeX","version":"0.16.11","spdx":"MIT","license":"katex/LICENSE"},
		{"name":"PlantUML","version":"1.2026.7","spdx":"MIT","license":"plantuml/LICENSE"},
		{"name":"Viz.js","version":"3.24.0","spdx":"MIT",
		 "license":"plantuml/licenses/viz-js-MIT.txt","bundledIn":"PlantUML"},
		{"name":"Graphviz","version":"","spdx":"EPL-2.0",
		 "license":"plantuml/licenses/graphviz-EPL-2.0.txt","bundledIn":"PlantUML"},
		{"name":"Expat","version":"","spdx":"MIT",
		 "license":"plantuml/licenses/expat-MIT.txt","bundledIn":"PlantUML"}
	]`

	// UT-801 ケース 5: Vendors は絞り込まない
	t.Run("Vendors はすべて返す", func(t *testing.T) {
		setVendorJSON(t, mixed)

		if got := Vendors(); len(got) != 6 {
			t.Errorf("Vendors の件数 = %d, want 6（記録は絞り込まない）", len(got))
		}
	})

	// UT-801 ケース 6: Bundled は bundledIn が空のものだけ
	t.Run("Bundled は最上位の資産だけを返す", func(t *testing.T) {
		setVendorJSON(t, mixed)

		got := Bundled()
		want := []string{"Mermaid", "KaTeX", "PlantUML"}

		if len(got) != len(want) {
			t.Fatalf("件数 = %d, want %d（%+v）", len(got), len(want), got)
		}
		for i, name := range want {
			if got[i].Name != name {
				t.Errorf("%d 件目 = %q, want %q（順序も保つ）", i, got[i].Name, name)
			}
			if got[i].BundledIn != "" {
				t.Errorf("%d 件目に bundledIn が入っている: %+v", i, got[i])
			}
		}
	})

	// UT-801 ケース 7: 空でも nil を返さない
	t.Run("全件が bundledIn を持つ", func(t *testing.T) {
		setVendorJSON(t, `[{"name":"Graphviz","bundledIn":"PlantUML"}]`)

		got := Bundled()
		if got == nil {
			t.Error("nil が返った。常に非 nil の空スライスを返すこと（FR-111）")
		}
		if len(got) != 0 {
			t.Errorf("件数 = %d, want 0", len(got))
		}
	})

	t.Run("記録が空でも落ちない", func(t *testing.T) {
		setVendorJSON(t, "[]")

		if got := Bundled(); got == nil || len(got) != 0 {
			t.Errorf("Bundled() = %+v, want 非 nil の空スライス", got)
		}
	})
}

// TestEnvironment は環境情報の組み立てを検証する（IMP-181, UI-100）。
//
// OS とアーキテクチャは実行環境そのものであり、期待値をリテラルで書けない。
// 形（区切りと構成）が保たれているかを見る。
func TestEnvironment(t *testing.T) {
	t.Run("WebView のバージョンがある", func(t *testing.T) {
		got := Environment("120.0.1")

		if !strings.HasPrefix(got, runtime.GOOS+"/") {
			t.Errorf("Environment = %q, want OS/アーキテクチャで始まる", got)
		}
		if !strings.Contains(got, "Go ") {
			t.Errorf("Environment = %q, want Go のバージョンを含む", got)
		}
		if !strings.Contains(got, "120.0.1") {
			t.Errorf("Environment = %q, want WebView のバージョンを含む", got)
		}
		if strings.Contains(got, "go1.") {
			t.Errorf("Environment = %q, Go のバージョンから go の接頭辞を外すこと", got)
		}
	})

	// バージョンが取れない場合、区画ごと省く（IMP-181）。
	t.Run("WebView のバージョンがない", func(t *testing.T) {
		got := Environment("")

		if strings.Contains(got, "WebView") || strings.Contains(got, "WebKitGTK") {
			t.Errorf("Environment = %q, want WebView の区画を含まない", got)
		}
		if strings.HasSuffix(got, " ") {
			t.Errorf("Environment = %q, 末尾に空白が残っている", got)
		}
		if !strings.Contains(got, "Go ") {
			t.Errorf("Environment = %q, want Go のバージョンを含む", got)
		}
	})

	t.Run("WebView の名称は OS に対応する", func(t *testing.T) {
		got := Environment("1.2.3")

		want := "WebKitGTK"
		if runtime.GOOS == "windows" {
			want = "WebView2"
		}
		if !strings.Contains(got, want) {
			t.Errorf("Environment = %q, want %q を含む", got, want)
		}
	})
}
