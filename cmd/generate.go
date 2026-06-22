package cmd

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

// genStep is one stage of the code-generation pipeline.
type genStep struct {
	name     string
	bin      string   // executable to look up
	args     []string // arguments
	softFail bool     // warn & continue instead of aborting
	skipMsg  string   // shown when the tool is not installed
}

func registerGenerate(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"gen"},
		Short:   "Run the codegen pipeline: sqlc → gqlgen → OpenAPI export",
		GroupID: groupCodegen,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			// Always tidy: make:resource introduces imports (orm, validation,
			// faker) that the gqlgen/openapi compile steps must resolve.
			if err := goModTidy(proj.Root); err != nil {
				ui.Warn("go mod tidy failed: %v", err)
			}
			only, _ := cmd.Flags().GetStringSlice("only")
			skip, _ := cmd.Flags().GetStringSlice("skip")
			return runGenerate(proj, only, skip)
		},
	}
	cmd.Flags().StringSlice("only", nil, "run only these steps (sqlc,gqlgen,openapi)")
	cmd.Flags().StringSlice("skip", nil, "skip these steps")
	root.AddCommand(cmd)
}

func runGenerate(proj *config.Project, only, skip []string) error {
	// Order is dictated by data flow; OpenAPI export compiles the whole program
	// and therefore runs last.
	// Tools run via `go run <pkg>@<version>` so nothing needs to be pre-installed.
	// Every step is soft-fail: a missing tool or partial setup warns but never
	// breaks the `togo generate && togo migrate && togo serve` chain.
	steps := []genStep{
		// sqlc uses a CGO Postgres parser, so prefer the installed binary rather
		// than compiling from source (brittle across toolchains).
		{name: "sqlc", bin: "sqlc", args: []string{"generate"}, softFail: true, skipMsg: "install: brew install sqlc (or https://docs.sqlc.dev/en/latest/overview/install.html)"},
		// No @version: resolves gqlgen from the project's go.mod so the CLI and the
		// runtime it generates against are always the same version.
		{name: "gqlgen", bin: "go", args: []string{"run", "github.com/99designs/gqlgen", "generate"}, softFail: true},
		// Migrations apply via `togo migrate` (driver-agnostic, applies the schema
		// files directly — no Atlas/dev-url needed). Atlas is optional/advanced and
		// available via `togo migrate:diff`, so it is NOT in the default pipeline.
		{name: "openapi", bin: "go", args: []string{"run", "./cmd/api", "openapi"}, softFail: true},
	}

	onlySet := toSet(only)
	skipSet := toSet(skip)

	ui.Info("Codegen pipeline (%s)", proj.Name)
	for _, s := range steps {
		if len(onlySet) > 0 && !onlySet[s.name] {
			continue
		}
		if skipSet[s.name] {
			ui.Step("%s  %s", ui.Label("SKIP"), s.name)
			continue
		}
		if err := runStep(proj.Root, s); err != nil {
			return err
		}
	}
	ui.Success("Codegen complete")
	return nil
}

func runStep(root string, s genStep) error {
	if _, err := exec.LookPath(s.bin); err != nil {
		ui.Warn("%s not found — skipping %s step. %s", s.bin, s.name, s.skipMsg)
		return nil
	}
	start := time.Now()
	c := exec.Command(s.bin, s.args...)
	c.Dir = root
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	c.Env = os.Environ()
	ui.Step("%s  %s", ui.Label("RUN"), s.name)
	if err := c.Run(); err != nil {
		if s.softFail {
			ui.Warn("%s failed (continuing): %v", s.name, err)
			return nil
		}
		return err
	}
	ui.Step("%s  %s (%s)", ui.Label("OK"), s.name, time.Since(start).Round(time.Millisecond))
	return nil
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[strings.TrimSpace(x)] = true
	}
	return m
}
