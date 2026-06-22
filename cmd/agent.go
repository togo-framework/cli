package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

// registerAgent adds `togo agent` — agentic scaffolding driven by Claude Code over
// the togo MCP. There is NO API key: the connected agent interprets the request
// and calls the MCP tools (make_resource, generate, migrate). This command wires
// the MCP if needed and prints a ready prompt.
func registerAgent(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "agent <description>",
		Short:   "Agentic scaffolding via Claude Code over the togo MCP (no API key)",
		GroupID: groupMCP,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			desc := strings.Join(args, " ")

			if _, err := os.Stat(".mcp.json"); err != nil {
				ui.Warn("togo MCP not wired in this project")
				ui.Step("run once: togo mcp:install   (wires .mcp.json + .claude skills/agents)")
			}

			ui.Info("Agentic generation runs through Claude Code + the togo MCP — no API key.")
			ui.Step("Open Claude Code in this project and paste:")
			fmt.Printf("\n  Using the togo MCP, %s.\n  Call make_resource for each entity, then run generate and migrate.\n\n", desc)
			ui.Step("Claude drives the MCP tools (make_resource → generate → migrate) to build it.")
			return nil
		},
	}
	root.AddCommand(cmd)
}
