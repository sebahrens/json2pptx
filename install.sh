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

# version_lt A B: true when dotted version A is older than B (numeric
# components; pre-release suffixes such as "rc1" are ignored).
version_lt() {
  local IFS=.
  local -a a=($1) b=($2)
  local i x y
  for i in 0 1 2; do
    x="${a[i]:-0}"; x="${x%%[^0-9]*}"; x="${x:-0}"
    y="${b[i]:-0}"; y="${y%%[^0-9]*}"; y="${y:-0}"
    if (( 10#$x < 10#$y )); then return 0; fi
    if (( 10#$x > 10#$y )); then return 1; fi
  done
  return 1
}

# --- Prerequisites ---

if [[ "$SKIP_BUILD" == false ]]; then
  # Check Go
  if ! command -v go &>/dev/null; then
    echo "ERROR: Go is required but not installed."
    echo "       Install from https://go.dev/dl/"
    exit 1
  fi

  # The minimum comes from go.mod's `go` directive so the check cannot drift
  # from what the build actually requires. Query the local toolchain from
  # outside the module so go.mod does not trigger a toolchain switch.
  GO_MIN="$(awk '$1 == "go" { print $2; exit }' "$SCRIPT_DIR/go.mod")"
  GO_VERSION="$(cd / && go env GOVERSION)"
  GO_VERSION="${GO_VERSION#go}"
  if version_lt "$GO_VERSION" "$GO_MIN"; then
    echo "ERROR: Go >= $GO_MIN required by go.mod (found $GO_VERSION)"
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
  # Only the templates embedded in the binary (templates/embed.go) are
  # installed; a local gitignored template such as p-style.pptx is not.
  if bash "$SCRIPT_DIR/scripts/shipped-templates.sh" --stage "$TEMPLATES_DIR"; then
    TEMPLATE_COUNT=$(bash "$SCRIPT_DIR/scripts/shipped-templates.sh" | wc -l | tr -d ' ')
    echo "    $TEMPLATES_DIR/ ($TEMPLATE_COUNT templates)"
  else
    echo "    WARNING: could not install the templates listed in templates/embed.go"
  fi
fi

# --- Install Claude Code skill ---

if [[ "$SKIP_SKILL" == false ]]; then
  echo ""
  echo "==> Installing Claude Code skills..."

  # Clean up old skill name if present
  OLD_SKILL_DST="$HOME/.claude/skills/$OLD_SKILL_NAME"
  if [[ -d "$OLD_SKILL_DST" ]]; then
    echo "    Removing old skill: $OLD_SKILL_DST"
    rm -rf "$OLD_SKILL_DST"
  fi

  # Same staging as `make install`: every skill under skills/, the
  # references/repository snapshot, and ../../docs-style links rewritten so
  # they resolve inside ~/.claude/skills.
  bash "$SCRIPT_DIR/scripts/stage-skills.sh" "$HOME/.claude/skills"
  for SKILL_SRC in "$SCRIPT_DIR"/skills/*/; do
    echo "    Installed: $HOME/.claude/skills/$(basename "$SKILL_SRC")"
  done
fi

# --- Install MCP config ---

if [[ "$SKIP_MCP" == false ]]; then
  echo ""
  echo "==> Configuring MCP server..."

  BINARY_PATH="$PREFIX/bin/json2pptx"
  TEMPLATES_PATH="$HOME/.json2pptx/templates"
  MCP_ADD=(claude mcp add --scope user json2pptx -- "$BINARY_PATH" mcp --templates-dir "$TEMPLATES_PATH" --output ./output)

  # Claude Code reads user-scope MCP servers from ~/.claude.json, managed by
  # `claude mcp add --scope user` (it does not read ~/.claude/mcp.json).
  if command -v claude &>/dev/null; then
    claude mcp remove --scope user json2pptx >/dev/null 2>&1 || true
    if "${MCP_ADD[@]}" >/dev/null; then
      echo "    Registered json2pptx with Claude Code (user scope)"
    else
      echo "    WARNING: 'claude mcp add' failed. Register manually with:"
      echo "      ${MCP_ADD[*]}"
    fi
  else
    echo "    Claude Code CLI ('claude') not found on PATH. Register the server with:"
    echo "      ${MCP_ADD[*]}"
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
  echo "  Skills:    ~/.claude/skills/*/"
fi
if [[ "$SKIP_MCP" == false ]]; then
  echo "  MCP:       json2pptx (Claude Code user scope; see: claude mcp get json2pptx)"
fi

# PATH warning
if [[ ":$PATH:" != *":$PREFIX/bin:"* ]]; then
  echo ""
  echo "WARNING: $PREFIX/bin is not in your PATH."
  echo "         Add to your shell profile:"
  echo ""
  echo "    export PATH=\"$PREFIX/bin:\$PATH\""
fi
