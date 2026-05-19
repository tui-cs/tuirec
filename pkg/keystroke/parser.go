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
	if milliseconds, ok, err := parseWait(token); ok || err != nil {
		if err != nil {
			return Action{}, err
		}

		return Action{Kind: Wait, Delay: time.Duration(milliseconds) * time.Millisecond}, nil
	}

	if sequence, ok, err := parseClick(token); ok || err != nil {
		return Action{Kind: Write, Sequence: sequence}, err
	}

	if sequence, ok, err := resolveTerminalGUIKey(token); ok || err != nil {
		if err != nil {
			return Action{}, err
		}

		return Action{Kind: Write, Sequence: sequence}, nil
	}

	if looksLikeKey(token) {
		return Action{}, fmt.Errorf("unknown key: %s", token)
	}

	return Action{Kind: Literal, Sequence: token}, nil
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

func parseClick(token string) (string, bool, error) {
	value, ok := strings.CutPrefix(token, "click:")
	if !ok {
		return "", false, nil
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 || !allDigits(parts[0]) || !allDigits(parts[1]) {
		return "", true, fmt.Errorf("invalid click token: %s", token)
	}

	col, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", true, fmt.Errorf("parse click column: %w", err)
	}

	row, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", true, fmt.Errorf("parse click row: %w", err)
	}

	if col < 1 || row < 1 {
		return "", true, fmt.Errorf("click coordinates are 1-based: %s", token)
	}

	return fmt.Sprintf("\x1b[<0;%d;%dM\x1b[<0;%d;%dm", col, row, col, row), true, nil
}

func looksLikeKey(token string) bool {
	if rest, ok := modifierRest(token, "Ctrl"); ok {
		return isKeyIdentifier(rest)
	}
	if rest, ok := modifierRest(token, "Alt"); ok {
		return startsWithUpperIdentifier(rest)
	}
	if rest, ok := modifierRest(token, "Shift"); ok {
		return isKeyIdentifier(rest)
	}
	if hasModifierPrefix(token, "Ctrl") {
		return true
	}
	if hasModifierPrefix(token, "Alt") {
		return len(token) == len("Alt+") || startsWithUpperIdentifier(token[len("Alt+"):])
	}
	if hasModifierPrefix(token, "Shift") {
		return true
	}
	lower := strings.ToLower(token)
	if isKeyIdentifier(token) && (strings.HasPrefix(lower, "arrow") || strings.HasPrefix(lower, "cursor") || strings.HasPrefix(lower, "page")) {
		return true
	}
	if (strings.HasPrefix(token, "F") || strings.HasPrefix(token, "f")) && len(token) > 1 && allDigits(token[1:]) {
		return true
	}

	return false
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
