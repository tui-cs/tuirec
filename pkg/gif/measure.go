package gif

import (
	"context"
	"fmt"
	"image/gif"
	"os"
	"path/filepath"
)

// probe cast column counts. The per-column width is isolated by the slope
// (wideCols - narrowCols), which cancels agg's fixed horizontal padding.
const (
	probeNarrowCols = 40
	probeWideCols   = 120
	probeRows       = 8
)

// MeasureColumnWidthPx renders two short probe casts that differ only in column
// count and returns the pixel width agg actually advances per column for the
// given font configuration. agg lays columns out at a fixed advance plus fixed
// horizontal padding, so (wideWidth - narrowWidth) / (wideCols - narrowCols)
// yields the per-column width independent of that padding — and independent of
// which font agg resolves on the host, which a static formula cannot know.
func MeasureColumnWidthPx(ctx context.Context, config Config) (float64, error) {
	config = NormalizeConfig(config)

	dir, err := os.MkdirTemp("", "tuirec-probe-*")
	if err != nil {
		return 0, fmt.Errorf("probe tempdir: %w", err)
	}
	defer os.RemoveAll(dir)

	narrow, err := renderProbeWidth(ctx, config, dir, probeNarrowCols)
	if err != nil {
		return 0, err
	}

	wide, err := renderProbeWidth(ctx, config, dir, probeWideCols)
	if err != nil {
		return 0, err
	}

	span := wide - narrow
	if span <= 0 {
		return 0, fmt.Errorf("probe produced non-increasing widths (%d, %d)", narrow, wide)
	}

	return float64(span) / float64(probeWideCols-probeNarrowCols), nil
}

// renderProbeWidth writes a minimal cast of the given column count, renders it
// with agg, and returns the resulting GIF's pixel width.
func renderProbeWidth(ctx context.Context, config Config, dir string, cols int) (int, error) {
	castPath := filepath.Join(dir, fmt.Sprintf("probe-%d.cast", cols))
	gifPath := filepath.Join(dir, fmt.Sprintf("probe-%d.gif", cols))

	cast := fmt.Sprintf("{\"version\":2,\"width\":%d,\"height\":%d}\n[0.1,\"o\",\"x\"]\n[0.2,\"o\",\"y\"]\n", cols, probeRows)
	if err := os.WriteFile(castPath, []byte(cast), 0o600); err != nil {
		return 0, fmt.Errorf("write probe cast: %w", err)
	}

	if err := Render(ctx, castPath, gifPath, config); err != nil {
		return 0, fmt.Errorf("render probe: %w", err)
	}

	file, err := os.Open(gifPath)
	if err != nil {
		return 0, fmt.Errorf("open probe gif: %w", err)
	}
	defer file.Close()

	cfg, err := gif.DecodeConfig(file)
	if err != nil {
		return 0, fmt.Errorf("decode probe gif: %w", err)
	}

	return cfg.Width, nil
}
