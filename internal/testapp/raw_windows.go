//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

const (
	enableProcessedInput = 0x0001
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
)

func enableRawMode() (func(), error) {
	handle := windows.Handle(os.Stdin.Fd())
	var original uint32
	if err := windows.GetConsoleMode(handle, &original); err != nil {
		return nil, err
	}

	raw := original
	raw &^= enableProcessedInput | enableLineInput | enableEchoInput

	if err := windows.SetConsoleMode(handle, raw); err != nil {
		return nil, err
	}

	return func() {
		_ = windows.SetConsoleMode(handle, original)
	}, nil
}
