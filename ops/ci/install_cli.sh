#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
BIN_DIR="${HOME}/.local/bin"
TARGET="${BIN_DIR}/ci-self"
SOURCE="${ROOT_DIR}/ops/ci/ci_self.sh"

mkdir -p "$BIN_DIR"
ln -sf "$SOURCE" "$TARGET"
chmod +x "$SOURCE"

echo "OK: installed ci-self -> $TARGET"
echo "HINT: ensure ${BIN_DIR} is in PATH"
echo "HINT: run 'ci-self help'"
