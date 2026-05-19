// Package recorder writes asciinema v2 cast files.
package recorder

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
	"unicode/utf8"
)

const (
	defaultWidth  = 120
	defaultHeight = 30
)

// Config describes an asciinema recording.
type Config struct {
	Width     int
	Height    int
	Timestamp time.Time
	Title     string
	Env       map[string]string
	Clock     Clock
}

// Recorder writes asciinema v2 output events as they arrive.
type Recorder struct {
	writer  io.Writer
	clock   Clock
	pending []byte
	closed  bool
}

type header struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Title     string            `json:"title,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

// New creates a recorder and writes the asciinema v2 header immediately.
func New(writer io.Writer, config Config) (*Recorder, error) {
	config = normalizeConfig(config)
	line, err := json.Marshal(header{
		Version:   2,
		Width:     config.Width,
		Height:    config.Height,
		Timestamp: config.Timestamp.Unix(),
		Title:     config.Title,
		Env:       config.Env,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal cast header: %w", err)
	}

	if _, err := writer.Write(append(line, '\n')); err != nil {
		return nil, fmt.Errorf("write cast header: %w", err)
	}

	return &Recorder{
		writer: writer,
		clock:  config.Clock,
	}, nil
}

// Write records PTY output bytes, buffering incomplete UTF-8 suffixes.
func (r *Recorder) Write(p []byte) (int, error) {
	if r.closed {
		return 0, fmt.Errorf("recorder is closed")
	}

	data := append(r.pending, p...)
	complete, pending := splitCompleteUTF8(data)
	r.pending = append(r.pending[:0], pending...)

	if len(complete) == 0 {
		return len(p), nil
	}

	if err := r.writeOutput(string(complete)); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Close flushes any pending bytes and prevents further writes.
func (r *Recorder) Close() error {
	if r.closed {
		return nil
	}

	r.closed = true
	if len(r.pending) == 0 {
		return nil
	}

	pending := string(r.pending)
	r.pending = nil

	return r.writeOutput(pending)
}

func normalizeConfig(config Config) Config {
	if config.Width <= 0 {
		config.Width = defaultWidth
	}

	if config.Height <= 0 {
		config.Height = defaultHeight
	}

	if config.Timestamp.IsZero() {
		config.Timestamp = time.Now()
	}

	if config.Clock == nil {
		clock := NewWallClock()
		config.Clock = clock
	}

	return config
}

func (r *Recorder) writeOutput(output string) error {
	event := []any{r.clock.Elapsed().Seconds(), "o", output}
	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal output event: %w", err)
	}

	if _, err := r.writer.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write output event: %w", err)
	}

	return nil
}

func splitCompleteUTF8(data []byte) ([]byte, []byte) {
	if len(data) == 0 {
		return nil, nil
	}

	start := len(data) - utf8.UTFMax
	if start < 0 {
		start = 0
	}

	for i := len(data) - 1; i >= start; i-- {
		if data[i] < utf8.RuneSelf {
			return data, nil
		}

		if !utf8.RuneStart(data[i]) {
			continue
		}

		if utf8.FullRune(data[i:]) {
			return data, nil
		}

		return data[:i], data[i:]
	}

	return data, nil
}
