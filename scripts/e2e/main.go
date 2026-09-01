// e2e は配布物に対する自動 E2E テストを実行する（E2E-100〜108）。
//
//	go run ./scripts/e2e archives -dir dist -version v1.0.0-rc.1
//	go run ./scripts/e2e binary   -archive dist/MarkView-v1.0.0-rc.1-linux-amd64.tar.gz -version v1.0.0-rc.1
//
// **見るのは「組み上がったものが動くか」だけである**（E2E-001）。内部の
// 境界値は単体テストが受け持つ。ここで確かめられるのは、単体テストでは
// 決して現れないもの——ldflags の埋め込み漏れ、ビルドタグの指定漏れ、
// アーカイブへの入れ忘れ、実行ファイルがそもそも起動しないこと——である。
//
// 2 つに分けているのは、要求する環境が違うためである。
//
//	archives  アーカイブの中身・サイズ・checksum。どの OS でも実行できる
//	binary    実行ファイルの起動と CLI 出力。**対象 OS の上でしか実行できない**
//
// **ウィンドウ内の操作は行わない**（E2E-020）。起動して生存を見るところまでで、
// 要素をクリックしたり描画完了を待ったりはしない。判断の伴う確認は 41 章の
// 手動テストに委ねる。
package main

import (
	"fmt"
	"os"
	"strings"
)

// outcome は 1 件の確認の結果。
type outcome int

const (
	passed  outcome = iota // 期待どおり
	failed                 // 期待と違う。**リリースを中止する**（E2E-021）
	warned                 // 閾値を超えたが失敗にしない（BR-060, E2E-107）
	skipped                // その環境では実施できない
	noted                  // 記録だけ
)

func (o outcome) String() string {
	switch o {
	case passed:
		return "OK  "
	case failed:
		return "NG  "
	case warned:
		return "WARN"
	case skipped:
		return "SKIP"
	}

	return "--  "
}

// row は結果表の 1 行。id と number は 40 章の表の「#」に対応する。
type row struct {
	id     string
	number int
	name   string
	result outcome
	detail string
}

type report struct {
	rows []row
}

func (r *report) add(id string, number int, name string, result outcome, detail string) {
	r.rows = append(r.rows, row{id: id, number: number, name: name, result: result, detail: detail})
}

// verify は真偽で合否が決まる確認を記録する。
func (r *report) verify(id string, number int, name string, ok bool, detail string) {
	result := failed
	if ok {
		result = passed
	}

	r.add(id, number, name, result, detail)
}

func (r *report) fail(id string, number int, name string, format string, args ...any) {
	r.add(id, number, name, failed, fmt.Sprintf(format, args...))
}

func (r *report) skip(id string, number int, name, why string) {
	r.add(id, number, name, skipped, why)
}

func (r *report) note(id string, number int, name, detail string) {
	r.add(id, number, name, noted, detail)
}

func (r *report) warn(id string, number int, name, detail string) {
	r.add(id, number, name, warned, detail)
}

// print は結果表を出し、失敗の件数を返す。
func (r *report) print() int {
	width := 0
	for _, row := range r.rows {
		if n := displayWidth(row.name); n > width {
			width = n
		}
	}

	fmt.Println()

	failures := 0
	previous := ""

	for _, row := range r.rows {
		if row.id != previous {
			fmt.Println()
			previous = row.id
		}

		if row.result == failed {
			failures++
		}

		fmt.Printf("%-8s %d  %s  %s  %s\n",
			row.id, row.number, row.result, pad(row.name, width), row.detail)
	}

	fmt.Println()

	counts := map[outcome]int{}
	for _, row := range r.rows {
		counts[row.result]++
	}

	fmt.Printf("合計 %d 件: OK %d / NG %d / WARN %d / SKIP %d\n",
		len(r.rows), counts[passed], failures, counts[warned], counts[skipped])

	return failures
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var (
		result *report
		err    error
	)

	switch os.Args[1] {
	case "archives":
		result, err = runArchives(os.Args[2:])
	case "binary":
		result, err = runBinary(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		os.Exit(1)
	}

	if result.print() > 0 {
		// **失敗したらリリースを中止する**（E2E-021）。
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `e2e - 配布物に対する自動 E2E テスト（40 章）

Usage:
  e2e archives -dir <配布物のディレクトリ> -version <タグ名>
        E2E-101 アーカイブの構成 / E2E-107 サイズ / E2E-108 チェックサム
        どの OS でも実行できる

  e2e binary -archive <アーカイブ> -version <タグ名> [-data <検証用データ>]
  e2e binary -exe <実行ファイル>   -version <タグ名> [-data <検証用データ>]
        E2E-102 --version / E2E-103 --help / E2E-104 起動と終了 /
        E2E-105 不正な引数 / E2E-106 Linux の依存ライブラリ
        **対象 OS の上でしか実行できない**
`)
}

// pad は表示幅を揃える。
//
// **`%-*s` は使えない。** Go の幅指定はバイト数で数えるため、日本語を含む
// 名前では桁が合わない。
func pad(s string, width int) string {
	if n := displayWidth(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}

	return s
}

// displayWidth は端末上の桁数を数える。全角は 2 桁として扱う。
func displayWidth(s string) int {
	width := 0

	for _, r := range s {
		if isWide(r) {
			width += 2

			continue
		}

		width++
	}

	return width
}

func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // ハングル字母
		r >= 0x2E80 && r <= 0xA4CF, // CJK 部首・かな・漢字
		r >= 0xAC00 && r <= 0xD7A3, // ハングル音節
		r >= 0xF900 && r <= 0xFAFF, // CJK 互換漢字
		r >= 0xFE30 && r <= 0xFE6F, // CJK 互換形
		r >= 0xFF00 && r <= 0xFF60, // 全角英数
		r >= 0xFFE0 && r <= 0xFFE6: // 全角記号
		return true
	}

	return false
}

// isASCII は文字列が ASCII だけでできているかを返す（UI-024, E2E-103）。
//
// 利用者に見えるものはすべて英語という規約の、機械で確かめられる部分。
// 日本語が紛れ込めばここで落ちる。
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}

	return true
}

// firstLine は表示用に 1 行目だけを取り出す。
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i] + " ..."
	}

	return s
}
