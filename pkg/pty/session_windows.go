//go:build windows

package pty

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"

	conpty "github.com/UserExistsError/conpty"
)

type windowsSession struct {
	pty       *conpty.ConPty
	closeOnce sync.Once
	closeErr  error
}

func start(binary string, args []string, size Size, options Options) (Session, error) {
	if !conpty.IsConPtyAvailable() {
		return nil, conpty.ErrConPtyUnsupported
	}

	commandLine := commandLine(binary, args)
	session, err := conpty.Start(
		commandLine,
		conpty.ConPtyDimensions(size.Cols, size.Rows),
		conpty.ConPtyEnv(options.Env),
		conpty.ConPtyWorkDir(options.Dir),
	)
	if err != nil {
		return nil, fmt.Errorf("start conpty: %w", err)
	}

	return &windowsSession{pty: session}, nil
}

func (s *windowsSession) Read(p []byte) (int, error) {
	n, err := s.pty.Read(p)
	if n == 0 && isConPTYEOF(err) {
		return 0, io.EOF
	}

	return n, err
}

func (s *windowsSession) Write(p []byte) (int, error) {
	return s.pty.Write(p)
}

func (s *windowsSession) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = s.pty.Close()
	})

	return s.closeErr
}

func (s *windowsSession) Pid() int {
	return s.pty.Pid()
}

func (s *windowsSession) Resize(size Size) error {
	size = NormalizeSize(size)
	if err := s.pty.Resize(size.Cols, size.Rows); err != nil {
		return fmt.Errorf("resize conpty: %w", err)
	}

	return nil
}

func (s *windowsSession) Wait(ctx context.Context) (ExitStatus, error) {
	code, err := s.pty.Wait(ctx)
	status := ExitStatus{Code: int(code)}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return status, ctxErr
		}

		return status, fmt.Errorf("wait process: %w", err)
	}

	return status, nil
}

func commandLine(binary string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, syscall.EscapeArg(binary))
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}

	return strings.Join(parts, " ")
}

func isConPTYEOF(err error) bool {
	return errors.Is(err, os.ErrClosed) ||
		errors.Is(err, syscall.ERROR_BROKEN_PIPE) ||
		errors.Is(err, syscall.Errno(6)) ||
		errors.Is(err, syscall.Errno(233))
}
