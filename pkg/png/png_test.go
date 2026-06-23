package png

import (
	"context"
	"image"
	"image/color"
	stdgif "image/gif"
	stdpng "image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tui-cs/tuirec/pkg/gif"
)

func TestParseFrameSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		err   bool
	}{
		{name: "default", value: "", err: false},
		{name: "last", value: "last", err: false},
		{name: "index", value: "2", err: false},
		{name: "at", value: "at:1500", err: false},
		{name: "bad-index", value: "-1", err: true},
		{name: "bad-at", value: "at:-1", err: true},
		{name: "bad-token", value: "middle", err: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseFrameSelection(tt.value)
			if tt.err && err == nil {
				t.Fatalf("ParseFrameSelection(%q) err = nil, want error", tt.value)
			}
			if !tt.err && err != nil {
				t.Fatalf("ParseFrameSelection(%q) err = %v, want nil", tt.value, err)
			}
		})
	}
}

func TestValidatePNG(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "snapshot.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	img.Set(1, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if err := stdpng.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	file.Close()

	validation, err := Validate(path)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validation.Width != 2 || validation.Height != 2 {
		t.Fatalf("dimensions = %dx%d, want 2x2", validation.Width, validation.Height)
	}
	if validation.PixelVariance == 0 {
		t.Fatal("PixelVariance = 0, want non-zero")
	}
}

func TestRendererRendersSelectedFrame(t *testing.T) {
	t.Parallel()

	cast := filepath.Join(t.TempDir(), "demo.cast")
	if err := os.WriteFile(cast, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write cast: %v", err)
	}

	agg := writeFakeAgg(t)
	output := filepath.Join(t.TempDir(), "snapshot.png")
	renderer := Renderer{Frame: FrameSelection{mode: frameIndex, index: 1}}

	err := renderer.Render(context.Background(), cast, output, gif.Config{AggPath: agg})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	file, err := os.Open(output)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer file.Close()

	img, err := stdpng.Decode(file)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	got := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
	if got.G != 255 {
		t.Fatalf("pixel = %+v, want green frame", got)
	}
}

func writeFakeAgg(t *testing.T) string {
	t.Helper()

	template := filepath.Join(t.TempDir(), "template.gif")
	writeTemplateGIF(t, template)

	if runtime.GOOS == "windows" {
		path := filepath.Join(t.TempDir(), "fake-agg.cmd")
		script := "@echo off\r\n" +
			"set out=\r\n" +
			":loop\r\n" +
			"if \"%~1\"==\"\" goto done\r\n" +
			"set out=%~1\r\n" +
			"shift\r\n" +
			"goto loop\r\n" +
			":done\r\n" +
			"copy /Y \"" + template + "\" \"%out%\" >nul\r\n"
		if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
			t.Fatalf("write fake agg: %v", err)
		}
		return path
	}

	path := filepath.Join(t.TempDir(), "fake-agg.sh")
	script := `#!/usr/bin/env sh
set -eu
for arg in "$@"; do out="$arg"; done
cp "` + template + `" "$out"
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write fake agg: %v", err)
	}

	return path
}

func writeTemplateGIF(t *testing.T, path string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create template gif: %v", err)
	}
	defer file.Close()

	palette := color.Palette{
		color.RGBA{R: 255, G: 0, B: 0, A: 255},
		color.RGBA{R: 0, G: 255, B: 0, A: 255},
	}
	first := image.NewPaletted(image.Rect(0, 0, 2, 2), palette)
	first.Pix[0] = 0
	second := image.NewPaletted(image.Rect(0, 0, 2, 2), palette)
	second.Pix[0] = 1
	err = stdgif.EncodeAll(file, &stdgif.GIF{
		Image: []*image.Paletted{first, second},
		Delay: []int{10, 10},
	})
	if err != nil {
		t.Fatalf("encode template gif: %v", err)
	}
}

func TestSelectFrameAtUsesTimeline(t *testing.T) {
	t.Parallel()

	g := &stdgif.GIF{
		Image: []*image.Paletted{
			image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black}),
			image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.White}),
		},
		Delay: []int{10, 10},
	}
	index, err := selectFrame(g, FrameSelection{mode: frameAt, atMS: 150})
	if err != nil {
		t.Fatalf("selectFrame: %v", err)
	}
	if index != 1 {
		t.Fatalf("index = %d, want 1", index)
	}
}

func TestComposeFrameCompositesPartialFrames(t *testing.T) {
	t.Parallel()

	basePalette := color.Palette{
		color.RGBA{R: 255, G: 0, B: 0, A: 255},
	}
	base := image.NewPaletted(image.Rect(0, 0, 2, 2), basePalette)
	for i := range base.Pix {
		base.Pix[i] = 0
	}

	overlayPalette := color.Palette{
		color.RGBA{0, 0, 0, 0},
		color.RGBA{R: 0, G: 255, B: 0, A: 255},
	}
	overlay := image.NewPaletted(image.Rect(1, 1, 2, 2), overlayPalette)
	overlay.Pix[0] = 1

	composed := composeFrame(&stdgif.GIF{
		Image: []*image.Paletted{base, overlay},
		Delay: []int{10, 10},
	}, 1)

	if got := color.RGBAModel.Convert(composed.At(0, 0)).(color.RGBA); got.R != 255 || got.G != 0 {
		t.Fatalf("pixel(0,0) = %+v, want red", got)
	}
	if got := color.RGBAModel.Convert(composed.At(1, 1)).(color.RGBA); got.G != 255 {
		t.Fatalf("pixel(1,1) = %+v, want green", got)
	}
}
