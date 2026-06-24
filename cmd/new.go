package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/scaffold"
	"github.com/togo-framework/cli/internal/toolchain"
	"github.com/togo-framework/cli/internal/ui"
)

// featureProviders maps a feature name to the provider plugin package that
// implements it. Selected features are blank-imported into internal/plugins so
// they self-register with the kernel (same mechanism as `togo install`).
var featureProviders = map[string]string{
	"cache":    "github.com/togo-framework/cache",
	"queue":    "github.com/togo-framework/queue",
	"storage":  "github.com/togo-framework/storage",
	"realtime": "github.com/togo-framework/realtime",
	"i18n":     "github.com/togo-framework/i18n",
}

// dbProviders maps a database stack to its driver PLUGIN package. The driver is a
// plugin (blank-imported into internal/plugins, same as features) — never baked
// into the app's go.mod/app.go. sqlite is built into the kernel, so no plugin.
// Postgres-wire-compatible stacks (supabase, togo-postgres) reuse db-postgres.
var dbProviders = map[string]string{
	"postgres":      "github.com/togo-framework/db-postgres",
	"togo-postgres": "github.com/togo-framework/db-postgres",
	"supabase":      "github.com/togo-framework/db-postgres",
	"mysql":         "github.com/togo-framework/db-mysql",
	"mongodb":       "github.com/togo-framework/db-mongodb",
}

func registerNew(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "new <app>",
		Short:   "Scaffold a new togo project (pick the features/providers you need)",
		GroupID: groupProject,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			module, _ := cmd.Flags().GetString("module")
			dir, _ := cmd.Flags().GetString("dir")
			// The arg may be a bare name or a path (`togo new /tmp/myapp`). The app
			// name (and Go module) is the basename; the target directory is the arg
			// unless --dir is given. Fixes the malformed module github.com//tmp/myapp//tmp/myapp.
			appName := filepath.Base(filepath.Clean(name))
			if dir == "" {
				dir = name
			}
			force, _ := cmd.Flags().GetBool("force")
			dry, _ := cmd.Flags().GetBool("dry-run")

			// Ensure Go + Node are present (install if missing) — scaffolding runs
			// `go mod tidy` and the frontend needs npm. Skipped on dry-run.
			if !dry {
				if e := toolchain.EnsureGo(); e != nil {
					ui.Warn("Go: %v", e)
				}
				if e := toolchain.EnsureNode(); e != nil {
					ui.Warn("Node: %v", e)
				}
			}

			frontend, err := resolveFrontend(cmd)
			if err != nil {
				return err
			}
			database, err := resolveDatabase(cmd)
			if err != nil {
				return err
			}
			selected := resolveSelection(cmd)

			opts := scaffold.Options{App: appName, Module: module, Dir: dir, Force: force, DryRun: dry, Frontend: frontend, DB: database}
			// Refuse to scaffold over a non-empty directory: overlaying onto leftover
			// files (e.g. an incomplete `rm`) silently skips them and produces a broken
			// mix of old + new code. --force overrides.
			if !force && !dry {
				if target := opts.Resolve().Dir; dirNotEmpty(target) {
					return fmt.Errorf("directory %q already exists and is not empty — remove it (rm -rf %s) or use --force", target, target)
				}
			}
			created, err := scaffold.New(opts)
			if err != nil {
				return err
			}
			target := opts.Resolve().Dir
			if dry {
				ui.Warn("dry-run: would scaffold %d files into %s (features: %s)", created, target, strings.Join(selected, ", "))
				return nil
			}
			ui.Success("Created togo project %q (%d files)", name, created)
			ui.Step("frontend: %s", frontend)
			ui.Step("database: %s", database)

			// Register feature providers (cache/queue/…) via blank-imports.
			for _, f := range selected {
				if pkg, ok := featureProviders[f]; ok {
					if err := addPluginImport(target, pkg); err != nil {
						ui.Warn("enable %s: %v", f, err)
					}
				}
			}

			// Install the database driver PLUGIN for the chosen stack (sqlite is
			// built into the kernel — no plugin). The driver lives in the plugin,
			// not the app's go.mod/app.go.
			if pkg := dbProviders[database]; pkg != "" {
				if err := addPluginImport(target, pkg); err != nil {
					ui.Warn("enable db %s: %v", database, err)
				}
			}

			// Install full plugins (auth backend + dev login + dashboard UI/layouts)
			// so the app ships login/register/dashboard/admin out of the box.
			if proj, err := config.Load(filepath.Join(target, "togo.yaml")); err == nil {
				var installs []string
				if contains(selected, "auth") {
					installs = append(installs, "auth", "auth-dev") // auth-dev is dev-only (no-op in prod)
				}
				// The dashboard plugin injects a Next.js web app; only install it for the
				// Next frontend. The TanStack template already ships its own dashboard.
				if contains(selected, "dashboard") && frontend == "nextjs" {
					installs = append(installs, "dashboard")
				}
				for _, p := range installs {
					if err := installPlugin(proj, "togo-framework/"+p); err != nil {
						ui.Warn("install %s: %v", p, err)
					}
				}
			}
			if len(selected) > 0 {
				ui.Step("included: %s", strings.Join(selected, ", "))
			}

			// Resolve Go modules so the project is runnable immediately.
			if skip, _ := cmd.Flags().GetBool("skip-tidy"); !skip {
				if err := goModTidy(target); err != nil {
					ui.Warn("go mod tidy failed: %v (run it manually in %s)", err, target)
				}
			}

			// Provision the DB stack for the docker-backed databases ("up it for him").
			provisionDatabase(database, target, name)

			ui.Step("cd %s", name)
			ui.Step("togo make:resource Post title:string body:text:nullable")
			ui.Step("togo generate && togo migrate && togo serve")
			return nil
		},
	}
	cmd.Flags().String("module", "", "Go module path (default: github.com/<app>/<app>)")
	cmd.Flags().String("dir", "", "target directory (default: ./<app>)")
	cmd.Flags().String("features", "", "comma-separated features (cache,queue,storage,realtime,i18n); default: all")
	cmd.Flags().String("frontend", "tanstack", "web frontend: tanstack (default) | nextjs")
	cmd.Flags().String("db", "sqlite", "database stack: sqlite (default) | postgres | togo-postgres | supabase | mysql | mongodb")
	cmd.Flags().Bool("skip-tidy", false, "do not run `go mod tidy` after scaffolding")
	root.AddCommand(cmd)
}

// selectable lists everything `togo new` can include: feature providers + the
// auth backend + the dashboard UI (login/register/dashboard/admin + layouts).
func selectable() []ui.Option {
	return []ui.Option{
		{Value: "cache", Label: "Cache", Hint: "in-memory/file/db/redis", Default: true},
		{Value: "queue", Label: "Queue", Hint: "background jobs", Default: true},
		{Value: "storage", Label: "Storage", Hint: "files/blobs", Default: true},
		{Value: "realtime", Label: "Realtime", Hint: "SSE/WebSocket events", Default: true},
		{Value: "i18n", Label: "i18n", Hint: "translations", Default: true},
		{Value: "auth", Label: "Auth", Hint: "JWT, RBAC, OTP/2FA, sessions", Default: true},
		{Value: "dashboard", Label: "Dashboard + Admin UI", Hint: "login/register/admin (needs auth)", Default: true},
	}
}

func allSelectable() []string {
	opts := selectable()
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Value
	}
	return out
}

// resolveSelection returns the chosen features + plugins. Precedence: explicit
// --features flag → interactive multi-select → all (non-interactive default).
func resolveSelection(cmd *cobra.Command) []string {
	if cmd.Flags().Changed("features") {
		return parseFeatures(mustString(cmd, "features"))
	}
	if isInteractive() {
		sel := ui.MultiSelect("Select features & plugins for your app", selectable())
		// dashboard implies auth.
		if contains(sel, "dashboard") && !contains(sel, "auth") {
			sel = append(sel, "auth")
		}
		return sel
	}
	return allSelectable()
}

// frontendOptions lists the web frontends `togo new` can scaffold.
func frontendOptions() []ui.Option {
	return []ui.Option{
		{Value: "tanstack", Label: "TanStack (React + Vite)", Hint: "default · SPA, same-origin API", Default: true},
		{Value: "nextjs", Label: "Next.js (App Router)", Hint: "SSR/SSG", Default: false},
	}
}

// resolveFrontend picks the web frontend. Precedence: explicit --frontend flag →
// interactive single-select → tanstack (non-interactive default).
func resolveFrontend(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("frontend") {
		f := mustString(cmd, "frontend")
		if f != "tanstack" && f != "nextjs" {
			return "", fmt.Errorf("invalid --frontend %q (allowed: tanstack, nextjs)", f)
		}
		return f, nil
	}
	if isInteractive() {
		return ui.Select("Choose a frontend stack", frontendOptions()), nil
	}
	return "tanstack", nil
}

// dbStacks is the set of databases `togo new` can wire (driver + DSN + compose).
var dbStacks = []string{"sqlite", "postgres", "togo-postgres", "supabase", "mysql", "mongodb"}

// dbOptions lists the database stacks for the interactive picker.
func dbOptions() []ui.Option {
	return []ui.Option{
		{Value: "sqlite", Label: "SQLite", Hint: "default · embedded file DB, no docker", Default: true},
		{Value: "postgres", Label: "PostgreSQL", Hint: "postgres:16 via docker", Default: false},
		{Value: "togo-postgres", Label: "togo-postgres", Hint: "batteries-included Supabase build — pgvector/pg_search/duckdb/cron/River/NATS", Default: false},
		{Value: "supabase", Label: "Supabase", Hint: "postgres + GoTrue + Storage + Studio (docker)", Default: false},
		{Value: "mysql", Label: "MySQL", Hint: "mysql:8 via docker", Default: false},
		{Value: "mongodb", Label: "MongoDB", Hint: "mongo (app-level client; not the SQL ORM)", Default: false},
	}
}

// resolveDatabase picks the database stack. Precedence: explicit --db flag →
// interactive single-select → sqlite (non-interactive default).
func resolveDatabase(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("db") {
		v := mustString(cmd, "db")
		if !contains(dbStacks, v) {
			return "", fmt.Errorf("invalid --db %q (allowed: %s)", v, strings.Join(dbStacks, ", "))
		}
		return v, nil
	}
	if isInteractive() {
		return ui.Select("Choose a database stack", dbOptions()), nil
	}
	return "sqlite", nil
}

// provisionDatabase brings up the docker stack for docker-backed databases.
// sqlite needs nothing. For the rest it prints the up command and, when docker is
// present and stdin is a TTY, offers to run `docker compose up -d` now.
func provisionDatabase(database, target, name string) {
	if database == "sqlite" || database == "" {
		return
	}
	ui.Step("cd %s && docker compose up -d   # start the %s stack", name, database)
	if !isInteractive() || !has("docker") {
		return
	}
	if !ui.Confirm(fmt.Sprintf("Start the %s docker stack now?", database), true) {
		return
	}
	c := exec.Command("docker", "compose", "up", "-d")
	c.Dir = target
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		ui.Warn("docker compose up failed: %v (run it manually in %s)", err, name)
		return
	}
	ui.Success("%s stack is up (docker compose)", database)
}

func parseFeatures(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "all" {
		return allSelectable()
	}
	if raw == "none" {
		return nil
	}
	valid := map[string]bool{}
	for _, v := range allSelectable() {
		valid[v] = true
	}
	var out []string
	for _, f := range strings.Split(raw, ",") {
		f = strings.TrimSpace(f)
		if valid[f] {
			out = append(out, f)
		}
	}
	if contains(out, "dashboard") && !contains(out, "auth") {
		out = append(out, "auth")
	}
	return out
}

func mustString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

// dirNotEmpty reports whether path exists and contains entries.
func dirNotEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

// isInteractive reports whether stdin is a terminal (so prompting won't block CI).
func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}
