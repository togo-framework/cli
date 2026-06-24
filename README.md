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
Project      togo new <app> · togo serve · togo version
Make         togo make:resource <Name> <field:type...>  (model, sqlc, Atlas, GraphQL, REST, seeder, Next.js page)
             togo make:model|query|migration|graphql|api|seeder|page
Codegen      togo generate   (sqlc → gqlgen → atlas diff → OpenAPI export)
             togo stub:publish
Database     togo migrate · migrate:diff · migrate:status · migrate:fresh · seed · db:reset
Plugins      togo install <owner/repo> · togo plugin:list
MCP / AI     togo mcp:install --agent claude-code · togo mcp:serve
Infra        togo infra:init <provider> · togo deploy
```

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
