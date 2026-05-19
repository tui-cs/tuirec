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
		{"Esc", "\x1b"},
		{"Escape", "\x1b"},
		{"Backspace", "\x7f"},
		{"Insert", "\x1b[2~"},
		{"Delete", "\x1b[3~"},
		{"CursorUp", "\x1b[A"},
		{"CursorDown", "\x1b[B"},
		{"CursorRight", "\x1b[C"},
		{"CursorLeft", "\x1b[D"},
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

func TestResolveTerminalGUIValidStrings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{"a", "a"},
		{"A", "A"},
		{"Shift+A", "A"},
		{"Ctrl+A", "\x01"},
		{"Ctrl+a", "\x01"},
		{"Ctrl-A", "\x01"},
		{"A-Ctrl", "\x01"},
		{"Alt+A", "\x1ba"},
		{"Alt-A", "\x1ba"},
		{"Alt-A-Ctrl", "\x1b\x01"},
		{"Ctrl+Alt+A", "\x1b\x01"},
		{"ctrl+alt+shift+cursorup", "\x1b[1;8A"},
		{"CTRL+ALT+SHIFT+CURSORUP", "\x1b[1;8A"},
		{"Ctrl+Alt+Shift+Delete", "\x1b[3;8~"},
		{"Shift+Tab", "\x1b[Z"},
		{"Ctrl+Tab", "\x1b[9;5u"},
		{"Alt+Tab", "\x1b\t"},
		{"Ctrl+Shift+Tab", "\x1b[9;6u"},
		{"Ctrl+Alt+Tab", "\x1b[9;7u"},
		{"Space", " "},
		{"Ctrl+Space", "\x00"},
		{"Alt+Space", "\x1b "},
		{"Shift+ ", " "},
		{"Ctrl++", "\x1b[43;5u"},
		{"Ctrl+Alt++", "\x1b[43;7u"},
		{"0", "0"},
		{"9", "9"},
		{"D0", "0"},
		{"65", "A"},
		{"97", "a"},
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

func TestResolveUnknownOrUnsupportedKeys(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Ctrl+Foo", "Alt+Foo", "F21", "PrintScreen"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, ok := ResolveNamedKey(name); ok {
				t.Fatalf("ResolveNamedKey(%q) ok = true, want false", name)
			}
		})
	}
}
