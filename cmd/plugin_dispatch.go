package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
)

// tryExternalPlugin implements git/kubectl-style command extension. If the first
// non-flag argument is not a known cobra command, it looks for an executable
// named "togo-<arg>" on PATH or in ~/.togo/bin and execs it with the remaining
// args. Returns true when it handled (dispatched) the invocation.
func tryExternalPlugin(args []string) bool {
	if len(args) == 0 {
		return false
	}
	verb := args[0]
	if verb == "" || verb[0] == '-' {
		return false
	}
	// Defer to cobra for any command it already knows.
	if isKnown(verb) {
		return false
	}

	bin := findPluginBinary("togo-" + verb)
	if bin == "" {
		return false // let cobra produce its normal "unknown command" error
	}

	c := exec.Command(bin, args[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = os.Environ()
	if err := c.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		os.Exit(1)
	}
	return true
}

// isKnown reports whether verb matches a registered top-level command name.
func isKnown(verb string) bool {
	for _, c := range rootCmd.Commands() {
		if c.Name() == verb {
			return true
		}
		for _, a := range c.Aliases {
			if a == verb {
				return true
			}
		}
	}
	return verb == "help" || verb == "completion"
}

// findPluginBinary searches ~/.togo/bin then PATH for the given executable.
func findPluginBinary(name string) string {
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".togo", "bin", name)
		if isExecutable(candidate) {
			return candidate
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

func isExecutable(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return false
	}
	return fi.Mode()&0o111 != 0
}
