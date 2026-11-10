// gentestdata は E2E の検証用データのうち、**リポジトリに含めないもの**を作る
// （E2E-012）。
//
//	go run ./scripts/gentestdata          # testdata/e2e/generated/ に作る
//	go run ./scripts/gentestdata -edit    # generated/edit/ の中身だけを作り直す（G18・G20 の各ケースと、ファイルを書き換える手順の前）
//	go run ./scripts/gentestdata -clean   # 消す（generated/edit/ を含む）
//
// 巨大ファイルをコミットしないのは、クローンと CI のたびに数十 MB を運ぶ
// ことになるためである。中身は決まった規則で作れるので、必要なときに
// 生成すれば足りる。**生成物は .gitignore の対象**。
//
// 作るものは 9 つ（と、生成物である旨の README.md）。
//
//	large-12mb.md     10 MiB 超 50 MiB 以下。確認画面が出る（FR-016, E2E-322）
//	large-60mb.md     50 MiB 超。上限超過の表示になる（FR-016, E2E-322）
//	large-1mb.md      約 1 MiB。編集モードを始めたときの描き直しの時間（E2E-381。large1mb.go）
//	large-12mb.png    10 MiB 超 50 MiB 以下の、画像として読める PNG（FR-027, E2E-395。images.go）
//	large-60mb.png    50 MiB 超。中身は画像でない（FR-027, E2E-395。images.go）
//	huge-pixels.png   30000 x 30000 px の 1 色の PNG（FR-027, FR-111, E2E-395。images.go）
//	deep-nest.md      1000 段の入れ子リスト（FR-111, E2E-323）
//	huge-table.md     巨大な表（FR-111, E2E-323）
//	slow-plantuml.md  描き終えるまで十数秒かかる PlantUML（FR-080, E2E-271）
//
// deep-nest.md と huge-table.md は「異常終了しないこと」を確かめるためのもので、**描画が
// 完了しなくてもよい**。時間がかかったうえで表示されるか、エラー表示に
// なるか、どちらでも合格である（E2E-323）。
//
// **-edit は、編集モードとファイルを書き換える手順の検証用の generated/edit/ だけを作り直す**
// （edit.go。G18 の E2E-381〜E2E-388、G20 の E2E-401〜E2E-405 と、E2E-236・E2E-262・E2E-352・
// E2E-354・E2E-362・E2E-391・E2E-394 ほか。E2E-206）。ケースがファイルを書き換えるため、ケースごとに
// 作り直す。大きなファイルは作り直さない。
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

	// slow-plantuml.md（E2E-271。BUG-017 の確認）。**枚数と線の数が待ち時間である。**
	// v1.1.0（PlantUML 1.2026.8）の実測（Chromium）は 1 枚 0.7 秒ほどで、16 枚で 12 秒ほどだった。**1.2026.9 では
	// 1 枚 0.15〜0.35 秒ほどに縮み**、16 枚では検索を打つ前に描き終えうるため、60 枚にした（BUG-030 の計測で気づいた。
	// 40 枚で 6.5 秒だった）。**線は増やさない**——1 枚を描く間はメインスレッドが止まり（BUG-030）、重くすると検索の語を
	// 打ちにくい。
	slowDiagrams = 60 // 埋めの図の枚数
	slowClasses  = 60 // 埋めの図 1 枚のクラスの数
	slowEdges    = 300
	// 最後の図（縦に長いシーケンス図）の、区間 1 つあたりの繰り返し（1 回で 4 本）。**上限（100,000 px。MD-083）を
	// 超える数**——110 で高さ約 116,000 px（Chromium の実測は 100 で 105,960 px。BUG-030）。v1.2.0-rc.1 までは
	// 4096 px を超える class 図（40 個。BUG-010）だった
	tooLargeRounds = 110
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
	edit := flag.Bool("edit", false, "編集モードの検証用の edit/ の中身だけを作り直す")
	flag.Parse()

	if *clean {
		return remove(*dir)
	}

	if *edit {
		return generateEdit(filepath.Join(*dir, "edit"))
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
		{"large-1mb.md", writeLarge1MB},
		{"large-12mb.png", writeLargePNG},
		{"large-60mb.png", writeOverMaxPNG},
		{"huge-pixels.png", writeHugePixelsPNG},
		{"deep-nest.md", writeDeepNest},
		{"huge-table.md", writeHugeTable},
		{"slow-plantuml.md", writeSlowPlantUML},
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

// remove は生成先を消す。**edit/ の読み取り専用のファイルと、権限を落としたディレクトリも戻してから消す**
// （edit.go の removeAll）。
func remove(dir string) error {
	if err := removeAll(dir); err != nil {
		return err
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
| ` + "`large-1mb.md`" + ` | 約 1 MiB。編集モードを始めたときの描き直しの時間を見る（E2E-381） |
| ` + "`large-12mb.png`" + ` | 10 MiB 超 50 MiB 以下の、画像として読める PNG。確認画面が出る（FR-027, E2E-395） |
| ` + "`large-60mb.png`" + ` | 50 MiB 超。上限超過の表示になる。中身は画像でない（FR-027, E2E-395） |
| ` + "`huge-pixels.png`" + ` | 30000 x 30000 px の 1 色の PNG。固まらず落ちないことを見る（FR-027, FR-111, E2E-395） |
| ` + "`deep-nest.md`" + ` | 1000 段の入れ子リスト（FR-111, E2E-323） |
| ` + "`huge-table.md`" + ` | 巨大な表（FR-111, E2E-323） |
| ` + "`slow-plantuml.md`" + ` | 描き終えるまで十数秒かかる PlantUML（FR-080, E2E-271。検索の語は ` + "`zzqpending`" + `） |
| ` + "`edit/`" + ` | 編集モードと、ファイルを書き換える手順の検証用（G18・G20 ほか。E2E-206）。` + "`go run ./scripts/gentestdata -edit`" + ` が中身だけを作り直す。**G18・G20 の各ケースと、ファイルを書き換える手順の前に実行する** |

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
				"書き換えるのは編集モードのチェックボックス・表のセル・コードブロックと図・選んだ文字の取り消し線と文字色の 4 種類だけです。"+
				"変換とシンタックスハイライトは Go 側で行います。\n\n",
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

// writeSlowPlantUML は、描き終えるまで十数秒かかる PlantUML の文書を書く（E2E-012, E2E-271）。
//
// **時間そのものが検証用データである。** 図は上から 1 枚ずつ描かれ（IMP-233）、**最後の図の順番が
// 来るまで原文（`<pre>`）が残る。** その間に `Ctrl+F` で原文の中の語を探せることが、
// [BUG-017](../../docs/bugs/2026-09-18-bug-017-search-hits-before-rendering.md) の確認に要る。
// 手元の実測では、1 枚あたり 0.7 秒ほどで、最後の図の順番は 12 秒ほど後になる。
//
// **最後の図は上限（100,000 px。MD-083）を超える**。描かれずに理由が添えられるため、
// 「原文の文字は見えているのに、ハイライトだけが消えている」ことが無いかを目で確かめられる。
// v1.2.0 からは**描いている間も原文が出たまま**である（DSP-272。BUG-030）。
// **探す語 `zzqpending` は、冒頭の段落と最後の図の最初のメッセージの行にだけある。**
func writeSlowPlantUML(w *bufio.Writer) error {
	_, err := fmt.Fprintf(w, `# 描画に時間のかかる PlantUML（検索の確認用）

このファイルは `+"`scripts/gentestdata`"+` が生成したものです（E2E-012）。
**重い図を %d 枚並べ、最後の図の順番が来るまでに十数秒かかる**ようにしてあります。その間に検索できます。

探す語は zzqpending です。**この文書の中に 2 つだけ**あります——**この段落に 1 つ**と、
**最後の図のソースに 1 つ**（最初のメッセージの行）です。最後の図は 100,000 px を超えるため描かれず、
ソースの上に理由が出ます。**そのとき、ソースの中の語にハイライトが付いているか**を見てください（E2E-271）。

`, slowDiagrams)
	if err != nil {
		return err
	}

	for diagram := range slowDiagrams {
		if _, err := fmt.Fprintf(w, "## 埋めの図 %d\n\n```plantuml\n@startuml H%d\n", diagram+1, diagram); err != nil {
			return err
		}

		for class := range slowClasses {
			if _, err := fmt.Fprintf(w, "class H%dC%d\n", diagram, class); err != nil {
				return err
			}
		}

		// **線を多く引く。** Graphviz の配置はクラスの数より線の数で重くなる。
		for edge := range slowEdges {
			from, to := edge%slowClasses, (edge*7+3)%slowClasses
			if _, err := fmt.Fprintf(w, "H%dC%d --> H%dC%d\n", diagram, from, diagram, to); err != nil {
				return err
			}
		}

		if _, err := w.WriteString("@enduml\n```\n\n"); err != nil {
			return err
		}
	}

	// **繰り返しは処理系の !while で書く**（取り込み指令ではないため拒まれない。MD-084）。メッセージを何千行も
	// 書かずに、上限を超える高さの図を作れる。Chromium での計算は 1 秒ほどである（BUG-030）。
	_, err = fmt.Fprintf(w, "## 最後の図（100,000 px を超えるため描かれない）\n\n```plantuml\n%s\n```\n", tallSequence("toolarge", "A -> B : zzqpending\n", tooLargeRounds))

	return err
}

// tallSequence は縦に長いシーケンス図（参加者 4 人・opt 4 つ）の原文を返す（BUG-030）。rounds は区間 1 つあたりの
// 繰り返しの数で、1 回で 4 本のメッセージになる。区間は 8 つ（opt の外 4・中 4）。first は最初に置く行。
func tallSequence(name, first string, rounds int) string {
	return fmt.Sprintf(`@startuml %[1]s
participant "ブラウザ" as A
participant "API サーバ" as B
participant "認証サービス" as C
database "データベース" as D
%[2]s!procedure $calls($n, $tag)
!$i = 1
!while $i <= $n
A -> B : リクエスト $tag-$i
B -> C : 照会 $tag-$i
C --> B : 応答 $tag-$i
B --> A : レスポンス $tag-$i
!$i = $i + 1
!endwhile
!endprocedure
$calls(%[3]d, "a")
opt 条件 1
$calls(%[3]d, "b")
end
$calls(%[3]d, "c")
opt 条件 2
$calls(%[3]d, "d")
end
$calls(%[3]d, "e")
opt 条件 3
$calls(%[3]d, "f")
end
$calls(%[3]d, "g")
opt 条件 4
$calls(%[3]d, "h")
end
@enduml`, name, first, rounds)
}
