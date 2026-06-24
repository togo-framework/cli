<p align="center"><img src=".github/assets/togo-mark.svg" width="96" alt="togo"></p>

# togo CLI

The `togo` command — a Laravel-artisan-like CLI for the [togo](https://github.com/togo-framework/togo) framework.

## Install

**npm** (downloads the prebuilt binary for your platform):
```bash
npm install -g @togo-framework/cli
```

**Linux / macOS**
```bash
curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/install.sh | sh
```

**Windows (PowerShell)**
```powershell
irm https://raw.githubusercontent.com/togo-framework/cli/main/install.ps1 | iex
```

**With Go** (note the `/cmd/togo` path — it's what makes the binary `togo`):
```bash
go install github.com/togo-framework/cli/cmd/togo@latest
```

## Update

```bash
curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/update.sh | sh   # Linux/macOS
irm https://raw.githubusercontent.com/togo-framework/cli/main/update.ps1 | iex        # Windows
```

## Commands

```
Project      togo new <app> · togo dev · togo serve · togo web · togo version · togo upgrade
Make         togo make:resource <Name> <field:type...>   (model, sqlc, Atlas schema, GraphQL, REST, seeder, page)
             togo make:model · make:controller · make:action · make:view · make:plugin
Codegen      togo generate   (sqlc → gqlgen → Atlas diff/migrate → OpenAPI export)
             togo stub:publish
Quality      togo format · togo lint · togo test
Marketplace  togo install <owner/repo | agent:<name> | skill:<name> | claude> · togo install --list · togo plugin:list
Deploy       togo deploy [env]
AI / MCP     togo mcp:install · togo mcp:serve · togo agent <description>
Diagnostics  togo doctor    (checks &amp; auto-installs Go, Node/npm, sqlc, atlas)
Infra        togo infra:init <provider>
```

`togo new` and the generators auto-install any missing toolchain (Go, Node/npm,
sqlc, atlas); run `togo doctor` any time to check or repair them.

## Install — plugins, agents & skills

Everything in the [togo marketplace](https://to-go.dev/marketplace) installs with one command:

```bash
togo install togo-framework/auth     # a plugin (Go capability) — go get + auto-register
togo install agent:togo-backend      # an AI agent  → .claude/agents/togo-backend.md
togo install skill:resource          # a skill       → .claude/commands/resource.md
togo install claude                  # the togo Claude Code plugin (15-agent team + auto-wired MCP)
togo install --list                  # list every installable plugin, agent and skill
```

Agents and skills are sourced from the [togo Claude Code plugin](https://github.com/togo-framework/claude-togo) and dropped into your project's `.claude/` so Claude Code picks them up immediately.

## Deploy

`togo deploy` ships a togo app to your server fast — build locally, rsync the
artifact (scp fallback), and run a restart command over SSH. Config lives in
`togo.yaml`:

```yaml
deploy:
  host: 203.0.113.10
  user: deploy
  path: /opt/myapp
  restart: systemctl restart myapp        # run over SSH after upload
  # optional: port · ssh_key · build · binary · goos · goarch · remote_build
  # multi-environment instead of the inline target:
  # default: production
  # targets:
  #   production: { host: …, user: …, path: …, restart: … }
  #   staging:    { host: …, user: …, path: …, restart: … }
```

```bash
togo deploy                  # the inline target (or deploy.default)
togo deploy staging          # a named target
togo deploy --dry-run        # print the plan, change nothing
togo deploy --remote-build   # rsync source and build on the server
```

Env overrides: `TOGO_DEPLOY_HOST` / `_USER` / `_PATH` / `_SSH_KEY`.

## Generators

`togo make:resource Post title:string body:text:nullable published:bool` emits
**per-resource fragments** across six targets plus regenerated route/resolver
registries, all driven by `togo.resources.yaml`. Field types: `string, text,
int, bool, float, decimal, uuid, time, date, json`; append `:nullable` (or a
quoted `?`) to make a field optional. Flags: `--dry-run`, `--force`.

## CLI is plugin-extensible

Any executable named `togo-<cmd>` on `PATH` or in `~/.togo/bin` becomes
`togo <cmd>` (git/kubectl-style), and `togo install owner/repo` registers plugin
commands.

## License

MIT

---

## 💎 Premium sponsors

togo is proudly sponsored by **ID8 Media** and **One Studio**.

<p align="center">
  <a href="https://id8media.com"><img src=".github/assets/id8media.svg" height="44" alt="ID8 Media" /></a>
  &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;
  <a href="https://one-studio.co"><img src=".github/assets/one-studio.jpeg" height="44" alt="One Studio" /></a>
</p>
