package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

func registerPlugin(root *cobra.Command) {
	install := &cobra.Command{
		Use:     "install <owner/repo>",
		Short:   "Install a togo plugin from a GitHub repository",
		GroupID: groupPlugin,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			repo := args[0]
			if strings.Count(repo, "/") < 1 {
				return fmt.Errorf("expected owner/repo (e.g. fadymondy/cms), got %q", repo)
			}
			module := "github.com/" + repo
			ui.Info("Installing plugin %s", repo)

			if _, err := exec.LookPath("go"); err == nil {
				c := exec.Command("go", "get", module+"@latest")
				c.Dir = proj.Root
				c.Stdout, c.Stderr = os.Stdout, os.Stderr
				c.Env = os.Environ()
				if err := c.Run(); err != nil {
					return fmt.Errorf("go get %s: %w", module, err)
				}
			} else {
				ui.Warn("go not found — skipped `go get %s`", module)
			}

			ui.Success("Fetched %s", module)
			ui.Step("register it in your kernel and run `togo generate` to wire routes/migrations")
			ui.Step("(plugin auto-discovery lands in the framework kernel phase)")
			return nil
		},
	}

	list := &cobra.Command{
		Use:     "plugin:list",
		Short:   "List installed plugins",
		GroupID: groupPlugin,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			if len(proj.Plugins) == 0 {
				ui.Info("No plugins installed")
				return nil
			}
			for _, p := range proj.Plugins {
				ui.Step("• %s", p)
			}
			return nil
		},
	}

	root.AddCommand(install, list)
}
