#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PREFIX="${PREFIX:-$HOME/.local/bin}"
BIN_NAME="tasker"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required. Install Go 1.22+ and try again."
  exit 1
fi

mkdir -p "$PREFIX"

cd "$ROOT_DIR"
go build -o "$PREFIX/$BIN_NAME" ./cmd/tasker

echo "Installed to: $PREFIX/$BIN_NAME"
echo "Ensure $PREFIX is on your PATH."
