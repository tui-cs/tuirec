package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultFrameCols = 120
	defaultFrameRows = 30
)

type castHeader struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ExtractFrameText replays cast output events into a terminal grid and returns
// the final frame text with trailing whitespace trimmed per line.
func ExtractFrameText(castPath string) (string, error) {
	data, err := os.ReadFile(castPath)
	if err != nil {
		return "", fmt.Errorf("read cast: %w", err)
	}

	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	if len(lines) == 0 {
		return "", nil
	}

	cols, rows := parseCastSize(lines[0])
	grid := newFrameGrid(cols, rows)
	for _, line := range lines[1:] {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		event, ok, err := parseOutputEvent(line)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		grid.write(event.output)
	}

	return grid.text(), nil
}

func parseCastSize(line []byte) (int, int) {
	cols, rows := defaultFrameCols, defaultFrameRows
	var header castHeader
	if err := json.Unmarshal(line, &header); err != nil {
		return cols, rows
	}
	if header.Width > 0 {
		cols = header.Width
	}
	if header.Height > 0 {
		rows = header.Height
	}

	return cols, rows
}

type frameGrid struct {
	cells [][]rune
	cols  int
	rows  int
	x     int
	y     int
}

func newFrameGrid(cols, rows int) *frameGrid {
	cells := make([][]rune, rows)
	for i := range cells {
		cells[i] = make([]rune, cols)
		for j := range cells[i] {
			cells[i][j] = ' '
		}
	}
	return &frameGrid{
		cells: cells,
		cols:  cols,
		rows:  rows,
	}
}

func (g *frameGrid) text() string {
	lines := make([]string, g.rows)
	for i, row := range g.cells {
		lines[i] = strings.TrimRight(string(row), " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (g *frameGrid) write(output string) {
	for i := 0; i < len(output); {
		r, size := utf8.DecodeRuneInString(output[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}

		switch r {
		case '\r':
			g.x = 0
			i += size
			continue
		case '\n':
			g.newLine()
			i += size
			continue
		case '\b':
			if g.x > 0 {
				g.x--
			}
			i += size
			continue
		case '\t':
			nextTab := ((g.x / 8) + 1) * 8
			for g.x < nextTab {
				g.put(' ')
			}
			i += size
			continue
		case '\x1b':
			i = g.handleEscape(output, i+size)
			continue
		case '\x9b':
			i = g.handleCSI(output, i+size)
			continue
		}

		if unicode.IsPrint(r) {
			g.put(r)
		}
		i += size
	}
}

func (g *frameGrid) put(r rune) {
	if g.y < 0 || g.y >= g.rows {
		return
	}
	if g.x < 0 {
		g.x = 0
	}
	if g.x >= g.cols {
		g.newLine()
	}
	if g.y < 0 || g.y >= g.rows || g.x < 0 || g.x >= g.cols {
		return
	}

	g.cells[g.y][g.x] = r
	g.x++
}

func (g *frameGrid) newLine() {
	g.x = 0
	g.y++
	if g.y < g.rows {
		return
	}
	copy(g.cells[0:], g.cells[1:])
	last := make([]rune, g.cols)
	for i := range last {
		last[i] = ' '
	}
	g.cells[g.rows-1] = last
	g.y = g.rows - 1
}

func (g *frameGrid) clear() {
	for i := range g.cells {
		for j := range g.cells[i] {
			g.cells[i][j] = ' '
		}
	}
	g.x = 0
	g.y = 0
}

func (g *frameGrid) clearLine(mode int) {
	switch mode {
	case 1:
		for col := 0; col <= g.x && col < g.cols; col++ {
			g.cells[g.y][col] = ' '
		}
	case 2:
		for col := 0; col < g.cols; col++ {
			g.cells[g.y][col] = ' '
		}
	default:
		for col := g.x; col < g.cols; col++ {
			g.cells[g.y][col] = ' '
		}
	}
}

func (g *frameGrid) clearScreen(mode int) {
	switch mode {
	case 1:
		for row := 0; row <= g.y && row < g.rows; row++ {
			limit := g.cols
			if row == g.y {
				limit = g.x + 1
			}
			for col := 0; col < limit && col < g.cols; col++ {
				g.cells[row][col] = ' '
			}
		}
	case 2, 3:
		g.clear()
	default:
		for row := g.y; row < g.rows; row++ {
			start := 0
			if row == g.y {
				start = g.x
			}
			for col := start; col < g.cols; col++ {
				g.cells[row][col] = ' '
			}
		}
	}
}

func (g *frameGrid) handleEscape(s string, i int) int {
	if i >= len(s) {
		return i
	}

	switch s[i] {
	case '[':
		return g.handleCSI(s, i+1)
	case ']':
		return skipStringEscape(s, i+1)
	default:
		_, size := utf8.DecodeRuneInString(s[i:])
		return i + size
	}
}

func (g *frameGrid) handleCSI(s string, i int) int {
	start := i
	for i < len(s) {
		b := s[i]
		if b >= 0x40 && b <= 0x7e {
			params := s[start:i]
			g.applyCSI(b, params)
			return i + 1
		}
		i++
	}
	return i
}

func (g *frameGrid) applyCSI(final byte, params string) {
	args := parseCSIArgs(params)
	switch final {
	case 'A':
		g.y -= csiArg(args, 0, 1)
	case 'B':
		g.y += csiArg(args, 0, 1)
	case 'C':
		g.x += csiArg(args, 0, 1)
	case 'D':
		g.x -= csiArg(args, 0, 1)
	case 'G':
		g.x = csiArg(args, 0, 1) - 1
	case 'H', 'f':
		g.y = csiArg(args, 0, 1) - 1
		g.x = csiArg(args, 1, 1) - 1
	case 'd':
		g.y = csiArg(args, 0, 1) - 1
	case 'J':
		g.clearScreen(csiArg(args, 0, 0))
	case 'K':
		g.clearLine(csiArg(args, 0, 0))
	}
	g.clampCursor()
}

func parseCSIArgs(params string) []int {
	params = strings.TrimPrefix(params, "?")
	if params == "" {
		return nil
	}
	parts := strings.Split(params, ";")
	args := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			args = append(args, 0)
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			args = append(args, 0)
			continue
		}
		args = append(args, n)
	}
	return args
}

func csiArg(args []int, index int, def int) int {
	if index < 0 || index >= len(args) || args[index] == 0 {
		return def
	}
	return args[index]
}

func (g *frameGrid) clampCursor() {
	if g.x < 0 {
		g.x = 0
	}
	if g.x >= g.cols {
		g.x = g.cols - 1
	}
	if g.y < 0 {
		g.y = 0
	}
	if g.y >= g.rows {
		g.y = g.rows - 1
	}
}
