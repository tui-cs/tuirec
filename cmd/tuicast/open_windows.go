//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
)

func openFile(path string) error {
	cmd := exec.Command("cmd", "/c", "start", "", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func copyToClipboard(text string) error {
	cmd := exec.Command("cmd", "/c", "echo|set /p="+text+"|clip")
	// PowerShell is more reliable for arbitrary text.
	cmd = exec.Command("powershell", "-NoProfile", "-Command",
		"Set-Clipboard -Value '"+strings.ReplaceAll(text, "'", "''")+"'")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
