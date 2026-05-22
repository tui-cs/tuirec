package keystroke

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Kind identifies the type of parsed script action.
type Kind int

const (
	// Wait pauses playback without writing input.
	Wait Kind = iota
	// Write sends a byte sequence to the terminal.
	Write
	// Literal writes text rune-by-rune.
	Literal
)

// Action is one parsed keystroke script action.
type Action struct {
	Kind     Kind
	Sequence string
	Label    string
	Delay    time.Duration
}

// Parse parses a comma-separated keystroke script.
func Parse(script string) ([]Action, error) {
	tokens, err := splitTokens(script)
	if err != nil {
		return nil, err
	}

	actions := make([]Action, 0, len(tokens))
	for _, token := range tokens {
		action, err := parseToken(token)
		if err != nil {
			return nil, err
		}

		actions = append(actions, action)
	}

	return actions, nil
}

func parseToken(token string) (Action, error) {
	// Backtick-quoted tokens are always literal text.
	if len(token) >= 2 && token[0] == '`' && token[len(token)-1] == '`' {
		text := token[1 : len(token)-1]
		return Action{Kind: Literal, Sequence: text, Label: token}, nil
	}

	if milliseconds, ok, err := parseWait(token); ok || err != nil {
		if err != nil {
			return Action{}, err
		}

		return Action{Kind: Wait, Label: token, Delay: time.Duration(milliseconds) * time.Millisecond}, nil
	}

	if sequence, ok, err := parseMouse(token); ok || err != nil {
		return Action{Kind: Write, Sequence: sequence, Label: token}, err
	}

	if sequence, ok, err := resolveTerminalGUIKey(token); ok || err != nil {
		if err != nil {
			return Action{}, err
		}

		return Action{Kind: Write, Sequence: sequence, Label: token}, nil
	}

	// Single printable characters that didn't resolve as named keys are typed
	// as literal (e.g. punctuation, digits in certain contexts).
	if len(token) == 1 {
		return Action{Kind: Literal, Sequence: token, Label: token}, nil
	}

	return Action{}, fmt.Errorf("unrecognized token %q: use backticks for literal text (`%s`), or check key name spelling", token, token)
}

func parseWait(token string) (int, bool, error) {
	value, ok := strings.CutPrefix(token, "wait:")
	if !ok {
		return 0, false, nil
	}

	if value == "" || !allDigits(value) {
		return 0, true, fmt.Errorf("invalid wait token: %s", token)
	}

	milliseconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, true, fmt.Errorf("parse wait duration: %w", err)
	}

	return milliseconds, true, nil
}

func parseMouse(token string) (string, bool, error) {
	mouseToken, mouseMods, hasMouseMods := parseMouseModifierPrefix(token)
	switch {
	case strings.HasPrefix(mouseToken, "click:"):
		return parseMouseClick(token, mouseToken, "click:", 0, mouseMods)
	case strings.HasPrefix(mouseToken, "rightclick:"):
		return parseMouseClick(token, mouseToken, "rightclick:", 2, mouseMods)
	case strings.HasPrefix(mouseToken, "middleclick:"):
		return parseMouseClick(token, mouseToken, "middleclick:", 1, mouseMods)
	case strings.HasPrefix(mouseToken, "doubleclick:"):
		return parseDoubleClick(token, mouseToken, mouseMods)
	case strings.HasPrefix(mouseToken, "scroll:"):
		return parseScroll(token, mouseToken, mouseMods)
	case strings.HasPrefix(mouseToken, "drag:"):
		return parseDrag(token, mouseToken, mouseMods)
	case strings.HasPrefix(mouseToken, "move:") || strings.HasPrefix(mouseToken, "hover:"):
		return parseMouseMove(token, mouseToken, mouseMods)
	default:
		if hasMouseMods && strings.Contains(mouseToken, ":") {
			return "", true, fmt.Errorf("invalid modified mouse token: %s", token)
		}
		return "", false, nil
	}
}

func parseMouseModifierPrefix(token string) (string, int, bool) {
	parts := strings.Split(token, "+")
	if len(parts) == 1 {
		return token, 0, false
	}

	mods := 0
	for _, part := range parts[:len(parts)-1] {
		modifier, ok := parseModifier(part)
		if !ok {
			return token, 0, false
		}
		mods |= modifier
	}

	return parts[len(parts)-1], mouseModifierBits(mods), true
}

func parseMouseClick(token, mouseToken, prefix string, button, mouseMods int) (string, bool, error) {
	value := mouseToken[len(prefix):]
	col, row, err := parseColRow(value, token)
	if err != nil {
		return "", true, err
	}

	return sgrPressRelease(button+mouseMods, col, row), true, nil
}

func parseDoubleClick(token, mouseToken string, mouseMods int) (string, bool, error) {
	value := mouseToken[len("doubleclick:"):]
	col, row, err := parseColRow(value, token)
	if err != nil {
		return "", true, err
	}

	// Two rapid left press+release pairs.
	seq := sgrPressRelease(mouseMods, col, row) + sgrPressRelease(mouseMods, col, row)
	return seq, true, nil
}

func parseScroll(token, mouseToken string, mouseMods int) (string, bool, error) {
	value := mouseToken[len("scroll:"):]
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 {
		return "", true, fmt.Errorf("invalid scroll token (expected scroll:up|down:col:row): %s", token)
	}

	var button int
	switch parts[0] {
	case "up":
		button = 64
	case "down":
		button = 65
	default:
		return "", true, fmt.Errorf("invalid scroll direction %q (expected up or down): %s", parts[0], token)
	}

	colRow := parts[1] + ":" + parts[2]
	col, row, err := parseColRow(colRow, token)
	if err != nil {
		return "", true, err
	}

	// Scroll events have no release.
	return sgrPress(button+mouseMods, col, row), true, nil
}

func parseDrag(token, mouseToken string, mouseMods int) (string, bool, error) {
	value := mouseToken[len("drag:"):]
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		return "", true, fmt.Errorf("invalid drag token (expected drag:col1:row1:col2:row2): %s", token)
	}

	for _, p := range parts {
		if !allDigits(p) || p == "" {
			return "", true, fmt.Errorf("invalid drag token (non-numeric coordinate): %s", token)
		}
	}

	col1, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", true, fmt.Errorf("invalid drag coordinate: %w", err)
	}
	row1, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", true, fmt.Errorf("invalid drag coordinate: %w", err)
	}
	col2, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", true, fmt.Errorf("invalid drag coordinate: %w", err)
	}
	row2, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", true, fmt.Errorf("invalid drag coordinate: %w", err)
	}

	if col1 < 1 || row1 < 1 || col2 < 1 || row2 < 1 {
		return "", true, fmt.Errorf("mouse coordinates are 1-based: %s", token)
	}

	// Press at start, interpolated motion events, release at end.
	seq := sgrPress(mouseMods, col1, row1)

	// Generate intermediate motion events for smooth dragging.
	dx := col2 - col1
	dy := row2 - row1
	steps := abs(dx)
	if abs(dy) > steps {
		steps = abs(dy)
	}
	if steps == 0 {
		steps = 1
	}
	for i := 1; i <= steps; i++ {
		cx := col1 + dx*i/steps
		cy := row1 + dy*i/steps
		seq += sgrPress(32+mouseMods, cx, cy)
	}

	seq += sgrRelease(mouseMods, col2, row2)
	return seq, true, nil
}

// parseMouseMove handles move: or hover: tokens for mouse motion events (used
// to trigger hover effects in TUIs that support mouse tracking).
func parseMouseMove(token, mouseToken string, mouseMods int) (string, bool, error) {
	prefix := "move:"
	if strings.HasPrefix(mouseToken, "hover:") {
		prefix = "hover:"
	}
	value := mouseToken[len(prefix):]
	col, row, err := parseColRow(value, token)
	if err != nil {
		return "", true, err
	}
	// SGR extended mouse motion event (32 = motion flag, M = button press/motion).
	return fmt.Sprintf("\x1b[<%d;%d;%dM", 32+mouseMods, col, row), true, nil
}

func parseColRow(value, token string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 || !allDigits(parts[0]) || !allDigits(parts[1]) || parts[0] == "" || parts[1] == "" {
		return 0, 0, fmt.Errorf("invalid mouse token (expected col:row): %s", token)
	}

	col, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid mouse coordinate: %w", err)
	}
	row, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid mouse coordinate: %w", err)
	}

	if col < 1 || row < 1 {
		return 0, 0, fmt.Errorf("mouse coordinates are 1-based: %s", token)
	}

	return col, row, nil
}

// sgrPress emits an SGR mouse press sequence.
func sgrPress(button, col, row int) string {
	return fmt.Sprintf("\x1b[<%d;%d;%dM", button, col, row)
}

// sgrRelease emits an SGR mouse release sequence.
func sgrRelease(button, col, row int) string {
	return fmt.Sprintf("\x1b[<%d;%d;%dm", button, col, row)
}

// sgrPressRelease emits an SGR press followed by release.
func sgrPressRelease(button, col, row int) string {
	return sgrPress(button, col, row) + sgrRelease(button, col, row)
}

func mouseModifierBits(mods int) int {
	bits := 0
	if mods&modShift != 0 {
		bits += 4
	}
	if mods&modAlt != 0 {
		bits += 8
	}
	if mods&modCtrl != 0 {
		bits += 16
	}

	return bits
}

func splitTokens(script string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	escaping := false
	inBacktick := false

	for _, r := range script {
		if escaping {
			switch r {
			case ',', '\\':
				current.WriteRune(r)
			default:
				current.WriteRune('\\')
				current.WriteRune(r)
			}
			escaping = false

			continue
		}

		if r == '`' {
			inBacktick = !inBacktick
			current.WriteRune(r)
			continue
		}

		if inBacktick {
			current.WriteRune(r)
			continue
		}

		switch r {
		case '\\':
			escaping = true
		case ',':
			tokens = append(tokens, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	if escaping {
		return nil, fmt.Errorf("script ends with dangling escape")
	}
	if inBacktick {
		return nil, fmt.Errorf("script has unclosed backtick")
	}

	tokens = append(tokens, current.String())

	return tokens, nil
}

func allDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
