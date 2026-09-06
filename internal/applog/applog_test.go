package applog

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// TestEnabled と TestNewLogger は、ログの出力先の切り替えを検証する
// （UT-806。根拠: NFR-041 / IMP-023）。
//
// **主眼は「有効になること」ではなく「有効にならないこと」である。**
// "1" 以外を有効と解釈すると、意図しない値で配布物が出力を始め、
// NFR-041 が静かに破れる。境界値を先に書く（UT-013）。
//
// プロセスの標準エラーを奪い合わないよう、出力先を渡せる newLogger で
// 検証する（UT-806 の注記）。
func TestEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool // false は環境変数を設定しないことを示す
		want  bool
	}{
		{"未設定", "", false, false},
		{"1", "1", true, true},
		{"0", "0", true, false},
		{"true", "true", true, false},
		{"空文字", "", true, false},
		{"前に空白", " 1", true, false},
		{"後ろに空白", "1 ", true, false},
		{"改行付き", "1\n", true, false},
		{"大文字の別表記", "ON", true, false},
		{"数値だが別の値", "11", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv は t.Cleanup で元へ戻す（UT-035）。
			// 未設定を作るには空文字ではなく Unsetenv が要る。
			if tt.set {
				t.Setenv(envDebug, tt.value)
			} else {
				// t.Setenv で復元を予約してから消す。t.Setenv 自体は
				// 「未設定」を作れないため、この 2 段が要る。
				t.Setenv(envDebug, "placeholder")

				if err := os.Unsetenv(envDebug); err != nil {
					t.Fatalf("環境変数を消せない: %v", err)
				}
			}

			if got := Enabled(); got != tt.want {
				t.Errorf("Enabled() = %t, want %t（値 %q）", got, tt.want, tt.value)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	t.Run("無効なら何も書かれない", func(t *testing.T) {
		t.Setenv(envDebug, "0")

		var buf bytes.Buffer
		newLogger(&buf).Error("should not appear")

		if buf.Len() != 0 {
			t.Errorf("無効なのに %d バイト書かれた: %s", buf.Len(), buf.String())
		}
	})

	t.Run("有効なら書かれる", func(t *testing.T) {
		t.Setenv(envDebug, "1")

		var buf bytes.Buffer
		newLogger(&buf).Error("hello", "key", "value")

		got := buf.String()
		if !strings.Contains(got, "hello") || !strings.Contains(got, "value") {
			t.Errorf("出力に内容が含まれない: %q", got)
		}
	})

	t.Run("New は出力先を固定して返す", func(t *testing.T) {
		t.Setenv(envDebug, "0")

		// 標準エラーへ書くため内容は見ない。**落ちないこと**だけを見る。
		if New() == nil {
			t.Error("New() が nil を返した")
		}
	})
}

// TestRecovered は recover の記録を検証する
// （UT-807。根拠: FR-111 / IMP-022, IMP-023）。
//
// **ここが落ちると FR-111 の保証そのものが壊れる。** Recovered は異常時にだけ
// 通る経路であり、引数がどうであっても落ちてはならない。
func TestRecovered(t *testing.T) {
	t.Run("無効なら何も出力しない", func(t *testing.T) {
		t.Setenv(envDebug, "0")

		var buf bytes.Buffer
		recovered(&buf, "renderer.Render", "boom")

		if buf.Len() != 0 {
			t.Errorf("無効なのに %d バイト書かれた: %s", buf.Len(), buf.String())
		}
	})

	t.Run("有効なら発生箇所と値とスタックが出る", func(t *testing.T) {
		t.Setenv(envDebug, "1")

		var buf bytes.Buffer
		recovered(&buf, "renderer.Render", "boom")

		got := buf.String()
		for _, want := range []string{"renderer.Render", "boom", "goroutine"} {
			if !strings.Contains(got, want) {
				t.Errorf("出力に %q が含まれない: %q", want, got)
			}
		}
	})

	t.Run("値が nil でも落ちない", func(t *testing.T) {
		t.Setenv(envDebug, "1")

		var buf bytes.Buffer
		recovered(&buf, "", nil) // where も空にする

		if buf.Len() == 0 {
			t.Error("nil でも記録は残ってほしい")
		}
	})

	t.Run("実際に recover した経路で呼べる", func(t *testing.T) {
		t.Setenv(envDebug, "1")

		// **Recovered は標準エラーへ書く。** 内容は見ず、deferred の中から
		// 呼んで落ちないことだけを確かめる（FR-111）。
		func() {
			defer func() {
				if v := recover(); v != nil {
					Recovered("applog_test", v)
				}
			}()

			panic("intentional")
		}()
	})
}
