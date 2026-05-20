package record

import (
	"bytes"
	"io"
	"testing"
)

func TestKittyInterceptorQueryResponse(t *testing.T) {
	t.Parallel()

	// Simulate target app sending \x1b[?u (query current keyboard mode)
	input := bytes.NewReader([]byte("hello\x1b[?uworld"))
	var response bytes.Buffer
	interceptor := newKittyInterceptor(input, &response)

	output := make([]byte, 256)
	var total int
	for {
		n, err := interceptor.Read(output[total:])
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	// All data passes through
	got := string(output[:total])
	if got != "hello\x1b[?uworld" {
		t.Errorf("passthrough = %q, want %q", got, "hello\x1b[?uworld")
	}

	// Response should be \x1b[?0u (mode 0 = legacy, no mode enabled yet)
	if response.String() != "\x1b[?0u" {
		t.Errorf("response = %q, want %q", response.String(), "\x1b[?0u")
	}
}

func TestKittyInterceptorEnableThenQuery(t *testing.T) {
	t.Parallel()

	// App enables mode 1, then queries
	input := bytes.NewReader([]byte("\x1b[>1u\x1b[?u"))
	var response bytes.Buffer
	interceptor := newKittyInterceptor(input, &response)

	buf := make([]byte, 256)
	for {
		_, err := interceptor.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	// After enable mode 1, query should return \x1b[?1u
	if response.String() != "\x1b[?1u" {
		t.Errorf("response = %q, want %q", response.String(), "\x1b[?1u")
	}
}

func TestKittyInterceptorPopMode(t *testing.T) {
	t.Parallel()

	// Enable mode 3, then pop, then query
	input := bytes.NewReader([]byte("\x1b[>3u\x1b[<u\x1b[?u"))
	var response bytes.Buffer
	interceptor := newKittyInterceptor(input, &response)

	buf := make([]byte, 256)
	for {
		_, err := interceptor.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	// After pop, mode reverts to 0
	if response.String() != "\x1b[?0u" {
		t.Errorf("response = %q, want %q", response.String(), "\x1b[?0u")
	}
}

func TestKittyInterceptorNoResponseWithoutQuery(t *testing.T) {
	t.Parallel()

	// Regular output with no Kitty sequences
	input := bytes.NewReader([]byte("just regular terminal output \x1b[31m red text"))
	var response bytes.Buffer
	interceptor := newKittyInterceptor(input, &response)

	buf := make([]byte, 256)
	for {
		_, err := interceptor.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read error: %v", err)
		}
	}

	if response.Len() != 0 {
		t.Errorf("unexpected response %q for non-kitty output", response.String())
	}
}
