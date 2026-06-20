package gif

import "testing"

// A Select range that trims past the probe cast's only events (0.1s/0.2s) would
// leave agg an empty selection and silently break calibration. probeConfig must
// clear it while preserving every option that affects column width.
func TestProbeConfigClearsSelectKeepsGeometry(t *testing.T) {
	in := Config{
		AggPath:       "agg",
		Theme:         "nord",
		Speed:         2,
		Font:          "JetBrains Mono",
		FontSize:      15,
		LineHeight:    1.3,
		LetterSpacing: 0.5,
		Select:        "1..",
	}

	got := probeConfig(in)

	if got.Select != "" {
		t.Fatalf("Select not cleared: %q", got.Select)
	}

	// Geometry- and render-affecting fields must survive untouched.
	if got.FontSize != in.FontSize ||
		got.LineHeight != in.LineHeight ||
		got.Font != in.Font ||
		got.LetterSpacing != in.LetterSpacing ||
		got.Theme != in.Theme ||
		got.AggPath != in.AggPath {
		t.Fatalf("probeConfig altered a non-timeline field: got %+v", got)
	}
}
