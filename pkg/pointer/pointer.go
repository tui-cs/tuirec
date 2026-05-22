// Package pointer provides mouse pointer visualization for terminal recordings.
package pointer

import (
	"fmt"
	"strconv"
	"strings"
)

// Mode controls which mouse events trigger the pointer indicator.
type Mode int

const (
	// None disables the pointer indicator.
	None Mode = iota
	// Clicks shows the pointer on clicks, drags, and scrolls.
	Clicks
	// All shows the pointer on all mouse events including move/hover.
	All
)

// ParseMode parses a mode string. Valid values: "none", "clicks", "all".
func ParseMode(s string) (Mode, error) {
	switch strings.ToLower(s) {
	case "none":
		return None, nil
	case "clicks":
		return Clicks, nil
	case "all":
		return All, nil
	default:
		return None, fmt.Errorf("invalid mouse-pointer mode %q: must be none, clicks, or all", s)
	}
}

// String returns the mode name.
func (m Mode) String() string {
	switch m {
	case None:
		return "none"
	case Clicks:
		return "clicks"
	case All:
		return "all"
	default:
		return fmt.Sprintf("mode(%d)", m)
	}
}

// Indicator generates ANSI sequences to show/hide a pointer character.
type Indicator struct {
	style   string
	lastCol int
	lastRow int
	visible bool
}

// NewIndicator creates an indicator with the given pointer character.
func NewIndicator(style string) *Indicator {
	if style == "" {
		style = "●"
	}
	return &Indicator{style: style}
}

// Show returns ANSI sequences to display the pointer at (col, row).
// Coordinates are 1-based. If a pointer is already visible at a different
// position, the returned sequence hides it first.
func (ind *Indicator) Show(col, row int) string {
	var b strings.Builder
	if ind.visible && (ind.lastCol != col || ind.lastRow != row) {
		b.WriteString(hide(ind.lastCol, ind.lastRow))
	}
	// Save cursor, move to position, bold bright yellow, pointer char, reset, restore cursor.
	b.WriteString("\x1b7")
	b.WriteString("\x1b[")
	b.WriteString(strconv.Itoa(row))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(col))
	b.WriteByte('H')
	b.WriteString("\x1b[1;93m")
	b.WriteString(ind.style)
	b.WriteString("\x1b[0m")
	b.WriteString("\x1b8")

	ind.lastCol = col
	ind.lastRow = row
	ind.visible = true
	return b.String()
}

// Hide returns ANSI sequences to clear the currently visible pointer.
// Returns empty string if no pointer is visible.
func (ind *Indicator) Hide() string {
	if !ind.visible {
		return ""
	}
	ind.visible = false
	return hide(ind.lastCol, ind.lastRow)
}

func hide(col, row int) string {
	var b strings.Builder
	b.WriteString("\x1b7")
	b.WriteString("\x1b[")
	b.WriteString(strconv.Itoa(row))
	b.WriteByte(';')
	b.WriteString(strconv.Itoa(col))
	b.WriteString("H \x1b8")
	return b.String()
}

// ShouldShow reports whether the given action label should trigger the pointer.
func ShouldShow(mode Mode, label string) bool {
	if mode == None {
		return false
	}
	if mode == All {
		return isMouseLabel(label)
	}
	// Clicks mode: clicks, drags, scrolls — but not move/hover.
	return isClickLabel(label)
}

// Position extracts (col, row) from a mouse action label.
// Returns (0, 0, false) if the label is not a recognized mouse token.
func Position(label string) (col, row int, ok bool) {
	// Strip modifier prefix (e.g., "Ctrl+click:10:5" -> "click:10:5").
	token := stripModifiers(label)

	switch {
	case strings.HasPrefix(token, "click:"),
		strings.HasPrefix(token, "rightclick:"),
		strings.HasPrefix(token, "middleclick:"),
		strings.HasPrefix(token, "doubleclick:"):
		return parseColRow(token)
	case strings.HasPrefix(token, "scroll:"):
		return parseScrollPos(token)
	case strings.HasPrefix(token, "drag:"):
		return parseDragEnd(token)
	case strings.HasPrefix(token, "move:"),
		strings.HasPrefix(token, "hover:"):
		return parseColRow(token)
	}
	return 0, 0, false
}

func stripModifiers(label string) string {
	parts := strings.Split(label, "+")
	if len(parts) <= 1 {
		return label
	}
	// The last part after the final "+" that contains ":" is the mouse token.
	last := parts[len(parts)-1]
	if strings.Contains(last, ":") {
		return last
	}
	return label
}

func parseColRow(token string) (int, int, bool) {
	// token is "prefix:col:row"
	idx := strings.Index(token, ":")
	if idx < 0 {
		return 0, 0, false
	}
	rest := token[idx+1:]
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	col, err1 := strconv.Atoi(parts[0])
	row, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || col < 1 || row < 1 {
		return 0, 0, false
	}
	return col, row, true
}

func parseScrollPos(token string) (int, int, bool) {
	// "scroll:dir:col:row"
	idx := strings.Index(token, ":")
	if idx < 0 {
		return 0, 0, false
	}
	rest := token[idx+1:] // "dir:col:row"
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) != 3 {
		return 0, 0, false
	}
	col, err1 := strconv.Atoi(parts[1])
	row, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || col < 1 || row < 1 {
		return 0, 0, false
	}
	return col, row, true
}

func parseDragEnd(token string) (int, int, bool) {
	// "drag:col1:row1:col2:row2" — show pointer at drag endpoint.
	idx := strings.Index(token, ":")
	if idx < 0 {
		return 0, 0, false
	}
	rest := token[idx+1:] // "col1:row1:col2:row2"
	parts := strings.SplitN(rest, ":", 4)
	if len(parts) != 4 {
		return 0, 0, false
	}
	col, err1 := strconv.Atoi(parts[2])
	row, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil || col < 1 || row < 1 {
		return 0, 0, false
	}
	return col, row, true
}

func isMouseLabel(label string) bool {
	token := stripModifiers(label)
	for _, prefix := range []string{"click:", "rightclick:", "middleclick:", "doubleclick:", "scroll:", "drag:", "move:", "hover:"} {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}

func isClickLabel(label string) bool {
	token := stripModifiers(label)
	for _, prefix := range []string{"click:", "rightclick:", "middleclick:", "doubleclick:", "scroll:", "drag:"} {
		if strings.HasPrefix(token, prefix) {
			return true
		}
	}
	return false
}
