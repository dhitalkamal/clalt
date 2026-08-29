#!/usr/bin/env bash
# clalt installer: build clalt from source and put it on your PATH, one command,
# any OS. Re-runnable. Env overrides:
#   INSTALL_DIR         where to copy the binary (default: ~/go/bin if present,
#                       else ~/.local/bin; on Windows/git-bash ~/bin)
#   CODESIGN_IDENTITY   macOS only: stable code-signing identity name so keychain
#                       access stops prompting (see README "macOS Keychain
#                       prompts"). If unset, the script uses "clalt-codesign" when
#                       that identity already exists.
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

echo "building $bin ..."
go build -o "$bin" .

if [ "$os" = "Darwin" ]; then
  id="${CODESIGN_IDENTITY:-}"
  if [ -z "$id" ] && security find-identity -v -p codesigning 2>/dev/null | grep -q "clalt-codesign"; then
    id="clalt-codesign"
  fi
  if [ -n "$id" ]; then
    echo "signing with identity '$id' (keychain grants will persist) ..."
    codesign --force --identifier com.clalt.cli --sign "$id" "$bin"
  else
    echo "note: no stable code-signing identity found; macOS will keep prompting for"
    echo "      your login password on keychain access. Create one once (README:"
    echo "      'macOS Keychain prompts'), then re-run: CODESIGN_IDENTITY=clalt-codesign $0"
  fi
fi

cp "$bin" "$INSTALL_DIR/$bin"
echo "installed to $INSTALL_DIR/$bin"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "warning: $INSTALL_DIR is not on your PATH; add it to use 'clalt' directly";;
esac

echo "done. run 'clalt help' to get started; background token keepalive turns on"
echo "automatically once you add an account."
