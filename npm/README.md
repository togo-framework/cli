# @togo-framework/cli

The **togo** CLI — scaffold and manage togo (Go + React) full-stack apps.

```bash
npm install -g @togo-framework/cli
# then:
togo new my-app
togo --help
```

On install, the matching prebuilt binary for your platform (macOS / Linux / Windows ·
x64 / arm64) is downloaded from the [GitHub release](https://github.com/togo-framework/cli/releases).
If no prebuilt binary is available but Go is installed, it falls back to `go install`.

**npm 11:** install scripts are gated by `allowScripts`. By default npm only warns; with
`strict-allow-scripts` (or `--ignore-scripts`) the postinstall is skipped and no binary is installed.
Allow it explicitly:

```bash
npm install -g --allow-scripts=@togo-framework/cli @togo-framework/cli
# or once, for every future global install:
npm config set allow-scripts=@togo-framework/cli --location=user
```

In a project-local install, add `"allowScripts": { "@togo-framework/cli": true }` to `package.json`
(`--allow-scripts` is rejected there). Or skip npm entirely:

```bash
go install github.com/togo-framework/cli/cmd/togo@latest
```

or grab a binary from [GitHub releases](https://github.com/togo-framework/cli/releases).

Prefer a single binary with no Node? Use the install script instead:

```bash
curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/install.sh | sh
```

MIT · https://github.com/togo-framework/cli
