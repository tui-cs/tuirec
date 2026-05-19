// Package gif renders asciinema cast files to animated GIFs.
package gif

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	stdgif "image/gif"
	"os"
	"os/exec"
	"strconv"
)

const (
	defaultAggPath    = "agg"
	defaultTheme      = "monokai"
	defaultSpeed      = 1.0
	defaultFontSize   = 14
	defaultLineHeight = 1.0
)

// ErrValidation indicates that a rendered GIF failed validation.
var ErrValidation = errors.New("gif validation failed")

// Config controls agg GIF rendering.
type Config struct {
	AggPath    string
	Theme      string
	Speed      float64
	Font       string
	FontSize   int
	LineHeight float64
}

// Validation describes a decoded GIF.
type Validation struct {
	Frames        int
	Width         int
	Height        int
	PixelVariance int
}

// Render converts an asciinema cast file to an animated GIF with agg.
func Render(ctx context.Context, castPath, outputPath string, config Config) error {
	config = normalizeConfig(config)

	args := renderArgs(castPath, outputPath, config)
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, config.AggPath, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run agg: %w: %s", err, stderr.String())
	}

	return nil
}

func renderArgs(castPath, outputPath string, config Config) []string {
	args := []string{
		"--theme", config.Theme,
		"--speed", formatFloat(config.Speed),
		"--font-size", strconv.Itoa(config.FontSize),
		"--line-height", formatFloat(config.LineHeight),
	}
	if config.Font != "" {
		args = append(args, "--font-family", config.Font)
	}
	args = append(args, castPath, outputPath)

	return args
}

// Validate decodes a GIF and verifies it has visible animation content.
func Validate(path string) (Validation, error) {
	file, err := os.Open(path)
	if err != nil {
		return Validation{}, fmt.Errorf("%w: open gif: %w", ErrValidation, err)
	}
	defer file.Close()

	decoded, err := stdgif.DecodeAll(file)
	if err != nil {
		return Validation{}, fmt.Errorf("%w: decode gif: %w", ErrValidation, err)
	}

	if len(decoded.Image) < 2 {
		return Validation{}, fmt.Errorf("%w: gif has %d frame(s), want at least 2", ErrValidation, len(decoded.Image))
	}

	bounds := decoded.Image[0].Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return Validation{}, fmt.Errorf("%w: gif dimensions are zero", ErrValidation)
	}

	variance := pixelVariance(decoded)
	if variance == 0 {
		return Validation{}, fmt.Errorf("%w: gif frames have no pixel variance", ErrValidation)
	}

	return Validation{
		Frames:        len(decoded.Image),
		Width:         bounds.Dx(),
		Height:        bounds.Dy(),
		PixelVariance: variance,
	}, nil
}

func normalizeConfig(config Config) Config {
	if config.AggPath == "" {
		config.AggPath = defaultAggPath
	}

	if config.Theme == "" {
		config.Theme = defaultTheme
	}

	if config.Speed == 0 {
		config.Speed = defaultSpeed
	}

	if config.FontSize == 0 {
		config.FontSize = defaultFontSize
	}

	if config.LineHeight == 0 {
		config.LineHeight = defaultLineHeight
	}

	return config
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func pixelVariance(decoded *stdgif.GIF) int {
	if len(decoded.Image) < 2 {
		return 0
	}

	first := decoded.Image[0]
	for _, frame := range decoded.Image[1:] {
		if variance := frameVariance(first, frame); variance > 0 {
			return variance
		}
	}

	return 0
}

func frameVariance(first, second image.Image) int {
	bounds := first.Bounds().Intersect(second.Bounds())
	var variance int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if colorsDiffer(first.At(x, y), second.At(x, y)) {
				variance++
			}
		}
	}

	return variance
}

func colorsDiffer(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()

	return ar != br || ag != bg || ab != bb || aa != ba
}
