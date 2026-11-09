package main

// large1mb.go — generated/large-1mb.md（E2E-012。E2E-381 の手順 16）。
//
// **編集モードを始めたときの描き直しの時間を見る文書である**（IMP-120, IMP-195。90.6.5 の「実装の時点で
// 実機で確かめると決めたもの」。NFR-011）。編集モードでは目印付きで描き直すため、目印の付くもの——
// 段落・リストの項目・タスク・表のセル——を繰り返して約 1 MiB にする。確認画面（10 MiB 超）には届かない。

import (
	"bufio"
	"fmt"
	"strings"
)

// editLargeSize は large-1mb.md の大きさ（これを超えるまで節を並べる）。
const editLargeSize int64 = 1 * mib

// writeLarge1MB は large-1mb.md を書く。
func writeLarge1MB(w *bufio.Writer) error {
	written, err := w.WriteString(`# 編集モードの描き直しの時間（約 1 MiB）

このファイルは ` + "`scripts/gentestdata`" + ` が生成したものです（E2E-012）。
編集モードを始めたときに、目印を付けて描き直す時間を見るために使います（E2E-381 の手順 16）。

`)
	if err != nil {
		return err
	}

	total := int64(written)

	for section := 1; total < editLargeSize; section++ {
		n, err := w.WriteString(large1MBSection(section))
		if err != nil {
			return err
		}

		total += int64(n)
	}

	return nil
}

// large1MBSection は 1 節（見出し・段落 2 つ・リスト・タスク・表）を返す。
func large1MBSection(section int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## 第 %d 節\n\n", section)

	for paragraph := 1; paragraph <= 2; paragraph++ {
		fmt.Fprintf(&b,
			"第 %d 節の第 %d 段落です。marker という語を含めています。編集モードでは、この段落の文字を選んで"+
				"取り消し線と文字色を付けられます。書き換えられるのは、チェックボックス・表のセル・コードブロックと図・"+
				"選んだ文字の取り消し線と文字色の 4 種類です。\n\n",
			section, paragraph)
	}

	fmt.Fprintf(&b, "- 第 %d 節のリストの項目 1\n- 第 %d 節のリストの項目 2\n- 第 %d 節のリストの項目 3\n\n",
		section, section, section)
	fmt.Fprintf(&b, "- [ ] 第 %d 節のタスク 1\n- [x] 第 %d 節のタスク 2\n\n", section, section)
	fmt.Fprintf(&b, "| 項目 | 数 | 備考 |\n| --- | --- | --- |\n| 第 %d 節の 1 | %d | 表のセル |\n| 第 %d 節の 2 | %d | 表のセル |\n\n",
		section, section, section, section+1)

	return b.String()
}
