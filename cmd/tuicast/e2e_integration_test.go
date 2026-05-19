//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gui-cs/TUIcast/pkg/gif"
)

func TestRecordCommandEndToEndGIF(t *testing.T) {
	repo := repoRoot(t)
	if !aggAvailable(repo) {
		t.Skip("agg not installed")
	}

	tuicast := buildTuicast(t, repo)
	testapp := buildTestapp(t, repo)
	output := filepath.Join(t.TempDir(), "cli-demo.gif")
	castOutput := filepath.Join(t.TempDir(), "cli-demo.cast")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, tuicast,
		"record",
		"--binary", testapp,
		"--keystrokes", "wait:1000,ArrowRight,ArrowDown,Hi,wait:500,Ctrl+Q",
		"--output", output,
		"--cast-output", castOutput,
		"--cols", "80",
		"--rows", "24",
	)
	cmd.Dir = repo
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("tuicast record: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	if _, err := os.Stat(castOutput); err != nil {
		t.Fatalf("stat cast output: %v", err)
	}

	validation, err := gif.Validate(output)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	if validation.Frames < 2 {
		t.Fatalf("Frames = %d, want >= 2", validation.Frames)
	}

	gotStdout := stdout.String()
	if !strings.Contains(gotStdout, "Wrote "+output) || !strings.Contains(gotStdout, "Wrote "+castOutput) {
		t.Fatalf("stdout = %q, want output and cast paths", gotStdout)
	}
}

func aggAvailable(repo string) bool {
	for _, candidate := range []string{
		filepath.Join(repo, "tools", "agg.exe"),
		filepath.Join(repo, "tools", "agg"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}

	_, err := exec.LookPath("agg")

	return err == nil
}

func buildTuicast(t *testing.T, repo string) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), executableName("tuicast"))
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/tuicast")
	cmd.Dir = repo

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build cmd/tuicast: %v\n%s", err, output)
	}

	return binary
}

func buildTestapp(t *testing.T, repo string) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), executableName("tuicast-testapp"))
	cmd := exec.Command("go", "build", "-o", binary, "./internal/testapp")
	cmd.Dir = repo

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build internal/testapp: %v\n%s", err, output)
	}

	return binary
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}

		dir = parent
	}
}

func executableName(name string) string {
	if filepath.Ext(name) != "" {
		return name
	}

	if os.PathSeparator == '\\' {
		return name + ".exe"
	}

	return name
}
