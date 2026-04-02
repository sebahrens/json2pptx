#!/bin/bash
#===============================================================================
# E2E Visual Inspection Test Script
#===============================================================================
#
# DESCRIPTION:
#   End-to-end visual testing pipeline for the Go Slide Creator.
#   Uses the json2pptx JSON pipeline (not the old markdown testgen pipeline).
#
#   Flow:
#   1. testrand visual --template=X → deck.json
#   2. json2pptx generate -json deck.json → visual_test.pptx
#   3. LibreOffice → PDF → pdftoppm → JPG slides
#   4. Create inspection report for Claude Code review
#
# USAGE:
#   # Random single template test (default)
#   ./scripts/e2e_visual_test.sh
#
#   # Test all templates
#   TEST_MODE=all ./scripts/e2e_visual_test.sh
#
#   # Test specific template
#   TEST_MODE=forest-green.pptx ./scripts/e2e_visual_test.sh
#
#   # Auto-create Beads issues for failures
#   CREATE_BEADS_ISSUES=true ./scripts/e2e_visual_test.sh
#
# ENVIRONMENT VARIABLES:
#   TEST_MODE            - "random" (default), "all", or specific template filename
#   CREATE_BEADS_ISSUES  - "true" to auto-create Beads issues for failures
#
# OUTPUT:
#   test_output/                        - All test artifacts
#   test_output/inspection_report.json  - Combined report for all templates
#   test_output/<template>/             - Per-template output directory
#     - deck.json                       - Generated JSON input
#     - generated.pptx                  - Generated presentation
#     - generated.pdf                   - PDF conversion
#     - <template>-slide-*.jpg          - Slide images
#     - inspection_report.json          - Per-template inspection report
#     - pptx_validation.json            - Structural validation
#
# EXIT CODES:
#   0 - All templates generated successfully (ready for visual review)
#   1 - Some templates failed generation
#
#===============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
OUTPUT_DIR="${PROJECT_DIR}/test_output"
TEMPLATE_DIR="${PROJECT_DIR}/templates"
REPORT_FILE="${OUTPUT_DIR}/inspection_report.json"

# Test mode: "random" (default), "all", or specific template name
TEST_MODE="${TEST_MODE:-random}"

# Get list of all templates
TEMPLATES=($(ls -1 "${TEMPLATE_DIR}"/*.pptx 2>/dev/null))

if [ ${#TEMPLATES[@]} -eq 0 ]; then
    echo "ERROR: No templates found in ${TEMPLATE_DIR}"
    exit 1
fi

# Select template(s) based on mode
if [ "${TEST_MODE}" = "all" ]; then
    SELECTED_TEMPLATES=("${TEMPLATES[@]}")
    echo "=== E2E Visual Inspection Test (ALL TEMPLATES) ==="
elif [ -f "${TEMPLATE_DIR}/${TEST_MODE}" ]; then
    SELECTED_TEMPLATES=("${TEMPLATE_DIR}/${TEST_MODE}")
    echo "=== E2E Visual Inspection Test (SPECIFIC TEMPLATE) ==="
else
    RANDOM_INDEX=$((RANDOM % ${#TEMPLATES[@]}))
    SELECTED_TEMPLATES=("${TEMPLATES[$RANDOM_INDEX]}")
    echo "=== E2E Visual Inspection Test (RANDOM TEMPLATE) ==="
fi

echo "Project: ${PROJECT_DIR}"
echo "Template Dir: ${TEMPLATE_DIR}"
echo "Available Templates: ${#TEMPLATES[@]}"
for t in "${TEMPLATES[@]}"; do
    echo "  - $(basename "$t")"
done
echo ""
echo "Testing Templates: ${#SELECTED_TEMPLATES[@]}"
for t in "${SELECTED_TEMPLATES[@]}"; do
    echo "  → $(basename "$t")"
done
echo "Output: ${OUTPUT_DIR}"
echo ""

# Cleanup previous run
rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

# Build required binaries
echo "Step 1: Building binaries..."
cd "${PROJECT_DIR}"

echo "  Building testrand..."
if ! go build -o "${OUTPUT_DIR}/testrand" ./cmd/testrand; then
    echo "ERROR: testrand build failed"
    exit 1
fi

echo "  Building json2pptx..."
if ! go build -o "${OUTPUT_DIR}/json2pptx" ./cmd/json2pptx; then
    echo "ERROR: json2pptx build failed"
    exit 1
fi

echo "  Building PPTX validator..."
if ! go build -o "${OUTPUT_DIR}/validatepptx" ./cmd/validatepptx; then
    echo "ERROR: PPTX validator build failed"
    exit 1
fi

# Track overall results
TOTAL_TEMPLATES=${#SELECTED_TEMPLATES[@]}
PASSED_TEMPLATES=0
FAILED_TEMPLATES=0
ALL_RESULTS=()

# Helper: create a bead for a generation failure (deduplicated)
create_failure_bead() {
    local template_name="$1"
    local failure_stage="$2"

    if [ "${CREATE_BEADS_ISSUES:-false}" != "true" ]; then
        return
    fi

    local title_prefix="E2E: ${template_name} ${failure_stage} failed"
    local existing
    existing=$(bd list --status=open 2>/dev/null | grep -c "${failure_stage}.*${template_name}" || echo "0")
    if [ "${existing}" != "0" ]; then
        echo "  ⏭ Bead already exists for ${template_name} ${failure_stage} failure"
        return
    fi

    echo "  Creating bead for ${template_name} ${failure_stage} failure..."
    local issue_id
    issue_id=$(bd create \
        --title="${title_prefix}" \
        --type=bug \
        --priority=1 \
        --labels="e2e,generation-failure" 2>/dev/null || echo "")
    if [ -n "${issue_id}" ]; then
        echo "  ✓ Created: ${issue_id} [P1]"
    fi
}

# Step 2-5: Loop through selected templates
for TEMPLATE in "${SELECTED_TEMPLATES[@]}"; do
    TEMPLATE_NAME=$(basename "${TEMPLATE}" .pptx)
    TEMPLATE_OUTPUT_DIR="${OUTPUT_DIR}/${TEMPLATE_NAME}"
    mkdir -p "${TEMPLATE_OUTPUT_DIR}"

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Testing template: ${TEMPLATE_NAME}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # Step 2: Generate JSON deck via testrand visual
    echo "Step 2: Generating visual stress test JSON..."
    DECK_JSON="${TEMPLATE_OUTPUT_DIR}/deck.json"

    if ! "${OUTPUT_DIR}/testrand" visual --template="${TEMPLATE_NAME}" --output="${DECK_JSON}" 2>&1; then
        echo "ERROR: testrand visual failed for ${TEMPLATE_NAME}"
        create_failure_bead "${TEMPLATE_NAME}" "JSON generation"
        FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
        ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"json_generation_failed\", \"score\": 0}")
        continue
    fi

    # Step 3: Generate PPTX via json2pptx
    echo "Step 3: Generating PPTX with json2pptx..."

    GEN_OUTPUT=$("${OUTPUT_DIR}/json2pptx" generate \
        -json "${DECK_JSON}" \
        -output "${TEMPLATE_OUTPUT_DIR}" \
        -templates-dir "${TEMPLATE_DIR}" \
        -partial 2>&1)
    GEN_STATUS=$?
    echo "${GEN_OUTPUT}"

    if [ ${GEN_STATUS} -ne 0 ]; then
        echo "ERROR: json2pptx generation failed for ${TEMPLATE_NAME}"
        create_failure_bead "${TEMPLATE_NAME}" "PPTX generation"
        FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
        ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"generation_failed\", \"score\": 0}")
        continue
    fi

    # Find the generated PPTX file
    OUTPUT_PPTX=$(find "${TEMPLATE_OUTPUT_DIR}" -name "*.pptx" -maxdepth 1 | head -1)
    if [ -z "${OUTPUT_PPTX}" ]; then
        echo "ERROR: No PPTX output found for ${TEMPLATE_NAME}"
        create_failure_bead "${TEMPLATE_NAME}" "PPTX generation"
        FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
        ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"no_output\", \"score\": 0}")
        continue
    fi

    # Rename to consistent name
    CONSISTENT_PPTX="${TEMPLATE_OUTPUT_DIR}/generated.pptx"
    mv "${OUTPUT_PPTX}" "${CONSISTENT_PPTX}"
    OUTPUT_PPTX="${CONSISTENT_PPTX}"

    echo "PPTX generated: ${OUTPUT_PPTX}"

    # Step 3.5: Validate PPTX structure
    echo "Step 3.5: Validating PPTX structure..."
    VALIDATE_REPORT="${TEMPLATE_OUTPUT_DIR}/pptx_validation.json"
    if ! "${OUTPUT_DIR}/validatepptx" -json "${OUTPUT_PPTX}" > "${VALIDATE_REPORT}" 2>&1; then
        echo "WARNING: PPTX validation reported issues"
        cat "${VALIDATE_REPORT}"
    else
        PPTX_SLIDES=$(grep -o '"slide_count": [0-9]*' "${VALIDATE_REPORT}" | cut -d: -f2 | tr -d ' ')
        PPTX_SVG=$(grep -o '"SVG": [0-9]*' "${VALIDATE_REPORT}" | cut -d: -f2 | tr -d ' ')
        PPTX_PNG=$(grep -o '"PNG": [0-9]*' "${VALIDATE_REPORT}" | cut -d: -f2 | tr -d ' ')
        echo "  PPTX Validation: ${PPTX_SLIDES} slides, ${PPTX_SVG:-0} SVG, ${PPTX_PNG:-0} PNG"
    fi

    # Step 4: Convert PPTX to JPG
    echo "Step 4: Converting PPTX to JPG..."

    LIBREOFFICE_CMD="libreoffice"
    if command -v soffice &> /dev/null && [[ "$(uname)" == "Darwin" ]]; then
        LIBREOFFICE_CMD="soffice"
    fi
    if ! $LIBREOFFICE_CMD --headless --convert-to pdf --outdir "${TEMPLATE_OUTPUT_DIR}" "${OUTPUT_PPTX}" 2>&1; then
        echo "ERROR: PDF conversion failed for ${TEMPLATE_NAME}"
        create_failure_bead "${TEMPLATE_NAME}" "PDF conversion"
        FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
        ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"pdf_conversion_failed\", \"score\": 0}")
        continue
    fi

    PDF_FILE="${TEMPLATE_OUTPUT_DIR}/generated.pdf"
    if [ ! -f "${PDF_FILE}" ]; then
        echo "ERROR: PDF was not created for ${TEMPLATE_NAME}"
        create_failure_bead "${TEMPLATE_NAME}" "PDF conversion"
        FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
        ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"no_pdf\", \"score\": 0}")
        continue
    fi

    # Convert PDF to JPG slides
    if command -v pdftoppm &> /dev/null; then
        if ! pdftoppm -jpeg -scale-to 1920 "${PDF_FILE}" "${TEMPLATE_OUTPUT_DIR}/${TEMPLATE_NAME}-slide" 2>&1; then
            echo "ERROR: JPG conversion failed for ${TEMPLATE_NAME}"
            create_failure_bead "${TEMPLATE_NAME}" "JPG conversion"
            FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
            ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"jpg_conversion_failed\", \"score\": 0}")
            continue
        fi
        # Rename pdftoppm output from -01.jpg to -0.jpg format (0-indexed)
        for f in "${TEMPLATE_OUTPUT_DIR}/${TEMPLATE_NAME}-slide-"*.jpg; do
            if [ -f "$f" ]; then
                page_num=$(echo "$f" | grep -oE '\-[0-9]+\.jpg$' | grep -oE '[0-9]+')
                new_num=$((10#${page_num} - 1))
                new_name="${TEMPLATE_OUTPUT_DIR}/${TEMPLATE_NAME}-slide-${new_num}.jpg"
                mv "$f" "$new_name" 2>/dev/null || true
            fi
        done
    elif command -v convert &> /dev/null; then
        if ! convert -density 150 "${PDF_FILE}" -quality 90 "${TEMPLATE_OUTPUT_DIR}/${TEMPLATE_NAME}-slide-%d.jpg" 2>&1; then
            echo "ERROR: JPG conversion failed for ${TEMPLATE_NAME}"
            create_failure_bead "${TEMPLATE_NAME}" "JPG conversion"
            FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
            ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"jpg_conversion_failed\", \"score\": 0}")
            continue
        fi
    else
        echo "ERROR: No PDF to image converter found (need pdftoppm or convert)"
        create_failure_bead "${TEMPLATE_NAME}" "missing converter tools"
        FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
        ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"no_converter\", \"score\": 0}")
        continue
    fi

    SLIDE_COUNT=$(ls -1 "${TEMPLATE_OUTPUT_DIR}"/*-slide-*.jpg 2>/dev/null | wc -l | tr -d ' ')
    echo "Generated ${SLIDE_COUNT} slide images"

    if [ "${SLIDE_COUNT}" -eq 0 ]; then
        echo "ERROR: No slide images generated for ${TEMPLATE_NAME}"
        create_failure_bead "${TEMPLATE_NAME}" "slide image generation"
        FAILED_TEMPLATES=$((FAILED_TEMPLATES + 1))
        ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"no_slides\", \"score\": 0}")
        continue
    fi

    # Step 5: Generate inspection report
    echo "Step 5: Creating inspection report..."

    TEMPLATE_REPORT="${TEMPLATE_OUTPUT_DIR}/inspection_report.json"
    SLIDE_IMAGES=$(ls -1 "${TEMPLATE_OUTPUT_DIR}"/*-slide-*.jpg 2>/dev/null | sort -V)

    cat > "${TEMPLATE_REPORT}" << REPORTEOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "template": "${TEMPLATE_NAME}",
  "pptx_file": "generated.pptx",
  "deck_json": "deck.json",
  "total_slides": ${SLIDE_COUNT},
  "status": "pending_review",
  "note": "Visual inspection to be performed by Claude Code",
  "slide_images": [
$(echo "${SLIDE_IMAGES}" | while read -r img; do echo "    \"${img}\","; done | sed '$ s/,$//')
  ],
  "pptx_validation": "${VALIDATE_REPORT}",
  "artifacts": {
    "pptx": "${OUTPUT_PPTX}",
    "pdf": "${PDF_FILE}",
    "deck_json": "${DECK_JSON}"
  }
}
REPORTEOF

    echo "  Generated ${SLIDE_COUNT} slides for visual review"
    echo "  Images: ${TEMPLATE_OUTPUT_DIR}/${TEMPLATE_NAME}-slide-*.jpg"
    echo ""
    echo "✅ ${TEMPLATE_NAME}: Ready for Claude Code visual inspection"
    PASSED_TEMPLATES=$((PASSED_TEMPLATES + 1))

    ALL_RESULTS+=("{\"template\": \"${TEMPLATE_NAME}\", \"status\": \"pending_review\", \"slides\": ${SLIDE_COUNT}}")
done

# Step 6: Generate combined report
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "          READY FOR CLAUDE CODE VISUAL REVIEW"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

RESULTS_JSON=$(printf '%s\n' "${ALL_RESULTS[@]}" | paste -sd',' -)

cat > "${REPORT_FILE}" << REPORTEOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "test_mode": "${TEST_MODE}",
  "total_templates": ${TOTAL_TEMPLATES},
  "templates_ready": ${PASSED_TEMPLATES},
  "templates_failed_generation": ${FAILED_TEMPLATES},
  "status": "pending_visual_review",
  "note": "Claude Code should review slide images and create Beads issues for any visual defects",
  "results": [${RESULTS_JSON}]
}
REPORTEOF

echo ""
echo "Templates Generated: ${TOTAL_TEMPLATES}"
echo "Ready for Review: ${PASSED_TEMPLATES}"
echo "Generation Failures: ${FAILED_TEMPLATES}"
echo ""
echo "Report saved to: ${REPORT_FILE}"
echo ""
echo "Claude Code should now:"
echo "  1. Read slide images from test_output/<template>/<template>-slide-*.jpg"
echo "  2. Compare against test_output/<template>/deck.json"
echo "  3. Create Beads issues for any visual defects found"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ ${FAILED_TEMPLATES} -eq 0 ]; then
    echo "✅ All ${TOTAL_TEMPLATES} templates generated successfully"
    echo "   Ready for Claude Code visual inspection"
    exit 0
else
    echo "⚠️  ${FAILED_TEMPLATES}/${TOTAL_TEMPLATES} templates failed to generate"
    echo "   Check logs above for generation errors"
    exit 1
fi
