//go:build windows

package pty

import "testing"

func TestCommandLineEscapesArguments(t *testing.T) {
	t.Parallel()

	got := commandLine(`C:\Program Files\App\app.exe`, []string{"plain", "two words", `quote"here`})
	want := `"C:\Program Files\App\app.exe" plain "two words" quote\"here`
	if got != want {
		t.Fatalf("commandLine() = %q, want %q", got, want)
	}
}
