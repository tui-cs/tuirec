package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gui-cs/TUIcast/pkg/gif"
	"github.com/gui-cs/TUIcast/pkg/record"
)

func TestRecordCommandParsesFlags(t *testing.T) {
	t.Parallel()

	var got record.Config
	stdout := &bytes.Buffer{}
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--args", "one,two",
		"--output", "demo.gif",
		"--cast-output", "demo.cast",
		"--keystrokes", "wait:10,Ctrl+Q",
		"--keystroke-delay", "25",
		"--cols", "80",
		"--rows", "24",
		"--theme", "dracula",
		"--font", "Cascadia Mono",
		"--font-size", "16",
		"--line-height", "1.2",
		"--speed", "1.5",
		"--max-duration", "7",
		"--title", "Demo",
		"--agg-path", "agg-bin",
		"--drain", "750",
	}, cliOptions{
		stdout: stdout,
		stderr: &bytes.Buffer{},
		look: func(path string) (string, error) {
			return filepath.Join("resolved", path), nil
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			got = config
			return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	if got.Binary != filepath.Join("resolved", "demo-app") {
		t.Fatalf("Binary = %q", got.Binary)
	}
	if got.GIF.AggPath != filepath.Join("resolved", "agg-bin") {
		t.Fatalf("AggPath = %q", got.GIF.AggPath)
	}
	if strings.Join(got.Args, ",") != "one,two" {
		t.Fatalf("Args = %#v", got.Args)
	}
	if got.Output != "demo.gif" || got.CastOutput != "demo.cast" {
		t.Fatalf("outputs = %q %q", got.Output, got.CastOutput)
	}
	if got.Keystrokes != "wait:10,Ctrl+Q" {
		t.Fatalf("Keystrokes = %q", got.Keystrokes)
	}
	if got.KeystrokeDelay.String() != "25ms" {
		t.Fatalf("KeystrokeDelay = %s", got.KeystrokeDelay)
	}
	if got.MaxDuration.String() != "7s" {
		t.Fatalf("MaxDuration = %s", got.MaxDuration)
	}
	if got.DrainDuration.String() != "750ms" {
		t.Fatalf("DrainDuration = %s", got.DrainDuration)
	}
	if got.Size.Cols != 80 || got.Size.Rows != 24 {
		t.Fatalf("Size = %#v", got.Size)
	}
	if got.Title != "Demo" {
		t.Fatalf("Title = %q", got.Title)
	}
	if got.GIF.Theme != "dracula" || got.GIF.Font != "Cascadia Mono" ||
		got.GIF.FontSize != 16 || got.GIF.LineHeight != 1.2 || got.GIF.Speed != 1.5 {
		t.Fatalf("GIF config = %#v", got.GIF)
	}

	output := stdout.String()
	if !strings.Contains(output, "Wrote demo.gif") || !strings.Contains(output, "Wrote demo.cast") {
		t.Fatalf("stdout = %q", output)
	}
}

func TestRecordCommandCreatesTemporaryCastWhenUnset(t *testing.T) {
	t.Parallel()

	var castPath string
	code := execute([]string{"record", "--binary", "demo-app", "--agg-path", "agg-bin"}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		look: func(path string) (string, error) {
			return path, nil
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			castPath = config.CastOutput
			if castPath == "" {
				t.Fatal("CastOutput is empty")
			}
			if _, err := os.Stat(castPath); err != nil {
				t.Fatalf("temp cast does not exist during run: %v", err)
			}
			return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	if _, err := os.Stat(castPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp cast stat err = %v, want not exist", err)
	}
}

func TestRecordCommandCastOnlyDoesNotRequireAgg(t *testing.T) {
	t.Parallel()

	var got record.Config
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--output", "",
		"--cast-output", "demo.cast",
	}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		look: func(path string) (string, error) {
			if path == "demo-app" {
				return path, nil
			}

			return "", execNotFoundError{}
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			got = config
			return record.Result{CastPath: config.CastOutput}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	if got.Output != "" {
		t.Fatalf("Output = %q, want empty", got.Output)
	}
}

func TestRecordCommandExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		look func(string) (string, error)
		run  func(context.Context, record.Config) (record.Result, error)
		want int
	}{
		{
			name: "usage missing binary",
			args: []string{"record"},
			want: exitUsage,
		},
		{
			name: "usage unknown command",
			args: []string{"unknown"},
			want: exitUsage,
		},
		{
			name: "usage unknown flag",
			args: []string{"record", "--unknown"},
			want: exitUsage,
		},
		{
			name: "usage unexpected arg",
			args: []string{"record", "extra"},
			want: exitUsage,
		},
		{
			name: "usage invalid keystrokes",
			args: []string{"record", "--binary", "demo-app", "--keystrokes", `abc\`},
			want: exitUsage,
		},
		{
			name: "missing prerequisite",
			args: []string{"record", "--binary", "demo-app"},
			look: func(string) (string, error) {
				return "", execNotFoundError{}
			},
			want: exitPrerequisite,
		},
		{
			name: "max duration",
			args: []string{"record", "--binary", "demo-app"},
			run: func(context.Context, record.Config) (record.Result, error) {
				return record.Result{}, record.ErrMaxDuration
			},
			want: exitMaxDuration,
		},
		{
			name: "gif validation",
			args: []string{"record", "--binary", "demo-app"},
			run: func(context.Context, record.Config) (record.Result, error) {
				return record.Result{}, gif.ErrValidation
			},
			want: exitValidation,
		},
		{
			name: "generic",
			args: []string{"record", "--binary", "demo-app"},
			run: func(context.Context, record.Config) (record.Result, error) {
				return record.Result{}, errors.New("boom")
			},
			want: exitGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			look := tt.look
			if look == nil {
				look = func(path string) (string, error) {
					return path, nil
				}
			}
			run := tt.run
			if run == nil {
				run = func(_ context.Context, config record.Config) (record.Result, error) {
					return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
				}
			}

			code := execute(tt.args, cliOptions{
				stdout: &bytes.Buffer{},
				stderr: &bytes.Buffer{},
				look:   look,
				run:    run,
			})
			if code != tt.want {
				t.Fatalf("execute code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestRecordHelpSnapshot(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	code := execute([]string{"record", "--help"}, cliOptions{
		stdout: stdout,
		stderr: &bytes.Buffer{},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	help := stdout.String()
	for _, want := range []string{
		"Record a terminal app",
		"--binary string",
		"--keystrokes string",
		"--cast-output string",
		"--agg-path string",
		"--max-duration int",
		"--line-height float",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

type execNotFoundError struct{}

func (execNotFoundError) Error() string {
	return "not found"
}
