package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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

var allFeatures = []string{"cache", "queue", "storage", "realtime", "i18n"}

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

			features := resolveFeatures(cmd)

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
				ui.Warn("dry-run: would scaffold %d files into %s (features: %s)", created, target, strings.Join(features, ", "))
				return nil
			}
			ui.Success("Created togo project %q (%d files)", name, created)

			// Register the chosen feature providers via the plugin mechanism.
			for _, f := range features {
				if pkg, ok := featureProviders[f]; ok {
					if err := addPluginImport(target, pkg); err != nil {
						ui.Warn("enable %s: %v", f, err)
					}
				}
			}
			if len(features) > 0 {
				ui.Step("features: %s", strings.Join(features, ", "))
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

// resolveFeatures returns the selected features. Precedence: explicit --features
// flag → interactive prompt (when attached to a terminal) → all (default, e.g. CI).
func resolveFeatures(cmd *cobra.Command) []string {
	if cmd.Flags().Changed("features") {
		raw, _ := cmd.Flags().GetString("features")
		return parseFeatures(raw)
	}
	if isInteractive() {
		return promptFeatures()
	}
	return allFeatures
}

func parseFeatures(raw string) []string {
	if strings.TrimSpace(raw) == "" || raw == "all" {
		return allFeatures
	}
	if strings.TrimSpace(raw) == "none" {
		return nil
	}
	var out []string
	for _, f := range strings.Split(raw, ",") {
		f = strings.TrimSpace(f)
		if _, ok := featureProviders[f]; ok {
			out = append(out, f)
		}
	}
	return out
}

// promptFeatures asks which feature plugins to include (Enter = all).
func promptFeatures() []string {
	ui.Info("Which feature plugins do you want? (each is a togo-framework provider)")
	ui.Step("available: %s", strings.Join(allFeatures, ", "))
	ui.Step("comma-separated list, 'none', or Enter for all")
	fmt.Print("  features [all]: ")
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return allFeatures
	}
	line := strings.TrimSpace(sc.Text())
	if line == "" {
		return allFeatures
	}
	return parseFeatures(line)
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
