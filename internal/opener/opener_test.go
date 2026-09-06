package opener

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// call は runCommand が受け取った引数を記録したもの。
type call struct {
	name string
	args []string
}

// record は runCommand を記録用に差し替える（UT-702）。
//
// **実際に外部プロセスを起動しない。** ブラウザや画像ビューアが立ち上がる
// テストは実行環境を汚し、CI でも成立しない（UT-035）。組み立てた引数までを
// 検証する。t.Cleanup で必ず元へ戻す（UT-017）。
func record(t *testing.T) *[]call {
	t.Helper()

	var calls []call
	original := runCommand

	runCommand = func(name string, args ...string) error {
		calls = append(calls, call{name: name, args: args})
		return nil
	}
	t.Cleanup(func() { runCommand = original })

	return &calls
}

// TestOpenURL_Schemes は URL スキームの検査を検証する
// （UT-701。根拠: NFR-030 / IMP-170）。
func TestOpenURL_Schemes(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		accept bool
	}{
		// UT-701 ケース 4〜7: 拒否するもの（境界値を先に。UT-013）
		{"javascript", "javascript:alert(1)", false},
		{"大文字混じりの JavaScript", "JavaScript:alert(1)", false},
		{"file", "file:///etc/passwd", false},
		{"data", "data:text/html,<script>alert(1)</script>", false},
		{"vbscript", "vbscript:msgbox(1)", false},
		{"空文字", "", false},
		{"スキームがない", "example.com/a", false},
		{"相対パス", "./other.md", false},
		{"ftp", "ftp://example.com/a", false},

		// UT-701 ケース 1〜3: 受け付けるもの
		{"https", "https://example.com", true},
		{"http", "http://example.com", true},
		{"mailto", "mailto:a@example.com", true},
		{"大文字の HTTPS", "HTTPS://example.com", true},
		{"クエリとフラグメント付き", "https://example.com/a?b=1#c", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := record(t)

			err := OpenURL(tt.url)

			if tt.accept {
				if err != nil {
					t.Fatalf("OpenURL(%q) がエラーを返した: %v", tt.url, err)
				}
				if len(*calls) != 1 {
					t.Fatalf("起動回数 = %d, want 1", len(*calls))
				}
				return
			}

			if !errors.Is(err, ErrUnsupportedScheme) {
				t.Errorf("OpenURL(%q) = %v, want ErrUnsupportedScheme", tt.url, err)
			}
			// **拒否したものは起動しない。** エラーを返すだけでは足りない。
			if len(*calls) != 0 {
				t.Errorf("拒否したのに起動した: %+v", *calls)
			}
		})
	}
}

// TestOpenURL_Command はコマンドの組み立てを検証する
// （UT-702。根拠: IMP-170）。
func TestOpenURL_Command(t *testing.T) {
	calls := record(t)

	const target = "https://example.com/a b?c=1"
	if err := OpenURL(target); err != nil {
		t.Fatalf("OpenURL がエラーを返した: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("起動回数 = %d, want 1", len(*calls))
	}

	got := (*calls)[0]
	checkCommand(t, got, target)
}

// TestOpenFile_Command はファイルを開くコマンドの組み立てを検証する
// （UT-702。根拠: FR-053 / IMP-170）。
//
// ケース 3・4 は、文字列連結でコマンドを組み立てていないこと（IMP-170）の
// 確認である。**シェルを経由していれば、これらの引数は分割されるか実行される。**
func TestOpenFile_Command(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{"普通のファイル名", "a.png"},
		{"空白を含む", "my image.png"},
		{"セミコロンとコマンドに見える文字列", "a; rm -rf x.png"},
		{"アンパサンド", "a & b.png"},
		{"パイプ", "a | b.png"},
		{"引用符", "a'b\"c.png"},
		{"バッククォートとドル", "a$(x).png"},
		{"日本語", "画像.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, tt.file)
			if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
				t.Skipf("この名前のファイルを作れない環境: %v", err)
			}

			calls := record(t)

			if err := OpenFile(p); err != nil {
				t.Fatalf("OpenFile がエラーを返した: %v", err)
			}
			if len(*calls) != 1 {
				t.Fatalf("起動回数 = %d, want 1", len(*calls))
			}

			checkCommand(t, (*calls)[0], p)
		})
	}
}

// checkCommand は組み立てたコマンドが OS ごとの形になっているかを見る。
//
// **対象は必ず 1 つの引数として渡る。** 分割されていれば、空白を含むパスが
// 別々の引数になり、意図しない対象を開くことになる。
func checkCommand(t *testing.T, got call, target string) {
	t.Helper()

	var wantName string
	var wantArgs []string

	if runtime.GOOS == "windows" {
		// UT-702 ケース 2
		wantName = "rundll32.exe"
		wantArgs = []string{"url.dll,FileProtocolHandler", target}
	} else {
		// UT-702 ケース 1
		wantName = "xdg-open"
		wantArgs = []string{target}
	}

	if got.name != wantName {
		t.Errorf("コマンド = %q, want %q", got.name, wantName)
	}
	if len(got.args) != len(wantArgs) {
		t.Fatalf("引数 = %q, want %q（分割されている）", got.args, wantArgs)
	}
	for i := range wantArgs {
		if got.args[i] != wantArgs[i] {
			t.Errorf("引数 %d = %q, want %q", i, got.args[i], wantArgs[i])
		}
	}

	// シェルを経由していないこと。コマンド名にシェルが現れてはならない。
	for _, shell := range []string{"cmd", "cmd.exe", "sh", "bash", "powershell"} {
		if strings.EqualFold(got.name, shell) {
			t.Errorf("シェルを経由している: %q", got.name)
		}
	}
}

// TestOpenFile_NotFound は存在しないファイルの扱いを検証する（FR-053, FR-110）。
//
// 起動コマンドは対象がなくても起動自体に成功するため、ここで弾かないと
// 利用者に何も伝わらない。
func TestOpenFile_NotFound(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"存在しないファイル", filepath.Join(t.TempDir(), "nosuch.png")},
		{"空文字", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := record(t)

			if err := OpenFile(tt.path); !errors.Is(err, ErrNotFound) {
				t.Errorf("OpenFile(%q) = %v, want ErrNotFound", tt.path, err)
			}
			if len(*calls) != 0 {
				t.Errorf("存在しないのに起動した: %+v", *calls)
			}
		})
	}
}

// TestOpenFile_Absolute は相対パスを絶対パスにして渡すことを検証する（IMP-025）。
func TestOpenFile_Absolute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	calls := record(t)

	if err := OpenFile("a.png"); err != nil {
		t.Fatalf("OpenFile がエラーを返した: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("起動回数 = %d, want 1", len(*calls))
	}

	got := (*calls)[0].args[len((*calls)[0].args)-1]
	if !filepath.IsAbs(got) {
		t.Errorf("渡した対象 = %q, 絶対パスでない", got)
	}
	if filepath.Base(got) != "a.png" {
		t.Errorf("渡した対象 = %q, want 末尾が a.png", got)
	}
}

// TestOpenURL_StartError は起動の失敗が呼び出し側へ返ることを検証する
// （FR-110）。
func TestOpenURL_StartError(t *testing.T) {
	original := runCommand
	wantErr := errors.New("boom")
	runCommand = func(string, ...string) error { return wantErr }
	t.Cleanup(func() { runCommand = original })

	if err := OpenURL("https://example.com"); !errors.Is(err, wantErr) {
		t.Errorf("OpenURL = %v, want %v を含む", err, wantErr)
	}
}
