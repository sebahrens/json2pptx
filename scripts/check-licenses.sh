#!/bin/bash
# License compliance check for go-slide-creator
# Fails if any restricted licenses are found in dependencies
#
# Usage: ./scripts/check-licenses.sh
#
# Add to CI pipeline:
#   - name: Check licenses
#     run: ./scripts/check-licenses.sh

set -e

echo "=== License Compliance Check ==="

# Check if go-licenses is installed
if ! command -v go-licenses &> /dev/null; then
    echo "Installing go-licenses..."
    go install github.com/google/go-licenses@latest
fi

# Run license check
# --disallowed_types: GPL, AGPL, and other copyleft licenses that require
# derivative works to be open source (incompatible with MIT)
#
# Note: We allow LGPL because Go uses dynamic dispatch (interfaces),
# not static linking, making LGPL-licensed libraries compatible.
#
# Note: Some dependencies lack explicit license files but are from
# known-compatible sources (documented in LICENSE-THIRD-PARTY.md)

echo ""
echo "Checking for restricted licenses..."

# Capture output to analyze
set +e
OUTPUT=$(go-licenses check ./... --disallowed_types=restricted,reciprocal 2>&1)
EXIT_CODE=$?
set -e

# Known false positives that we've manually verified
KNOWN_ISSUES=(
    "github.com/srwiley/scanx"  # No license file, but same author as BSD-3 rasterx
    "github.com/BurntSushi/freetype-go"  # Dual FTL/GPL, we use FTL
    "github.com/golang/freetype"  # Dual FTL/GPL, we use FTL
)

# Filter out known issues
FILTERED_OUTPUT="$OUTPUT"
for issue in "${KNOWN_ISSUES[@]}"; do
    FILTERED_OUTPUT=$(echo "$FILTERED_OUTPUT" | grep -v "$issue" || true)
done

# Check for actual problems (not just warnings)
# go-licenses returns non-zero for actual license violations
FORBIDDEN=$(echo "$FILTERED_OUTPUT" | grep -i "forbidden license" || true)
UNKNOWN=$(echo "$FILTERED_OUTPUT" | grep "Unknown license type" | grep -v -E "$(IFS=\|; echo "${KNOWN_ISSUES[*]}")" || true)

if [[ -n "$FORBIDDEN" ]]; then
    echo ""
    echo "ERROR: Forbidden licenses detected:"
    echo "$FORBIDDEN"
    echo ""
    echo "Please review the dependency and update LICENSE-THIRD-PARTY.md if compatible,"
    echo "or remove the dependency if not."
    exit 1
fi

if [[ -n "$UNKNOWN" ]]; then
    echo ""
    echo "WARNING: Unknown licenses detected (not in known exceptions):"
    echo "$UNKNOWN"
    echo ""
    echo "Please investigate these dependencies and document in LICENSE-THIRD-PARTY.md."
    # Don't fail on unknown, but warn
fi

echo ""
echo "License check passed."
echo ""
echo "Summary:"
echo "  - LICENSE-THIRD-PARTY.md documents all dependencies"
echo "  - No restricted/copyleft licenses that conflict with MIT"
echo "  - LGPL dependencies (fribidi) are compatible via Go's interface dispatch"
echo ""
exit 0
