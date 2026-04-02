#!/bin/bash
#===============================================================================
# Unit Test Runner with Beads Integration
#===============================================================================
#
# DESCRIPTION:
#   Runs the Go test suite with build validation and linting. Optionally
#   creates Beads issues for any failures. This is for unit/integration tests
#   only - use e2e_visual_test.sh for visual inspection.
#
# USAGE:
#   # Run all tests
#   ./scripts/run_tests.sh
#
#   # Run specific package tests
#   ./scripts/run_tests.sh ./internal/generator/...
#
#   # Auto-create Beads issues for failures
#   CREATE_BEADS_ISSUES=true ./scripts/run_tests.sh
#
#   # Skip linting (faster)
#   SKIP_LINT=true ./scripts/run_tests.sh
#
# ENVIRONMENT VARIABLES:
#   CREATE_BEADS_ISSUES - "true" to auto-create Beads issues for failures
#   SKIP_LINT           - "true" to skip golangci-lint
#
# STEPS EXECUTED:
#   1. Build check   (go build ./...)
#   2. Unit tests    (go test <packages> -v)
#   3. Linting       (golangci-lint run ./...) [unless SKIP_LINT=true]
#
# OUTPUT:
#   Exits with:
#     0 - All checks passed
#     1 - Tests or lint failed
#     2 - Build failed
#
# SEE ALSO:
#   - scripts/e2e_visual_test.sh  - Visual inspection (post unit tests)
#   - AGENTS.md                   - Full validation workflow
#
#===============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Parse arguments
TEST_PACKAGES="${1:-./...}"
SKIP_LINT="${SKIP_LINT:-false}"

echo "=== Go Slide Creator Test Runner ==="
echo "Project: ${PROJECT_DIR}"
echo "Packages: ${TEST_PACKAGES}"
echo ""

# Track failures
BUILD_FAILED=false
TEST_FAILED=false
LINT_FAILED=false
FAILED_TESTS=()
LINT_ERRORS=()

# Step 1: Build
echo "Step 1: Building..."
if ! go build ./... 2>&1; then
    echo "❌ Build FAILED"
    BUILD_FAILED=true
else
    echo "✅ Build passed"
fi
echo ""

# Step 2: Run Tests
echo "Step 2: Running tests..."
TEST_OUTPUT=$(mktemp)
if ! go test ${TEST_PACKAGES} -v 2>&1 | tee "${TEST_OUTPUT}"; then
    echo "❌ Tests FAILED"
    TEST_FAILED=true

    # Extract failed test names
    while IFS= read -r line; do
        if [[ "$line" =~ ^---\ FAIL:\ (.+)\ \( ]]; then
            FAILED_TESTS+=("${BASH_REMATCH[1]}")
        fi
    done < "${TEST_OUTPUT}"
else
    echo "✅ Tests passed"
fi
rm -f "${TEST_OUTPUT}"
echo ""

# Step 3: Lint (optional)
if [ "${SKIP_LINT}" != "true" ]; then
    echo "Step 3: Running linter..."
    LINT_OUTPUT=$(mktemp)
    if ! golangci-lint run ./... 2>&1 | tee "${LINT_OUTPUT}"; then
        echo "❌ Lint FAILED"
        LINT_FAILED=true

        # Extract lint errors (first 10)
        while IFS= read -r line; do
            if [[ "$line" =~ ^[a-zA-Z] ]]; then
                LINT_ERRORS+=("$line")
                if [ ${#LINT_ERRORS[@]} -ge 10 ]; then
                    break
                fi
            fi
        done < "${LINT_OUTPUT}"
    else
        echo "✅ Lint passed"
    fi
    rm -f "${LINT_OUTPUT}"
    echo ""
fi

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "                    SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Build:  $([ "$BUILD_FAILED" = true ] && echo "❌ FAILED" || echo "✅ Passed")"
echo "Tests:  $([ "$TEST_FAILED" = true ] && echo "❌ FAILED (${#FAILED_TESTS[@]} tests)" || echo "✅ Passed")"
if [ "${SKIP_LINT}" != "true" ]; then
    echo "Lint:   $([ "$LINT_FAILED" = true ] && echo "❌ FAILED (${#LINT_ERRORS[@]} errors)" || echo "✅ Passed")"
fi
echo ""

# Create Beads issues for failures
if [ "${CREATE_BEADS_ISSUES:-false}" = "true" ]; then
    if [ "$BUILD_FAILED" = true ] || [ "$TEST_FAILED" = true ] || [ "$LINT_FAILED" = true ]; then
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "              CREATING BEADS ISSUES"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

        # Create issue for build failure
        if [ "$BUILD_FAILED" = true ]; then
            EXISTING=$(bd list --status=open 2>/dev/null | grep -c "Build failure" || echo "0")
            if [ "${EXISTING}" = "0" ]; then
                echo "Creating build failure issue..."
                ISSUE_ID=$(bd create --title="Build failure: go build ./... fails" \
                    --type=bug \
                    --priority=0 \
                    --body="The project fails to build.

## Reproduction
\`\`\`bash
go build ./...
\`\`\`

## Investigation
1. Check for syntax errors in recently modified files
2. Check for missing imports
3. Check for type mismatches

## Files to Check
- Recently modified \`.go\` files
- Check \`git diff\` for recent changes" \
                    --labels="build,test-failure" 2>/dev/null || echo "")
                if [ -n "${ISSUE_ID}" ]; then
                    echo "  ✓ Created: ${ISSUE_ID} [P0]"
                fi
            else
                echo "  ⏭ Build failure issue already exists"
            fi
        fi

        # Create issues for test failures
        if [ "$TEST_FAILED" = true ] && [ ${#FAILED_TESTS[@]} -gt 0 ]; then
            for test_name in "${FAILED_TESTS[@]}"; do
                # Check if issue already exists for this test
                EXISTING=$(bd list --status=open 2>/dev/null | grep -c "Test: ${test_name}" || echo "0")
                if [ "${EXISTING}" = "0" ]; then
                    echo "Creating issue for failing test: ${test_name}..."

                    # Extract package name from test name (format: TestName or TestName/SubTest)
                    PACKAGE=$(echo "${test_name}" | cut -d'/' -f1)

                    ISSUE_ID=$(bd create --title="Test: ${test_name} fails" \
                        --type=bug \
                        --priority=1 \
                        --body="Test **${test_name}** is failing.

## Reproduction
\`\`\`bash
go test -v -run ${PACKAGE} ./...
\`\`\`

## Investigation
1. Run the test in isolation to see the error
2. Check the test expectations
3. Check the code being tested

## Common Causes
- Recent code changes broke expected behavior
- Test data or fixtures changed
- Race condition or timing issue" \
                        --labels="test-failure,${PACKAGE}" 2>/dev/null || echo "")
                    if [ -n "${ISSUE_ID}" ]; then
                        echo "  ✓ Created: ${ISSUE_ID} [P1]"
                    fi
                else
                    echo "  ⏭ Test issue already exists: ${test_name}"
                fi
            done
        fi

        # Create issue for lint failures
        if [ "$LINT_FAILED" = true ] && [ ${#LINT_ERRORS[@]} -gt 0 ]; then
            EXISTING=$(bd list --status=open 2>/dev/null | grep -c "Lint errors" || echo "0")
            if [ "${EXISTING}" = "0" ]; then
                echo "Creating lint failure issue..."

                # Format lint errors for the issue body
                LINT_ERRORS_TEXT=""
                for error in "${LINT_ERRORS[@]}"; do
                    LINT_ERRORS_TEXT="${LINT_ERRORS_TEXT}\n- ${error}"
                done

                ISSUE_ID=$(bd create --title="Lint errors: golangci-lint run fails" \
                    --type=task \
                    --priority=2 \
                    --body="Linting is failing with errors.

## Reproduction
\`\`\`bash
golangci-lint run ./...
\`\`\`

## Errors Found
${LINT_ERRORS_TEXT}

## Fix
Address each lint error according to the linter's suggestions." \
                    --labels="lint,code-quality" 2>/dev/null || echo "")
                if [ -n "${ISSUE_ID}" ]; then
                    echo "  ✓ Created: ${ISSUE_ID} [P2]"
                fi
            else
                echo "  ⏭ Lint failure issue already exists"
            fi
        fi

        echo ""
        echo "Run 'bd ready' to see all issues"
    fi
fi

# Exit with appropriate code
if [ "$BUILD_FAILED" = true ]; then
    exit 2
elif [ "$TEST_FAILED" = true ]; then
    exit 1
elif [ "$LINT_FAILED" = true ]; then
    exit 1
else
    exit 0
fi
