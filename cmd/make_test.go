package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

// End-to-end regression for MH-475/MH-476: `togo make:resource Customer ...
// --force` against a real manifest must keep enums, relations, controller:false
// and the resource order, keep hand-managed resources out of the seeder
// registry, and emit snake_case TS property names.
func TestMakeResourcePreservesManifest(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "internal", "config", "testdata", "wasla.resources.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	mustWrite(t, filepath.Join(dir, config.ManifestFile), src)
	mustWrite(t, filepath.Join(dir, config.ConfigFile), []byte("name: app\nmodule: example.com/app\n"))
	before, err := config.ParseManifest(src)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		rootCmd.SetArgs(nil)
		_ = rootCmd.PersistentFlags().Set("force", "false")
		_ = rootCmd.PersistentFlags().Set("config", "")
	}()
	rootCmd.SetArgs([]string{"make:resource", "Customer", "phone:string", "name:string",
		"default_address:text:nullable", "--force", "--config", filepath.Join(dir, config.ConfigFile)})
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out, err := os.ReadFile(filepath.Join(dir, config.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	after, err := config.ParseManifest(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Resources) != len(before.Resources) {
		t.Fatalf("resource count %d, want %d", len(after.Resources), len(before.Resources))
	}
	for i, r := range before.Resources {
		if after.Resources[i].Name != r.Name {
			t.Fatalf("resource %d is %s, want %s (order changed)", i, after.Resources[i].Name, r.Name)
		}
	}
	for _, key := range []string{"enum:", "relation:", "controller: false"} {
		if got, want := strings.Count(string(out), key), strings.Count(string(src), key); got != want {
			t.Errorf("%q occurrences: got %d, want %d", key, got, want)
		}
	}

	seeders, _ := os.ReadFile(filepath.Join(dir, "internal", "db", "seeders", "registry.gen.go"))
	for _, name := range []string{"DispatchOffer", "LedgerAccount", "LedgerTransaction", "LedgerEntry"} {
		if strings.Contains(string(seeders), "Seed"+name+"(") {
			t.Errorf("seeder registry wires controller:false resource %s", name)
		}
	}
	if !strings.Contains(string(seeders), "SeedCustomer(") {
		t.Error("seeder registry is missing SeedCustomer")
	}

	ts, _ := os.ReadFile(filepath.Join(dir, "web", "lib", "api", "customer.ts"))
	if !strings.Contains(string(ts), "default_address?: string;") || strings.Contains(string(ts), "defaultAddress") {
		t.Errorf("customer.ts should use the snake_case JSON name:\n%s", ts)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
