package keystroke

import (
	"reflect"
	"testing"
	"time"
)

func TestParseScript(t *testing.T) {
	t.Parallel()

	actions, err := Parse(`wait:300,click:39:3,Enter,hello\,world,slash\\done`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{
		{Kind: Wait, Delay: 300 * time.Millisecond},
		{Kind: Write, Sequence: "\x1b[<0;39;3M\x1b[<0;39;3m"},
		{Kind: Write, Sequence: "\r"},
		{Kind: Literal, Sequence: "hello,world"},
		{Kind: Literal, Sequence: `slash\done`},
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

func TestParseWaitLikeLiteral(t *testing.T) {
	t.Parallel()

	actions, err := Parse("wait:abc")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{{Kind: Literal, Sequence: "wait:abc"}}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}

func TestParseClickLikeLiteral(t *testing.T) {
	t.Parallel()

	actions, err := Parse("click:left:top")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []Action{{Kind: Literal, Sequence: "click:left:top"}}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("Parse() = %#v, want %#v", actions, want)
	}
}
