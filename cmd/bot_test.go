package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/togo-framework/cli/internal/config"
)

func TestDotenvMap(t *testing.T) {
	dir := t.TempDir()
	env := `# comment
BOT_DRIVER=telegram
export AI_DRIVER="openai"
TELEGRAM_BOT_TOKEN='123:abc'

EMPTY=
NOEQ
`
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}
	m := dotenvMap(dir)
	if m["BOT_DRIVER"] != "telegram" {
		t.Errorf("BOT_DRIVER = %q", m["BOT_DRIVER"])
	}
	if m["AI_DRIVER"] != "openai" {
		t.Errorf("AI_DRIVER (export + quotes) = %q", m["AI_DRIVER"])
	}
	if m["TELEGRAM_BOT_TOKEN"] != "123:abc" {
		t.Errorf("token (single quotes) = %q", m["TELEGRAM_BOT_TOKEN"])
	}
	if _, ok := m["NOEQ"]; ok {
		t.Error("line without = should be skipped")
	}
}

func TestDotenvMapMissingFile(t *testing.T) {
	if m := dotenvMap(t.TempDir()); len(m) != 0 {
		t.Errorf("expected empty map for missing .env, got %v", m)
	}
}

func TestResolveEnvDriver(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("BOT_DRIVER=discord\n"), 0o600)
	proj := &config.Project{Root: dir}

	// Override wins.
	if got := resolveEnvDriver(proj, "Telegram", "BOT_DRIVER"); got != "telegram" {
		t.Errorf("override = %q, want telegram (lowercased)", got)
	}
	// Falls back to .env.
	if got := resolveEnvDriver(proj, "", "BOT_DRIVER"); got != "discord" {
		t.Errorf("dotenv = %q, want discord", got)
	}
	// Process env when not in .env.
	t.Setenv("AI_DRIVER", "openai")
	if got := resolveEnvDriver(proj, "", "AI_DRIVER"); got != "openai" {
		t.Errorf("process env = %q, want openai", got)
	}
}

func TestEnsurePluginInstalled(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module x\n\nrequire github.com/togo-framework/bot-telegram v0.1.0\n"), 0o600)
	proj := &config.Project{Root: dir}

	if err := ensurePluginInstalled(proj, "bot driver", "telegram",
		"github.com/togo-framework/bot-telegram", "bot-telegram"); err != nil {
		t.Errorf("installed plugin should pass: %v", err)
	}
	if err := ensurePluginInstalled(proj, "bot driver", "discord",
		"github.com/togo-framework/bot-discord", "bot-discord"); err == nil {
		t.Error("missing plugin should error with install hint")
	}
}
