package keystroke

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPlayerWritesAndSleeps(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	sleeper := &recordingSleeper{}
	player := NewPlayer(&output, sleeper, 25*time.Millisecond)

	actions := []Action{
		{Kind: Write, Sequence: "\r"},
		{Kind: Wait, Delay: 100 * time.Millisecond},
		{Kind: Literal, Sequence: "ab"},
	}

	if err := player.PlayActions(actions); err != nil {
		t.Fatalf("PlayActions: %v", err)
	}

	if output.String() != "\rab" {
		t.Fatalf("output = %q, want %q", output.String(), "\rab")
	}

	wantSleeps := []time.Duration{25 * time.Millisecond, 100 * time.Millisecond, 25 * time.Millisecond}
	if !reflect.DeepEqual(sleeper.sleeps, wantSleeps) {
		t.Fatalf("sleeps = %v, want %v", sleeper.sleeps, wantSleeps)
	}
}

func TestPlayerParsesScript(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	sleeper := &recordingSleeper{}
	player := NewPlayer(&output, sleeper, 10*time.Millisecond)

	if err := player.Play("A,Enter"); err != nil {
		t.Fatalf("Play: %v", err)
	}

	if output.String() != "A\r" {
		t.Fatalf("output = %q, want %q", output.String(), "A\r")
	}
}

func TestPlayerLogsActionsAndPacing(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	var log bytes.Buffer
	sleeper := &recordingSleeper{}
	player := NewPlayer(&output, sleeper, 25*time.Millisecond, WithLogWriter(&log))

	actions := []Action{
		{Kind: Write, Sequence: "\x11", Label: "Ctrl+Q"},
		{Kind: Wait, Label: "wait:100", Delay: 100 * time.Millisecond},
		{Kind: Literal, Sequence: "ab", Label: "ab"},
	}

	if err := player.PlayActions(actions); err != nil {
		t.Fatalf("PlayActions: %v", err)
	}

	got := log.String()
	for _, want := range []string{
		"tuicast: key Ctrl+Q",
		"delay 25ms",
		"tuicast: wait wait:100 (100ms)",
		"tuicast: literal 'a'",
		"tuicast: literal delay 25ms",
		"tuicast: literal 'b'",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("log missing %q:\n%s", want, got)
		}
	}
}

type recordingSleeper struct {
	sleeps []time.Duration
}

func (s *recordingSleeper) Sleep(duration time.Duration) {
	s.sleeps = append(s.sleeps, duration)
}
