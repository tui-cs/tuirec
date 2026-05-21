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
	switch {
	case strings.HasPrefix(token, "click:"):
		return parseMouseClick(token, "click:", 0)
	case strings.HasPrefix(token, "rightclick:"):
		return parseMouseClick(token, "rightclick:", 2)
	case strings.HasPrefix(token, "middleclick:"):
		return parseMouseClick(token, "middleclick:", 1)
	case strings.HasPrefix(token, "doubleclick:"):
		return parseDoubleClick(token)
	case strings.HasPrefix(token, "scroll:"):
		return parseScroll(token)
	case strings.HasPrefix(token, "drag:"):
		return parseDrag(token)
	case strings.HasPrefix(token, "move:") || strings.HasPrefix(token, "hover:"):
		return parseMouseMove(token)
	default:
		return "", false, nil
	}
}

func parseMouseClick(token, prefix string, button int) (string, bool, error) {
	value := token[len(prefix):]
	col, row, err := parseColRow(value, token)
	if err != nil {
		return "", true, err
	}

	return sgrPressRelease(button, col, row), true, nil
}

func parseDoubleClick(token string) (string, bool, error) {
	value := token[len("doubleclick:"):]
	col, row, err := parseColRow(value, token)
	if err != nil {
		return "", true, err
	}

	// Two rapid left press+release pairs.
	seq := sgrPressRelease(0, col, row) + sgrPressRelease(0, col, row)
	return seq, true, nil
}

func parseScroll(token string) (string, bool, error) {
	value := token[len("scroll:"):]
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
	return sgrPress(button, col, row), true, nil
}

func parseDrag(token string) (string, bool, error) {
	value := token[len("drag:"):]
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		return "", true, fmt.Errorf("invalid drag token (expected drag:col1:row1:col2:row2): %s", token)
	}

	for _, p := range parts {
		if !allDigits(p) || p == "" {
			return "", true, fmt.Errorf("invalid drag token (non-numeric coordinate): %s", token)
		}
	}

	col1, _ := strconv.Atoi(parts[0])
	row1, _ := strconv.Atoi(parts[1])
	col2, _ := strconv.Atoi(parts[2])
	row2, _ := strconv.Atoi(parts[3])

	if col1 < 1 || row1 < 1 || col2 < 1 || row2 < 1 {
		return "", true, fmt.Errorf("mouse coordinates are 1-based: %s", token)
	}

	// Press at start, motion at end, release at end.
	seq := sgrPress(0, col1, row1) +
		sgrPress(32, col2, row2) +
		sgrRelease(0, col2, row2)
	return seq, true, nil
}

// parseMouseMove handles move: or hover: tokens for mouse motion events (used
// to trigger hover effects in TUIs that support mouse tracking).
func parseMouseMove(token string) (string, bool, error) {
	prefix := "move:"
	if strings.HasPrefix(token, "hover:") {
		prefix = "hover:"
	}
	value := token[len(prefix):]
	col, row, err := parseColRow(value, token)
	if err != nil {
		return "", true, err
	}
	// SGR extended mouse motion event (32 = motion flag, M = button press/motion).
	return fmt.Sprintf("\x1b[<32;%d;%dM", col, row), true, nil
}

func parseColRow(value, token string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 || !allDigits(parts[0]) || !allDigits(parts[1]) || parts[0] == "" || parts[1] == "" {
		return 0, 0, fmt.Errorf("invalid mouse token (expected col:row): %s", token)
	}

	col, _ := strconv.Atoi(parts[0])
	row, _ := strconv.Atoi(parts[1])

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

func modifierRest(token, modifier string) (string, bool) {
	for _, separator := range []string{"+", "-"} {
		prefix := modifier + separator
		if len(token) > len(prefix) && strings.EqualFold(token[:len(prefix)], prefix) {
			return token[len(prefix):], true
		}
	}

	return "", false
}

func hasModifierPrefix(token, modifier string) bool {
	if len(token) <= len(modifier) {
		return false
	}

	return strings.EqualFold(token[:len(modifier)], modifier) && (token[len(modifier)] == '+' || token[len(modifier)] == '-')
}

func isKeyIdentifier(token string) bool {
	if token == "" {
		return false
	}

	for _, r := range token {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}

	return true
}

func startsWithUpperIdentifier(token string) bool {
	if token == "" {
		return false
	}

	first := token[0]
	return first >= 'A' && first <= 'Z' && isKeyIdentifier(token)
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
