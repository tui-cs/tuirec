// Package pty provides a small cross-platform pseudoterminal abstraction.
package pty

import (
	"context"
	"io"
	"os"
	"strings"
)

const (
	defaultCols = 120
	defaultRows = 30
)

// Size describes terminal dimensions in character cells.
type Size struct {
	Cols int
	Rows int
}

// Options configures a spawned pseudoterminal session.
type Options struct {
	Dir string
	Env []string
}

// ExitStatus describes a completed child process.
type ExitStatus struct {
	Code int
}

// Session is a running application attached to a pseudoterminal.
type Session interface {
	io.Reader
	io.Writer

	Close() error
	Pid() int
	Resize(Size) error
	Wait(context.Context) (ExitStatus, error)
}

// Start spawns binary with args attached to a pseudoterminal.
func Start(binary string, args []string, size Size, options Options) (Session, error) {
	size = normalizeSize(size)
	options.Env = normalizeEnv(options.Env)

	return start(binary, args, size, options)
}

func normalizeSize(size Size) Size {
	if size.Cols <= 0 {
		size.Cols = defaultCols
	}

	if size.Rows <= 0 {
		size.Rows = defaultRows
	}

	return size
}

func normalizeEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	} else {
		env = append([]string{}, env...)
	}

	env = appendDefaultEnv(env, "TERM", "xterm-256color")
	env = appendDefaultEnv(env, "COLORTERM", "truecolor")

	return env
}

func appendDefaultEnv(env []string, key, value string) []string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return env
		}
	}

	return append(env, prefix+value)
}
