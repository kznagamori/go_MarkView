package mdfile

import "testing"

// TestIsMarkdown は拡張子判定を検証する（UT-105。根拠: FR-010, FR-031, FR-013 / IMP-105）。
func TestIsMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		// UT-105 ケース 1〜3: 対象とする拡張子
		{"README.md は対象", "README.md", true},
		{"大文字小文字を区別しない", "README.MD", true},
		{".markdown は対象", "a.markdown", true},
		{".mdown は対象", "a.mdown", true},
		{".mkd は対象", "a.mkd", true},

		// UT-105 ケース 4: 対象外の拡張子
		{".txt は対象外", "a.txt", false},
		{".png は対象外", "a.png", false},
		{".go は対象外", "a.go", false},

		// UT-105 ケース 5〜8: 境界値
		{"拡張子がなく名前が md", "md", false},
		{"名前が拡張子のみ", ".md", false},
		{"二重拡張子は末尾で判定する", "a.md.txt", false},
		{"空文字列", "", false},

		// UT-090 に従って追加した境界値。
		// 判定は最終要素の拡張子のみを見るため、ディレクトリを含んでも変わらない。
		{"ディレクトリを含むパス", "docs/specs/README.md", true},
		{"ディレクトリ名にドットを含む", "a.b/README.md", true},
		{"ドットで始まる名前でも拡張子があれば対象", ".README.md", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMarkdown(tt.in); got != tt.want {
				t.Errorf("IsMarkdown(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
