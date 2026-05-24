package renderer_test

import (
	"bytes"
	"fmt"
	"image"
	stdgif "image/gif"
	"os"
	"strings"
	"testing"

	"github.com/gui-cs/tuirec/pkg/renderer"
)

// TestRenderEmojiGIF is the end-to-end proof that the x/vt-based renderer
// produces a GIF without the tearing exhibited by agg (#59).
//
// It creates a synthetic cast file with emoji rows that rely on auto-wrap,
// renders it to a GIF, then verifies that emoji are correctly positioned
// in the output image (no column drift).
func TestRenderEmojiGIF(t *testing.T) {
	const cols = 120
	const rows = 10

	// Build a cast file in memory simulating Terminal.Gui's CharMap emoji grid.
	// Each row: 16 emoji (32 cols) + 88 ASCII dots = 120 cols, auto-wrapping.
	emoji := []rune{
		'🌀', '🌁', '🌂', '🌃', '🌄', '🌅', '🌆', '🌇',
		'🌈', '🌉', '🌊', '🌋', '🌌', '🌍', '🌎', '🌏',
	}

	var rowContent strings.Builder
	for _, e := range emoji {
		rowContent.WriteRune(e)
	}
	rowContent.WriteString(strings.Repeat(".", 88))
	rowStr := rowContent.String()

	// 5 rows of emoji+ASCII, no cursor positioning (pure auto-wrap)
	fullContent := strings.Repeat(rowStr, 5)

	// Build asciinema v2 cast
	castHeader := fmt.Sprintf(`{"version":2,"width":%d,"height":%d,"timestamp":1700000000,"title":"emoji-test","env":{"TERM":"xterm-256color"}}`, cols, rows)
	// Escape the content for JSON
	escaped := jsonEscape(fullContent)
	castEvent := fmt.Sprintf(`[0.5,"o","%s"]`, escaped)
	castFile := castHeader + "\n" + castEvent + "\n"

	// Render to GIF using explicit cell size for deterministic assertions
	const cellW = 8
	const cellH = 17
	var gifBuf bytes.Buffer
	err := renderer.RenderGIF(
		strings.NewReader(castFile),
		&gifBuf,
		renderer.RenderConfig{
			CellWidth:  cellW,
			CellHeight: cellH,
			MaxFrames:  10,
		},
	)
	if err != nil {
		t.Fatalf("RenderGIF failed: %v", err)
	}

	// Decode and validate
	decoded, err := stdgif.DecodeAll(bytes.NewReader(gifBuf.Bytes()))
	if err != nil {
		t.Fatalf("decode GIF: %v", err)
	}

	if len(decoded.Image) < 2 {
		t.Fatalf("GIF has %d frames, want at least 2", len(decoded.Image))
	}

	// Check dimensions
	bounds := decoded.Image[0].Bounds()
	wantW := cols * cellW
	wantH := rows * cellH
	if bounds.Dx() != wantW || bounds.Dy() != wantH {
		t.Errorf("GIF dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), wantW, wantH)
	}

	// Verify NO tearing: check that row boundaries are consistent.
	// In a correct render, row 0's emoji glyphs should produce non-background
	// pixels in the first 32*cellW pixel columns of the first cellH pixel rows.
	lastFrame := decoded.Image[len(decoded.Image)-1]
	emojiPixelW := 32 * cellW
	if !hasContentInRegion(lastFrame, 0, 0, emojiPixelW, cellH) {
		t.Error("row 0 emoji region has no content — possible blank render")
	}
	if !hasContentInRegion(lastFrame, 0, cellH, emojiPixelW, 2*cellH) {
		t.Error("row 1 emoji region has no content — possible tearing/drift")
	}

	// The critical tearing check: if agg's bug were present, row 1's emoji
	// would start at a pixel offset shifted by (16 cols × cellW).
	// With correct rendering, row 1 emoji start at x=0 (same as row 0).
	row0Pattern := getRowPixelPattern(lastFrame, 0, cellH)
	row1Pattern := getRowPixelPattern(lastFrame, cellH, 2*cellH)
	drift := detectDrift(row0Pattern, row1Pattern)
	if drift != 0 {
		t.Errorf("TEARING DETECTED: row 1 is shifted %d pixels from row 0 (agg-style drift)", drift)
	} else {
		t.Logf("✓ No tearing: rows are aligned (0 pixel drift between rows)")
	}

	t.Logf("GIF: %d frames, %dx%d pixels, %d bytes",
		len(decoded.Image), bounds.Dx(), bounds.Dy(), gifBuf.Len())

	// Write GIF to artifacts for visual inspection
	outPath := "../../artifacts/charmap-emoji-xvt-proof.gif"
	if err := os.MkdirAll("../../artifacts", 0o755); err == nil {
		if err := os.WriteFile(outPath, gifBuf.Bytes(), 0o644); err != nil {
			t.Logf("warning: could not write artifact: %v", err)
		} else {
			t.Logf("✓ GIF written to %s for visual inspection", outPath)
		}
	}
}

func jsonEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			fmt.Fprintf(&b, `\u%04x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hasContentInRegion(img image.Image, x0, y0, x1, y1 int) bool {
	// Check if any non-background pixel exists in the region
	bg := img.At(0, 0) // assume top-left is background
	bgR, bgG, bgB, _ := bg.RGBA()
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r != bgR || g != bgG || b != bgB {
				return true
			}
		}
	}
	return false
}

// getRowPixelPattern returns a 1D bool slice indicating which x-positions
// have non-background content in the given y range.
func getRowPixelPattern(img image.Image, yStart, yEnd int) []bool {
	bounds := img.Bounds()
	pattern := make([]bool, bounds.Dx())
	bg := img.At(0, 0)
	bgR, bgG, bgB, _ := bg.RGBA()
	for x := 0; x < bounds.Dx(); x++ {
		for y := yStart; y < yEnd; y++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r != bgR || g != bgG || b != bgB {
				pattern[x] = true
				break
			}
		}
	}
	return pattern
}

// detectDrift finds the pixel offset between two row patterns.
// Returns 0 if aligned, positive if row1 is shifted right.
func detectDrift(row0, row1 []bool) int {
	// Find first non-bg pixel in each row
	start0 := -1
	start1 := -1
	for i, v := range row0 {
		if v {
			start0 = i
			break
		}
	}
	for i, v := range row1 {
		if v {
			start1 = i
			break
		}
	}
	if start0 < 0 || start1 < 0 {
		return 0 // one row is empty
	}
	return start1 - start0
}
