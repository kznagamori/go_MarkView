// Package document は Markdown ファイルの読み込みと、表示に渡す 1 文書の
// 組み立てを担う（IMP-100 系）。
//
// internal のうち本パッケージが依存してよいのは renderer と mdfile だけである
// （IMP-012）。Wails の API は呼ばない。
package document

import (
	"bytes"
	"unicode/utf8"
)

// utf8BOM は UTF-8 のバイト順マーク（EF BB BF）。
//
// 先頭に現れたときだけ取り除く。本文の途中に現れた同じ 3 バイトは
// ゼロ幅ノーブレークスペース（U+FEFF）であり、本文の一部として残す（IMP-103）。
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// replacementChar は不正なバイト列の置き換えに使う U+FFFD。
var replacementChar = []byte("\uFFFD")

// Normalize は生バイト列を UTF-8 テキストへ正規化する（IMP-103, FR-021）。
//
// 戻り値の replaced は、UTF-8 として不正なバイト列を U+FFFD へ置き換えたか
// どうかを示す。呼び出し側はこれを Warning（WarnInvalidEncoding）に変換して
// ステータス領域へ伝える。置き換えが起きても読み込みは失敗させない。
//
// UTF-16 の BOM（FF FE / FE FF）で始まる場合も変換は試みない。レガシー
// エンコーディングの自動判別を行わないためである（FR-021 の注記）。結果は
// 文字化けするが、読み込み自体は成功させる。
//
// 返すスライスは raw と領域を共有することがある。呼び出し側は書き換えない。
func Normalize(raw []byte) (text []byte, replaced bool) {
	text = normalizeNewlines(bytes.TrimPrefix(raw, utf8BOM))
	if utf8.Valid(text) {
		return text, false
	}
	return bytes.ToValidUTF8(text, replacementChar), true
}

// normalizeNewlines は CRLF と単独の CR を LF に揃える（IMP-103 の 2）。
//
// IMP-103 は「CRLF → LF、単独の CR → LF の順で置換する」と定めるが、ここでは
// 1 回の走査で同じ結果を得ている。CR を見つけたら LF を書き、直後が LF なら
// まとめて 1 つの改行として扱う。2 段階の置換は順序を誤ると CRLF が LF 2 つに
// なるため、順序への依存そのものをなくした。
func normalizeNewlines(b []byte) []byte {
	if bytes.IndexByte(b, '\r') < 0 {
		// CR を含まない。大半の文書はこの経路を通り、複製も走査も起きない。
		return b
	}

	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] != '\r' {
			out = append(out, b[i])
			continue
		}
		out = append(out, '\n')
		if i+1 < len(b) && b[i+1] == '\n' {
			i++ // CRLF。続く LF は書かない
		}
	}
	return out
}

// CountLines は正規化後のテキストの行数を返す（IMP-104）。
//
// LF の個数に 1 を加える。ただし末尾が LF で終わる場合は加算しない。
// 空のテキストは 1 行として数える。UI-060 のステータス表示に使う。
func CountLines(text []byte) int {
	n := bytes.Count(text, []byte{'\n'})
	if len(text) > 0 && text[len(text)-1] == '\n' {
		return n
	}
	return n + 1
}
