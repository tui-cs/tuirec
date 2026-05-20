package keystroke

import "testing"

func TestKittySequence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  terminalKey
		want string
	}{
		{"enter no mods", terminalKey{name: "Enter"}, "\x1b[13u"},
		{"tab no mods", terminalKey{name: "Tab"}, "\x1b[9u"},
		{"backspace no mods", terminalKey{name: "Backspace"}, "\x1b[127u"},
		{"escape no mods", terminalKey{name: "Esc"}, "\x1b[27u"},
		{"space no mods", terminalKey{name: "Space"}, "\x1b[32u"},
		{"ctrl+m", terminalKey{name: "Enter", mods: modCtrl}, "\x1b[13;5u"},
		{"ctrl+i", terminalKey{name: "Tab", mods: modCtrl}, "\x1b[9;5u"},
		{"shift+tab", terminalKey{name: "Tab", mods: modShift}, "\x1b[9;2u"},
		{"alt+enter", terminalKey{name: "Enter", mods: modAlt}, "\x1b[13;3u"},
		{"ctrl+alt+shift+enter", terminalKey{name: "Enter", mods: modCtrl | modAlt | modShift}, "\x1b[13;8u"},
		{"F1 no mods", terminalKey{name: "F1"}, "\x1b[57364u"},
		{"F12 no mods", terminalKey{name: "F12"}, "\x1b[57375u"},
		{"ctrl+F5", terminalKey{name: "F5", mods: modCtrl}, "\x1b[57368;5u"},
		{"cursor up", terminalKey{name: "CursorUp"}, "\x1b[57352u"},
		{"shift+cursor down", terminalKey{name: "CursorDown", mods: modShift}, "\x1b[57353;2u"},
		{"insert", terminalKey{name: "Insert"}, "\x1b[57348u"},
		{"delete", terminalKey{name: "Delete"}, "\x1b[57349u"},
		{"home", terminalKey{name: "Home"}, "\x1b[57356u"},
		{"end", terminalKey{name: "End"}, "\x1b[57357u"},
		{"page up", terminalKey{name: "PageUp"}, "\x1b[57350u"},
		{"page down", terminalKey{name: "PageDown"}, "\x1b[57351u"},
		{"lowercase a", terminalKey{rune: 'A'}, "\x1b[97u"},
		{"uppercase A (shift)", terminalKey{rune: 'A', mods: modShift}, "\x1b[97;2u"},
		{"ctrl+a", terminalKey{rune: 'A', mods: modCtrl}, "\x1b[97;5u"},
		{"ctrl+shift+a", terminalKey{rune: 'A', mods: modCtrl | modShift}, "\x1b[97;6u"},
		{"digit 5", terminalKey{rune: '5'}, "\x1b[53u"},
		{"alt+5", terminalKey{rune: '5', mods: modAlt}, "\x1b[53;3u"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.key.kittySequence()
			if err != nil {
				t.Fatalf("kittySequence() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("kittySequence() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKittySequenceStandaloneModifierError(t *testing.T) {
	t.Parallel()

	key := terminalKey{mods: modCtrl}
	_, err := key.kittySequence()
	if err == nil {
		t.Fatal("expected error for standalone modifier")
	}
}

func TestResolveKittyKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"enter", "Enter", "\x1b[13u"},
		{"ctrl+m", "Ctrl+M", "\x1b[109;5u"},
		{"ctrl+i", "Ctrl+I", "\x1b[105;5u"},
		{"tab", "Tab", "\x1b[9u"},
		{"shift+tab", "Shift+Tab", "\x1b[9;2u"},
		{"ctrl+c", "Ctrl+C", "\x1b[99;5u"},
		{"F1", "F1", "\x1b[57364u"},
		{"escape", "Escape", "\x1b[27u"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := resolveKittyKey(tt.input)
			if !ok {
				t.Fatal("resolveKittyKey returned false")
			}
			if got != tt.want {
				t.Errorf("resolveKittyKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveKittyKeyDisambiguates(t *testing.T) {
	t.Parallel()

	// The whole point: Ctrl+M and Enter must produce different sequences
	ctrlM, ok := resolveKittyKey("Ctrl+M")
	if !ok {
		t.Fatal("resolveKittyKey(Ctrl+M) returned false")
	}

	enter, ok := resolveKittyKey("Enter")
	if !ok {
		t.Fatal("resolveKittyKey(Enter) returned false")
	}

	if ctrlM == enter {
		t.Errorf("Ctrl+M and Enter must differ in kitty mode, both = %q", ctrlM)
	}

	// Similarly: Ctrl+I vs Tab
	ctrlI, ok := resolveKittyKey("Ctrl+I")
	if !ok {
		t.Fatal("resolveKittyKey(Ctrl+I) returned false")
	}

	tab, ok := resolveKittyKey("Tab")
	if !ok {
		t.Fatal("resolveKittyKey(Tab) returned false")
	}

	if ctrlI == tab {
		t.Errorf("Ctrl+I and Tab must differ in kitty mode, both = %q", ctrlI)
	}
}
