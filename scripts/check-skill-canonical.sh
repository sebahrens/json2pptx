#!/usr/bin/env bash
# check-skill-canonical.sh — Enforce that skills/ is the single source of truth
# for SKILL.md / TEMPLATE_GUIDE.md files in this repo.
#
# Background (go-slide-creator-hkor):
#   .claude/skills/ and skills/ both used to ship SKILL.md files. They diverged
#   (88KB old copy in .claude/ vs 96KB current in skills/), references in the
#   .claude/ copy pointed at paths that only existed under skills/, and a stale
#   SKILL.md.new sat next to the divergent file. Canonical location is now
#   skills/. The .claude/skills/ tree may exist locally as symlinks to skills/,
#   but no SKILL/TEMPLATE_GUIDE content under .claude/skills/ may be tracked,
#   because tracked copies are what allowed the drift in the first place.
#
# This check fails if:
#   1. Any file under .claude/skills/ is tracked by git.
#   2. The canonical skills/generate-deck/SKILL.md is missing.
#
# Usage:
#   scripts/check-skill-canonical.sh
# Exit 0 = pass, Exit 1 = violation.

set -euo pipefail

ERRORS=()

# 1. The canonical SKILL must exist.
CANONICAL="skills/generate-deck/SKILL.md"
if [ ! -f "$CANONICAL" ]; then
  ERRORS+=("canonical skill missing: $CANONICAL")
fi

# 2. Nothing under .claude/skills/ may be tracked (it's gitignored; tracked
#    files there were the source of the .claude vs skills divergence).
TRACKED_IN_CLAUDE=$(git ls-files -- '.claude/skills/' 2>/dev/null || true)
if [ -n "$TRACKED_IN_CLAUDE" ]; then
  ERRORS+=("files tracked under .claude/skills/ (must live only under skills/):")
  while IFS= read -r f; do
    ERRORS+=("  - $f")
  done <<< "$TRACKED_IN_CLAUDE"
fi

if [ ${#ERRORS[@]} -gt 0 ]; then
  echo ""
  echo "=========================================="
  echo "  SKILL-CANONICAL CHECK FAILED"
  echo "=========================================="
  echo ""
  for e in "${ERRORS[@]}"; do
    echo "  ✗ $e"
  done
  echo ""
  echo "Canonical SKILL.md / TEMPLATE_GUIDE.md files live under skills/."
  echo ".claude/skills/ is gitignored and may contain only local symlinks"
  echo "into skills/. See go-slide-creator-hkor for context."
  exit 1
fi

echo "skill-canonical: skills/ is the single source of truth. ✓"
exit 0
