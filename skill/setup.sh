#!/usr/bin/env bash
# setup.sh — build & install the s1temap CLI binary (Linux / macOS)
#
# The binary is ALWAYS installed to one fixed, session-independent location:
#     $S1TEMAP_HOME/s1temap   (default: ~/.s1temap/bin/s1temap)
#
# This is the key to reliability: every future agent session — in Claude Code,
# Codex, Cursor, etc. — looks in the same fixed path, regardless of the current
# working directory or where the skill folder happens to live.
#
# The script is safe to run from anywhere: it walks up from its own location to
# find the Go module root (the directory containing go.mod) only when a build is
# actually needed.
#
# The LAST line printed to stdout is the absolute path to the binary, so an agent
# can capture and remember it.
set -e

REQUIRED_MAJOR=1
REQUIRED_MINOR=26
BINARY=s1temap

# ── Fixed install location (override with S1TEMAP_HOME) ───────────────────────
INSTALL_DIR="${S1TEMAP_HOME:-$HOME/.s1temap/bin}"
INSTALL_PATH="$INSTALL_DIR/$BINARY"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "$INSTALL_DIR"

# ── 1) Already installed? Use it, print path, done. ──────────────────────────
if [ -x "$INSTALL_PATH" ]; then
  echo "Binary already installed — skipping build."
  echo "To rebuild: delete '$INSTALL_PATH' and re-run this script."
  echo ""
  echo "Quick start:"
  echo "  $INSTALL_PATH start https://example.com/sitemap.xml"
  echo "  $INSTALL_PATH start https://example.com/sitemap.xml --filter-status=!200"
  echo "  $INSTALL_PATH list ./urls.txt"
  echo ""
  echo "$INSTALL_PATH"
  exit 0
fi

# ── 2) Need to build: locate the Go module root (walk up to find go.mod) ─────
MODULE_DIR="$SCRIPT_DIR"
while [ "$MODULE_DIR" != "/" ] && [ ! -f "$MODULE_DIR/go.mod" ]; do
  MODULE_DIR="$(dirname "$MODULE_DIR")"
done

if [ ! -f "$MODULE_DIR/go.mod" ]; then
  echo "Error: could not find go.mod above '$SCRIPT_DIR'."
  echo ""
  echo "The binary is not installed yet and the Go source is required to build it."
  echo "Clone the repository and run this script from inside it:"
  echo "  git clone https://github.com/preditrix/s1temap"
  echo "  cd s1temap"
  echo "  bash skill/setup.sh"
  exit 1
fi

# ── 3) Check Go ───────────────────────────────────────────────────────────────
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

# ── 4) Build straight into the fixed install location ────────────────────────
echo "Building ${BINARY} -> ${INSTALL_PATH} ..."
( cd "$MODULE_DIR" && go build -trimpath -ldflags="-s -w" -o "$INSTALL_PATH" ./cmd/cli )
echo "Installed: $INSTALL_PATH"
echo ""

echo "Quick start:"
echo "  $INSTALL_PATH start https://example.com/sitemap.xml"
echo "  $INSTALL_PATH start https://example.com/sitemap.xml --filter-status=!200"
echo "  $INSTALL_PATH list ./urls.txt"
echo ""
echo "Optional HTTP API server: (cd \"$MODULE_DIR\" && go build -o \"$INSTALL_DIR/s1temap-api\" ./cmd/api)"
echo "See SKILL.md for the full command reference and examples."
echo ""

# LAST line = absolute binary path (agents capture this)
echo "$INSTALL_PATH"
