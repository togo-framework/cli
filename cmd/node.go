package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/togo-framework/cli/internal/ui"
)

// errSkipWeb signals that the web service can't run (e.g. Node not installed)
// but the backend should continue.
var errSkipWeb = errors.New("skip web service")

// packageManager describes a JS package manager and how to invoke it.
type packageManager struct {
	bin     string
	install []string
	dev     []string
}

// detectPM picks a package manager for the web dir: it honors a lockfile when
// the matching tool is installed, otherwise falls back to npm.
func detectPM(webDir string) packageManager {
	type cand struct {
		lock string
		pm   packageManager
	}
	cands := []cand{
		{"bun.lockb", packageManager{"bun", []string{"install"}, []string{"run", "dev"}}},
		{"pnpm-lock.yaml", packageManager{"pnpm", []string{"install"}, []string{"dev"}}},
		{"yarn.lock", packageManager{"yarn", []string{"install"}, []string{"dev"}}},
		{"package-lock.json", packageManager{"npm", []string{"install"}, []string{"run", "dev"}}},
	}
	for _, c := range cands {
		if fileExists(filepath.Join(webDir, c.lock)) && hasBin(c.pm.bin) {
			return c.pm
		}
	}
	return packageManager{"npm", []string{"install"}, []string{"run", "dev"}}
}

// ensureNodeModules installs frontend dependencies the first time (when
// node_modules is absent), as the user expects from `togo serve`.
func ensureNodeModules(webDir string, pm packageManager) error {
	if fileExists(filepath.Join(webDir, "node_modules")) {
		return nil
	}
	if !hasBin(pm.bin) {
		ui.Warn("%s not found — install Node.js to run the frontend (https://nodejs.org)", pm.bin)
		return errSkipWeb
	}
	ui.Info("Installing frontend dependencies with %s (first run)…", pm.bin)
	c := exec.Command(pm.bin, pm.install...)
	c.Dir = webDir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	c.Env = os.Environ()
	return c.Run()
}

func hasBin(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
