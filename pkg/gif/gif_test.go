package gif

import (
	"image"
	"image/color"
	stdgif "image/gif"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidateAnimatedGIF(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "animated.gif")
	writeTestGIF(t, path, true)

	validation, err := Validate(path)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if validation.Frames != 2 {
		t.Fatalf("Frames = %d, want 2", validation.Frames)
	}

	if validation.Width != 2 || validation.Height != 2 {
		t.Fatalf("dimensions = %dx%d, want 2x2", validation.Width, validation.Height)
	}

	if validation.PixelVariance == 0 {
		t.Fatal("PixelVariance = 0, want non-zero")
	}
}

func TestRenderArgsDefaults(t *testing.T) {
	t.Parallel()

	got := renderArgs("in.cast", "out.gif", NormalizeConfig(Config{}))
	want := []string{
		"--theme", "monokai",
		"--speed", "1",
		"--font-size", "14",
		"--line-height", "1.3",
		"in.cast", "out.gif",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renderArgs() = %#v, want %#v", got, want)
	}
}

func TestRenderArgsIncludesFontWhenSet(t *testing.T) {
	t.Parallel()

	got := renderArgs("in.cast", "out.gif", NormalizeConfig(Config{Font: "Cascadia Mono"}))
	want := []string{
		"--theme", "monokai",
		"--speed", "1",
		"--font-size", "14",
		"--line-height", "1.3",
		"--font-family", "Cascadia Mono",
		"in.cast", "out.gif",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renderArgs() = %#v, want %#v", got, want)
	}
}

func TestRenderArgsIncludesLetterSpacingWhenSet(t *testing.T) {
	t.Parallel()

	got := renderArgs("in.cast", "out.gif", NormalizeConfig(Config{LetterSpacing: -0.5}))
	want := []string{
		"--theme", "monokai",
		"--speed", "1",
		"--font-size", "14",
		"--line-height", "1.3",
		"--letter-spacing", "-0.5",
		"in.cast", "out.gif",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renderArgs() = %#v, want %#v", got, want)
	}
}

func TestRenderArgsIncludesSelectWhenSet(t *testing.T) {
	t.Parallel()

	got := renderArgs("in.cast", "out.gif", NormalizeConfig(Config{Select: "0.2.."}))
	want := []string{
		"--theme", "monokai",
		"--speed", "1",
		"--font-size", "14",
		"--line-height", "1.3",
		"--select", "0.2..",
		"in.cast", "out.gif",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renderArgs() = %#v, want %#v", got, want)
	}
}

func TestValidateRejectsSingleFrameGIF(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "single.gif")
	writeGIF(t, path, []*image.Paletted{newFrame(0)})

	if _, err := Validate(path); err == nil {
		t.Fatal("Validate(single-frame) err = nil, want error")
	}
}

func TestValidateRejectsStaticGIF(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "static.gif")
	frame := newFrame(0)
	writeGIF(t, path, []*image.Paletted{frame, frame})

	if _, err := Validate(path); err == nil {
		t.Fatal("Validate(static) err = nil, want error")
	}
}

func writeTestGIF(t *testing.T, path string, animated bool) {
	t.Helper()

	second := byte(0)
	if animated {
		second = 1
	}

	writeGIF(t, path, []*image.Paletted{newFrame(0), newFrame(second)})
}

func writeGIF(t *testing.T, path string, frames []*image.Paletted) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gif: %v", err)
	}
	defer file.Close()

	delays := make([]int, len(frames))
	err = stdgif.EncodeAll(file, &stdgif.GIF{
		Image: frames,
		Delay: delays,
	})
	if err != nil {
		t.Fatalf("encode gif: %v", err)
	}
}

func newFrame(index byte) *image.Paletted {
	palette := color.Palette{
		color.RGBA{R: 0, G: 0, B: 0, A: 255},
		color.RGBA{R: 255, G: 255, B: 255, A: 255},
	}
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), palette)
	frame.Pix[0] = index

	return frame
}
