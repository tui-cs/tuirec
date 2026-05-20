package record

import (
	"bytes"
	"io"
	"sync"
)

// kittyInterceptor wraps a PTY reader, scanning output for Kitty keyboard
// protocol sequences. When the target app sends a query or enable request,
// the interceptor writes the appropriate response back to the PTY input.
type kittyInterceptor struct {
	reader    io.Reader
	writer    io.Writer // PTY input (write side)
	mu        sync.Mutex
	modeStack []int // stack of pushed keyboard modes
	buf       []byte
}

// newKittyInterceptor wraps reader and injects protocol responses via writer.
func newKittyInterceptor(reader io.Reader, writer io.Writer) *kittyInterceptor {
	return &kittyInterceptor{
		reader:    reader,
		writer:    writer,
		modeStack: []int{0},
	}
}

func (k *kittyInterceptor) Read(p []byte) (int, error) {
	n, err := k.reader.Read(p)
	if n > 0 {
		k.scan(p[:n])
	}

	return n, err
}

// scan looks for Kitty keyboard protocol sequences in output data.
// Sequences recognized:
//   - \x1b[?u         — query current mode → respond \x1b[?{mode}u
//   - \x1b[>Nu        — push/enable mode N
//   - \x1b[<u         — pop mode (revert to previous)
func (k *kittyInterceptor) scan(data []byte) {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Append to any leftover partial sequence from last read
	if len(k.buf) > 0 {
		data = append(k.buf, data...)
		k.buf = nil
	}

	for i := 0; i < len(data); i++ {
		if data[i] != '\x1b' {
			continue
		}

		remaining := data[i:]
		if len(remaining) < 3 {
			// Possible partial escape at end of buffer
			k.buf = append([]byte{}, remaining...)
			return
		}

		if remaining[1] != '[' {
			continue
		}

		// \x1b[?u — query mode
		if remaining[2] == '?' {
			if len(remaining) < 4 {
				k.buf = append([]byte{}, remaining...)
				return
			}
			if remaining[3] == 'u' {
				k.respondQuery()
				i += 3
				continue
			}
		}

		// \x1b[<u — pop mode
		if remaining[2] == '<' {
			if len(remaining) < 4 {
				k.buf = append([]byte{}, remaining...)
				return
			}
			if remaining[3] == 'u' {
				if len(k.modeStack) > 1 {
					k.modeStack = k.modeStack[:len(k.modeStack)-1]
				} else {
					k.modeStack = []int{0}
				}
				i += 3
				continue
			}
		}

		// \x1b[>Nu — push/enable mode N
		if remaining[2] == '>' {
			end := bytes.IndexByte(remaining[3:], 'u')
			if end == -1 {
				if len(remaining) < 8 {
					// Possibly incomplete
					k.buf = append([]byte{}, remaining...)
					return
				}
				continue
			}
			modeStr := remaining[3 : 3+end]
			mode := 0
			for _, b := range modeStr {
				if b < '0' || b > '9' {
					mode = -1
					break
				}
				mode = mode*10 + int(b-'0')
			}
			if mode >= 0 {
				k.modeStack = append(k.modeStack, mode)
			}
			i += 3 + end
			continue
		}
	}
}

// respondQuery writes the current mode back to the PTY as \x1b[?{mode}u.
func (k *kittyInterceptor) respondQuery() {
	resp := []byte{'\x1b', '[', '?'}
	mode := 0
	if len(k.modeStack) > 0 {
		mode = k.modeStack[len(k.modeStack)-1]
	}
	if mode == 0 {
		resp = append(resp, '0')
	} else {
		digits := []byte{}
		m := mode
		for m > 0 {
			digits = append([]byte{byte('0' + m%10)}, digits...)
			m /= 10
		}
		resp = append(resp, digits...)
	}
	resp = append(resp, 'u')

	// Best-effort write; don't block the reader goroutine on errors
	_, _ = k.writer.Write(resp)
}
