package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

// registerSupabase adds `togo supabase` — manage the local Supabase stack (the
// togo custom image with ParadeDB/pgvector/pg_partman). Prefers the Supabase CLI;
// falls back to the self-hosted docker-compose from the supabase plugin.
func registerSupabase(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "supabase",
		Short:   "Manage the local Supabase stack (custom image: ParadeDB/pgvector/pg_partman)",
		GroupID: groupProject,
	}
	cmd.AddCommand(
		supabaseSub("up", "Start the Supabase stack", "start", []string{"compose", "up", "-d"}),
		supabaseSub("down", "Stop the Supabase stack", "stop", []string{"compose", "down"}),
		supabaseSub("status", "Show Supabase stack status", "status", []string{"compose", "ps"}),
	)
	root.AddCommand(cmd)
}

func supabaseSub(use, short, cliArg string, composeArgs []string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return supabaseRun(cliArg, composeArgs)
		},
	}
}

// supabaseRun prefers the supabase CLI; falls back to docker compose.
func supabaseRun(cliArg string, composeArgs []string) error {
	if _, err := exec.LookPath("supabase"); err == nil {
		return runPassthrough("supabase", cliArg)
	}
	if _, err := exec.LookPath("docker"); err == nil {
		ui.Warn("supabase CLI not found — using docker compose")
		return runPassthrough("docker", composeArgs...)
	}
	ui.Error("neither the supabase CLI nor docker is installed")
	ui.Step("install: https://supabase.com/docs/guides/cli")
	return nil
}

func runPassthrough(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}
