package pty_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gui-cs/TUIcast/pkg/pty"
)

func TestSessionEchoRoundTrip(t *testing.T) {
	t.Parallel()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	session, err := pty.Start(
		executable,
		[]string{"-test.run=TestHelperProcess", "--", "echo"},
		pty.Size{Cols: 80, Rows: 24},
		pty.Options{Env: append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")},
	)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer session.Close()

	output := make(chan string, 32)
	readErr := make(chan error, 1)
	go readChunks(ctx, session, output, readErr)

	waitForOutput(t, ctx, output, "ready>")

	if _, err := session.Write([]byte("hello\r\n")); err != nil {
		t.Fatalf("session.Write hello: %v", err)
	}

	waitForOutput(t, ctx, output, "echo: hello")

	if _, err := session.Write([]byte("exit\r\n")); err != nil {
		t.Fatalf("session.Write exit: %v", err)
	}

	status, err := session.Wait(ctx)
	if err != nil {
		t.Fatalf("session.Wait: %v", err)
	}

	if status.Code != 0 {
		t.Fatalf("exit code = %d, want 0", status.Code)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	defer os.Exit(0)

	fmt.Fprintln(os.Stdout, "ready>")
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
			os.Exit(1)
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "exit" {
			return
		}

		fmt.Fprintf(os.Stdout, "echo: %s\n", line)
	}
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
