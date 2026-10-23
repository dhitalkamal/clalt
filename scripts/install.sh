#!/usr/bin/env bash
# clalt installer: build clalt from source and put it on your PATH, one command,
# any OS. Re-runnable. Env overrides:
#   INSTALL_DIR   where to copy the binary (default: ~/go/bin if present, else
#                 ~/.local/bin; on Windows/git-bash ~/bin)
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo"

os="$(uname -s 2>/dev/null || echo unknown)"

# pick an install dir that is likely already on PATH
if [ -z "${INSTALL_DIR:-}" ]; then
  if [ -d "$HOME/go/bin" ]; then
    INSTALL_DIR="$HOME/go/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi
mkdir -p "$INSTALL_DIR"

bin="clalt"
case "$os" in MINGW*|MSYS*|CYGWIN*) bin="clalt.exe";; esac

version="$(git describe --tags --always --dirty 2>/dev/null || true)"
commit="$(git rev-parse HEAD 2>/dev/null || true)"
date="$(date -u +%Y-%m-%d)"
pkg="github.com/dhitalkamal/clalt/internal/version"
ldflags="-X ${pkg}.Commit=${commit} -X ${pkg}.Date=${date}"
if [ -n "$version" ]; then
  ldflags="${ldflags} -X ${pkg}.Version=${version}"
fi

echo "building $bin ..."
go build -ldflags "$ldflags" -o "$bin" .

cp "$bin" "$INSTALL_DIR/$bin"
echo "installed to $INSTALL_DIR/$bin"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "warning: $INSTALL_DIR is not on your PATH; add it to use 'clalt' directly";;
esac

echo "done. run 'clalt help', then 'clalt login <name>' to sign an account in."
