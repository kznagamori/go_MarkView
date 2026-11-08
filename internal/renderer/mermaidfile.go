package renderer

import "bytes"

// RenderMermaidFile はファイル全体を 1 つの図のブロックへ変換する（IMP-127, FR-026, MD-087）。
//
//   - **ファイル全体を 1 つの図とし**、Markdown の中の Mermaid のブロック（IMP-115）と同じ形の
//     div.code-block を 1 つだけ出す（writeMermaidBlock を共有する）。NeedsMermaid を立てる
//   - **data-source は、先頭の空行（空白とタブだけの行）を除いたもの**（MD-087 の 2）。処理系は先頭に
//     空行があると Front Matter を読まない。除くのは表示のときだけで、ファイルは変えない（書き換えの
//     編集欄にはファイル全体を出す。IMP-122）
//   - **中身が空白だけのファイル**は、図を出さず（HTML は空）、NoDiagrams を真にする（MD-087 の 3）。
//     `No diagrams found.` の文言はフロントエンドが出す（UI 文言を Go に書かない。IMP-290）
//   - 見出しは付けない（Headings は空配列。MD-087 の 4）。修飾の目印は付けない（TextRefs は見ない）
//   - **Go 側は Mermaid の構文を解かない**（AR-031）。図が 2 つある・MultiMarkdown の文書である、など
//     は処理系が構文エラーとして示す（FR-023, FR-026）
//   - 出力は IMP-116 のサニタイズを通す。パニックは recover してエラーとして返す（IMP-022）
func (r *Renderer) RenderMermaidFile(source []byte, opts Options) (result Result, err error) {
	defer recoverRender(&result, &err)

	body := trimLeadingBlankLines(source)
	if len(body) == 0 {
		return Result{Headings: []Heading{}, NoDiagrams: true}, nil
	}

	var b bytes.Buffer
	writeMermaidBlock(&b, body, diagramRef(opts.RefKey, "mermaid", 0), false)

	// 生 HTML は無い（自分で組み立てた）ため、サニタイズの後の走査は省く。
	sanitized := sanitizeAfter(r.policy.SanitizeBytes(b.Bytes()), false)

	return Result{HTML: string(sanitized), Headings: []Heading{}, NeedsMermaid: true}, nil
}

// trimLeadingBlankLines は、先頭の空白とタブだけの行（空の行を含む）を除いた残りを返す（MD-087 の 2）。
// 残りがすべて空白とタブだけなら空を返す。最初の空でない行の字下げは残す。
func trimLeadingBlankLines(src []byte) []byte {
	rest := src
	for len(rest) > 0 {
		line := rest
		next := []byte(nil)
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			line, next = rest[:i], rest[i+1:]
		}
		if len(bytes.Trim(line, " \t")) != 0 {
			return rest
		}
		rest = next
	}

	return nil
}
