package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

// mcpModule is the go-installable path of the togo MCP server.
const mcpModule = "github.com/togo-framework/mcp"

func registerMCP(root *cobra.Command) {
	serve := &cobra.Command{
		Use:     "mcp:serve",
		Short:   "Run the togo MCP server (stdio) for AI agents",
		GroupID: groupMCP,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Prefer an installed togo-mcp binary; fall back to `go run`.
			var c *exec.Cmd
			if path, err := exec.LookPath("togo-mcp"); err == nil {
				c = exec.Command(path)
			} else if goAvailable() {
				c = exec.Command("go", "run", mcpModule+"@latest")
			} else {
				return fmt.Errorf("togo-mcp not found and go is unavailable; install: go install %s@latest", mcpModule)
			}
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			c.Env = os.Environ()
			return c.Run()
		},
	}

	install := &cobra.Command{
		Use:     "mcp:install",
		Short:   "Wire the togo MCP server into an AI agent (claude-code, cursor, …)",
		GroupID: groupMCP,
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			return installMCP(agent)
		},
	}
	install.Flags().String("agent", "claude-code", "target agent: claude-code|cursor|windsurf|cline")

	root.AddCommand(serve, install)
}

// installMCP writes/merges an MCP server entry into the agent's config file.
func installMCP(agent string) error {
	// All listed agents read a JSON file with an "mcpServers" map; the path differs.
	var path string
	switch agent {
	case "claude-code", "cursor", "windsurf":
		path = ".mcp.json" // project-scoped
	case "cline":
		path = ".cline/mcp.json"
	default:
		return fmt.Errorf("unknown agent %q (claude-code|cursor|windsurf|cline)", agent)
	}

	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &doc)
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers["togo"] = map[string]any{
		"command": "togo",
		"args":    []string{"mcp:serve"},
	}
	doc["mcpServers"] = servers

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return err
	}
	ui.Success("Wired togo MCP server into %s (%s)", agent, path)
	ui.Step("tools: make_resource, generate, list_resources, migrate")
	return nil
}
