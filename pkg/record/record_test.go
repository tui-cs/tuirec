package record

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gui-cs/TUIcast/pkg/gif"
	"github.com/gui-cs/TUIcast/pkg/pty"
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
	mu     sync.Mutex
	closed chan struct{}
	once   sync.Once
	input  []byte
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
