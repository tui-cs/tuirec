package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/tui-cs/tuirec/examples/internal/demoagg"
	"github.com/tui-cs/tuirec/pkg/gif"
	"github.com/tui-cs/tuirec/pkg/pty"
	"github.com/tui-cs/tuirec/pkg/record"
)

func main() {
	outputPath := flag.String("output", "pipeline-demo.gif", "GIF file to write")
	castPath := flag.String("cast-output", "pipeline-demo.cast", "cast file to write")
	aggPath := flag.String("agg-path", demoagg.DefaultPath(), "path to agg binary")
	flag.Parse()

	goPath, err := exec.LookPath("go")
	if err != nil {
		fmt.Fprintf(os.Stderr, "find go: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := record.Run(ctx, record.Config{
		Binary:         goPath,
		Args:           []string{"run", "./internal/testapp"},
		CastOutput:     *castPath,
		Output:         *outputPath,
		Title:          "tuirec pipeline demo",
		Keystrokes:     "wait:1000,ArrowRight,ArrowDown,Hi,wait:500,Ctrl+Q",
		KeystrokeDelay: 100 * time.Millisecond,
		MaxDuration:    20 * time.Second,
		DrainDuration:  750 * time.Millisecond,
		Size:           pty.Size{Cols: 80, Rows: 24},
		GIF:            gif.Config{AggPath: *aggPath},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "record pipeline: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Wrote %s and %s\n", result.GIFPath, result.CastPath)
}
