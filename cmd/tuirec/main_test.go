package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gui-cs/tuirec/pkg/gif"
	"github.com/gui-cs/tuirec/pkg/record"
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
		"--input-delay", "1000",
		"--startup-delay", "1500",
		"--show-command", "PS> demo-app",
		"--show-command-delay", "15",
		"--show-command-hold", "250",
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
		"--verbosity", "high",
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
	if got.InputDelay.String() != "1s" {
		t.Fatalf("InputDelay = %s", got.InputDelay)
	}
	if got.StartupDelay.String() != "1.5s" {
		t.Fatalf("StartupDelay = %s", got.StartupDelay)
	}
	if got.ShowCommand != "PS> demo-app" {
		t.Fatalf("ShowCommand = %q", got.ShowCommand)
	}
	if got.CommandDelay.String() != "15ms" {
		t.Fatalf("CommandDelay = %s", got.CommandDelay)
	}
	if got.CommandHold.String() != "250ms" {
		t.Fatalf("CommandHold = %s", got.CommandHold)
	}
	if !got.Verbose {
		t.Fatal("Verbose = false, want true")
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

func TestRecordCommandAllowsZeroShowCommandPacing(t *testing.T) {
	t.Parallel()

	var got record.Config
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--show-command", "PS> demo-app",
		"--show-command-delay", "0",
		"--show-command-hold", "0",
	}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		look: func(path string) (string, error) {
			return path, nil
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			got = config
			return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}
	if got.CommandDelay != 0 {
		t.Fatalf("CommandDelay = %s, want 0", got.CommandDelay)
	}
	if got.CommandHold != 0 {
		t.Fatalf("CommandHold = %s, want 0", got.CommandHold)
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

func TestDefaultAggPathPrefersSiblingBinary(t *testing.T) {
	t.Parallel()

	executablePath := filepath.Join("release", "tuirec.exe")
	siblingAgg := filepath.Join("release", "agg.exe")
	toolsAgg := filepath.Join("tools", "agg.exe")

	got := defaultAggPathFor(
		func() (string, error) {
			return executablePath, nil
		},
		func(path string) (os.FileInfo, error) {
			switch path {
			case siblingAgg, toolsAgg:
				return fakeFileInfo{}, nil
			default:
				return nil, os.ErrNotExist
			}
		},
	)
	if got != siblingAgg {
		t.Fatalf("defaultAggPathFor() = %q, want %q", got, siblingAgg)
	}
}

func TestDefaultAggPathFallsBackToTools(t *testing.T) {
	t.Parallel()

	toolsAgg := filepath.Join("tools", "agg")
	got := defaultAggPathFor(
		func() (string, error) {
			return filepath.Join("release", "tuirec"), nil
		},
		func(path string) (os.FileInfo, error) {
			if path == toolsAgg {
				return fakeFileInfo{}, nil
			}

			return nil, os.ErrNotExist
		},
	)
	if got != toolsAgg {
		t.Fatalf("defaultAggPathFor() = %q, want %q", got, toolsAgg)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "agg" }
func (fakeFileInfo) Size() int64        { return 1 }
func (fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }

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
			name: "usage unknown key",
			args: []string{"record", "--binary", "demo-app", "--keystrokes", "Ctrl-Foo"},
			want: exitUsage,
		},
		{
			name: "usage invalid verbosity",
			args: []string{"record", "--binary", "demo-app", "--verbosity", "loud"},
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
		"Terminal.Gui Key strings",
		"Ctrl+Alt+Shift+CursorUp",
		"Ctrl+click:col:row",
		"--show-command string",
		"--startup-delay int",
		"--input-delay int",
		"--verbosity string",
		"--binary string",
		"--keystrokes string",
		"--cast-output string",
		"--agg-path string",
		"--max-duration int",
		"--line-height float",
		"--name string",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q:\n%s", want, help)
		}
	}
}

func TestOpenCLICommandPrintsDocument(t *testing.T) {
	t.Parallel()

	stdout := &bytes.Buffer{}
	code := execute([]string{"opencli"}, cliOptions{
		stdout: stdout,
		stderr: &bytes.Buffer{},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	var doc struct {
		OpenCLI string `json:"opencli"`
		Command struct {
			Name     string `json:"name"`
			Commands []struct {
				Name string `json:"name"`
			} `json:"commands"`
		} `json:"command"`
		Info struct {
			Title string `json:"title"`
		} `json:"info"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("opencli output is not valid json: %v\n%s", err, stdout.String())
	}

	if doc.OpenCLI != "0.1" {
		t.Fatalf("opencli version = %q, want %q", doc.OpenCLI, "0.1")
	}
	if doc.Command.Name != "tuirec" {
		t.Fatalf("command name = %q, want %q", doc.Command.Name, "tuirec")
	}
	if doc.Info.Title != "tuirec" {
		t.Fatalf("info title = %q, want %q", doc.Info.Title, "tuirec")
	}

	subcommands := map[string]bool{}
	for _, command := range doc.Command.Commands {
		subcommands[command.Name] = true
	}
	for _, expected := range []string{"record", "agent-guide", "opencli"} {
		if !subcommands[expected] {
			t.Fatalf("missing command %q in opencli output: %s", expected, stdout.String())
		}
	}
}

type execNotFoundError struct{}

func (execNotFoundError) Error() string {
	return "not found"
}

func TestRecordCommandNameFlag(t *testing.T) {
	t.Parallel()

	var got record.Config
	stderr := &bytes.Buffer{}
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--name", "my-demo",
		"--keystrokes", "wait:10,Ctrl+Q",
	}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: stderr,
		look: func(path string) (string, error) {
			return path, nil
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			got = config
			return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d; stderr: %s", code, exitSuccess, stderr.String())
	}

	wantOutput := filepath.Join("artifacts", "my-demo.gif")
	wantCast := filepath.Join("artifacts", "my-demo.cast")
	if got.Output != wantOutput {
		t.Fatalf("Output = %q, want %q", got.Output, wantOutput)
	}
	if got.CastOutput != wantCast {
		t.Fatalf("CastOutput = %q, want %q", got.CastOutput, wantCast)
	}
}

func TestRecordCommandNameFlagExplicitOutputOverrides(t *testing.T) {
	t.Parallel()

	var got record.Config
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--name", "my-demo",
		"--output", "custom.gif",
		"--cast-output", "custom.cast",
	}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		look: func(path string) (string, error) {
			return path, nil
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			got = config
			return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	if got.Output != "custom.gif" {
		t.Fatalf("Output = %q, want %q", got.Output, "custom.gif")
	}
	if got.CastOutput != "custom.cast" {
		t.Fatalf("CastOutput = %q, want %q", got.CastOutput, "custom.cast")
	}
}

func TestRecordCommandPrintsSummary(t *testing.T) {
	t.Parallel()

	stderr := &bytes.Buffer{}
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--name", "summary-test",
		"--verbosity", "normal",
	}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: stderr,
		look: func(path string) (string, error) {
			return path, nil
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	output := stderr.String()
	for _, want := range []string{"Recording:", "Binary:", "Keystrokes:", "Output:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stderr missing %q:\n%s", want, output)
		}
	}
}

func TestRecordCommandQuietSuppressesSummary(t *testing.T) {
	t.Parallel()

	stderr := &bytes.Buffer{}
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--verbosity", "quiet",
	}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: stderr,
		look: func(path string) (string, error) {
			return path, nil
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			return record.Result{CastPath: config.CastOutput, GIFPath: config.Output}, nil
		},
	})
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d", code, exitSuccess)
	}

	if strings.Contains(stderr.String(), "Recording:") {
		t.Fatalf("quiet mode should suppress summary, got: %s", stderr.String())
	}
}

func TestRecordCommandAutoDownloadAgg(t *testing.T) {
	t.Parallel()

	// Simulate agg not found via look, but verify resolveAgg tries the cache.
	stderr := &bytes.Buffer{}
	code := execute([]string{
		"record",
		"--binary", "demo-app",
		"--output", "",
		"--cast-output", "demo.cast",
	}, cliOptions{
		stdout: &bytes.Buffer{},
		stderr: stderr,
		look: func(path string) (string, error) {
			if path == "demo-app" {
				return path, nil
			}
			return "", execNotFoundError{}
		},
		run: func(_ context.Context, config record.Config) (record.Result, error) {
			return record.Result{CastPath: config.CastOutput}, nil
		},
	})
	// With output="" (cast-only mode), agg is not needed.
	if code != exitSuccess {
		t.Fatalf("execute code = %d, want %d; stderr: %s", code, exitSuccess, stderr.String())
	}
}
