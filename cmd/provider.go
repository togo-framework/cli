package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// capabilities mirrors github.com/togo-framework/providers — the selectable slots.
// A backend is chosen with `togo provider:use <cap> <name>`, which writes
// TOGO_<CAP>_PROVIDER to .env (the env tier that providers.Active() reads first).
var capabilities = []string{"impl", "exec", "compute", "data", "catalog", "share", "queue", "cache", "storage", "realtime"}

// knownEnv documents the config keys a few providers expect, so `provider:test`
// can flag missing values. Extend as providers are added.
var knownEnv = map[string][]string{
	"coder":      {"CODER_URL", "CODER_TOKEN"},
	"omnigent":   {"OMNIGENT_API"},
	"bigquery":   {"BIGQUERY_PROJECT", "GOOGLE_APPLICATION_CREDENTIALS"},
	"databricks": {"DATABRICKS_HOST", "DATABRICKS_TOKEN"},
	"kafka":      {"KAFKA_BROKERS"},
	"rabbitmq":   {"RABBITMQ_URL"},
	"nats":       {"NATS_URL"},
}

func registerProvider(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "provider:list [capability]",
		Short:   "Show capabilities and the selected backend for each",
		GroupID: groupPlugin,
		RunE: func(cmd *cobra.Command, args []string) error {
			envm := dotenvMap(projectRoot())
			caps := capabilities
			if len(args) == 1 {
				caps = []string{args[0]}
			}
			for _, c := range caps {
				sel := envm["TOGO_"+strings.ToUpper(c)+"_PROVIDER"]
				if sel == "" {
					sel = "(default)"
				}
				fmt.Printf("  %-9s → %s\n", c, sel)
			}
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "provider:use <capability> <backend>",
		Short:   "Select the active backend for a capability (writes .env)",
		GroupID: groupPlugin,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			capability, name := args[0], args[1]
			key := "TOGO_" + strings.ToUpper(capability) + "_PROVIDER"
			if err := dotenvSet(projectRoot(), key, name); err != nil {
				return err
			}
			fmt.Printf("✓ %s → %s  (%s in .env)\n", capability, name, key)
			if want := knownEnv[name]; len(want) > 0 {
				fmt.Printf("  next: set %s in .env (or `togo config:set`)\n", strings.Join(want, ", "))
			}
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "provider:test [backend]",
		Short:   "Validate provider config (selected backends, or one named)",
		GroupID: groupPlugin,
		RunE: func(cmd *cobra.Command, args []string) error {
			envm := dotenvMap(projectRoot())
			var names []string
			if len(args) == 1 {
				names = []string{args[0]}
			} else {
				for _, c := range capabilities {
					if v := envm["TOGO_"+strings.ToUpper(c)+"_PROVIDER"]; v != "" {
						names = append(names, v)
					}
				}
			}
			if len(names) == 0 {
				fmt.Println("no non-default backends selected — nothing to test")
				return nil
			}
			bad := 0
			for _, n := range names {
				var missing []string
				for _, k := range knownEnv[n] {
					if envm[k] == "" && os.Getenv(k) == "" {
						missing = append(missing, k)
					}
				}
				if len(missing) == 0 {
					fmt.Printf("  ✓ %s — config present\n", n)
				} else {
					bad++
					fmt.Printf("  ✗ %s — missing: %s\n", n, strings.Join(missing, ", "))
				}
			}
			if bad > 0 {
				return fmt.Errorf("%d provider(s) missing config", bad)
			}
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "config:get <KEY>",
		Short:   "Print a project config value from .env",
		GroupID: groupProject,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(dotenvValue(projectRoot(), args[0]))
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:     "config:set <KEY> <VALUE>",
		Short:   "Set a project config value in .env",
		GroupID: groupProject,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := dotenvSet(projectRoot(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Printf("✓ %s set\n", args[0])
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:     "config:list",
		Short:   "List project config keys (secret values masked)",
		GroupID: groupProject,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := dotenvMap(projectRoot())
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := m[k]
				if isSecretKey(k) {
					v = "••••••"
				}
				fmt.Printf("  %s=%s\n", k, v)
			}
			return nil
		},
	})
}

func isSecretKey(k string) bool {
	up := strings.ToUpper(k)
	for _, s := range []string{"TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "DSN"} {
		if strings.Contains(up, s) {
			return true
		}
	}
	return false
}

// projectRoot walks up from the cwd to the directory holding togo.yaml, else cwd.
func projectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "togo.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return start
		}
		dir = parent
	}
}

// dotenvSet upserts key=value in <root>/.env, preserving other lines.
func dotenvSet(root, key, value string) error {
	path := filepath.Join(root, ".env")
	var lines []string
	found := false
	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			l := sc.Text()
			if strings.HasPrefix(strings.TrimSpace(l), key+"=") {
				lines = append(lines, key+"="+value)
				found = true
			} else {
				lines = append(lines, l)
			}
		}
		f.Close()
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}
