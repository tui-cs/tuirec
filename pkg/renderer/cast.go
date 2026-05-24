package renderer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// CastHeader is the asciinema v2 header (first line of a .cast file).
type CastHeader struct {
	Version int            `json:"version"`
	Width   int            `json:"width"`
	Height  int            `json:"height"`
	Title   string         `json:"title,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// CastEvent is a single output event: [time, type, data].
type CastEvent struct {
	Time float64
	Type string
	Data []byte
}

// ParseCast reads an asciinema v2 cast file, returning the header and events.
func ParseCast(r io.Reader) (CastHeader, []CastEvent, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		return CastHeader{}, nil, fmt.Errorf("empty cast file")
	}

	var hdr CastHeader
	if err := json.Unmarshal(scanner.Bytes(), &hdr); err != nil {
		return CastHeader{}, nil, fmt.Errorf("parse header: %w", err)
	}

	var events []CastEvent
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var raw []json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			return hdr, nil, fmt.Errorf("parse event: %w", err)
		}
		if len(raw) < 3 {
			continue
		}

		var t float64
		if err := json.Unmarshal(raw[0], &t); err != nil {
			return hdr, nil, fmt.Errorf("parse event time: %w", err)
		}

		var typ string
		if err := json.Unmarshal(raw[1], &typ); err != nil {
			return hdr, nil, fmt.Errorf("parse event type: %w", err)
		}

		var data string
		if err := json.Unmarshal(raw[2], &data); err != nil {
			return hdr, nil, fmt.Errorf("parse event data: %w", err)
		}

		events = append(events, CastEvent{
			Time: t,
			Type: typ,
			Data: []byte(data),
		})
	}

	return hdr, events, scanner.Err()
}
