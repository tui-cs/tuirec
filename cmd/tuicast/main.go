package main

import (
	"fmt"
	"os"

	"github.com/gui-cs/TUIcast/pkg/record"
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
	root.AddCommand(&cobra.Command{
		Use:   record.CommandName,
		Short: "Record a terminal app (planned)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s command is not implemented yet", record.CommandName)
		},
	})

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
