package cmd

import (
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

// pluginsFile is the generated blank-import file that drives kernel auto-discovery.
const pluginsFile = "internal/plugins/plugins.gen.go"

// pluginManifest mirrors the fields of a plugin's togo.plugin.yaml we care about.
type pluginManifest struct {
	Name    string `yaml:"name"`
	Backend struct {
		Package string `yaml:"package"`
	} `yaml:"backend"`
	Frontend struct {
		Dir string `yaml:"dir"`
	} `yaml:"frontend"`
	Env []string `yaml:"env"`
}

func registerPlugin(root *cobra.Command) {
	install := &cobra.Command{
		Use:     "install <owner/repo | claude>",
		Short:   "Install a togo plugin, or the togo Claude Code plugin (`togo install claude`)",
		Long: `Install a togo plugin from a GitHub repository.

Use "togo install claude" to install the togo Claude Code plugin
(` + claudeMarketplace + `) — its agents, commands, rules and hooks, with the
togo MCP auto-connected — so Claude Code can scaffold and drive togo apps.`,
		GroupID: groupPlugin,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if t := strings.ToLower(args[0]); t == "claude" || t == "claude-code" {
				return installClaudePlugin()
			}
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			return installPlugin(proj, args[0])
		},
	}

	list := &cobra.Command{
		Use:     "plugin:list",
		Short:   "List installed plugins",
		GroupID: groupPlugin,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			pkgs := readPluginImports(proj.Root)
			if len(pkgs) == 0 {
				ui.Info("No plugins installed")
				return nil
			}
			for _, p := range pkgs {
				ui.Step("• %s", p)
			}
			return nil
		},
	}

	root.AddCommand(install, list)
}

// claudeMarketplace is the GitHub repo hosting the togo Claude Code plugin + marketplace.
const claudeMarketplace = "togo-framework/claude-togo"

// installClaudePlugin installs the togo Claude Code plugin into the user's Claude
// Code: it adds the claude-togo marketplace and installs the `togo` plugin (which
// auto-connects the togo MCP). If the Claude Code CLI isn't on PATH, it prints the
// in-session slash commands instead.
func installClaudePlugin() error {
	ui.Step("Installing the togo Claude Code plugin (%s)…", claudeMarketplace)
	claude, err := exec.LookPath("claude")
	if err != nil {
		ui.Warn("Claude Code CLI not found on PATH.")
		ui.Info("Install Claude Code (https://claude.com/claude-code), then run in a session:")
		ui.Step("/plugin marketplace add %s", claudeMarketplace)
		ui.Step("/plugin install togo@togo")
		return nil
	}
	for _, a := range [][]string{
		{"plugin", "marketplace", "add", claudeMarketplace},
		{"plugin", "install", "togo@togo"},
	} {
		ui.Step("claude %s", strings.Join(a, " "))
		c := exec.Command(claude, a...)
		c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := c.Run(); err != nil {
			return fmt.Errorf("claude %s: %w", strings.Join(a, " "), err)
		}
	}
	ui.Success("togo Claude Code plugin installed — the togo MCP is auto-connected.")
	ui.Info("Try: /togo:new · /togo:resource · /togo:serve")
	return nil
}

func installPlugin(proj *config.Project, repo string) error {
	repo = strings.TrimPrefix(strings.TrimSuffix(repo, "/"), "github.com/")
	if strings.Count(repo, "/") < 1 {
		return fmt.Errorf("expected owner/repo (e.g. fadymondy/cms), got %q", repo)
	}
	module := "github.com/" + repo
	ui.Info("Installing plugin %s", repo)

	// Resolve the backend import package from the plugin's manifest (best-effort).
	pkg := module
	if m, err := fetchManifest(repo); err == nil {
		if m.Backend.Package != "" {
			pkg = m.Backend.Package
		}
		if len(m.Env) > 0 {
			ui.Step("env required: %s", strings.Join(m.Env, ", "))
		}
		ui.Step("manifest: %s", m.Name)
	} else {
		ui.Warn("no togo.plugin.yaml found upstream; assuming package %s", module)
	}

	// Fetch the module.
	if err := goGet(proj.Root, module+"@latest"); err != nil {
		return err
	}

	// Register for auto-discovery + record in togo.yaml.
	if err := addPluginImport(proj.Root, pkg); err != nil {
		return err
	}
	recordPluginInConfig(proj, pkg)

	// Inject the plugin's frontend (pages/components) into the app's web/.
	webDir := "web"
	if m, _ := fetchManifest(repo); m != nil && m.Frontend.Dir != "" {
		webDir = m.Frontend.Dir
	}
	if n, err := injectFrontend(proj, module, webDir); err != nil {
		ui.Warn("frontend injection skipped: %v", err)
	} else if n > 0 {
		ui.Step("injected %d frontend file(s) into %s/", n, proj.Frontend.Dir)
	}

	// Resolve the new dependency.
	if err := goModTidy(proj.Root); err != nil {
		ui.Warn("go mod tidy failed: %v (run it manually)", err)
	}

	ui.Success("Installed %s", pkg)
	ui.Step("it auto-registers with the kernel on next `togo serve`")
	return nil
}

// fetchManifest tries main then master for togo.plugin.yaml.
func fetchManifest(repo string) (*pluginManifest, error) {
	for _, branch := range []string{"main", "master"} {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/togo.plugin.yaml", repo, branch)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var m pluginManifest
		if err := yaml.Unmarshal(body, &m); err != nil {
			return nil, err
		}
		return &m, nil
	}
	return nil, fmt.Errorf("manifest not found")
}

// readPluginImports parses the blank-import package paths from plugins.gen.go.
func readPluginImports(root string) []string {
	data, err := os.ReadFile(filepath.Join(root, pluginsFile))
	if err != nil {
		return nil
	}
	var pkgs []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "_ \"") {
			pkgs = append(pkgs, strings.TrimSuffix(strings.TrimPrefix(line, "_ \""), "\""))
		}
	}
	return pkgs
}

// addPluginImport adds pkg to the set of installed plugins and rewrites plugins.gen.go.
func addPluginImport(root, pkg string) error {
	set := map[string]bool{}
	for _, p := range readPluginImports(root) {
		set[p] = true
	}
	set[pkg] = true
	pkgs := make([]string, 0, len(set))
	for p := range set {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)

	var b strings.Builder
	b.WriteString("// Code generated by togo. DO NOT EDIT.\n//\n")
	b.WriteString("// Installed plugins are blank-imported here so their init() registers them\n")
	b.WriteString("// with the kernel for auto-discovery. Managed by `togo install`.\n")
	b.WriteString("package plugins\n")
	if len(pkgs) > 0 {
		b.WriteString("\nimport (\n")
		for _, p := range pkgs {
			fmt.Fprintf(&b, "\t_ %q\n", p)
		}
		b.WriteString(")\n")
	}
	out, err := format.Source([]byte(b.String()))
	if err != nil {
		return err
	}
	dest := filepath.Join(root, pluginsFile)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, out, 0o644)
}

// recordPluginInConfig appends the plugin to togo.yaml's plugins list (best-effort,
// for human visibility — plugins.gen.go is the source of truth).
func recordPluginInConfig(proj *config.Project, pkg string) {
	path := filepath.Join(proj.Root, config.ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil {
		return
	}
	existing, _ := doc["plugins"].([]any)
	for _, e := range existing {
		if s, ok := e.(string); ok && s == pkg {
			return
		}
	}
	doc["plugins"] = append(existing, pkg)
	if out, err := yaml.Marshal(doc); err == nil {
		_ = os.WriteFile(path, out, 0o644)
	}
}

// goModuleDir returns the local (module cache) directory of a required module.
func goModuleDir(projRoot, module string) (string, error) {
	c := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", module)
	c.Dir = projRoot
	c.Env = os.Environ()
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// injectFrontend copies a plugin's web/ subtree (pages, components) into the
// app's frontend dir so the plugin can serve UI (e.g. auth views).
func injectFrontend(proj *config.Project, module, webSub string) (int, error) {
	dir, err := goModuleDir(proj.Root, module)
	if err != nil || dir == "" {
		return 0, err
	}
	src := filepath.Join(dir, webSub)
	if _, err := os.Stat(src); err != nil {
		return 0, nil // plugin has no frontend
	}
	dest := filepath.Join(proj.Root, proj.Frontend.Dir)
	count := 0
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "node_modules" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

func goGet(dir, mod string) error {
	if !goAvailable() {
		ui.Warn("go not found — skipped `go get %s`", mod)
		return nil
	}
	c := exec.Command("go", "get", mod)
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		return fmt.Errorf("go get %s: %w", mod, err)
	}
	return nil
}
