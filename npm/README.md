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

Prefer a single binary with no Node? Use the install script instead:

```bash
curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/install.sh | sh
```

MIT · https://github.com/togo-framework/cli
