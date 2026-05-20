package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gui-cs/tuirec/examples/internal/demoagg"
	"github.com/gui-cs/tuirec/pkg/gif"
)

func main() {
	castPath := flag.String("cast", "pkg/gif/testdata/animated.cast", "asciinema cast file to render")
	outputPath := flag.String("output", "demo.gif", "GIF file to write")
	aggPath := flag.String("agg-path", demoagg.DefaultPath(), "path to agg binary")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := gif.Config{AggPath: *aggPath}
	if err := gif.Render(ctx, *castPath, *outputPath, config); err != nil {
		fmt.Fprintf(os.Stderr, "render GIF: %v\n", err)
		os.Exit(1)
	}

	validation, err := gif.Validate(*outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate GIF: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %s (%d frames, %dx%d)\n", *outputPath, validation.Frames, validation.Width, validation.Height)
}
