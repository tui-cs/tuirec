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

func TestNormalizeEnvAdvertisesDeterministicKittyIdentity(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "host-99")
	t.Setenv("KITTY_PID", "4242")
	t.Setenv("GHOSTTY_RESOURCES_DIR", "/usr/share/ghostty")

	env := normalizeEnv(nil)
	// Host-specific operational vars are stripped...
	assertEnvKeyAbsent(t, env, "KITTY_PID")
	assertEnvKeyAbsent(t, env, "GHOSTTY_RESOURCES_DIR")
	// ...and a single deterministic Kitty identity is advertised so the recorded
	// app detects Kitty graphics support regardless of the host terminal.
	assertEnvContains(t, env, "KITTY_WINDOW_ID="+kittyWindowID)
	assertEnvKeyCount(t, env, "KITTY_WINDOW_ID", 1)
}

func TestNormalizeEnvPreservesKittyClassTermProgram(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "ghostty")
	t.Setenv("TERM_PROGRAM_VERSION", "1.0.0")

	env := normalizeEnv(nil)
	// TERM_PROGRAM is no longer scrubbed; Kitty detection is driven by the
	// advertised KITTY_WINDOW_ID instead.
	assertEnvContains(t, env, "TERM_PROGRAM=ghostty")
	assertEnvContains(t, env, "TERM_PROGRAM_VERSION=1.0.0")
	assertEnvContains(t, env, "KITTY_WINDOW_ID="+kittyWindowID)
}

func TestNormalizeEnvPreservesNonKittyTermProgram(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "vscode")
	t.Setenv("TERM_PROGRAM_VERSION", "1.90.0")

	env := normalizeEnv(nil)
	assertEnvContains(t, env, "TERM_PROGRAM=vscode")
	assertEnvContains(t, env, "TERM_PROGRAM_VERSION=1.90.0")
}

func TestNormalizeEnvNormalizesKittyIdentityFromOverrides(t *testing.T) {
	t.Parallel()

	env := normalizeEnv([]string{"KITTY_WINDOW_ID=9", "PATH=/bin"})
	// A caller-supplied host identity is replaced by the deterministic one.
	assertEnvContains(t, env, "KITTY_WINDOW_ID="+kittyWindowID)
	assertEnvKeyCount(t, env, "KITTY_WINDOW_ID", 1)
	assertEnvContains(t, env, "PATH=/bin")
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

func assertEnvKeyAbsent(t *testing.T, env []string, key string) {
	t.Helper()

	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			t.Fatalf("env %v unexpectedly contains key %q", env, key)
		}
	}
}

func assertEnvKeyCount(t *testing.T, env []string, key string, want int) {
	t.Helper()

	prefix := key + "="
	got := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			got++
		}
	}

	if got != want {
		t.Fatalf("env %v has key %q %d times, want %d", env, key, got, want)
	}
}
