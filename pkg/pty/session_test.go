package pty

import "testing"

func TestNormalizeSizeUsesDefaults(t *testing.T) {
	t.Parallel()

	got := normalizeSize(Size{})
	if got.Cols != defaultCols {
		t.Fatalf("Cols = %d, want %d", got.Cols, defaultCols)
	}

	if got.Rows != defaultRows {
		t.Fatalf("Rows = %d, want %d", got.Rows, defaultRows)
	}
}

func TestNormalizeSizePreservesPositiveValues(t *testing.T) {
	t.Parallel()

	got := normalizeSize(Size{Cols: 80, Rows: 24})
	if got.Cols != 80 {
		t.Fatalf("Cols = %d, want 80", got.Cols)
	}

	if got.Rows != 24 {
		t.Fatalf("Rows = %d, want 24", got.Rows)
	}
}

func TestNormalizeEnvAddsTerminalDefaults(t *testing.T) {
	t.Parallel()

	env := normalizeEnv([]string{"PATH=/bin"})
	assertEnvContains(t, env, "TERM=xterm-256color")
	assertEnvContains(t, env, "COLORTERM=truecolor")
}

func TestNormalizeEnvPreservesExistingTerminalValues(t *testing.T) {
	t.Parallel()

	env := normalizeEnv([]string{"TERM=vt100", "COLORTERM=false"})
	assertEnvContains(t, env, "TERM=vt100")
	assertEnvContains(t, env, "COLORTERM=false")
}

func assertEnvContains(t *testing.T, env []string, want string) {
	t.Helper()

	for _, entry := range env {
		if entry == want {
			return
		}
	}

	t.Fatalf("env %v does not contain %q", env, want)
}
