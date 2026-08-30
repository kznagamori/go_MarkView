package localurl

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestEncodeDecode_RoundTrip は組み立てと解読が対になっていることを検証する
// （根拠: AR-040 / IMP-118, IMP-161）。
//
// この対が崩れると、画像が一枚も表示されなくなる。
func TestEncodeDecode_RoundTrip(t *testing.T) {
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
// 検証する（IMP-118, IMP-161）。
//
// 二重スラッシュを含む URL は経路によって正規化されうるため、パス全体を
// 1 セグメントに収めている。
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

// TestDecode_Rejects は解読を拒む入力を検証する（IMP-161）。
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
