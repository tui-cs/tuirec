package pty

import (
	"strings"
	"testing"
)

func TestNormalizeSizeUsesDefaults(t *testing.T) {
	t.Parallel()

	got := NormalizeSize(Size{})
	if got.Cols != defaultCols {
		t.Fatalf("Cols = %d, want %d", got.Cols, defaultCols)
	}

	if got.Rows != defaultRows {
		t.Fatalf("Rows = %d, want %d", got.Rows, defaultRows)
	}
}

func TestNormalizeSizePreservesPositiveValues(t *testing.T) {
	t.Parallel()

	got := NormalizeSize(Size{Cols: 80, Rows: 24})
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
	assertEnvKeyPresent(t, env, "TERM")
	assertEnvKeyPresent(t, env, "COLORTERM")
}

func TestNormalizeEnvPreservesExistingTerminalValues(t *testing.T) {
	t.Parallel()

	env := normalizeEnv([]string{"TERM=vt100", "COLORTERM=false"})
	assertEnvContains(t, env, "TERM=vt100")
	assertEnvContains(t, env, "COLORTERM=false")
}

func TestNormalizeEnvMergesParentEnvironment(t *testing.T) {
	t.Setenv("TUIREC_TEST_PARENT", "present")
	env := normalizeEnv([]string{"DOTNET_ROOT=/dotnet"})

	assertEnvContains(t, env, "TUIREC_TEST_PARENT=present")
	assertEnvContains(t, env, "DOTNET_ROOT=/dotnet")
}

func TestNormalizeEnvOverridesParentValues(t *testing.T) {
	t.Setenv("TUIREC_TEST_OVERRIDE", "parent")
	env := normalizeEnv([]string{"TUIREC_TEST_OVERRIDE=child"})

	var matches int
	for _, entry := range env {
		if entry == "TUIREC_TEST_OVERRIDE=child" {
			matches++
		}
		if entry == "TUIREC_TEST_OVERRIDE=parent" {
			t.Fatalf("env unexpectedly contains parent value: %v", env)
		}
	}
	if matches != 1 {
		t.Fatalf("env expected one overridden entry, got %d in %v", matches, env)
	}
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

func assertEnvKeyPresent(t *testing.T, env []string, key string) {
	t.Helper()

	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return
		}
	}

	t.Fatalf("env %v does not contain key %q", env, key)
}
