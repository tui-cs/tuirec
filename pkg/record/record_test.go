package record

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
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

const floatTolerance = 1e-6

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

func TestRunInlineModeOmitsAlternateScreen(t *testing.T) {
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
		ShowCommand:    "PS> inline-app",
		CommandDelay:   10 * time.Millisecond,
		CommandHold:    20 * time.Millisecond,
		Inline:         true,
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
	// Inline mode must NOT contain the alternate screen escape.
	// The cast file is JSON-encoded, so ESC (0x1B) appears as literal
	// text \u001b in the file — match that form.
	altScreen := `\u001b[?1049h`
	if strings.Contains(got, altScreen) {
		t.Fatalf("inline mode cast should not contain alternate screen escape:\n%s", got)
	}
	// But it should still contain the show-command text.
	for _, want := range []string{`"P"`, `"S"`, `"\u003e"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("cast missing %q:\n%s", want, got)
		}
	}
}

func TestTrimCastPreservesSetupAndTrimsPostRoll(t *testing.T) {
	t.Parallel()

	castPath := filepath.Join(t.TempDir(), "recording.cast")
	cast := strings.Join([]string{
		`{"version":2,"width":80,"height":24}`,
		"[0,\"o\",\"\\u001b[?1049h\\u001b[2J\"]",
		"[0.3,\"o\",\"\\u001b[1;1H\"]",
		"[0.4,\"o\",\"\\u001b[1;1HHello\"]",
		"[0.6,\"o\",\" world\"]",
		"[0.8,\"o\",\"\\u001b[?1049l\\u001b[2J\"]",
		"[0.9,\"o\",\"after exit\"]",
		"",
	}, "\n")
	if err := os.WriteFile(castPath, []byte(cast), 0o600); err != nil {
		t.Fatalf("write cast: %v", err)
	}

	if err := trimCast(castPath); err != nil {
		t.Fatalf("trimCast: %v", err)
	}

	trimmed, err := os.ReadFile(castPath)
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(trimmed)), "\n")
	// Header + 2 setup events + 2 visible events = 5 lines
	if len(lines) != 5 {
		t.Fatalf("trimmed lines = %d, want 5:\n%s", len(lines), trimmed)
	}

	// Setup events are preserved with timestamps rebased to 0.
	setup1, ok, err := parseOutputEvent([]byte(lines[1]))
	if err != nil || !ok {
		t.Fatalf("parse setup1 = %#v, %t, %v", setup1, ok, err)
	}
	if setup1.time != 0 || setup1.output != "\x1b[?1049h\x1b[2J" {
		t.Fatalf("setup1 = %#v", setup1)
	}

	setup2, ok, err := parseOutputEvent([]byte(lines[2]))
	if err != nil || !ok {
		t.Fatalf("parse setup2 = %#v, %t, %v", setup2, ok, err)
	}
	if setup2.time != 0 || setup2.output != "\x1b[1;1H" {
		t.Fatalf("setup2 = %#v", setup2)
	}

	// First visible event starts at t=0.
	first, ok, err := parseOutputEvent([]byte(lines[3]))
	if err != nil || !ok {
		t.Fatalf("parse first visible = %#v, %t, %v", first, ok, err)
	}
	if first.time != 0 || first.output != "\x1b[1;1HHello" {
		t.Fatalf("first visible = %#v", first)
	}

	// Second visible event is rebased relative to the first.
	second, ok, err := parseOutputEvent([]byte(lines[4]))
	if err != nil || !ok {
		t.Fatalf("parse second visible = %#v, %t, %v", second, ok, err)
	}
	if math.Abs(second.time-0.2) > floatTolerance || second.output != " world" {
		t.Fatalf("second visible = %#v", second)
	}

	// Postroll (alt-screen exit and anything after) must be trimmed.
	if strings.Contains(string(trimmed), "1049l") || strings.Contains(string(trimmed), "after exit") {
		t.Fatalf("trimmed cast kept postroll:\n%s", trimmed)
	}
}

func TestTrimCastPreservesCommandPreRollForShowCommand(t *testing.T) {
	t.Parallel()

	// Simulates a cast produced with --show-command on a fullscreen app:
	// writeCommandPreRoll emits ESC[?1049h then characters, then the app
	// starts. Trim must keep the alt-screen-enter so the command text
	// renders in the same buffer as the app.
	castPath := filepath.Join(t.TempDir(), "recording.cast")
	cast := strings.Join([]string{
		`{"version":2,"width":80,"height":24}`,
		"[0,\"o\",\"\\u001b[?1049h\"]",
		"[0.05,\"o\",\"P\"]",
		"[0.1,\"o\",\"S\"]",
		"[0.15,\"o\",\">\"]",
		"[0.5,\"o\",\"\\r\\n\"]",
		"[1.0,\"o\",\"\\u001b[2J\\u001b[1;1HApp UI\"]",
		"[2.0,\"o\",\"\\u001b[?1049l\"]",
		"",
	}, "\n")
	if err := os.WriteFile(castPath, []byte(cast), 0o600); err != nil {
		t.Fatalf("write cast: %v", err)
	}

	if err := trimCast(castPath); err != nil {
		t.Fatalf("trimCast: %v", err)
	}

	trimmed, err := os.ReadFile(castPath)
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}

	// The alt-screen-enter must be preserved.
	if !strings.Contains(string(trimmed), `\u001b[?1049h`) {
		t.Fatalf("trimmed cast lost alt-screen-enter:\n%s", trimmed)
	}

	// The command characters (P, S, >) must be present as the first
	// visible content and start at t=0.
	lines := strings.Split(strings.TrimSpace(string(trimmed)), "\n")
	// Find first event with visible text.
	foundVisible := false
	for _, line := range lines[1:] {
		ev, ok, err := parseOutputEvent([]byte(line))
		if err != nil || !ok {
			continue
		}
		if hasVisibleOutput(ev.output) {
			if ev.time != 0 {
				t.Fatalf("first visible event time = %f, want 0", ev.time)
			}
			foundVisible = true
			break
		}
	}
	if !foundVisible {
		t.Fatalf("no visible event found in trimmed cast:\n%s", trimmed)
	}

	// Postroll must be gone.
	if strings.Contains(string(trimmed), `\u001b[?1049l`) {
		t.Fatalf("trimmed cast kept alt-screen-exit:\n%s", trimmed)
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
