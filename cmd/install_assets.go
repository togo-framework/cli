package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

// The togo Claude Code plugin hosts the installable agents (agents/*.md) and
// skills (commands/*.md). `togo install agent:/skill:<name>` copies them into the
// current project's .claude/ so Claude Code picks them up — agents/skills install
// as easily as plugins.
const claudeTogoAssetRepo = "togo-framework/claude-togo"

type ghContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// assetDir maps a kind to its directory in the claude-togo repo and the
// destination directory under the project's .claude/.
func assetDirs(kind string) (srcDir, dstDir string, ok bool) {
	switch kind {
	case "agent":
		return "agents", filepath.Join(".claude", "agents"), true
	case "skill":
		// claude-togo ships skills as Claude Code commands; install them as
		// project commands (.claude/commands/<name>.md → /<name>).
		return "commands", filepath.Join(".claude", "commands"), true
	default:
		return "", "", false
	}
}

// splitAssetRef parses "agent:<name>" / "skill:<name>" into (kind, name).
func splitAssetRef(arg string) (kind, name string, ok bool) {
	for _, k := range []string{"agent", "skill"} {
		if rest, found := strings.CutPrefix(arg, k+":"); found && rest != "" {
			return k, rest, true
		}
	}
	return "", "", false
}

// listAssets returns the available asset names (without .md) under a claude-togo dir.
func listAssets(dir string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", claudeTogoAssetRepo, dir)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github contents %s: %s", dir, resp.Status)
	}
	var items []ghContent
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}
	var names []string
	for _, it := range items {
		if it.Type == "file" && strings.HasSuffix(it.Name, ".md") && !strings.EqualFold(it.Name, "README.md") {
			names = append(names, strings.TrimSuffix(it.Name, ".md"))
		}
	}
	sort.Strings(names)
	return names, nil
}

// fetchAsset downloads a raw .md (tries main then master).
func fetchAsset(path string) ([]byte, error) {
	for _, branch := range []string{"main", "master"} {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", claudeTogoAssetRepo, branch, path)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			return b, err
		}
		resp.Body.Close()
	}
	return nil, fmt.Errorf("not found: %s", path)
}

// installAsset copies an agent or skill from claude-togo into the project's .claude/.
func installAsset(proj *config.Project, kind, name string) error {
	name = strings.TrimSuffix(name, ".md")
	srcDir, dstDir, ok := assetDirs(kind)
	if !ok {
		return fmt.Errorf("unknown asset kind %q", kind)
	}
	ui.Step("Installing %s %q from %s…", kind, name, claudeTogoAssetRepo)
	body, err := fetchAsset(srcDir + "/" + name + ".md")
	if err != nil {
		if avail, e := listAssets(srcDir); e == nil {
			return fmt.Errorf("%s %q not found. Available: %s", kind, name, strings.Join(avail, ", "))
		}
		return fmt.Errorf("%s %q not found upstream", kind, name)
	}
	dst := filepath.Join(proj.Root, dstDir, name+".md")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		return err
	}
	rel, _ := filepath.Rel(proj.Root, dst)
	ui.Success("Installed %s → %s", kind, rel)
	if kind == "skill" {
		ui.Info("Use it in Claude Code as /%s", name)
	} else {
		ui.Info("Claude Code auto-loads the %s agent next session", name)
	}
	return nil
}

// resolveBareAsset checks whether a bare name matches an agent or skill upstream.
func resolveBareAsset(name string) (kind string, ok bool) {
	if agents, err := listAssets("agents"); err == nil {
		for _, a := range agents {
			if a == name {
				return "agent", true
			}
		}
	}
	if skills, err := listAssets("commands"); err == nil {
		for _, s := range skills {
			if s == name {
				return "skill", true
			}
		}
	}
	return "", false
}

// listInstallable prints the agents + skills available from the togo marketplace.
func listInstallable() error {
	ui.Step("Installable from the togo marketplace (%s):", claudeTogoAssetRepo)
	agents, err := listAssets("agents")
	if err != nil {
		return fmt.Errorf("fetch agents: %w", err)
	}
	ui.Info("Agents  — togo install agent:<name>")
	for _, a := range agents {
		ui.Step("  • %s", a)
	}
	skills, err := listAssets("commands")
	if err != nil {
		return fmt.Errorf("fetch skills: %w", err)
	}
	ui.Info("Skills  — togo install skill:<name>")
	for _, s := range skills {
		ui.Step("  • %s", s)
	}
	ui.Info("Plugins — togo install <owner>/<repo>   ·   Claude Code plugin — togo install claude")
	return nil
}
