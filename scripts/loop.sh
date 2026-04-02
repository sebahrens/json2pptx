#!/bin/bash

# Go Slide Creator - Ralph Loop Runner
# Usage:
#   ./scripts/loop.sh                    - Run build loop (build only)
#   ./scripts/loop.sh plan               - Run planning loop
#   ./scripts/loop.sh N                  - Run build loop for max N iterations
#   ./scripts/loop.sh plan N             - Run planning loop for max N iterations
#   ./scripts/loop.sh include-tests      - Run build loop with fuzz + visual tests
#   ./scripts/loop.sh include-tests N    - Run build loop with tests for max N iterations
#
# Models:
#   Build/plan iterations use Opus 4.6 (--model opus) for complex reasoning.
#   Visual inspection uses Haiku 4.5 (--model haiku) for cost-effective image analysis.
#
# By default, only the build phase runs. The random E2E fuzz test and visual
# inspection phases are opt-in via the "include-tests" flag.
#
# FIX for Claude Code hang bug (GitHub #19060, #25629, #31050):
# Claude completes work but never calls process.exit(). The process hangs
# indefinitely at 0% CPU with stdout open. Using --output-format stream-json
# lets us detect the {"type":"result"} event and kill the process ourselves.

set -e
MODE="build"
INCLUDE_TESTS=false
MAX_ITERATIONS=0
ITERATION=0
BUILD_MODEL="opus"
VISUAL_MODEL="haiku"
HARD_TIMEOUT=2700  # 45min safety net (should never hit with stream-json detection)

# Get absolute path of project directory (parent of scripts/)
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMP_OUTPUT=$(mktemp)
trap "rm -f $TEMP_OUTPUT" EXIT

# Kill any orphaned Claude processes from previous runs
cleanup_orphan_claude_processes() {
    local current_ppid=$$
    ps aux | grep -E "claude.*-p.*--dangerously-skip-permissions" | grep -v grep | while read -r line; do
        local pid=$(echo "$line" | awk '{print $2}')
        if [ "$pid" != "$current_ppid" ]; then
            kill "$pid" 2>/dev/null || true
        fi
    done
}
cleanup_orphan_claude_processes

# Run claude with stream-json and detect completion via result event.
# Returns 0 on successful result, 1 on timeout/no result.
run_claude_with_completion_detection() {
    local prompt_file="$1"
    local model="$2"
    local temp_out="$3"
    local err_log="${temp_out}.err"

    > "$temp_out"
    > "$err_log"

    # Start claude in background with stream-json output
    # Prompt piped via stdin to handle large prompts; stdout=json, stderr=separate log
    cd "$PROJECT_DIR" && cat "$prompt_file" \
        | claude -p --dangerously-skip-permissions --verbose \
            --output-format stream-json --model "$model" \
            > "$temp_out" 2>"$err_log" &
    local claude_pid=$!

    # Hard timeout watchdog (kills claude if stream-json detection fails)
    ( sleep $HARD_TIMEOUT; kill $claude_pid 2>/dev/null ) &
    local watchdog_pid=$!

    # Monitor stream-json output for the result event
    local result_received=false
    while kill -0 $claude_pid 2>/dev/null; do
        if grep -q '"type":"result"' "$temp_out" 2>/dev/null; then
            result_received=true
            # Give claude 3s to exit cleanly, then force kill
            ( sleep 3; kill $claude_pid 2>/dev/null ) &
            local killer_pid=$!
            wait $claude_pid 2>/dev/null
            kill $killer_pid 2>/dev/null
            break
        fi
        sleep 1
    done

    # Clean up watchdog
    kill $watchdog_pid 2>/dev/null
    wait $watchdog_pid 2>/dev/null
    wait $claude_pid 2>/dev/null

    # Final check: process may have exited (e.g. hook crash) after emitting the result
    # but before our polling loop caught it
    if [ "$result_received" = false ] && grep -q '"type":"result"' "$temp_out" 2>/dev/null; then
        result_received=true
    fi

    # Extract and display the result text
    local result_text
    result_text=$(grep '"type":"result"' "$temp_out" | python3 -c "
import sys, json
for line in sys.stdin:
    line = line.strip()
    if line:
        try:
            obj = json.loads(line)
            if obj.get('result'):
                print(obj['result'][:500])
                break
        except: pass
" 2>/dev/null)
    [ -n "$result_text" ] && echo "$result_text"

    if [ "$result_received" = true ]; then
        echo "  (completed via stream-json result detection)"
        rm -f "$err_log"
        return 0
    else
        # Show stderr to help diagnose failures
        if [ -s "$err_log" ]; then
            echo "  stderr output:"
            head -5 "$err_log" | sed 's/^/    /'
        fi
        echo "  (no result event received)"
        rm -f "$err_log"
        return 1
    fi
}

# Parse arguments
for arg in "$@"; do
    if [ "$arg" = "plan" ]; then
        MODE="plan"
    elif [ "$arg" = "include-tests" ]; then
        INCLUDE_TESTS=true
    elif [ "$arg" -eq "$arg" ] 2>/dev/null; then
        MAX_ITERATIONS=$arg
    fi
done

echo "=== Go Slide Creator Ralph Loop ==="
echo "Mode: $MODE"
echo "Tests: $INCLUDE_TESTS"
echo "Project: $PROJECT_DIR"
if [ $MAX_ITERATIONS -gt 0 ]; then
    echo "Max iterations: $MAX_ITERATIONS"
fi
echo ""

# Select prompt file (all prompts live in scripts/)
if [ "$MODE" = "plan" ]; then
    PROMPT_FILE="scripts/PROMPT_plan.md"
else
    PROMPT_FILE="scripts/PROMPT_build.md"
fi
VISUAL_PROMPT_FILE="scripts/PROMPT_visual_review.md"

# Check prompt file exists
if [ ! -f "$PROJECT_DIR/$PROMPT_FILE" ]; then
    echo "Error: $PROMPT_FILE not found in $PROJECT_DIR"
    exit 1
fi

# Main loop
while true; do
    ITERATION=$((ITERATION + 1))
    START_EPOCH=$(date +%s)

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Iteration $ITERATION — $(date)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # Show next bead to work on (mirrors PROMPT_build.md logic: in_progress first, then ready)
    echo ""
    IN_PROGRESS=$(cd "$PROJECT_DIR" && bd list --status=in_progress 2>/dev/null | head -1)
    if [ -n "$IN_PROGRESS" ]; then
        echo "Resuming in-progress bead:"
        echo "  $IN_PROGRESS"
    else
        echo "Next ready bead:"
        cd "$PROJECT_DIR" && bd ready 2>/dev/null | head -1 || echo "  (could not fetch beads)"
    fi
    echo ""

    # Phase 1: Build/plan with Opus 4.6
    # Uses stream-json to detect completion and kill hung process (GitHub #19060 fix)
    echo "  Phase 1: Build ($BUILD_MODEL)"
    set +e
    run_claude_with_completion_detection "$PROJECT_DIR/$PROMPT_FILE" "$BUILD_MODEL" "$TEMP_OUTPUT"
    BUILD_EXIT=$?
    set -e

    BUILD_ELAPSED=$(( $(date +%s) - START_EPOCH ))
    echo ""
    echo "  Build phase completed (exit $BUILD_EXIT, ${BUILD_ELAPSED}s)"

    # Fallback: create tracking bead if build phase crashed without creating its own beads
    if [ $BUILD_EXIT -ne 0 ]; then
        echo "  ⚠ Build phase exited $BUILD_EXIT — checking for untracked failures..."
        EXISTING=$(cd "$PROJECT_DIR" && bd list --status=open 2>/dev/null | grep -c "Loop iteration.*build.*crash" || echo "0")
        if [ "${EXISTING}" = "0" ]; then
            cd "$PROJECT_DIR" && bd create \
                --title="Loop iteration $ITERATION build phase crash (exit $BUILD_EXIT)" \
                --type=bug \
                --priority=1 \
                --labels="loop,build-crash" 2>/dev/null || true
            echo "  Created fallback bead for build phase failure"
        fi
    fi

    # Phase 1.5: Random E2E fuzz test (only with include-tests)
    if [ "$INCLUDE_TESTS" = true ] && [ "$MODE" = "build" ]; then
        echo ""
        echo "  Phase 1.5: Random E2E fuzz test"
        FUZZ_SEED=$(date +%s)
        FUZZ_DIR="$PROJECT_DIR/test_output/random_fuzz"
        mkdir -p "$FUZZ_DIR"
        set +e

        # Build testrand and json2pptx if not already built
        go build -o "$PROJECT_DIR/bin/testrand" "$PROJECT_DIR/cmd/testrand/" 2>/dev/null
        go build -o "$PROJECT_DIR/bin/json2pptx" "$PROJECT_DIR/cmd/json2pptx/" 2>/dev/null

        # Generate random JSON deck
        "$PROJECT_DIR/bin/testrand" generate --seed="$FUZZ_SEED" --output="$FUZZ_DIR/random_deck.json" 2>"$FUZZ_DIR/generate.log"
        GEN_EXIT=$?

        if [ $GEN_EXIT -eq 0 ]; then
            # Run json2pptx on the random deck
            TEMPLATE=$(python3 -c "import json; print(json.load(open('$FUZZ_DIR/random_deck.json'))['template'])" 2>/dev/null || echo "warm-coral")
            "$PROJECT_DIR/bin/json2pptx" generate \
                -json "$FUZZ_DIR/random_deck.json" \
                -output "$FUZZ_DIR" \
                -templates-dir "$PROJECT_DIR/templates" \
                -partial 2>"$FUZZ_DIR/json2pptx.log"
            PPTX_EXIT=$?

            if [ $PPTX_EXIT -ne 0 ]; then
                echo "  ⚠ json2pptx FAILED on random deck (seed=$FUZZ_SEED, exit=$PPTX_EXIT)"
                # Auto-file bead for the failure
                FUZZ_ERROR=$(tail -5 "$FUZZ_DIR/json2pptx.log" 2>/dev/null | head -3)
                EXISTING_FUZZ=$(cd "$PROJECT_DIR" && bd list --status=open 2>/dev/null | grep -c "Random E2E:" || echo "0")
                if [ "${EXISTING_FUZZ}" -lt 5 ]; then
                    cd "$PROJECT_DIR" && bd create \
                        --title="Random E2E: json2pptx failure (seed=$FUZZ_SEED)" \
                        --type=bug \
                        --priority=1 \
                        --labels="random-e2e,fuzz" 2>/dev/null || true
                    echo "  Filed bead for random E2E failure"
                fi
            else
                # Validate the generated PPTX
                PPTX_FILE=$(find "$FUZZ_DIR" -name "*.pptx" -maxdepth 1 | head -1)
                if [ -n "$PPTX_FILE" ]; then
                    "$PROJECT_DIR/bin/testrand" validate --pptx="$PPTX_FILE" 2>/dev/null | tee "$FUZZ_DIR/validation.json" | head -1
                    echo "  ✓ Random E2E passed (seed=$FUZZ_SEED)"
                else
                    echo "  ⚠ No PPTX output found (seed=$FUZZ_SEED)"
                fi
            fi
        else
            echo "  ⚠ testrand generate failed (exit=$GEN_EXIT)"
        fi
        set -e
    fi

    # Phase 2: Visual inspection with Haiku 4.5 (only with include-tests)
    if [ "$INCLUDE_TESTS" = true ] && [ "$MODE" = "build" ]; then
        echo ""
        echo "  Phase 2: Visual inspection ($VISUAL_MODEL)"
        VISUAL_START=$(date +%s)
        set +e
        run_claude_with_completion_detection "$PROJECT_DIR/$VISUAL_PROMPT_FILE" "$VISUAL_MODEL" "$TEMP_OUTPUT"
        VISUAL_EXIT=$?
        set -e
        VISUAL_ELAPSED=$(( $(date +%s) - VISUAL_START ))
        echo "  Visual inspection completed (exit $VISUAL_EXIT, ${VISUAL_ELAPSED}s)"

        # Fallback: create tracking bead if visual phase crashed without creating its own beads
        if [ $VISUAL_EXIT -ne 0 ]; then
            echo "  ⚠ Visual phase exited $VISUAL_EXIT — checking for untracked failures..."
            EXISTING=$(cd "$PROJECT_DIR" && bd list --status=open 2>/dev/null | grep -c "Loop iteration.*visual.*crash" || echo "0")
            if [ "${EXISTING}" = "0" ]; then
                cd "$PROJECT_DIR" && bd create \
                    --title="Loop iteration $ITERATION visual phase crash (exit $VISUAL_EXIT)" \
                    --type=bug \
                    --priority=2 \
                    --labels="loop,visual-crash" 2>/dev/null || true
                echo "  Created fallback bead for visual phase failure"
            fi
        fi

        # Clean up test_output to prevent spillover between iterations
        rm -rf "$PROJECT_DIR/test_output"
        echo "  Cleaned up test_output/"
    fi

    ELAPSED=$(( $(date +%s) - START_EPOCH ))
    echo ""
    echo "Iteration $ITERATION completed (total ${ELAPSED}s)"
    echo ""

    # Check for explicit exit signal (file-based)
    if [ -f "$PROJECT_DIR/.ralph-exit" ]; then
        echo "Exit signal detected (.ralph-exit file found)"
        rm -f "$PROJECT_DIR/.ralph-exit"
        break
    fi

    # Check iteration limit
    if [ $MAX_ITERATIONS -gt 0 ] && [ $ITERATION -ge $MAX_ITERATIONS ]; then
        echo "Reached maximum iterations ($MAX_ITERATIONS)"
        break
    fi

    # Small delay between iterations to avoid hammering
    sleep 2
done

echo "=== Loop completed ==="
echo "Total iterations: $ITERATION"
