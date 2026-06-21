package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

func registerDB(root *cobra.Command) {
	root.AddCommand(
		dbCmd("migrate", "Apply pending Atlas migrations", []string{"migrate", "apply", "--env", "local"}),
		dbCmd("migrate:status", "Show migration status", []string{"migrate", "status", "--env", "local"}),
		dbCmd("migrate:diff", "Generate a migration from the desired schema", []string{"migrate", "diff", "--env", "local"}),
		appCmd("seed", "Seed the database", []string{"run", "./cmd/seed"}),
		appCmd("migrate:fresh", "Drop everything and re-migrate, then seed", []string{"run", "./cmd/db", "fresh"}),
		appCmd("db:reset", "Reset the database", []string{"run", "./cmd/db", "reset"}),
	)
}

// dbCmd builds a command that shells out to the Atlas binary.
func dbCmd(name, short string, atlasArgs []string) *cobra.Command {
	return &cobra.Command{
		Use:     name,
		Short:   short,
		GroupID: groupDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			return shellTool(proj, "atlas", append(atlasArgs, args...), "install: https://atlasgo.io")
		},
	}
}

// appCmd builds a command that shells to a generated Go entrypoint in the app.
func appCmd(name, short string, goArgs []string) *cobra.Command {
	return &cobra.Command{
		Use:     name,
		Short:   short,
		GroupID: groupDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			return shellTool(proj, "go", append(goArgs, args...), "")
		},
	}
}

func shellTool(proj *config.Project, bin string, args []string, hint string) error {
	if _, err := exec.LookPath(bin); err != nil {
		ui.Warn("%s not found. %s", bin, hint)
		return nil
	}
	c := exec.Command(bin, args...)
	c.Dir = proj.Root
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = os.Environ()
	return c.Run()
}
