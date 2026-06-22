package record

import (
	"context"
	"math"

	"github.com/gui-cs/tuirec/pkg/gif"
)

// alignTolerancePx is the largest per-column rounding error tolerated before the
// font-size is nudged to align the sixel cell grid. agg lays columns out at a
// fractional advance (≈0.6 × font-size), but terminal cell-size reports (CSI
// 16t) are integers, so an app that sizes a sixel raster as cells × reportedCell
// underfills by (advance − round(advance)) per column. At ~0.15px that residual
// is well under a third of a cell across a full-width image; beyond it the gap
// becomes visible (see gui-cs/tuirec#84).
const alignTolerancePx = 0.15

// cellRoundingErr is how far agg's per-column advance at fontSize is from the
// nearest integer — i.e. the per-column error if that integer is reported.
func cellRoundingErr(ratio float64, fontSize int) float64 {
	w := ratio * float64(fontSize)

	return math.Abs(w - math.Round(w))
}

// alignFontSize picks the font-size whose agg cell width rounds closest to an
// integer, so the reported (integer) cell matches what agg renders. The
// requested size is kept when its rounding error is already within tolerance;
// otherwise nearby sizes (±1 then ±2) are searched and the best is chosen,
// preferring the smallest move on ties. ratio is agg's measured advance per
// font-size pixel. Returns the chosen size and the integer cell width to report.
func alignFontSize(ratio float64, requested int) (fontSize, cellW int) {
	if requested < 1 {
		requested = 1
	}

	best := requested
	bestErr := cellRoundingErr(ratio, requested)

	if bestErr > alignTolerancePx {
		for _, delta := range []int{-1, 1, -2, 2} {
			cand := requested + delta
			if cand < 1 {
				continue
			}

			if e := cellRoundingErr(ratio, cand); e < bestErr-1e-9 {
				best = cand
				bestErr = e
			}
		}
	}

	cellW = int(math.Round(ratio * float64(best)))
	if cellW < 1 {
		cellW = 1
	}

	return best, cellW
}

// alignLineHeight returns the integer cell height to report and the line-height
// to pass to agg so that agg renders exactly that height. agg's row height is
// fontSize × lineHeight, so reporting round(fontSize × requested) and rendering
// with lineHeight = cellH / fontSize makes the report exact.
func alignLineHeight(fontSize int, requestedLineHeight float64) (cellH int, aggLineHeight float64) {
	cellH = int(math.Round(float64(fontSize) * requestedLineHeight))
	if cellH < 1 {
		cellH = 1
	}

	return cellH, float64(cellH) / float64(fontSize)
}

// calibrateGeometry measures agg's actual per-column advance for the configured
// font and returns font settings whose rendered cell is an integer, plus that
// integer cell size to report to sixel queries. This replaces a static
// advance-ratio guess (which is wrong for whatever font agg resolves on the
// host) with a measurement, and aligns the grid so apps that size rasters as
// cells × reportedCell fill exactly (gui-cs/tuirec#84). The returned Config has
// FontSize/LineHeight adjusted for rendering; changed reports whether the
// font-size moved from the request.
func calibrateGeometry(ctx context.Context, gifCfg gif.Config) (adjusted gif.Config, cellW, cellH int, changed bool, err error) {
	cfg := gif.NormalizeConfig(gifCfg)

	colPx, err := gif.MeasureColumnWidthPx(ctx, cfg)
	if err != nil {
		return gifCfg, 0, 0, false, err
	}

	ratio := colPx / float64(cfg.FontSize)
	fontSize, cellW := alignFontSize(ratio, cfg.FontSize)
	cellH, aggLineHeight := alignLineHeight(fontSize, cfg.LineHeight)

	changed = fontSize != cfg.FontSize
	cfg.FontSize = fontSize
	cfg.LineHeight = aggLineHeight

	return cfg, cellW, cellH, changed, nil
}
