//go:build unix

package pty

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"

	creackpty "github.com/creack/pty"
)

type unixSession struct {
	file *os.File
	cmd  *exec.Cmd

	waitOnce sync.Once
	waitCh   chan waitResult
}

type waitResult struct {
	status ExitStatus
	err    error
}

func start(binary string, args []string, size Size, options Options) (Session, error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = options.Dir
	cmd.Env = options.Env

	file, err := creackpty.StartWithSize(cmd, &creackpty.Winsize{
		Rows: uint16(size.Rows),
		Cols: uint16(size.Cols),
	})
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	session := &unixSession{
		file:   file,
		cmd:    cmd,
		waitCh: make(chan waitResult, 1),
	}
	session.waitOnce.Do(func() {
		go session.wait()
	})

	return session, nil
}

func (s *unixSession) Read(p []byte) (int, error) {
	return s.file.Read(p)
}

func (s *unixSession) Write(p []byte) (int, error) {
	return s.file.Write(p)
}

func (s *unixSession) Close() error {
	var closeErr error
	if s.file != nil {
		closeErr = s.file.Close()
	}

	if s.cmd.Process == nil {
		return closeErr
	}

	if err := s.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		if closeErr != nil {
			return fmt.Errorf("close pty: %w; kill process: %v", closeErr, err)
		}

		return fmt.Errorf("kill process: %w", err)
	}

	return closeErr
}

func (s *unixSession) Pid() int {
	if s.cmd.Process == nil {
		return 0
	}

	return s.cmd.Process.Pid
}

func (s *unixSession) Resize(size Size) error {
	size = normalizeSize(size)
	err := creackpty.Setsize(s.file, &creackpty.Winsize{
		Rows: uint16(size.Rows),
		Cols: uint16(size.Cols),
	})
	if err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}

	return nil
}

func (s *unixSession) Wait(ctx context.Context) (ExitStatus, error) {
	select {
	case result := <-s.waitCh:
		return result.status, result.err
	case <-ctx.Done():
		return ExitStatus{}, ctx.Err()
	}
}

func (s *unixSession) wait() {
	err := s.cmd.Wait()
	status := ExitStatus{}
	if s.cmd.ProcessState != nil {
		status.Code = s.cmd.ProcessState.ExitCode()
	}

	if err != nil {
		s.waitCh <- waitResult{status: status, err: fmt.Errorf("wait process: %w", err)}

		return
	}

	s.waitCh <- waitResult{status: status}
}
