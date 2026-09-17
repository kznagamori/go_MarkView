package document

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/kznagamori/go_MarkView/internal/renderer"
)

// testKey は目印の鍵の固定値（UT-108 ほか）。
const testKey = "0123456789abcdef"

// splice は p を raw に当てた結果を、Patch.Apply を使わずに組み立てる。
//
// 書き換え位置のテスト（UT-107, UT-109, UT-110）が Patch.Apply（UT-111）の
// 正しさに寄りかからないよう、テストの側で素直に切り貼りする。
func splice(t *testing.T, raw []byte, p Patch) []byte {
	t.Helper()

	end := p.Offset + len(p.Old)
	if p.Offset < 0 || end > len(raw) {
		t.Fatalf("Patch が範囲外: Offset=%d len(Old)=%d len(raw)=%d", p.Offset, len(p.Old), len(raw))
	}
	if !bytes.Equal(raw[p.Offset:end], p.Old) {
		t.Fatalf("Patch の Old %q が raw[%d:%d] = %q と一致しない", p.Old, p.Offset, end, raw[p.Offset:end])
	}

	out := make([]byte, 0, len(raw)-len(p.Old)+len(p.New))
	out = append(out, raw[:p.Offset]...)
	out = append(out, p.New...)
	return append(out, raw[end:]...)
}

// TestPlanTask_RawOffsets は、生バイト列と正規化後の位置の対応を検証する
// （UT-107。根拠: FR-141, FR-142, FR-143, FR-021 / IMP-106）。
//
// **位置の対応は、LF だけの文書では壊れていても見えない。** 改行コードと BOM の
// 組み合わせを先に置く（UT-013）。どれも最後のタスク（2 番）をオンにする。
func TestPlanTask_RawOffsets(t *testing.T) {
	r := renderer.New()
	last := Ref{Kind: RefTask, Index: 2}

	tests := []struct {
		name   string
		raw    string
		offset int
	}{
		{"1: LF", "- [ ] a\n- [ ] b\n- [ ] c\n", 19},
		// ケース 2: 前の 2 行の CR の分だけ後ろ
		{"2: CRLF", "- [ ] a\r\n- [ ] b\r\n- [ ] c\r\n", 21},
		{"3: CR だけ", "- [ ] a\r- [ ] b\r- [ ] c\r", 19},
		// ケース 4: BOM の 3 バイトの分だけ後ろ
		{"4: 先頭に BOM + LF", "\xEF\xBB\xBF- [ ] a\n- [ ] b\n- [ ] c\n", 22},
		{"5: 先頭に BOM + CRLF", "\xEF\xBB\xBF- [ ] a\r\n- [ ] b\r\n- [ ] c\r\n", 24},
		// ケース 6: 除いた CR は 1 つ
		{"6: 1 行目だけ CRLF、残りは LF", "- [ ] a\r\n- [ ] b\n- [ ] c\n", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, changed, err := PlanTask(r, []byte(tt.raw), last, true)
			if err != nil {
				t.Fatalf("PlanTask がエラーを返した: %v", err)
			}
			if !changed {
				t.Error("changed が偽")
			}
			if p.Offset != tt.offset {
				t.Errorf("Offset = %d, want %d", p.Offset, tt.offset)
			}
			if string(p.Old) != " " || string(p.New) != "x" {
				t.Errorf("Old = %q, New = %q, want %q と %q", p.Old, p.New, " ", "x")
			}
		})
	}

	// ケース 8: 適用した結果は、括弧の中の 1 バイト以外は入力と 1 バイトも違わない
	// （BOM・CR・末尾の改行を含む）。
	for _, tt := range tests {
		t.Run("8: 適用の結果が 1 バイトだけ違う（"+tt.name+"）", func(t *testing.T) {
			raw := []byte(tt.raw)
			p, _, err := PlanTask(r, raw, last, true)
			if err != nil {
				t.Fatalf("PlanTask がエラーを返した: %v", err)
			}

			out := splice(t, raw, p)
			if len(out) != len(raw) {
				t.Fatalf("長さ = %d, want %d", len(out), len(raw))
			}
			for i := range raw {
				switch {
				case i == tt.offset && out[i] != 'x':
					t.Errorf("out[%d] = %q, want 'x'", i, out[i])
				case i != tt.offset && out[i] != raw[i]:
					t.Errorf("out[%d] = %q, want %q（括弧の中以外を書き換えた）", i, out[i], raw[i])
				}
			}
		})
	}

	// ケース 7: 置き換えが長さを変え、位置を対応させられない
	t.Run("7: 不正な UTF-8 を含む文書", func(t *testing.T) {
		raw := []byte("- [ ] a\xFF\n- [ ] b\n- [ ] c\n")
		if _, _, err := PlanTask(r, raw, last, true); !errors.Is(err, ErrNotEditable) {
			t.Errorf("エラー = %v, want ErrNotEditable", err)
		}
	})

	// ケース 7 の境界（T14-3 で足した。UT-090）: 不正なバイトが 3 つ並ぶと、
	// bytes.ToValidUTF8 はまとめて 1 つの U+FFFD（3 バイト）に置き換え、長さが変わらない。
	// **長さの照合だけで拒む実装はここを通してしまう**——置き換えの有無そのものを見ること。
	t.Run("7: 置き換えで長さが変わらない不正な並び（3 バイト）", func(t *testing.T) {
		raw := []byte("- [ ] a\xFF\xFE\xFD\n- [ ] b\n- [ ] c\n")
		if _, _, err := PlanTask(r, raw, last, true); !errors.Is(err, ErrNotEditable) {
			t.Errorf("エラー = %v, want ErrNotEditable", err)
		}
	})

	// ケース 9: BOM 3 と CR 3 つの分だけ後ろ。Front Matter の位置の補正と組み合わせる
	t.Run("9: BOM + CRLF の TOML の Front Matter の後のタスク", func(t *testing.T) {
		raw := []byte("\xEF\xBB\xBF+++\r\na = 1\r\n+++\r\n- [ ] a\r\n")
		p, _, err := PlanTask(r, raw, Ref{Kind: RefTask, Index: 0}, true)
		if err != nil {
			t.Fatalf("PlanTask がエラーを返した: %v", err)
		}
		if p.Offset != 23 {
			t.Errorf("Offset = %d, want 23", p.Offset)
		}
	})

	// ケース 10・11: セルの書き換えも同じ対応表を通る
	cellRaw := []byte("\xEF\xBB\xBF| a | b |\r\n| --- | --- |\r\n| 1 | 2 |\r\n")
	cell := Ref{Kind: RefCell, Index: 0, Row: 1, Col: 0}

	t.Run("10: BOM + CRLF の表の PlanCell", func(t *testing.T) {
		p, _, err := PlanCell(r, cellRaw, cell, "9")
		if err != nil {
			t.Fatalf("PlanCell がエラーを返した: %v", err)
		}
		if p.Offset != 31 {
			t.Errorf("Offset = %d, want 31", p.Offset)
		}
		if string(p.Old) != "1" || string(p.New) != "9" {
			t.Errorf("Old = %q, New = %q, want %q と %q", p.Old, p.New, "1", "9")
		}
	})

	t.Run("11: BOM + CRLF の表の CellSource", func(t *testing.T) {
		got, err := CellSource(r, cellRaw, cell)
		if err != nil {
			t.Fatalf("CellSource がエラーを返した: %v", err)
		}
		if got != "1" {
			t.Errorf("CellSource = %q, want %q（CR を含まない）", got, "1")
		}
	})
}

// TestParseRef は目印の値の解読を検証する
// （UT-108。根拠: FR-141, FR-142, NFR-030 / IMP-106, IMP-120）。
func TestParseRef(t *testing.T) {
	t.Run("1: タスク", func(t *testing.T) {
		got, err := ParseRef(testKey + ":task:3")
		if err != nil {
			t.Fatalf("ParseRef がエラーを返した: %v", err)
		}
		want := Ref{Key: testKey, Kind: RefTask, Index: 3}
		if got != want {
			t.Errorf("ParseRef = %+v, want %+v", got, want)
		}
	})

	t.Run("2: セル", func(t *testing.T) {
		got, err := ParseRef(testKey + ":cell:1:0:2")
		if err != nil {
			t.Fatalf("ParseRef がエラーを返した: %v", err)
		}
		want := Ref{Key: testKey, Kind: RefCell, Index: 1, Row: 0, Col: 2}
		if got != want {
			t.Errorf("ParseRef = %+v, want %+v", got, want)
		}
	})

	bad := []struct {
		name string
		in   string
	}{
		// ケース 3: 表の目印は書き込みの対象ではない
		{"3: 表の目印", testKey + ":table:1"},
		// 図のブロックの目印も書き込みの対象ではない（IMP-106 の ParseRef の定義）
		{"3: Mermaid の目印", testKey + ":mermaid:0"},
		{"3: PlantUML の目印", testKey + ":plantuml:0"},
		{"4: 鍵が 15 文字", "0123456789abcde:task:3"},
		{"4: 鍵が 17 文字", "0123456789abcdef0:task:3"},
		{"4: 鍵が大文字の 16 進", "0123456789ABCDEF:task:3"},
		// ケース 5: strconv.Atoi は +3 を受け付けるが、IMP-116 の正規表現は受け付けない
		{"5: 負の数", testKey + ":task:-1"},
		{"5: 符号付きの数", testKey + ":task:+3"},
		{"6: 区切りが多い", testKey + ":task:3:9"},
		{"7: 数が足りない", testKey + ":cell:1:0"},
		{"8: 空文字", ""},
		// ケース 9: int に収まらない。panic しない
		{"9: int に収まらない", testKey + ":task:99999999999999999999"},
	}

	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseRef(tt.in); !errors.Is(err, ErrBadRef) {
				t.Errorf("ParseRef(%q) のエラー = %v, want ErrBadRef", tt.in, err)
			}
		})
	}
}

// TestPatch は Patch の適用と逆を検証する
// （UT-111。根拠: FR-143, FR-144 / IMP-106）。
func TestPatch(t *testing.T) {
	// 範囲の外と一致しない場合を先に置く（UT-013）。
	failing := []struct {
		name string
		raw  string
		p    Patch
	}{
		{"2: Old が一致しない", "abc123def", Patch{Offset: 3, Old: []byte("999"), New: []byte("X")}},
		{"3: Offset + len(Old) が長さを超える", "abc", Patch{Offset: 2, Old: []byte("cd"), New: []byte("X")}},
		{"4: Offset が負", "abc", Patch{Offset: -1, Old: []byte("a"), New: []byte("X")}},
		{"9: Old が空で Offset が長さを超える", "abc", Patch{Offset: 4, Old: nil, New: []byte("X")}},
	}

	for _, tt := range failing {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.p.Apply([]byte(tt.raw))
			if !errors.Is(err, ErrChanged) {
				t.Errorf("エラー = %v, want ErrChanged", err)
			}
			if got != nil {
				t.Errorf("書き換えたものを返した: %q", got)
			}
		})
	}

	succeeding := []struct {
		name string
		raw  string
		p    Patch
		want string
	}{
		{"1: Offset と Old が一致する", "abc123def", Patch{Offset: 3, Old: []byte("123"), New: []byte("X")}, "abcXdef"},
		{"5: 長さの違う置き換え", "a1b", Patch{Offset: 1, Old: []byte("1"), New: []byte("123")}, "a123b"},
		{"8: Old が空で Offset が長さと等しい（末尾への挿入）", "abc", Patch{Offset: 3, Old: nil, New: []byte("X")}, "abcX"},
	}

	for _, tt := range succeeding {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.p.Apply([]byte(tt.raw))
			if err != nil {
				t.Fatalf("Apply がエラーを返した: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Apply = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("6: 適用した結果に Inverse を適用すると元に戻る", func(t *testing.T) {
		const original = "abc123def"
		p := Patch{Offset: 3, Old: []byte("123"), New: []byte("X")}

		inv := p.Inverse()
		wantInv := Patch{Offset: 3, Old: []byte("X"), New: []byte("123")}
		if fmt.Sprintf("%+v", inv) != fmt.Sprintf("%+v", wantInv) {
			t.Errorf("Inverse = %+v, want %+v", inv, wantInv)
		}

		applied, err := p.Apply([]byte(original))
		if err != nil {
			t.Fatalf("Apply がエラーを返した: %v", err)
		}
		back, err := inv.Apply(applied)
		if err != nil {
			t.Fatalf("Inverse の Apply がエラーを返した: %v", err)
		}
		if string(back) != original {
			t.Errorf("元に戻らない: %q, want %q", back, original)
		}
	})

	t.Run("7: 渡したバイト列の内容を変えない", func(t *testing.T) {
		raw := []byte("abc123def")
		p := Patch{Offset: 3, Old: []byte("123"), New: []byte("XYZ")}

		if _, err := p.Apply(raw); err != nil {
			t.Fatalf("Apply がエラーを返した: %v", err)
		}
		if string(raw) != "abc123def" {
			t.Errorf("渡したバイト列が変わった: %q", raw)
		}
	})
}
