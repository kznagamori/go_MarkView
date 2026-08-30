package document

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// writeFile はテスト用のファイルを作り、そのパスを返す。
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("テストファイルを作れない: %v", err)
	}
	return p
}

// truncateFile は指定サイズのファイルを作る。
//
// 中身は書かない。サイズ判定は os.Stat の結果だけを見るため、実際に
// バイトを書く必要がない（超過するファイルは読み込みまで進まない）。
func truncateFile(t *testing.T, dir, name string, size int64) string {
	t.Helper()

	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("テストファイルを作れない: %v", err)
	}
	defer f.Close()

	if err := f.Truncate(size); err != nil {
		t.Fatalf("サイズを設定できない: %v", err)
	}
	return p
}

// TestCheckSize はサイズ閾値の判定を検証する
// （UT-104。根拠: FR-016 / IMP-101, IMP-102）。
//
// **閾値ちょうどは超過ではない**ことと、**ErrTooLarge は Confirmed でも
// 覆らない**ことが、この機能で最も間違えやすい（UT-013）。実ファイルを
// 作らずに判定だけを取り出しているのは、50 MiB のファイルを何本も作らずに
// 境界を網羅するためである。
func TestCheckSize(t *testing.T) {
	tests := []struct {
		name      string
		size      int64
		confirmed bool
		wantErr   error
		wantLimit int64
	}{
		// 通常の範囲
		{"0 バイト", 0, false, nil, 0},
		{"1 バイト", 1, false, nil, 0},
		{"確認閾値の 1 つ下", ConfirmThreshold - 1, false, nil, 0},

		// UT-104 ケース 2: 閾値ちょうどは超過ではない
		{"確認閾値ちょうど", ConfirmThreshold, false, nil, 0},

		// UT-104 ケース 3: 確認が要る
		{"確認閾値を 1 バイト超える", ConfirmThreshold + 1, false, ErrNeedsConfirm, ConfirmThreshold},

		// UT-104 ケース 4: 確認済みなら読む
		{"確認閾値を超えるが確認済み", ConfirmThreshold + 1, true, nil, 0},
		{"上限の 1 つ下で確認済み", MaxSize - 1, true, nil, 0},

		// UT-104 ケース 5: 上限ちょうども超過ではない
		{"上限ちょうどで確認済み", MaxSize, true, nil, 0},

		// UT-104 ケース 6: 上限超過は確認済みでも拒む
		{"上限を 1 バイト超え、確認済み", MaxSize + 1, true, ErrTooLarge, MaxSize},
		{"上限を 1 バイト超え、未確認", MaxSize + 1, false, ErrTooLarge, MaxSize},

		// 未確認の場合、上限ちょうどは先に確認を求める
		{"上限ちょうどで未確認", MaxSize, false, ErrNeedsConfirm, ConfirmThreshold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, err := checkSize(tt.size, tt.confirmed)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("checkSize(%d, %v) のエラー = %v, want %v", tt.size, tt.confirmed, err, tt.wantErr)
			}
			if limit != tt.wantLimit {
				t.Errorf("checkSize(%d, %v) の閾値 = %d, want %d", tt.size, tt.confirmed, limit, tt.wantLimit)
			}
		})
	}
}

// TestThresholds は閾値の実数値を固定する（IMP-101, FR-016）。
//
// 「10 MB」を 10,000,000 と解釈する実装との取り違えを防ぐ。
func TestThresholds(t *testing.T) {
	if ConfirmThreshold != 10485760 {
		t.Errorf("ConfirmThreshold = %d, want %d（10 MiB）", ConfirmThreshold, 10485760)
	}
	if MaxSize != 52428800 {
		t.Errorf("MaxSize = %d, want %d（50 MiB）", MaxSize, 52428800)
	}
}

// TestLoad は正常な読み込みを検証する
// （UT-104 ケース 1・2・7。根拠: FR-016, FR-021 / IMP-102）。
func TestLoad(t *testing.T) {
	dir := t.TempDir()
	r := renderer.New()

	t.Run("通常のファイル", func(t *testing.T) {
		p := writeFile(t, dir, "a.md", "# 見出し\n\n本文\n")

		doc, err := Load(r, p, LoadOptions{})
		if err != nil {
			t.Fatalf("Load がエラーを返した: %v", err)
		}

		if doc.Path != p {
			t.Errorf("Path = %q, want %q", doc.Path, p)
		}
		if doc.LineCount != 3 {
			t.Errorf("LineCount = %d, want 3", doc.LineCount)
		}
		if !strings.Contains(doc.HTML, "<h1") {
			t.Errorf("HTML に見出しがない: %q", doc.HTML)
		}
		if len(doc.Headings) != 1 || doc.Headings[0].Text != "見出し" {
			t.Errorf("Headings = %+v, want 見出しが 1 件", doc.Headings)
		}
		if doc.NeedsMermaid || doc.NeedsKaTeX {
			t.Errorf("遅延ロードのフラグが立っている: mermaid=%v katex=%v", doc.NeedsMermaid, doc.NeedsKaTeX)
		}
		if doc.Warnings == nil {
			t.Error("Warnings が nil。空スライスを返す規約に反する")
		}
		if len(doc.Warnings) != 0 {
			t.Errorf("Warnings = %+v, want 空", doc.Warnings)
		}
	})

	// UT-104 ケース 7
	t.Run("空のファイル", func(t *testing.T) {
		p := writeFile(t, dir, "empty.md", "")

		doc, err := Load(r, p, LoadOptions{})
		if err != nil {
			t.Fatalf("Load がエラーを返した: %v", err)
		}
		if doc.Size != 0 {
			t.Errorf("Size = %d, want 0", doc.Size)
		}
		if doc.HTML != "" {
			t.Errorf("HTML = %q, want 空", doc.HTML)
		}
		if doc.LineCount != 1 {
			t.Errorf("LineCount = %d, want 1", doc.LineCount)
		}
		if len(doc.Headings) != 0 {
			t.Errorf("Headings = %+v, want 空", doc.Headings)
		}
	})

	t.Run("実バイト数を Size に入れる", func(t *testing.T) {
		const content = "abcde"
		p := writeFile(t, dir, "size.md", content)

		doc, err := Load(r, p, LoadOptions{})
		if err != nil {
			t.Fatalf("Load がエラーを返した: %v", err)
		}
		if doc.Size != int64(len(content)) {
			t.Errorf("Size = %d, want %d", doc.Size, len(content))
		}
	})

	t.Run("不正な UTF-8 を含む", func(t *testing.T) {
		p := filepath.Join(dir, "invalid.md")
		if err := os.WriteFile(p, []byte{'a', 0xFF, 'b'}, 0o644); err != nil {
			t.Fatal(err)
		}

		doc, err := Load(r, p, LoadOptions{})
		if err != nil {
			t.Fatalf("Load がエラーを返した（読み込みは失敗させない。FR-021）: %v", err)
		}
		if len(doc.Warnings) != 1 || doc.Warnings[0].Kind != WarnInvalidEncoding {
			t.Errorf("Warnings = %+v, want WarnInvalidEncoding が 1 件", doc.Warnings)
		}
		if !strings.Contains(doc.HTML, "\uFFFD") {
			t.Errorf("置換文字が本文にない: %q", doc.HTML)
		}
	})

	t.Run("Mermaid と数式を含む", func(t *testing.T) {
		p := writeFile(t, dir, "lazy.md", "~~~mermaid\ngraph TD\n~~~\n\n$a+b$\n")

		doc, err := Load(r, p, LoadOptions{})
		if err != nil {
			t.Fatalf("Load がエラーを返した: %v", err)
		}
		if !doc.NeedsMermaid || !doc.NeedsKaTeX {
			t.Errorf("フラグが立っていない: mermaid=%v katex=%v", doc.NeedsMermaid, doc.NeedsKaTeX)
		}
	})

	t.Run("相対パスを絶対パスに直す", func(t *testing.T) {
		// カレントディレクトリを移して相対パスで開く。t.Chdir は
		// テスト終了時に元へ戻す（UT-017）。filepath.Rel で相対パスを
		// 作る方式にすると、一時ディレクトリが別ドライブにある環境で
		// 常に Skip され、何も検証しないテストになる。
		sub := t.TempDir()
		writeFile(t, sub, "rel.md", "x")
		t.Chdir(sub)

		doc, err := Load(r, "rel.md", LoadOptions{})
		if err != nil {
			t.Fatalf("Load がエラーを返した: %v", err)
		}
		if !filepath.IsAbs(doc.Path) {
			t.Errorf("Path = %q, 絶対パスでない（IMP-025）", doc.Path)
		}
		if filepath.Base(doc.Path) != "rel.md" {
			t.Errorf("Path = %q, want 末尾が rel.md", doc.Path)
		}
	})

	// UT-104 ケース 2 を実ファイルで確かめる。
	t.Run("確認閾値ちょうどのファイル", func(t *testing.T) {
		p := truncateFile(t, dir, "exact.md", ConfirmThreshold)

		doc, err := Load(r, p, LoadOptions{})
		if err != nil {
			t.Fatalf("閾値ちょうどで失敗した: %v", err)
		}
		if doc.Size != ConfirmThreshold {
			t.Errorf("Size = %d, want %d", doc.Size, ConfirmThreshold)
		}
	})
}

// TestLoad_SizeErrors はサイズ超過の返し方を検証する
// （UT-104 ケース 3・4・6。根拠: FR-016 / IMP-102）。
func TestLoad_SizeErrors(t *testing.T) {
	dir := t.TempDir()
	r := renderer.New()

	t.Run("確認閾値を超えると確認を求める", func(t *testing.T) {
		p := truncateFile(t, dir, "confirm.md", ConfirmThreshold+1)

		_, err := Load(r, p, LoadOptions{})
		if !errors.Is(err, ErrNeedsConfirm) {
			t.Fatalf("エラー = %v, want ErrNeedsConfirm", err)
		}

		var se *SizeError
		if !errors.As(err, &se) {
			t.Fatalf("*SizeError でラップされていない: %v", err)
		}
		if se.Size != ConfirmThreshold+1 {
			t.Errorf("SizeError.Size = %d, want %d", se.Size, ConfirmThreshold+1)
		}
		if se.Limit != ConfirmThreshold {
			t.Errorf("SizeError.Limit = %d, want %d", se.Limit, ConfirmThreshold)
		}
		if se.Path != p {
			t.Errorf("SizeError.Path = %q, want %q", se.Path, p)
		}
	})

	t.Run("確認済みなら閾値を超えても読む", func(t *testing.T) {
		p := truncateFile(t, dir, "confirmed.md", ConfirmThreshold+1)

		doc, err := Load(r, p, LoadOptions{Confirmed: true})
		if err != nil {
			t.Fatalf("Confirmed でも読めない: %v", err)
		}
		if doc.Size != ConfirmThreshold+1 {
			t.Errorf("Size = %d, want %d", doc.Size, ConfirmThreshold+1)
		}
	})

	t.Run("上限を超えると確認済みでも拒む", func(t *testing.T) {
		p := truncateFile(t, dir, "toolarge.md", MaxSize+1)

		_, err := Load(r, p, LoadOptions{Confirmed: true})
		if !errors.Is(err, ErrTooLarge) {
			t.Fatalf("エラー = %v, want ErrTooLarge", err)
		}
		if errors.Is(err, ErrNeedsConfirm) {
			t.Error("ErrNeedsConfirm としても判定できてしまう")
		}

		var se *SizeError
		if !errors.As(err, &se) {
			t.Fatalf("*SizeError でラップされていない: %v", err)
		}
		if se.Limit != MaxSize {
			t.Errorf("SizeError.Limit = %d, want %d", se.Limit, MaxSize)
		}
	})
}

// TestLoad_Errors は読み込みエラーの種別を検証する
// （UT-106。根拠: FR-110 / IMP-021, IMP-102）。
//
// エラーメッセージの文字列は比較しない。種別のみを errors.Is で見る（UT-042）。
func TestLoad_Errors(t *testing.T) {
	dir := t.TempDir()
	r := renderer.New()

	writeFile(t, dir, "note.txt", "x")
	for _, name := range []string{"sub", "dir.md"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name string
		path string
		want error
	}{
		// UT-106 ケース 1
		{"存在しないパス", filepath.Join(dir, "nosuch.md"), ErrNotFound},
		{"存在しないディレクトリの下", filepath.Join(dir, "nosuch", "a.md"), ErrNotFound},

		// UT-106 ケース 3
		{"拡張子が Markdown でない", filepath.Join(dir, "note.txt"), ErrNotMarkdown},
		{"拡張子がない", filepath.Join(dir, "README"), ErrNotMarkdown},

		// UT-106 ケース 4: ディレクトリ
		{"ディレクトリ", filepath.Join(dir, "sub"), ErrNotMarkdown},
		{"Markdown の拡張子を持つディレクトリ", filepath.Join(dir, "dir.md"), ErrNotMarkdown},

		// 検査順序（IMP-102）: 拡張子は存在確認より先に見る
		{"存在しないうえ拡張子も違う", filepath.Join(dir, "nosuch.txt"), ErrNotMarkdown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Load(r, tt.path, LoadOptions{})
			if doc != nil {
				t.Errorf("エラー時に Document が返っている: %+v", doc)
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("エラー = %v, want %v", err, tt.want)
			}
		})
	}
}

// TestLoad_Permission は読み取り権限のないファイルを検証する
// （UT-106 ケース 2。根拠: FR-110 / IMP-021）。
//
// Windows では権限を落としたファイルを再現しにくいため実行しない
// （UT-106 が明示的に認めている）。
func TestLoad_Permission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows では読み取り権限を落としたファイルを再現できない（UT-106 ケース 2）")
	}

	dir := t.TempDir()
	p := writeFile(t, dir, "secret.md", "x")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Fatalf("権限を変更できない: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	if _, err := Load(renderer.New(), p, LoadOptions{}); !errors.Is(err, ErrPermission) {
		t.Errorf("エラー = %v, want ErrPermission", err)
	}
}
