package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/generator"
	"github.com/togo-framework/cli/internal/templates"
	"github.com/togo-framework/cli/internal/ui"
)

// providers maps a provider key to a sensible default region.
var providers = map[string]string{
	"gcp":    "me-central1",
	"fly":    "fra",
	"docker": "local",
}

type infraData struct {
	App      string
	Provider string
	Region   string
}

func registerInfra(root *cobra.Command) {
	initCmd := &cobra.Command{
		Use:     "infra:init <provider>",
		Short:   "Scaffold Terraform + Dockerfile for a cloud provider (gcp|fly|docker)",
		GroupID: groupInfra,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			provider := args[0]
			region, ok := providers[provider]
			if !ok {
				return fmt.Errorf("unknown provider %q (gcp|fly|docker)", provider)
			}
			if r, _ := cmd.Flags().GetString("region"); r != "" {
				region = r
			}
			return infraInit(proj, provider, region)
		},
	}
	initCmd.Flags().String("region", "", "deployment region")

	plan := terraformCmd("infra:plan", "Show the Terraform plan", "plan")
	apply := terraformCmd("infra:apply", "Apply the Terraform plan", "apply", "-auto-approve")
	deploy := terraformCmd("deploy", "Provision infra (Terraform) and ship the app", "apply", "-auto-approve")

	root.AddCommand(initCmd, plan, apply, deploy)
}

func infraInit(proj *config.Project, provider, region string) error {
	data := infraData{App: proj.Name, Provider: provider, Region: region}
	files := []struct{ tmpl, dest string }{
		{"infra/Dockerfile.tmpl", "Dockerfile"},
		{"infra/" + provider + "/main.tf.tmpl", "infra/main.tf"},
		{"infra/README.md.tmpl", "infra/README.md"},
	}
	for _, f := range files {
		out, err := renderInfra(proj.Root, f.tmpl, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", f.tmpl, err)
		}
		dest := filepath.Join(proj.Root, f.dest)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, out, 0o644); err != nil {
			return err
		}
		ui.Step("%s  %s", ui.Label("CREATE"), f.dest)
	}
	ui.Success("Scaffolded %s infra (region %s)", provider, region)
	ui.Step("next: docker build -t %s:latest . && togo infra:plan && togo deploy", proj.Name)
	return nil
}

func renderInfra(root, key string, data infraData) ([]byte, error) {
	raw, err := templates.Read(root, key)
	if err != nil {
		return nil, err
	}
	t, err := template.New(filepath.Base(key)).Funcs(generator.FuncMap()).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// terraformCmd builds a command that runs terraform in the project's infra dir.
func terraformCmd(use, short string, tfArgs ...string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		GroupID: groupInfra,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			infraDir := filepath.Join(proj.Root, "infra")
			if _, err := os.Stat(infraDir); err != nil {
				return fmt.Errorf("no infra/ directory — run `togo infra:init <provider>` first")
			}
			if _, err := exec.LookPath("terraform"); err != nil {
				ui.Warn("terraform not found — install: https://developer.hashicorp.com/terraform/install")
				return nil
			}
			// Ensure providers are installed.
			if !fileExists(filepath.Join(infraDir, ".terraform")) {
				if err := runTerraform(infraDir, "init"); err != nil {
					return err
				}
			}
			return runTerraform(infraDir, append(tfArgs, args...)...)
		},
	}
}

func runTerraform(dir string, args ...string) error {
	c := exec.Command("terraform", append([]string{"-chdir=" + dir}, args...)...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.Env = os.Environ()
	return c.Run()
}
