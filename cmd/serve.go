package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

func registerServe(root *cobra.Command) {
	// serve — production-style run (no watcher by default); also used by deploy.
	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Serve the app (backend + frontend) — production-style; used by deploy",
		GroupID: groupProject,
		Long: `Serve the togo app. Runs the Go API and the Next.js frontend (use --api-only
/ --web-only for one). Set --host and --port for the API; --web-port for the
frontend. For local development with hot reload, use 'togo dev'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return serveFromFlags(cmd, false)
		},
	}
	addServeFlags(cmd)
	cmd.Flags().Bool("watch", false, "rebuild/restart the API on file changes")
	root.AddCommand(cmd)

	// dev — composer-dev style: all services, hot reload, colored prefixed logs.
	dev := &cobra.Command{
		Use:     "dev",
		Short:   "Run all services in dev mode (hot reload, colored per-service logs)",
		GroupID: groupProject,
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Info("dev mode — all services, hot reload, Ctrl-C to stop")
			return serveFromFlags(cmd, true)
		},
	}
	addServeFlags(dev)
	root.AddCommand(dev)

	// `togo web` — frontend only (convenience alias for serve --web-only).
	web := &cobra.Command{
		Use:     "web",
		Short:   "Run only the Next.js frontend (installs deps on first run)",
		GroupID: groupProject,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			webPort, _ := cmd.Flags().GetString("web-port")
			return runDev(proj, devOptions{webOnly: true, webPort: webPort})
		},
	}
	web.Flags().String("web-port", "3000", "frontend dev server port")
	root.AddCommand(web)
}

// addServeFlags adds the shared host/port/scope flags to serve and dev.
func addServeFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("api-only", false, "run only the Go API")
	cmd.Flags().Bool("web-only", false, "run only the Next.js frontend")
	cmd.Flags().String("host", "", "host/interface to bind (default all interfaces)")
	cmd.Flags().String("port", "8080", "API port")
	cmd.Flags().String("web-port", "3000", "frontend dev server port")
}

// serveFromFlags builds devOptions from flags and runs. watch defaults from the
// command (dev=on, serve=off unless --watch).
func serveFromFlags(cmd *cobra.Command, devDefault bool) error {
	proj, err := loadProject(cmd)
	if err != nil {
		return err
	}
	apiOnly, _ := cmd.Flags().GetBool("api-only")
	webOnly, _ := cmd.Flags().GetBool("web-only")
	host, _ := cmd.Flags().GetString("host")
	port, _ := cmd.Flags().GetString("port")
	webPort, _ := cmd.Flags().GetString("web-port")
	watch := devDefault
	if cmd.Flags().Changed("watch") {
		watch, _ = cmd.Flags().GetBool("watch")
	}
	return runDev(proj, devOptions{
		apiOnly: apiOnly, webOnly: webOnly, watch: watch,
		addr: host + ":" + port, host: host, webPort: webPort,
	})
}

type devOptions struct {
	apiOnly bool
	webOnly bool
	watch   bool
	addr    string
	host    string
	webPort string
}

func runDev(proj *config.Project, opts devOptions) error {
	if opts.addr == "" {
		opts.addr = ":8080"
	}
	if opts.webPort == "" {
		opts.webPort = "3000"
	}
	apiOrigin := "http://localhost" + opts.addr

	var services []service

	// Backend
	if !opts.webOnly {
		if err := ensureModules(proj.Root); err != nil {
			return err
		}
		services = append(services, service{
			name: "api",
			bin:  "go",
			args: []string{"run", "./cmd/api"},
			dir:  proj.Root,
			env:  append(os.Environ(), "ADDR="+opts.addr),
		})
	}

	// Frontend
	if !opts.apiOnly {
		webDir := filepath.Join(proj.Root, proj.Frontend.Dir)
		if fileExists(filepath.Join(webDir, "package.json")) {
			pm := detectPM(webDir)
			if err := ensureNodeModules(webDir, pm); err != nil {
				if err == errSkipWeb {
					ui.Warn("continuing without the frontend")
				} else {
					return err
				}
			} else {
				services = append(services, service{
					name: "web",
					bin:  pm.bin,
					args: pm.dev,
					dir:  webDir,
					env: append(os.Environ(),
						"PORT="+opts.webPort,
						"API_ORIGIN="+apiOrigin,
					),
				})
			}
		} else if opts.webOnly {
			return fmt.Errorf("no frontend found at %s", proj.Frontend.Dir)
		}
	}

	if len(services) == 0 {
		return fmt.Errorf("nothing to run")
	}

	printBanner(proj, opts, services)

	// Watch mode (default): restart the API on .go changes; web hot-reloads itself.
	if opts.watch && !opts.apiOnly {
		var api *service
		var web *service
		for i := range services {
			if services[i].name == "api" {
				api = &services[i]
			} else if services[i].name == "web" {
				web = &services[i]
			}
		}
		if api != nil {
			return watchAndServe(proj.Root, *api, web)
		}
	}
	if opts.watch && opts.apiOnly && len(services) == 1 {
		return watchAndServe(proj.Root, services[0], nil)
	}
	return runServices(services)
}

func printBanner(proj *config.Project, opts devOptions, services []service) {
	ui.Info("Serving %s", proj.Name)
	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, s.name)
	}
	hasAPI, hasWeb := contains(names, "api"), contains(names, "web")
	if hasAPI {
		ui.Step("API      http://localhost%s   (GraphQL %s · REST %s · Docs %s)",
			opts.addr, proj.API.GraphQL, proj.API.REST, proj.API.Docs)
	}
	if hasWeb {
		ui.Step("Web      http://localhost:%s", opts.webPort)
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
