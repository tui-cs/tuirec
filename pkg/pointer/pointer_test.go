package pointer

import (
	"testing"
)

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		want  Mode
		err   bool
	}{
		{"none", None, false},
		{"clicks", Clicks, false},
		{"all", All, false},
		{"None", None, false},
		{"CLICKS", Clicks, false},
		{"ALL", All, false},
		{"invalid", None, true},
		{"", None, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseMode(tt.input)
			if (err != nil) != tt.err {
				t.Fatalf("ParseMode(%q) err = %v, want err = %v", tt.input, err, tt.err)
			}
			if got != tt.want {
				t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIndicatorShow(t *testing.T) {
	ind := NewIndicator("●")
	seq := ind.Show(10, 5)
	// Should contain save cursor, move, style, char, reset, restore cursor.
	want := "\x1b[s\x1b[5;10H\x1b[7;1;93m●\x1b[0m\x1b[u"
	if seq != want {
		t.Errorf("Show(10,5) = %q, want %q", seq, want)
	}
}

func TestIndicatorShowMovesPointer(t *testing.T) {
	ind := NewIndicator("●")
	ind.Show(10, 5)
	seq := ind.Show(20, 8)
	// hide is now a no-op (app redraws the old cell), so only show new position.
	want := "\x1b[s\x1b[8;20H\x1b[7;1;93m●\x1b[0m\x1b[u"
	if seq != want {
		t.Errorf("Show(20,8) after Show(10,5) = %q, want %q", seq, want)
	}
}

func TestIndicatorShowSamePosition(t *testing.T) {
	ind := NewIndicator("●")
	ind.Show(10, 5)
	seq := ind.Show(10, 5)
	// Same position: no hide, just show again.
	want := "\x1b[s\x1b[5;10H\x1b[7;1;93m●\x1b[0m\x1b[u"
	if seq != want {
		t.Errorf("Show(10,5) again = %q, want %q", seq, want)
	}
}

func TestIndicatorHide(t *testing.T) {
	ind := NewIndicator("●")
	ind.Show(10, 5)
	seq := ind.Hide()
	// hide is now a no-op; app redraws handle clearing the pointer cell.
	if seq != "" {
		t.Errorf("Hide() = %q, want empty", seq)
	}
}

func TestIndicatorHideNotVisible(t *testing.T) {
	ind := NewIndicator("●")
	seq := ind.Hide()
	if seq != "" {
		t.Errorf("Hide() when not visible = %q, want empty", seq)
	}
}

func TestIndicatorCustomStyle(t *testing.T) {
	ind := NewIndicator("►")
	seq := ind.Show(3, 7)
	want := "\x1b[s\x1b[7;3H\x1b[7;1;93m►\x1b[0m\x1b[u"
	if seq != want {
		t.Errorf("Show with custom style = %q, want %q", seq, want)
	}
}

func TestShouldShow(t *testing.T) {
	tests := []struct {
		mode  Mode
		label string
		want  bool
	}{
		{None, "click:10:5", false},
		{None, "move:3:7", false},
		{Clicks, "click:10:5", true},
		{Clicks, "rightclick:20:8", true},
		{Clicks, "middleclick:3:12", true},
		{Clicks, "doubleclick:15:7", true},
		{Clicks, "scroll:up:10:5", true},
		{Clicks, "drag:1:1:40:20", true},
		{Clicks, "move:3:7", false},
		{Clicks, "hover:10:5", false},
		{All, "click:10:5", true},
		{All, "move:3:7", true},
		{All, "hover:10:5", true},
		{All, "drag:1:1:40:20", true},
		{Clicks, "Ctrl+click:10:5", true},
		{Clicks, "Alt+Shift+rightclick:20:8", true},
		{All, "Ctrl+move:3:7", true},
		{Clicks, "Enter", false},
		{All, "Enter", false},
		{All, "wait:500", false},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String()+"/"+tt.label, func(t *testing.T) {
			got := ShouldShow(tt.mode, tt.label)
			if got != tt.want {
				t.Errorf("ShouldShow(%v, %q) = %v, want %v", tt.mode, tt.label, got, tt.want)
			}
		})
	}
}

func TestPosition(t *testing.T) {
	tests := []struct {
		label   string
		wantCol int
		wantRow int
		wantOK  bool
	}{
		{"click:10:5", 10, 5, true},
		{"rightclick:20:8", 20, 8, true},
		{"middleclick:3:12", 3, 12, true},
		{"doubleclick:15:7", 15, 7, true},
		{"scroll:up:10:5", 10, 5, true},
		{"scroll:down:3:7", 3, 7, true},
		{"drag:1:1:40:20", 1, 1, true},
		{"move:85:1", 85, 1, true},
		{"hover:70:2", 70, 2, true},
		{"Ctrl+click:10:5", 10, 5, true},
		{"Alt+Shift+rightclick:20:8", 20, 8, true},
		{"Enter", 0, 0, false},
		{"wait:500", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			col, row, ok := Position(tt.label)
			if ok != tt.wantOK {
				t.Fatalf("Position(%q) ok = %v, want %v", tt.label, ok, tt.wantOK)
			}
			if col != tt.wantCol || row != tt.wantRow {
				t.Errorf("Position(%q) = (%d, %d), want (%d, %d)", tt.label, col, row, tt.wantCol, tt.wantRow)
			}
		})
	}
}
