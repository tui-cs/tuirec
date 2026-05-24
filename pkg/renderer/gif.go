package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	stdgif "image/gif"
	"io"
	"sync"

	"github.com/charmbracelet/x/vt"
	"github.com/gui-cs/tuirec/pkg/renderer/fonts"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// RenderConfig controls the GIF rendering.
type RenderConfig struct {
	// CellWidth and CellHeight define the pixel size per terminal cell.
	// If zero, derived from the font metrics at FontSize.
	CellWidth  int
	CellHeight int

	// FontSize is the TrueType font size in points. Default: 14.
	FontSize float64

	// Foreground and Background colors.
	Foreground color.Color
	Background color.Color

	// MaxFrames limits the number of frames (0 = no limit).
	MaxFrames int

	// FrameDelay is the GIF delay between frames in 100ths of a second.
	// Default: 50 (500ms).
	FrameDelay int
}

var (
	parsedFont     *opentype.Font
	parsedFontOnce sync.Once
	parsedFontErr  error
)

func loadFont() (*opentype.Font, error) {
	parsedFontOnce.Do(func() {
		parsedFont, parsedFontErr = opentype.Parse(fonts.CaskaydiaCoveRegular)
	})
	return parsedFont, parsedFontErr
}

func (c RenderConfig) normalize() (RenderConfig, font.Face, error) {
	if c.FontSize == 0 {
		c.FontSize = 14
	}
	if c.Foreground == nil {
		c.Foreground = color.White
	}
	if c.Background == nil {
		c.Background = color.Black
	}
	if c.FrameDelay == 0 {
		c.FrameDelay = 50
	}

	ft, err := loadFont()
	if err != nil {
		return c, nil, fmt.Errorf("loading embedded font: %w", err)
	}

	face, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size:    c.FontSize,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return c, nil, fmt.Errorf("creating font face: %w", err)
	}

	// Derive cell dimensions from font metrics if not explicitly set
	if c.CellWidth == 0 || c.CellHeight == 0 {
		metrics := face.Metrics()
		if c.CellHeight == 0 {
			h := (metrics.Ascent + metrics.Descent).Ceil()
			if h < metrics.Height.Ceil() {
				h = metrics.Height.Ceil()
			}
			c.CellHeight = h
		}
		if c.CellWidth == 0 {
			// Use advance of 'M' for monospace width
			adv, ok := face.GlyphAdvance('M')
			if ok {
				c.CellWidth = adv.Ceil()
			} else {
				c.CellWidth = c.CellHeight / 2
			}
		}
	}

	return c, face, nil
}

// RenderGIF reads an asciinema cast from r and writes an animated GIF to w.
func RenderGIF(r io.Reader, w io.Writer, cfg RenderConfig) error {
	cfg, face, err := cfg.normalize()
	if err != nil {
		return err
	}

	hdr, events, err := ParseCast(r)
	if err != nil {
		return err
	}

	emu := vt.NewEmulator(hdr.Width, hdr.Height)

	imgWidth := hdr.Width * cfg.CellWidth
	imgHeight := hdr.Height * cfg.CellHeight

	gif := &stdgif.GIF{}

	// Render initial blank frame
	gif.Image = append(gif.Image, renderFrame(emu, hdr.Width, hdr.Height, imgWidth, imgHeight, cfg, face))
	gif.Delay = append(gif.Delay, cfg.FrameDelay)

	frameCount := 1
	for _, ev := range events {
		if ev.Type != "o" {
			continue
		}
		if _, err := emu.Write(ev.Data); err != nil {
			return err
		}

		frame := renderFrame(emu, hdr.Width, hdr.Height, imgWidth, imgHeight, cfg, face)
		gif.Image = append(gif.Image, frame)

		delay := cfg.FrameDelay
		gif.Delay = append(gif.Delay, delay)

		frameCount++
		if cfg.MaxFrames > 0 && frameCount >= cfg.MaxFrames {
			break
		}
	}

	return stdgif.EncodeAll(w, gif)
}

func renderFrame(emu *vt.Emulator, cols, rows, imgW, imgH int, cfg RenderConfig, face font.Face) *image.Paletted {
	// Render to RGBA for full 24-bit color support
	rgba := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	// Fill background
	draw.Draw(rgba, rgba.Bounds(), &image.Uniform{cfg.Background}, image.Point{}, draw.Src)

	metrics := face.Metrics()
	baseline := metrics.Ascent.Ceil()

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cell := emu.CellAt(col, row)
			if cell == nil || cell.Content == "" || cell.Content == " " {
				continue
			}

			// Skip placeholder cells (second cell of a wide char)
			if cell.Width == 0 {
				continue
			}

			// Determine colors
			fg := cfg.Foreground
			bg := cfg.Background

			cellFg := cell.Style.Fg
			cellBg := cell.Style.Bg
			if cellFg != nil {
				fg = cellFg
			}
			if cellBg != nil {
				bg = cellBg
			}

			x := col * cfg.CellWidth
			y := row * cfg.CellHeight

			// Draw cell background if non-default
			if !colorsEqual(bg, cfg.Background) {
				cellRect := image.Rect(x, y, x+cfg.CellWidth*cell.Width, y+cfg.CellHeight)
				draw.Draw(rgba, cellRect, &image.Uniform{bg}, image.Point{}, draw.Src)
			}

			// Draw the glyph
			drawStringRGBA(rgba, face, x, y+baseline, cell.Content, fg)
		}
	}

	// Quantize RGBA to paletted for GIF encoding
	return quantizeFrame(rgba)
}

func drawStringRGBA(dst *image.RGBA, face font.Face, x, y int, s string, col color.Color) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{col},
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// quantizeFrame converts an RGBA image to a paletted image using
// Floyd-Steinberg dithering against the Plan 9 palette (256 colors).
func quantizeFrame(rgba *image.RGBA) *image.Paletted {
	bounds := rgba.Bounds()
	paletted := image.NewPaletted(bounds, palette.Plan9)
	draw.FloydSteinberg.Draw(paletted, bounds, rgba, image.Point{})
	return paletted
}

// colorsEqual compares two colors for equality.
func colorsEqual(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
