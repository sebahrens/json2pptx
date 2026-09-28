#!/usr/bin/env bash
# shipped-templates.sh — the single source of truth for which templates ship.
#
# The list is derived from the //go:embed directive in templates/embed.go, so
# the binary, `make install`, `make dist-*`, install.sh and install.ps1 all
# agree. A local, gitignored template (e.g. templates/p-style.pptx) is never
# part of that directive and therefore never ships.
#
# Usage:
#   scripts/shipped-templates.sh                 print shipped template names, one per line
#   scripts/shipped-templates.sh --stage <dir>   copy shipped .pptx + previews/<name> into <dir>
#   scripts/shipped-templates.sh --check <dir>   fail if <dir> holds any template (or
#                                                preview dir) not in the list, or lacks one
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EMBED_GO="$ROOT/templates/embed.go"

shipped_names() {
  local names
  names="$(awk '/^\/\/go:embed / { for (i = 2; i <= NF; i++) if ($i ~ /\.pptx$/) { sub(/\.pptx$/, "", $i); print $i } }' "$EMBED_GO" | sort -u)"
  if [[ -z "$names" ]]; then
    echo "ERROR: no .pptx entries found in the //go:embed directive of $EMBED_GO" >&2
    exit 1
  fi
  printf '%s\n' "$names"
}

is_shipped() {
  local name="$1" s
  for s in $SHIPPED; do
    [[ "$s" == "$name" ]] && return 0
  done
  return 1
}

stage() {
  local dst="$1" name
  mkdir -p "$dst"
  for name in $SHIPPED; do
    cp "$ROOT/templates/$name.pptx" "$dst/"
    if [[ -d "$ROOT/templates/previews/$name" ]]; then
      mkdir -p "$dst/previews"
      cp -R "$ROOT/templates/previews/$name" "$dst/previews/"
    fi
  done
}

check() {
  local dir="$1" f name bad=0
  if [[ ! -d "$dir" ]]; then
    echo "ERROR: template staging dir not found: $dir" >&2
    exit 1
  fi
  shopt -s nullglob
  for f in "$dir"/*.pptx "$dir"/*.potx; do
    name="$(basename "$f")"
    name="${name%.*}"
    if ! is_shipped "$name"; then
      echo "ERROR: $f is not listed in templates/embed.go and must not ship" >&2
      bad=1
    fi
  done
  for f in "$dir"/previews/*/; do
    name="$(basename "$f")"
    if ! is_shipped "$name"; then
      echo "ERROR: ${f%/} is a preview dir for a template not listed in templates/embed.go" >&2
      bad=1
    fi
  done
  for name in $SHIPPED; do
    if [[ ! -f "$dir/$name.pptx" ]]; then
      echo "ERROR: shipped template $name.pptx is missing from $dir" >&2
      bad=1
    fi
  done
  shopt -u nullglob
  if [[ "$bad" -ne 0 ]]; then
    exit 1
  fi
  echo "    staged templates match templates/embed.go ($(printf '%s\n' $SHIPPED | wc -l | tr -d ' ') templates)"
}

SHIPPED="$(shipped_names)"

case "${1:-}" in
  "")      printf '%s\n' "$SHIPPED" ;;
  --stage) stage "${2:?--stage needs a destination dir}" ;;
  --check) check "${2:?--check needs a staging dir}" ;;
  *)       echo "usage: $0 [--stage <dir> | --check <dir>]" >&2; exit 2 ;;
esac
