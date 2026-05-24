// Package castfix post-processes asciinema v2 cast files to inject explicit
// CUP (Cursor Position) sequences at row boundaries. This fixes visual
// tearing caused by renderers (like agg) that miscount wide character widths.
//
// When a terminal app emits a full-screen frame relying on auto-wrap at the
// configured width, a renderer with incorrect wcwidth will accumulate drift.
// By injecting \x1b[row;1H at each wrap point, the cast becomes self-correcting
// regardless of the renderer's width tables.
package castfix

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/gui-cs/tuirec/pkg/cellwidth"
)

// Fix reads an asciinema v2 cast from input, injects CUP sequences at row
// boundaries based on the terminal width (read from the cast header), and
// writes the fixed cast to output. The operation is idempotent.
//
// If cols is > 0, it overrides the width from the cast header.
func Fix(input io.Reader, output io.Writer, cols int) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	// First line is the JSON header.
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("read cast header: %w", err)
		}
		return fmt.Errorf("empty cast file")
	}

	headerLine := scanner.Text()

	// Extract width from header if cols not explicitly provided.
	if cols <= 0 {
		var hdr struct {
			Width int `json:"width"`
		}
		if err := json.Unmarshal([]byte(headerLine), &hdr); err == nil && hdr.Width > 0 {
			cols = hdr.Width
		} else {
			cols = 120 // fallback default
		}
	}

	if _, err := fmt.Fprintf(output, "%s\n", headerLine); err != nil {
		return fmt.Errorf("write cast header: %w", err)
	}

	state := newCursorState(cols)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event [3]json.RawMessage
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Pass through non-event lines unchanged.
			if _, err := fmt.Fprintf(output, "%s\n", line); err != nil {
				return fmt.Errorf("write passthrough line: %w", err)
			}
			continue
		}

		var timestamp float64
		if err := json.Unmarshal(event[0], &timestamp); err != nil {
			if _, err := fmt.Fprintf(output, "%s\n", line); err != nil {
				return fmt.Errorf("write passthrough line: %w", err)
			}
			continue
		}

		var eventType string
		if err := json.Unmarshal(event[1], &eventType); err != nil {
			if _, err := fmt.Fprintf(output, "%s\n", line); err != nil {
				return fmt.Errorf("write passthrough line: %w", err)
			}
			continue
		}

		// Only process output events.
		if eventType != "o" {
			if _, err := fmt.Fprintf(output, "%s\n", line); err != nil {
				return fmt.Errorf("write non-output event: %w", err)
			}
			continue
		}

		var data string
		if err := json.Unmarshal(event[2], &data); err != nil {
			if _, err := fmt.Fprintf(output, "%s\n", line); err != nil {
				return fmt.Errorf("write malformed event: %w", err)
			}
			continue
		}

		fixed := state.process(data)

		fixedJSON, err := json.Marshal([]any{timestamp, "o", fixed})
		if err != nil {
			return fmt.Errorf("marshal fixed event: %w", err)
		}

		if _, err := fmt.Fprintf(output, "%s\n", fixedJSON); err != nil {
			return fmt.Errorf("write fixed event: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read cast: %w", err)
	}

	return nil
}

// cursorState tracks the cursor position as VT output is processed.
type cursorState struct {
	col  int // current column (0-based)
	row  int // current row (1-based for CUP sequences)
	cols int // terminal width
}

func newCursorState(cols int) *cursorState {
	return &cursorState{col: 0, row: 1, cols: cols}
}

// process scans a VT output string, tracking cursor position and injecting
// CUP sequences when the cursor wraps past the terminal width.
func (s *cursorState) process(data string) string {
	var out strings.Builder
	out.Grow(len(data) + len(data)/10) // slight overalloc for injected sequences

	i := 0
	for i < len(data) {
		b := data[i]

		// ESC sequence.
		if b == 0x1B && i+1 < len(data) {
			seqEnd := s.handleEscape(data, i)
			out.WriteString(data[i:seqEnd])
			i = seqEnd
			continue
		}

		// Carriage return.
		if b == '\r' {
			s.col = 0
			out.WriteByte(b)
			i++
			continue
		}

		// Line feed / vertical tab / form feed.
		if b == '\n' || b == 0x0B || b == 0x0C {
			s.col = 0
			s.row++
			out.WriteByte(b)
			i++
			continue
		}

		// Backspace.
		if b == 0x08 {
			if s.col > 0 {
				s.col--
			}
			out.WriteByte(b)
			i++
			continue
		}

		// Tab: advance to next multiple of 8.
		if b == '\t' {
			nextTab := (s.col/8 + 1) * 8
			if nextTab > s.cols {
				nextTab = s.cols
			}
			s.col = nextTab
			out.WriteByte(b)
			i++
			continue
		}

		// Other C0 control characters: pass through, no column advance.
		if b < 0x20 {
			out.WriteByte(b)
			i++
			continue
		}

		// Printable character: decode UTF-8 rune.
		r, size := utf8.DecodeRuneInString(data[i:])
		w := cellwidth.RuneWidth(r)

		// Check if printing this character would exceed terminal width.
		if w > 0 && s.col+w > s.cols {
			// Auto-wrap: move to next row, column 0.
			s.row++
			s.col = 0
			// Inject CUP sequence to explicitly position cursor.
			cup := fmt.Sprintf("\x1b[%d;1H", s.row)
			out.WriteString(cup)
		}

		out.WriteString(data[i : i+size])
		s.col += w
		i += size

		// If we've reached exactly the terminal width, the next printable
		// character will trigger a wrap (handled above).
	}

	return out.String()
}

// handleEscape processes an escape sequence starting at position i in data.
// It updates cursor state for cursor-movement sequences and returns the index
// past the end of the sequence.
func (s *cursorState) handleEscape(data string, i int) int {
	if i+1 >= len(data) {
		return i + 1
	}

	// CSI sequence: ESC [
	if data[i+1] == '[' {
		return s.handleCSI(data, i)
	}

	// OSC sequence: ESC ]
	if data[i+1] == ']' {
		return s.handleOSC(data, i)
	}

	// Two-character escape sequences (e.g., ESC M, ESC D, ESC 7, ESC 8).
	switch data[i+1] {
	case 'M': // Reverse Index: move cursor up one row.
		if s.row > 1 {
			s.row--
		}
	case 'D': // Index: move cursor down one row.
		s.row++
		s.col = 0
	}

	return i + 2
}

// handleCSI processes a CSI (Control Sequence Introducer) sequence.
func (s *cursorState) handleCSI(data string, start int) int {
	// Skip ESC [
	i := start + 2
	if i >= len(data) {
		return i
	}

	// Collect parameter bytes (digits, semicolons, question marks).
	paramStart := i
	for i < len(data) && ((data[i] >= '0' && data[i] <= '9') || data[i] == ';' || data[i] == '?' || data[i] == '>' || data[i] == '!') {
		i++
	}

	// Collect intermediate bytes (0x20-0x2F).
	for i < len(data) && data[i] >= 0x20 && data[i] <= 0x2F {
		i++
	}

	// Final byte.
	if i >= len(data) {
		return i
	}

	finalByte := data[i]
	params := data[paramStart:i]
	i++ // consume final byte

	// Update cursor state based on the sequence.
	switch finalByte {
	case 'H', 'f': // CUP: Cursor Position
		row, col := parseCSIParams2(params, 1, 1)
		s.row = row
		s.col = col - 1 // convert to 0-based
	case 'A': // CUU: Cursor Up
		n := parseCSIParam1(params, 1)
		s.row -= n
		if s.row < 1 {
			s.row = 1
		}
	case 'B': // CUD: Cursor Down
		n := parseCSIParam1(params, 1)
		s.row += n
	case 'C': // CUF: Cursor Forward
		n := parseCSIParam1(params, 1)
		s.col += n
		if s.col >= s.cols {
			s.col = s.cols - 1
		}
	case 'D': // CUB: Cursor Back
		n := parseCSIParam1(params, 1)
		s.col -= n
		if s.col < 0 {
			s.col = 0
		}
	case 'E': // CNL: Cursor Next Line
		n := parseCSIParam1(params, 1)
		s.row += n
		s.col = 0
	case 'F': // CPL: Cursor Previous Line
		n := parseCSIParam1(params, 1)
		s.row -= n
		if s.row < 1 {
			s.row = 1
		}
		s.col = 0
	case 'G': // CHA: Cursor Horizontal Absolute
		n := parseCSIParam1(params, 1)
		s.col = n - 1
	case 'd': // VPA: Vertical Position Absolute
		n := parseCSIParam1(params, 1)
		s.row = n
	}

	return i
}

// handleOSC processes an OSC sequence (ESC ] ... ST or ESC ] ... BEL).
func (s *cursorState) handleOSC(data string, start int) int {
	i := start + 2
	for i < len(data) {
		if data[i] == 0x07 { // BEL terminates OSC
			return i + 1
		}
		if data[i] == 0x1B && i+1 < len(data) && data[i+1] == '\\' { // ST terminates OSC
			return i + 2
		}
		i++
	}
	return i
}

// parseCSIParam1 parses a single numeric parameter with a default value.
func parseCSIParam1(params string, defaultVal int) int {
	if params == "" || params == "0" {
		return defaultVal
	}
	n := 0
	for _, b := range params {
		if b >= '0' && b <= '9' {
			n = n*10 + int(b-'0')
		} else {
			break
		}
	}
	if n == 0 {
		return defaultVal
	}
	return n
}

// parseCSIParams2 parses two semicolon-separated numeric parameters.
func parseCSIParams2(params string, default1, default2 int) (int, int) {
	parts := strings.SplitN(params, ";", 2)
	v1 := default1
	v2 := default2
	if len(parts) >= 1 && parts[0] != "" {
		v1 = parseCSIParam1(parts[0], default1)
	}
	if len(parts) >= 2 && parts[1] != "" {
		v2 = parseCSIParam1(parts[1], default2)
	}
	return v1, v2
}
