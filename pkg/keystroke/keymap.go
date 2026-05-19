// Package keystroke parses and plays scripted terminal input.
package keystroke

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ResolveNamedKey returns the terminal sequence for a named key.
func ResolveNamedKey(name string) (string, bool) {
	if sequence, ok := resolveFixedNamedKey(name); ok {
		return sequence, true
	}

	if sequence, ok := resolveCtrlLetter(name); ok {
		return sequence, true
	}

	return resolveAltChar(name)
}

func resolveFixedNamedKey(name string) (string, bool) {
	switch name {
	case "Enter", "Return":
		return "\r", true
	case "Tab":
		return "\t", true
	case "Escape":
		return "\x1b", true
	case "Backspace":
		return "\x7f", true
	case "Delete":
		return "\x1b[3~", true
	case "ArrowUp":
		return "\x1b[A", true
	case "ArrowDown":
		return "\x1b[B", true
	case "ArrowRight":
		return "\x1b[C", true
	case "ArrowLeft":
		return "\x1b[D", true
	case "Home":
		return "\x1b[H", true
	case "End":
		return "\x1b[F", true
	case "PageUp":
		return "\x1b[5~", true
	case "PageDown":
		return "\x1b[6~", true
	case "F1":
		return "\x1bOP", true
	case "F2":
		return "\x1bOQ", true
	case "F3":
		return "\x1bOR", true
	case "F4":
		return "\x1bOS", true
	case "F5":
		return "\x1b[15~", true
	case "F6":
		return "\x1b[17~", true
	case "F7":
		return "\x1b[18~", true
	case "F8":
		return "\x1b[19~", true
	case "F9":
		return "\x1b[20~", true
	case "F10":
		return "\x1b[21~", true
	case "F11":
		return "\x1b[23~", true
	case "F12":
		return "\x1b[24~", true
	default:
		return "", false
	}
}

func resolveCtrlLetter(name string) (string, bool) {
	rest, ok := modifierRest(name, "Ctrl")
	if !ok {
		return "", false
	}

	var r rune
	if _, err := fmt.Sscanf(rest, "%c", &r); err != nil {
		return "", false
	}

	if r < 'A' || r > 'Z' || len(rest) != len("A") {
		return "", false
	}

	return string(byte(r - 'A' + 1)), true
}

func resolveAltChar(name string) (string, bool) {
	rest, ok := modifierRest(name, "Alt")
	if !ok {
		return "", false
	}

	r, size := utf8.DecodeRuneInString(rest)
	if r == utf8.RuneError || size != len(rest) {
		return "", false
	}

	return "\x1b" + string(r), true
}

func modifierRest(name, modifier string) (string, bool) {
	for _, separator := range []string{"+", "-"} {
		prefix := modifier + separator
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			return name[len(prefix):], true
		}
	}

	return "", false
}
