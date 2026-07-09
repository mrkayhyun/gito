#!/usr/bin/env bash
#
# install.sh — build gito from this repo and install it locally for zsh.
#
# What it does:
#   1. Builds the gito binary (with the version stamped from git).
#   2. Installs it to an install dir (default: ~/.local/bin).
#   3. Ensures that dir is on your PATH via a marked block in ~/.zshrc
#      (idempotent — running it again won't duplicate anything).
#
# Usage:
#   ./install.sh                 # build + install to ~/.local/bin
#   INSTALL_DIR=/usr/local/bin ./install.sh
#   ./install.sh --uninstall     # remove binary + the ~/.zshrc block
#
set -euo pipefail

# ── config ────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
ZSHRC="${ZSHRC:-$HOME/.zshrc}"
BIN_NAME="gito"
MARK_BEGIN="# >>> gito installer >>>"
MARK_END="# <<< gito installer <<<"

# ── pretty output ───────────────────────────────────────────────────────────────
info()  { printf '\033[0;36m•\033[0m %s\n' "$*"; }
ok()    { printf '\033[0;32m✓\033[0m %s\n' "$*"; }
warn()  { printf '\033[0;33m!\033[0m %s\n' "$*"; }
die()   { printf '\033[0;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

# ── uninstall ─────────────────────────────────────────────────────────────────
uninstall() {
  if [ -f "$INSTALL_DIR/$BIN_NAME" ]; then
    rm -f "$INSTALL_DIR/$BIN_NAME"
    ok "removed $INSTALL_DIR/$BIN_NAME"
  else
    warn "no binary at $INSTALL_DIR/$BIN_NAME"
  fi
  if [ -f "$ZSHRC" ] && grep -qF "$MARK_BEGIN" "$ZSHRC"; then
    # delete the marked block (portable in-place edit via temp file)
    awk -v b="$MARK_BEGIN" -v e="$MARK_END" '
      $0==b {skip=1} skip && $0==e {skip=0; next} !skip {print}
    ' "$ZSHRC" > "$ZSHRC.gito.tmp" && mv "$ZSHRC.gito.tmp" "$ZSHRC"
    ok "removed gito PATH block from $ZSHRC"
  fi
  info "done. open a new terminal (or 'source $ZSHRC') to apply."
  exit 0
}

[ "${1:-}" = "--uninstall" ] && uninstall

# ── preflight ─────────────────────────────────────────────────────────────────
command -v go >/dev/null 2>&1 || die "Go toolchain not found. Install Go first: https://go.dev/dl/"

cd "$SCRIPT_DIR"
[ -f go.mod ] || die "go.mod not found in $SCRIPT_DIR — run this from the gito repo."

# version: prefer an exact tag, else <tag>-<n>-g<hash>, else short hash, else 'dev'
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

# ── build ─────────────────────────────────────────────────────────────────────
info "building $BIN_NAME ($VERSION)"
mkdir -p "$INSTALL_DIR"
CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$VERSION" \
  -o "$INSTALL_DIR/$BIN_NAME" .
ok "installed → $INSTALL_DIR/$BIN_NAME"

# ── ensure PATH in ~/.zshrc (idempotent) ───────────────────────────────────────
if grep -qF "$MARK_BEGIN" "$ZSHRC" 2>/dev/null; then
  ok "$ZSHRC already contains the gito PATH block (left unchanged)"
else
  {
    printf '\n%s\n' "$MARK_BEGIN"
    printf 'export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    printf '%s\n' "$MARK_END"
  } >> "$ZSHRC"
  ok "added PATH block to $ZSHRC"
fi

# ── done ────────────────────────────────────────────────────────────────────────
echo
if command -v "$BIN_NAME" >/dev/null 2>&1 && [ "$(command -v "$BIN_NAME")" = "$INSTALL_DIR/$BIN_NAME" ]; then
  ok "gito is on your PATH: $("$BIN_NAME" version)"
else
  warn "gito installed, but not yet on the PATH of THIS shell."
  info "run:  source $ZSHRC   (or open a new terminal)"
fi
echo
info "try it:  gito            # launcher menu"
info "         gito status     # or a specific command"
