package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version metadata, overridable at build time via -ldflags.
var (
	Version = "0.1.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

func registerVersion(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "version",
		Short:   "Print the togo CLI version",
		GroupID: groupProject,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("togo %s (commit %s, built %s, %s)\n", Version, Commit, Date, runtime.Version())
		},
	})
}
