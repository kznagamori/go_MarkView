// domids の検査ロジックの単体テスト（UT-810）。
//
// 根拠: BR-042 / BR-043 / IMP-202 / IMP-230。
// 対象範囲の扱いは UT-002（`scripts/` の検証ロジックは単体テストの対象）。
//
// **実物のリポジトリを見ない。** 見てしまうと、index.html を直した瞬間に
// テストの意味が変わる。実物に対する検査はコマンド自身が行い、CI が実行する
// （BR-052）。ここで確かめるのは「拾い方が正しいか」だけである。
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree はテスト用の資産ディレクトリを組み立てる（UT-052）。
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	return dir
}

// TestVendorIDs は資産から決め打ちの id を拾うことを検証する
// （UT-810 のケース 1・3・4・6・7・8）。
func TestVendorIDs(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{
			name:  "シングルクォート",
			files: map[string]string{"a.js": `var el=document.getElementById('status');`},
			want:  []string{"status"},
		},
		{
			name:  "ダブルクォート",
			files: map[string]string{"a.js": `var el=document.getElementById("status");`},
			want:  []string{"status"},
		},
		{
			name:  "変数を渡す呼び出しは決め打ちではない",
			files: map[string]string{"a.js": `var el=document.getElementById(name);`},
			want:  nil,
		},
		{
			name:  "引数の前後に空白があっても拾う",
			files: map[string]string{"a.js": `document.getElementById( 'status' )`},
			want:  []string{"status"},
		},
		{
			name: "複数のファイルにまたがっても拾う",
			files: map[string]string{
				"a/one.js": `getElementById('status')`,
				"b/two.js": `getElementById('cy')`,
			},
			want: []string{"cy", "status"},
		},
		{
			name:  "同じ id が何度現れても 1 件",
			files: map[string]string{"a.js": `getElementById('status');getElementById('status')`},
			want:  []string{"status"},
		},
		{
			name: "js 以外は走査しない",
			files: map[string]string{
				"a.css":  `/* getElementById('status') */`,
				"b.json": `{"x":"getElementById('cy')"}`,
				"c.js":   `getElementById('app')`,
			},
			want: []string{"app"},
		},
		{
			name:  "空文字の id は無視する",
			files: map[string]string{"a.js": `getElementById('')`},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vendorIDs(writeTree(t, tt.files))
			if err != nil {
				t.Fatalf("vendorIDs() error = %v", err)
			}

			keys := sortedKeys(got)
			if len(keys) != len(tt.want) {
				t.Fatalf("vendorIDs() = %v, want %v", keys, tt.want)
			}
			for i, k := range keys {
				if k != tt.want[i] {
					t.Errorf("vendorIDs()[%d] = %q, want %q", i, k, tt.want[i])
				}
			}
		})
	}
}

// TestVendorIDs_RecordsFile は、どのファイルで見つかったかを持つことを検証する
// （UT-810 のケース 6）。**原因のファイルが分からないと差し替えの判断ができない。**
func TestVendorIDs_RecordsFile(t *testing.T) {
	dir := writeTree(t, map[string]string{"plantuml/plantuml.js": `getElementById('status')`})

	got, err := vendorIDs(dir)
	if err != nil {
		t.Fatalf("vendorIDs() error = %v", err)
	}

	files := got["status"]
	if len(files) != 1 {
		t.Fatalf("files = %v, want 1 件", files)
	}
	if !strings.HasSuffix(files[0], "plantuml/plantuml.js") {
		t.Errorf("files[0] = %q, want plantuml/plantuml.js で終わること", files[0])
	}
}

// TestVendorIDs_MissingDir は資産のディレクトリが無い場合を検証する（UT-013）。
func TestVendorIDs_MissingDir(t *testing.T) {
	if _, err := vendorIDs(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("vendorIDs() error = nil, want エラー")
	}
}

// TestHTMLIDs は HTML から id 属性を拾うことを検証する
// （UT-810 のケース 5・9）。
func TestHTMLIDs(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "ダブルクォート",
			html: `<footer id="statusbar" class="status"></footer>`,
			want: []string{"statusbar"},
		},
		{
			name: "シングルクォート",
			html: `<footer id='statusbar'></footer>`,
			want: []string{"statusbar"},
		},
		{
			name: "data-id は id 属性ではない",
			html: `<span data-id="status"></span>`,
			want: nil,
		},
		{
			name: "重複は 1 件にまとめる",
			html: `<a id="x"></a><b id="x"></b>`,
			want: []string{"x"},
		},
		{
			name: "id が 1 つも無くてもエラーにしない",
			html: `<div class="status"></div>`,
			want: nil,
		},
		{
			name: "複数の id を昇順で返す",
			html: `<div id="viewer"></div><div id="app"></div>`,
			want: []string{"app", "viewer"},
		},
		{
			name: "= の前後に空白があっても拾う",
			html: `<div id = "app"></div>`,
			want: []string{"app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "index.html")
			if err := os.WriteFile(path, []byte(tt.html), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			got, err := htmlIDs(path)
			if err != nil {
				t.Fatalf("htmlIDs() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("htmlIDs() = %v, want %v", got, tt.want)
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("htmlIDs()[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

// TestRun は検査の合否を検証する（UT-810 のケース 1・2）。
//
// **これが本体である。** 交差があれば必ずエラーになり、無ければ通る。
func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		vendor  string
		html    string
		wantErr bool
		wantID  string
	}{
		{
			name:    "衝突している",
			vendor:  `document.getElementById('status')`,
			html:    `<footer id="status"></footer>`,
			wantErr: true,
			wantID:  "status",
		},
		{
			name:    "資産が別の id を見ている",
			vendor:  `document.getElementById('cy')`,
			html:    `<footer id="status"></footer>`,
			wantErr: false,
		},
		{
			name:    "改名して衝突を避けた形",
			vendor:  `document.getElementById('status')`,
			html:    `<footer id="statusbar"></footer>`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := writeTree(t, map[string]string{"a.js": tt.vendor})
			path := filepath.Join(t.TempDir(), "index.html")
			if err := os.WriteFile(path, []byte(tt.html), 0o644); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}

			err := run(dir, path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("run() error = nil, want エラー")
				}
				if !strings.Contains(err.Error(), tt.wantID) {
					t.Errorf("run() error = %q, want %q を含む", err, tt.wantID)
				}
				return
			}
			if err != nil {
				t.Errorf("run() error = %v, want nil", err)
			}
		})
	}
}
