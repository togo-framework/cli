package cmd

import (
	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/ui"
)

func registerInfra(root *cobra.Command) {
	initCmd := &cobra.Command{
		Use:     "infra:init <provider>",
		Short:   "Scaffold Terraform infra for a cloud provider (aws|gcp|azure|fly|hetzner|do)",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Info("Scaffolding Terraform for provider %q", args[0])
			ui.Warn("Terraform modules land in the infra phase")
			return nil
		},
	}

	deploy := &cobra.Command{
		Use:     "deploy",
		Short:   "Provision infra (Terraform) and ship the app to your cloud",
		GroupID: groupInfra,
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Warn("`togo deploy` lands in the infra phase (Terraform plan/apply to a separate infra repo)")
			return nil
		},
	}

	root.AddCommand(initCmd, deploy)
}
