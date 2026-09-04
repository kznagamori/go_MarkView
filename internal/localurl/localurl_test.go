package localurl

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestEncodeDecode_RoundTrip は組み立てと解読が対になっていることを検証する
// （UT-808 ケース 2〜4。根拠: AR-040 / IMP-118, IMP-161）。
//
// この対が崩れると、画像が一枚も表示されなくなる。
//
// **組み立てる側（renderer の IMP-118）と解く側（assetsrv の IMP-161）は
// 互いに依存できない**（IMP-012）。それぞれの単体テストは片側の規則しか
// 見ないため、**逆変換の対をここで確かめることが食い違いを防ぐ唯一の手段**
// である。
func TestEncodeDecode_RoundTrip(t *testing.T) {
	// UT-808 ケース 2: 記号と非 ASCII を含むパス
	// UT-808 ケース 3: Windows 形式のパス
	// UT-808 ケース 4: 根
	paths := []string{
		"/docs/a.png",
		"/docs/sub/dir/image.jpeg",
		"/docs/a b.png",         // 空白
		"/docs/a#b.png",         // フラグメント記号
		"/docs/a%20b.png",       // すでに % を含む名前
		"/docs/日本語.png",         // 非 ASCII
		"/docs/a?b.png",         // クエリ記号
		"/docs/a&b=c.png",       // & と =
		"/docs/a+b.png",         // +
		`C:\Users\x\docs\a.png`, // Windows 形式
		"/",                     // 根
	}

	for _, want := range paths {
		t.Run(want, func(t *testing.T) {
			encoded := Encode(want)

			if !strings.HasPrefix(encoded, Prefix) {
				t.Fatalf("Encode(%q) = %q, 接頭辞 %q で始まらない", want, encoded, Prefix)
			}

			got, ok := Decode(encoded)
			if !ok {
				t.Fatalf("Decode(%q) が失敗した", encoded)
			}
			if got != filepath.FromSlash(want) {
				t.Errorf("往復で一致しない: %q -> %q -> %q", want, encoded, got)
			}
		})
	}
}

// TestEncode_EscapesSeparators は区切りとしての / までエスケープすることを
// 検証する（UT-808 ケース 1。根拠: AR-040 / IMP-118, IMP-161）。
//
// 二重スラッシュを含む URL は経路によって正規化されうるため、パス全体を
// 1 セグメントに収めている。**区切りの / を残すと `/__local//docs/...` が
// 生まれ、経路によって正規化されて 404 になる。**
func TestEncode_EscapesSeparators(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"POSIX の絶対パス", "/docs/a.png", "/__local/%2Fdocs%2Fa.png"},
		{"空白を含む", "/docs/a b.png", "/__local/%2Fdocs%2Fa%20b.png"},
		{"# を含む", "/docs/a#b.png", "/__local/%2Fdocs%2Fa%23b.png"},
		{"% を含む", "/docs/a%b.png", "/__local/%2Fdocs%2Fa%25b.png"},
		{"? を含む", "/docs/a?b.png", "/__local/%2Fdocs%2Fa%3Fb.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Encode(tt.in); got != tt.want {
				t.Errorf("Encode(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestDecode_Rejects は解読を拒む入力を検証する
// （UT-808 ケース 5〜8。根拠: AR-040 / IMP-161）。
//
// 呼び出し側はここで false を受けた時点で 404 を返してよい（IMP-161）。
func TestDecode_Rejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"空文字", ""},
		{"接頭辞がない", "/docs/a.png"},
		{"接頭辞に似た別のパス", "/__locale/%2Fa.png"},
		{"接頭辞のみ", Prefix},
		{"壊れたエスケープ", Prefix + "%zz"},
		{"途中で切れたエスケープ", Prefix + "%2"},
		{"アプリ資産のパス", "/index.html"},
		{"アイコンのパス", "/appicon.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, ok := Decode(tt.in); ok {
				t.Errorf("Decode(%q) = %q, true。拒否されるべき", tt.in, got)
			}
		})
	}
}
