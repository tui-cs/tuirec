package renderer

import (
	"image"
	"image/color"
	"image/draw"
	stdgif "image/gif"
	"io"

	"github.com/charmbracelet/x/vt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// RenderConfig controls the GIF rendering.
type RenderConfig struct {
	// CellWidth and CellHeight define the pixel size per terminal cell.
	// Defaults: 7×13 (matching basicfont.Face7x13).
	CellWidth  int
	CellHeight int

	// Foreground and Background colors.
	Foreground color.Color
	Background color.Color

	// MaxFrames limits the number of frames (0 = no limit).
	MaxFrames int

	// FrameDelay is the GIF delay between frames in 100ths of a second.
	// Default: 50 (500ms).
	FrameDelay int
}

func (c RenderConfig) normalize() RenderConfig {
	if c.CellWidth == 0 {
		c.CellWidth = 7
	}
	if c.CellHeight == 0 {
		c.CellHeight = 13
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
	return c
}

// RenderGIF reads an asciinema cast from r and writes an animated GIF to w.
func RenderGIF(r io.Reader, w io.Writer, cfg RenderConfig) error {
	cfg = cfg.normalize()

	hdr, events, err := ParseCast(r)
	if err != nil {
		return err
	}

	emu := vt.NewEmulator(hdr.Width, hdr.Height)

	imgWidth := hdr.Width * cfg.CellWidth
	imgHeight := hdr.Height * cfg.CellHeight

	gif := &stdgif.GIF{}

	// Render initial blank frame
	gif.Image = append(gif.Image, renderFrame(emu, hdr.Width, hdr.Height, imgWidth, imgHeight, cfg))
	gif.Delay = append(gif.Delay, cfg.FrameDelay)

	frameCount := 1
	for _, ev := range events {
		if ev.Type != "o" {
			continue
		}
		if _, err := emu.Write(ev.Data); err != nil {
			return err
		}

		frame := renderFrame(emu, hdr.Width, hdr.Height, imgWidth, imgHeight, cfg)
		gif.Image = append(gif.Image, frame)

		// Calculate delay from event timing
		delay := cfg.FrameDelay
		gif.Delay = append(gif.Delay, delay)

		frameCount++
		if cfg.MaxFrames > 0 && frameCount >= cfg.MaxFrames {
			break
		}
	}

	return stdgif.EncodeAll(w, gif)
}

func renderFrame(emu *vt.Emulator, cols, rows, imgW, imgH int, cfg RenderConfig) *image.Paletted {
	// Build a palette with basic terminal colors
	palette := buildPalette(cfg)

	img := image.NewPaletted(image.Rect(0, 0, imgW, imgH), palette)
	// Fill background
	draw.Draw(img, img.Bounds(), &image.Uniform{cfg.Background}, image.Point{}, draw.Src)

	face := basicfont.Face7x13

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
			if bg != cfg.Background {
				cellRect := image.Rect(x, y, x+cfg.CellWidth*cell.Width, y+cfg.CellHeight)
				draw.Draw(img, cellRect, &image.Uniform{bg}, image.Point{}, draw.Src)
			}

			// Draw the glyph
			drawString(img, face, x, y+cfg.CellHeight-2, cell.Content, fg)
		}
	}

	return img
}

func drawString(dst *image.Paletted, face font.Face, x, y int, s string, col color.Color) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{col},
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

func buildPalette(cfg RenderConfig) color.Palette {
	// Basic 16-color terminal palette + bg/fg
	return color.Palette{
		cfg.Background,                    // 0: background
		cfg.Foreground,                    // 1: foreground
		color.RGBA{0, 0, 0, 255},         // 2: black
		color.RGBA{170, 0, 0, 255},       // 3: red
		color.RGBA{0, 170, 0, 255},       // 4: green
		color.RGBA{170, 170, 0, 255},     // 5: yellow
		color.RGBA{0, 0, 170, 255},       // 6: blue
		color.RGBA{170, 0, 170, 255},     // 7: magenta
		color.RGBA{0, 170, 170, 255},     // 8: cyan
		color.RGBA{170, 170, 170, 255},   // 9: white
		color.RGBA{85, 85, 85, 255},      // 10: bright black
		color.RGBA{255, 85, 85, 255},     // 11: bright red
		color.RGBA{85, 255, 85, 255},     // 12: bright green
		color.RGBA{255, 255, 85, 255},    // 13: bright yellow
		color.RGBA{85, 85, 255, 255},     // 14: bright blue
		color.RGBA{255, 85, 255, 255},    // 15: bright magenta
		color.RGBA{85, 255, 255, 255},    // 16: bright cyan
		color.RGBA{255, 255, 255, 255},   // 17: bright white
	}
}
