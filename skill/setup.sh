#!/usr/bin/env bash
# setup.sh — build the s1temap CLI binary (Linux / macOS)
#
# Safe to run from anywhere: it resolves the Go module root (repo root) relative to
# this script, so it works whether the skill lives in the repo or is copied into
# an agent's skills directory.
set -e

REQUIRED_MAJOR=1
REQUIRED_MINOR=26
BINARY=s1temap

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)" # repo root — the directory containing go.mod
cd "$MODULE_DIR"

# ── Check Go ──────────────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
  echo "Error: Go is not installed or not in PATH."
  echo ""
  echo "Install Go 1.${REQUIRED_MINOR} or later:"
  echo "  macOS:   brew install go"
  echo "  Linux:   sudo apt install golang-go   (Debian/Ubuntu)"
  echo "  Any:     https://go.dev/dl/"
  echo ""
  echo "After installation, open a new terminal and re-run this script."
  exit 1
fi

# Portable version parse: "go version go1.26.1 ..." -> "1.26"
GO_VERSION=$(go version | awk '{print $3}' | sed 's/^go//' | cut -d. -f1,2)
MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
MINOR=$(echo "$GO_VERSION" | cut -d. -f2)

if [ "$MAJOR" -lt "$REQUIRED_MAJOR" ] || { [ "$MAJOR" -eq "$REQUIRED_MAJOR" ] && [ "$MINOR" -lt "$REQUIRED_MINOR" ]; }; then
  echo "Error: Go ${REQUIRED_MAJOR}.${REQUIRED_MINOR}+ is required (found go${GO_VERSION})."
  echo ""
  echo "Upgrade Go:"
  echo "  macOS:   brew upgrade go"
  echo "  Any:     https://go.dev/dl/"
  echo ""
  echo "After upgrading, open a new terminal and re-run this script."
  exit 1
fi

echo "Go $(go version | awk '{print $3}') — OK"

# ── Build (skip if binary already exists) ────────────────────────────────────
if [ -x "./${BINARY}" ]; then
  echo "Binary ${MODULE_DIR}/${BINARY} already exists — skipping build."
  echo "Delete it or run 'go build -o ${BINARY} ./cmd/cli' to rebuild."
else
  echo "Building ${BINARY}..."
  go build -o "${BINARY}" ./cmd/cli
  echo ""
  echo "Build complete: ${MODULE_DIR}/${BINARY}"
fi

echo ""
echo "Quick start:"
echo "  ${MODULE_DIR}/${BINARY} start https://example.com/sitemap.xml"
echo "  ${MODULE_DIR}/${BINARY} start https://example.com/sitemap.xml --filter-status=!200"
echo "  ${MODULE_DIR}/${BINARY} list ./urls.txt"
echo ""
echo "Optional HTTP API server: go build -o s1temap-api ./cmd/api"
echo "See SKILL.md for the full command reference and examples."
