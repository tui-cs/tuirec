// Package png renders asciinema cast files to still PNG images.
package png

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	stdgif "image/gif"
	stdpng "image/png"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gui-cs/tuirec/pkg/gif"
)

// ErrValidation indicates that a rendered PNG failed validation.
var ErrValidation = errors.New("png validation failed")

type frameMode int

const (
	frameLast frameMode = iota
	frameIndex
	frameAt
)

// FrameSelection identifies which frame to snapshot from a rendered GIF.
type FrameSelection struct {
	mode  frameMode
	index int
	atMS  int
}

// Renderer renders a cast into a PNG snapshot.
type Renderer struct {
	Frame FrameSelection
}

// Validation describes a decoded PNG.
type Validation struct {
	Width         int
	Height        int
	PixelVariance int
}

// ParseFrameSelection parses snapshot frame syntax: last, <index>, at:<ms>.
func ParseFrameSelection(value string) (FrameSelection, error) {
	if value == "" || value == "last" {
		return FrameSelection{mode: frameLast}, nil
	}

	if strings.HasPrefix(value, "at:") {
		raw := strings.TrimPrefix(value, "at:")
		ms, err := strconv.Atoi(raw)
		if err != nil || ms < 0 {
			return FrameSelection{}, fmt.Errorf("invalid --frame %q: use last, <index>, or at:<ms>", value)
		}

		return FrameSelection{mode: frameAt, atMS: ms}, nil
	}

	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return FrameSelection{}, fmt.Errorf("invalid --frame %q: use last, <index>, or at:<ms>", value)
	}

	return FrameSelection{mode: frameIndex, index: index}, nil
}

// Render converts an asciinema cast file to a still PNG by rendering with agg
// first and selecting one frame from the produced GIF.
func Render(ctx context.Context, castPath, outputPath string, config gif.Config, frame FrameSelection) error {
	tmpFile, err := os.CreateTemp("", "tuirec-snapshot-*.gif")
	if err != nil {
		return fmt.Errorf("create temporary gif: %w", err)
	}
	tmpGIF := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temporary gif: %w", err)
	}
	defer os.Remove(tmpGIF)

	if err := gif.Render(ctx, castPath, tmpGIF, config); err != nil {
		return err
	}

	decoded, err := decodeGIF(tmpGIF)
	if err != nil {
		return err
	}

	frameIndex, err := selectFrame(decoded, frame)
	if err != nil {
		return err
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create png: %w", err)
	}
	defer outFile.Close()

	if err := stdpng.Encode(outFile, composeFrame(decoded, frameIndex)); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}

	return nil
}

func composeFrame(decoded *stdgif.GIF, frameIndex int) image.Image {
	bounds := image.Rect(0, 0, decoded.Config.Width, decoded.Config.Height)
	if bounds.Empty() {
		bounds = decoded.Image[0].Bounds()
	}

	var background color.Color = color.Transparent
	if palette, ok := decoded.Config.ColorModel.(color.Palette); ok {
		if idx := int(decoded.BackgroundIndex); idx >= 0 && idx < len(palette) {
			background = palette[idx]
		}
	}

	canvas := image.NewRGBA(bounds)
	var previous *image.RGBA
	clearRect := func(rect image.Rectangle) {
		draw.Draw(canvas, rect.Intersect(bounds), image.NewUniform(background), image.Point{}, draw.Src)
	}

	for i := 0; i <= frameIndex; i++ {
		if i > 0 && i-1 < len(decoded.Disposal) {
			prevFrame := decoded.Image[i-1].Bounds()
			switch decoded.Disposal[i-1] {
			case stdgif.DisposalBackground:
				clearRect(prevFrame)
			case stdgif.DisposalPrevious:
				if previous != nil {
					draw.Draw(canvas, bounds, previous, bounds.Min, draw.Src)
				}
			}
		}

		previous = nil
		if i < len(decoded.Disposal) && decoded.Disposal[i] == stdgif.DisposalPrevious {
			previous = image.NewRGBA(bounds)
			draw.Draw(previous, bounds, canvas, bounds.Min, draw.Src)
		}

		frame := decoded.Image[i]
		draw.Draw(canvas, frame.Bounds().Intersect(bounds), frame, frame.Bounds().Min, draw.Over)
	}

	return canvas
}

func (r Renderer) Render(ctx context.Context, castPath, outputPath string, config gif.Config) error {
	if err := Render(ctx, castPath, outputPath, config, r.Frame); err != nil {
		return err
	}

	if _, err := Validate(outputPath); err != nil {
		return err
	}

	return nil
}

func decodeGIF(path string) (*stdgif.GIF, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open rendered gif: %w", err)
	}
	defer file.Close()

	decoded, err := stdgif.DecodeAll(file)
	if err != nil {
		return nil, fmt.Errorf("decode rendered gif: %w", err)
	}
	if len(decoded.Image) == 0 {
		return nil, fmt.Errorf("decode rendered gif: no frames")
	}

	return decoded, nil
}

func selectFrame(decoded *stdgif.GIF, frame FrameSelection) (int, error) {
	count := len(decoded.Image)

	switch frame.mode {
	case frameLast:
		return count - 1, nil
	case frameIndex:
		if frame.index >= count {
			return 0, fmt.Errorf("frame index %d out of range (0-%d)", frame.index, count-1)
		}

		return frame.index, nil
	case frameAt:
		target := time.Duration(frame.atMS) * time.Millisecond
		if target <= 0 {
			return 0, nil
		}

		var elapsed time.Duration
		for i, delay := range decoded.Delay {
			// GIF delay units are centiseconds.
			elapsed += time.Duration(delay) * 10 * time.Millisecond
			if target < elapsed {
				return i, nil
			}
		}

		return count - 1, nil
	default:
		return 0, fmt.Errorf("unknown frame selection mode")
	}
}

// Validate decodes a PNG and verifies it has visible image content.
func Validate(path string) (Validation, error) {
	file, err := os.Open(path)
	if err != nil {
		return Validation{}, fmt.Errorf("%w: open png: %w", ErrValidation, err)
	}
	defer file.Close()

	decoded, err := stdpng.Decode(file)
	if err != nil {
		return Validation{}, fmt.Errorf("%w: decode png: %w", ErrValidation, err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return Validation{}, fmt.Errorf("%w: png dimensions are zero", ErrValidation)
	}

	variance := pixelVariance(decoded)
	if variance == 0 {
		return Validation{}, fmt.Errorf("%w: png has no pixel variance", ErrValidation)
	}

	return Validation{
		Width:         bounds.Dx(),
		Height:        bounds.Dy(),
		PixelVariance: variance,
	}, nil
}

func pixelVariance(img image.Image) int {
	bounds := img.Bounds()
	base := img.At(bounds.Min.X, bounds.Min.Y)
	var variance int
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if colorsDiffer(base, img.At(x, y)) {
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
