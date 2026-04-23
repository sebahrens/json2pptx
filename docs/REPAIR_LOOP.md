# Author→Validate→Fix Repair Loop

The repair loop is a protocol for agents to self-correct dense or overflowing slide content.
It wraps `json2pptx validate --fit-report` with an anti-thrash cap to prevent infinite
shrink-font cycles.

## Protocol

```
1. Author slide JSON
2. Run:  json2pptx validate -fit-report=- input.json
3. Parse NDJSON stdout — each line is a fitFinding
4. If any finding has action="unfittable":
   a. Filter to failing cells only
   b. Build feedback: filtered NDJSON + 1-line summary per failing slide
   c. Send back to agent for repair turn
   d. GOTO 2
5. If 2 repair attempts failed for the same slide:
   → Inject split_slide envelope (or downgrade to bullets)
   → Do NOT loop further
6. Stop when: zero unfittable AND zero TDR violations
```

## Anti-Thrash Cap

**Hard limit: 2 repair attempts per slide.** After 2 failed attempts, the driver forces
a `split_slide` envelope to split the table across pages. Models will shrink fonts forever
if not externally capped — this is a property of the loop driver, not the agent.

## NDJSON Fit-Report Format

Each line is a JSON object with these fields:

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | `fit_overflow` or `density_exceeded` |
| `path` | string | JSON path, e.g. `slides[0].content[1].rows[3][2]` |
| `message` | string | Human-readable description |
| `fix` | object | `{kind, params}` — machine-readable fix suggestion |
| `action` | string | `unfittable` (must fix) or `warning` (informational) |
| `binding_dimension` | string | `height` or `width` |
| `required_pt` | number | Space needed (points) |
| `allocated_pt` | number | Space available (points) |
| `wrap_lines` | int | Lines needed after word-wrap |

## Integration Recipes

### Claude Code (Skill-Driven)

The `generate-deck` skill instructs the agent to run validation after each slide batch.
Add this to the skill's post-generation step:

```bash
json2pptx validate -fit-report=- "$JSON_PATH" | while read -r line; do
  action=$(echo "$line" | jq -r '.action')
  if [ "$action" = "unfittable" ]; then
    echo "REPAIR NEEDED: $line"
  fi
done
```

The skill should track attempt counts per slide and force `split_slide` after 2 failures.

### MCP Harness

Callers using the MCP `generate` and `validate_fit_report` tools wrap them in a retry loop:

```python
attempts = {}  # slide_index -> count

for _ in range(10):  # outer safety cap
    result = mcp.call("validate_fit_report", {"json_path": path})
    findings = parse_ndjson(result)
    
    unfittable = [f for f in findings if f["action"] == "unfittable"]
    if not unfittable:
        break  # all clear
    
    # Group by slide
    by_slide = group_by_slide(unfittable)
    
    capped = []
    repairable = []
    for slide_idx, slide_findings in by_slide.items():
        attempts[slide_idx] = attempts.get(slide_idx, 0) + 1
        if attempts[slide_idx] > 2:
            capped.append(slide_idx)
        else:
            repairable.append((slide_idx, slide_findings))
    
    if capped:
        # Inject split_slide for capped slides
        inject_split_slide(json_data, capped)
    
    if repairable:
        # Send findings back to agent for repair
        agent.repair(repairable)
    else:
        break
```

### CI (Pre-Commit Guard)

Add to your CI pipeline or pre-commit hook:

```bash
#!/bin/bash
# Validate all example decks — fail on unfittable findings
for json_file in examples/*.json; do
    output=$(json2pptx validate -fit-report=- "$json_file" 2>/dev/null)
    if echo "$output" | grep -q '"action":"unfittable"'; then
        echo "FAIL: $json_file has unfittable content"
        echo "$output" | jq -c 'select(.action=="unfittable") | {path, message}'
        exit 1
    fi
done
```

## Reference Implementation

The reference loop driver is in `tests/quality/loop_driver.go`. Key types:

- `LoopConfig` — binary path and force-split behavior
- `LoopState` — tracks per-slide attempt counts
- `LoopResult` — structured output with action, affected slides, and message
- `RunValidatePass()` — executes validate and parses NDJSON
- `EvaluateFindings()` — applies anti-thrash cap and returns action

## split_slide Envelope

When the repair cap is reached, the agent should wrap the failing table in a `split_slide`
envelope:

```json
{
  "type": "split_slide",
  "base": { /* original slide */ },
  "split": {
    "by": "table.rows",
    "group_size": 8,
    "title_suffix": " ({page}/{total})",
    "repeat_headers": true
  }
}
```

This expands into N regular slides at parse time, each containing a window of the table rows
with headers repeated on each page. See `cmd/json2pptx/split_slide.go` for details.
