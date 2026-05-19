package recorder

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecorderMatchesGoldenCast(t *testing.T) {
	t.Parallel()

	clock := NewScriptedClock()
	var output bytes.Buffer
	recorder, err := New(&output, Config{
		Width:     80,
		Height:    24,
		Timestamp: time.Unix(1710000000, 0).UTC(),
		Title:     "fixture",
		Env:       map[string]string{"TERM": "xterm-256color"},
		Clock:     clock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := recorder.Write([]byte("hello\r\n")); err != nil {
		t.Fatalf("Write hello: %v", err)
	}

	clock.Advance(1500 * time.Millisecond)
	if _, err := recorder.Write([]byte("world\n")); err != nil {
		t.Fatalf("Write world: %v", err)
	}

	if err := recorder.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	want, err := os.ReadFile(filepath.Join("testdata", "simple.cast"))
	if err != nil {
		t.Fatalf("read golden cast: %v", err)
	}

	if output.String() != string(want) {
		t.Fatalf("cast output mismatch\nwant:\n%s\ngot:\n%s", want, output.String())
	}
}

func TestRecorderBuffersSplitUTF8Rune(t *testing.T) {
	t.Parallel()

	clock := NewScriptedClock()
	var output bytes.Buffer
	recorder, err := New(&output, Config{
		Width:     80,
		Height:    24,
		Timestamp: time.Unix(1710000000, 0).UTC(),
		Clock:     clock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	writeBytes(t, recorder, []byte{0xe2})
	clock.Advance(100 * time.Millisecond)
	writeBytes(t, recorder, []byte{0x82})
	clock.Advance(100 * time.Millisecond)
	writeBytes(t, recorder, []byte{0xac})

	if err := recorder.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := output.String()
	if strings.Contains(got, "\ufffd") {
		t.Fatalf("split UTF-8 was corrupted: %q", got)
	}

	if !strings.Contains(got, "[0.2,\"o\",\"€\"]\n") {
		t.Fatalf("expected one complete euro-sign event at 0.2s, got:\n%s", got)
	}
}

func TestRecorderStreamsOutputEvents(t *testing.T) {
	t.Parallel()

	writer := &countingWriter{}
	recorder, err := New(writer, Config{
		Width:     80,
		Height:    24,
		Timestamp: time.Unix(1710000000, 0).UTC(),
		Clock:     NewScriptedClock(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if writer.writes != 1 {
		t.Fatalf("writes after New = %d, want 1 header write", writer.writes)
	}

	if _, err := recorder.Write([]byte("frame")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if writer.writes != 2 {
		t.Fatalf("writes after event = %d, want 2", writer.writes)
	}
}

func writeBytes(t *testing.T, recorder *Recorder, data []byte) {
	t.Helper()

	n, err := recorder.Write(data)
	if err != nil {
		t.Fatalf("Write(%v): %v", data, err)
	}

	if n != len(data) {
		t.Fatalf("Write(%v) n = %d, want %d", data, n, len(data))
	}
}

type countingWriter struct {
	writes int
	bytes.Buffer
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++

	return w.Buffer.Write(p)
}
