#!/usr/bin/env sh
# Update the togo CLI to the latest version (Linux / macOS).
#
#   curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/update.sh | sh
set -eu
info() { printf '\033[36m→\033[0m %s\n' "$1"; }
ok()   { printf '\033[32m✓\033[0m %s\n' "$1"; }

info "Updating togo…"
# Re-run the installer, which always fetches @latest / the newest release.
curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/install.sh | sh
ok "togo is up to date"
