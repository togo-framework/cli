package cmd

import (
	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
)

const tunnelLong = `Expose the local togo app to the public internet — provider-routed.

togo tunnel drives the togo-framework/tunnel plugin and its tunnel-<provider>
driver (cloudflare, ngrok, tailscale, frp). The provider is chosen from
--provider, then tunnel.provider in togo.yaml, then the TUNNEL_DRIVER env.
Install the driver you use first:

  togo install togo-framework/tunnel-ngrok

  # togo.yaml:
  tunnel:
    provider: cloudflare   # or ngrok | tailscale | frp
    addr: localhost:8080

Like ` + "`togo proxy`" + ` and ` + "`togo deploy`" + `, the tunnel runs through a tiny
generated runner under .togo/tunnel/runner that blank-imports tunnel-<provider>
and calls the plugin's API, so the CLI never has to import every provider.
` + "`togo tunnel:start`" + ` blocks and prints the public URL — press Ctrl-C to stop.
Pass --dry-run to print the plan without opening anything.

Examples:
  togo tunnel                                  # start (most common) using the configured provider
  togo tunnel:start --addr localhost:3000      # tunnel a specific local address
  togo tunnel:start -p ngrok                   # override the provider
  togo tunnel:status
  togo tunnel:start -p cloudflare --dry-run`

func registerTunnel(root *cobra.Command) {
	// startRun is shared by the bare `togo tunnel` and `togo tunnel:start`.
	startRun := func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject(cmd)
		if err != nil {
			return err
		}
		provider := tunnelProvider(proj, flagStr(cmd, "provider"))
		addr := tunnelAddr(proj, flagStr(cmd, "addr"))
		dry, _ := cmd.Flags().GetBool("dry-run")
		return runTunnelOp(proj, provider, tunnelReq{Op: "start", Addr: addr}, dry)
	}

	tunnel := &cobra.Command{
		Use:     "tunnel",
		Short:   "Expose the local app publicly via the tunnel plugin (start)",
		Long:    tunnelLong,
		GroupID: groupInfra,
		RunE:    startRun, // bare `togo tunnel` = start
	}
	tunnel.Flags().StringP("provider", "p", "", "tunnel provider override (cloudflare, ngrok, tailscale, frp); else tunnel.provider / TUNNEL_DRIVER")
	tunnel.Flags().String("addr", "", "local address to tunnel (default: tunnel.addr in togo.yaml, else localhost:8080)")
	tunnel.Flags().Bool("dry-run", false, "print the plan without opening a tunnel")

	start := &cobra.Command{
		Use:     "tunnel:start",
		Short:   "Open a public tunnel to the local app (blocks; Ctrl-C to stop)",
		GroupID: groupInfra,
		Args:    cobra.NoArgs,
		RunE:    startRun,
	}
	start.Flags().StringP("provider", "p", "", "tunnel provider override (cloudflare, ngrok, tailscale, frp); else tunnel.provider / TUNNEL_DRIVER")
	start.Flags().String("addr", "", "local address to tunnel (default: tunnel.addr in togo.yaml, else localhost:8080)")

	status := &cobra.Command{
		Use:     "tunnel:status",
		Short:   "Report the tunnel status for the configured provider",
		GroupID: groupInfra,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider := tunnelProvider(proj, flagStr(cmd, "provider"))
			dry, _ := cmd.Flags().GetBool("dry-run")
			return runTunnelOp(proj, provider, tunnelReq{Op: "status"}, dry)
		},
	}
	status.Flags().StringP("provider", "p", "", "tunnel provider override; else tunnel.provider / TUNNEL_DRIVER")

	stop := &cobra.Command{
		Use:     "tunnel:stop",
		Short:   "Stop a tunnel managed by this provider (where supported)",
		GroupID: groupInfra,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider := tunnelProvider(proj, flagStr(cmd, "provider"))
			dry, _ := cmd.Flags().GetBool("dry-run")
			return runTunnelOp(proj, provider, tunnelReq{Op: "stop"}, dry)
		},
	}
	stop.Flags().StringP("provider", "p", "", "tunnel provider override; else tunnel.provider / TUNNEL_DRIVER")

	// --dry-run is a root persistent flag, inherited by the colon siblings.
	root.AddCommand(tunnel, start, status, stop)
}

// flagStr reads a string flag, tolerating its absence.
func flagStr(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

// tunnelAddr resolves the local address to tunnel: --addr > tunnel.addr > :8080.
func tunnelAddr(proj *config.Project, override string) string {
	if override != "" {
		return override
	}
	if proj.Tunnel.Addr != "" {
		return proj.Tunnel.Addr
	}
	return "localhost:8080"
}
