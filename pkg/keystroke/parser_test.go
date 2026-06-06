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

func TestParseMouseEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  Action
	}{
		{
			name:  "left click",
			input: "click:10:5",
			want:  Action{Kind: Write, Sequence: "\x1b[<0;10;5M\x1b[<0;10;5m", Label: "click:10:5"},
		},
		{
			name:  "right click",
			input: "rightclick:20:8",
			want:  Action{Kind: Write, Sequence: "\x1b[<2;20;8M\x1b[<2;20;8m", Label: "rightclick:20:8"},
		},
		{
			name:  "middle click",
			input: "middleclick:3:12",
			want:  Action{Kind: Write, Sequence: "\x1b[<1;3;12M\x1b[<1;3;12m", Label: "middleclick:3:12"},
		},
		{
			name:  "double click",
			input: "doubleclick:15:7",
			want:  Action{Kind: Write, Sequence: "\x1b[<0;15;7M\x1b[<0;15;7m\x1b[<0;15;7M\x1b[<0;15;7m", Label: "doubleclick:15:7"},
		},
		{
			name:  "scroll up",
			input: "scroll:up:5:10",
			want:  Action{Kind: Write, Sequence: "\x1b[<64;5;10M", Label: "scroll:up:5:10"},
		},
		{
			name:  "scroll down",
			input: "scroll:down:5:10",
			want:  Action{Kind: Write, Sequence: "\x1b[<65;5;10M", Label: "scroll:down:5:10"},
		},
		{
			name:  "drag",
			input: "drag:5:5:8:5",
			want:  Action{Kind: Write, Sequence: "\x1b[<0;5;5M\x1b[<32;6;5M\x1b[<32;7;5M\x1b[<32;8;5M\x1b[<0;8;5m", Label: "drag:5:5:8:5"},
		},
		{
			name:  "mouse move",
			input: "move:42:3",
			want:  Action{Kind: Write, Sequence: "\x1b[<32;42;3M", Label: "move:42:3"},
		},
		{
			name:  "hover alias for move",
			input: "hover:80:1",
			want:  Action{Kind: Write, Sequence: "\x1b[<32;80;1M", Label: "hover:80:1"},
		},
		{
			name:  "ctrl click",
			input: "Ctrl+click:10:5",
			want:  Action{Kind: Write, Sequence: "\x1b[<16;10;5M\x1b[<16;10;5m", Label: "Ctrl+click:10:5"},
		},
		{
			name:  "alt shift right click",
			input: "Alt+Shift+rightclick:20:8",
			want:  Action{Kind: Write, Sequence: "\x1b[<14;20;8M\x1b[<14;20;8m", Label: "Alt+Shift+rightclick:20:8"},
		},
		{
			name:  "ctrl alt double click",
			input: "Ctrl+Alt+doubleclick:15:7",
			want:  Action{Kind: Write, Sequence: "\x1b[<24;15;7M\x1b[<24;15;7m\x1b[<24;15;7M\x1b[<24;15;7m", Label: "Ctrl+Alt+doubleclick:15:7"},
		},
		{
			name:  "shift scroll down",
			input: "Shift+scroll:down:5:10",
			want:  Action{Kind: Write, Sequence: "\x1b[<69;5;10M", Label: "Shift+scroll:down:5:10"},
		},
		{
			name:  "alt drag",
			input: "Alt+drag:5:5:8:5",
			want:  Action{Kind: Write, Sequence: "\x1b[<8;5;5M\x1b[<40;6;5M\x1b[<40;7;5M\x1b[<40;8;5M\x1b[<8;8;5m", Label: "Alt+drag:5:5:8:5"},
		},
		{
			name:  "ctrl hover",
			input: "Ctrl+hover:80:1",
			want:  Action{Kind: Write, Sequence: "\x1b[<48;80;1M", Label: "Ctrl+hover:80:1"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actions, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.input, err)
			}
			if len(actions) != 1 {
				t.Fatalf("Parse(%q) returned %d actions, want 1", tt.input, len(actions))
			}
			if !reflect.DeepEqual(actions[0], tt.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tt.input, actions[0], tt.want)
			}
		})
	}
}

func TestParseMouseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"click zero col", "click:0:1"},
		{"click zero row", "click:1:0"},
		{"rightclick zero", "rightclick:0:5"},
		{"middleclick zero", "middleclick:5:0"},
		{"doubleclick zero", "doubleclick:0:0"},
		{"scroll invalid direction", "scroll:left:5:5"},
		{"scroll missing coords", "scroll:up:5"},
		{"scroll non-numeric", "scroll:up:a:b"},
		{"drag missing coords", "drag:1:2:3"},
		{"drag zero coord", "drag:0:1:5:5"},
		{"drag non-numeric", "drag:a:1:5:5"},
		{"click non-numeric", "click:abc:def"},
		{"modified click zero col", "Ctrl+click:0:1"},
		{"modified scroll invalid direction", "Alt+scroll:left:5:5"},
		{"modified drag missing coords", "Shift+drag:1:2:3"},
		{"modified unknown mouse action", "Ctrl+tap:1:1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(tt.input)
			if err == nil {
				t.Fatalf("Parse(%q) err = nil, want error", tt.input)
			}
		})
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

func TestParseSmoothDragExpandsToTimedSteps(t *testing.T) {
	t.Parallel()

	actions, err := Parse("smoothdrag:5:5:8:5:50")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	want := []string{
		"\x1b[<0;5;5M",  // press
		"\x1b[<32;6;5M", // motion
		"\x1b[<32;7;5M",
		"\x1b[<32;8;5M",
		"\x1b[<0;8;5m", // release
	}
	if len(actions) != len(want) {
		t.Fatalf("got %d actions, want %d: %#v", len(actions), len(want), actions)
	}
	for i, a := range actions {
		if a.Kind != Write {
			t.Fatalf("action %d kind = %v, want Write", i, a.Kind)
		}
		if a.Sequence != want[i] {
			t.Fatalf("action %d sequence = %q, want %q", i, a.Sequence, want[i])
		}
		if a.Delay != 50*time.Millisecond {
			t.Fatalf("action %d delay = %s, want 50ms", i, a.Delay)
		}
	}
}

func TestParseSmoothDragDefaultsStepDelay(t *testing.T) {
	t.Parallel()

	actions, err := Parse("smoothdrag:1:1:1:4")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	// press + 3 vertical steps + release.
	if len(actions) != 5 {
		t.Fatalf("got %d actions, want 5", len(actions))
	}
	if actions[1].Delay != defaultSmoothDragStepMs*time.Millisecond {
		t.Fatalf("default step delay = %s, want %dms", actions[1].Delay, defaultSmoothDragStepMs)
	}
}
