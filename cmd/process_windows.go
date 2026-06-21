//go:build windows

package cmd

import "os/exec"

// setProcessGroup is a no-op on Windows (no POSIX process groups).
func setProcessGroup(c *exec.Cmd) {}

// terminate kills the child process on Windows.
func terminate(c *exec.Cmd) {
	if c.Process != nil {
		_ = c.Process.Kill()
	}
}
