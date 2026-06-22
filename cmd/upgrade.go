package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

// installPath is the go-installable path that produces the `togo` binary.
const installPath = "github.com/togo-framework/cli/cmd/togo"

func registerUpgrade(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "upgrade [version]",
		Aliases: []string{"self-update", "update"},
		Short:   "Update the togo CLI to the latest version (or a given version)",
		GroupID: groupProject,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := "latest"
			if len(args) == 1 {
				version = args[0]
			}
			ui.Info("Upgrading togo CLI to %s…", version)

			if !goAvailable() {
				ui.Warn("go not found — use the install script instead:")
				ui.Step("curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/install.sh | sh")
				return nil
			}

			c := exec.Command("go", "install", installPath+"@"+version)
			c.Stdout, c.Stderr = os.Stdout, os.Stderr
			c.Env = os.Environ()
			if err := c.Run(); err != nil {
				return fmt.Errorf("upgrade failed: %w", err)
			}

			gobin, _ := exec.Command("go", "env", "GOBIN").Output()
			path := trim(string(gobin))
			if path == "" {
				gopath, _ := exec.Command("go", "env", "GOPATH").Output()
				path = trim(string(gopath)) + "/bin"
			}
			ui.Success("togo upgraded — installed to %s/togo", path)
			ui.Step("run `togo version` to confirm")
			return nil
		},
	}
	root.AddCommand(cmd)
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
