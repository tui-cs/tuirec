package demoagg

import (
	"os"
	"path/filepath"
)

// DefaultPath prefers the repo-local demo install, then falls back to PATH.
func DefaultPath() string {
	for _, candidate := range []string{
		filepath.Join("tools", "agg.exe"),
		filepath.Join("tools", "agg"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return "agg"
}
