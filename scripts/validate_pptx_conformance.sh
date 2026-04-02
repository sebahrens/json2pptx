#!/bin/bash
#===============================================================================
# PPTX Conformance Validation via LibreOffice
#===============================================================================
#
# DESCRIPTION:
#   Validates that generated PPTX files are conformant by opening them with
#   LibreOffice headless and converting to PDF. This catches malformed XML,
#   missing relationships, broken image references, and invalid table structures
#   that unit tests miss because they only check Go struct correctness.
#
# USAGE:
#   ./scripts/validate_pptx_conformance.sh
#
# ENVIRONMENT:
#   TEMPLATES      - Space-separated template names (default: "warm-coral template_2")
#   FIXTURE        - Path to test fixture (default: testdata/fixtures/ci_validation.md)
#   OUTPUT_DIR     - Output directory (default: test_ci_output)
#
# EXIT CODES:
#   0 - All PPTX files passed conformance validation
#   1 - One or more PPTX files failed
#   2 - Setup error (missing tool, build failure)
#
#===============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

OUTPUT_DIR="${OUTPUT_DIR:-test_ci_output}"
FIXTURE="${FIXTURE:-testdata/fixtures/ci_validation.md}"
TEMPLATES="${TEMPLATES:-template_2 forest-green midnight-blue warm-coral}"

echo "=== PPTX Conformance Validation ==="
echo "Project: ${PROJECT_DIR}"
echo "Fixture: ${FIXTURE}"
echo "Templates: ${TEMPLATES}"
echo ""

# Check for LibreOffice
LIBREOFFICE_CMD=""
if command -v soffice &>/dev/null && [[ "$(uname)" == "Darwin" ]]; then
    LIBREOFFICE_CMD="soffice"
elif command -v libreoffice &>/dev/null; then
    LIBREOFFICE_CMD="libreoffice"
else
    echo "ERROR: LibreOffice not found. Install with:"
    echo "  macOS:  brew install --cask libreoffice"
    echo "  Linux:  sudo apt-get install -y libreoffice-impress"
    exit 2
fi

echo "LibreOffice: ${LIBREOFFICE_CMD}"
echo ""

# Build tools
echo "Step 1: Building tools..."
mkdir -p "${OUTPUT_DIR}"

if ! go build -o "${OUTPUT_DIR}/json2pptx" ./cmd/json2pptx; then
    echo "ERROR: json2pptx build failed"
    exit 2
fi

if ! go build -o "${OUTPUT_DIR}/validatepptx" ./cmd/validatepptx; then
    echo "ERROR: validatepptx build failed"
    exit 2
fi

echo "  Tools built successfully"
echo ""

# Track results
TOTAL=0
PASSED=0
FAILED=0
FAILURES=()

# Process each template
for TEMPLATE_NAME in ${TEMPLATES}; do
    TOTAL=$((TOTAL + 1))
    TEMPLATE_FILE="templates/${TEMPLATE_NAME}.pptx"
    PPTX_OUTPUT="${OUTPUT_DIR}/${TEMPLATE_NAME}_conformance.pptx"
    PDF_OUTPUT="${OUTPUT_DIR}/${TEMPLATE_NAME}_conformance.pdf"

    echo "━━━ Template: ${TEMPLATE_NAME} ━━━"

    # Check template exists
    if [ ! -f "${TEMPLATE_FILE}" ]; then
        echo "  SKIP: Template file not found: ${TEMPLATE_FILE}"
        FAILED=$((FAILED + 1))
        FAILURES+=("${TEMPLATE_NAME}: template not found")
        continue
    fi

    # Step 2: Generate PPTX
    echo "  Generating PPTX..."
    if ! "${OUTPUT_DIR}/json2pptx" -template "${TEMPLATE_NAME}" -output "${OUTPUT_DIR}" "${FIXTURE}" 2>&1; then
        echo "  FAIL: PPTX generation failed"
        FAILED=$((FAILED + 1))
        FAILURES+=("${TEMPLATE_NAME}: generation failed")
        continue
    fi

    # json2pptx names output after input file — find the generated file
    GENERATED=$(ls -t "${OUTPUT_DIR}"/*.pptx 2>/dev/null | grep -v "_conformance" | head -1)
    if [ -z "${GENERATED}" ] || [ ! -f "${GENERATED}" ]; then
        echo "  FAIL: No PPTX output found"
        FAILED=$((FAILED + 1))
        FAILURES+=("${TEMPLATE_NAME}: no output file")
        continue
    fi
    mv "${GENERATED}" "${PPTX_OUTPUT}"

    # Step 3: Structural validation (existing tool)
    echo "  Validating PPTX structure..."
    VALIDATE_JSON=$("${OUTPUT_DIR}/validatepptx" -json "${PPTX_OUTPUT}" 2>&1) || true
    IS_VALID=$(echo "${VALIDATE_JSON}" | grep -o '"is_valid": *[a-z]*' | head -1 | awk '{print $2}')
    SLIDE_COUNT=$(echo "${VALIDATE_JSON}" | grep -o '"slide_count": *[0-9]*' | head -1 | awk '{print $2}')
    SLIDE_COUNT=${SLIDE_COUNT:-0}

    if [ "${IS_VALID}" != "true" ]; then
        echo "  FAIL: Structural validation failed"
        echo "  ${VALIDATE_JSON}"
        FAILED=$((FAILED + 1))
        FAILURES+=("${TEMPLATE_NAME}: structural validation failed")
        continue
    fi
    echo "  Structure OK: ${SLIDE_COUNT} slides"

    # Step 4: LibreOffice conformance — convert PPTX to PDF
    echo "  LibreOffice conformance check..."

    # Remove any previous PDF
    rm -f "${PDF_OUTPUT}"

    # Run LibreOffice headless conversion
    # --infilter forces PPTX import, --convert-to pdf produces PDF output
    if ! timeout 60 ${LIBREOFFICE_CMD} --headless --convert-to pdf \
        --outdir "${OUTPUT_DIR}" "${PPTX_OUTPUT}" 2>&1; then
        echo "  FAIL: LibreOffice could not convert PPTX to PDF"
        FAILED=$((FAILED + 1))
        FAILURES+=("${TEMPLATE_NAME}: LibreOffice conversion failed")
        continue
    fi

    # Check PDF was actually created (LibreOffice names it after the input)
    EXPECTED_PDF="${OUTPUT_DIR}/$(basename "${PPTX_OUTPUT}" .pptx).pdf"
    if [ ! -f "${EXPECTED_PDF}" ]; then
        echo "  FAIL: PDF was not created by LibreOffice"
        FAILED=$((FAILED + 1))
        FAILURES+=("${TEMPLATE_NAME}: no PDF output from LibreOffice")
        continue
    fi
    mv "${EXPECTED_PDF}" "${PDF_OUTPUT}" 2>/dev/null || true

    # Check PDF is non-empty and non-corrupt (must be >1KB)
    PDF_SIZE=$(wc -c < "${PDF_OUTPUT}" | tr -d ' ')
    if [ "${PDF_SIZE}" -lt 1024 ]; then
        echo "  FAIL: PDF too small (${PDF_SIZE} bytes) — likely corrupt"
        FAILED=$((FAILED + 1))
        FAILURES+=("${TEMPLATE_NAME}: corrupt PDF (${PDF_SIZE} bytes)")
        continue
    fi

    # Step 5: Verify PDF page count matches slide count (optional, best-effort)
    PDF_PAGES=""
    if command -v pdfinfo &>/dev/null; then
        PDF_PAGES=$(pdfinfo "${PDF_OUTPUT}" 2>/dev/null | grep "^Pages:" | awk '{print $2}')
    elif command -v pdftotext &>/dev/null; then
        # Fallback: count form feeds in extracted text
        PDF_PAGES=$(pdftotext "${PDF_OUTPUT}" - 2>/dev/null | grep -c $'\f' || echo "")
        if [ -n "${PDF_PAGES}" ]; then
            PDF_PAGES=$((PDF_PAGES + 1))
        fi
    fi

    if [ -n "${PDF_PAGES}" ] && [ "${PDF_PAGES}" -gt 0 ]; then
        if [ "${PDF_PAGES}" -ne "${SLIDE_COUNT}" ]; then
            echo "  WARN: PDF pages (${PDF_PAGES}) != PPTX slides (${SLIDE_COUNT})"
        else
            echo "  Page count verified: ${PDF_PAGES} pages = ${SLIDE_COUNT} slides"
        fi
    else
        echo "  Page count verification skipped (no pdfinfo/pdftotext)"
    fi

    echo "  PASS: ${TEMPLATE_NAME} (${SLIDE_COUNT} slides, ${PDF_SIZE} bytes PDF)"
    PASSED=$((PASSED + 1))
    echo ""
done

# Summary
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "               CONFORMANCE SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Total:  ${TOTAL}"
echo "Passed: ${PASSED}"
echo "Failed: ${FAILED}"

if [ ${FAILED} -gt 0 ]; then
    echo ""
    echo "Failures:"
    for failure in "${FAILURES[@]}"; do
        echo "  - ${failure}"
    done
    echo ""
    exit 1
fi

echo ""
echo "All PPTX files passed LibreOffice conformance validation."
exit 0
