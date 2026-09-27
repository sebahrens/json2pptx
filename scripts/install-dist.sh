#!/bin/sh
# install.sh — Install json2pptx binary, Claude Code skill, and MCP config.
#
# Usage:
#   ./install.sh                       # Install to ~/.local
#   ./install.sh --prefix /usr/local   # Install to /usr/local (needs sudo)
#   ./install.sh --skip-skill          # Binary only, no Claude skill
#   ./install.sh --skip-mcp            # Skip MCP server config
#
# This script is POSIX sh compatible (no bashisms).

set -eu

# Defaults
PREFIX="$HOME/.local"
SKIP_SKILL=false
SKIP_MCP=false

# Resolve the directory this script lives in
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Parse arguments
while [ $# -gt 0 ]; do
  case "$1" in
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    --skip-skill)
      SKIP_SKILL=true
      shift
      ;;
    --skip-mcp)
      SKIP_MCP=true
      shift
      ;;
    -h|--help)
      cat <<'USAGE'
Usage: ./install.sh [OPTIONS]

Options:
  --prefix DIR    Install prefix (default: ~/.local)
  --skip-skill    Don't install Claude Code skill
  --skip-mcp      Don't install MCP server config
  -h, --help      Show this help

Installs:
  $PREFIX/bin/json2pptx                    CLI binary (also serves as MCP server)
  ~/.claude/skills/*/                      Claude Code skills (unless --skip-skill)
  claude mcp add --scope user json2pptx    MCP server registration (unless --skip-mcp)
USAGE
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

echo "==> json2pptx installer"
echo "    prefix: $PREFIX"
echo ""

# --- Install binary ---

echo "==> Installing binary..."
mkdir -p "$PREFIX/bin"

# Support both old (md2pptx) and new (json2pptx) binary names in archive
if [ -f "$SCRIPT_DIR/bin/json2pptx" ]; then
  cp "$SCRIPT_DIR/bin/json2pptx" "$PREFIX/bin/json2pptx"
elif [ -f "$SCRIPT_DIR/bin/md2pptx" ]; then
  cp "$SCRIPT_DIR/bin/md2pptx" "$PREFIX/bin/json2pptx"
else
  echo "ERROR: No binary found in archive." >&2
  exit 1
fi
chmod +x "$PREFIX/bin/json2pptx"
echo "    $PREFIX/bin/json2pptx"

# --- Install templates (lean distribution) ---

if [ -d "$SCRIPT_DIR/templates" ]; then
  echo ""
  echo "==> Installing templates..."
  TEMPLATES_DIR="$HOME/.json2pptx/templates"
  mkdir -p "$TEMPLATES_DIR"
  cp "$SCRIPT_DIR/templates/"*.pptx "$TEMPLATES_DIR/"
  TEMPLATE_COUNT=$(ls -1 "$SCRIPT_DIR/templates/"*.pptx | wc -l)
  echo "    $TEMPLATES_DIR/ ($TEMPLATE_COUNT templates)"
fi

# --- Install Claude Code skill ---

if [ "$SKIP_SKILL" = false ]; then
  echo ""
  echo "==> Installing Claude Code skills..."

  # Clean up old skill name if present
  OLD_SKILL_DIR="$HOME/.claude/skills/make-slides"
  if [ -d "$OLD_SKILL_DIR" ]; then
    rm -rf "$OLD_SKILL_DIR"
    echo "    Removed old skill: $OLD_SKILL_DIR"
  fi

  # The archive's skills/ tree was already staged by scripts/stage-skills.sh
  # (every skill, references snapshot, rewritten links); install all of it.
  for skill_src in "$SCRIPT_DIR"/skills/*/; do
    [ -d "$skill_src" ] || continue
    skill_name=$(basename "$skill_src")
    SKILL_DIR="$HOME/.claude/skills/$skill_name"
    mkdir -p "$SKILL_DIR"
    cp -R "$skill_src". "$SKILL_DIR/"
    echo "    $SKILL_DIR/"
  done
fi

# --- Install MCP config ---

if [ "$SKIP_MCP" = false ]; then
  echo ""
  echo "==> Configuring MCP server..."

  BINARY_PATH="$PREFIX/bin/json2pptx"
  TEMPLATES_DIR="$HOME/.json2pptx/templates"

  # Claude Code reads user-scope MCP servers from ~/.claude.json, managed by
  # `claude mcp add --scope user` (it does not read ~/.claude/mcp.json).
  if command -v claude >/dev/null 2>&1; then
    claude mcp remove --scope user json2pptx >/dev/null 2>&1 || true
    if claude mcp add --scope user json2pptx -- "$BINARY_PATH" mcp --templates-dir "$TEMPLATES_DIR" --output ./output >/dev/null; then
      echo "    Registered json2pptx with Claude Code (user scope)"
    else
      echo "    WARNING: 'claude mcp add' failed. Register manually with:"
      echo "      claude mcp add --scope user json2pptx -- \"$BINARY_PATH\" mcp --templates-dir \"$TEMPLATES_DIR\" --output ./output"
    fi
  else
    echo "    Claude Code CLI ('claude') not found on PATH. Register the server with:"
    echo "      claude mcp add --scope user json2pptx -- \"$BINARY_PATH\" mcp --templates-dir \"$TEMPLATES_DIR\" --output ./output"
  fi
fi

# --- Verify ---

echo ""
echo "==> Verifying..."
if "$PREFIX/bin/json2pptx" version >/dev/null 2>&1; then
  VERSION_OUT=$("$PREFIX/bin/json2pptx" version 2>&1)
  echo "    $VERSION_OUT"
else
  echo "    WARNING: json2pptx version check failed."
fi

# --- Summary ---

echo ""
echo "==> Done!"
echo ""
echo "  Binary:    $PREFIX/bin/json2pptx"
if [ -d "$SCRIPT_DIR/templates" ]; then
  echo "  Templates: $HOME/.json2pptx/templates/"
fi
if [ "$SKIP_SKILL" = false ]; then
  echo "  Skills:    ~/.claude/skills/*/"
fi
if [ "$SKIP_MCP" = false ]; then
  echo "  MCP:       json2pptx (Claude Code user scope; see: claude mcp get json2pptx)"
fi

# PATH warning
case ":${PATH:-}:" in
  *":$PREFIX/bin:"*) ;;
  *)
    echo ""
    echo "NOTE: $PREFIX/bin is not in your PATH."
    echo "      Add to your shell profile:"
    echo ""
    echo "    export PATH=\"$PREFIX/bin:\$PATH\""
    ;;
esac
