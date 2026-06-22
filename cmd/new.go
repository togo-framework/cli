package cmd

import (
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

// resolveFeatures returns the selected features from --features, or all by default.
func resolveFeatures(cmd *cobra.Command) []string {
	raw, _ := cmd.Flags().GetString("features")
	if strings.TrimSpace(raw) == "" || raw == "all" {
		return allFeatures
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
