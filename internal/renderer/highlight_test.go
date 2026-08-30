package renderer

import (
	"strings"
	"testing"
)

// TestRender_CodeBlockWrapper はコードブロックのラッパを検証する
// （UT-206。根拠: MD-032 / IMP-115）。
func TestRender_CodeBlockWrapper(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-206 ケース 2・3: 言語指定がない経路（境界値を先に。UT-013）
		{
			name:        "言語指定なしのフェンス",
			in:          "```\nplain\n```",
			contains:    []string{`<div class="code-block">`, "<pre><code>plain"},
			notContains: []string{"data-lang"},
		},
		{
			name:        "インデント形式のコードブロック",
			in:          "    indented",
			contains:    []string{`<div class="code-block">`, "<pre><code>indented"},
			notContains: []string{"data-lang"},
		},

		// UT-206 ケース 4: 未知の言語でもエラーにしない
		{
			name:        "未知の言語",
			in:          "```nosuchlang\nabc\n```",
			contains:    []string{`<div class="code-block" data-lang="nosuchlang">`, "<pre><code>abc"},
			notContains: []string{"chroma"},
		},

		// UT-206 ケース 1: 言語指定あり
		{
			name:     "言語指定あり",
			in:       "```go\nx := 1\n```",
			contains: []string{`<div class="code-block" data-lang="go">`, `<pre class="chroma">`},
		},

		// UT-090 に従って追加した境界値
		{
			name:     "言語名のエイリアス",
			in:       "```golang\nx := 1\n```",
			contains: []string{`data-lang="golang"`, "chroma"},
		},
		{
			name:     "言語名が大文字",
			in:       "```GO\nx := 1\n```",
			contains: []string{`data-lang="GO"`, "chroma"},
		},
		{
			name:     "空のコードブロック",
			in:       "```go\n```",
			contains: []string{`<div class="code-block" data-lang="go">`},
		},
		{
			name:        "コード中の HTML をエスケープする",
			in:          "```\n<b>x</b>\n```",
			contains:    []string{"&lt;b&gt;x&lt;/b&gt;"},
			notContains: []string{"<b>x</b>"},
		},
		{
			// goldmark-highlighting は info string の属性から行番号を有効に
			// できるが、MD-032 はそれを許さない。文書側から破れないことを見る。
			name:     "行番号を要求されても出さない",
			in:       "```go {linenos=table}\nx := 1\n```",
			contains: []string{`<div class="code-block" data-lang="go">`},
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

			// UT-206 ケース 5: 行番号の要素を含まない（MD-032）。
			// 個別の期待値ではなく全ケース共通の検査にする。
			for _, ng := range []string{"lntable", `class="lnt"`, `class="ln"`, "<table"} {
				if strings.Contains(got, ng) {
					t.Errorf("行番号の要素 %q が出力に含まれている\n出力: %s", ng, got)
				}
			}
		})
	}
}

// TestRender_Highlight はシンタックスハイライトを検証する
// （UT-208。根拠: MD-030 / IMP-114）。
func TestRender_Highlight(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		contains    []string
		notContains []string
	}{
		// UT-208 ケース 4: 未登録の言語（境界値を先に。UT-013）
		{
			name:        "未登録の言語はハイライトしない",
			in:          "```nosuchlang\nfunc main() {}\n```",
			contains:    []string{"func main() {}"},
			notContains: []string{"chroma", "<span"},
		},
		{
			name:        "言語指定なしはハイライトしない",
			in:          "```\nfunc main() {}\n```",
			contains:    []string{"func main() {}"},
			notContains: []string{"chroma", "<span"},
		},

		// UT-208 ケース 1: クラスによるハイライト
		{
			name:     "Go のコード",
			in:       "```go\nfunc main() {}\n```",
			contains: []string{`<pre class="chroma">`, `<span class=`},
		},

		// UT-208 ケース 3: エイリアス
		{
			name:     "sh",
			in:       "```sh\necho hi\n```",
			contains: []string{`<pre class="chroma">`},
		},
		{
			name:     "bash",
			in:       "```bash\necho hi\n```",
			contains: []string{`<pre class="chroma">`},
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

			// UT-208 ケース 2: style 属性を出さない（WithClasses(true) の確認）。
			// **この 1 行が崩れると FR-070（テーマ切り替え）が成立しなくなる。**
			// 個別の期待値ではなく全ケース共通の検査にする。
			if strings.Contains(got, "style=") {
				t.Errorf("出力に style 属性が含まれている。配色は CSS 側の責務（DSP-013）\n出力: %s", got)
			}
		})
	}
}

// TestRender_Highlight_MD031Languages は MD-031 が列挙する言語がすべて
// ハイライトされることを検証する（根拠: MD-031 / IMP-114, AR-033）。
//
// 登録言語を限定した場合（AR-033）に、一覧から漏れた言語をそのまま
// 見落とさないための一覧である。ハイライトの正しさ自体は chroma の責務
// であり検証しない（UT-034）。ここで見るのは「解決できるか」だけ。
func TestRender_Highlight_MD031Languages(t *testing.T) {
	languages := []string{
		"go", "c", "cpp", "csharp", "java", "kotlin", "swift", "rust",
		"python", "ruby", "php", "javascript", "typescript", "jsx", "tsx",
		"html", "css", "scss", "sql", "shell", "powershell", "batch",
		"dockerfile", "makefile", "yaml", "toml", "json", "xml", "ini",
		"diff", "markdown", "lua", "perl", "r", "scala", "haskell",
		"elixir", "zig", "hcl", "protobuf", "graphql", "vim", "awk",
	}

	// MD-031 が明示するエイリアス。
	aliases := []string{"sh", "bash", "zsh", "py", "yml", "ps1"}

	for _, lang := range append(languages, aliases...) {
		t.Run(lang, func(t *testing.T) {
			got := render(t, "```"+lang+"\nx\n```")

			if !strings.Contains(got, `<pre class="chroma">`) {
				t.Errorf("%q がハイライトされていない（lexer を解決できない）\n出力: %s", lang, got)
			}
			if !strings.Contains(got, `data-lang="`+lang+`"`) {
				t.Errorf("data-lang が %q になっていない\n出力: %s", lang, got)
			}
		})
	}
}
