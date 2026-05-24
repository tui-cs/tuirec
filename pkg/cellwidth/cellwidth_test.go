package cellwidth

import "testing"

func TestRuneWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		r    rune
		want int
	}{
		// ASCII
		{"ascii A", 'A', 1},
		{"ascii space", ' ', 1},
		{"ascii tilde", '~', 1},

		// CJK Ideographs
		{"cjk ideograph", '中', 2},
		{"cjk ideograph 2", '日', 2},
		{"cjk ideograph 3", '語', 2},

		// Hiragana/Katakana
		{"hiragana", 'あ', 2},
		{"katakana", 'ア', 2},

		// Hangul
		{"hangul", '한', 2},

		// Fullwidth forms
		{"fullwidth A", 'Ａ', 2},
		{"fullwidth excl", '！', 2},

		// Emoji (wide)
		{"emoji grinning face", '😀', 2},
		{"emoji globe", '🌍', 2},
		{"emoji rocket", '🚀', 2},
		{"emoji heart", '❤', 2},
		{"emoji cyclone", '🌀', 2},
		{"emoji 1F300 start", '\U0001F300', 2},
		{"emoji 1F9FF end", '\U0001F9FF', 2},

		// Zero-width
		{"combining acute", '\u0301', 0},
		{"combining tilde", '\u0303', 0},
		{"zwsp", '\u200B', 0},
		{"zwj", '\u200D', 0},
		{"zwnj", '\u200C', 0},
		{"variation selector 15", '\uFE0E', 0},
		{"variation selector 16", '\uFE0F', 0},
		{"soft hyphen", '\u00AD', 0},
		{"BOM", '\uFEFF', 0},

		// Control characters
		{"null", 0x00, 0},
		{"bell", 0x07, 0},
		{"escape", 0x1B, 0},
		{"DEL", 0x7F, 0},

		// Regular non-ASCII single-width
		{"latin accent e", 'é', 1},
		{"greek alpha", 'α', 1},
		{"cyrillic a", 'а', 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RuneWidth(tt.r)
			if got != tt.want {
				t.Errorf("RuneWidth(%U %q) = %d, want %d", tt.r, tt.r, got, tt.want)
			}
		})
	}
}

func TestStringWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"cjk", "中文", 4},
		{"mixed", "A中B", 4},
		{"emoji row", "🌀🌁🌂🌃", 8},
		{"emoji plus ascii", "Hi🚀Go", 6},
		{"combining", "e\u0301", 1}, // é as e + combining acute
		{"hangul word", "한글", 4},
		{"fullwidth ascii", "ＡＢＣ", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := StringWidth(tt.s)
			if got != tt.want {
				t.Errorf("StringWidth(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

// TestStringWidth_EmojiGridRow verifies the exact scenario from issue #59:
// a CharMap emoji row with 16 emoji (2 cols each = 32) plus ASCII filling
// to exactly 120 columns.
func TestStringWidth_EmojiGridRow(t *testing.T) {
	t.Parallel()

	// Simulate: 16 emoji at 2 cols each = 32 cols, then 88 ASCII chars = 120 total.
	emoji16 := "🌀🌁🌂🌃🌄🌅🌆🌇🌈🌉🌊🌋🌌🌍🌎🌏"
	ascii88 := "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567"

	row := emoji16 + ascii88
	got := StringWidth(row)
	want := 120

	if got != want {
		t.Errorf("StringWidth(emoji16+ascii88) = %d, want %d", got, want)
	}
}
