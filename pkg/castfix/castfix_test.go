package castfix

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFix_NoWrap(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "nowrap.cast")
	var output bytes.Buffer

	if err := Fix(strings.NewReader(input), &output, 20); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	// Each event has only 10 chars (half the width), no wrapping needed.
	events := decodeOutputEvents(t, output.String())
	for _, ev := range events {
		if strings.Contains(ev, "\x1b[") && strings.Contains(ev, "H") {
			t.Errorf("expected no CUP injected for no-wrap case, got event: %q", ev)
		}
	}
}

func TestFix_WrapASCII(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "wrap_ascii.cast")
	var output bytes.Buffer

	if err := Fix(strings.NewReader(input), &output, 20); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	// The 40-char string should wrap at col 20, injecting a CUP to row 2.
	events := decodeOutputEvents(t, output.String())
	found := false
	for _, ev := range events {
		if strings.Contains(ev, "\x1b[2;1H") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CUP \\x1b[2;1H in decoded output events")
	}
}

func TestFix_WrapEmoji(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "wrap_emoji.cast")
	var output bytes.Buffer

	if err := Fix(strings.NewReader(input), &output, 20); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	// 10 ASCII (10 cols) + 5 emoji (10 cols) = 20 cols exactly, then wrap.
	events := decodeOutputEvents(t, output.String())
	found := false
	for _, ev := range events {
		if strings.Contains(ev, "\x1b[2;1H") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CUP \\x1b[2;1H in decoded output events")
	}
}

func TestFix_Idempotent(t *testing.T) {
	t.Parallel()

	input := readTestdata(t, "wrap_emoji.cast")

	// First pass.
	var first bytes.Buffer
	if err := Fix(strings.NewReader(input), &first, 20); err != nil {
		t.Fatalf("Fix pass 1: %v", err)
	}

	// Second pass on already-fixed output.
	var second bytes.Buffer
	if err := Fix(strings.NewReader(first.String()), &second, 20); err != nil {
		t.Fatalf("Fix pass 2: %v", err)
	}

	if first.String() != second.String() {
		t.Errorf("Fix is not idempotent\npass1:\n%s\npass2:\n%s", first.String(), second.String())
	}
}

func TestFix_SGRPassthrough(t *testing.T) {
	t.Parallel()

	// SGR sequences should not count as columns.
	// 10 chars + SGR + 10 chars = exactly 20 cols, no wrap needed.
	cast := `{"version":2,"width":20,"height":5,"timestamp":1710000000}` + "\n" +
		`[0,"o","AAAAAAAAAA\u001b[31mBBBBBBBBBB"]` + "\n"

	var output bytes.Buffer
	if err := Fix(strings.NewReader(cast), &output, 20); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	// No wrapping should occur.
	events := decodeOutputEvents(t, output.String())
	for _, ev := range events {
		if strings.Contains(ev, "\x1b[2;1H") {
			t.Errorf("SGR should not cause wrap, got CUP in decoded event: %q", ev)
		}
	}
}

func TestFix_ExistingCUP(t *testing.T) {
	t.Parallel()

	// Input already has a CUP sequence moving cursor to row 3 col 1.
	cast := `{"version":2,"width":20,"height":5,"timestamp":1710000000}` + "\n" +
		`[0,"o","AAAAA\u001b[3;1HBBBBB"]` + "\n"

	var output bytes.Buffer
	if err := Fix(strings.NewReader(cast), &output, 20); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	// The existing CUP updates cursor state; no additional CUP should be injected.
	events := decodeOutputEvents(t, output.String())
	cupCount := 0
	for _, ev := range events {
		cupCount += strings.Count(ev, "\x1b[3;1H")
	}
	if cupCount != 1 {
		t.Errorf("expected exactly 1 CUP \\x1b[3;1H, got %d", cupCount)
	}
}

func TestFix_EmptyInput(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := Fix(strings.NewReader(""), &output, 80)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestFix_EmojiGridRow(t *testing.T) {
	t.Parallel()

	// Simulates issue #59: 16 emoji (32 cols) + 88 ASCII = 120 cols per row.
	// Two complete rows = 240 cols total, should get a CUP at row boundary.
	emoji16 := "🌀🌁🌂🌃🌄🌅🌆🌇🌈🌉🌊🌋🌌🌍🌎🌏"
	ascii88 := "0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567"
	row1 := emoji16 + ascii88
	row2 := emoji16 + ascii88

	// Build cast with two rows of content in one event.
	data := row1 + row2
	cast := `{"version":2,"width":120,"height":30,"timestamp":1710000000}` + "\n" +
		`[0,"o",` + mustMarshalString(data) + `]` + "\n"

	var output bytes.Buffer
	if err := Fix(strings.NewReader(cast), &output, 120); err != nil {
		t.Fatalf("Fix: %v", err)
	}

	events := decodeOutputEvents(t, output.String())
	found := false
	for _, ev := range events {
		if strings.Contains(ev, "\x1b[2;1H") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CUP \\x1b[2;1H for emoji grid row wrap")
	}
}

// decodeOutputEvents parses the cast output and returns decoded string data
// from all "o" events.
func decodeOutputEvents(t *testing.T, cast string) []string {
	t.Helper()
	var events []string
	lines := strings.Split(cast, "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		var ev [3]json.RawMessage
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		var data string
		if err := json.Unmarshal(ev[2], &data); err != nil {
			continue
		}
		events = append(events, data)
	}
	return events
}

func mustMarshalString(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return string(data)
}
