#!/usr/bin/env sh
# togo CLI installer (Linux / macOS).
#
#   curl -fsSL https://raw.githubusercontent.com/togo-framework/cli/main/install.sh | sh
#
# Installs the `togo` binary. Prefers a prebuilt release asset; falls back to
# `go install` when Go is available.
set -eu

REPO="togo-framework/cli"
BIN="togo"
INSTALL_PATH="github.com/togo-framework/cli/cmd/togo@latest"

info()  { printf '\033[36m→\033[0m %s\n' "$1"; }
ok()    { printf '\033[32m✓\033[0m %s\n' "$1"; }
err()   { printf '\033[31m✗\033[0m %s\n' "$1" >&2; }

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo linux ;;
    Darwin*) echo darwin ;;
    *)       echo unsupported ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    arm64|aarch64) echo arm64 ;;
    *) echo unsupported ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
info "Detected ${OS}/${ARCH}"

# 1) Try a prebuilt release asset (once releases are published).
try_release() {
  [ "$OS" = "unsupported" ] && return 1
  [ "$ARCH" = "unsupported" ] && return 1
  TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
        | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
  [ -z "${TAG:-}" ] && return 1
  ASSET="${BIN}_${OS}_${ARCH}.tar.gz"
  URL="https://github.com/${REPO}/releases/download/${TAG}/${ASSET}"
  info "Downloading ${ASSET} (${TAG})"
  TMP="$(mktemp -d)"
  curl -fsSL "$URL" -o "${TMP}/${ASSET}" || { rm -rf "$TMP"; return 1; }
  tar -xzf "${TMP}/${ASSET}" -C "$TMP" || { rm -rf "$TMP"; return 1; }
  # Install OVER an existing togo (avoids two binaries shadowing each other);
  # otherwise honor TOGO_INSTALL_DIR, else /usr/local/bin.
  EXISTING="$(command -v "$BIN" 2>/dev/null || true)"
  if [ -n "${TOGO_INSTALL_DIR:-}" ]; then DEST="$TOGO_INSTALL_DIR"
  elif [ -n "$EXISTING" ]; then DEST="$(dirname "$EXISTING")"
  else DEST="/usr/local/bin"; fi
  if [ -w "$DEST" ]; then mv -f "${TMP}/${BIN}" "${DEST}/${BIN}"; else sudo mv -f "${TMP}/${BIN}" "${DEST}/${BIN}"; fi
  chmod +x "${DEST}/${BIN}" 2>/dev/null || true
  rm -rf "$TMP"
  ok "Installed ${BIN} ${TAG} to ${DEST}/${BIN}"
  return 0
}

# 2) Fall back to `go install`.
try_go() {
  command -v go >/dev/null 2>&1 || return 1
  info "Installing via go install (${INSTALL_PATH})"
  go install "$INSTALL_PATH"
  GOBIN="$(go env GOBIN)"; [ -z "$GOBIN" ] && GOBIN="$(go env GOPATH)/bin"
  ok "Installed ${BIN} to ${GOBIN}/${BIN}"
  case ":$PATH:" in
    *":$GOBIN:"*) : ;;
    *) info "Add to PATH:  export PATH=\"\$PATH:${GOBIN}\"" ;;
  esac
  return 0
}

if try_release; then :; elif try_go; then :; else
  err "Could not install: no release asset for ${OS}/${ARCH} and Go is not installed."
  err "Install Go (https://go.dev/dl) or grab a binary from https://github.com/${REPO}/releases"
  exit 1
fi

# Refresh the shell's command cache and report what will actually run.
hash -r 2>/dev/null || true
ACTIVE="$(command -v "$BIN" 2>/dev/null || true)"
if [ -n "$ACTIVE" ]; then
  ok "$("$ACTIVE" version 2>/dev/null || echo "$BIN installed")"
  info "active binary: ${ACTIVE}"
  # Warn if another togo earlier on PATH would shadow the one we installed.
  if [ -n "${DEST:-}" ] && [ "$(dirname "$ACTIVE")" != "$DEST" ]; then
    err "another '${BIN}' at ${ACTIVE} shadows ${DEST}/${BIN} — remove it or fix PATH order"
  fi
  info "if 'togo version' still shows an old version, run:  hash -r   (or open a new shell)"
fi
