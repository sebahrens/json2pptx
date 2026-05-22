#!/usr/bin/env bash
#===============================================================================
# Per-Template Visual QA Driver
#===============================================================================
#
# DESCRIPTION:
#   Drives the per-template visual QA workflow for the Go Slide Creator.
#
#   For every templates/*.pptx the script:
#     1. Runs json2pptx template-check --json (records conformance to JSON).
#     2. Generates a PPTX from the fixed reference deck
#        (examples/template-qa-deck.json) for that template — the deck covers
#        all 7 mandatory layout roles (title, content, two-column, section,
#        blank-canvas, blank-title, closing) and 3 representative patterns
#        (shape_grid, comparison-2col, journey-maturity-model).
#     3. Converts the PPTX to per-slide JPGs via cmd/pptx2jpg.
#     4. Writes a REPORT.md skeleton under output/visual-qa/<template>/REPORT.md
#        with embedded screenshots and file:line references back to the source
#        deck JSON. The skeleton has placeholder sections that the
#        slide-visual-qa skill (Haiku subagent) fills in with findings.
#
#   The script itself is non-interactive; running the visual-qa subagent over
#   the produced JPGs and merging its findings into REPORT.md is a manual /
#   subagent step (see "Next steps" output at the bottom of each run).
#
# USAGE:
#   ./scripts/test_template_visual_qa.sh                  # all templates
#   TEMPLATE=midnight-blue ./scripts/test_template_visual_qa.sh   # single template
#
# ENVIRONMENT VARIABLES:
#   TEMPLATE         - if set, only run for templates/<TEMPLATE>.pptx
#   KEEP_OUTPUT      - if "1", preserve previous output/visual-qa/* directories
#                      (default: clean per-template subdirs before regenerating)
#
# OUTPUT LAYOUT:
#   output/visual-qa/<template>/
#     deck.json               - copy of the deck JSON used (for findings refs)
#     template-check.json     - conformance check output
#     generated.pptx          - rendered deck
#     generated-slide-N.jpg   - per-slide JPGs (0-indexed by ImageMagick; sorted
#                               numerically and relabelled 1-indexed in REPORT.md)
#     REPORT.md               - report skeleton + embedded screenshots
#
# EXIT CODES:
#   0 - All templates rendered; REPORT.md skeletons written
#   1 - One or more stages failed (build / generate / convert / template-check)
#
#===============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TEMPLATE_DIR="${PROJECT_DIR}/templates"
DECK_JSON="${PROJECT_DIR}/examples/template-qa-deck.json"
OUTPUT_ROOT="${PROJECT_DIR}/output/visual-qa"
BIN_DIR="${PROJECT_DIR}/output/visual-qa/.bin"

if [[ ! -f "${DECK_JSON}" ]]; then
    echo "ERROR: reference deck not found at ${DECK_JSON}" >&2
    exit 1
fi

mkdir -p "${BIN_DIR}"

echo "Step 1: Building json2pptx + pptx2jpg..."
(cd "${PROJECT_DIR}" && go build -o "${BIN_DIR}/json2pptx" ./cmd/json2pptx)
(cd "${PROJECT_DIR}" && go build -o "${BIN_DIR}/pptx2jpg" ./cmd/pptx2jpg)

JSON2PPTX="${BIN_DIR}/json2pptx"
PPTX2JPG="${BIN_DIR}/pptx2jpg"

# Collect templates to process.
if [[ -n "${TEMPLATE:-}" ]]; then
    if [[ ! -f "${TEMPLATE_DIR}/${TEMPLATE}.pptx" ]]; then
        echo "ERROR: TEMPLATE=${TEMPLATE} but ${TEMPLATE_DIR}/${TEMPLATE}.pptx does not exist" >&2
        exit 1
    fi
    TEMPLATES=("${TEMPLATE_DIR}/${TEMPLATE}.pptx")
else
    TEMPLATES=()
    while IFS= read -r f; do
        TEMPLATES+=("$f")
    done < <(find "${TEMPLATE_DIR}" -maxdepth 1 -name '*.pptx' | sort)
fi

if [[ ${#TEMPLATES[@]} -eq 0 ]]; then
    echo "ERROR: no templates found in ${TEMPLATE_DIR}" >&2
    exit 1
fi

# Pre-compute the slide → source-JSON line map from the deck so REPORT.md
# can link each slide back to its definition (criterion #3: file:line refs).
# The pattern matches the top-level slide-object opening brace exactly
# (4-space indent + "{" + end-of-line), so it does not catch nested cell
# objects, shape objects, or pattern values.
SLIDE_LINES=()
while IFS= read -r line; do
    SLIDE_LINES+=("$line")
done < <(grep -nE '^    \{$' "${DECK_JSON}" | awk -F: '{print $1}')

TOTAL=${#TEMPLATES[@]}
PASSED=0
FAILED=0
FAILED_NAMES=()

echo "Step 2: Processing ${TOTAL} template(s)..."
echo ""

for TEMPLATE_PATH in "${TEMPLATES[@]}"; do
    TEMPLATE_NAME="$(basename "${TEMPLATE_PATH}" .pptx)"
    TEMPLATE_OUT="${OUTPUT_ROOT}/${TEMPLATE_NAME}"

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Template: ${TEMPLATE_NAME}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    if [[ "${KEEP_OUTPUT:-0}" != "1" ]]; then
        rm -rf "${TEMPLATE_OUT}"
    fi
    mkdir -p "${TEMPLATE_OUT}"

    # Copy deck.json so REPORT.md file:line refs resolve under the QA dir
    # even when the source examples/template-qa-deck.json moves.
    cp "${DECK_JSON}" "${TEMPLATE_OUT}/deck.json"

    # --- template-check ---
    echo "  [1/4] template-check..."
    TC_JSON="${TEMPLATE_OUT}/template-check.json"
    TC_STATUS=0
    "${JSON2PPTX}" template-check --json "${TEMPLATE_PATH}" >"${TC_JSON}" 2>"${TEMPLATE_OUT}/template-check.stderr" || TC_STATUS=$?
    if [[ ${TC_STATUS} -ne 0 ]]; then
        echo "    WARN: template-check exit=${TC_STATUS} (see template-check.json + stderr)"
    fi

    # --- generate ---
    echo "  [2/4] generate PPTX..."
    GEN_LOG="${TEMPLATE_OUT}/generate.log"
    if ! "${JSON2PPTX}" generate \
            -json "${DECK_JSON}" \
            -template "${TEMPLATE_NAME}" \
            -templates-dir "${TEMPLATE_DIR}" \
            -output "${TEMPLATE_OUT}" \
            >"${GEN_LOG}" 2>&1; then
        echo "    FAIL: generate failed (see ${GEN_LOG})"
        FAILED=$((FAILED + 1))
        FAILED_NAMES+=("${TEMPLATE_NAME}:generate")
        continue
    fi

    # Rename produced .pptx to a stable name.
    PRODUCED=$(find "${TEMPLATE_OUT}" -maxdepth 1 -name '*.pptx' | head -1)
    if [[ -z "${PRODUCED}" ]]; then
        echo "    FAIL: no PPTX produced"
        FAILED=$((FAILED + 1))
        FAILED_NAMES+=("${TEMPLATE_NAME}:no-pptx")
        continue
    fi
    GEN_PPTX="${TEMPLATE_OUT}/generated.pptx"
    if [[ "${PRODUCED}" != "${GEN_PPTX}" ]]; then
        mv "${PRODUCED}" "${GEN_PPTX}"
    fi

    # --- convert to JPG ---
    echo "  [3/4] PPTX → JPG..."
    JPG_LOG="${TEMPLATE_OUT}/pptx2jpg.log"
    if ! "${PPTX2JPG}" -input "${GEN_PPTX}" -output "${TEMPLATE_OUT}" -density 150 >"${JPG_LOG}" 2>&1; then
        echo "    FAIL: pptx2jpg failed (see ${JPG_LOG})"
        FAILED=$((FAILED + 1))
        FAILED_NAMES+=("${TEMPLATE_NAME}:pptx2jpg")
        continue
    fi

    # pptx2jpg emits unpadded ImageMagick indices (generated-slide-0.jpg ..
    # generated-slide-10.jpg). A lexicographic sort would order slide-10 before
    # slide-2, so the 1-indexed REPORT.md labels below would attach screenshots
    # to the wrong source slide for decks with 10+ slides. `sort -V` (version
    # sort, matching scripts/e2e_visual_test.sh) orders by the numeric index.
    SLIDE_JPGS=()
    while IFS= read -r jpg; do
        SLIDE_JPGS+=("$(basename "$jpg")")
    done < <(find "${TEMPLATE_OUT}" -maxdepth 1 -name '*-slide-*.jpg' | sort -V)

    if [[ ${#SLIDE_JPGS[@]} -eq 0 ]]; then
        echo "    FAIL: no slide JPGs produced"
        FAILED=$((FAILED + 1))
        FAILED_NAMES+=("${TEMPLATE_NAME}:no-jpg")
        continue
    fi

    # --- REPORT.md skeleton ---
    echo "  [4/4] write REPORT.md..."
    REPORT="${TEMPLATE_OUT}/REPORT.md"
    {
        echo "# Visual QA Report — ${TEMPLATE_NAME}"
        echo ""
        echo "- Generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
        echo "- Template: \`templates/${TEMPLATE_NAME}.pptx\`"
        echo "- Reference deck: [\`deck.json\`](deck.json) (copy of [\`examples/template-qa-deck.json\`](../../../examples/template-qa-deck.json))"
        echo "- Generated PPTX: [\`generated.pptx\`](generated.pptx)"
        echo "- Slides rendered: ${#SLIDE_JPGS[@]}"
        echo ""
        echo "## Template layouts (from \`json2pptx template-check\`)"
        echo ""
        echo "Raw output: [\`template-check.json\`](template-check.json)"
        echo ""
        echo "> _Filled by the \`slide-visual-qa\` subagent. The subagent reads"
        echo "> \`template-check.json\` and reports renames vs new-layouts per the"
        echo "> 'Template Layout Review' section of \`skills/slide-visual-qa/SKILL.md\`._"
        echo ""
        echo "\`\`\`"
        echo "TEMPLATE LAYOUTS"
        echo "  (pending: run the slide-visual-qa skill against the JPGs in this directory)"
        echo "\`\`\`"
        echo ""
        echo "## Composition (from \`analyze_deck_rhythm\`)"
        echo ""
        echo "> _Run \`json2pptx mcp\` and call \`analyze_deck_rhythm\` against \`deck.json\`,"
        echo "> then paste the prose findings here._"
        echo ""
        echo "\`\`\`"
        echo "COMPOSITION"
        echo "  (pending)"
        echo "\`\`\`"
        echo ""
        echo "## Per-slide rendering"
        echo ""
        echo "Each section embeds the rendered JPG and links to the source slide block"
        echo "in \`deck.json\` (file:line). Replace 'No findings recorded yet' with the"
        echo "subagent's per-slide findings — be specific and use stable codes from"
        echo "\`skills/slide-visual-qa/SKILL.md\` (e.g. \`text_overflow\`, \`contrast\`,"
        echo "\`ACCENT_OVERLOAD\`, \`BASELINE_MISALIGN\`, \`MISSING_TAKEAWAY\`)."
        echo ""

        for i in "${!SLIDE_JPGS[@]}"; do
            SLIDE_NUM=$((i + 1))
            JPG_NAME="${SLIDE_JPGS[$i]}"
            # File:line reference back to the slide block in deck.json.
            # SLIDE_LINES[$i] holds the line of the i-th slide object; if the
            # deck has more slides than rendered JPGs (or vice versa) we still
            # cite the lowest-effort approximation.
            LINE_REF=""
            if [[ ${i} -lt ${#SLIDE_LINES[@]} ]]; then
                LINE_REF="${SLIDE_LINES[$i]}"
            fi
            echo "### Slide ${SLIDE_NUM}"
            echo ""
            if [[ -n "${LINE_REF}" ]]; then
                echo "Source: [\`deck.json:${LINE_REF}\`](deck.json#L${LINE_REF})"
            else
                echo "Source: [\`deck.json\`](deck.json) (no line ref)"
            fi
            echo ""
            echo "![Slide ${SLIDE_NUM}](${JPG_NAME})"
            echo ""
            echo "**Findings:** _No findings recorded yet — pending slide-visual-qa subagent._"
            echo ""
        done

        echo "## Findings JSON"
        echo ""
        echo "> _Paste the JSON \`findings\` block produced by the slide-visual-qa"
        echo "> subagent here. The block must validate against the catalog in"
        echo "> \`skills/slide-visual-qa/SKILL.md\` and is consumed by automation."
        echo "> Emit \`{\"findings\": []}\` when the slides are clean._"
        echo ""
        echo "\`\`\`json"
        echo "{\"findings\": []}"
        echo "\`\`\`"
        echo ""
        echo "## Maintainer review checklist"
        echo ""
        echo "Before shipping any change to \`templates/${TEMPLATE_NAME}.pptx\`:"
        echo ""
        echo "- [ ] \`template-check.json\` shows zero FAIL findings"
        echo "- [ ] \`TEMPLATE LAYOUTS\` block filled in (all 7 mandatory roles present, no duplicates)"
        echo "- [ ] \`COMPOSITION\` block filled in (no monotony / accent imbalance)"
        echo "- [ ] Per-slide findings recorded for every slide (or explicit 'no findings')"
        echo "- [ ] \`Findings JSON\` block validates against the SKILL.md catalog"
        echo "- [ ] No \`blocking\` severity findings remain unresolved"
    } >"${REPORT}"

    echo "    OK  → ${REPORT}"
    PASSED=$((PASSED + 1))
done

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Summary"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Templates processed: ${TOTAL}"
echo "Reports written:     ${PASSED}"
echo "Failures:            ${FAILED}"
if [[ ${#FAILED_NAMES[@]} -gt 0 ]]; then
    echo "Failed stages:"
    for n in "${FAILED_NAMES[@]}"; do
        echo "  - ${n}"
    done
fi

echo ""
echo "Next steps (per template directory):"
echo "  1. Inspect output/visual-qa/<template>/*.jpg with the slide-visual-qa skill"
echo "     (spawn a Haiku subagent, pass the JPG paths)."
echo "  2. Paste the subagent's prose findings into the REPORT.md placeholders."
echo "  3. Paste the structured JSON findings block."
echo "  4. Maintainer reviews REPORT.md before the template ships."

if [[ ${FAILED} -gt 0 ]]; then
    exit 1
fi
exit 0
