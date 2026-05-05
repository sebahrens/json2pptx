# Author→Validate→Fix Repair Loop

The repair loop is a protocol for agents to self-correct dense or overflowing slide content.
It wraps `json2pptx validate --fit-report` with an anti-thrash cap to prevent infinite
shrink-font cycles.

## Protocol

```
1. Author slide JSON
2. Run:  json2pptx validate -fit-report=- input.json
3. Parse NDJSON stdout — each line is a fitFinding
4. If any finding has action="refuse":
   a. Filter to failing cells only
   b. Build feedback: filtered NDJSON + 1-line summary per failing slide
   c. Send back to agent for repair turn
   d. GOTO 2
5. If 2 repair attempts failed for the same slide:
   → Inject split_slide envelope (or downgrade to bullets)
   → Do NOT loop further
6. Stop when: zero refuse AND zero TDR violations
```

## Anti-Thrash Cap

**Hard limit: 2 repair attempts per slide.** After 2 failed attempts, the driver forces
a `split_slide` envelope to split the table across pages. Models will shrink fonts forever
if not externally capped — this is a property of the loop driver, not the agent.

## NDJSON Fit-Report Format

Each line is a JSON object with these fields:

| Field | Type | Description |
|-------|------|-------------|
| `code` | string | Finding code (see FIT_FINDINGS.md for full catalog) |
| `path` | string | JSON Pointer (RFC 6901), e.g. `/slides/0/content/body` |
| `message` | string | Human-readable description |
| `fix` | object | `{kind, params}` — machine-readable fix suggestion |
| `action` | string | `refuse` (must fix), `shrink_or_split`, `review`, or `info` (see FIT_FINDINGS.md) |
| `next_tool_call` | object | `{tool, args_template}` — the MCP tool call that resolves this finding (see FIT_FINDINGS.md § next_tool_call) |
| `binding_dimension` | string | `height` or `width` |
| `required_pt` | number | Space needed (points) |
| `allocated_pt` | number | Space available (points) |
| `wrap_lines` | int | Lines needed after word-wrap |

## Structural Fix Kinds

Beyond text-shrinking and table-splitting, the repair loop supports structural fix kinds that reshape, swap, or split content at the pattern level:

| Kind | Params | When Emitted | repair_slide Support |
|------|--------|--------------|---------------------|
| `swap_pattern` | `filled_pct`, `filled_slots`, `total_slots`, `reason` | `pattern_underfilled` or `wrong_pattern` — content doesn't match the chosen pattern's slot count | No (use `recommend_pattern` tool instead) |
| `reshape_grid` | (embedded in `swap_pattern` params as `reason`) | `pattern_underfilled` — grid has too few items for the pattern | No (informational — tells agent why `swap_pattern` was suggested) |
| `split_pattern` | `filled_slots`, `recommended_max`, `first`, `second`, `title_part_2` | `pattern_overcrowded` — grid exceeds the pattern's recommended max | **Yes** — splits grid rows across two slides |
| `convert_content` | `from_type`, `to_type`, `reason` | Agent-initiated — convert e.g. table to bullets when density is unfixable | Not yet (agent must regenerate the slide) |

### End-to-End Loop Example: wrong_pattern → swap_pattern → regenerate

```
1. Agent authors slide with pattern "kpi-6up" but provides only 2 KPI items.

2. validate --fit-report emits:
   {
     "code": "wrong_pattern",
     "path": "/slides/1/shape_grid",
     "message": "kpi-6up: content shape (2 items) matches a different pattern; consider kpi-2up",
     "fix": { "kind": "swap_pattern", "params": { "suggested": [{"from": "kpi-6up", "to": "kpi-2up"}] } },
     "action": "review",
     "next_tool_call": { "tool": "recommend_pattern", "args_template": { "item_count": 2 } }
   }

3. Agent calls recommend_pattern(item_count=2) → gets "kpi-2up" confirmation.

4. Agent regenerates the slide with pattern "kpi-2up" and 2 items.

5. Re-validation passes with no findings for that slide. Loop ends.
```

### End-to-End Loop Example: pattern_overcrowded → split_pattern → repair_slide

```
1. Agent authors a card-grid with 12 items (recommended max is 8).

2. validate --fit-report emits:
   {
     "code": "pattern_overcrowded",
     "path": "/slides/3/shape_grid",
     "fix": { "kind": "split_pattern", "params": { "first": 6, "second": 6, "title_part_2": "(continued)" } },
     "action": "review",
     "next_tool_call": { "tool": "repair_slide", "args_template": { "slide_index": 3, "fixes": [{"kind": "split_pattern", "params": {"first": 6}}] } }
   }

3. Agent calls repair_slide(slide_index=3, fixes=[{kind: "split_pattern", params: {first: 6}}]).

4. Tool returns patched deck with slide 3 split into two (6 + 6 cells), title suffixed with "(continued)" on second slide.

5. Re-validation passes. Loop ends.
```

## Integration Recipes

### Claude Code (Skill-Driven)

The `generate-deck` skill instructs the agent to run validation after each slide batch.
Add this to the skill's post-generation step:

```bash
json2pptx validate -fit-report=- "$JSON_PATH" | while read -r line; do
  action=$(echo "$line" | jq -r '.action')
  if [ "$action" = "refuse" ]; then
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
    
    unfittable = [f for f in findings if f["action"] == "refuse"]
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
# Validate all example decks — fail on refuse-action findings
for json_file in examples/*.json; do
    output=$(json2pptx validate -fit-report=- "$json_file" 2>/dev/null)
    if echo "$output" | grep -q '"action":"refuse"'; then
        echo "FAIL: $json_file has refuse-action content"
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
