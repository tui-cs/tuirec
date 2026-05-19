package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := &cobra.Command{
		Use:   "tuicast",
		Short: "Record terminal apps and produce animated GIFs",
		Long:  "TUIcast records terminal application sessions and renders them as animated GIFs.",
	}

	root.SetVersionTemplate("tuicast {{.Version}}\n")
	root.Version = fmt.Sprintf("%s (%s, %s)", version, commit, date)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
