package record

import (
	"math"
	"testing"

	"github.com/gui-cs/tuirec/pkg/gif"
)

func TestAlignFontSize(t *testing.T) {
	// DejaVu Sans Mono advance ratio measured from agg (8.425px / 14px).
	const ratio = 0.601786

	tests := []struct {
		name      string
		ratio     float64
		requested int
		wantSize  int
		wantCellW int
	}{
		// 14 → cell 8.425, rounding error 0.425 > tolerance, so nudge to 15
		// (cell 9.027, error 0.027).
		{"poorly aligned nudges up", ratio, 14, 15, 9},
		// 20 → cell 12.036, error 0.036 < tolerance, keep as requested.
		{"well aligned kept", ratio, 20, 20, 12},
		// 15 → cell 9.027, error 0.027 < tolerance, keep.
		{"already aligned kept", ratio, 15, 15, 9},
		// Exact integer ratio: every size is aligned, keep requested.
		{"exact ratio kept", 0.5, 14, 14, 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSize, gotCellW := alignFontSize(tt.ratio, tt.requested)
			if gotSize != tt.wantSize || gotCellW != tt.wantCellW {
				t.Fatalf("alignFontSize(%.5f, %d) = (size %d, cellW %d); want (size %d, cellW %d)",
					tt.ratio, tt.requested, gotSize, gotCellW, tt.wantSize, tt.wantCellW)
			}
		})
	}
}

func TestAlignFontSizeNeverBelowOne(t *testing.T) {
	size, cellW := alignFontSize(0.6, 1)
	if size < 1 || cellW < 1 {
		t.Fatalf("alignFontSize(0.6, 1) = (%d, %d); want both >= 1", size, cellW)
	}
}

func TestAlignLineHeightRendersExactInteger(t *testing.T) {
	// Reporting cellH and rendering with the returned line-height must make
	// agg's row height (fontSize × lineHeight) equal cellH exactly.
	cellH, lh := alignLineHeight(15, 1.3) // round(19.5) = 20
	if cellH != 20 {
		t.Fatalf("cellH = %d; want 20", cellH)
	}

	rendered := 15 * lh
	if math.Abs(rendered-float64(cellH)) > 1e-9 {
		t.Fatalf("agg row height %.6f != reported cellH %d", rendered, cellH)
	}
}

func TestWillRenderGIF(t *testing.T) {
	// A GIF is rendered whenever Output is set; calibration must run there even
	// when AggPath is the zero value (normalized to "agg" downstream).
	if !willRenderGIF(Config{Output: "out.gif"}) {
		t.Fatal("Output set (empty AggPath) should render and calibrate")
	}

	if !willRenderGIF(Config{Output: "out.gif", GIF: gif.Config{AggPath: "agg"}}) {
		t.Fatal("Output set with explicit AggPath should render and calibrate")
	}

	if willRenderGIF(Config{}) {
		t.Fatal("no Output should not render")
	}
}

func TestCellRoundingErr(t *testing.T) {
	if e := cellRoundingErr(0.6, 10); e > 1e-9 { // 6.0 exactly
		t.Fatalf("cellRoundingErr(0.6, 10) = %.6f; want 0", e)
	}

	if e := cellRoundingErr(0.601786, 14); math.Abs(e-0.425) > 0.01 { // 8.425
		t.Fatalf("cellRoundingErr(0.601786, 14) = %.6f; want ~0.425", e)
	}
}
