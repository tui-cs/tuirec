package keystroke

import "fmt"

// kittyKeyCodepoints maps named keys to their Kitty keyboard protocol
// unicode codepoints. These follow the spec at:
// https://sw.kovidgoyal.net/kitty/keyboard-protocol/#functional-key-definitions
var kittyKeyCodepoints = map[string]int{
	"Backspace":   127,
	"Tab":         9,
	"Enter":       13,
	"Esc":         27,
	"Space":       32,

	"F1":          57364,
	"F2":          57365,
	"F3":          57366,
	"F4":          57367,
	"F5":          57368,
	"F6":          57369,
	"F7":          57370,
	"F8":          57371,
	"F9":          57372,
	"F10":         57373,
	"F11":         57374,
	"F12":         57375,
	"F13":         57376,
	"F14":         57377,
	"F15":         57378,
	"F16":         57379,
	"F17":         57380,
	"F18":         57381,
	"F19":         57382,
	"F20":         57383,
	"PrintScreen": 57361,
	"Clear":       57422,
}

// kittySequence encodes a terminalKey using the Kitty keyboard protocol CSI u format.
// All keys are encoded as ESC [ codepoint ; modifiers u, providing full disambiguation.
func (key terminalKey) kittySequence() (string, error) {
	if key.name == "" && key.rune == 0 {
		return "", fmt.Errorf("standalone modifier keys are not supported")
	}

	if key.name != "" {
		return key.kittyNamedSequence()
	}

	return key.kittyRuneSequence(), nil
}

func (key terminalKey) kittyNamedSequence() (string, error) {
	// Digit keys stored as name
	if len(key.name) == 1 && key.name[0] >= '0' && key.name[0] <= '9' {
		return (terminalKey{rune: rune(key.name[0]), mods: key.mods}).kittyRuneSequence(), nil
	}

	// Keys with defined Kitty codepoints use CSI u encoding.
	if codepoint, ok := kittyKeyCodepoints[key.name]; ok {
		return kittyCsiU(codepoint, key.mods), nil
	}

	// Navigation keys use legacy CSI sequences per the Kitty spec — they do
	// not have CSI u codepoints.
	if sequence, ok := key.xtermModifiedSequence(); ok {
		return sequence, nil
	}

	return "", fmt.Errorf("unsupported key for kitty encoding: %s", key.display())
}

func (key terminalKey) kittyRuneSequence() string {
	r := key.rune
	// Kitty spec: always use the unshifted (lowercase) codepoint for letters.
	// Shift is conveyed only via the modifier parameter.
	if r >= 'A' && r <= 'Z' {
		r += 'a' - 'A'
	}

	return kittyCsiU(int(r), key.mods)
}

// kittyCsiU formats a Kitty CSI u sequence. Omits modifiers param when no
// modifiers are active (the spec allows the short form ESC [ codepoint u).
func kittyCsiU(codepoint, mods int) string {
	modifier := xtermModifier(mods)
	if modifier == 1 {
		return fmt.Sprintf("\x1b[%du", codepoint)
	}

	return fmt.Sprintf("\x1b[%d;%du", codepoint, modifier)
}

// resolveKittyKey resolves a Terminal.Gui key name to its Kitty CSI u sequence.
func resolveKittyKey(name string) (string, bool) {
	key, ok := parseTerminalGUIKey(name)
	if !ok {
		return "", false
	}

	sequence, err := key.kittySequence()
	if err != nil {
		return "", false
	}

	return sequence, true
}
