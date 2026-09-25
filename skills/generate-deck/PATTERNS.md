# Pattern Library and Text Capacity

How to pick a named pattern, size content for its cells, and read the pre-flight density reports.

---

## Pattern Library

Named patterns expand into validated `shape_grid` structures. Choose from
the live `list_patterns` catalog; call `show_pattern` for a candidate's
`use_when`, `not_when`, value schema, example, and text budget. The catalog
changes with the registry, so this guide intentionally does not enumerate it.
Use `recommend_visual` when the visual family itself is undecided; its
ranking covers patterns, native layouts, charts, and diagrams. Use a diagram
for a data-driven or topologically complex graphic rather than forcing it
into a text grid.

- **Browse the catalog:** `list_patterns` (MCP) or `json2pptx patterns list` (CLI)
- **View a pattern's value schema:** `show_pattern` (MCP) or `json2pptx patterns show <name>` (CLI). Grid-shaped patterns include a `text_budget_guide` block with per-configuration `body_max_chars` and `header_max_chars` — use these to size content before calling `expand_pattern`. The response also includes `example_values` — canonical example values showing the expected shape and realistic content for the `values` parameter. Use these as a template when populating pattern values.
- **Validate before generating:** `validate_pattern` (MCP) or `json2pptx patterns validate <name> <values.json>` (CLI) checks values and content fit. Pass `theme_template` (MCP) or `--template` + `--templates-dir` (CLI) to check against the intended template; CLI `--json` returns the standard findings envelope and exits non-zero for blocking findings.
- **Preview expansion + density pre-flight:** `expand_pattern` (MCP) or `json2pptx patterns expand` (CLI). Returns `density_warnings` for any embedded tables that exceed TDR ceilings (Rule 20) — run this before `generate_presentation` to catch density issues without paying generation cost. Pass `theme_template` (MCP) or `--template` + `--templates-dir` (CLI) for template-aware layout bounds; the response `bounds_source` field indicates `"template"` or `"default_fallback"`. When all populated cells are consistently suboptimal, the response includes `layout_suggestions[]` with alternative patterns and overrides.
- **Cold-start helper:** `recommend_visual` (MCP) ranks across all visual categories for a slide intent — use as the primary entry point. `recommend_pattern` is the pattern-only subset if you already know you need a named pattern.

Apply at the slide level via the top-level `pattern` field (XOR with `shape_grid` — never both):

```json
{
  "layout_id": "blank",
  "pattern": {
    "name": "kpi-3up",
    "values": [
      {"big": "$127M", "small": "Revenue"},
      {"big": "43%",   "small": "Gross margin"},
      {"big": "2.1x",  "small": "YoY growth"}
    ],
    "callout": {"text": "Takeaway", "emphasis": "accent1"}
  }
}
```

KPI `values` is a JSON **array** of cells (one per metric). Each cell is an object `{"big", "small"}` (aliases `value`/`number` → `big`, `label`/`caption` → `small`) or a pipe-delimited string `"$127M | Revenue"`. The two forms are interchangeable and may be mixed in one array.

Do NOT hand-roll shape grids when a named pattern exists. Use the pattern, fill in the values, and let the engine handle grid structure, bounds, and gap arithmetic.

**Callouts.** Patterns with `supports_callout=true` accept an envelope-level `callout: {text, emphasis?, accent?}` — a full-width band rendered below the pattern. Use for one-line takeaways; text is plain string (no bullets / structured content).

### Picking a Grid Configuration with `text_budget_guide`

Grid-shaped patterns support multiple configurations (e.g., 2×2, 3×2, 4×2). `show_pattern` returns a `text_budget_guide` block that tells you how much text fits in each configuration — use it to pick the right grid size **before** writing cell content.

**Response shape** (inside `show_pattern` output):

| Field | Type | Description |
|-------|------|-------------|
| `example_values` | object | Canonical example values showing expected shape and realistic content for `values` |
| `text_budget_guide.target_density` | object | Global density thresholds: `min_pct` (60), `ideal_pct` (85), `max_pct` (110) |
| `text_budget_guide.configurations[]` | array | One entry per supported grid size |
| `configurations[].columns` | int | Number of columns in this configuration |
| `configurations[].rows` | int | Number of rows in this configuration |
| `configurations[].body_max_chars` | int | Maximum body characters per cell (at 12pt) |
| `configurations[].header_max_chars` | int | Maximum header characters per cell (at 16pt) |

**Workflow — pick the right configuration:**

1. **Estimate** your planned content length per cell (in characters).
2. **Call `show_pattern`** and read `text_budget_guide.configurations[]`.
3. **Choose** the configuration whose `body_max_chars` is closest to `planned_chars / 0.85` — this targets ~85% density (the ideal band).
4. **Write** cell content sized to that budget.
5. **Verify** post-write by calling `expand_pattern` and checking `cell_budgets[]` — every cell should land in the 35–110% density band.

**Non-grid patterns** (e.g., `pull-quote`, `stat-hero`, single-cell patterns) have no `text_budget_guide`. For those, use per-placeholder budgets from `list_templates` instead.

**Business Model Canvas.** `bmc-canvas` has nine fixed cells of different sizes. Its `show_pattern` schema describes the per-bullet copy target for each cell and bullet count at default font sizes. The narrow middle cells (`key_activities`, `key_resources`, `customer_relations`, `channels`) hold about 177 characters with one bullet, 52 each with three, and 26 each with four to seven. Use at most seven bullets in these cells and in the bottom cost/revenue cells. `BODY_TOO_LONG` warnings name the cell and measured target. The 200-character schema maximum permits copy in spacious cells; it is not a readable target for every cell.

**Driver tree.** At default sizes and without units, `driver-tree` supports up to 14 total leaf rows. Leaf copy holds about 120 characters through seven rows; at 8–9 rows it holds 120 without annotations or 101 with them; at 10–14 rows it holds 75 without annotations or 51 with them. A branch spanning one row in a 10–14-row tree holds about 38 label characters without annotations or 32 with them. Annotation limits also vary by branch span; the `show_pattern` schema gives the table. Use the `BODY_TOO_LONG` finding for the exact field and target, and check the fit report when units or font overrides are present.

**Two-column comparisons.** For `comparison-2col`, count body rows plus one when the optional headers are present. At default font sizes, each body cell holds about 200 characters through four effective rows, 196 at five, 131 at six or seven, and 66 at eight or more. The 60-character header limit remains readable at all supported row counts. `BODY_TOO_LONG` identifies the row and left/right cell to shorten.

**Phased roadmaps.** `roadmap-phased` activity pills shrink as phase columns and workstream rows increase. At default font sizes, an 8-phase, 6-workstream grid holds about 32 characters per activity; a 2-phase, 2-workstream grid holds the full 80. `show_pattern` lists the measured target for each 2–8 phase by 2–6 workstream combination. `BODY_TOO_LONG` identifies the exact workstream and phase item to shorten.

**Team bios.** `team-bios` has about 220 readable bio characters per card with 1–4 members and 141 with 5–8, at default font sizes. The fifth member adds a second card row. A headshot changes the photo zone but not this bio target. `BODY_TOO_LONG` names the member and target; move longer biographies to a separate slide.

**Metric lists.** `metric-list` stacks 3–7 big values with a one-line label each. At default sizes, `detail` lines hold the full 120 characters through four items and about 90 at five; with six or seven items omit the detail lines (and, at seven, the `callout`). Values share one size shrunk until the longest fits on one line; `TEXT_EXCEEDS_SHAPE` names a value that still wraps, so shorten it or raise `overrides.value_width_pct`. At most one item may set `highlight`.

**Labeled rows.** `labeled-rows` (WHY / WHAT / HOW keyword rows) holds 300-character bodies through four rows and about 190 per row at five or six, where a sublabel that wraps to two lines no longer fits. `BODY_TOO_LONG` reports the measured overflow; `TEXT_EXCEEDS_SHAPE` names a keyword with a word wider than the label column.

**Swimlanes.** `swimlane` step text holds the full 80-character schema limit in sparse grids and about 32 characters in an 8-step, 6-lane grid at default sizes. `show_pattern` lists the measured target for each 2–8 step by 2–6 lane combination. Actor labels remain readable through their 40-character limit. `BODY_TOO_LONG` names the lane and step to shorten.

**Numbered steps.** For `numbered-step-strip`, a six-step `chevron` holds about 47 readable label characters at default size; three to five chevrons and all `stacked-box` / `toc` labels hold the 60-character schema limit. Optional step bodies hold 180. `BODY_TOO_LONG` names labels past the six-chevron target; `TEXT_EXCEEDS_SHAPE` remains the stronger signal when the chevron renderer knows a label will break mid-word.

**Inline KPIs.** `kpi-inline` has a compact height. Captions hold the 40-character schema limit without icons or with up to four KPIs. With icons in five or six cells, the number and optional delta share space with the caption: an eight-character number in a five-KPI bar leaves about 16 caption characters without a delta and none with one. `show_pattern` describes the dense cases; `BODY_TOO_LONG` names the affected KPI and suggests shortening copy or dropping the delta/icon.

---

## Text Capacity Awareness

Every shape grid cell has a measurable text capacity. `density_pct` is a **height** ratio — each paragraph wrapped at its own font size, the line heights summed, compared with the height the cell offers — computed deterministically from embedded font metrics (no OS font dependency); `max_chars` is the derived character hint at the cell's dominant size. Agent content should target **35–110%** of each cell's height. Patterns leave whitespace on purpose, so the median well-authored cell fills about half its box.

### Density Bands

| Band | Density % | Status | Severity | Meaning |
|------|-----------|--------|----------|---------|
| Underfilled | < 60% | `underfilled` | info | Cell looks sparse; content doesn't justify the allocated space |
| Optimal | 35–110% | `optimal` | — | Content fits naturally with appropriate whitespace |
| Overflow | > 110% | `overflow` | warning | Text will clip or require aggressive shrinking to fit |

### Workflow Integration

**Phase 1 PLAN.** When choosing patterns, estimate content volume per cell. A 3-cell grid with single-sentence items fits `kpi-3up`; multi-paragraph items need `card-grid` or a 2-column layout. Use `recommend_pattern` with your content volume in mind. For a quick capacity check, call `list_templates` with legacy `mode="compact"` (or `fields="full"`) and read `layout_summaries[].placeholders[].max_chars`; the default `fields="compact"` omits those per-layout budgets.

**Phase 2 VARY.** After building JSON, call `expand_pattern` to read `cell_budgets[]` before generating. Each entry contains:

| Field | Type | Description |
|-------|------|-------------|
| `cell_index` | int | Zero-based cell position in the grid |
| `row` | int | Row index |
| `col` | int | Column index |
| `max_chars` | int | Maximum characters that fit at the resolved font size |
| `actual_chars` | int | Characters currently in the cell content |
| `density_pct` | int | `actual_chars / max_chars × 100` |
| `status` | string | `"underfilled"`, `"optimal"`, or `"overflow"` |
| `font_size_pt` | float | Font size used for the budget calculation |

Compare your planned content against `max_chars` for each cell. Adjust before rendering — it's cheaper to rewrite content than to repair after generation.

**Phase 3 RENDER.** The pre-emit checklist (in WORKFLOW.md) includes: *"Every cell at 35–110% density."* Verify this by checking `cell_budgets[].density_pct` from `expand_pattern`.

**Phase 4 REPAIR.** If `fit_overflow` or `density_exceeded` findings appear after generation, apply this decision sequence:

1. **Rewrite content** to fit the budget — shorter sentences, fewer bullets, tighter phrasing. This preserves meaning and produces richer output than mechanical truncation.
2. **Swap layout** — if the content genuinely needs more space, use `repair_slide` with `swap_layout` or switch to a pattern with higher capacity (see `layout_suggestions[]` from `expand_pattern`).
3. **`reduce_text` / `split_at_row`** — use `repair_slide` with `fix.kind: reduce_text` or `split_at_row` to bring cells back into the optimal band.
4. **`reduce_cell_text`** — truncates a single shape_grid cell to `max_chars` with an ellipsis. Use only when the agent should not rephrase the text (e.g., user-supplied verbatim content that the agent shouldn't alter). After applying, re-validate with `fit_report: true` to confirm zero residual `cell_overflow` findings.

**Anti-pattern:** do not use `reduce_cell_text` as the default response to overflow. It is a last-resort fallback, not a substitute for the upstream Text Capacity Awareness loop (Phase 2 VARY).

### Decision Rules

When `expand_pattern` returns cells outside the optimal band:

1. **Underfilled cells (< 60%):**
   - Add supporting detail, examples, or context to bring density above 60%.
   - If content is inherently short (a metric label, a one-word status), switch to a pattern designed for sparse content — e.g., `kpi-3up` instead of `card-grid`.
   - For a grid where most cells are underfilled, reduce the grid configuration (e.g., 2×2 instead of 3×2) so each cell gets less space.

2. **Overflow cells (> 110%):**
   - Trim content: shorten sentences, remove low-value bullets, abbreviate labels.
   - If trimming would lose essential information, switch to a larger grid configuration or a different pattern with more text capacity (e.g., `card-grid` with fewer columns).
   - As a last resort, split the slide — use `split_at_row` to distribute content across multiple slides.

3. **Check `layout_suggestions[]`:** When **all** populated cells are consistently suboptimal (all underfilled or all overflowing), `expand_pattern` returns `layout_suggestions[]` — an array of alternative patterns with optional overrides and a `reason` string. Use these as actionable swap recommendations instead of guessing. Suggestions only appear when density is unanimously bad; mixed-density grids (some optimal, some not) produce no suggestions — adjust content manually in that case.

### Bounds Override: `bounds` and `max_height_pct`

When patterns produce oversized cells for short content (e.g., a 3-step process-flow with terse labels), use a compact variant or constrain the grid using `bounds` or `max_height_pct`:

- **Compact variants** (`process-flow-compact`, `before-after-compact`, `kpi-inline`): pre-configured height-capped patterns for short content that leaves room for other content on the slide. Prefer these over manual `max_height_pct` when the content is brief.

- **`max_height_pct`** (number, 1–99): constrains grid height to this percentage of the content area. Equivalent to `bounds: {x:0, y:0, width:100, height:<value>}`.
- **`bounds`** (object): explicit bounding rectangle as percentages of slide dimensions (`x`, `y`, `width`, `height`). Takes priority over `max_height_pct`.

Pass these as parameters to `expand_pattern` (MCP) or in the slide-level `pattern.bounds`/`pattern.max_height_pct` fields (JSON input). The bounds are applied to the expanded grid, and density math automatically uses the reduced area — eliminating false `cell_underfilled` warnings.

When `capacity_warnings[]` reports underfilled cells without explicit bounds, each warning includes a `next_tool_call` suggesting re-expansion with a recommended `max_height_pct`. Follow the suggestion directly.

#### Density-Class Divergence Warnings

`expand_pattern` checks whether the average cell density matches the pattern's declared `density_class` (from its taxonomy). When a medium-density pattern has <15% avg density, or a high-density pattern has <30% avg density, a `density_class_divergence` capacity warning is emitted. The `next_tool_call` suggests either:
- A compact variant (e.g., `process-flow-compact`) if one is registered
- A `max_height_pct` override to reduce the grid area

#### `density_hint` in `recommend_visual` / `recommend_pattern`

Pass `density_hint` ("low", "medium", or "high") in `content_hints` to bias pattern recommendations toward patterns matching the expected content density. Patterns whose `density_class` matches get a scoring boost; distant density classes (e.g., low content on a high-density pattern) receive a penalty. Use this when you already know the content is sparse or dense.

#### `candidates:[]` in `recommend_visual` / `recommend_pattern`

Pass `candidates` (array of strings) to rank an **explicit shortlist** instead of the full catalog. Every supplied name is returned with `score`, `rationale`, and `confidence_band` — the 0.5 threshold cutoff, top-K truncation, near-miss collection, and diversity-bonus injection are all bypassed. For `recommend_visual`, the `category` field is auto-resolved from the catalog (placeholder layout / named pattern / chart / diagram / raw_shape_grid); unknown names still appear with score 0 and a rationale noting the miss. Use this when you have 2–8 specific options in mind and want them ranked against your intent rather than re-discovering them from keywords.

### The `bounds_assumption` Field

`expand_pattern` returns `bounds_assumption` indicating what area the budgets were computed against:

- `"full_content_area"` — budgets reflect the full layout content area (default when no bounds override is provided)
- `"explicit_override"` — budgets reflect the reduced area specified via `bounds` or `max_height_pct`

Always read this field to understand what the budgets represent.

### Related Tools

- **`expand_pattern`** — returns `cell_budgets[]`, `capacity_warnings[]`, and `layout_suggestions[]` for pre-generation density checks and alternative layout recommendations
- **`validate_input`** (with `fit_report: true`) — post-generation findings including `fit_overflow` and `density_exceeded`
- **`repair_slide`** — apply `reduce_text`, `split_at_row`, or `reduce_cell_text` fixes to bring cells into the optimal band (see Phase 4 REPAIR decision sequence above)
