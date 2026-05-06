#!/usr/bin/env bash
# check-skill-sync.sh — Fail when agent-facing surface changes lack skill+docs updates.
#
# Usage:
#   scripts/check-skill-sync.sh [base_ref]
#
# base_ref defaults to origin/main. In CI, pass ${{ github.event.pull_request.base.sha }}
# or the appropriate merge base.
#
# Exit 0 = pass, Exit 1 = sync violation detected.

set -euo pipefail

BASE_REF="${1:-origin/main}"

# Get the list of changed files relative to the base.
# For PRs this is the merge-base diff; for pushes we compare HEAD~1.
if git rev-parse "$BASE_REF" >/dev/null 2>&1; then
  CHANGED_FILES=$(git diff --name-only "$BASE_REF"...HEAD 2>/dev/null || git diff --name-only "$BASE_REF" HEAD)
else
  CHANGED_FILES=$(git diff --name-only HEAD~1 HEAD 2>/dev/null || echo "")
fi

if [ -z "$CHANGED_FILES" ]; then
  echo "skill-sync: no changed files detected, skipping."
  exit 0
fi

# --- Check for Skill-Sync-Exempt trailer ---
# Look in all commits since base for the trailer with reason >= 20 chars.
EXEMPT=false
COMMITS=$(git rev-list "$BASE_REF"..HEAD 2>/dev/null || echo "HEAD")
for commit in $COMMITS; do
  trailer=$(git log -1 --format='%(trailers:key=Skill-Sync-Exempt,valueonly)' "$commit" 2>/dev/null || true)
  if [ -n "$trailer" ] && [ "${#trailer}" -ge 20 ]; then
    echo "skill-sync: exempt via trailer in $commit: $trailer"
    EXEMPT=true
    break
  fi
done

if $EXEMPT; then
  exit 0
fi

# --- Heuristic: detect agent-facing surface changes ---
AGENT_SURFACE_PATTERNS=(
  # CLI/MCP command response structs
  "cmd/json2pptx/.*_cmd\\.go"
  "cmd/json2pptx/skill_info\\.go"
  # Fit-report finding emission
  "cmd/json2pptx/fit_report\\.go"
  "internal/textfit/.*\\.go"
  # Pattern overrides and schemas
  "internal/patterns/.*\\.go"
  # Public types reachable from CLI/MCP responses
  "internal/types/.*\\.go"
  # Pipeline (generation response shapes)
  "internal/pipeline/.*\\.go"
)

TRIGGERED_FILES=()
for file in $CHANGED_FILES; do
  for pattern in "${AGENT_SURFACE_PATTERNS[@]}"; do
    if echo "$file" | grep -qE "^${pattern}$"; then
      TRIGGERED_FILES+=("$file")
      break
    fi
  done
done

if [ ${#TRIGGERED_FILES[@]} -eq 0 ]; then
  echo "skill-sync: no agent-facing surface changes detected, skipping."
  exit 0
fi

# --- Check that skill and docs files were updated ---
SKILL_FILE="skills/generate-deck/SKILL.md"
DOC_FILES=(
  "docs/FIT_FINDINGS.md"
  "docs/PATTERNS.md"
  "docs/STYLE_DEFAULTS.md"
  "docs/INPUT_FORMAT.md"
)

SKILL_UPDATED=false
DOCS_UPDATED=false

for file in $CHANGED_FILES; do
  if [ "$file" = "$SKILL_FILE" ]; then
    SKILL_UPDATED=true
  fi
  for doc in "${DOC_FILES[@]}"; do
    if [ "$file" = "$doc" ]; then
      DOCS_UPDATED=true
      break
    fi
  done
done

ERRORS=()
if ! $SKILL_UPDATED; then
  ERRORS+=("$SKILL_FILE was not updated")
fi
if ! $DOCS_UPDATED; then
  ERRORS+=("No docs/ file was updated (expected one of: ${DOC_FILES[*]})")
fi

if [ ${#ERRORS[@]} -gt 0 ]; then
  echo ""
  echo "=========================================="
  echo "  SKILL-SYNC CHECK FAILED"
  echo "=========================================="
  echo ""
  echo "Agent-facing surface files changed:"
  for f in "${TRIGGERED_FILES[@]}"; do
    echo "  - $f"
  done
  echo ""
  echo "Missing updates:"
  for e in "${ERRORS[@]}"; do
    echo "  ✗ $e"
  done
  echo ""
  echo "The skill-code-docs sync policy requires that changes to agent-facing"
  echo "surfaces are accompanied by updates to SKILL.md and relevant docs/."
  echo ""
  echo "To fix: update the skill and docs files in the same commit."
  echo ""
  echo "If this is a pure refactor with no agent-visible behavior change,"
  echo "add a commit trailer:"
  echo "  Skill-Sync-Exempt: <reason (>= 20 chars)>"
  echo ""
  echo "See CLAUDE.md 'Skill, Docs, and Code Sync' for the full policy."
  exit 1
fi

echo "skill-sync: agent-facing changes detected and skill+docs updated. ✓"
exit 0
