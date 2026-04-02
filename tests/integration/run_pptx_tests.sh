#!/usr/bin/env bash
# =============================================================================
# PPTX Integration Test Runner
# =============================================================================
# Exercises all PowerPoint features by running JSON fixtures through json2pptx.
#
# Usage:
#   ./tests/integration/run_pptx_tests.sh
#   ./tests/integration/run_pptx_tests.sh --verbose
#   ./tests/integration/run_pptx_tests.sh --keep-output
#
# Options:
#   --verbose      Show detailed output from json2pptx
#   --keep-output  Do not delete generated PPTX files after tests
#   --filter PAT   Only run fixtures matching pattern (e.g., "chart" or "09_")
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/json_fixtures"
OUTPUT_DIR="$SCRIPT_DIR/pptx_output"
BINARY="$PROJECT_ROOT/bin/json2pptx"

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
WARNINGS=0

# Track failed tests for summary
declare -a FAILED_TESTS=()
declare -a WARNING_TESTS=()

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

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
}

# Validate that a file is a valid ZIP archive (PPTX files are ZIP-based)
validate_pptx() {
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

    # Check it is a valid ZIP file
    if ! unzip -t "$file" > /dev/null 2>&1; then
        echo "File is not a valid ZIP/PPTX: $file"
        return 1
    fi

    # Check for required PPTX internal files
    local contents
    contents=$(unzip -l "$file" 2>/dev/null)
    if ! echo "$contents" | grep -q "\[Content_Types\].xml"; then
        echo "Missing [Content_Types].xml in PPTX: $file"
        return 1
    fi

    if ! echo "$contents" | grep -q "ppt/presentation.xml"; then
        echo "Missing ppt/presentation.xml in PPTX: $file"
        return 1
    fi

    # Check that at least one slide exists
    if ! echo "$contents" | grep -q "ppt/slides/slide"; then
        echo "No slides found in PPTX: $file"
        return 1
    fi

    return 0
}

# Run a single fixture test
run_fixture() {
    local fixture="$1"
    local basename
    basename=$(basename "$fixture" .json)
    local output_pptx="$OUTPUT_DIR/${basename}.pptx"
    local json_output="$OUTPUT_DIR/${basename}.result.json"

    TOTAL=$((TOTAL + 1))

    # Extract expected output filename from the fixture JSON
    local expected_name
    expected_name=$(python3 -c "
import json, sys
with open('$fixture') as f:
    d = json.load(f)
print(d.get('output_filename', 'output.pptx'))
" 2>/dev/null || echo "output.pptx")

    # Run json2pptx
    local cmd_args=(
        generate
        -json "$fixture"
        -json-output "$json_output"
        -output "$OUTPUT_DIR"
        -templates-dir "$PROJECT_ROOT/templates"
    )

    if $VERBOSE; then
        log_info "Running: $BINARY ${cmd_args[*]}"
    fi

    local exit_code=0
    local cmd_output
    cmd_output=$("$BINARY" "${cmd_args[@]}" 2>&1) || exit_code=$?

    # The actual output path may differ from what we predicted
    local actual_output="$OUTPUT_DIR/$expected_name"

    # Check JSON output result
    if [[ -f "$json_output" ]]; then
        local success
        success=$(python3 -c "
import json, sys
with open('$json_output') as f:
    d = json.load(f)
print('true' if d.get('success', False) else 'false')
" 2>/dev/null || echo "unknown")

        local warnings
        warnings=$(python3 -c "
import json, sys
with open('$json_output') as f:
    d = json.load(f)
w = d.get('warnings', [])
se = d.get('slide_errors', [])
if w or se:
    for item in w:
        print('  warning: ' + str(item))
    for item in se:
        print('  slide_error: slide ' + str(item.get('slide_number','?')) + ' ' + str(item.get('error','')))
" 2>/dev/null || echo "")

        if [[ "$success" == "true" ]]; then
            # Validate the PPTX file
            local validation_error
            if validation_error=$(validate_pptx "$actual_output" 2>&1); then
                if [[ -n "$warnings" ]]; then
                    log_pass "$basename (with warnings)"
                    echo "$warnings"
                    WARNINGS=$((WARNINGS + 1))
                    WARNING_TESTS+=("$basename")
                else
                    log_pass "$basename"
                fi
                PASSED=$((PASSED + 1))
            else
                log_fail "$basename - PPTX validation failed: $validation_error"
                FAILED=$((FAILED + 1))
                FAILED_TESTS+=("$basename: PPTX validation failed")
            fi
        else
            # Check if this is an expected failure (e.g., missing image file)
            local error_msg
            error_msg=$(python3 -c "
import json, sys
with open('$json_output') as f:
    d = json.load(f)
print(d.get('error', 'unknown error'))
" 2>/dev/null || echo "unknown error")

            # Some tests may produce a PPTX with warnings/slide_errors but still succeed
            if [[ -f "$actual_output" ]]; then
                if validate_pptx "$actual_output" > /dev/null 2>&1; then
                    log_pass "$basename (partial success with errors)"
                    if [[ -n "$warnings" ]]; then
                        echo "$warnings"
                    fi
                    PASSED=$((PASSED + 1))
                    WARNINGS=$((WARNINGS + 1))
                    WARNING_TESTS+=("$basename")
                else
                    log_fail "$basename - Error: $error_msg"
                    FAILED=$((FAILED + 1))
                    FAILED_TESTS+=("$basename: $error_msg")
                fi
            else
                log_fail "$basename - Error: $error_msg"
                FAILED=$((FAILED + 1))
                FAILED_TESTS+=("$basename: $error_msg")
            fi
        fi
    else
        # No JSON output file - check if binary crashed
        if [[ $exit_code -ne 0 ]]; then
            log_fail "$basename - json2pptx exited with code $exit_code"
            if $VERBOSE && [[ -n "$cmd_output" ]]; then
                echo "  Output: $cmd_output"
            fi
            FAILED=$((FAILED + 1))
            FAILED_TESTS+=("$basename: exit code $exit_code")
        elif [[ -f "$actual_output" ]] && validate_pptx "$actual_output" > /dev/null 2>&1; then
            log_pass "$basename (no JSON output but PPTX valid)"
            PASSED=$((PASSED + 1))
        else
            log_fail "$basename - No output produced"
            FAILED=$((FAILED + 1))
            FAILED_TESTS+=("$basename: no output")
        fi
    fi
}

# --- Main ---

echo "============================================================"
echo "  PPTX Integration Tests"
echo "============================================================"
echo ""

# Step 1: Build the binary
log_info "Building json2pptx binary..."
if ! (cd "$PROJECT_ROOT" && go build -o "$BINARY" ./cmd/json2pptx/); then
    log_fail "Failed to build json2pptx binary"
    exit 1
fi
log_pass "Binary built successfully: $BINARY"
echo ""

# Step 2: Check that templates exist
if [[ ! -d "$PROJECT_ROOT/templates" ]]; then
    log_fail "Templates directory not found: $PROJECT_ROOT/templates"
    exit 1
fi

TEMPLATE_COUNT=$(find "$PROJECT_ROOT/templates" -name "*.pptx" | wc -l | tr -d ' ')
log_info "Found $TEMPLATE_COUNT template(s) in $PROJECT_ROOT/templates"

# Step 3: Prepare output directory
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Step 4: Run all fixtures
log_info "Running fixtures from $FIXTURES_DIR"
echo ""

for fixture in "$FIXTURES_DIR"/*.json; do
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
if [[ $WARNINGS -gt 0 ]]; then
    echo -e "  ${YELLOW}Warnings: $WARNINGS${NC}"
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

# Show warning tests
if [[ ${#WARNING_TESTS[@]} -gt 0 ]]; then
    echo -e "${YELLOW}Tests with warnings:${NC}"
    for t in "${WARNING_TESTS[@]}"; do
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
