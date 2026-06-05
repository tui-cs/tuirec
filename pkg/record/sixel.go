package record

import (
	"fmt"
	"io"
	"sync"
)

// sixelInterceptor wraps a PTY reader, scanning output for the terminal queries
// a sixel-capable terminal must answer for an app to emit sixel: DA1 (sixel
// capability) and the window/cell geometry reports (so the app can size the
// raster and lay out its UI). Responses are written back to the PTY input.
//
// The reported cell size must match the cell size agg renders at, or the app
// scales its sixel raster to the wrong number of pixels and the image overflows
// (or underfills) its on-screen cells. agg's row height is fontSize*lineHeight
// and its column width is ~0.6*fontSize for a monospace font, so the geometry
// is derived from the GIF font settings rather than guessed.
type sixelInterceptor struct {
	reader       io.Reader
	writer       io.Writer // PTY input (write side)
	cols         int
	rows         int
	cellWidthPx  int
	cellHeightPx int
	mu           sync.Mutex
	buf          []byte
}

// newSixelInterceptor wraps reader and injects query responses via writer. cols
// and rows are the recording's terminal size; cellWidthPx and cellHeightPx are
// the pixel cell size agg will render at, used for the geometry reports.
func newSixelInterceptor(reader io.Reader, writer io.Writer, cols, rows, cellWidthPx, cellHeightPx int) *sixelInterceptor {
	return &sixelInterceptor{
		reader:       reader,
		writer:       writer,
		cols:         cols,
		rows:         rows,
		cellWidthPx:  cellWidthPx,
		cellHeightPx: cellHeightPx,
	}
}

func (s *sixelInterceptor) Read(p []byte) (int, error) {
	n, err := s.reader.Read(p)
	if n > 0 {
		s.scan(p[:n])
	}

	return n, err
}

// da1Response is a VT220-level DA1 response advertising sixel support, modeled
// on a real xterm reply. Format: CSI ? 62 ; 4 ; 6 ; 22 c
//   - 62 = VT220 conformance level
//   - 4  = sixel graphics
//   - 6  = selective erase
//   - 22 = ANSI color
//
// The sixel attribute (4) is deliberately not the last parameter: apps commonly
// detect support with response.Split(";").Contains("4") (e.g. Terminal.Gui's
// SixelSupportDetector), and a trailing "4c" token defeats that check. Keeping
// 4 mid-list, as real terminals do, makes detection robust.
var da1Response = []byte("\x1b[?62;4;6;22c")

// scan looks for DA1 query sequences in output data.
// Sequences recognized:
//   - \x1b[c     — DA1 request (no parameter)
//   - \x1b[0c    — DA1 request (explicit parameter 0)
func (s *sixelInterceptor) scan(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Append any leftover partial sequence from last read.
	if len(s.buf) > 0 {
		data = append(s.buf, data...)
		s.buf = nil
	}

	for i := 0; i < len(data); i++ {
		if data[i] != '\x1b' {
			continue
		}

		remaining := data[i:]
		if len(remaining) < 3 {
			// Possible partial escape at end of buffer.
			s.buf = append([]byte{}, remaining...)
			return
		}

		if remaining[1] != '[' {
			continue
		}

		// \x1b[c — DA1 with no parameter
		if remaining[2] == 'c' {
			s.respondDA1()
			i += 2
			continue
		}

		// \x1b[0c — DA1 with explicit 0 parameter
		if remaining[2] == '0' {
			if len(remaining) < 4 {
				s.buf = append([]byte{}, remaining...)
				return
			}
			if remaining[3] == 'c' {
				s.respondDA1()
				i += 3
				continue
			}
		}

		// \x1b[>c or \x1b[>0c — DA2 (Secondary Device Attributes)
		if remaining[2] == '>' {
			if len(remaining) < 4 {
				s.buf = append([]byte{}, remaining...)
				return
			}
			if remaining[3] == 'c' {
				// DA2 with no parameter — respond with DA2
				s.respondDA2()
				i += 3
				continue
			}
			if remaining[3] == '0' {
				if len(remaining) < 5 {
					s.buf = append([]byte{}, remaining...)
					return
				}
				if remaining[4] == 'c' {
					s.respondDA2()
					i += 4
					continue
				}
			}
		}

		// Window/cell geometry reports (xterm CSI 1{4,6,8} t). A real terminal
		// answers these; an app that gets no reply cannot determine the screen
		// size (so it never lays out its UI) or the cell size (so sixel falls
		// back to 1 pixel per cell). Terminal.Gui's ANSI driver relies on them.
		// ESC [ 1 X t is 5 bytes.
		if remaining[2] == '1' {
			if len(remaining) < 5 {
				s.buf = append([]byte{}, remaining...)
				return
			}
			if remaining[4] == 't' {
				switch remaining[3] {
				case '4': // report window size in pixels
					s.respond(s.windowPixelsReport())
					i += 4
					continue
				case '6': // report cell size in pixels
					s.respond(s.cellSizeReport())
					i += 4
					continue
				case '8': // report text area size in characters
					s.respond(s.textAreaReport())
					i += 4
					continue
				}
			}
		}
	}
}

// respondDA1 writes the DA1 response to the PTY input.
func (s *sixelInterceptor) respondDA1() {
	// Best-effort write; don't block the reader goroutine on errors.
	_, _ = s.writer.Write(da1Response)
}

// da2Response is a DA2 (Secondary Device Attributes) response.
// Format: CSI > 1 ; 0 ; 0 c (VT220, firmware version 0, ROM 0)
var da2Response = []byte("\x1b[>1;0;0c")

// respondDA2 writes the DA2 response to the PTY input.
func (s *sixelInterceptor) respondDA2() {
	_, _ = s.writer.Write(da2Response)
}

// respond writes a query reply to the PTY input (best-effort).
func (s *sixelInterceptor) respond(reply []byte) {
	_, _ = s.writer.Write(reply)
}

// cellSizeReport answers CSI 16 t: CSI 6 ; heightPx ; widthPx t.
func (s *sixelInterceptor) cellSizeReport() []byte {
	return []byte(fmt.Sprintf("\x1b[6;%d;%dt", s.cellHeightPx, s.cellWidthPx))
}

// textAreaReport answers CSI 18 t: CSI 8 ; rows ; cols t.
func (s *sixelInterceptor) textAreaReport() []byte {
	return []byte(fmt.Sprintf("\x1b[8;%d;%dt", s.rows, s.cols))
}

// windowPixelsReport answers CSI 14 t: CSI 4 ; heightPx ; widthPx t.
func (s *sixelInterceptor) windowPixelsReport() []byte {
	return []byte(fmt.Sprintf("\x1b[4;%d;%dt", s.rows*s.cellHeightPx, s.cols*s.cellWidthPx))
}
