#!/usr/bin/env bash
set -euo pipefail

# install.sh — Build and install json2pptx + Claude Code skill
#
# Usage:
#   ./install.sh                   # Build + install to ~/.local
#   ./install.sh --prefix /usr/local
#   ./install.sh --skip-skill      # Binary only, no Claude skill
#   ./install.sh --skip-build      # Use pre-built bin/json2pptx

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Defaults
PREFIX="$HOME/.local"
SKIP_SKILL=false
SKIP_BUILD=false
SKILL_NAME="template-deck"
OLD_SKILL_NAME="make-slides"

# Parse flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    --skip-skill)
      SKIP_SKILL=true
      shift
      ;;
    --skip-build)
      SKIP_BUILD=true
      shift
      ;;
    -h|--help)
      echo "Usage: ./install.sh [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --prefix DIR    Install prefix (default: ~/.local)"
      echo "  --skip-skill    Don't install Claude Code skill"
      echo "  --skip-build    Use pre-built bin/json2pptx"
      echo "  -h, --help      Show this help"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Resolve prefix to absolute path
PREFIX="$(cd "$PREFIX" 2>/dev/null && pwd || echo "$PREFIX")"

echo "==> json2pptx installer"
echo "    prefix: $PREFIX"
echo ""

# --- Prerequisites ---

if [[ "$SKIP_BUILD" == false ]]; then
  # Check Go
  if ! command -v go &>/dev/null; then
    echo "ERROR: Go is required but not installed."
    echo "       Install from https://go.dev/dl/"
    exit 1
  fi

  GO_VERSION="$(go version | grep -oE '[0-9]+\.[0-9]+' | head -1)"
  GO_MAJOR="${GO_VERSION%%.*}"
  GO_MINOR="${GO_VERSION#*.}"
  if [[ "$GO_MAJOR" -lt 1 ]] || { [[ "$GO_MAJOR" -eq 1 ]] && [[ "$GO_MINOR" -lt 23 ]]; }; then
    echo "ERROR: Go >= 1.23 required (found $GO_VERSION)"
    exit 1
  fi
  echo "    go: $(go version)"
fi

# --- Build ---

if [[ "$SKIP_BUILD" == false ]]; then
  echo ""
  echo "==> Building json2pptx..."
  cd "$SCRIPT_DIR"
  go build -o bin/json2pptx ./cmd/json2pptx
  echo "    Built: bin/json2pptx"
fi

# Verify binary exists
BINARY="$SCRIPT_DIR/bin/json2pptx"
if [[ ! -f "$BINARY" ]]; then
  echo "ERROR: bin/json2pptx not found. Run without --skip-build or build first."
  exit 1
fi

# --- Install binary ---

echo ""
echo "==> Installing binary..."
mkdir -p "$PREFIX/bin"
cp "$BINARY" "$PREFIX/bin/json2pptx"
chmod +x "$PREFIX/bin/json2pptx"
echo "    Installed: $PREFIX/bin/json2pptx"

# --- Install Claude Code skill ---

if [[ "$SKIP_SKILL" == false ]]; then
  echo ""
  echo "==> Installing Claude Code skill ($SKILL_NAME)..."

  SKILL_SRC="$SCRIPT_DIR/.claude/skills/$SKILL_NAME"
  SKILL_DST="$HOME/.claude/skills/$SKILL_NAME"

  if [[ ! -d "$SKILL_SRC" ]]; then
    echo "ERROR: Skill source not found: $SKILL_SRC"
    exit 1
  fi

  # Clean up old skill name if present
  OLD_SKILL_DST="$HOME/.claude/skills/$OLD_SKILL_NAME"
  if [[ -d "$OLD_SKILL_DST" ]]; then
    echo "    Removing old skill: $OLD_SKILL_DST"
    rm -rf "$OLD_SKILL_DST"
  fi

  mkdir -p "$SKILL_DST"
  cp -R "$SKILL_SRC"/* "$SKILL_DST"/
  echo "    Installed: $SKILL_DST"
fi

# --- Verify ---

echo ""
echo "==> Verifying..."
if "$PREFIX/bin/json2pptx" version 2>/dev/null; then
  echo "    Binary OK"
else
  echo "WARNING: json2pptx version check failed"
fi

# --- Summary ---

echo ""
echo "==> Done!"
echo ""
echo "  Binary:  $PREFIX/bin/json2pptx"
if [[ "$SKIP_SKILL" == false ]]; then
  echo "  Skill:   ~/.claude/skills/$SKILL_NAME/"
fi

# PATH warning
if [[ ":$PATH:" != *":$PREFIX/bin:"* ]]; then
  echo ""
  echo "WARNING: $PREFIX/bin is not in your PATH."
  echo "         Add to your shell profile:"
  echo ""
  echo "    export PATH=\"$PREFIX/bin:\$PATH\""
fi
