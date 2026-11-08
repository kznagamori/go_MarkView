package renderer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kznagamori/go_MarkView/internal/localurl"
)

// render はテスト用の短縮形。変換に失敗した時点で打ち切る。
func render(t *testing.T, source string) string {
	t.Helper()

	res, err := New().Render([]byte(source), Options{})
	if err != nil {
		t.Fatalf("Render(%q) がエラーを返した: %v", source, err)
	}
	return res.HTML
}

// TestRender_GFMExtensions は goldmark に渡した拡張が有効であることを検証する
// （UT-201。根拠: MD-022, MD-024, MD-025, MD-050, MD-051 / IMP-111）。
//
// goldmark 自体の記法を網羅しない（UT-034）。ここで検出したいのは
// **拡張の登録漏れ**であり、各拡張を代表する 1 記法だけを見る。
func TestRender_GFMExtensions(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-201 ケース 1〜4: GFM
		{
			name:     "表",
			in:       "| a | b |\n| --- | --- |\n| 1 | 2 |",
			contains: []string{"<table>", "<th>a</th>", "<td>1</td>"},
		},
		{
			name:     "打消し線",
			in:       "~~del~~",
			contains: []string{"<del>del</del>"},
		},
		{
			// 属性の順序は goldmark の出力に従うため、個別の属性で確認する。
			name:     "タスクリスト",
			in:       "- [x] done",
			contains: []string{"<input", `type="checkbox"`, "checked"},
		},
		{
			name:     "裸の URL の自動リンク",
			in:       "https://example.com",
			contains: []string{`<a href="https://example.com"`},
		},

		// UT-201 ケース 5: 脚注（MD-050）
		{
			name:     "脚注の参照と定義がリンクされる",
			in:       "text[^1]\n\n[^1]: note",
			contains: []string{`id="user-content-fnref:1"`, `href="#user-content-fn:1"`, `id="user-content-fn:1"`, `href="#user-content-fnref:1"`},
		},

		// UT-201 ケース 6: 絵文字（MD-051）
		{
			// 登録漏れの検出はショートコードが残らないことで足りる。
			// 併せて ✨ に変換されていることも固定しておく。
			name: "絵文字ショートコード",
			in:   ":sparkles:",
			// サニタイズが数値文字参照を実体へ戻すため、出力には文字そのものが現れる。
			contains:    []string{"✨"},
			notContains: []string{":sparkles:"},
		},

		// UT-090 に従って追加。html.WithUnsafe() は IMP-116 のサニタイズと
		// 対で意味を持つ（IMP-111）。片方だけ外れていないかを見る。
		{
			name:     "生 HTML を通す",
			in:       "<details><summary>s</summary>d</details>",
			contains: []string{"<details>", "<summary>s</summary>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, ng := range tt.notContains {
				if strings.Contains(got, ng) {
					t.Errorf("出力に %q が含まれている\n出力: %s", ng, got)
				}
			}
		})
	}
}

// TestRender_FrontMatter は Front Matter の除去を検証する
// （UT-211。根拠: MD-073, MD-002 / IMP-111）。
//
// UT-211 ケース 5（閉じのない区切り）は**本文として描画する**（IMP-111, IMP-121）。**開きと閉じが揃う場合のみ Front Matter とみなす。**
// UT-218 のケース 7（閉じの無い YAML の位置）はこの振る舞いに依存する。
func TestRender_FrontMatter(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-211 ケース 1・2・4: 開きと閉じが揃う場合は取り除く
		{
			name:        "YAML の Front Matter",
			in:          "---\ntitle: T\nauthor: A\n---\n\n# H\n\npara",
			contains:    []string{"H</h1>", "<p>para</p>"},
			notContains: []string{"title", "author", "<hr>"},
		},
		{
			name:        "TOML の Front Matter",
			in:          "+++\ntitle = \"T\"\n+++\n\n# H\n\npara",
			contains:    []string{"H</h1>", "<p>para</p>"},
			notContains: []string{"title", "+++"},
		},

		// UT-211 ケース 3: 途中の区切りは Front Matter ではない
		{
			name:     "文書の途中の --- は水平線",
			in:       "# H\n\npara\n\n---\n\npara2",
			contains: []string{"<hr>", "<p>para2</p>"},
		},
		{
			name:        "文書の途中の +++ は本文",
			in:          "# H\n\n+++\ntitle = \"T\"\n+++",
			contains:    []string{"H</h1>", "+++"},
			notContains: []string{"<hr>"},
		},

		// UT-211 ケース 5: 閉じがない場合は本文として描画する。
		// 飲み込むと、水平線で始まるだけの文書が丸ごと消える。
		{
			name:     "閉じのない YAML は本文として描画する",
			in:       "---\ntitle: T\n\n# H",
			contains: []string{"<hr>", "title: T", "H</h1>"},
		},
		{
			name:     "閉じのない TOML は本文として描画する",
			in:       "+++\ntitle = \"T\"\n\n# H",
			contains: []string{"+++", "H</h1>"},
		},
		{
			name:     "--- だけの文書は水平線になる",
			in:       "---",
			contains: []string{"<hr>"},
		},
		{
			name:     "--- で始まり閉じがない文書の本文が残る",
			in:       "---\n\n# H\n\npara",
			contains: []string{"H</h1>", "<p>para</p>"},
		},

		// UT-090 に従って追加した境界値
		{
			name:        "空の YAML Front Matter",
			in:          "---\n---\n\n# H",
			contains:    []string{"H</h1>"},
			notContains: []string{"<hr>"},
		},
		{
			name:        "Front Matter だけの文書",
			in:          "---\ntitle: T\n---\n",
			notContains: []string{"title", "<hr>"},
		},
		{
			name:     "区切りが 1 行目にない場合は Front Matter ではない",
			in:       "\n---\ntitle: T\n---\n\n# H",
			contains: []string{"title: T", "H</h1>"},
		},
		{
			name:     "区切りに見えるが前後に文字がある行",
			in:       "+++x\ntitle = \"T\"\n+++\n\n# H",
			contains: []string{"+++x", "H</h1>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := render(t, tt.in)

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("出力に %q が含まれていない\n出力: %s", want, got)
				}
			}
			for _, ng := range tt.notContains {
				if strings.Contains(got, ng) {
					t.Errorf("出力に %q が含まれている\n出力: %s", ng, got)
				}
			}
		})
	}
}

// TestRender_DoesNotModifyInput は、変換が入力を書き換えないことを検証する。
//
// Front Matter の扱いで source を差し替えているため、呼び出し側が持つ
// バイト列を壊していないことを確かめる。壊すと、再読み込み（FR-015）で
// 同じバイト列を変換したときに結果が変わる。
func TestRender_DoesNotModifyInput(t *testing.T) {
	const source = "---\ntitle: T\n\n# H"
	raw := []byte(source)

	if _, err := New().Render(raw, Options{}); err != nil {
		t.Fatalf("Render がエラーを返した: %v", err)
	}
	if string(raw) != source {
		t.Errorf("Render が入力を書き換えた: %q, want %q", raw, source)
	}
}

// localPathOf は /__local/ の URL から絶対パスを取り出す。
//
// 比較を OS に依存させないため、区切りをスラッシュへ揃える（UT-035）。
// 復号には localurl.Decode を使う。実装のロジックを書き写しているのではなく、
// 組み立ての対になる標準の逆変換であり、期待値は人が書いたリテラルである。
func localPathOf(t *testing.T, src string) string {
	t.Helper()

	p, ok := localurl.Decode(src)
	if !ok {
		t.Fatalf("%q が /__local/ の URL になっていない", src)
	}
	return filepath.ToSlash(p)
}

// TestRewriteImageURL_Local はローカル画像の書き換えを検証する
// （UT-210 ケース 1〜3, 7。根拠: FR-022, AR-042 / IMP-118）。
func TestRewriteImageURL_Local(t *testing.T) {
	const baseDir = "/docs"

	tests := []struct {
		name string
		in   string
		want string
	}{
		// UT-210 ケース 1・2・3
		{"同じディレクトリ", "a.png", "/docs/a.png"},
		{"親をたどる相対パス", "../img/a.png", "/img/a.png"},
		{"絶対パス", "/abs/a.png", "/abs/a.png"},

		// UT-210 ケース 7: URL として正しくエスケープされること
		{"空白を含む", "a b.png", "/docs/a b.png"},
		{"%20 で書かれた空白", "a%20b.png", "/docs/a b.png"},
		{"# を含む", "a#b.png", "/docs/a#b.png"},

		// UT-090 に従って追加した境界値
		{"明示的な ./", "./a.png", "/docs/a.png"},
		{"下位ディレクトリ", "sub/a.png", "/docs/sub/a.png"},
		{"根まで戻る", "../../x.png", "/x.png"},
		{"非 ASCII のファイル名", "日本語.png", "/docs/日本語.png"},
		{"冗長な区切りを正規化する", "./sub/../a.png", "/docs/a.png"},
		// 相対パスは filepath.Join が正規化するが、絶対パスは通らない。
		{"絶対パスの冗長な区切りも正規化する", "/abs/../x.png", "/x.png"},
		{"絶対パスの ./ も正規化する", "/abs/./x.png", "/abs/x.png"},
		// http / https / data 以外のスキームはローカルパスとして扱う。
		// 存在しないパスになり配信されないため、素通しするより安全である。
		{"file スキーム", "file:///etc/passwd", "/docs/file:/etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteImageURL(tt.in, baseDir)

			if !strings.HasPrefix(got, localurl.Prefix) {
				t.Fatalf("rewriteImageURL(%q) = %q, %q で始まらない", tt.in, got, localurl.Prefix)
			}
			if p := localPathOf(t, got); p != tt.want {
				t.Errorf("rewriteImageURL(%q) が指すパス = %q, want %q", tt.in, p, tt.want)
			}
		})
	}
}

// TestRewriteImageURL_Unchanged は書き換えない URL を検証する
// （UT-210 ケース 4・5。根拠: MD-071 / IMP-118）。
func TestRewriteImageURL_Unchanged(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"https", "https://example.com/a.png"},
		{"http", "http://example.com/a.png"},
		{"大文字のスキーム", "HTTPS://example.com/a.png"},
		{"クエリ付き", "https://example.com/a.png?v=1"},
		{"data URI", "data:image/png;base64,iVBORw0KGgo="},
		{"空文字", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteImageURL(tt.in, "/docs"); got != tt.in {
				t.Errorf("rewriteImageURL(%q) = %q, 書き換えられている", tt.in, got)
			}
		})
	}
}

// TestRender_ImageAndLink は、画像だけが書き換えられ、リンクはそのまま残る
// ことを検証する（UT-210 ケース 6。根拠: IMP-118, AR-060）。
//
// リンクを書き換えると、クリックを捕捉して Go 側へ渡す経路（IMP-223）が
// 元のパスを受け取れなくなる。
func TestRender_ImageAndLink(t *testing.T) {
	got := render(t, "![alt](a.png) と [link](a.md) と [外部](https://example.com)")

	if !strings.Contains(got, `src="`+localurl.Prefix) {
		t.Errorf("画像が書き換えられていない\n出力: %s", got)
	}
	if !strings.Contains(got, `href="a.md"`) {
		t.Errorf("リンクが書き換えられている（元のパスのまま残すこと）\n出力: %s", got)
	}
	if !strings.Contains(got, `href="https://example.com"`) {
		t.Errorf("外部リンクが変わっている\n出力: %s", got)
	}
	if strings.Contains(got, `href="`+localurl.Prefix) {
		t.Errorf("リンクが /__local/ に書き換えられている\n出力: %s", got)
	}
}

// TestRender_ImageURLSurvivesSanitize は、書き換えた URL がサニタイズを
// 通り抜けることを検証する（IMP-116, IMP-118）。
//
// エスケープが二重に掛かったり、相対 URL が落とされたりすると画像が出ない。
func TestRender_ImageURLSurvivesSanitize(t *testing.T) {
	// 宛先に空白を含める場合、Markdown では <> で囲む必要がある。
	res, err := New().Render([]byte("![alt](<a b.png>)"), Options{BaseDir: "/docs"})
	if err != nil {
		t.Fatalf("Render がエラーを返した: %v", err)
	}

	m := regexp.MustCompile(`src="([^"]*)"`).FindStringSubmatch(res.HTML)
	if m == nil {
		t.Fatalf("src が出力にない\n出力: %s", res.HTML)
	}
	if p := localPathOf(t, m[1]); p != "/docs/a b.png" {
		t.Errorf("src が指すパス = %q, want %q（出力: %s）", p, "/docs/a b.png", res.HTML)
	}
}

// imgSrcs は出力 HTML の img の src を文書の順に返す。
func imgSrcs(t *testing.T, html string) []string {
	t.Helper()
	var srcs []string
	for _, e := range byTag(scanElements(t, html), "img") {
		v, _ := e.Attr("src")
		srcs = append(srcs, v)
	}
	return srcs
}

// TestRender_ImageVersion は、ローカルの画像の URL にファイルの版を付けることを検証する
// （UT-210 のケース 8〜11。根拠: FR-022, FR-014 / IMP-118, IMP-160。BUG-026）。
//
// **ケース 8 と 9 は対で意味を持つ。** 8 だけなら「固定の値を付ける」実装が通るが、版が
// 変わらなければ WebView は書き換えた画像を取り直さない（Cache-Control: no-store でも、
// ページの中で一度読んだ画像を使い回す。IMP-162 の NOTE）。
//
// 期待値はケースの中で決めた更新時刻（Unix 時刻の 1 秒 = 0x3b9aca00 ナノ秒）と大きさから
// 人が書いた（UT-031, UT-035）。更新時刻は秒の単位で与える——NTFS は 100 ナノ秒単位でしか
// 持たず、端数のあるナノ秒は丸められる。
func TestRender_ImageVersion(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "a.png")
	mtime := time.Unix(1, 0)

	write := func(t *testing.T, data string) {
		t.Helper()
		if err := os.WriteFile(img, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(img, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	renderIn := func(t *testing.T, source, baseDir string) string {
		t.Helper()
		res, err := New().Render([]byte(source), Options{BaseDir: baseDir})
		if err != nil {
			t.Fatalf("Render がエラーを返した: %v", err)
		}
		return res.HTML
	}
	// checkVersion は src が img を指し、版が want であることを確かめる。
	checkVersion := func(t *testing.T, html, want string) {
		t.Helper()
		srcs := imgSrcs(t, html)
		if len(srcs) != 1 {
			t.Fatalf("img が %d 個, want 1\n出力: %s", len(srcs), html)
		}
		path, query, found := strings.Cut(srcs[0], "?")
		if !found || query != "v="+want {
			t.Errorf("src = %q, クエリが v=%s で終わらない", srcs[0], want)
		}
		if got, wantPath := localPathOf(t, path), filepath.ToSlash(img); got != wantPath {
			t.Errorf("src が指すパス = %q, want %q", got, wantPath)
		}
	}

	// ケース 8: 3 バイトの a.png。更新時刻のナノ秒の 16 進と大きさの 16 進。
	t.Run("8 版を付ける", func(t *testing.T) {
		write(t, "abc")
		checkVersion(t, renderIn(t, "![](a.png)", dir), "3b9aca00-3")
	})

	// ケース 9: 4 バイトに書き換え、更新時刻を戻して変換し直すと、URL が変わる。
	t.Run("9 書き換えると版が変わる", func(t *testing.T) {
		write(t, "abcd")
		checkVersion(t, renderIn(t, "![](a.png)", dir), "3b9aca00-4")
	})

	// ケース 10: 存在しないローカルの画像には版を付けない（os.Stat に失敗したら付けない）。
	t.Run("10 存在しない画像", func(t *testing.T) {
		for _, src := range imgSrcs(t, renderIn(t, "![](a.png) ![](../img/a.png) ![](/abs/a.png)", "/docs")) {
			if strings.Contains(src, "?") {
				t.Errorf("存在しない画像の src に版が付いた: %q", src)
			}
		}
	})

	// ケース 11: 版を付けるのはローカルの画像の src だけ。
	t.Run("11 リモート・data・リンクには付けない", func(t *testing.T) {
		write(t, "abc")
		html := renderIn(t, "![](https://example.com/b.png) ![](data:image/png;base64,iVBORw0KGgo=) [link](a.png)", dir)
		for _, src := range imgSrcs(t, html) {
			if strings.Contains(src, "v=") {
				t.Errorf("ローカルでない画像の src に版が付いた: %q", src)
			}
		}
		for _, a := range byTag(scanElements(t, html), "a") {
			if href, _ := a.Attr("href"); href != "a.png" {
				t.Errorf("リンクの href = %q, want %q（版を付けない）", href, "a.png")
			}
		}
	})
}
