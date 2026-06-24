package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/toolchain"
	"github.com/togo-framework/cli/internal/ui"
)

// registerDoctor adds `togo doctor` — checks (and with --fix, installs) the
// external prerequisites togo needs: Go, Node/npm, sqlc, atlas.
func registerDoctor(root *cobra.Command) {
	var fix bool
	cmd := &cobra.Command{
		Use:     "doctor",
		Short:   "Check (and install) prerequisites: Go, Node/npm, sqlc, atlas",
		GroupID: groupProject,
		Long: `Report whether the tools togo relies on are installed, and optionally
install any that are missing (--fix). These also auto-install on first use by
togo new / generate / serve, so doctor is just an explicit check.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if fix {
				if err := toolchain.EnsureAll(); err != nil {
					ui.Warn("%v", err)
				}
			}
			ui.Info("Toolchain")
			allOK := true
			for _, t := range toolchain.Status() {
				if t.OK {
					ui.Step("%s  %-6s %s", ui.Label("OK"), t.Name, t.Version)
				} else {
					allOK = false
					ui.Warn("%s missing — run `togo doctor --fix` (or it auto-installs on first use)", t.Name)
				}
			}
			if allOK {
				ui.Success("All prerequisites present")
			} else if !fix {
				fmt.Fprintln(os.Stderr, "\nRun `togo doctor --fix` to install the missing tools.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "install any missing prerequisites")
	root.AddCommand(cmd)
}
