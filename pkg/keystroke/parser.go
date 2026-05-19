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
	if milliseconds, ok := parseWait(token); ok {
		return Action{Kind: Wait, Delay: time.Duration(milliseconds) * time.Millisecond}, nil
	}

	if sequence, ok, err := parseClick(token); ok || err != nil {
		return Action{Kind: Write, Sequence: sequence}, err
	}

	if sequence, ok := ResolveNamedKey(token); ok {
		return Action{Kind: Write, Sequence: sequence}, nil
	}

	return Action{Kind: Literal, Sequence: token}, nil
}

func parseWait(token string) (int, bool) {
	value, ok := strings.CutPrefix(token, "wait:")
	if !ok || value == "" || !allDigits(value) {
		return 0, false
	}

	milliseconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}

	return milliseconds, true
}

func parseClick(token string) (string, bool, error) {
	value, ok := strings.CutPrefix(token, "click:")
	if !ok {
		return "", false, nil
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 || !allDigits(parts[0]) || !allDigits(parts[1]) {
		return "", false, nil
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
