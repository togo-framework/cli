package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/generator"
	"github.com/togo-framework/cli/internal/ui"
)

// pluginData is the view model for scaffolding a plugin mini-app.
type pluginData struct {
	Name   string // kebab plugin name, e.g. "billing"
	Pascal string // PascalCase, e.g. "Billing"
	Module string // Go module path
}

// pluginFiles maps output path -> template body. A togo plugin is a mini-app:
// self-registering provider, internal package, injectable web/, manifest, docs.
var pluginFiles = map[string]string{
	"go.mod": `module {{.Module}}

go 1.26.4

require github.com/togo-framework/togo v0.18.0
`,
	"plugin.go": `// Package {{.Name}} is a togo plugin (mini-app). It self-registers on blank-import
// via ` + "`togo install`" + `. Backend logic lives in internal/{{.Name}}; UI in web/.
package {{.Name}}

import (
	"github.com/togo-framework/togo"

	"{{.Module}}/internal/{{.Name}}"
)

const Name = "{{.Name}}"

func init() {
	togo.RegisterProviderFunc(Name, togo.PriorityLate, func(k *togo.Kernel) error {
		svc := {{.Name}}.New(k)
		k.Router.Get("/api/{{.Name}}/ping", svc.Ping)
		k.Set(Name, svc)
		if k.Log != nil {
			k.Log.Info("plugin active", "plugin", Name)
		}
		return nil
	})
}
`,
	"internal/{{.Name}}/service.go": `package {{.Name}}

import (
	"net/http"

	"github.com/togo-framework/togo"
)

// Service is the {{.Name}} plugin backend.
type Service struct{ k *togo.Kernel }

func New(k *togo.Kernel) *Service { return &Service{k: k} }

// Ping is a sample endpoint (GET /api/{{.Name}}/ping).
func (s *Service) Ping(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(` + "`" + `{"plugin":"{{.Name}}","status":"ok"}` + "`" + `))
}
`,
	"web/app/{{.Name}}/page.tsx": `"use client";

import { useEffect, useState } from "react";

export default function {{.Pascal}}Page() {
  const [status, setStatus] = useState("…");
  useEffect(() => {
    fetch("/api/{{.Name}}/ping").then((r) => r.json()).then((d) => setStatus(d.status)).catch(() => setStatus("error"));
  }, []);
  return (
    <div className="mx-auto max-w-xl p-8">
      <h1 className="text-2xl font-semibold">{{.Pascal}}</h1>
      <p className="mt-2 text-slate-500">Backend status: {status}</p>
    </div>
  );
}
`,
	"togo.plugin.yaml": `name: {{.Name}}
description: A togo {{.Name}} plugin (backend + frontend).
priority: 50
backend:
  package: {{.Module}}
frontend:
  dir: web
commands: []
`,
	"README.md": `# {{.Name}}

A togo plugin (mini-app) for [togo](https://github.com/togo-framework/togo).

` + "```bash\ntogo install <owner>/{{.Name}}\n```" + `

Exposes ` + "`GET /api/{{.Name}}/ping`" + ` and injects ` + "`web/app/{{.Name}}/page.tsx`" + `.
`,
	".claude/skills/{{.Name}}/SKILL.md": `---
name: {{.Name}}
description: Work with the {{.Name}} togo plugin.
---

# {{.Name}} plugin

Backend in ` + "`internal/{{.Name}}`" + `, routes in ` + "`plugin.go`" + `, UI under ` + "`web/`" + `.
`,
}

func registerMakePlugin(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "make:plugin <name>",
		Short:   "Scaffold a new togo plugin as a mini-app (provider + web + internal + .claude)",
		GroupID: groupMake,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := generator.Kebab(args[0])
			module, _ := cmd.Flags().GetString("module")
			if module == "" {
				module = "github.com/yourname/togo-" + name
			}
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				dir = filepath.Join("plugins", name)
			}
			force, _ := cmd.Flags().GetBool("force")

			data := pluginData{Name: name, Pascal: generator.Pascal(name), Module: module}
			created := 0
			for tmplPath, body := range pluginFiles {
				outRel := renderString(tmplPath, data)
				out := filepath.Join(dir, outRel)
				if _, err := os.Stat(out); err == nil && !force {
					ui.Warn("skip (exists): %s", outRel)
					continue
				}
				if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(out, []byte(renderString(body, data)), 0o644); err != nil {
					return err
				}
				ui.Step("CREATE %s", filepath.Join(dir, outRel))
				created++
			}
			ui.Success("Scaffolded plugin %q (%d files) in %s", name, created, dir)
			ui.Step("cd %s && go mod tidy", dir)
			return nil
		},
	}
	cmd.Flags().String("module", "", "Go module path (default: github.com/yourname/togo-<name>)")
	cmd.Flags().String("dir", "", "target directory (default: ./plugins/<name>)")
	cmd.Flags().Bool("force", false, "overwrite existing files")
	root.AddCommand(cmd)
}

func renderString(tmpl string, data pluginData) string {
	t, err := template.New("p").Parse(tmpl)
	if err != nil {
		return tmpl
	}
	var b strings.Builder
	if t.Execute(&b, data) != nil {
		return tmpl
	}
	return b.String()
}
