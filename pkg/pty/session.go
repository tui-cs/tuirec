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
	size = NormalizeSize(size)
	options.Env = normalizeEnv(options.Env)

	return start(binary, args, size, options)
}

// NormalizeSize replaces non-positive dimensions with the defaults Start
// applies, so callers (e.g. the sixel geometry reports) can mirror the size the
// PTY will actually use.
func NormalizeSize(size Size) Size {
	if size.Cols <= 0 {
		size.Cols = defaultCols
	}

	if size.Rows <= 0 {
		size.Rows = defaultRows
	}

	return size
}

func normalizeEnv(env []string) []string {
	normalized := os.Environ()
	if env != nil {
		normalized = mergeEnv(normalized, env)
	}

	normalized = scrubGraphicsIdentityEnv(normalized)

	normalized = appendDefaultEnv(normalized, "TERM", "xterm-256color")
	normalized = appendDefaultEnv(normalized, "COLORTERM", "truecolor")

	return normalized
}

// kittyIdentityVars are environment variables that mark the host terminal as
// Kitty- or Ghostty-class. Apps detect Kitty graphics-protocol support purely
// from these (e.g. Terminal.Gui's KittyGraphicsSupportDetector keys off
// KITTY_WINDOW_ID, never a query/response) — see also the TERM_PROGRAM handling
// in scrubGraphicsIdentityEnv.
//
// tuirec presents a deterministic, sixel-capable xterm identity: TERM is forced
// to xterm-256color and the DA1/geometry interceptor advertises sixel. The agg
// replay pipeline renders sixel but not Kitty graphics. If these markers leak in
// from the recording shell (i.e. recording from inside kitty/ghostty), the app
// detects Kitty, prefers it over sixel, and emits Kitty graphics that agg cannot
// render — the GIF comes out blank. Strip them so the app uses the sixel path
// tuirec actually supports.
var kittyIdentityVars = map[string]bool{
	"KITTY_WINDOW_ID":        true,
	"KITTY_PID":              true,
	"KITTY_INSTALLATION_DIR": true,
	"KITTY_LISTEN_ON":        true,
	"KITTY_PUBLIC_KEY":       true,
	"GHOSTTY_RESOURCES_DIR":  true,
	"GHOSTTY_BIN_DIR":        true,
}

// scrubGraphicsIdentityEnv removes the environment variables that would make a
// recorded app believe it is running under a Kitty graphics-capable terminal.
// TERM_PROGRAM is only dropped when it names a Kitty-class terminal (kitty or
// ghostty), so unrelated values (vscode, iTerm.app, WezTerm, Apple_Terminal,
// ...) and any app behavior keyed off them are left untouched.
func scrubGraphicsIdentityEnv(env []string) []string {
	dropTermProgram := false
	for _, entry := range env {
		if key, ok := envKey(entry); ok && key == "TERM_PROGRAM" {
			value := entry[len(key)+1:]
			if strings.EqualFold(value, "kitty") || strings.EqualFold(value, "ghostty") {
				dropTermProgram = true
				break
			}
		}
	}

	filtered := env[:0]
	for _, entry := range env {
		key, ok := envKey(entry)
		if ok && kittyIdentityVars[key] {
			continue
		}
		if ok && dropTermProgram && (key == "TERM_PROGRAM" || key == "TERM_PROGRAM_VERSION") {
			continue
		}
		filtered = append(filtered, entry)
	}

	return filtered
}

func mergeEnv(base []string, overrides []string) []string {
	merged := append([]string{}, base...)
	for _, override := range overrides {
		key, ok := envKey(override)
		if !ok {
			merged = append(merged, override)
			continue
		}
		merged = setEnv(merged, key, override)
	}
	return merged
}

func envKey(entry string) (string, bool) {
	idx := strings.IndexByte(entry, '=')
	if idx <= 0 {
		return "", false
	}
	return entry[:idx], true
}

func setEnv(env []string, key string, value string) []string {
	prefix := key + "="
	filtered := env[:0]
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				filtered = append(filtered, value)
				replaced = true
			}
			continue
		}
		filtered = append(filtered, entry)
	}
	if !replaced {
		filtered = append(filtered, value)
	}
	return filtered
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
