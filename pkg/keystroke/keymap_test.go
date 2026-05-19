package keystroke

import (
	"fmt"
	"testing"
)

func TestResolveNamedKeyTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"Enter", "\r"},
		{"Return", "\r"},
		{"Tab", "\t"},
		{"Escape", "\x1b"},
		{"Backspace", "\x7f"},
		{"Delete", "\x1b[3~"},
		{"ArrowUp", "\x1b[A"},
		{"ArrowDown", "\x1b[B"},
		{"ArrowRight", "\x1b[C"},
		{"ArrowLeft", "\x1b[D"},
		{"Home", "\x1b[H"},
		{"End", "\x1b[F"},
		{"PageUp", "\x1b[5~"},
		{"PageDown", "\x1b[6~"},
		{"F1", "\x1bOP"},
		{"F2", "\x1bOQ"},
		{"F3", "\x1bOR"},
		{"F4", "\x1bOS"},
		{"F5", "\x1b[15~"},
		{"F6", "\x1b[17~"},
		{"F7", "\x1b[18~"},
		{"F8", "\x1b[19~"},
		{"F9", "\x1b[20~"},
		{"F10", "\x1b[21~"},
		{"F11", "\x1b[23~"},
		{"F12", "\x1b[24~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := ResolveNamedKey(tt.name)
			if !ok {
				t.Fatalf("ResolveNamedKey(%q) ok = false, want true", tt.name)
			}

			if got != tt.want {
				t.Fatalf("ResolveNamedKey(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestResolveCtrlLetters(t *testing.T) {
	t.Parallel()

	for r := 'A'; r <= 'Z'; r++ {
		r := r
		t.Run(fmt.Sprintf("Ctrl+%c", r), func(t *testing.T) {
			t.Parallel()

			name := fmt.Sprintf("Ctrl+%c", r)
			got, ok := ResolveNamedKey(name)
			if !ok {
				t.Fatalf("ResolveNamedKey(%q) ok = false, want true", name)
			}

			want := string(byte(r - 'A' + 1))
			if got != want {
				t.Fatalf("ResolveNamedKey(%q) = %q, want %q", name, got, want)
			}
		})
	}
}

func TestResolveCtrlLettersWithHyphen(t *testing.T) {
	t.Parallel()

	got, ok := ResolveNamedKey("Ctrl-A")
	if !ok {
		t.Fatalf("ResolveNamedKey(Ctrl-A) ok = false, want true")
	}

	if got != "\x01" {
		t.Fatalf("ResolveNamedKey(Ctrl-A) = %q, want %q", got, "\x01")
	}
}

func TestResolveAltChar(t *testing.T) {
	t.Parallel()

	got, ok := ResolveNamedKey("Alt+x")
	if !ok {
		t.Fatalf("ResolveNamedKey(Alt+x) ok = false, want true")
	}

	if got != "\x1bx" {
		t.Fatalf("ResolveNamedKey(Alt+x) = %q, want %q", got, "\x1bx")
	}
}

func TestResolveAltCharWithHyphen(t *testing.T) {
	t.Parallel()

	got, ok := ResolveNamedKey("Alt-x")
	if !ok {
		t.Fatalf("ResolveNamedKey(Alt-x) ok = false, want true")
	}

	if got != "\x1bx" {
		t.Fatalf("ResolveNamedKey(Alt-x) = %q, want %q", got, "\x1bx")
	}
}

func TestResolveNamedKeyIsCaseSensitive(t *testing.T) {
	t.Parallel()

	if _, ok := ResolveNamedKey("Ctrl+c"); ok {
		t.Fatal("ResolveNamedKey(Ctrl+c) ok = true, want false")
	}
}
