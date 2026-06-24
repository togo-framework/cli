package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

const proxyLong = `Manage DNS records, reverse-proxy hosts, and API-gateway routes — provider-routed.

togo proxy drives the togo-framework/dns plugin and its dns-<provider> driver
(cloudflare, npm, caddy, kong). The provider is chosen from --provider, then
dns.provider / proxy.provider in togo.yaml, then the DNS_DRIVER env. Install the
driver you use first:

  togo install togo-framework/dns-cloudflare

  # togo.yaml:
  dns:
    provider: cloudflare   # or npm | caddy | kong
    zone: example.com

Like ` + "`togo deploy`" + `, the op runs through a tiny generated runner under
.togo/proxy/runner that blank-imports dns-<provider> and calls the plugin's API,
so the CLI never has to import every provider. Pass --dry-run to print the plan.

Examples:
  togo proxy:record:add www A 203.0.113.7          # DNS record (cloudflare)
  togo proxy:record:list
  togo proxy:host:add app.example.com http://127.0.0.1:8080 --tls   # reverse proxy (npm/caddy)
  togo proxy:route:add api.example.com http://svc:9000             # gateway route (kong/caddy)
  togo proxy:host:add app.example.com http://127.0.0.1:8080 -p caddy --dry-run`

// proxyFlags pulls the shared --provider/--zone/--dry-run flags off a command.
func proxyFlags(cmd *cobra.Command, proj *config.Project) (provider, zone string, dryRun bool) {
	pf, _ := cmd.Flags().GetString("provider")
	zf, _ := cmd.Flags().GetString("zone")
	dryRun, _ = cmd.Flags().GetBool("dry-run")
	return proxyProvider(proj, pf), proxyZone(proj, zf), dryRun
}

func registerProxy(root *cobra.Command) {
	proxy := &cobra.Command{
		Use:     "proxy",
		Short:   "Manage DNS records / proxy hosts / gateway routes via the dns plugin",
		Long:    proxyLong,
		GroupID: groupInfra,
		RunE:    func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	proxy.PersistentFlags().StringP("provider", "p", "", "dns provider override (cloudflare, npm, caddy, kong); else dns.provider / DNS_DRIVER")
	proxy.PersistentFlags().String("zone", "", "DNS zone for record ops (default: dns.zone in togo.yaml)")
	proxy.PersistentFlags().Bool("dry-run", false, "print the planned op without changing anything")

	// --- DNS records ---
	recordAdd := &cobra.Command{
		Use:     "proxy:record:add <name> <type> <value>",
		Short:   "Create/update a DNS record (e.g. www A 203.0.113.7)",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider, zone, dry := proxyFlags(cmd, proj)
			ttl, _ := cmd.Flags().GetInt("ttl")
			proxied, _ := cmd.Flags().GetBool("proxied")
			prio, _ := cmd.Flags().GetInt("prio")
			name, typ, value := args[0], strings.ToUpper(args[1]), args[2]
			plan := fmtPlan("record", typ+" "+name+" → "+value+"  zone="+nz(zone))
			return runProxyOp(proj, provider, plan, proxyReq{
				Op:     "record.upsert",
				Zone:   zone,
				Record: map[string]any{"Type": typ, "Name": name, "Content": value, "TTL": ttl, "Proxied": proxied, "Prio": prio},
			}, dry)
		},
	}
	recordAdd.Flags().Int("ttl", 1, "record TTL in seconds (1 = automatic)")
	recordAdd.Flags().Bool("proxied", false, "enable provider-side proxy/CDN (Cloudflare orange-cloud)")
	recordAdd.Flags().Int("prio", 0, "priority (MX/SRV records)")

	recordList := &cobra.Command{
		Use:     "proxy:record:list",
		Short:   "List DNS records in the zone",
		GroupID: groupInfra,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider, zone, dry := proxyFlags(cmd, proj)
			return runProxyOp(proj, provider, fmtPlan("record", "list  zone="+nz(zone)), proxyReq{Op: "record.list", Zone: zone}, dry)
		},
	}

	recordRm := &cobra.Command{
		Use:     "proxy:record:rm <name> <type>",
		Short:   "Delete a DNS record by name + type",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider, zone, dry := proxyFlags(cmd, proj)
			id, _ := cmd.Flags().GetString("id")
			name, typ := args[0], strings.ToUpper(args[1])
			return runProxyOp(proj, provider, fmtPlan("record", "delete "+typ+" "+name+"  zone="+nz(zone)), proxyReq{
				Op:     "record.delete",
				Zone:   zone,
				Record: map[string]any{"Name": name, "Type": typ},
				ID:     id,
			}, dry)
		},
	}
	recordRm.Flags().String("id", "", "delete by provider record id instead of resolving name+type")

	// --- reverse-proxy hosts ---
	hostAdd := &cobra.Command{
		Use:     "proxy:host:add <domain> <upstream>",
		Short:   "Create/update a reverse-proxy host (npm, caddy)",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider, _, dry := proxyFlags(cmd, proj)
			tls, _ := cmd.Flags().GetBool("tls")
			domain, upstream := args[0], args[1]
			return runProxyOp(proj, provider, fmtPlan("host", domain+" → "+upstream+"  tls="+yn(tls)), proxyReq{
				Op:   "host.upsert",
				Host: map[string]any{"Domain": domain, "Upstream": upstream, "SSL": tls},
			}, dry)
		},
	}
	hostAdd.Flags().Bool("tls", false, "request/force TLS (Let's Encrypt where supported)")

	hostRm := &cobra.Command{
		Use:     "proxy:host:rm <domain|id>",
		Short:   "Delete a reverse-proxy host",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider, _, dry := proxyFlags(cmd, proj)
			return runProxyOp(proj, provider, fmtPlan("host", "delete "+args[0]), proxyReq{Op: "host.delete", ID: args[0]}, dry)
		},
	}

	// --- API-gateway routes ---
	routeAdd := &cobra.Command{
		Use:     "proxy:route:add <host> <upstream>",
		Short:   "Create/update a gateway route (kong, caddy)",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider, _, dry := proxyFlags(cmd, proj)
			path, _ := cmd.Flags().GetString("path")
			host, upstream := args[0], args[1]
			return runProxyOp(proj, provider, fmtPlan("route", host+path+" → "+upstream), proxyReq{
				Op:    "route.upsert",
				Route: map[string]any{"Domain": host, "Path": path, "Upstream": upstream},
			}, dry)
		},
	}
	routeAdd.Flags().String("path", "/", "path prefix to match")

	routeRm := &cobra.Command{
		Use:     "proxy:route:rm <host|id>",
		Short:   "Delete a gateway route",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider, _, dry := proxyFlags(cmd, proj)
			return runProxyOp(proj, provider, fmtPlan("route", "delete "+args[0]), proxyReq{Op: "route.delete", ID: args[0]}, dry)
		},
	}

	// host/route listing isn't part of the dns Provider interface (only records).
	listNote := func(kind string) *cobra.Command {
		return &cobra.Command{
			Use:     "proxy:" + kind + ":list",
			Short:   "List " + kind + "s (not exposed by the dns provider interface yet)",
			GroupID: groupInfra,
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				ui.Info("Listing %ss isn't exposed by the dns provider interface yet — only DNS records support listing.", kind)
				ui.Step("Use `togo proxy:record:list` for DNS records, or manage %ss in your provider's dashboard.", kind)
				return nil
			},
		}
	}

	// The colon commands are top-level siblings (not children of `proxy`), so they
	// don't inherit the parent's persistent flags — give each its own --provider/--zone.
	// (--dry-run is a root persistent flag, inherited everywhere.)
	for _, c := range []*cobra.Command{recordAdd, recordList, recordRm, hostAdd, hostRm, routeAdd, routeRm} {
		c.Flags().StringP("provider", "p", "", "dns provider override (cloudflare, npm, caddy, kong); else dns.provider / DNS_DRIVER")
		c.Flags().String("zone", "", "DNS zone for record ops (default: dns.zone in togo.yaml)")
	}

	root.AddCommand(proxy, recordAdd, recordList, recordRm, hostAdd, hostRm, listNote("host"), routeAdd, routeRm, listNote("route"))
}

// fmtPlan formats the dry-run plan body line.
func fmtPlan(kind, detail string) string { return kind + "    : " + detail }

func yn(b bool) string {
	if b {
		return "on"
	}
	return "off"
}
