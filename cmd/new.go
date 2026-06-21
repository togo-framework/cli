package cmd

import (
	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/scaffold"
	"github.com/togo-framework/cli/internal/ui"
)

func registerNew(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "new <app>",
		Short:   "Scaffold a new togo project",
		GroupID: groupProject,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			module, _ := cmd.Flags().GetString("module")
			dir, _ := cmd.Flags().GetString("dir")
			force, _ := cmd.Flags().GetBool("force")
			dry, _ := cmd.Flags().GetBool("dry-run")

			opts := scaffold.Options{
				App:    name,
				Module: module,
				Dir:    dir,
				Force:  force,
				DryRun: dry,
			}
			created, err := scaffold.New(opts)
			if err != nil {
				return err
			}
			if dry {
				ui.Warn("dry-run: would scaffold %d files into %s", created, opts.Resolve().Dir)
				return nil
			}
			ui.Success("Created togo project %q (%d files)", name, created)
			ui.Step("cd %s", name)
			ui.Step("togo make:resource Post title:string body:text?")
			ui.Step("togo generate && togo migrate && togo serve")
			return nil
		},
	}
	cmd.Flags().String("module", "", "Go module path (default: github.com/<app>/<app>)")
	cmd.Flags().String("dir", "", "target directory (default: ./<app>)")
	root.AddCommand(cmd)
}
