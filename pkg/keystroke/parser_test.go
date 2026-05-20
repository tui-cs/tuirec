package keystroke

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseScript(t *testing.T) {
	t.Parallel()

	actions, err := Parse("wait:300,click:39:3,Enter,`hello world`,`slash\\done`")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{
		{Kind: Wait, Label: "wait:300", Delay: 300 * time.Millisecond},
		{Kind: Write, Sequence: "\x1b[<0;39;3M\x1b[<0;39;3m", Label: "click:39:3"},
		{Kind: Write, Sequence: "\r", Label: "Enter"},
		{Kind: Literal, Sequence: "hello world", Label: "`hello world`"},
		{Kind: Literal, Sequence: `slash\done`, Label: "`slash\\done`"},
	}

	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}

func TestParseClickRejectsZeroCoordinates(t *testing.T) {
	t.Parallel()

	_, err := Parse("click:0:1")
	if err == nil {
		t.Fatal("Parse(click:0:1) err = nil, want error")
	}
}

func TestParseDanglingEscape(t *testing.T) {
	t.Parallel()

	_, err := Parse(`literal\`)
	if err == nil {
		t.Fatal("Parse(dangling escape) err = nil, want error")
	}
}

func TestParseInvalidWait(t *testing.T) {
	t.Parallel()

	if _, err := Parse("wait:abc"); err == nil {
		t.Fatal("Parse(wait:abc) err = nil, want error")
	}
}

func TestParseInvalidClick(t *testing.T) {
	t.Parallel()

	if _, err := Parse("click:left:top"); err == nil {
		t.Fatal("Parse(click:left:top) err = nil, want error")
	}
}

func TestParseUnknownKey(t *testing.T) {
	t.Parallel()

	tests := []string{
		"Ctrl+Foo",
		"Ctrl-Foo",
		"Shift+Foo",
		"Alt+Foo",
		"Alt-Foo",
		"ArrowDiagonal",
		"F21",
		"PrintScreen",
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse(tt); err == nil {
				t.Fatalf("Parse(%q) err = nil, want error", tt)
			}
		})
	}
}

func TestParseTerminalGUIKeyStrings(t *testing.T) {
	t.Parallel()

	actions, err := Parse("ctrl+alt+shift+cursorup,Ctrl-c,A-Ctrl,Shift+Tab,D4")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{
		{Kind: Write, Sequence: "\x1b[1;8A", Label: "ctrl+alt+shift+cursorup"},
		{Kind: Write, Sequence: "\x03", Label: "Ctrl-c"},
		{Kind: Write, Sequence: "\x01", Label: "A-Ctrl"},
		{Kind: Write, Sequence: "\x1b[Z", Label: "Shift+Tab"},
		{Kind: Write, Sequence: "4", Label: "D4"},
	}

	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}

func TestParseLiteralWithBackticks(t *testing.T) {
	t.Parallel()

	actions, err := Parse("`Page title`,`Arrow key`,`Ctrl-C to stop`,`Alt-text`")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{
		{Kind: Literal, Sequence: "Page title", Label: "`Page title`"},
		{Kind: Literal, Sequence: "Arrow key", Label: "`Arrow key`"},
		{Kind: Literal, Sequence: "Ctrl-C to stop", Label: "`Ctrl-C to stop`"},
		{Kind: Literal, Sequence: "Alt-text", Label: "`Alt-text`"},
	}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}

func TestParseBareWordsRequireBackticks(t *testing.T) {
	t.Parallel()

	// Bare multi-char tokens that aren't keys must be backtick-quoted.
	bareWords := []string{"cursor", "page", "arrow", "hello"}
	for _, word := range bareWords {
		if _, err := Parse(word); err == nil {
			t.Errorf("Parse(%q) err = nil, want error requiring backticks", word)
		}
	}

	// Backtick-quoted versions work fine.
	actions, err := Parse("`cursor`,`page`,`arrow`,`hello`")
	if err != nil {
		t.Fatalf("Parse backtick-quoted: %v", err)
	}

	want := []Action{
		{Kind: Literal, Sequence: "cursor", Label: "`cursor`"},
		{Kind: Literal, Sequence: "page", Label: "`page`"},
		{Kind: Literal, Sequence: "arrow", Label: "`arrow`"},
		{Kind: Literal, Sequence: "hello", Label: "`hello`"},
	}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}

func TestParseLiteralWithCommasInBackticks(t *testing.T) {
	t.Parallel()

	actions, err := Parse("`hello,world`,Enter,`a,b,c`")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{
		{Kind: Literal, Sequence: "hello,world", Label: "`hello,world`"},
		{Kind: Write, Sequence: "\r", Label: "Enter"},
		{Kind: Literal, Sequence: "a,b,c", Label: "`a,b,c`"},
	}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}

func TestParseUnclosedBacktickError(t *testing.T) {
	t.Parallel()

	_, err := Parse("`hello")
	if err == nil {
		t.Fatal("Parse(`hello) err = nil, want unclosed backtick error")
	}
	if !strings.Contains(err.Error(), "unclosed backtick") {
		t.Fatalf("error = %q, want it to mention unclosed backtick", err.Error())
	}
}

func TestParseCapitalizedKeysStillWork(t *testing.T) {
	t.Parallel()

	actions, err := Parse("CursorUp,PageDown,Delete,Enter,Esc")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{
		{Kind: Write, Sequence: "\x1b[A", Label: "CursorUp"},
		{Kind: Write, Sequence: "\x1b[6~", Label: "PageDown"},
		{Kind: Write, Sequence: "\x1b[3~", Label: "Delete"},
		{Kind: Write, Sequence: "\r", Label: "Enter"},
		{Kind: Write, Sequence: "\x1b", Label: "Esc"},
	}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}