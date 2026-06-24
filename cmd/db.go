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
		appCmd("migrate", "Apply the schema to the database (driver-agnostic)", []string{"run", "./cmd/migrate"}),
		appCmd("seed", "Seed the database", []string{"run", "./cmd/seed"}),
		dbCmd("migrate:diff", "Generate an Atlas migration (advanced)", []string{"migrate", "diff", "--env", "local"}),
		dbCmd("migrate:status", "Show Atlas migration status (advanced)", []string{"migrate", "status", "--env", "local"}),
		composeCmd("db:up", "Start the project's database stack (docker compose up -d)", []string{"up", "-d"}),
		composeCmd("db:down", "Stop the project's database stack (docker compose down)", []string{"down"}),
	)
}

// composeCmd builds a command that runs docker compose in the project root —
// brings the chosen DB stack (postgres / togo-postgres / supabase / mysql / mongodb)
// up or down. SQLite needs no stack.
func composeCmd(name, short string, composeArgs []string) *cobra.Command {
	return &cobra.Command{
		Use:     name,
		Short:   short,
		GroupID: groupDB,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			if _, err := exec.LookPath("docker"); err != nil {
				ui.Warn("docker not found on PATH — install Docker to manage the database stack")
				return nil
			}
			if _, err := os.Stat(proj.Root + "/docker-compose.yml"); err != nil {
				ui.Warn("no docker-compose.yml — this project uses SQLite (no stack to manage)")
				return nil
			}
			return shellTool(proj, "docker", append([]string{"compose"}, append(composeArgs, args...)...), "https://docs.docker.com/get-docker/")
		},
	}
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
