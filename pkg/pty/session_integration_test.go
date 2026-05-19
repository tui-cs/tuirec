package pty_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gui-cs/TUIcast/pkg/pty"
)

func TestSessionRunsTestappAndQuitsWithCtrlQ(t *testing.T) {
	t.Parallel()

	binary := buildTestapp(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := pty.Start(
		binary,
		nil,
		pty.Size{Cols: 80, Rows: 24},
		pty.Options{Env: os.Environ()},
	)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer session.Close()

	output := make(chan string, 32)
	readErr := make(chan error, 1)
	go readChunks(ctx, session, output, readErr)

	waitForOutput(t, ctx, output, "TUIcast testapp ready")

	if _, err := session.Write([]byte{0x11}); err != nil {
		t.Fatalf("write Ctrl+Q: %v", err)
	}

	status, err := session.Wait(ctx)
	if err != nil {
		t.Fatalf("session.Wait: %v", err)
	}

	if status.Code != 0 {
		t.Fatalf("exit code = %d, want 0", status.Code)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("session.Close: %v", err)
	}

	select {
	case err := <-readErr:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("read error after clean exit = %v, want EOF", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for read EOF after clean exit")
	}
}

func buildTestapp(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), executableName("tuicast-testapp"))
	cmd := exec.Command("go", "build", "-o", binary, "./internal/testapp")
	cmd.Dir = repoRoot(t)

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
		t.Fatalf("os.Getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", dir)
		}

		dir = parent
	}
}

func executableName(name string) string {
	if os.PathSeparator == '\\' {
		return name + ".exe"
	}

	return name
}

func readChunks(ctx context.Context, reader io.Reader, output chan<- string, readErr chan<- error) {
	buffer := make([]byte, 1024)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			select {
			case output <- string(buffer[:n]):
			case <-ctx.Done():
				return
			}
		}

		if err != nil {
			readErr <- err

			return
		}
	}
}

func waitForOutput(t *testing.T, ctx context.Context, output <-chan string, want string) {
	t.Helper()

	var seen strings.Builder
	for {
		select {
		case chunk := <-output:
			seen.WriteString(chunk)
			if strings.Contains(seen.String(), want) {
				return
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for %q; saw %q", want, seen.String())
		}
	}
}
