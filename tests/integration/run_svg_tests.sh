#!/usr/bin/env bash
# =============================================================================
# SVG Integration Test Runner
# =============================================================================
# Exercises all SVG diagram types by rendering YAML fixtures through the svggen
# CLI tool.
#
# Usage:
#   ./tests/integration/run_svg_tests.sh
#   ./tests/integration/run_svg_tests.sh --verbose
#   ./tests/integration/run_svg_tests.sh --keep-output
#
# Options:
#   --verbose      Show detailed output from svggen
#   --keep-output  Do not delete generated SVG files after tests
#   --filter PAT   Only run fixtures matching pattern (e.g., "chart" or "16_")
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/svg_fixtures"
OUTPUT_DIR="$SCRIPT_DIR/svg_output"
BINARY="$PROJECT_ROOT/bin/svggen"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No color

# Options
VERBOSE=false
KEEP_OUTPUT=false
FILTER=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --verbose)
            VERBOSE=true
            shift
            ;;
        --keep-output)
            KEEP_OUTPUT=true
            shift
            ;;
        --filter)
            FILTER="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--verbose] [--keep-output] [--filter PATTERN]"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Counters
TOTAL=0
PASSED=0
FAILED=0
SKIPPED=0

# Track failed tests for summary
declare -a FAILED_TESTS=()

# --- Helper Functions ---

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
}

# Validate that output is well-formed SVG (XML with svg root element)
validate_svg() {
    local file="$1"

    # Check file exists and is non-empty
    if [[ ! -f "$file" ]]; then
        echo "File does not exist: $file"
        return 1
    fi

    local size
    size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)
    if [[ "$size" -eq 0 ]]; then
        echo "File is empty: $file"
        return 1
    fi

    # Check it starts with XML or SVG declaration
    local first_line
    first_line=$(head -c 500 "$file")
    if ! echo "$first_line" | grep -q '<svg'; then
        echo "Output does not contain <svg element: $file"
        return 1
    fi

    # Check for closing </svg> tag
    if ! grep -q '</svg>' "$file"; then
        echo "Output missing closing </svg> tag: $file"
        return 1
    fi

    # Use xmllint if available for full validation
    if command -v xmllint > /dev/null 2>&1; then
        if ! xmllint --noout "$file" 2>/dev/null; then
            echo "XML validation failed (xmllint): $file"
            return 1
        fi
    fi

    return 0
}

# Run a single fixture test
run_fixture() {
    local fixture="$1"
    local basename
    basename=$(basename "$fixture")
    basename="${basename%.*}" # remove extension
    local output_svg="$OUTPUT_DIR/${basename}.svg"

    TOTAL=$((TOTAL + 1))

    # Run svggen render
    local cmd_args=(
        render
        -i "$fixture"
        -o "$output_svg"
    )

    if $VERBOSE; then
        log_info "Running: $BINARY ${cmd_args[*]}"
    fi

    local exit_code=0
    local cmd_output
    cmd_output=$("$BINARY" "${cmd_args[@]}" 2>&1) || exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        log_fail "$basename - svggen exited with code $exit_code"
        if $VERBOSE && [[ -n "$cmd_output" ]]; then
            echo "  Output: $cmd_output"
        fi
        FAILED=$((FAILED + 1))
        FAILED_TESTS+=("$basename: exit code $exit_code - $cmd_output")
        return
    fi

    # Validate the SVG output
    local validation_error
    if validation_error=$(validate_svg "$output_svg" 2>&1); then
        log_pass "$basename"
        PASSED=$((PASSED + 1))
    else
        log_fail "$basename - SVG validation failed: $validation_error"
        FAILED=$((FAILED + 1))
        FAILED_TESTS+=("$basename: $validation_error")
    fi
}

# --- Main ---

echo "============================================================"
echo "  SVG Integration Tests"
echo "============================================================"
echo ""

# Step 1: Build the svggen binary
log_info "Building svggen binary..."
if ! (cd "$PROJECT_ROOT/svggen" && go build -o "$BINARY" ./cmd/svggen/); then
    log_fail "Failed to build svggen binary"
    exit 1
fi
log_pass "Binary built successfully: $BINARY"
echo ""

# Step 2: List registered diagram types
log_info "Registered diagram types:"
"$BINARY" types 2>/dev/null | head -40 || true
echo ""

# Step 3: Prepare output directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Step 4: Run all fixtures
log_info "Running fixtures from $FIXTURES_DIR"
echo ""

for fixture in "$FIXTURES_DIR"/*.yaml "$FIXTURES_DIR"/*.yml "$FIXTURES_DIR"/*.json; do
    [[ -f "$fixture" ]] || continue

    # Apply filter if specified
    if [[ -n "$FILTER" ]] && ! echo "$(basename "$fixture")" | grep -qi "$FILTER"; then
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    run_fixture "$fixture"
done

echo ""

# Step 5: Summary
echo "============================================================"
echo "  Results"
echo "============================================================"
echo ""
echo -e "  Total:    $TOTAL"
echo -e "  ${GREEN}Passed:   $PASSED${NC}"
if [[ $FAILED -gt 0 ]]; then
    echo -e "  ${RED}Failed:   $FAILED${NC}"
fi
if [[ $SKIPPED -gt 0 ]]; then
    echo -e "  Skipped:  $SKIPPED"
fi
echo ""

# Show failed tests
if [[ ${#FAILED_TESTS[@]} -gt 0 ]]; then
    echo -e "${RED}Failed tests:${NC}"
    for t in "${FAILED_TESTS[@]}"; do
        echo "  - $t"
    done
    echo ""
fi

# Cleanup
if ! $KEEP_OUTPUT; then
    rm -rf "$OUTPUT_DIR"
    log_info "Output directory cleaned up"
else
    log_info "Output preserved in $OUTPUT_DIR"
fi

# Exit code
if [[ $FAILED -gt 0 ]]; then
    exit 1
fi
exit 0
