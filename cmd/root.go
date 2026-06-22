package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

// Command group IDs used to organize `togo help` like artisan's grouped list.
const (
	groupProject = "project"
	groupDB      = "database"
	groupMake    = "make"
	groupCodegen = "codegen"
	groupPlugin  = "plugin"
	groupMCP     = "mcp"
	groupInfra   = "infra"
)

var rootCmd = &cobra.Command{
	Use:   "togo",
	Short: "togo — an artisan-like CLI for the Go + sqlc + Atlas + GraphQL/OpenAPI + Next.js stack",
	Long: ui.Banner() + `
togo is the command-line companion for the togo framework. It scaffolds projects,
generates resources across GraphQL + REST in one shot, runs migrations, installs
plugins, wires MCP into your AI agent, and deploys to any cloud.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the CLI entrypoint. It adds git-style external plugin dispatch on
// top of cobra: an unknown `togo <x>` is resolved to a `togo-<x>` executable on
// PATH or in ~/.togo/bin before cobra reports "unknown command".
func Execute() {
	if dispatched := tryExternalPlugin(os.Args[1:]); dispatched {
		return
	}
	if err := rootCmd.Execute(); err != nil {
		ui.Error("%s", err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool("force", false, "overwrite files that already exist")
	rootCmd.PersistentFlags().Bool("dry-run", false, "show what would change without writing any files")
	rootCmd.PersistentFlags().Bool("no-color", false, "disable colored output")
	rootCmd.PersistentFlags().String("config", "", "path to togo.yaml (default: search up from cwd)")

	rootCmd.AddGroup(
		&cobra.Group{ID: groupProject, Title: "Project:"},
		&cobra.Group{ID: groupMake, Title: "Make (generators):"},
		&cobra.Group{ID: groupCodegen, Title: "Codegen:"},
		&cobra.Group{ID: groupDB, Title: "Database:"},
		&cobra.Group{ID: groupPlugin, Title: "Plugins:"},
		&cobra.Group{ID: groupMCP, Title: "MCP / AI:"},
		&cobra.Group{ID: groupInfra, Title: "Infra / Deploy:"},
	)

	cobra.OnInitialize(func() {
		if nc, _ := rootCmd.PersistentFlags().GetBool("no-color"); nc {
			ui.DisableColor()
		}
	})

	registerVersion(rootCmd)
	registerUpgrade(rootCmd)
	registerTest(rootCmd)
	registerNew(rootCmd)
	registerServe(rootCmd)
	registerQuality(rootCmd)
	RegisterMake(rootCmd)
	registerGenerate(rootCmd)
	registerDB(rootCmd)
	registerPlugin(rootCmd)
	registerMCP(rootCmd)
	registerInfra(rootCmd)
	registerStubPublish(rootCmd)
}

// loadProject loads the nearest togo.yaml relative to cwd. Commands that operate
// on a project call this; project-creation commands (new) do not.
func loadProject(cmd *cobra.Command) (*config.Project, error) {
	path, _ := cmd.Flags().GetString("config")
	p, err := config.Load(path)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return nil, fmt.Errorf("not inside a togo project (no togo.yaml found). Run `togo new <app>` first")
		}
		return nil, err
	}
	return p, nil
}
