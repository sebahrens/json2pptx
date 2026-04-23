#!/usr/bin/env bash
# =============================================================================
# Quality Eval Harness Runner
# =============================================================================
# Computes mechanical quality metrics for JSON deck fixtures and example decks.
# Optionally runs visual QA (requires ANTHROPIC_API_KEY and LibreOffice).
#
# Usage:
#   ./tests/quality/run.sh              # Mechanical metrics only
#   ./tests/quality/run.sh --visual-qa  # Include Haiku visual QA (needs API key)
#   ./tests/quality/run.sh --update-baseline  # Update baseline after review
#
# Output:
#   tests/quality/results.csv — current run metrics
#   tests/quality/baseline.csv — reference baseline (create with --update-baseline)
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VISUAL_QA=false
UPDATE_BASELINE=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --visual-qa)
            VISUAL_QA=true
            shift
            ;;
        --update-baseline)
            UPDATE_BASELINE=true
            shift
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

echo "=== Quality Eval Harness ==="
echo "Project root: $PROJECT_ROOT"
echo ""

# Step 1: Run Go test harness (mechanical metrics + fit-report).
echo "--- Running mechanical metrics ---"
cd "$PROJECT_ROOT"
go test ./tests/quality/ -v -count=1 -timeout=120s 2>&1 | tee "$SCRIPT_DIR/test-output.log"

if [[ -f "$SCRIPT_DIR/results.csv" ]]; then
    echo ""
    echo "--- Results ---"
    column -t -s',' "$SCRIPT_DIR/results.csv" | head -30
fi

# Step 2: Update baseline if requested.
if [[ "$UPDATE_BASELINE" == "true" ]]; then
    if [[ -f "$SCRIPT_DIR/results.csv" ]]; then
        cp "$SCRIPT_DIR/results.csv" "$SCRIPT_DIR/baseline.csv"
        echo ""
        echo "Baseline updated: $SCRIPT_DIR/baseline.csv"
    else
        echo "ERROR: No results.csv to use as baseline" >&2
        exit 1
    fi
fi

# Step 3: Visual QA (optional — requires ANTHROPIC_API_KEY + LibreOffice).
if [[ "$VISUAL_QA" == "true" ]]; then
    if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
        echo "SKIP: Visual QA requires ANTHROPIC_API_KEY" >&2
        exit 0
    fi

    if ! command -v soffice &>/dev/null; then
        echo "SKIP: Visual QA requires LibreOffice (soffice)" >&2
        exit 0
    fi

    BINARY="$PROJECT_ROOT/bin/json2pptx"
    if [[ ! -x "$BINARY" ]]; then
        echo "Building json2pptx..."
        (cd "$PROJECT_ROOT" && make)
    fi

    PPTX_OUT="$SCRIPT_DIR/pptx_output"
    SLIDE_OUT="$SCRIPT_DIR/slide_images"
    mkdir -p "$PPTX_OUT" "$SLIDE_OUT"

    echo ""
    echo "--- Generating PPTX files ---"
    for fixture in "$SCRIPT_DIR/fixtures/"*.json; do
        name="$(basename "$fixture" .json)"
        echo "  Generating: $name"
        "$BINARY" generate \
            -json "$fixture" \
            -template midnight-blue \
            -templates-dir "$PROJECT_ROOT/templates" \
            -output "$PPTX_OUT" 2>/dev/null || true
    done

    echo ""
    echo "--- Converting to images ---"
    for pptx in "$PPTX_OUT/"*.pptx; do
        name="$(basename "$pptx" .pptx)"
        echo "  Converting: $name"
        "$PROJECT_ROOT/bin/pptx2jpg" \
            -input "$pptx" \
            -output "$SLIDE_OUT/$name/" \
            -density 150 2>/dev/null || true
    done

    echo ""
    echo "Visual QA: slide images ready in $SLIDE_OUT/"
    echo "Run the slide-visual-qa skill manually on these images."
fi

echo ""
echo "=== Done ==="
