package cmd

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/togo-framework/cli/internal/ui"
)

// goAvailable reports whether the go toolchain is on PATH.
func goAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

// goModTidy runs `go mod tidy` in dir, streaming output. It is a no-op when go
// is not installed.
func goModTidy(dir string) error {
	if !goAvailable() {
		ui.Warn("go not found — skipping module resolution (install Go: https://go.dev/dl)")
		return nil
	}
	c := exec.Command("go", "mod", "tidy")
	c.Dir = dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	c.Env = os.Environ()
	return c.Run()
}

// goSumExists reports whether the project already has a resolved go.sum.
func goSumExists(root string) bool {
	_, err := os.Stat(filepath.Join(root, "go.sum"))
	return err == nil
}

// ensureModules makes sure the project's Go dependencies are resolved before a
// command that compiles the app (serve, generate). It tidies only when go.sum is
// missing so steady-state runs stay fast.
func ensureModules(root string) error {
	if goSumExists(root) {
		return nil
	}
	ui.Info("Resolving Go modules (first run)…")
	return goModTidy(root)
}
