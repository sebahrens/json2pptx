#!/usr/bin/env bash
set -euo pipefail

# install.sh — Build and install json2pptx + Claude Code skill
#
# Usage:
#   ./install.sh                   # Build + install to ~/.local
#   ./install.sh --prefix /usr/local
#   ./install.sh --skip-skill      # Binary only, no Claude skill
#   ./install.sh --skip-build      # Use pre-built bin/json2pptx
#   ./install.sh --skip-mcp        # Skip MCP server config
#   ./install.sh --skip-templates  # Skip template file installation

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Defaults
PREFIX="$HOME/.local"
SKIP_SKILL=false
SKIP_BUILD=false
SKIP_MCP=false
SKIP_TEMPLATES=false
SKILL_NAME="template-deck"
OLD_SKILL_NAME="make-slides"

# Binaries to install (user-facing tools)
INSTALL_CMDS=(json2pptx svggen svggen-server svggen-mcp)

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
    --skip-mcp)
      SKIP_MCP=true
      shift
      ;;
    --skip-templates)
      SKIP_TEMPLATES=true
      shift
      ;;
    -h|--help)
      echo "Usage: ./install.sh [OPTIONS]"
      echo ""
      echo "Options:"
      echo "  --prefix DIR      Install prefix (default: ~/.local)"
      echo "  --skip-skill      Don't install Claude Code skill"
      echo "  --skip-build      Use pre-built binaries"
      echo "  --skip-mcp        Don't install MCP server config"
      echo "  --skip-templates  Don't install template files"
      echo "  -h, --help        Show this help"
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
  echo "==> Building..."
  cd "$SCRIPT_DIR"
  mkdir -p bin

  # Main module binaries
  echo "    Building json2pptx..."
  go build -o bin/json2pptx ./cmd/json2pptx

  # svggen module binaries
  if [[ -f svggen/go.mod ]]; then
    for cmd in svggen svggen-server svggen-mcp; do
      if [[ -d "svggen/cmd/$cmd" ]]; then
        echo "    Building $cmd..."
        cd svggen && go build -o ../bin/"$cmd" "./cmd/$cmd" && cd ..
      fi
    done
  fi
fi

# --- Install binaries ---

echo ""
echo "==> Installing binaries..."
mkdir -p "$PREFIX/bin"

for cmd in "${INSTALL_CMDS[@]}"; do
  BINARY="$SCRIPT_DIR/bin/$cmd"
  if [[ -f "$BINARY" ]]; then
    cp "$BINARY" "$PREFIX/bin/$cmd"
    chmod +x "$PREFIX/bin/$cmd"
    echo "    $PREFIX/bin/$cmd"
  fi
done

# Verify main binary exists
if [[ ! -f "$PREFIX/bin/json2pptx" ]]; then
  echo "ERROR: json2pptx not found after install. Build may have failed."
  exit 1
fi

# --- Install templates ---

if [[ "$SKIP_TEMPLATES" == false ]]; then
  echo ""
  echo "==> Installing templates..."
  TEMPLATES_DIR="$HOME/.json2pptx/templates"
  mkdir -p "$TEMPLATES_DIR"
  if ls "$SCRIPT_DIR"/templates/*.pptx >/dev/null 2>&1; then
    cp "$SCRIPT_DIR"/templates/*.pptx "$TEMPLATES_DIR/"
    TEMPLATE_COUNT=$(ls -1 "$SCRIPT_DIR"/templates/*.pptx | wc -l | tr -d ' ')
    echo "    $TEMPLATES_DIR/ ($TEMPLATE_COUNT templates)"
  else
    echo "    WARNING: No .pptx templates found in templates/"
  fi
fi

# --- Install Claude Code skill ---

if [[ "$SKIP_SKILL" == false ]]; then
  echo ""
  echo "==> Installing Claude Code skill ($SKILL_NAME)..."

  SKILL_SRC="$SCRIPT_DIR/.claude/skills/$SKILL_NAME"
  SKILL_DST="$HOME/.claude/skills/$SKILL_NAME"

  if [[ ! -d "$SKILL_SRC" ]]; then
    echo "    Skipped (no skill files found)"
  else
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
fi

# --- Install MCP config ---

if [[ "$SKIP_MCP" == false ]]; then
  echo ""
  echo "==> Configuring MCP server..."

  MCP_FILE="$HOME/.claude/mcp.json"
  BINARY_PATH="$PREFIX/bin/json2pptx"
  TEMPLATES_PATH="$HOME/.json2pptx/templates"

  mkdir -p "$(dirname "$MCP_FILE")"

  if command -v jq &>/dev/null; then
    TMPFILE="$MCP_FILE.$$.tmp"
    if [[ -f "$MCP_FILE" ]]; then
      jq --arg bin "$BINARY_PATH" --arg tdir "$TEMPLATES_PATH" \
        '.mcpServers["json2pptx"] = {command: $bin, args: ["mcp", "--templates-dir", $tdir, "--output", "./output"]}' \
        "$MCP_FILE" > "$TMPFILE" && mv "$TMPFILE" "$MCP_FILE"
    else
      printf '{"mcpServers":{"json2pptx":{"command":"%s","args":["mcp","--templates-dir","%s","--output","./output"]}}}\n' \
        "$BINARY_PATH" "$TEMPLATES_PATH" | jq . > "$TMPFILE" && mv "$TMPFILE" "$MCP_FILE"
    fi
    echo "    $MCP_FILE (json2pptx server configured)"
  else
    # No jq -- write or warn
    if [[ ! -f "$MCP_FILE" ]]; then
      cat > "$MCP_FILE" <<MCPEOF
{
  "mcpServers": {
    "json2pptx": {
      "command": "$BINARY_PATH",
      "args": ["mcp", "--templates-dir", "$TEMPLATES_PATH", "--output", "./output"]
    }
  }
}
MCPEOF
      echo "    $MCP_FILE (json2pptx server configured)"
    else
      echo "    WARNING: jq not found and $MCP_FILE already exists."
      echo "             Add json2pptx to $MCP_FILE manually."
    fi
  fi
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
echo "  Binaries:  $PREFIX/bin/"
if [[ "$SKIP_TEMPLATES" == false ]]; then
  echo "  Templates: ~/.json2pptx/templates/"
fi
if [[ "$SKIP_SKILL" == false ]]; then
  echo "  Skill:     ~/.claude/skills/$SKILL_NAME/"
fi
if [[ "$SKIP_MCP" == false ]]; then
  echo "  MCP:       ~/.claude/mcp.json"
fi

# PATH warning
if [[ ":$PATH:" != *":$PREFIX/bin:"* ]]; then
  echo ""
  echo "WARNING: $PREFIX/bin is not in your PATH."
  echo "         Add to your shell profile:"
  echo ""
  echo "    export PATH=\"$PREFIX/bin:\$PATH\""
fi
