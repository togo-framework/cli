package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

func registerTest(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "test [packages]",
		Short:   "Run the project's Go tests (set TEST_DATABASE_URL for DB-backed tests)",
		GroupID: groupProject,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			pkgs := "./..."
			if len(args) > 0 {
				pkgs = args[0]
			}
			ui.Info("Running tests (%s)", proj.Name)
			c := exec.Command("go", "test", pkgs)
			c.Dir = proj.Root
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			c.Env = os.Environ()
			if err := c.Run(); err != nil {
				return err
			}
			ui.Success("Tests passed")
			return nil
		},
	}
	root.AddCommand(cmd)
}
