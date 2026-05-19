// Package keystroke parses and plays scripted terminal input.
package keystroke

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	modShift = 1 << iota
	modAlt
	modCtrl
)

type terminalKey struct {
	name string
	rune rune
	mods int
}

// ResolveNamedKey returns the terminal sequence for a Terminal.Gui-compatible key string.
func ResolveNamedKey(name string) (string, bool) {
	sequence, ok, err := resolveTerminalGUIKey(name)
	return sequence, ok && err == nil
}

func resolveTerminalGUIKey(name string) (string, bool, error) {
	key, ok := parseTerminalGUIKey(name)
	if !ok {
		return "", false, nil
	}

	sequence, err := key.sequence()
	if err != nil {
		return "", true, err
	}

	return sequence, true, nil
}

func parseTerminalGUIKey(text string) (terminalKey, bool) {
	if text == "" {
		return terminalKey{}, false
	}

	if modifier, ok := parseModifier(text); ok {
		return terminalKey{mods: modifier}, true
	}

	separator := terminalGUISeparator(text)
	parts := strings.Split(text, separator)
	if len(parts) > 4 {
		return terminalKey{}, false
	}

	if len(parts) == 1 {
		return parseTerminalGUIKeyPart(parts[0], 0)
	}

	if strings.HasSuffix(text, separator) {
		parts[len(parts)-1] = separator
	} else if hasEmptyPart(parts) {
		return terminalKey{}, false
	}

	mods := 0
	for index, part := range parts {
		if modifier, ok := parseModifier(part); ok {
			mods |= modifier
			parts[index] = ""
		}
	}

	for _, part := range parts {
		if part == "" {
			continue
		}

		return parseTerminalGUIKeyPart(part, mods)
	}

	return terminalKey{mods: mods}, true
}

func terminalGUISeparator(text string) string {
	for _, modifier := range []string{"Ctrl", "Alt", "Shift"} {
		if startsWithFold(text, modifier) && len(text) > len(modifier) {
			return text[len(modifier) : len(modifier)+1]
		}
		if endsWithFold(text, modifier) && len(text) > len(modifier) {
			return text[len(text)-len(modifier)-1 : len(text)-len(modifier)]
		}
	}

	return "+"
}

func parseTerminalGUIKeyPart(part string, mods int) (terminalKey, bool) {
	if key, ok := parseSingleRuneKey(part, mods); ok {
		return key, true
	}

	if name, ok := parseKeyName(part); ok {
		return terminalKey{name: name, mods: mods}, true
	}

	if codepoint, ok := parseCodepoint(part); ok {
		if codepoint >= 'A' && codepoint <= 'Z' && mods == 0 {
			mods = modShift
		}
		return terminalKey{rune: codepoint, mods: mods}, true
	}

	return terminalKey{}, false
}

func parseSingleRuneKey(part string, mods int) (terminalKey, bool) {
	r, size := utf8.DecodeRuneInString(part)
	if r == utf8.RuneError || size != len(part) {
		return terminalKey{}, false
	}

	if r >= '0' && r <= '9' {
		return terminalKey{rune: r, mods: mods}, true
	}

	if r >= 'A' && r <= 'Z' {
		if mods == 0 {
			mods = modShift
		}
		return terminalKey{rune: r, mods: mods}, true
	}

	if r >= 'a' && r <= 'z' {
		return terminalKey{rune: r - ('a' - 'A'), mods: mods}, true
	}

	return terminalKey{rune: r, mods: mods}, true
}

func parseKeyName(part string) (string, bool) {
	switch strings.ToLower(part) {
	case "backspace":
		return "Backspace", true
	case "tab":
		return "Tab", true
	case "enter", "return":
		return "Enter", true
	case "clear":
		return "Clear", true
	case "esc", "escape":
		return "Esc", true
	case "space":
		return "Space", true
	case "insert":
		return "Insert", true
	case "delete":
		return "Delete", true
	case "printscreen":
		return "PrintScreen", true
	case "cursorup", "arrowup":
		return "CursorUp", true
	case "cursordown", "arrowdown":
		return "CursorDown", true
	case "cursorleft", "arrowleft":
		return "CursorLeft", true
	case "cursorright", "arrowright":
		return "CursorRight", true
	case "pageup":
		return "PageUp", true
	case "pagedown":
		return "PageDown", true
	case "home":
		return "Home", true
	case "end":
		return "End", true
	}

	if len(part) >= 2 && (part[0] == 'D' || part[0] == 'd') && len(part) == 2 && part[1] >= '0' && part[1] <= '9' {
		return string(part[1]), true
	}

	if len(part) >= 2 && (part[0] == 'F' || part[0] == 'f') {
		number, err := strconv.Atoi(part[1:])
		if err == nil && number >= 1 && number <= 24 {
			return fmt.Sprintf("F%d", number), true
		}
	}

	return "", false
}

func parseCodepoint(part string) (rune, bool) {
	codepoint, err := strconv.Atoi(part)
	if err != nil || codepoint < 0 || codepoint > utf8.MaxRune {
		return 0, false
	}

	return rune(codepoint), true
}

func parseModifier(part string) (int, bool) {
	switch strings.ToLower(part) {
	case "shift":
		return modShift, true
	case "alt":
		return modAlt, true
	case "ctrl":
		return modCtrl, true
	default:
		return 0, false
	}
}

func (key terminalKey) sequence() (string, error) {
	if key.name == "" && key.rune == 0 {
		return "", fmt.Errorf("standalone modifier keys are not supported")
	}

	if key.name != "" {
		return key.namedSequence()
	}

	return key.runeSequence()
}

func (key terminalKey) runeSequence() (string, error) {
	r := key.rune
	if r >= 'A' && r <= 'Z' && key.mods&modShift == 0 {
		r += 'a' - 'A'
	}

	sequence := string(r)
	if key.mods&modCtrl != 0 {
		ctrl, ok := ctrlRune(r)
		if !ok {
			return csiU(int(r), key.mods), nil
		}
		sequence = string(ctrl)
	}

	if key.mods&modAlt != 0 {
		sequence = "\x1b" + sequence
	}

	return sequence, nil
}

func (key terminalKey) namedSequence() (string, error) {
	if len(key.name) == 1 && key.name[0] >= '0' && key.name[0] <= '9' {
		return terminalKey{rune: rune(key.name[0]), mods: key.mods}.runeSequence()
	}

	base, ok := baseNamedSequences[key.name]
	if !ok {
		return "", fmt.Errorf("unsupported key: %s", key.display())
	}

	if key.mods == 0 {
		return base, nil
	}

	if key.name == "Space" {
		return terminalKey{rune: ' ', mods: key.mods}.runeSequence()
	}

	if key.name == "Tab" {
		return key.tabSequence(), nil
	}

	if key.name == "Enter" && key.mods == modAlt {
		return "\x1b\r", nil
	}

	if sequence, ok := key.xtermModifiedSequence(); ok {
		return sequence, nil
	}

	if key.name == "Esc" && key.mods == modAlt {
		return "\x1b\x1b", nil
	}

	return key.csiUSequence()
}

func (key terminalKey) tabSequence() string {
	switch key.mods {
	case modShift:
		return "\x1b[Z"
	case modAlt:
		return "\x1b\t"
	default:
		return csiU(9, key.mods)
	}
}

func (key terminalKey) xtermModifiedSequence() (string, bool) {
	if suffix, ok := xtermSS3Suffix[key.name]; ok {
		modifier := xtermModifier(key.mods)
		if modifier == 1 {
			return baseNamedSequences[key.name], true
		}

		return fmt.Sprintf("\x1b[1;%d%s", modifier, suffix), true
	}

	parameter, ok := xtermParameters[key.name]
	if !ok {
		return "", false
	}

	modifier := xtermModifier(key.mods)
	if modifier == 1 {
		return baseNamedSequences[key.name], true
	}

	return fmt.Sprintf("\x1b[%d;%d~", parameter, modifier), true
}

func (key terminalKey) csiUSequence() (string, error) {
	codepoint, ok := csiUCodepoints[key.name]
	if !ok {
		return "", fmt.Errorf("unsupported modified key: %s", key.display())
	}

	return csiU(codepoint, key.mods), nil
}

func csiU(codepoint, mods int) string {
	return fmt.Sprintf("\x1b[%d;%du", codepoint, xtermModifier(mods))
}

func xtermModifier(mods int) int {
	modifier := 1
	if mods&modShift != 0 {
		modifier++
	}
	if mods&modAlt != 0 {
		modifier += 2
	}
	if mods&modCtrl != 0 {
		modifier += 4
	}

	return modifier
}

func ctrlRune(r rune) (rune, bool) {
	if r >= 'a' && r <= 'z' {
		r -= 'a' - 'A'
	}

	if r >= 'A' && r <= 'Z' {
		return r - 'A' + 1, true
	}

	switch r {
	case ' ', '2':
		return 0x00, true
	case '[', '3':
		return 0x1b, true
	case '\\', '4':
		return 0x1c, true
	case ']', '5':
		return 0x1d, true
	case '^', '6':
		return 0x1e, true
	case '_', '7':
		return 0x1f, true
	case '?':
		return 0x7f, true
	default:
		return 0, false
	}
}

func (key terminalKey) display() string {
	parts := make([]string, 0, 4)
	if key.mods&modCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if key.mods&modAlt != 0 {
		parts = append(parts, "Alt")
	}
	if key.mods&modShift != 0 {
		parts = append(parts, "Shift")
	}
	if key.name != "" {
		parts = append(parts, key.name)
	} else {
		parts = append(parts, string(key.rune))
	}

	return strings.Join(parts, "+")
}

func hasEmptyPart(parts []string) bool {
	for _, part := range parts {
		if part == "" {
			return true
		}
	}

	return false
}

func startsWithFold(text, prefix string) bool {
	return len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix)
}

func endsWithFold(text, suffix string) bool {
	return len(text) >= len(suffix) && strings.EqualFold(text[len(text)-len(suffix):], suffix)
}

var baseNamedSequences = map[string]string{
	"Backspace":   "\x7f",
	"Tab":         "\t",
	"Enter":       "\r",
	"Clear":       "\x0c",
	"Esc":         "\x1b",
	"Space":       " ",
	"Insert":      "\x1b[2~",
	"Delete":      "\x1b[3~",
	"CursorUp":    "\x1b[A",
	"CursorDown":  "\x1b[B",
	"CursorRight": "\x1b[C",
	"CursorLeft":  "\x1b[D",
	"PageUp":      "\x1b[5~",
	"PageDown":    "\x1b[6~",
	"Home":        "\x1b[H",
	"End":         "\x1b[F",
	"F1":          "\x1bOP",
	"F2":          "\x1bOQ",
	"F3":          "\x1bOR",
	"F4":          "\x1bOS",
	"F5":          "\x1b[15~",
	"F6":          "\x1b[17~",
	"F7":          "\x1b[18~",
	"F8":          "\x1b[19~",
	"F9":          "\x1b[20~",
	"F10":         "\x1b[21~",
	"F11":         "\x1b[23~",
	"F12":         "\x1b[24~",
	"F13":         "\x1b[25~",
	"F14":         "\x1b[26~",
	"F15":         "\x1b[28~",
	"F16":         "\x1b[29~",
	"F17":         "\x1b[31~",
	"F18":         "\x1b[32~",
	"F19":         "\x1b[33~",
	"F20":         "\x1b[34~",
}

var xtermSS3Suffix = map[string]string{
	"CursorUp":    "A",
	"CursorDown":  "B",
	"CursorRight": "C",
	"CursorLeft":  "D",
	"Home":        "H",
	"End":         "F",
	"F1":          "P",
	"F2":          "Q",
	"F3":          "R",
	"F4":          "S",
}

var xtermParameters = map[string]int{
	"Insert":   2,
	"Delete":   3,
	"PageUp":   5,
	"PageDown": 6,
	"F5":       15,
	"F6":       17,
	"F7":       18,
	"F8":       19,
	"F9":       20,
	"F10":      21,
	"F11":      23,
	"F12":      24,
	"F13":      25,
	"F14":      26,
	"F15":      28,
	"F16":      29,
	"F17":      31,
	"F18":      32,
	"F19":      33,
	"F20":      34,
}

var csiUCodepoints = map[string]int{
	"Backspace": 8,
	"Tab":       9,
	"Enter":     13,
	"Esc":       27,
	"Space":     32,
}
