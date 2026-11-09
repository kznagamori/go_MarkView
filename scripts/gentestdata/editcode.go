package main

// editcode.go — generated/edit/ に置く、コードブロックと図の書き換えの文書と、その期待値（E2E-012 の表。
// FR-145。G20 の E2E-401〜E2E-403 と E2E-251 / E2E-382 / E2E-391）。
//
// **expected/ は人が書いたリテラルである**（editdata.go の冒頭と同じ。document.PlanCode を呼んで作らない）。
// 期待値は IMP-122 の書き込みの規則（前置き・改行コード・囲いの延長・閉じの囲いの無いブロック）に従う。
//
// **Go の生文字列にはバッククォートを書けない**ため、この中の文書では ' をバッククォートとして書き、
// tick で置き換える。**これらの文書には ' を書かない。** 囲いの文字そのものを試す 2 節（~~~text）だけは
// チルダで書く。

import "strings"

// tick は ' をバッククォートに置き換える。
func tick(s string) string {
	return strings.ReplaceAll(s, "'", "`")
}

// codeMD は code.md（E2E-012 の表。E2E-251, E2E-382, E2E-401, E2E-403）。LF。
//
// **見出しは節の番号で始める**（ケースは「12 節のリンク」のように番号で指す。4.80.0）。1 節の見出しに
// インラインコード、2 節の見出しに太字を置き、2 節のコードブロックの中に # で始まる行を置く（E2E-251）。
// 鉛筆が出るのはフェンス付きのコードブロック（1・2・8・9・11 節）と図（3・4 節）で、字下げ形式（5）・
// math（6）・生 HTML の pre（7）には出ない。10 節は前置きが行ごとに違うため淡色（E2E-382 の確認内容 8）。
// 12 節の「1 節へ」は 1 節の見出しのスラッグ（MD-021）を指す。
func codeMD() string {
	return tick(`# Code blocks

## 1. 'go' のコードブロック

'''go
func main() {
`+"\t"+`fmt.Println("hello")
}
'''

## 2. **チルダ**の囲い

~~~text
# not a heading
alpha
beta
~~~

## 3. Mermaid の図

'''mermaid
flowchart LR
    Start --> Open[Open URL] --> End
    click Open "https://example.com/code-click"
'''

## 4. PlantUML の図

'''plantuml
@startuml
Alice -> Bob : hello
@enduml
'''

## 5. 字下げ形式のコードブロック

    indented code
    second line

## 6. math のブロック

'''math
x^2 + y^2 = z^2
'''

## 7. 生 HTML の pre

<pre>raw html pre</pre>

## 8. 引用の中のコードブロック

> '''sh
> echo in quote
> '''

## 9. リストの項目の中のコードブロック

- リストの項目
  '''
  in list
  '''

## 10. 前置きが行ごとに違うブロック

> '''
> a
>b
> '''

## 11. text のコードブロック

'''text
plain
'''

## 12. リンク

- [table.md へ](table.md)
- <https://example.com/code-link>
- [1 節へ](#1-go-のコードブロック)

## 13. タスク

- [ ] code.md のタスク

## 14. 表

| Name | Qty |
| --- | --- |
| apple | 3 |
| banana | 1 |
| cherry | 2 |

`) + filler("code", 30)
}

// codeCRLFSource は bytes/code-crlf.md の本文（E2E-402）。LF で書き、withNewlines で BOM と CRLF にして、
// **末尾の改行を落とす**（Last の節の閉じの囲いの無いブロックが、改行の無い文書の最後の行で終わる）。
//
// 節は上から Quote（引用の中）・List（リストの項目の中）・Fence（text）・Locked（引用の中で、> a の行と
// >b の行。前置きが行ごとに違う）・日本語の行を持つ段落・Last（閉じの囲いの無いブロック）。
var codeCRLFSource = tick(`## Quote

> '''sh
> echo quoted
> '''

## List

- list item
  '''
  in list
  '''

## Fence

'''text
plain
'''

## Locked

> '''
> a
>b
> '''

日本語の行を持つ段落です（BOM と CRLF の確認用）。全角と半角 ASCII を混ぜます。

## Last

'''
last
`)

// codeCRLFExpected は bytes/code-crlf.md に E2E-402 の手順 1〜6 をした後の本文（同じく BOM と CRLF、
// 末尾の改行なし）。手順 6 は取り消しとやり直しを 4 回ずつで、手順 4 の後と同じになる。
//
//   - Quote: 末尾に空の行と echo added。**空の行は前置きの末尾の空白を除いた > だけ**（IMP-122 の 4）
//   - List: in list を in list 2 に。リストの字下げ（半角空白 2 つ）は残る
//   - Fence: plain の後にバッククォート 3 つの行と more の行。**中身に 3 つの並びがあるので、開きと閉じの囲いを 4 つに
//     延ばし、言語指定 text は残す**（IMP-122 の 5・6）
//   - Locked: 書き込まない（編集欄が開かない。E2E-402 の確認内容 5）
//   - Last: last を last 2 に。**閉じの囲いを足さず、末尾に改行を足さない**（IMP-122 の 7。FR-143）
var codeCRLFExpected = tick(`## Quote

> '''sh
> echo quoted
>
> echo added
> '''

## List

- list item
  '''
  in list 2
  '''

## Fence

''''text
plain
'''
more
''''

## Locked

> '''
> a
>b
> '''

日本語の行を持つ段落です（BOM と CRLF の確認用）。全角と半角 ASCII を混ぜます。

## Last

'''
last 2
`)

// diagramPUML は diagram.puml（E2E-391 の手順 9、E2E-402 の手順 8・9）。図が 2 つの PlantUML ファイル。LF。
const diagramPUML = `@startuml first
Alice -> Bob : one
@enduml

@startuml second
Bob -> Carol : two
@enduml
`

// diagramPUMLExpected は diagram.puml に E2E-402 の手順 8・9 をした後の内容。
//
// 手順 8 で 1 つ目の図の first を renamed に、one を uno に変える（編集欄は @startuml の行から @enduml の
// 行の改行の手前まで。IMP-122）。手順 9（@enduml を消す）は図の数が変わるため拒まれ、書き込まない。
const diagramPUMLExpected = `@startuml renamed
Alice -> Bob : uno
@enduml

@startuml second
Bob -> Carol : two
@enduml
`

// flowMMD は flow.mmd（E2E-402 の手順 10・11）。**先頭に空行を 1 つ置き**、Front Matter と flowchart。LF。
const flowMMD = `
---
title: Flow
---
flowchart LR
    A --> B
`

// flowMMDExpected は flow.mmd に E2E-402 の手順 10・11 をした後の内容。
//
// 手順 10 で B を C に変える（編集欄はファイル全体。先頭の空行と末尾の改行を含む。IMP-122）。手順 11（中身を
// すべて消す）は図が無くなるため拒まれ、書き込まない。
const flowMMDExpected = `
---
title: Flow
---
flowchart LR
    A --> C
`
