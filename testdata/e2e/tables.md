# 表の並べ替え

このファイルは **表の表示上の並べ替え**（G16。E2E-361, E2E-362）の検証用です（E2E-012）。
節の番号は手動テストの手順が参照します。**節を足したり並べ替えたりしないでください。**

どの表も、ファイルに書いた行の順（元の順）は、どの列で見ても昇順にも降順にもなっていません。並べ替えたことと、元の順に戻ったことが見て分かります。

## 1. 数値の列

`Amount` は、桁区切りの `,` と末尾の `%` を無視して数として比べます（FR-130）。昇順では `-1.5` → `10%` → `250` → `1,234` の順に並びます。

| Name | Amount |
| --- | --- |
| cherry | 1,234 |
| apple | 10% |
| date | -1.5 |
| banana | 250 |

## 2. 文字列の比較と、数値と文字列の混在

- `Item` は、文字列の中の数字の並びを数の大小で比べます。`item2` は `item10` より前に来ます
- `Case` は、大文字と小文字を区別しません
- `Mixed` は数値と文字列が混ざっています。**昇順では数値が先、降順では文字列が先**に来ます

| Item | Case | Mixed |
| --- | --- | --- |
| item10 | banana | Zeta |
| item2 | Apple | 10 |
| item1 | cherry | abc |
| item21 | BANANA2 | 3 |

## 3. 空のセルと同じ値

- `Value` には空のセルがあります。**昇順でも降順でも、空のセルの行は末尾に並びます**
- `Value` が同じ `b` の行（`Order` が 1 と 4）は、並べ替えても元の順（1 が 4 より前）を保ちます

| Order | Value |
| --- | --- |
| 1 | b |
| 2 |  |
| 3 | a |
| 4 | b |
| 5 |  |
| 6 | c |

## 4. 本体が 1 行の表

見出し行を除く行が 1 行しかないため、**並べ替えのボタンが付きません**（FR-130）。

| Name | Amount |
| --- | --- |
| only | 1 |

## 5. 生 HTML の表

生 HTML で書いた表は対象にしないため、**並べ替えのボタンが付きません**（FR-130, MD-072）。

<table>
<thead><tr><th>Name</th><th>Amount</th></tr></thead>
<tbody>
<tr><td>raw-b</td><td>2</td></tr>
<tr><td>raw-a</td><td>1</td></tr>
<tr><td>raw-c</td><td>3</td></tr>
</tbody>
</table>

## 6. 見出しの文字が長い表

見出しの文字が長くても、**並べ替えのボタンが見出しの文字に重ならず**、右端・上下中央に描かれます（DSP-125）。

| A very long column header that takes up much of the table width | Another long header for the second column | Short |
| --- | --- | --- |
| second row value | beta | 2 |
| first row value | alpha | 1 |
| third row value | gamma | 3 |
