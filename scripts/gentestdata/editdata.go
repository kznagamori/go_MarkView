package main

// editdata.go — generated/edit/ に置く文書の中身と、expected/ の期待値（E2E-012 の表）。
//
// **expected/ は人が書いたリテラルである**（E2E-012, UT-031 と同じ考え方）。書き換えの実装
// （internal/document の PlanTask / PlanCell）を呼んで作らない——実装が間違っていれば期待値も
// 同じだけ間違い、E2E-385 が意味を失う。改行コードと BOM の付け方（withNewlines）は、書き換え前と
// 書き換え後の両方に同じように当てる。
//
// コードブロックの柵は `~~~` で書く（Go の生文字列にバッククォートを入れられないため）。GFM では
// どちらの柵も同じフェンス付きコードブロックになる。

import (
	"fmt"
	"strings"
)

// tasksLF は tasks-lf.md（E2E-381〜E2E-383, E2E-283, E2E-313）。LF。
//
// タスクは文書の順に one / two / three（ソースは [X]）/ nested / quoted / last の 6 つ。
// コードブロックの中の項目と生 HTML のチェックボックスはタスクに数えない（MD-072）。
// **スクロールできる長さの段落を後ろに置く**（E2E-383 は少し下へスクロールしてから始める）。
func tasksLF() string {
	return `# Tasks

- [ ] one
- [ ] two
- [X] three
- parent
  - [ ] nested

> - [ ] quoted

- [ ] last

~~~text
- [ ] in code
~~~

<input type="checkbox" disabled> raw html

[先頭へ](#tasks)

` + filler("tasks", 40)
}

// tableGFM は table.md と bytes/table-crlf.md の GFM の表（E2E-384, E2E-385）。
//
// 本体は apple（3、**fresh**）/ banana（10、tasks-lf.md へのリンク）/ cherry（7。Note は補われる）/
// date（1、エスケープした縦棒）/ elder（4）/ Name が空の行（5）。**cherry の行はセルを 2 つだけ書く**
// ——3 列目はソース上に存在しないセルであり、編集できない（FR-142）。
const tableGFM = `| Name | Qty | Note |
| --- | --- | --- |
| apple | 3 | **fresh** |
| banana | 10 | [tasks](tasks-lf.md) |
| cherry | 7 |
| date | 1 | x \| y |
| elder | 4 |  |
|  | 5 |  |
`

// tableCRLFExpected は bytes/table-crlf.md に E2E-385 の手順 3 をした後の表（改行は LF で書き、CRLF にして置く）。
//
//   - apple を `pear` + 縦棒 + `x` に: 縦棒はエスケープして書く（FR-142）。セルの前後の空白は残る
//   - cherry の Qty を 70 に
//   - 空の Name を fig に: 区切りの間の空白（2 つ）の 1 文字目の直後へ入る（IMP-106）。空白の数は変わらない
const tableCRLFExpected = `| Name | Qty | Note |
| --- | --- | --- |
| pear\|x | 3 | **fresh** |
| banana | 10 | [tasks](tasks-lf.md) |
| cherry | 70 |
| date | 1 | x \| y |
| elder | 4 |  |
| fig | 5 |  |
`

// tableMD は table.md（E2E-382, E2E-384）。GFM の表・生 HTML の表・スクロールできる長さの段落。LF。
func tableMD() string {
	return tableGFM + `
<table>
<tr><th>Raw</th><th>HTML</th></tr>
<tr><td>raw cell</td><td>not editable</td></tr>
</table>

` + filler("table", 40)
}

// tasksBytes は bytes/ の 3 つのタスクの文書の本文（E2E-385）。tasks-lf.md と同じ 6 つの項目と、
// コードブロックの中の項目。改行コード・BOM・末尾の改行・Front Matter は withNewlines で付ける。
const tasksBytes = `# Tasks

- [ ] one
- [ ] two
- [X] three
- parent
  - [ ] nested

> - [ ] quoted

- [ ] last

~~~text
- [ ] in code
~~~
`

// tasksBytesExpected は tasksBytes に E2E-385 の手順 1 をした後の本文。
// one をオン（x）、three をオフ（[X] が半角空白）、last をオン（x）。**コードブロックの中は変わらない。**
const tasksBytesExpected = `# Tasks

- [x] one
- [ ] two
- [ ] three
- parent
  - [ ] nested

> - [ ] quoted

- [x] last

~~~text
- [ ] in code
~~~
`

// japaneseNote は bytes/ の 4 つの文書の末尾に付ける日本語の行（E2E-237 の確認内容 1）。
//
// **BOM・CRLF・CR と多バイト文字の組み合わせ**を実機で見るために入れる。`v1.1.0-rc.1` の手動テストで
// 「日本語が記載されてない」と分かった（E2E-237 の備考）。**行を足すだけにする**——チェックボックスや
// 表のセルを増やすと、E2E-384 / E2E-385 の手順が指す番号が変わる。expected/ にも同じものを付ける。
const japaneseNote = `
日本語の行（BOM と改行コードの確認用）。全角と半角 ASCII を混ぜる。
`

// frontMatter は bytes/tasks-cr.md の先頭の YAML の Front Matter（MD-073）。
const frontMatter = `---
title: tasks-cr
tags: [e2e, bytes]
---
`

// undoMD は undo.md と expected/undo-original.md（E2E-387）。タスク 4 つと、本体 3 行の表。
//
// **expected/undo-original.md は、作り直した直後の undo.md と同じバイト列にする。** 取り消しを 3 回
// した後にバイト単位で比べる。
const undoMD = `# Undo

- [ ] first
- [ ] second
- [ ] third
- [ ] fourth

| Name | Qty |
| --- | --- |
| apple | 1 |
| banana | 2 |
| cherry | 3 |
`

// conflictMD は conflict.md（E2E-386）。タスク A / B / C。
const conflictMD = `# Conflict

- [ ] A
- [ ] B
- [ ] C
`

// readonlyMD は readonly.md（E2E-386）。作った後に読み取り専用にする。
const readonlyMD = `# Read-only

This file is read-only. Checking the box must not change it.

- [ ] read-only task
`

// lockedMD は locked/locked.md（E2E-386 の手順 9。L1 でディレクトリの権限を落とす）。
const lockedMD = `# Locked

- [ ] locked task
`

// invalidUTF8MD は invalid-utf8.md（E2E-381 の手順 7）。不正なバイト列を含むため、編集モードを始められない（FR-140）。
const invalidUTF8MD = "# Invalid UTF-8\n\n- [ ] task\n\nBroken bytes: \xff\xfe here.\n"

// permMD と linkedMD は Linux で作ったときだけ置く（E2E-385 の手順 6〜8）。
const (
	permMD = `# Permission

- [ ] perm task
`
	linkedMD = `# Linked

- [ ] linked task
`
)

// carryMD は carry.md（E2E-388）。折りたたみ（中に段落とタスク）・並べ替えできる表（Qty は上から
// 5 / 3 / 8 / 2）・横に広い Mermaid 図・スクロールできる長さの本文。
//
// **折りたたみの中のタスクの前に段落を置く**——手順 2 でその段落をクリックしてから Tab を押すと、
// 次のフォーカス先がチェックボックスになる。折りたたみを表と図より前に置くのも同じ理由である
// （並べ替えのボタンと図のボタンもフォーカスを受ける）。
func carryMD() string {
	var b strings.Builder

	b.WriteString("# Carry\n\n")
	b.WriteString(filler("carry-intro", 6))
	b.WriteString(`<details>
<summary>Folded tasks</summary>

Click this paragraph, then press Tab to move to the first checkbox below.

- [ ] carry one
- [ ] carry two

</details>

| Item | Qty |
| --- | --- |
| alpha | 5 |
| beta | 3 |
| gamma | 8 |
| delta | 2 |

`)
	b.WriteString("```mermaid\nflowchart LR\n")
	for i := 1; i < 20; i++ {
		fmt.Fprintf(&b, "  N%d[node number %d] --> N%d[node number %d]\n", i, i, i+1, i+1)
	}
	b.WriteString("```\n\n")
	b.WriteString(filler("carry", 40))

	return b.String()
}

// filler はスクロールできる長さの段落を n 個返す。検索で見つけられる語（marker）を含める。
func filler(topic string, n int) string {
	var b strings.Builder

	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b,
			"Paragraph %d of %s. This text exists so that the document can scroll; it contains the word marker "+
				"so that search has hits. MarkView rewrites only checkboxes and table cells in edit mode.\n\n",
			i, topic)
	}

	return b.String()
}

// withNewlines は LF で書いた文字列の改行を newline に置き換え、必要なら BOM を前に付け、
// trailing が偽なら末尾の改行を 1 つ落とす（E2E-385 の bytes/ の 4 つ）。
func withNewlines(text, newline string, bom, trailing bool) string {
	out := strings.ReplaceAll(text, "\n", newline)
	if !trailing {
		out = strings.TrimSuffix(out, newline)
	}
	if bom {
		out = "\xef\xbb\xbf" + out
	}

	return out
}
