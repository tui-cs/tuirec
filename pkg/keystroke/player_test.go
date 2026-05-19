package keystroke

import (
	"bytes"
	"reflect"
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

type recordingSleeper struct {
	sleeps []time.Duration
}

func (s *recordingSleeper) Sleep(duration time.Duration) {
	s.sleeps = append(s.sleeps, duration)
}
