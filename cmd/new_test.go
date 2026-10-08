package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/togo-framework/cli/internal/config"
)

func TestRequiredPlugins(t *testing.T) {
	cases := []struct {
		name     string
		selected []string
		frontend string
		want     []string
	}{
		{"nextjs auth+dashboard", []string{"cache", "auth", "dashboard"}, "nextjs", []string{"auth", "auth-dev", "dashboard"}},
		{"tanstack ships its own dashboard", []string{"auth", "dashboard"}, "tanstack", []string{"auth", "auth-dev"}},
		{"no plugins selected", []string{"cache", "queue"}, "nextjs", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requiredPlugins(c.selected, c.frontend); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("requiredPlugins(%v, %q) = %v, want %v", c.selected, c.frontend, got, c.want)
			}
		})
	}
}

// Every required plugin is attempted and every failure is reported, not just
// the first one.
func TestInstallRequiredReportsEveryFailure(t *testing.T) {
	var attempted []string
	stubInstall(t, func(_ *config.Project, repo string, _ bool) error {
		attempted = append(attempted, repo)
		if repo == "togo-framework/auth-dev" || repo == "togo-framework/dashboard" {
			return errors.New("go get failed")
		}
		return nil
	})

	err := installRequired(&config.Project{}, []string{"auth", "auth-dev", "dashboard"}, false)
	if err == nil {
		t.Fatal("installRequired returned nil, want an error")
	}
	want := []string{"togo-framework/auth", "togo-framework/auth-dev", "togo-framework/dashboard"}
	if !reflect.DeepEqual(attempted, want) {
		t.Fatalf("attempted %v, want %v", attempted, want)
	}
	for _, p := range []string{"auth-dev", "dashboard"} {
		if !strings.Contains(err.Error(), "install required plugin "+p) {
			t.Errorf("error %q does not name failed plugin %s", err, p)
		}
	}
	if strings.Contains(err.Error(), "plugin auth:") {
		t.Errorf("error %q names auth, which installed fine", err)
	}
}

func TestInstallRequiredSucceeds(t *testing.T) {
	stubInstall(t, func(*config.Project, string, bool) error { return nil })
	if err := installRequired(&config.Project{}, []string{"auth", "auth-dev"}, false); err != nil {
		t.Fatalf("installRequired: %v", err)
	}
}

// `togo new` must exit non-zero (Execute returns an error, which main turns
// into exit status 1) when a required plugin fails to install, and succeed
// when every install succeeds.
func TestNewExitStatusFollowsRequiredInstalls(t *testing.T) {
	cases := []struct {
		name    string
		install func(*config.Project, string, bool) error
		wantErr bool
	}{
		{"install fails", func(_ *config.Project, repo string, _ bool) error {
			if repo == "togo-framework/dashboard" {
				return errors.New("go get github.com/togo-framework/dashboard@latest: exit status 1")
			}
			return nil
		}, true},
		{"install succeeds", func(*config.Project, string, bool) error { return nil }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var installed []string
			stubInstall(t, func(p *config.Project, repo string, force bool) error {
				installed = append(installed, repo)
				return c.install(p, repo, force)
			})
			target := filepath.Join(t.TempDir(), "acc")
			err := runNew(t, target, "--module", "example.com/acc", "--frontend", "nextjs",
				"--db", "sqlite", "--features", "auth,dashboard", "--skip-tidy")

			if c.wantErr {
				if err == nil {
					t.Fatal("togo new returned nil, want a non-zero exit")
				}
				if !strings.Contains(err.Error(), "install required plugin dashboard") || !strings.Contains(err.Error(), "is incomplete") {
					t.Fatalf("unexpected error: %v", err)
				}
			} else if err != nil {
				t.Fatalf("togo new: %v", err)
			}
			if len(installed) != 3 {
				t.Fatalf("installed %v, want auth, auth-dev and dashboard", installed)
			}
			if _, statErr := os.Stat(filepath.Join(target, "togo.yaml")); statErr != nil {
				t.Fatalf("scaffold missing togo.yaml: %v", statErr)
			}
		})
	}
}

func stubInstall(t *testing.T, fn func(*config.Project, string, bool) error) {
	t.Helper()
	orig := installPluginFn
	installPluginFn = fn
	t.Cleanup(func() { installPluginFn = orig })
}

// runNew executes `togo new <target> args...` on the shared root command and
// restores the `new` flags afterwards so other tests see the defaults.
func runNew(t *testing.T, target string, args ...string) error {
	t.Helper()
	newCmd, _, err := rootCmd.Find([]string{"new"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		newCmd.Flags().VisitAll(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	})
	rootCmd.SetArgs(append([]string{"new", target}, args...))
	return rootCmd.Execute()
}
