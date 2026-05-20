package record

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gui-cs/tuirec/pkg/gif"
	"github.com/gui-cs/tuirec/pkg/pty"
	"github.com/gui-cs/tuirec/pkg/recorder"
)

func TestRunRecordsKeystrokesAndRenders(t *testing.T) {
	originalStartPTY := startPTY
	fakeSession := newFakeSession()
	startPTY = func(binary string, args []string, size pty.Size, options pty.Options) (pty.Session, error) {
		if binary != "fake-app" {
			t.Fatalf("binary = %q, want fake-app", binary)
		}
		return fakeSession, nil
	}
	defer func() {
		startPTY = originalStartPTY
	}()

	renderer := &fakeRenderer{}
	castPath := filepath.Join(t.TempDir(), "recording.cast")

	result, err := Run(context.Background(), Config{
		Binary:         "fake-app",
		CastOutput:     castPath,
		Output:         filepath.Join(t.TempDir(), "recording.gif"),
		Keystrokes:     "Ctrl+Q",
		KeystrokeDelay: time.Millisecond,
		DrainDuration:  time.Millisecond,
		MaxDuration:    time.Second,
		Renderer:       renderer,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if result.CastPath != castPath {
		t.Fatalf("CastPath = %q, want %q", result.CastPath, castPath)
	}

	if got := fakeSession.written(); got != "\x11" {
		t.Fatalf("written keystrokes = %q, want Ctrl+Q", got)
	}

	if renderer.castPath != castPath {
		t.Fatalf("renderer castPath = %q, want %q", renderer.castPath, castPath)
	}
}

func TestRunWritesCommandPreRollToCast(t *testing.T) {
	originalStartPTY := startPTY
	fakeSession := newFakeSession()
	startPTY = func(string, []string, pty.Size, pty.Options) (pty.Session, error) {
		return fakeSession, nil
	}
	defer func() {
		startPTY = originalStartPTY
	}()

	castPath := filepath.Join(t.TempDir(), "recording.cast")
	clock := recorder.NewScriptedClock()
	_, err := Run(context.Background(), Config{
		Binary:         "fake-app",
		CastOutput:     castPath,
		Keystrokes:     "Ctrl+Q",
		KeystrokeDelay: time.Millisecond,
		DrainDuration:  time.Millisecond,
		MaxDuration:    time.Second,
		Clock:          clock,
		ShowCommand:    "PS> ted foo.cs",
		CommandDelay:   10 * time.Millisecond,
		CommandHold:    20 * time.Millisecond,
		Renderer:       &fakeRenderer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cast, err := os.ReadFile(castPath)
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}

	got := string(cast)
	for _, want := range []string{`"P"`, `"S"`, `"\u003e"`, `"t"`, `"\r\n"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("cast missing %q:\n%s", want, got)
		}
	}
}

func TestRunHighVerbosityLogsPacing(t *testing.T) {
	originalStartPTY := startPTY
	fakeSession := newFakeSession()
	startPTY = func(string, []string, pty.Size, pty.Options) (pty.Session, error) {
		return fakeSession, nil
	}
	defer func() {
		startPTY = originalStartPTY
	}()

	var log bytes.Buffer
	_, err := Run(context.Background(), Config{
		Binary:         "fake-app",
		CastOutput:     filepath.Join(t.TempDir(), "recording.cast"),
		Keystrokes:     "Ctrl+Q",
		KeystrokeDelay: time.Millisecond,
		StartupDelay:   time.Millisecond,
		InputDelay:     2 * time.Millisecond,
		DrainDuration:  time.Millisecond,
		MaxDuration:    time.Second,
		Clock:          recorder.NewScriptedClock(),
		ShowCommand:    "PS> fake-app",
		CommandDelay:   time.Millisecond,
		CommandHold:    time.Millisecond,
		Renderer:       &fakeRenderer{},
		Verbose:        true,
		LogWriter:      &log,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := log.String()
	for _, want := range []string{
		`tuirec: show command "PS> fake-app"`,
		"tuirec: startup delay 1ms",
		"tuirec: input delay 2ms",
		"tuirec: key Ctrl+Q",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log missing %q:\n%s", want, got)
		}
	}
}

func TestRunRendersAfterNonZeroChildExit(t *testing.T) {
	originalStartPTY := startPTY
	fakeSession := newFakeSession()
	fakeSession.waitStatus = pty.ExitStatus{Code: 130}
	fakeSession.waitErr = errors.New("wait process: exit status 130")
	startPTY = func(string, []string, pty.Size, pty.Options) (pty.Session, error) {
		return fakeSession, nil
	}
	defer func() {
		startPTY = originalStartPTY
	}()

	renderer := &fakeRenderer{}
	castPath := filepath.Join(t.TempDir(), "recording.cast")

	_, err := Run(context.Background(), Config{
		Binary:         "fake-app",
		CastOutput:     castPath,
		Output:         filepath.Join(t.TempDir(), "recording.gif"),
		Keystrokes:     "Ctrl+C",
		KeystrokeDelay: time.Millisecond,
		DrainDuration:  time.Millisecond,
		MaxDuration:    time.Second,
		Renderer:       renderer,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if renderer.castPath != castPath {
		t.Fatalf("renderer castPath = %q, want %q", renderer.castPath, castPath)
	}
}

func TestRunRequiresBinaryAndCastOutput(t *testing.T) {
	t.Parallel()

	if _, err := Run(context.Background(), Config{CastOutput: "out.cast"}); err == nil {
		t.Fatal("Run without binary err = nil, want error")
	}

	if _, err := Run(context.Background(), Config{Binary: "app"}); err == nil {
		t.Fatal("Run without cast output err = nil, want error")
	}
}

func TestRunValidatesKeystrokesBeforeStartingPTY(t *testing.T) {
	originalStartPTY := startPTY
	startPTY = func(string, []string, pty.Size, pty.Options) (pty.Session, error) {
		t.Fatal("startPTY called before validating keystrokes")

		return nil, nil
	}
	defer func() {
		startPTY = originalStartPTY
	}()

	_, err := Run(context.Background(), Config{
		Binary:     "fake-app",
		CastOutput: filepath.Join(t.TempDir(), "recording.cast"),
		Keystrokes: "click:0:1",
	})
	if err == nil {
		t.Fatal("Run err = nil, want invalid keystroke error")
	}
}

func TestRunReportsMaxDuration(t *testing.T) {
	originalStartPTY := startPTY
	startPTY = func(string, []string, pty.Size, pty.Options) (pty.Session, error) {
		return newFakeSession(), nil
	}
	defer func() {
		startPTY = originalStartPTY
	}()

	_, err := Run(context.Background(), Config{
		Binary:         "fake-app",
		CastOutput:     filepath.Join(t.TempDir(), "recording.cast"),
		Keystrokes:     "wait:1000",
		KeystrokeDelay: time.Millisecond,
		DrainDuration:  time.Millisecond,
		MaxDuration:    time.Millisecond,
	})
	if !errors.Is(err, ErrMaxDuration) {
		t.Fatalf("Run err = %v, want ErrMaxDuration", err)
	}
}

func TestWaitForStopMapsPlayerDeadlineToMaxDuration(t *testing.T) {
	t.Parallel()

	playerDone := make(chan error, 1)
	playerDone <- context.DeadlineExceeded

	readDone := make(chan error)
	waitDone := make(chan error)

	err, source := waitForStop(context.Background(), playerDone, readDone, waitDone, time.Second)
	if !errors.Is(err, ErrMaxDuration) {
		t.Fatalf("waitForStop err = %v, want ErrMaxDuration", err)
	}

	if source != sourcePlayer {
		t.Fatalf("source = %d, want sourcePlayer", source)
	}
}

func TestWaitSessionPreservesDeadlineWithNonZeroStatus(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	done := make(chan error, 1)
	fakeSession := newFakeSession()
	fakeSession.waitStatus = pty.ExitStatus{Code: 259}
	fakeSession.waitErr = errors.New("wait canceled")

	waitSession(ctx, fakeSession, done)

	err := <-done
	if !errors.Is(err, ErrMaxDuration) {
		t.Fatalf("waitSession err = %v, want ErrMaxDuration", err)
	}
}

type fakeRenderer struct {
	castPath string
}

func (r *fakeRenderer) Render(_ context.Context, castPath, outputPath string, _ gif.Config) error {
	if _, err := os.Stat(castPath); err != nil {
		return err
	}

	r.castPath = castPath

	return os.WriteFile(outputPath, []byte("gif"), 0o600)
}

type fakeSession struct {
	mu         sync.Mutex
	closed     chan struct{}
	once       sync.Once
	input      []byte
	waitStatus pty.ExitStatus
	waitErr    error
}

func newFakeSession() *fakeSession {
	return &fakeSession{closed: make(chan struct{})}
}

func (s *fakeSession) Read(_ []byte) (int, error) {
	<-s.closed

	return 0, io.EOF
}

func (s *fakeSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.closed:
		return 0, errors.New("session is closed")
	default:
	}

	s.input = append(s.input, p...)

	return len(p), nil
}

func (s *fakeSession) Close() error {
	s.once.Do(func() {
		close(s.closed)
	})

	return nil
}

func (s *fakeSession) Pid() int {
	return 1
}

func (s *fakeSession) Resize(pty.Size) error {
	return nil
}

func (s *fakeSession) Wait(ctx context.Context) (pty.ExitStatus, error) {
	if s.waitErr != nil {
		return s.waitStatus, s.waitErr
	}

	select {
	case <-s.closed:
		return pty.ExitStatus{}, nil
	case <-ctx.Done():
		return pty.ExitStatus{}, ctx.Err()
	}
}

func (s *fakeSession) written() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return string(s.input)
}
