package record

import (
	"bytes"
	"context"
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

// A minimal sixel DCS payload: ESC P q <data> ESC \
const sixelPayload = "\x1bPq#0;2;0;0;0#1;2;100;100;100#1~~-#0~~-\x1b\\"

// TestDA1ResponseAdvertisesSixelAsDelimitedToken verifies that the DA1 response
// exposes the sixel attribute (4) as a standalone ';'-delimited token. Apps
// commonly detect sixel support with response.Split(";").Contains("4") — for
// example Terminal.Gui's SixelSupportDetector — which fails if 4 is glued to the
// terminator (yielding the token "4c"). Real terminals never place the sixel
// attribute immediately before the final 'c', so tuirec must not either.
func TestDA1ResponseAdvertisesSixelAsDelimitedToken(t *testing.T) {
	t.Parallel()

	for _, tok := range strings.Split(string(da1Response), ";") {
		if tok == "4" {
			return
		}
	}

	t.Fatalf("DA1 response %q exposes no standalone \"4\" token; tokens=%v",
		da1Response, strings.Split(string(da1Response), ";"))
}

// TestInterceptorAnswersCellSizeQuery verifies that a cell-size query (CSI 16 t)
// is answered with a pixel cell-size report (CSI 6 ; h ; w t). Without it, apps
// that advertise sixel fall back to 1 pixel per cell and emit a degenerate
// image.
func TestInterceptorAnswersCellSizeQuery(t *testing.T) {
	t.Parallel()

	var ptyInput bytes.Buffer
	// Cell 8x18 px (e.g. font size 14, line height 1.3 -> 8x18).
	si := newSixelInterceptor(strings.NewReader("\x1b[16t"), &ptyInput, 120, 35, 8, 18)

	buf := make([]byte, 64)
	for {
		if _, err := si.Read(buf); err != nil {
			break
		}
	}

	// CSI 6 ; heightPx ; widthPx t — the reported cell size agg will render at.
	if got := ptyInput.String(); got != "\x1b[6;18;8t" {
		t.Fatalf("expected cell-size report CSI 6;18;8t, got %q", got)
	}
}

// TestSixelGeometryNormalizesZeroConfig verifies that a zero-valued Config —
// valid because pty.Start and gif.Render fill in defaults later — still yields
// the real screen and cell size for the sixel geometry reports, rather than
// zero rows/cols/cell size that would break layout or produce a 0x0 raster.
func TestSixelGeometryNormalizesZeroConfig(t *testing.T) {
	t.Parallel()

	cols, rows, cellW, cellH := sixelGeometry(pty.Size{}, gif.Config{})

	// pty defaults: 120x30; gif defaults: font 14, line height 1.3 -> 8x18 px.
	if cols != 120 || rows != 30 {
		t.Fatalf("size = %dx%d, want 120x30", cols, rows)
	}

	if cellW != 8 || cellH != 18 {
		t.Fatalf("cell = %dx%d px, want 8x18", cellW, cellH)
	}
}

// TestInterceptorAnswersTextAreaQuery verifies that a text-area size query
// (CSI 18 t) is answered with the recording's character dimensions (CSI 8 ;
// rows ; cols t). Terminal.Gui's ANSI driver needs this to lay out its UI;
// without it the screen stays unsized and nothing is drawn.
func TestInterceptorAnswersTextAreaQuery(t *testing.T) {
	t.Parallel()

	var ptyInput bytes.Buffer
	si := newSixelInterceptor(strings.NewReader("\x1b[18t"), &ptyInput, 120, 35, 8, 18)

	buf := make([]byte, 64)
	for {
		if _, err := si.Read(buf); err != nil {
			break
		}
	}

	if got := ptyInput.String(); got != "\x1b[8;35;120t" {
		t.Fatalf("expected text-area report CSI 8;35;120t, got %q", got)
	}
}

// TestPipelineRecordsSixelOutput verifies that sixel DCS sequences emitted by
// an app are preserved in the cast file. This tests the data path only — the
// fake session unconditionally emits sixel without waiting for a DA1 response.
func TestPipelineRecordsSixelOutput(t *testing.T) {
	originalStartPTY := startPTY
	session := &sixelFakeSession{
		output: []byte("Hello\r\n" + sixelPayload + "Done\r\n"),
		closed: make(chan struct{}),
	}
	startPTY = func(string, []string, pty.Size, pty.Options) (pty.Session, error) {
		return session, nil
	}
	defer func() { startPTY = originalStartPTY }()

	castPath := filepath.Join(t.TempDir(), "sixel.cast")
	clock := recorder.NewScriptedClock()

	_, err := Run(context.Background(), Config{
		Binary:         "fake-sixel-app",
		CastOutput:     castPath,
		Keystrokes:     "wait:50",
		KeystrokeDelay: time.Millisecond,
		DrainDuration:  10 * time.Millisecond,
		MaxDuration:    time.Second,
		Clock:          clock,
		Renderer:       &fakeRenderer{},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	cast, err := os.ReadFile(castPath)
	if err != nil {
		t.Fatalf("read cast: %v", err)
	}

	// The cast JSON-encodes ESC as \u001b. Check for the DCS introducer.
	if !strings.Contains(string(cast), `\u001bPq`) {
		t.Fatalf("cast missing sixel DCS payload:\n%s", cast)
	}
}

// TestDA1ResponseIncludesSixel verifies that when a recorded app sends a DA1
// query (\x1b[c), tuirec responds with a DA1 response that advertises sixel
// capability (attribute 4). Without this, apps won't emit sixel at all.
func TestDA1ResponseIncludesSixel(t *testing.T) {
	originalStartPTY := startPTY

	// Session that emits DA1 query in its output, then reads the response.
	session := &da1FakeSession{
		closed: make(chan struct{}),
	}
	startPTY = func(string, []string, pty.Size, pty.Options) (pty.Session, error) {
		return session, nil
	}
	defer func() { startPTY = originalStartPTY }()

	castPath := filepath.Join(t.TempDir(), "da1.cast")
	clock := recorder.NewScriptedClock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Run(ctx, Config{
		Binary:         "fake-da1-app",
		CastOutput:     castPath,
		Keystrokes:     "wait:100",
		KeystrokeDelay: time.Millisecond,
		DrainDuration:  10 * time.Millisecond,
		MaxDuration:    2 * time.Second,
		Clock:          clock,
		Renderer:       &fakeRenderer{},
	})
	// We don't care about the run error (timeout is fine); we care about
	// whether the DA1 response was received.
	_ = err

	// The DA1 response must include attribute 4 (sixel graphics).
	response := session.receivedResponse()
	if response == "" {
		t.Fatal("app received no DA1 response from tuirec")
	}
	// Expected format: \x1b[?62;4c or similar with ;4 somewhere.
	if !strings.Contains(response, ";4") {
		t.Fatalf("DA1 response %q does not advertise sixel (attribute 4)", response)
	}
}

// TestTrimCastPreservesSixelDCS verifies that the trim feature does not
// corrupt or discard sixel DCS payloads from the cast.
func TestTrimCastPreservesSixelDCS(t *testing.T) {
	t.Parallel()

	castPath := filepath.Join(t.TempDir(), "sixel-trim.cast")
	cast := strings.Join([]string{
		`{"version":2,"width":80,"height":24}`,
		`[0,"o","\u001b[?1049h"]`,
		`[0.1,"o","Hello"]`,
		`[0.2,"o","\u001bPq#0;2;0;0;0#1;2;100;100;100#1~~-#0~~-\u001b\\"]`,
		`[0.5,"o"," world"]`,
		`[0.8,"o","\u001b[?1049l"]`,
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

	// The sixel DCS payload must survive trimming.
	if !strings.Contains(string(trimmed), `\u001bPq`) {
		t.Fatalf("trimmed cast lost sixel DCS:\n%s", trimmed)
	}
	// The visible text must also be present.
	if !strings.Contains(string(trimmed), `Hello`) {
		t.Fatalf("trimmed cast lost visible text:\n%s", trimmed)
	}
}

// TestTrimCastPreservesSixelOnlyDCS verifies that sixel output can start a
// trimmed cast even when there is no surrounding visible text.
func TestTrimCastPreservesSixelOnlyDCS(t *testing.T) {
	t.Parallel()

	castPath := filepath.Join(t.TempDir(), "sixel-only-trim.cast")
	cast := strings.Join([]string{
		`{"version":2,"width":80,"height":24}`,
		`[0,"o","\u001b[?1049h"]`,
		`[0.2,"o","\u001bPq#0;2;0;0;0#1;2;100;100;100#1~~-#0~~-\u001b\\"]`,
		`[0.8,"o","\u001b[?1049l"]`,
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

	if !strings.Contains(string(trimmed), `\u001bPq`) {
		t.Fatalf("trimmed cast lost sixel-only DCS:\n%s", trimmed)
	}
}

// TestHasVisibleOutputIgnoresDCS verifies that a DCS payload (like sixel) is
// not treated as visible output (it's a device control sequence, not text).
func TestHasVisibleOutputIgnoresDCS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"sixel only", "\x1bPq#0;2;0;0;0#1~~-\x1b\\", false},
		{"sixel with text", "\x1bPq#1~~-\x1b\\Hello", true},
		{"text only", "Hello", true},
		{"CSI only", "\x1b[2J", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasVisibleOutput(tt.input)
			if got != tt.expect {
				t.Errorf("hasVisibleOutput(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

// TestHasSixelOutput verifies that only true sixel DCS sequences (no
// intermediate bytes, final byte 'q') are recognized, and that non-sixel
// DCS sequences whose final byte happens to be 'q' (such as a DECRQSS query
// ESC P $ q ... ESC \) are rejected.
func TestHasSixelOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"sixel ESC P q", "\x1bPq#1~~-\x1b\\", true},
		{"sixel with params", "\x1bP0;0;0q#1~~-\x1b\\", true},
		{"sixel C1 DCS", "\x90q#1~~-\x1b\\", true},
		{"decrqss query", "\x1bP$q\"p\x1b\\", false},
		{"decrqss C1", "\x90$q\"p\x1b\\", false},
		{"dcs non-q final", "\x1bP+q\x1b\\", false},
		{"no dcs", "Hello", false},
		{"csi only", "\x1b[2J", false},
		{"empty dcs", "\x1bP\x1b\\", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSixelOutput(tt.input)
			if got != tt.expect {
				t.Errorf("hasSixelOutput(%q) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

// --- fake sessions for sixel tests ---

// sixelFakeSession emits canned output then EOF.
type sixelFakeSession struct {
	mu     sync.Mutex
	output []byte
	pos    int
	input  []byte
	closed chan struct{}
	once   sync.Once
}

func (s *sixelFakeSession) Read(p []byte) (int, error) {
	s.mu.Lock()
	if s.pos < len(s.output) {
		n := copy(p, s.output[s.pos:])
		s.pos += n
		s.mu.Unlock()
		return n, nil
	}
	s.mu.Unlock()

	// Wait for close
	<-s.closed
	return 0, io.EOF
}

func (s *sixelFakeSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.input = append(s.input, p...)
	return len(p), nil
}

func (s *sixelFakeSession) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func (s *sixelFakeSession) Pid() int              { return 1 }
func (s *sixelFakeSession) Resize(pty.Size) error { return nil }
func (s *sixelFakeSession) Wait(ctx context.Context) (pty.ExitStatus, error) {
	select {
	case <-s.closed:
		return pty.ExitStatus{}, nil
	case <-ctx.Done():
		return pty.ExitStatus{}, ctx.Err()
	}
}

// da1FakeSession emits a DA1 query then waits for the response.
type da1FakeSession struct {
	mu     sync.Mutex
	phase  int // 0=emit DA1 query, 1=emit EOF
	input  bytes.Buffer
	closed chan struct{}
	once   sync.Once
}

func (s *da1FakeSession) Read(p []byte) (int, error) {
	s.mu.Lock()
	phase := s.phase
	s.mu.Unlock()

	switch phase {
	case 0:
		// Emit DA1 query: \x1b[c
		s.mu.Lock()
		s.phase = 1
		s.mu.Unlock()
		n := copy(p, []byte("\x1b[c"))
		return n, nil
	default:
		// Give time for response to arrive, then EOF
		time.Sleep(50 * time.Millisecond)
		<-s.closed
		return 0, io.EOF
	}
}

func (s *da1FakeSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.input.Write(p)
	return len(p), nil
}

func (s *da1FakeSession) Close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func (s *da1FakeSession) Pid() int              { return 1 }
func (s *da1FakeSession) Resize(pty.Size) error { return nil }
func (s *da1FakeSession) Wait(ctx context.Context) (pty.ExitStatus, error) {
	select {
	case <-s.closed:
		return pty.ExitStatus{}, nil
	case <-ctx.Done():
		return pty.ExitStatus{}, ctx.Err()
	}
}

func (s *da1FakeSession) receivedResponse() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.input.String()
}

// Verify interface compliance.
var (
	_ pty.Session = (*sixelFakeSession)(nil)
	_ pty.Session = (*da1FakeSession)(nil)
)
