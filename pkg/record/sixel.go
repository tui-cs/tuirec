package record

import (
	"io"
	"sync"
)

// sixelInterceptor wraps a PTY reader, scanning output for DA1 (Primary Device
// Attributes) queries. When the target app sends \x1b[c or \x1b[0c, the
// interceptor writes a DA1 response that advertises sixel graphics support
// (attribute 4) back to the PTY input.
type sixelInterceptor struct {
	reader io.Reader
	writer io.Writer // PTY input (write side)
	mu     sync.Mutex
	buf    []byte
}

// newSixelInterceptor wraps reader and injects DA1 responses via writer.
func newSixelInterceptor(reader io.Reader, writer io.Writer) *sixelInterceptor {
	return &sixelInterceptor{
		reader: reader,
		writer: writer,
	}
}

func (s *sixelInterceptor) Read(p []byte) (int, error) {
	n, err := s.reader.Read(p)
	if n > 0 {
		s.scan(p[:n])
	}

	return n, err
}

// da1Response is a VT220 level-2 DA1 response advertising sixel support.
// Format: CSI ? 62 ; 4 c
//   - 62 = VT220 conformance level
//   - 4 = sixel graphics
var da1Response = []byte("\x1b[?62;4c")

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
