// gentestdata は E2E の検証用データのうち、**リポジトリに含めないもの**を作る
// （E2E-012）。
//
//	go run ./scripts/gentestdata          # testdata/e2e/generated/ に作る
//	go run ./scripts/gentestdata -clean   # 消す
//
// 巨大ファイルをコミットしないのは、クローンと CI のたびに数十 MB を運ぶ
// ことになるためである。中身は決まった規則で作れるので、必要なときに
// 生成すれば足りる。**生成物は .gitignore の対象**。
//
// 作るものは 4 つ。
//
//	large-12mb.md   10 MiB 超 50 MiB 以下。確認画面が出る（FR-016, E2E-322）
//	large-60mb.md   50 MiB 超。上限超過の表示になる（FR-016, E2E-322）
//	deep-nest.md    1000 段の入れ子リスト（FR-111, E2E-323）
//	huge-table.md   巨大な表（FR-111, E2E-323）
//
// 後ろの 2 つは「異常終了しないこと」を確かめるためのもので、**描画が
// 完了しなくてもよい**。時間がかかったうえで表示されるか、エラー表示に
// なるか、どちらでも合格である（E2E-323）。
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FR-016 の閾値（IMP-101）。**2 進接頭辞で解釈する。**
const (
	mib          int64 = 1 << 20
	confirmSize  int64 = 12 * mib // 10 MiB 超 50 MiB 以下
	overMaxSize  int64 = 60 * mib // 50 MiB 超
	nestDepth          = 1000     // 入れ子リストの段数
	tableColumns       = 30
	tableRows          = 3000
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gentestdata:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", filepath.Join("testdata", "e2e", "generated"), "生成先")
	clean := flag.Bool("clean", false, "生成先を削除する")
	flag.Parse()

	if *clean {
		return remove(*dir)
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return fmt.Errorf("生成先を作れない: %w", err)
	}

	// 生成先が何のディレクトリかを、そこを見た人にも分かるようにしておく。
	if err := writeFile(filepath.Join(*dir, "README.md"), writeNotice); err != nil {
		return err
	}

	jobs := []struct {
		name  string
		write func(*bufio.Writer) error
	}{
		{"large-12mb.md", func(w *bufio.Writer) error { return writeLarge(w, confirmSize, "12 MiB") }},
		{"large-60mb.md", func(w *bufio.Writer) error { return writeLarge(w, overMaxSize, "60 MiB") }},
		{"deep-nest.md", writeDeepNest},
		{"huge-table.md", writeHugeTable},
	}

	fmt.Printf("%-16s %14s\n", "ファイル", "バイト")

	for _, job := range jobs {
		path := filepath.Join(*dir, job.name)
		if err := writeFile(path, job.write); err != nil {
			return err
		}

		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("%s の大きさを取れない: %w", job.name, err)
		}

		fmt.Printf("%-16s %14d\n", job.name, info.Size())
	}

	fmt.Printf("\n生成先: %s\n", *dir)

	return nil
}

func remove(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("削除できない: %w", err)
	}

	fmt.Printf("削除した: %s\n", dir)

	return nil
}

// writeFile はファイルを作り、書き込み関数へ渡す。
//
// **バッファを大きく取る。** 60 MiB を既定の 4 KiB で書くと write が
// 15,000 回を超え、生成そのものが検証の待ち時間になる。
func writeFile(path string, write func(*bufio.Writer) error) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("作成できない (%s): %w", path, err)
	}
	defer file.Close() //nolint:errcheck // 下の Sync とエラーで結果は判断できる

	buffered := bufio.NewWriterSize(file, 1<<20)

	if err := write(buffered); err != nil {
		return fmt.Errorf("書き込めない (%s): %w", path, err)
	}

	if err := buffered.Flush(); err != nil {
		return fmt.Errorf("書き出せない (%s): %w", path, err)
	}

	return errors.Join(file.Sync(), file.Close())
}

func writeNotice(w *bufio.Writer) error {
	_, err := w.WriteString(`# 生成された検証用データ

**このディレクトリは ` + "`go run ./scripts/gentestdata`" + ` が作る。手で書き換えない。**

E2E-012 により、巨大ファイルはリポジトリに含めず検証のたびに生成する。
` + "`.gitignore`" + ` の対象であり、コミットされない。

| ファイル | 用途 |
| --- | --- |
| ` + "`large-12mb.md`" + ` | 10 MiB 超 50 MiB 以下。確認画面が出る（FR-016, E2E-322） |
| ` + "`large-60mb.md`" + ` | 50 MiB 超。上限超過の表示になる（FR-016, E2E-322） |
| ` + "`deep-nest.md`" + ` | 1000 段の入れ子リスト（FR-111, E2E-323） |
| ` + "`huge-table.md`" + ` | 巨大な表（FR-111, E2E-323） |

消すときは ` + "`go run ./scripts/gentestdata -clean`" + ` を実行する。
`)

	return err
}

// writeLarge は指定した大きさを超えるまで本文を並べる。
//
// **見出しを混ぜる。** 単一の巨大な段落にすると、変換の負荷が本物の
// 文書とかけ離れる。見出しがあればアウトライン（FR-040）にも数千件が
// 並び、そちらの限界も同時に見える。
func writeLarge(w *bufio.Writer, size int64, label string) error {
	written, err := w.WriteString(fmt.Sprintf(`# 大きな文書（%s）

このファイルは `+"`scripts/gentestdata`"+` が生成したものです（E2E-012）。
FR-016 の閾値の振る舞いを確かめるために使います（E2E-322）。

`, label))
	if err != nil {
		return err
	}

	total := int64(written)

	for section := 1; total < size; section++ {
		n, err := w.WriteString(largeSection(section))
		if err != nil {
			return err
		}

		total += int64(n)
	}

	return nil
}

func largeSection(section int) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## 第 %d 節\n\n", section)

	// 1 節あたり 4 段落。長さを稼ぐと同時に、検索（FR-080）で大量の
	// ヒットを作れるよう同じ語を入れておく。
	for paragraph := 1; paragraph <= 4; paragraph++ {
		fmt.Fprintf(&b,
			"第 %d 節の第 %d 段落です。marker という語をここに含めています。"+
				"MarkView は Markdown の閲覧に特化した軽量デスクトップアプリケーションであり、"+
				"編集機能を持ちません。変換とシンタックスハイライトは Go 側で行います。\n\n",
			section, paragraph)
	}

	return b.String()
}

// writeDeepNest は 1000 段の入れ子リストを書く（E2E-323）。
//
// 深さは変換器の再帰に効く。**異常終了しないこと**が確認点であり、
// 描画しきれなくてもよい。
func writeDeepNest(w *bufio.Writer) error {
	_, err := w.WriteString(fmt.Sprintf(`# 極端に深い入れ子（%d 段）

このファイルは `+"`scripts/gentestdata`"+` が生成したものです（E2E-012）。
**確認点は「異常終了しないこと」**です（FR-111, E2E-323）。
描画に時間がかかっても、最終的に表示されるかエラー表示になれば合格です。

`, nestDepth))
	if err != nil {
		return err
	}

	for depth := 0; depth < nestDepth; depth++ {
		if _, err := w.WriteString(strings.Repeat("  ", depth)); err != nil {
			return err
		}

		if _, err := fmt.Fprintf(w, "- %d 段目\n", depth+1); err != nil {
			return err
		}
	}

	_, err = w.WriteString("\n入れ子はここまでです。\n")

	return err
}

// writeHugeTable は巨大な表を書く（E2E-323）。
//
// 表は 1 セルごとに要素が増えるため、列数と行数の積がそのまま DOM の
// 規模になる。30 x 3000 で 9 万セルとなり、表の描画（DSP-121）が
// 現実的な上限に触れる。
func writeHugeTable(w *bufio.Writer) error {
	_, err := fmt.Fprintf(w, `# 巨大な表（%d 列 x %d 行）

このファイルは `+"`scripts/gentestdata`"+` が生成したものです（E2E-012）。
**確認点は「異常終了しないこと」**です（FR-111, E2E-323）。

`, tableColumns, tableRows)
	if err != nil {
		return err
	}

	header := make([]string, tableColumns)
	divider := make([]string, tableColumns)

	for column := range header {
		header[column] = fmt.Sprintf("列 %d", column+1)
		divider[column] = "---"
	}

	if _, err := fmt.Fprintf(w, "| %s |\n| %s |\n",
		strings.Join(header, " | "), strings.Join(divider, " | ")); err != nil {
		return err
	}

	cells := make([]string, tableColumns)

	for row := 1; row <= tableRows; row++ {
		for column := range cells {
			cells[column] = fmt.Sprintf("R%dC%d", row, column+1)
		}

		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(cells, " | ")); err != nil {
			return err
		}
	}

	return nil
}
