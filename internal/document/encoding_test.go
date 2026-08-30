package document

import (
	"bytes"
	"testing"
)

// withBOM は UTF-8 BOM を先頭に付けたバイト列を作る。
//
// 毎回新しいスライスを作るのは、共有した配列に append して他のケースの
// 入力を壊すことがないようにするためである（UT-017）。
func withBOM(s string) []byte {
	return append([]byte{0xEF, 0xBB, 0xBF}, s...)
}

// TestNormalize_BOMAndNewlines は BOM の除去と改行コードの統一を検証する
// （UT-101。根拠: FR-021 / IMP-103）。
//
// この表では不正なバイト列を扱わないため、replaced は常に false を期待する。
func TestNormalize_BOMAndNewlines(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		// UT-101 ケース 9・7: 空入力と BOM のみ（境界値を先に置く。UT-013）
		{"空のバイト列", []byte{}, ""},
		{"nil スライス", nil, ""},
		{"BOM のみ", withBOM(""), ""},

		// UT-101 ケース 1・2: BOM の除去
		{"BOM 付きは除去する", withBOM("# A"), "# A"},
		{"BOM なしはそのまま", []byte("# A"), "# A"},

		// UT-101 ケース 8: 先頭以外の BOM は本文として残す
		{"中間の EF BB BF は除去しない", []byte("a\uFEFFb"), "a\uFEFFb"},
		{"BOM が 2 つ続く場合は 1 つだけ除去する", withBOM("\uFEFFa"), "\uFEFFa"},

		// UT-101 ケース 3〜6: 改行コード
		{"CRLF は LF になる", []byte("a\r\nb"), "a\nb"},
		{"単独の CR は LF になる", []byte("a\rb"), "a\nb"},
		{"LF は変化しない", []byte("a\nb"), "a\nb"},
		{"連続する CRLF は空行になる", []byte("a\r\n\r\nb"), "a\n\nb"},

		// UT-090 に従って追加した境界値。
		// CRLF を素朴に 2 段階で置換したときに壊れやすい並びを重点的に置く。
		{"末尾の CR", []byte("a\r"), "a\n"},
		{"CRLF のみ", []byte("\r\n"), "\n"},
		{"連続する単独 CR は 2 行分になる", []byte("\r\r"), "\n\n"},
		{"CR の直後に CRLF が続く", []byte("a\r\r\nb"), "a\n\nb"},
		{"LF の直後の CR は別の改行", []byte("a\n\rb"), "a\n\nb"},
		{"3 種類の改行が混在する", []byte("a\r\nb\rc\nd"), "a\nb\nc\nd"},
		{"BOM と CRLF の組み合わせ", withBOM("a\r\nb"), "a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, replaced := Normalize(tt.in)
			if string(got) != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if replaced {
				t.Errorf("Normalize(%q) replaced = true, want false", tt.in)
			}
		})
	}
}

// TestNormalize_InvalidUTF8 は不正なバイト列の置換を検証する
// （UT-102。根拠: FR-021 / IMP-103）。
func TestNormalize_InvalidUTF8(t *testing.T) {
	tests := []struct {
		name         string
		in           []byte
		want         string
		wantReplaced bool
	}{
		// UT-102 ケース 2: 正常な UTF-8 は触らない
		{"日本語はそのまま", []byte("こんにちは"), "こんにちは", false},
		{"絵文字はそのまま", []byte("a🎉b"), "a🎉b", false},

		// UT-102 ケース 1: 単独の不正バイト
		{"不正な 1 バイト", []byte{'a', 0xFF, 'b'}, "a\uFFFDb", true},

		// UT-102 ケース 4: 途中で切れたマルチバイト列
		{"3 バイト文字が 2 バイトで切れている", []byte{0xE3, 0x81, 'a'}, "\uFFFDa", true},
		{"末尾でマルチバイト列が切れている", []byte{'a', 0xE3, 0x81}, "a\uFFFD", true},

		// UT-102 ケース 3: UTF-16 の BOM。変換せず、エラーにもしない
		{"UTF-16LE の BOM で始まる", []byte{0xFF, 0xFE, 'a', 0x00}, "\uFFFDa\x00", true},
		{"UTF-16BE の BOM で始まる", []byte{0xFE, 0xFF, 0x00, 'a'}, "\uFFFD\x00a", true},

		// UT-090 に従って追加した境界値。
		// bytes.ToValidUTF8 は不正バイトの「連続」を 1 つの U+FFFD にまとめる。
		// 数がずれると利用者に見える文字数が変わるため、明示的に固定する。
		{"連続する不正バイトはまとめて 1 つ", []byte{'a', 0xFF, 0xFE, 'b'}, "a\uFFFDb", true},
		{"離れた不正バイトはそれぞれ置換する", []byte{'a', 0xFF, 'b', 0xFF, 'c'}, "a\uFFFDb\uFFFDc", true},
		{"不正バイトのみ", []byte{0xFF}, "\uFFFD", true},

		// 正規化の順序を固定する。改行を揃えてから置換するため、
		// CRLF は不正バイトの有無にかかわらず LF になる。
		{"CRLF と不正バイトが混在する", []byte{'a', '\r', '\n', 0xFF}, "a\n\uFFFD", true},
		{"BOM の後ろに不正バイト", withBOM("a\xffb"), "a\uFFFDb", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, replaced := Normalize(tt.in)
			if string(got) != tt.want {
				t.Errorf("Normalize(% x) = %q, want %q", tt.in, got, tt.want)
			}
			if replaced != tt.wantReplaced {
				t.Errorf("Normalize(% x) replaced = %v, want %v", tt.in, replaced, tt.wantReplaced)
			}
		})
	}
}

// TestNormalize_DoesNotModifyInput は、入力のバイト列を書き換えないことを
// 検証する（根拠: IMP-103）。
//
// 戻り値が入力と領域を共有しうる実装のため、正規化そのものが入力を壊して
// いないことを確かめておく。壊すと、再読み込みや警告表示で同じバイト列を
// 使い回したときに結果が変わる。
func TestNormalize_DoesNotModifyInput(t *testing.T) {
	raw := withBOM("a\r\nb\rc")
	before := bytes.Clone(raw)

	if _, _ = Normalize(raw); !bytes.Equal(raw, before) {
		t.Errorf("Normalize が入力を書き換えた: % x, want % x", raw, before)
	}
}

// TestCountLines は行数の算出を検証する（UT-103。根拠: FR-021 / IMP-104）。
//
// 入力は正規化済みのテキストとする。CRLF を含む場合は
// TestCountLines_AfterNormalize が担当する。
func TestCountLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		// UT-103 ケース 4・5・3: 境界値を先に置く（UT-013）
		{"空のテキストは 1 行", "", 1},
		{"改行 1 つだけは 1 行", "\n", 1},
		{"改行を含まない 1 行", "a", 1},

		// UT-103 ケース 1・2: 末尾の改行で加算しない
		{"改行で区切られた 3 行", "a\nb\nc", 3},
		{"末尾が改行の 3 行", "a\nb\nc\n", 3},

		// UT-090 に従って追加した境界値
		{"1 行と末尾の改行", "a\n", 1},
		{"連続する改行は空行を数える", "\n\n", 2},
		{"末尾の空行は 1 つだけ数える", "a\n\n", 2},
		{"先頭が空行", "\na", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountLines([]byte(tt.in)); got != tt.want {
				t.Errorf("CountLines(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestCountLines_AfterNormalize は UT-103 ケース 6 を検証する。
//
// IMP-104 は行数を「正規化後のテキスト」に対して数えると定める。CRLF を
// 正規化前に数えると LF が 1 つしかないため、区別できるケースになっている。
func TestCountLines_AfterNormalize(t *testing.T) {
	text, _ := Normalize([]byte("a\r\nb"))

	if got := CountLines(text); got != 2 {
		t.Errorf(`CountLines(Normalize("a\r\nb")) = %d, want 2`, got)
	}
}
