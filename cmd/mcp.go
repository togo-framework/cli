package cmd

import (
	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

func registerMCP(root *cobra.Command) {
	install := &cobra.Command{
		Use:     "mcp:install",
		Short:   "Wire the togo MCP server into an AI agent (claude-code, cursor, …)",
		GroupID: groupMCP,
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			role, _ := cmd.Flags().GetString("role")
			ui.Info("Wiring togo MCP server into %s (role: %s)", agent, role)
			ui.Warn("MCP server lands in the `mcp` repo phase — this records intent for now")
			return nil
		},
	}
	install.Flags().String("agent", "claude-code", "target agent: claude-code|cursor|windsurf|cline|json")
	install.Flags().String("role", "admin", "MCP role: admin|user")

	serve := &cobra.Command{
		Use:     "mcp:serve",
		Short:   "Run the togo MCP server",
		GroupID: groupMCP,
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Warn("MCP server implementation lands in the `mcp` repo phase")
			return nil
		},
	}

	root.AddCommand(install, serve)
}
