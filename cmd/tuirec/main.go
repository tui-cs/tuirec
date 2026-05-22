package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gui-cs/tuirec/pkg/gif"
	"github.com/gui-cs/tuirec/pkg/keystroke"
	"github.com/gui-cs/tuirec/pkg/pointer"
	"github.com/gui-cs/tuirec/pkg/pty"
	"github.com/gui-cs/tuirec/pkg/record"
	"github.com/spf13/cobra"
)

//go:generate go run ../../internal/tools/syncfile.go ../../agent/RECORDING-AGENT.md agent-guide.md

//go:embed agent-guide.md
var agentGuide string

const (
	exitSuccess = iota
	exitGeneric
	exitUsage
	exitPrerequisite
	exitMaxDuration
	exitValidation
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	// When installed via `go install`, ldflags aren't set. Fall back to
	// the build info embedded by the Go toolchain.
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 && commit == "none" {
				commit = s.Value[:7]
			}
		case "vcs.time":
			if date == "unknown" {
				date = s.Value
			}
		}
	}
}

type cliError struct {
	code int
	err  error
}

type cliOptions struct {
	stdout io.Writer
	stderr io.Writer
	run    func(context.Context, record.Config) (record.Result, error)
	look   func(string) (string, error)
}

type recordFlags struct {
	config             record.Config
	args               []string
	name               string
	outputExplicit     bool
	castOutputExplicit bool
	keystrokeDelayMS   int
	inputDelayMS       int
	startupDelayMS     int
	showCommandDelayMS int
	showCommandHoldMS  int
	maxDurationSec     int
	drainMS            int
	kittyKeyboard      bool
	mousePointer       string
	pointerStyle       string
	openGIF            bool
	copyPath           bool
	verbosity          string
}

func main() {
	os.Exit(execute(os.Args[1:], cliOptions{
		stdout: os.Stdout,
		stderr: os.Stderr,
		run:    record.Run,
		look:   exec.LookPath,
	}))
}

func execute(args []string, options cliOptions) int {
	options = normalizeOptions(options)
	root := newRootCommand(options)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		err = classifyCommandError(err)
		fmt.Fprintln(options.stderr, err)

		return exitCode(err)
	}

	return exitSuccess
}

func normalizeOptions(options cliOptions) cliOptions {
	if options.stdout == nil {
		options.stdout = io.Discard
	}
	if options.stderr == nil {
		options.stderr = io.Discard
	}
	if options.run == nil {
		options.run = record.Run
	}
	if options.look == nil {
		options.look = exec.LookPath
	}

	return options
}

func newRootCommand(options cliOptions) *cobra.Command {
	root := &cobra.Command{
		Use:           "tuirec",
		Short:         "Record terminal apps and produce animated GIFs",
		Long:          "tuirec records terminal application sessions and renders them as animated GIFs.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetOut(options.stdout)
	root.SetErr(options.stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err)
	})
	root.SetVersionTemplate("tuirec {{.Version}}\n")
	root.Version = fmt.Sprintf("%s (%s, %s)", version, commit, date)
	root.AddCommand(newRecordCommand(options))
	root.AddCommand(newAgentGuideCommand(options))

	return root
}

func newRecordCommand(options cliOptions) *cobra.Command {
	flags := defaultRecordFlags()
	cmd := &cobra.Command{
		Use:   record.CommandName,
		Short: "Record a terminal app",
		Long: `Record a terminal app.

Keystroke tokens are comma-separated. Use wait:<ms> for delays, click:col:row
or move:col:row (for hover) for SGR mouse events, modifier mouse tokens like
Ctrl+click:col:row, literal text for typed text, and Terminal.Gui Key strings
for named keys. Key strings use the same format as Terminal.Gui Key.ToString()
and Key.TryParse(), such as Ctrl+C, Ctrl-C, A-Ctrl, Shift+Tab,
Ctrl+Alt+Shift+CursorUp, Esc, Enter, Delete, and F1.

Use --show-command to type a synthetic shell prompt/command into the GIF before
the target starts. Use --startup-delay to wait before copying target output,
--input-delay to pace scripted input, and --verbosity high to log keys and
pacing to stderr.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return usageError(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags.outputExplicit = cmd.Flags().Changed("output")
			flags.castOutputExplicit = cmd.Flags().Changed("cast-output")
			return runRecord(cmd.Context(), options, flags)
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err)
	})

	cmd.Flags().StringVar(&flags.config.Binary, "binary", "", "Path to executable to record")
	cmd.Flags().StringSliceVar(&flags.args, "args", nil, "Arguments to pass to the binary")
	cmd.Flags().StringVar(&flags.config.Output, "output", flags.config.Output, "Output GIF path")
	cmd.Flags().StringVar(&flags.config.CastOutput, "cast-output", "", "Also save the raw asciinema cast file")
	cmd.Flags().StringVar(&flags.name, "name", "", "Short name for the recording; sets --output artifacts/<name>.gif and --cast-output artifacts/<name>.cast unless explicitly set")
	cmd.Flags().StringVar(&flags.config.Keystrokes, "keystrokes", flags.config.Keystrokes, "Comma-separated script: wait:<ms>, click:col:row, Ctrl+click:col:row, move:col:row, backtick-quoted literal text, or Terminal.Gui Key strings")
	cmd.Flags().IntVar(&flags.keystrokeDelayMS, "keystroke-delay", flags.keystrokeDelayMS, "Default pause between keystrokes in milliseconds")
	cmd.Flags().IntVar(&flags.inputDelayMS, "input-delay", flags.inputDelayMS, "Milliseconds to wait before playing the keystroke script")
	cmd.Flags().IntVar(&flags.startupDelayMS, "startup-delay", flags.startupDelayMS, "Milliseconds to wait after starting the target before copying output and key input")
	cmd.Flags().StringVar(&flags.config.ShowCommand, "show-command", "", "Synthetic shell command line to type into the GIF before the target starts")
	cmd.Flags().IntVar(&flags.showCommandDelayMS, "show-command-delay", flags.showCommandDelayMS, "Milliseconds between typed show-command characters")
	cmd.Flags().IntVar(&flags.showCommandHoldMS, "show-command-hold", flags.showCommandHoldMS, "Milliseconds to hold after show-command Enter before target starts")
	cmd.Flags().IntVar(&flags.config.Size.Cols, "cols", flags.config.Size.Cols, "Terminal columns")
	cmd.Flags().IntVar(&flags.config.Size.Rows, "rows", flags.config.Size.Rows, "Terminal rows")
	cmd.Flags().StringVar(&flags.config.GIF.Theme, "theme", flags.config.GIF.Theme, "agg color theme")
	cmd.Flags().StringVar(&flags.config.GIF.Font, "font", "", "Font family for agg; omit for agg default")
	cmd.Flags().IntVar(&flags.config.GIF.FontSize, "font-size", flags.config.GIF.FontSize, "Font size in pixels")
	cmd.Flags().Float64Var(&flags.config.GIF.LineHeight, "line-height", flags.config.GIF.LineHeight, "Line-height multiplier")
	cmd.Flags().Float64Var(&flags.config.GIF.LetterSpacing, "letter-spacing", flags.config.GIF.LetterSpacing, "Letter-spacing adjustment in pixels (negative closes gaps)")
	cmd.Flags().Float64Var(&flags.config.GIF.Speed, "speed", flags.config.GIF.Speed, "GIF playback speed multiplier")
	cmd.Flags().IntVar(&flags.maxDurationSec, "max-duration", flags.maxDurationSec, "Max recording duration in seconds")
	cmd.Flags().StringVar(&flags.config.Title, "title", "", "Title embedded in the cast file")
	cmd.Flags().StringVar(&flags.config.GIF.AggPath, "agg-path", flags.config.GIF.AggPath, "Path to agg binary")
	cmd.Flags().IntVar(&flags.drainMS, "drain", flags.drainMS, "Milliseconds to keep recording after keystrokes finish")
	cmd.Flags().StringVar(&flags.verbosity, "verbosity", flags.verbosity, "Output verbosity: quiet, normal, or high")
	cmd.Flags().BoolVar(&flags.kittyKeyboard, "kitty-keyboard", false, "Enable Kitty keyboard protocol: encode keystrokes as CSI u and respond to app mode queries")
	cmd.Flags().StringVar(&flags.mousePointer, "mouse-pointer", "clicks", "Mouse pointer indicator mode: none, clicks, or all")
	cmd.Flags().StringVar(&flags.pointerStyle, "pointer-style", "●", "Unicode character to display as the mouse pointer indicator")
	cmd.Flags().BoolVar(&flags.config.Inline, "inline", false, "Record an inline-mode app: skip alternate screen so prompt and app output share the normal screen buffer")
	cmd.Flags().BoolVar(&flags.openGIF, "open", false, "Open the GIF in the default viewer after recording")
	cmd.Flags().BoolVar(&flags.copyPath, "copy", false, "Copy the GIF file path to the system clipboard after recording")

	return cmd
}

func newAgentGuideCommand(options cliOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "agent-guide",
		Short: "Print the AI agent recording guide to stdout",
		Long:  "Prints the full tuirec recording agent guide (keystroke syntax, best practices, examples) for use by AI coding agents.",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Fprint(options.stdout, agentGuide)
			return nil
		},
	}
}

func defaultRecordFlags() *recordFlags {
	return &recordFlags{
		config: record.Config{
			Output:     "recording.gif",
			Keystrokes: "wait:3000,Ctrl+C",
			Size:       pty.Size{Cols: 120, Rows: 30},
			GIF: gif.Config{
				AggPath:    defaultAggPath(),
				Theme:      "monokai",
				Speed:      1.0,
				FontSize:   14,
				LineHeight: 1.3,
			},
		},
		keystrokeDelayMS:   200,
		showCommandDelayMS: 35,
		showCommandHoldMS:  500,
		maxDurationSec:     60,
		drainMS:            500,
		verbosity:          "normal",
	}
}

func runRecord(ctx context.Context, options cliOptions, flags *recordFlags) error {
	if flags.config.Binary == "" {
		return usageError(fmt.Errorf("--binary is required"))
	}

	if _, err := keystroke.Parse(flags.config.Keystrokes); err != nil {
		return usageError(err)
	}
	if err := validateVerbosity(flags.verbosity); err != nil {
		return usageError(err)
	}

	mouseMode, err := pointer.ParseMode(flags.mousePointer)
	if err != nil {
		return usageError(err)
	}

	binary, err := options.look(flags.config.Binary)
	if err != nil {
		return prerequisiteError(fmt.Errorf("find target binary %q: %w", flags.config.Binary, err))
	}

	config := flags.config
	config.Binary = binary
	config.Args = flags.args
	config.KeystrokeDelay = time.Duration(flags.keystrokeDelayMS) * time.Millisecond
	config.InputDelay = time.Duration(flags.inputDelayMS) * time.Millisecond
	config.StartupDelay = time.Duration(flags.startupDelayMS) * time.Millisecond
	config.CommandDelay = time.Duration(flags.showCommandDelayMS) * time.Millisecond
	config.CommandHold = time.Duration(flags.showCommandHoldMS) * time.Millisecond
	config.MaxDuration = time.Duration(flags.maxDurationSec) * time.Second
	config.DrainDuration = time.Duration(flags.drainMS) * time.Millisecond
	config.KittyKeyboard = flags.kittyKeyboard
	config.MousePointer = mouseMode
	config.PointerStyle = flags.pointerStyle
	config.Verbose = flags.verbosity == "high"
	config.LogWriter = options.stderr

	// Apply --name defaults when explicit paths are not set.
	if flags.name != "" {
		if !flags.outputExplicit {
			config.Output = filepath.Join("artifacts", flags.name+".gif")
		}
		if !flags.castOutputExplicit {
			config.CastOutput = filepath.Join("artifacts", flags.name+".cast")
		}
	}

	if config.Output != "" {
		aggPath, err := resolveAgg(config.GIF.AggPath, options.look, options.stderr, flags.verbosity)
		if err != nil {
			return prerequisiteError(fmt.Errorf("find agg binary: %w", err))
		}
		config.GIF.AggPath = aggPath
	}

	// Print recording summary.
	if flags.verbosity != "quiet" {
		fmt.Fprintf(options.stderr, "Recording:\n")
		fmt.Fprintf(options.stderr, "  Binary:     %s\n", config.Binary)
		fmt.Fprintf(options.stderr, "  Keystrokes: %s\n", config.Keystrokes)
		if config.Output != "" {
			fmt.Fprintf(options.stderr, "  Output:     %s\n", config.Output)
		}
		if config.CastOutput != "" {
			fmt.Fprintf(options.stderr, "  Cast:       %s\n", config.CastOutput)
		}
	}

	// Ensure output directories exist.
	if config.Output != "" {
		if dir := filepath.Dir(config.Output); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create output directory: %w", err)
			}
		}
	}
	if config.CastOutput != "" {
		if dir := filepath.Dir(config.CastOutput); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create cast output directory: %w", err)
			}
		}
	}

	cleanupCast := false
	if config.CastOutput == "" {
		castFile, err := os.CreateTemp("", "tuirec-*.cast")
		if err != nil {
			return err
		}
		if err := castFile.Close(); err != nil {
			return err
		}
		config.CastOutput = castFile.Name()
		cleanupCast = true
	}
	if cleanupCast {
		defer os.Remove(config.CastOutput)
	}

	result, err := options.run(ctx, config)
	if err != nil {
		return classifyRunError(err)
	}

	if flags.verbosity != "quiet" && result.GIFPath != "" {
		fmt.Fprintf(options.stdout, "Wrote %s\n", result.GIFPath)
	}
	if flags.verbosity != "quiet" && !cleanupCast && result.CastPath != "" {
		fmt.Fprintf(options.stdout, "Wrote %s\n", result.CastPath)
	}

	if result.GIFPath != "" {
		absGIF, _ := filepath.Abs(result.GIFPath)
		if absGIF == "" {
			absGIF = result.GIFPath
		}

		if flags.copyPath {
			if err := copyToClipboard(absGIF); err != nil && flags.verbosity != "quiet" {
				fmt.Fprintf(options.stderr, "Warning: could not copy to clipboard: %v\n", err)
			} else if flags.verbosity != "quiet" {
				fmt.Fprintf(options.stderr, "Copied path to clipboard\n")
			}
		}
		if flags.openGIF {
			if err := openFile(absGIF); err != nil && flags.verbosity != "quiet" {
				fmt.Fprintf(options.stderr, "Warning: could not open GIF: %v\n", err)
			}
		}
	}

	return nil
}

func validateVerbosity(verbosity string) error {
	switch verbosity {
	case "quiet", "normal", "high":
		return nil
	default:
		return fmt.Errorf("invalid --verbosity %q: use quiet, normal, or high", verbosity)
	}
}

// resolveAgg locates the agg binary: first via LookPath, then via cache,
// and finally by downloading it.
func resolveAgg(aggPath string, look func(string) (string, error), stderr io.Writer, verbosity string) (string, error) {
	// Try the configured path via LookPath.
	if resolved, err := look(aggPath); err == nil {
		return resolved, nil
	}

	// Check if already cached.
	cached := gif.CachedAggPath()
	if _, err := os.Stat(cached); err == nil {
		return cached, nil
	}

	// Auto-download.
	if verbosity != "quiet" {
		fmt.Fprintf(stderr, "agg not found; downloading %s...\n", gif.DefaultAggVersion)
	}
	downloaded, err := gif.DownloadAgg()
	if err != nil {
		return "", err
	}
	if verbosity != "quiet" {
		fmt.Fprintf(stderr, "agg downloaded to %s\n", downloaded)
	}
	return downloaded, nil
}

func defaultAggPath() string {
	return defaultAggPathFor(os.Executable, os.Stat)
}

func defaultAggPathFor(executable func() (string, error), stat func(string) (os.FileInfo, error)) string {
	if executablePath, err := executable(); err == nil {
		executableDir := filepath.Dir(executablePath)
		for _, candidate := range []string{
			filepath.Join(executableDir, "agg.exe"),
			filepath.Join(executableDir, "agg"),
		} {
			if _, err := stat(candidate); err == nil {
				return candidate
			}
		}
	}

	for _, candidate := range []string{
		filepath.Join("tools", "agg.exe"),
		filepath.Join("tools", "agg"),
	} {
		if _, err := stat(candidate); err == nil {
			return candidate
		}
	}

	return "agg"
}

func usageError(err error) error {
	return cliError{code: exitUsage, err: err}
}

func prerequisiteError(err error) error {
	return cliError{code: exitPrerequisite, err: err}
}

func classifyRunError(err error) error {
	switch {
	case errors.Is(err, record.ErrMaxDuration):
		return cliError{code: exitMaxDuration, err: err}
	case errors.Is(err, gif.ErrValidation):
		return cliError{code: exitValidation, err: err}
	default:
		return err
	}
}

func classifyCommandError(err error) error {
	var coded cliError
	if errors.As(err, &coded) {
		return err
	}

	if strings.HasPrefix(err.Error(), "unknown command ") {
		return usageError(err)
	}

	return err
}

func exitCode(err error) int {
	var coded cliError
	if errors.As(err, &coded) {
		return coded.code
	}

	return exitGeneric
}

func (e cliError) Error() string {
	return e.err.Error()
}

func (e cliError) Unwrap() error {
	return e.err
}
