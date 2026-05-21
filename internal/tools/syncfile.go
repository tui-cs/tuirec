//go:build ignore

// syncfile copies src to dst. Used by go:generate for cross-platform file sync.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: syncfile <src> <dst>\n")
		os.Exit(1)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "syncfile: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "syncfile: %v\n", err)
		os.Exit(1)
	}
}
