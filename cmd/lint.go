package cmd

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

// registerQuality adds `togo format` (Pint-style) and `togo lint` (PHPStan-style):
// they wrap the Go toolchain (gofmt/goimports/gofumpt, go vet, staticcheck,
// golangci-lint) and the web toolchain (prettier/eslint) so a togo project has one
// consistent code-standards command. Tools that aren't installed are skipped with
// an install hint rather than failing.
func registerQuality(root *cobra.Command) {
	format := &cobra.Command{
		Use:     "format",
		Short:   "Format the codebase (gofmt/goimports/gofumpt + web prettier)",
		GroupID: groupProject,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			dir := proj.Root
			ui.Step("gofmt")
			runTool(dir, "gofmt", "-w", ".")
			if has("goimports") {
				ui.Step("goimports")
				runTool(dir, "goimports", "-w", ".")
			}
			if has("gofumpt") {
				ui.Step("gofumpt")
				runTool(dir, "gofumpt", "-w", ".")
			}
			formatWeb(dir)
			ui.Success("formatted")
			return nil
		},
	}

	lint := &cobra.Command{
		Use:     "lint",
		Short:   "Lint / static-analyze the codebase (go vet, staticcheck, golangci-lint, eslint)",
		GroupID: groupProject,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			fix, _ := cmd.Flags().GetBool("fix")
			dir := proj.Root
			ok := true

			ui.Step("go vet ./...")
			ok = runTool(dir, "go", "vet", "./...") && ok

			switch {
			case has("golangci-lint"):
				ui.Step("golangci-lint run")
				gargs := []string{"run"}
				if fix {
					gargs = append(gargs, "--fix")
				}
				ok = runTool(dir, "golangci-lint", gargs...) && ok
			case has("staticcheck"):
				ui.Step("staticcheck ./...")
				ok = runTool(dir, "staticcheck", "./...") && ok
			default:
				ui.Warn("golangci-lint/staticcheck not found — install golangci-lint for deeper analysis")
			}

			lintWeb(dir, fix)
			if !ok {
				ui.Error("lint found issues")
				os.Exit(1)
			}
			ui.Success("lint passed")
			return nil
		},
	}
	lint.Flags().Bool("fix", false, "apply autofixes where supported")

	root.AddCommand(format, lint)
}

// formatWeb runs prettier in web/ when it's installed there.
func formatWeb(dir string) {
	web := filepath.Join(dir, "web")
	if !fileExists(filepath.Join(web, "node_modules", ".bin", "prettier")) {
		return
	}
	ui.Step("prettier (web)")
	runTool(web, filepath.Join("node_modules", ".bin", "prettier"), "--write", ".")
}

// lintWeb runs eslint in web/ when it's installed there.
func lintWeb(dir string, fix bool) {
	web := filepath.Join(dir, "web")
	bin := filepath.Join(web, "node_modules", ".bin", "eslint")
	if !fileExists(bin) {
		return
	}
	ui.Step("eslint (web)")
	eargs := []string{"."}
	if fix {
		eargs = append(eargs, "--fix")
	}
	runTool(web, filepath.Join("node_modules", ".bin", "eslint"), eargs...)
}

// runTool runs a command in dir, streaming output; returns true on success.
func runTool(dir, name string, args ...string) bool {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run() == nil
}

func has(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}
