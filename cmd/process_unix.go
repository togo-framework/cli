//go:build !windows

package cmd

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the child in its own process group so we can signal the
// whole group (e.g. `go run` and the server it spawns) on shutdown.
func setProcessGroup(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminate sends SIGTERM to the child's process group.
func terminate(c *exec.Cmd) {
	if c.Process == nil {
		return
	}
	_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
}
