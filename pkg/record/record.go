// Package record orchestrates PTY capture, input playback, and GIF rendering.
package record

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gui-cs/TUIcast/pkg/gif"
	"github.com/gui-cs/TUIcast/pkg/keystroke"
	"github.com/gui-cs/TUIcast/pkg/pty"
	"github.com/gui-cs/TUIcast/pkg/recorder"
)

const (
	// CommandName is the CLI subcommand for recording a terminal app.
	CommandName = "record"

	defaultKeystrokes     = "wait:3000,Ctrl+C"
	defaultKeystrokeDelay = 200 * time.Millisecond
	defaultMaxDuration    = 60 * time.Second
	defaultDrainDuration  = 500 * time.Millisecond
)

// ErrMaxDuration indicates that recording hit the configured max duration.
var ErrMaxDuration = errors.New("recording hit max duration")

var startPTY = pty.Start

// Renderer renders a cast file to a GIF.
type Renderer interface {
	Render(context.Context, string, string, gif.Config) error
}

// Config configures one recording run.
type Config struct {
	Binary         string
	Args           []string
	Dir            string
	Env            []string
	Size           pty.Size
	CastOutput     string
	Output         string
	Title          string
	Keystrokes     string
	KeystrokeDelay time.Duration
	MaxDuration    time.Duration
	DrainDuration  time.Duration
	Clock          recorder.Clock
	Timestamp      time.Time
	GIF            gif.Config
	Renderer       Renderer
}

// Result describes files produced by a recording run.
type Result struct {
	CastPath string
	GIFPath  string
}

type gifRenderer struct{}

type contextWriter struct {
	ctx    context.Context
	writer io.Writer
}

type contextSleeper struct {
	ctx context.Context
}

type stopSource int

const (
	sourcePlayer stopSource = iota
	sourceRead
	sourceWait
	sourceContext
)

// Run executes one recording pipeline.
func Run(parent context.Context, config Config) (Result, error) {
	config = normalizeConfig(config)
	if config.Binary == "" {
		return Result{}, fmt.Errorf("binary is required")
	}
	if config.CastOutput == "" {
		return Result{}, fmt.Errorf("cast output is required")
	}
	actions, err := keystroke.Parse(config.Keystrokes)
	if err != nil {
		return Result{}, err
	}

	ctx, cancel := context.WithTimeout(parent, config.MaxDuration)
	defer cancel()

	castFile, err := os.Create(config.CastOutput)
	if err != nil {
		return Result{}, fmt.Errorf("create cast output: %w", err)
	}
	defer castFile.Close()

	castRecorder, err := recorder.New(castFile, recorder.Config{
		Width:     config.Size.Cols,
		Height:    config.Size.Rows,
		Timestamp: config.Timestamp,
		Title:     config.Title,
		Env:       map[string]string{"TERM": "xterm-256color"},
		Clock:     config.Clock,
	})
	if err != nil {
		return Result{}, err
	}
	defer castRecorder.Close()

	session, err := startPTY(config.Binary, config.Args, config.Size, pty.Options{
		Dir: config.Dir,
		Env: config.Env,
	})
	if err != nil {
		return Result{}, fmt.Errorf("start pty: %w", err)
	}
	defer session.Close()

	readDone := make(chan error, 1)
	go copyPTY(session, castRecorder, readDone)

	playerDone := make(chan error, 1)
	go playKeystrokes(ctx, session, actions, config.KeystrokeDelay, playerDone)

	waitDone := make(chan error, 1)
	go waitSession(ctx, session, waitDone)

	runErr, source := waitForStop(ctx, playerDone, readDone, waitDone, config.DrainDuration)
	cancel()
	if err := session.Close(); err != nil && runErr == nil {
		runErr = fmt.Errorf("close session: %w", err)
	}

	if source != sourceRead {
		if err := <-readDone; err != nil && runErr == nil {
			runErr = err
		}
	}

	if source != sourcePlayer {
		<-playerDone
	}

	if err := castRecorder.Close(); err != nil && runErr == nil {
		runErr = err
	}

	if err := castFile.Close(); err != nil && runErr == nil {
		runErr = fmt.Errorf("close cast output: %w", err)
	}

	result := Result{CastPath: config.CastOutput}
	if runErr != nil {
		return result, runErr
	}

	if config.Output == "" {
		return result, nil
	}

	if err := config.Renderer.Render(parent, config.CastOutput, config.Output, config.GIF); err != nil {
		return result, err
	}

	result.GIFPath = config.Output

	return result, nil
}

func normalizeConfig(config Config) Config {
	if config.Keystrokes == "" {
		config.Keystrokes = defaultKeystrokes
	}

	if config.KeystrokeDelay <= 0 {
		config.KeystrokeDelay = defaultKeystrokeDelay
	}

	if config.MaxDuration <= 0 {
		config.MaxDuration = defaultMaxDuration
	}

	if config.DrainDuration <= 0 {
		config.DrainDuration = defaultDrainDuration
	}

	if config.DrainDuration < config.KeystrokeDelay {
		config.DrainDuration = config.KeystrokeDelay
	}

	if config.Timestamp.IsZero() {
		config.Timestamp = time.Now()
	}

	if config.Clock == nil {
		config.Clock = recorder.NewWallClock()
	}

	if config.Renderer == nil {
		config.Renderer = gifRenderer{}
	}

	return config
}

func copyPTY(reader io.Reader, writer io.Writer, done chan<- error) {
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			if _, writeErr := writer.Write(buffer[:n]); writeErr != nil {
				done <- writeErr

				return
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				done <- nil
			} else {
				done <- fmt.Errorf("read pty: %w", err)
			}

			return
		}
	}
}

func playKeystrokes(ctx context.Context, writer io.Writer, actions []keystroke.Action, keystrokeDelay time.Duration, done chan<- error) {
	player := keystroke.NewPlayer(contextWriter{ctx: ctx, writer: writer}, contextSleeper{ctx: ctx}, keystrokeDelay)
	done <- player.PlayActions(actions)
}

func waitSession(ctx context.Context, session pty.Session, done chan<- error) {
	_, err := session.Wait(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		done <- ErrMaxDuration

		return
	}

	if errors.Is(err, context.Canceled) {
		done <- ctx.Err()

		return
	}

	done <- err
}

func waitForStop(ctx context.Context, playerDone, readDone, waitDone <-chan error, drainDuration time.Duration) (error, stopSource) {
	select {
	case err := <-playerDone:
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return ErrMaxDuration, sourcePlayer
			}

			return err, sourcePlayer
		}
		return drain(ctx, drainDuration), sourcePlayer
	case err := <-readDone:
		return err, sourceRead
	case err := <-waitDone:
		return err, sourceWait
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrMaxDuration, sourceContext
		}

		return ctx.Err(), sourceContext
	}
}

func drain(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ErrMaxDuration
		}

		return ctx.Err()
	}
}

func (r gifRenderer) Render(ctx context.Context, castPath, outputPath string, config gif.Config) error {
	if err := gif.Render(ctx, castPath, outputPath, config); err != nil {
		return err
	}

	if _, err := gif.Validate(outputPath); err != nil {
		return err
	}

	return nil
}

func (w contextWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
		return w.writer.Write(p)
	}
}

func (s contextSleeper) Sleep(duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-s.ctx.Done():
	}
}
