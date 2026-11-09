package main

// editformat.go — generated/edit/ に置く、選んだ文字の修飾の文書と、その期待値（E2E-012 の表。FR-146, FR-147。
// G20 の E2E-404 / E2E-405）。
//
// **expected/ は人が書いたリテラルである**（editdata.go の冒頭と同じ。document.PlanFormat を呼んで作らない）。
// 期待値は IMP-124 の例の表の形に従う（UT-120 と同じ形）。
//
// どちらの文書も、**番号の付いた見出しの下にブロックを 1 つずつ置く**（ケースは「3 節の …」と番号で指す）。
// バッククォートは editcode.go と同じく ' で書いて tick で置き換える。

// formatStrikeMD は format-strike.md（E2E-404）。見出し ## 1〜## 12。LF。
var formatStrikeMD = tick(`## 1

今日は会議です

## 2

<del>あいう</del>

## 3

**重要**な点

## 4

日本語**重要**です

## 5

~~あいう~~

## 6

<del>あ**いう**え</del>

## 7

'code' と文字

## 8

脚注つきの文[^1]

[^1]: 脚注の本文

## 9

| 列 |
| --- |
| セルの文字 |

## 10

[リンクの文字](https://example.com/format)

## 11

### 見出しの文字

## 12

> [!NOTE]
> アラートの本文
`)

// formatStrikeExpected は format-strike.md に E2E-404 の手順 3〜14 をした後の内容。
//
//   - 1 節: 会議 → 今日は<del>会議</del>です（手順 3）
//   - 2 節: い を外す → <del>あ</del>い<del>う</del>（手順 4。一部を外す）
//   - 3 節: 要な → **重<del>要</del>**<del>な</del>点（手順 5。境界ごとに分けて差し込む）
//   - 4 節: 重 だけは拒まれ（日本語に接した太字の一部）、重要 で日本語<del>**重要**</del>です（手順 6。外側に差し込む）
//   - 5 節: い → あいう（手順 7。手書きの取り消し線は全体を外す）
//   - 6 節: 変わらない（手順 8。分けられない）
//   - 7 節: de と → <del>`code` と</del>文字（手順 9。インラインコードの全体まで広げる）
//   - 8 節: 脚注の本文（戻りリンクは選択から外す。手順 10）
//   - 9〜12 節: セル・リンクの文字・見出し・Alert の本文の一部（手順 11）
//   - 手順 12・13 は何も起きず、手順 14 は取り消しとやり直しを 1 回ずつ（12 節の書き込みが残る）
var formatStrikeExpected = tick(`## 1

今日は<del>会議</del>です

## 2

<del>あ</del>い<del>う</del>

## 3

**重<del>要</del>**<del>な</del>点

## 4

日本語<del>**重要**</del>です

## 5

あいう

## 6

<del>あ**いう**え</del>

## 7

<del>'code' と</del>文字

## 8

脚注つきの文[^1]

[^1]: <del>脚注の本文</del>

## 9

| 列 |
| --- |
| <del>セル</del>の文字 |

## 10

[<del>リンク</del>の文字](https://example.com/format)

## 11

### <del>見出し</del>の文字

## 12

> [!NOTE]
> <del>アラート</del>の本文
`)

// formatColorMD は format-color.md（E2E-405）。見出し ## 1〜## 8。LF。
//
// 2・3・7 節は MarkView の形の赤、4 節は手で書いた揺れ（コロンの後に空白が無く、大文字の 16 進）、
// 5 節は赤の中の太字（入れ子で替える例。UT-120 のケース 15 と同じ形）。
const formatColorMD = `## 1

赤くする文字

## 2

<span style="color: #d1242f">赤い文字</span>

## 3

<span style="color: #d1242f">赤い文字</span>

## 4

<span style="color:#D1242F">手書きの赤</span>

## 5

<span style="color: #d1242f">あ**いうえ**お</span>

## 6

色の無い文字

## 7

<span style="color: #d1242f">赤い文字</span>

## 8

- リストの項目
`

// formatColorExpected は format-color.md に E2E-405 の手順 1〜7 をした後の内容。**色は Light の値を
// 決めた形（コロンの後に半角空白 1 つ、小文字の 16 進）で書く**（FR-147）。
//
//   - 1 節: 赤く を Red（手順 1）
//   - 2 節: 文字 を Blue（手順 2。赤の span を分ける）
//   - 3 節: 赤い を Default（手順 3。端に接するのでタグを動かす）
//   - 4 節: 全体を Green（手順 4。手で書いた赤も MarkView の形として読み、style を書き直す）
//   - 5 節: う を Blue は入れ子で替える。い を Default は拒まれる（手順 5。IMP-124 の 7）
//   - 6・7 節: 変わらない（手順 6。同じ色・色の無い文字に Default。書き込まない）
//   - 8 節: 項目 に取り消し線を付けてから Purple。**色の span を <del> の外側に差し込む**（手順 7。
//     IMP-124 の 4。4.80.0。利用者の判断）
const formatColorExpected = `## 1

<span style="color: #d1242f">赤く</span>する文字

## 2

<span style="color: #d1242f">赤い</span><span style="color: #0969da">文字</span>

## 3

赤い<span style="color: #d1242f">文字</span>

## 4

<span style="color: #1a7f37">手書きの赤</span>

## 5

<span style="color: #d1242f">あ**い<span style="color: #0969da">う</span>え**お</span>

## 6

色の無い文字

## 7

<span style="color: #d1242f">赤い文字</span>

## 8

- リストの<span style="color: #8250df"><del>項目</del></span>
`

// formatVisibleMD は format-visible.md（E2E-404 の手順 16・17）。**見える文字の数え方**（IMP-123 の表の行）を
// 見る段落 1 つと、その脚注の定義。数式・画像・脚注の参照・絵文字・インラインコードを含む。
// **バイト単位では比べない**（expected/ に置かない）。画像は testdata/images/sample.png（1 x 1）。
var formatVisibleMD = tick(`数式 $x^2$ と画像 ![i](../../../images/sample.png) と脚注の参照[^1]と絵文字 :sparkles: とインラインコード 'code' を含む段落です。

[^1]: 脚注の定義です。
`)
