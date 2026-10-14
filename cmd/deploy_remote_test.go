package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

func TestRemoteBuildFilters_Defaults(t *testing.T) {
	got := strings.Join(remoteBuildFilters(config.DeployTarget{}), " ")
	for _, want := range []string{
		"--exclude=.env", "--exclude=.env.*", "--exclude=.git/", "--exclude=*.db",
		"--exclude=*.db-wal", "--exclude=*.db-shm", "--exclude=*.sqlite", "--exclude=*.sqlite3",
		"--exclude=node_modules/", "--exclude=.togo/", "--include=.env.example", "--filter=:- .gitignore",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("filters missing %q: %s", want, got)
		}
	}
	// rsync uses the first matching rule: .env.example must win over .env.*.
	if strings.Index(got, "--include=.env.example") > strings.Index(got, "--exclude=.env.*") {
		t.Errorf(".env.example include must come before the .env.* exclude: %s", got)
	}
}

func TestRemoteBuildFilters_IncludeComesFirst(t *testing.T) {
	f := remoteBuildFilters(config.DeployTarget{Include: []string{"data/seed.db"}})
	if f[0] != "--include=data/seed.db" {
		t.Fatalf("deploy.include must be the first rule, got %v", f)
	}
}

func TestRsyncArgs(t *testing.T) {
	args := rsyncArgs("/proj/", "u@h:/opt/app", "ssh -p 2222", []string{"--exclude=.env"})
	want := []string{"-az", "-e", "ssh -p 2222", "--exclude=.env", "/proj/", "u@h:/opt/app"}
	if strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("args = %v; want %v", args, want)
	}
}

// Without rsync, a filtered (remote-build) upload must fail instead of falling
// back to scp, which would upload .env, .git and local databases.
func TestRsyncTo_RefusesScpFallbackWithFilters(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	err := rsyncTo("/proj/", "u@h:/opt/app", config.DeployTarget{}, remoteBuildFilters(config.DeployTarget{}))
	if err == nil || !strings.Contains(err.Error(), "rsync not found") {
		t.Fatalf("expected a refusal mentioning rsync, got %v", err)
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			r, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(r))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// End-to-end with the real rsync (local destination): secrets, VCS history and
// local databases are not delivered; .env.example and the source are.
func TestRemoteBuildFilters_RsyncDelivery(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not installed")
	}
	src, dst := t.TempDir(), t.TempDir()
	for rel, body := range map[string]string{
		"main.go":                "package main",
		"togo.yaml":              "name: demo",
		".env":                   "AUTH_SECRET=do-not-ship",
		".env.local":             "x=1",
		"api/.env":               "nested",
		".env.example":           "AUTH_SECRET=",
		".git/HEAD":              "ref: refs/heads/main",
		"togo.db":                "db",
		"togo.db-wal":            "wal",
		"data/dev.sqlite3":       "db",
		"node_modules/x/i.js":    "x",
		"web/node_modules/y.js":  "y",
		".togo/cache":            "c",
		".gitignore":             "ignored.txt\n",
		"ignored.txt":            "gitignored",
		"data/seed.db":           "seed",
		"deploy/keep.env.sample": "ok",
	} {
		write(t, src, rel, body)
	}
	target := config.DeployTarget{Include: []string{"data/seed.db"}}
	args := rsyncArgs(src+string(os.PathSeparator), dst, "ssh", remoteBuildFilters(target))
	if out, err := exec.Command("rsync", args...).CombinedOutput(); err != nil {
		t.Fatalf("rsync: %v\n%s", err, out)
	}
	got := strings.Join(listFiles(t, dst), ",")
	want := ".env.example,.gitignore,data/seed.db,deploy/keep.env.sample,main.go,togo.yaml"
	if got != want {
		t.Fatalf("delivered files:\n got  %s\n want %s", got, want)
	}
}
