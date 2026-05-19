//go:build integration

package gif

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderWithAgg(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("agg"); err != nil {
		t.Skip("agg not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	output := filepath.Join(t.TempDir(), "fixture.gif")
	cast := filepath.Join("testdata", "animated.cast")

	if err := Render(ctx, cast, output, Config{}); err != nil {
		t.Fatalf("Render: %v", err)
	}

	validation, err := Validate(output)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if validation.Frames < 2 {
		t.Fatalf("Frames = %d, want >= 2", validation.Frames)
	}
}
