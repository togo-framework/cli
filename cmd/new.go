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

			// Resolve Go modules so the project is runnable immediately.
			if skip, _ := cmd.Flags().GetBool("skip-tidy"); !skip {
				if err := goModTidy(opts.Resolve().Dir); err != nil {
					ui.Warn("go mod tidy failed: %v (run it manually in %s)", err, opts.Resolve().Dir)
				}
			}

			ui.Step("cd %s", name)
			ui.Step("togo make:resource Post title:string body:text:nullable")
			ui.Step("togo generate && togo migrate && togo serve")
			return nil
		},
	}
	cmd.Flags().String("module", "", "Go module path (default: github.com/<app>/<app>)")
	cmd.Flags().String("dir", "", "target directory (default: ./<app>)")
	cmd.Flags().Bool("skip-tidy", false, "do not run `go mod tidy` after scaffolding")
	root.AddCommand(cmd)
}
