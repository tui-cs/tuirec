package record

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractFrameText(t *testing.T) {
	t.Parallel()

	castPath := filepath.Join(t.TempDir(), "frame.cast")
	cast := strings.Join([]string{
		`{"version":2,"width":10,"height":3}`,
		`[0.1,"o","hello"]`,
		`[0.2,"o","\r\u001b[2Kbye"]`,
		`[0.3,"o","\nline2"]`,
	}, "\n") + "\n"
	if err := os.WriteFile(castPath, []byte(cast), 0o600); err != nil {
		t.Fatalf("write cast: %v", err)
	}

	got, err := ExtractFrameText(castPath)
	if err != nil {
		t.Fatalf("ExtractFrameText: %v", err)
	}

	want := "bye\nline2"
	if got != want {
		t.Fatalf("ExtractFrameText = %q, want %q", got, want)
	}
}

func TestExtractFrameTextDefaultsSizeWhenHeaderMissing(t *testing.T) {
	t.Parallel()

	castPath := filepath.Join(t.TempDir(), "frame.cast")
	cast := strings.Join([]string{
		`{"version":2}`,
		`[0.1,"o","x"]`,
	}, "\n") + "\n"
	if err := os.WriteFile(castPath, []byte(cast), 0o600); err != nil {
		t.Fatalf("write cast: %v", err)
	}

	got, err := ExtractFrameText(castPath)
	if err != nil {
		t.Fatalf("ExtractFrameText: %v", err)
	}
	if got != "x" {
		t.Fatalf("ExtractFrameText = %q, want %q", got, "x")
	}
}
