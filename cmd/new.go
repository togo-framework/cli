package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/scaffold"
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
			force, _ := cmd.Flags().GetBool("force")
			dry, _ := cmd.Flags().GetBool("dry-run")

			selected := resolveSelection(cmd)

			opts := scaffold.Options{App: name, Module: module, Dir: dir, Force: force, DryRun: dry}
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

			// Register feature providers (cache/queue/…) via blank-imports.
			for _, f := range selected {
				if pkg, ok := featureProviders[f]; ok {
					if err := addPluginImport(target, pkg); err != nil {
						ui.Warn("enable %s: %v", f, err)
					}
				}
			}

			// Install full plugins (auth backend + dev login + dashboard UI/layouts)
			// so the app ships login/register/dashboard/admin out of the box.
			if proj, err := config.Load(filepath.Join(target, "togo.yaml")); err == nil {
				var installs []string
				if contains(selected, "auth") {
					installs = append(installs, "auth", "auth-dev") // auth-dev is dev-only (no-op in prod)
				}
				if contains(selected, "dashboard") {
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

			ui.Step("cd %s", name)
			ui.Step("togo make:resource Post title:string body:text:nullable")
			ui.Step("togo generate && togo migrate && togo serve")
			return nil
		},
	}
	cmd.Flags().String("module", "", "Go module path (default: github.com/<app>/<app>)")
	cmd.Flags().String("dir", "", "target directory (default: ./<app>)")
	cmd.Flags().String("features", "", "comma-separated features (cache,queue,storage,realtime,i18n); default: all")
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
