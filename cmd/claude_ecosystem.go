package cmd

import (
	"os"
	"path/filepath"

	"github.com/togo-framework/cli/internal/ui"
)

// claudePack is a named bundle of .claude/ files (skills, agents, rules) that
// `togo mcp:install --pack <name>` publishes so a togo app is agent-ready.
type claudePack struct {
	desc  string
	files map[string]string
}

// claudePacks are the installable ecosystem packs (à la orchestra-mcp).
var claudePacks = map[string]claudePack{
	"essentials": {
		desc: "Core togo skills, agents, and rules for Claude Code",
		files: map[string]string{
			"CLAUDE.md": `# Project guidance for Claude Code

This is a **togo** app (Go + sqlc + Atlas + GraphQL/REST + Next.js). Conventions:

- Add entities with ` + "`togo make:resource <Name> field:type`" + `, then ` + "`togo generate && togo migrate`" + `.
- ` + "`*.gen.go`" + ` and ` + "`internal/**/gen/`" + ` are generated — never hand-edit.
- API-first: every resource is REST/OpenAPI + GraphQL. Config via ` + "`.env`" + `/togo.yaml.
- Everything is a plugin (microkernel). Add capabilities with ` + "`togo install <owner>/<repo>`" + `.
- Use ` + "`togo dev`" + ` for local (hot reload), ` + "`togo format`" + ` / ` + "`togo lint`" + ` for code standards.

See .claude/rules/togo.md for detail and .claude/skills for slash commands.
`,
			".claude/settings.json": `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          { "type": "command", "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/gofmt.sh" }
        ]
      }
    ]
  }
}
`,
			".claude/hooks/gofmt.sh": `#!/usr/bin/env bash
# togo PostToolUse hook: gofmt Go files after Claude edits them.
input=$(cat)
file=$(printf '%s' "$input" | sed -n 's/.*"file_path"[: ]*"\([^"]*\)".*/\1/p' | head -1)
case "$file" in
  *.go) command -v gofmt >/dev/null 2>&1 && gofmt -w "$file" >/dev/null 2>&1 ;;
esac
exit 0
`,
			".claude/rules/togo.md": `# togo conventions

- Entities are added with ` + "`togo make:resource`" + `; ` + "`togo.resources.yaml`" + ` is the source of truth.
- ` + "`*.gen.go`" + ` and ` + "`internal/**/gen/`" + ` are generated — never hand-edit.
- ` + "`togo generate`" + ` runs sqlc -> gqlgen -> atlas -> OpenAPI (the build gate).
- API-first: every resource is exposed over REST/OpenAPI and GraphQL.
- Config is dynamic (togo.yaml + .env); never hard-code URLs/connections.
- Everything is a plugin; the kernel is a microkernel. Add features with ` + "`togo install`" + `.
`,
			".claude/skills/togo-resource/SKILL.md": `---
name: togo-resource
description: Scaffold a full resource (model + migration + REST + GraphQL + page) in a togo app.
---

# togo-resource

Run ` + "`togo make:resource <Name> field:type ...`" + ` then ` + "`togo generate`" + ` and ` + "`togo migrate`" + `.
Fields support ` + "`name:type[:nullable]`" + `. The resource is exposed over REST + GraphQL with a page.
`,
			".claude/skills/togo-migrate/SKILL.md": `---
name: togo-migrate
description: Manage the database schema in a togo app.
---

# togo-migrate

` + "`togo migrate`" + ` applies schema. After ` + "`make:resource`" + `, run ` + "`togo generate && togo migrate`" + `.
`,
			".claude/skills/togo-deploy/SKILL.md": `---
name: togo-deploy
description: Build and deploy a togo app.
---

# togo-deploy

` + "`togo dev`" + ` for local (all services, hot reload). ` + "`togo serve --host --port`" + ` for production.
` + "`togo deploy`" + ` provisions infra (Terraform).
`,
			".claude/agents/togo-backend.md": `---
name: togo-backend
description: Go backend specialist for togo apps (ORM, REST/GraphQL, providers, plugins).
---

You build togo backends. Use the core flow (make:model -> migrate -> make:controller),
the ORM (models.Xs(a).Where(...).Get(ctx)), API resource transformers, and the
provider/plugin pattern. Keep features as plugins; never hand-edit generated code.
`,
			".claude/agents/togo-frontend.md": `---
name: togo-frontend
description: Next.js frontend specialist for togo apps (App Router, Tailwind, trans()).
---

You build togo frontends with Next.js (App Router) + Tailwind. Use generated API
types/hooks under web/lib, localize with trans(), and subscribe to realtime via
useEvents(). Pages live under web/app.
`,
			".claude/agents/togo-db.md": `---
name: togo-db
description: Database/schema specialist for togo (sqlc, Atlas, drivers).
---

You design togo schemas. SQLite is the default driver; Postgres/MySQL/Supabase via
DB provider plugins. Keep schema files driver-agnostic; let the ORM handle dialects.
`,
		},
	},
}

// publishClaude writes a pack's .claude/ files (idempotent unless force).
func publishClaude(pack string, force bool) error {
	p, ok := claudePacks[pack]
	if !ok {
		ui.Warn("unknown pack %q (have: essentials)", pack)
		return nil
	}
	created := 0
	for rel, body := range p.files {
		if _, err := os.Stat(rel); err == nil && !force {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if filepath.Ext(rel) == ".sh" {
			mode = 0o755 // hooks must be executable
		}
		if err := os.WriteFile(rel, []byte(body), mode); err != nil {
			return err
		}
		created++
	}
	ui.Success("Published Claude pack %q (%d files): skills, agents, rules", pack, created)
	return nil
}
