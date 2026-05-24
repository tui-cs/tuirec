// Package cellwidth provides Unicode-aware terminal cell width calculation.
//
// It determines how many terminal columns a rune occupies (0, 1, or 2) using
// Unicode East Asian Width properties and emoji detection. This is used by
// pkg/castfix to track cursor position accurately regardless of the renderer's
// internal wcwidth implementation.
package cellwidth

import "unicode"

// RuneWidth returns the number of terminal columns a rune occupies: 0, 1, or 2.
//
// Zero-width: combining marks, zero-width space/joiner/non-joiner, variation
// selectors, default-ignorable code points.
//
// Double-width: East Asian Wide/Fullwidth characters, emoji presentation
// sequences (most emoji U+1F300+).
//
// Everything else is single-width.
func RuneWidth(r rune) int {
	if isZeroWidth(r) {
		return 0
	}
	if isWide(r) {
		return 2
	}
	return 1
}

// StringWidth returns the total terminal column width of a string.
func StringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += RuneWidth(r)
	}
	return w
}

// isZeroWidth reports whether r is a zero-width character in terminal display.
func isZeroWidth(r rune) bool {
	// Common zero-width characters.
	switch r {
	case 0x200B: // ZERO WIDTH SPACE
		return true
	case 0x200C: // ZERO WIDTH NON-JOINER
		return true
	case 0x200D: // ZERO WIDTH JOINER
		return true
	case 0xFEFF: // ZERO WIDTH NO-BREAK SPACE (BOM)
		return true
	}

	// Unicode combining marks (Mn, Mc, Me categories).
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return true
	}

	// Variation selectors.
	if r >= 0xFE00 && r <= 0xFE0F {
		return true
	}
	if r >= 0xE0100 && r <= 0xE01EF {
		return true
	}

	// Soft hyphen.
	if r == 0x00AD {
		return true
	}

	// Control characters (C0/C1) are zero-width except tab/newline/CR which
	// are handled at a higher level.
	if r < 0x20 || (r >= 0x7F && r < 0xA0) {
		return true
	}

	return false
}

// isWide reports whether r occupies two terminal columns.
func isWide(r rune) bool {
	// CJK Unified Ideographs and extensions.
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	if r >= 0x20000 && r <= 0x2A6DF {
		return true
	}
	if r >= 0x2A700 && r <= 0x2CEAF {
		return true
	}
	if r >= 0x2CEB0 && r <= 0x2EBEF {
		return true
	}
	if r >= 0x30000 && r <= 0x3134F {
		return true
	}
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	if r >= 0x2F800 && r <= 0x2FA1F {
		return true
	}

	// CJK Compatibility Ideographs Supplement, Kangxi Radicals, etc.
	if r >= 0x2E80 && r <= 0x2EFF {
		return true
	}
	if r >= 0x2F00 && r <= 0x2FDF {
		return true
	}
	if r >= 0x2FF0 && r <= 0x2FFF {
		return true
	}
	if r >= 0x3000 && r <= 0x303E {
		return true
	}
	if r >= 0x3041 && r <= 0x3096 {
		return true
	}
	if r >= 0x3099 && r <= 0x30FF {
		return true
	}
	if r >= 0x3105 && r <= 0x312F {
		return true
	}
	if r >= 0x3131 && r <= 0x318E {
		return true
	}
	if r >= 0x3190 && r <= 0x31BF {
		return true
	}
	if r >= 0x31F0 && r <= 0x321E {
		return true
	}
	if r >= 0x3220 && r <= 0x3247 {
		return true
	}
	if r >= 0x3250 && r <= 0x32FE {
		return true
	}
	if r >= 0x3300 && r <= 0x4DBF {
		return true
	}
	if r >= 0xA000 && r <= 0xA48C {
		return true
	}
	if r >= 0xA490 && r <= 0xA4C6 {
		return true
	}

	// Hangul Syllables.
	if r >= 0xAC00 && r <= 0xD7A3 {
		return true
	}

	// Fullwidth Forms.
	if r >= 0xFF01 && r <= 0xFF60 {
		return true
	}
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return true
	}

	// Emoji that are typically rendered as wide.
	// Miscellaneous Symbols and Pictographs, Emoticons, Transport/Map,
	// Supplemental Symbols, Symbols Extended-A, Playing Cards, etc.
	if r >= 0x1F300 && r <= 0x1F9FF {
		return true
	}
	if r >= 0x1FA00 && r <= 0x1FA6F {
		return true
	}
	if r >= 0x1FA70 && r <= 0x1FAFF {
		return true
	}
	if r >= 0x1F600 && r <= 0x1F64F {
		return true
	}
	if r >= 0x1F680 && r <= 0x1F6FF {
		return true
	}
	if r >= 0x1F1E0 && r <= 0x1F1FF {
		return true
	}

	// Dingbats that are commonly rendered wide.
	if r >= 0x2600 && r <= 0x27BF {
		return true
	}

	// Mahjong Tiles, Domino Tiles.
	if r >= 0x1F000 && r <= 0x1F02F {
		return true
	}
	if r >= 0x1F030 && r <= 0x1F09F {
		return true
	}

	return false
}
