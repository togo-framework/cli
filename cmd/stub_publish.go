package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/templates"
	"github.com/togo-framework/cli/internal/ui"
)

func registerStubPublish(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "stub:publish",
		Short:   "Copy generator stubs to ./.togo/stubs for per-project customization",
		GroupID: groupCodegen,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			force, _ := cmd.Flags().GetBool("force")
			keys, err := templates.Keys()
			if err != nil {
				return err
			}
			n := 0
			for _, key := range keys {
				dest := filepath.Join(proj.Root, templates.StubDir, key)
				if !force {
					if _, err := os.Stat(dest); err == nil {
						continue
					}
				}
				data, err := templates.Read("", key)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(dest, data, 0o644); err != nil {
					return err
				}
				n++
			}
			ui.Success("Published %d stubs to %s", n, templates.StubDir)
			return nil
		},
	}
	root.AddCommand(cmd)
}
