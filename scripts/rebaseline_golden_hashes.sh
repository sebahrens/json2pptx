#!/usr/bin/env bash
# rebaseline_golden_hashes.sh — Regenerate cmd/json2pptx/testdata/golden_hashes.json.
#
# Per go-slide-creator-l50r, TestDeterministicCorpus pins the sha256 of
# examples/basic-deck.json rendered against every bundled template. Run this
# script after an intentional change to PPTX byte output (zip ordering tweak,
# default-style migration, template repair, etc.) to refresh the goldens.
#
# Run on linux for the canonical CI hashes — text-fit budgets are
# cross-platform-deterministic by design (Liberation Sans embedded metrics),
# but new code paths can leak platform-specific bytes; if a future regression
# does, regenerating here keeps the file authoritative.
#
# Usage:
#   scripts/rebaseline_golden_hashes.sh
#
# The script:
#   1. Builds the json2pptx binary into a scratch dir
#   2. Renders examples/basic-deck.json against every template listed in
#      determinismTemplates (kept in sync with cmd/json2pptx/determinism_corpus_test.go)
#   3. Writes the new sha256s to cmd/json2pptx/testdata/golden_hashes.json
#   4. Shows a unified diff against the previous file

set -euo pipefail

cd "$(dirname "$0")/.."

REPO_ROOT="$(pwd)"
GOLDEN_PATH="cmd/json2pptx/testdata/golden_hashes.json"
EXAMPLE="examples/basic-deck.json"
TEMPLATES_DIR="templates"

# Must mirror determinismTemplates in cmd/json2pptx/determinism_corpus_test.go.
TEMPLATES=(forest-green midnight-blue modern-template warm-coral)

SCRATCH="$(mktemp -d)"
trap 'rm -rf "$SCRATCH"' EXIT

BIN="$SCRATCH/json2pptx"
echo "Building json2pptx → $BIN"
go build -o "$BIN" ./cmd/json2pptx

# Match the env the test sets so any future timestamp-using code path picks up
# the same epoch we record the hash for.
export SOURCE_DATE_EPOCH=1700000000
export TZ=UTC

NEW_JSON="$SCRATCH/golden_hashes.json"
{
    echo "{"
    for i in "${!TEMPLATES[@]}"; do
        tmpl="${TEMPLATES[$i]}"
        out="$SCRATCH/${tmpl}.pptx"
        "$BIN" generate \
            -json "$EXAMPLE" \
            -template "$tmpl" \
            -templates-dir "$TEMPLATES_DIR" \
            -output "$out" >/dev/null 2>&1
        hash=$(shasum -a 256 "$out" | awk '{print $1}')
        sep=","
        if (( i == ${#TEMPLATES[@]} - 1 )); then
            sep=""
        fi
        printf '  "%s": "%s"%s\n' "$tmpl" "$hash" "$sep"
    done
    echo "}"
} > "$NEW_JSON"

if [[ -f "$REPO_ROOT/$GOLDEN_PATH" ]]; then
    echo
    echo "Diff against existing $GOLDEN_PATH:"
    diff -u "$REPO_ROOT/$GOLDEN_PATH" "$NEW_JSON" || true
fi

cp "$NEW_JSON" "$REPO_ROOT/$GOLDEN_PATH"
echo
echo "Wrote $GOLDEN_PATH"
