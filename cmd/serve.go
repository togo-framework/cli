package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

func registerServe(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Run the app (API: GraphQL + REST/OpenAPI) with hot reload",
		GroupID: groupProject,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			ui.Info("Serving %s", proj.Name)
			ui.Step("GraphQL  %s", proj.API.GraphQL)
			ui.Step("REST     %s", proj.API.REST)
			ui.Step("Docs     %s", proj.API.Docs)

			c := exec.Command("go", "run", "./cmd/api")
			c.Dir = proj.Root
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			c.Env = os.Environ()
			return c.Run()
		},
	}
	root.AddCommand(cmd)
}
