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
	t.Run("正常な vendor.json", func(t *testing.T) {
		setVendorJSON(t, `[
			{"name":"mermaid","version":"11.4.1","source":"https://example.com/mermaid","fetched":"2026-08-30"},
			{"name":"katex","version":"0.16.11","source":"https://example.com/katex","fetched":"2026-08-30"}
		]`)

		got := Vendors()
		if len(got) != 2 {
			t.Fatalf("件数 = %d, want 2", len(got))
		}

		if got[0].Name != "mermaid" || got[0].Version != "11.4.1" {
			t.Errorf("1 件目 = %+v, want mermaid 11.4.1", got[0])
		}
		if got[0].Source != "https://example.com/mermaid" || got[0].Fetched != "2026-08-30" {
			t.Errorf("1 件目の取得元・取得日 = %+v", got[0])
		}
		if got[1].Name != "katex" || got[1].Version != "0.16.11" {
			t.Errorf("2 件目 = %+v, want katex 0.16.11", got[1])
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
