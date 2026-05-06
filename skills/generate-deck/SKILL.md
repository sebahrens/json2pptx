---
name: generate-deck
description: >
  Generate consulting-quality PowerPoint decks from user prompts using json2pptx.
  Applies constrained generation: 4-phase workflow (Plan → Vary → Render → Repair),
  pattern-based shape grids, accent strategy, deck-rhythm analysis, invariant
  enforcement, and validate-repair loop. Use when the user asks to create, generate,
  or build a presentation or slide deck.
---

# Deck Generation Skill

You generate structured JSON for the json2pptx engine. Your output must be valid input
for the `generate_presentation` MCP tool (or the CLI `json2pptx generate -json`).

Read `../template-deck/TEMPLATE_GUIDE.md` for the complete field reference (content types, chart types, diagram types, shape grid properties, patch operations). This skill covers the **generation workflow and patterns** — not the field reference.

See `examples/four-phase-workflow.md` for a worked end-to-end example of the 4-phase flow.

---

## MCP Tools (prefer over CLI shell-outs)

When operating through the MCP server, prefer these tools over shelling out to the CLI:

| Purpose | MCP tool | CLI equivalent |
|---|---|---|
| **Detect API drift** — fetch `schema_version`, live tool list, deprecations, feature flags | `get_capabilities` | (CLI inlines) |
| **Authoritative input schema** — JSON Schema for `PresentationInput` with all nested types, `x-field-scope` annotations, and inline enums. Digest-cacheable. | `get_input_schema` | `json2pptx input-schema` |
| Introspect templates, patterns, layouts, `color_roles`, `table_styles`, `white_text_safe`, `data_format_hints_digest` | `list_templates` | `json2pptx skill-info` |
| Fetch data-format hints by digest (paginated) | `get_data_format_hints` | (CLI inlines in skill-info) |
| Resolve a template's theme colors (semantic name → hex, including tint modifiers) | `resolve_theme` | (CLI inlines) |
| Recommend a pattern given an intent (cold-start helper) | `recommend_pattern` | (CLI inlines) |
| Validate input JSON (schema + optional `fit_report`) | `validate_input` | `json2pptx validate [-fit-report]` |
| Preview the planned generation (layout selection, placeholder mapping, fit findings) without rendering | `preview_presentation_plan` | (CLI inlines) |
| Generate the PPTX (accepts `strict_fit` + `fit_report`) | `generate_presentation` | `json2pptx generate` |
| Apply targeted fixes to a single slide (uses the `Fix.Kind` vocabulary fit-report emits) | `repair_slide` | (CLI inlines) |
| Score a generated deck (0-100 with structured findings) | `score_deck` | (CLI inlines) |
| Render one slide to a PNG image (preferred over `pptx2jpg` shell-out) | `render_slide_image` | `pptx2jpg` |
| Render the whole deck as thumbnails (preferred over `pptx2jpg` shell-out) | `render_deck_thumbnails` | `pptx2jpg` |
| Browse pattern catalog | `list_patterns` | `json2pptx patterns list` |
| Show a pattern's value schema | `show_pattern` | `json2pptx patterns show <name>` |
| Validate a pattern's input values | `validate_pattern` | `json2pptx patterns validate` |
| Expand a pattern (preview the `shape_grid` + run table-density checks; returns `density_warnings`, `bounds_source`). Pass `theme_template` (MCP) or `--template` (CLI) for template-aware layout bounds. | `expand_pattern` | `json2pptx patterns expand` |
| Analyze deck rhythm — pattern runs, density variation, accent balance, composition score (lightweight, pre-generation) | `analyze_deck_rhythm` | `json2pptx analyze-rhythm` |
| Table density reference (TDR) — font size + row-count guidance per template/style | `table_density_guide` | `json2pptx tables guide` |
| Icon catalog | `list_icons` | `json2pptx icons list` |
| Chart capability metadata (limits, density behavior, label strategy per type) | `get_chart_capabilities` | (CLI inlines in skill-info) |
| Diagram capability metadata (max nodes, overflow behavior, required fields per type) | `get_diagram_capabilities` | (CLI inlines in skill-info) |
| List named `table_styles`/`cell_styles` registered for a template (read-only) | `list_template_settings` | (CLI inlines) |
| Register a named `table_style` or `cell_style` (**write — gated**) | `register_template_setting` | (CLI inlines) |
| Delete a named template setting (**write — gated**) | `delete_template_setting` | (CLI inlines) |

**Contract drift detection.** Call `get_capabilities` once per session to fetch `schema_version`, the live tool list, deprecated fields, and feature flags (`features.strict_fit`, `compact_responses`, `fit_report`, `strict_unknown_keys`, `named_patterns`, `template_settings`). Compare `schema_version` against the value you cached last session — a major bump means breaking changes and you should re-read this skill. Prefer `features.strict_fit` and `features.named_patterns` over hardcoding mode lists.

**Compact responses (server-driven).** The server unconditionally advertises `experimental.compact_responses: true` in its `initialize` response. There is no client-side opt-in — read the capability after `initialize` if you want to confirm it. Use `get_capabilities` for structural feature discovery.

**Write tools are gated.** `register_template_setting` and `delete_template_setting` require the `JSON2PPTX_ALLOW_SETTINGS_WRITE=1` environment variable on the server. Without it, both return `SETTINGS_WRITE_DISABLED`. Check `get_capabilities().features.template_settings` to confirm support before attempting writes.

**Digest protocol.** `list_templates` returns `data_format_hints_digest` instead of the inline hints payload. Reuse the digest across calls; fetch the full hints only when the digest changes via `get_data_format_hints{digest: "..."}`. The tool short-circuits on `not_modified`. The same digest protocol applies to `get_input_schema{digest: "..."}` — cache the digest and skip re-fetching the full JSON Schema when it hasn't changed.

**Input schema introspection.** Call `get_input_schema` to discover the full `PresentationInput` JSON Schema derived from the live Go structs. Each field is annotated with `x-field-scope` (`deck`, `slide`, `content`, or `shape`) so you know where to place it in the JSON hierarchy. Enum-constrained fields include inline `enum` arrays. Use this to prevent field-scope mistakes (e.g., putting `contrast_check` on a content item instead of the slide, or `design_mode` inside a slide instead of at deck level).

**Chart and diagram capabilities.** `list_templates` includes `chart_capabilities` and `diagram_capabilities` arrays alongside the existing `chart_types`/`diagram_types` string lists. Each entry includes concrete limits (`max_series`, `max_points`, `max_categories` for charts; `max_nodes`, `max_depth` for diagrams), density behavior, and label strategy. Use `get_chart_capabilities` / `get_diagram_capabilities` for the full arrays on demand. Some diagram types have `status: "stub"` indicating the renderer exists but is not yet production-hardened.

**Isolated diagram validation.** The separate `svggen-mcp` server exposes `validate_diagram` for checking a diagram payload in isolation. It returns `{valid: bool, errors?: [{pattern, path, code, message, fix}]}` — note the wrapping `valid`/`errors` envelope (distinct from `expand_pattern`, which returns `{pattern, version, bounds_source, shape_grid, occupancy, density_warnings}`). Per-error `fix.kind` values come from the chart enum: `align_series`, `truncate_or_split`, `replace_value`, `explicit_scale`, `reduce_items`. Invalid style payloads return a structured rejection instead of being silently ignored. Use when validating a chart/diagram before embedding it into a slide.

---

## Workflow: Plan → Vary → Render → Repair

### Phase 1: PLAN — Design the Deck Outline

Before writing any JSON, produce a short outline:

```
Deck: [title]
Template: [template name]
Accent strategy: [primary | rotate | section-keyed]
Slides:
  1. [layout] — [title] — [pattern or content type] — [accent]
  2. [layout] — [title] — [pattern or content type] — [accent]
  ...
```

Each line picks a `layout_id` and a visual approach. For shape grid slides, name the pattern (call `list_patterns` via MCP, or `json2pptx patterns list` from the CLI, for the catalog). For content slides, note the content type (bullets, chart, table, diagram). Use `recommend_pattern` to pick patterns for intents you're unsure about.

Present the outline to the user. Proceed to Phase 2 only after approval or if the user asked for the full deck directly.

**Narrative coherence matters.** A consulting deck tells a story: situation, complication, resolution, evidence, implementation, call to action. The outline is where you design the argument arc. Do not fragment this across phases.

**Cold-start checklist (for decks ≥4 slides):**

1. Pick a template. Call `list_templates` for available options and `color_roles`. The compact response includes `layout_summaries[]` with per-layout `placeholders[]` (each entry has `id`, `type`, `max_chars`) — use these to rough-size content before calling `expand_pattern`. For full placeholder bounds and font details, switch to full mode.
2. Choose `accent_strategy` — `"rotate"` for multi-section decks, `"section-keyed"` for decks with distinct chapters, `"primary"` only for ≤5-slide decks.
3. For each slide intent, call `recommend_pattern` to get the best pattern match. Avoid choosing the same pattern more than twice in a row.
4. Ensure the outline alternates density: high-density slides (tables, grids) should be followed by low-density (stat-hero, pull-quote, section divider). Place a narrative-break pattern (stat-hero, pull-quote) every ~5 slides.
5. Check the outline against the rhythm rule: no pattern should appear 3+ times consecutively. If it does, swap the middle occurrence for a contrasting pattern from a different visual family.

### Phase 2: VARY — Check Rhythm and Accent Balance

After building the JSON but **before** generating the PPTX, run `analyze_deck_rhythm` to catch monotony and accent imbalance.

```
analyze_deck_rhythm(presentation: {template: "...", slides: [...]})
```

Returns:
- `per_slide` — visual fingerprint per slide (pattern, density_class, accent_role, dominant_visual, within_slide_accent_variety)
- `per_slide[].within_slide_accent_variety` — count of distinct accent slots used across the slide's shape_grid cells (0 for non-grid slides)
- `aggregates.longest_run` — longest consecutive run of one pattern (target: ≤2)
- `aggregates.repetition_index` — 0.0 (all unique) to 1.0 (all same) (target: <0.5)
- `aggregates.accent_balance` — fraction of slides per accent (target: no single accent >80%)
- `aggregates.density_cv` — density variation coefficient (target: >0.1 for decks >3 slides)
- `aggregates.density_distribution` — `{underfilled_cells, optimal_cells, overflow_cells}` totals across all shape_grid cells in the deck
- `composition_score` — 0–100 overall quality score
- `recommendations` — actionable suggestions with `recommended_break_patterns`

**Act on rhythm findings before generating:**

- `longest_run ≥ 3` → swap the middle slide of the run to a `recommended_break_patterns` suggestion
- `accent_balance` shows one accent at >80% → set `accent_strategy: "rotate"` or manually vary accent fills
- `density_cv < 0.1` on a 5+ slide deck → insert a low-density narrative break (stat-hero, pull-quote)
- `within_slide_accent_variety == 1` on a slide with 5+ cells → add `cell_accent_mode: progressive` to the pattern overrides
- `density_distribution.underfilled_cells > 30%` of total → add detail text or switch to smaller grid patterns
- `composition_score < 70` → iterate on the outline until score ≥ 70

This is a lightweight static check (no PPTX generation cost). Run it iteratively: fix → re-analyze → confirm score improved.

### Phase 3: RENDER — Generate Full JSON and PPTX

Generate the complete JSON in one pass. Use named patterns for shape grid slides — call `show_pattern` (MCP) or `json2pptx patterns show <name>` (CLI) for each pattern's value schema, then fill in content. Set the pattern at the slide level via the `pattern` field (XOR with `shape_grid` — never set both).

**Accent strategy.** Set `accent_strategy` at the top level of the presentation JSON:
- `"primary"` (default) — all slides use the template's primary accent. Good for short decks.
- `"rotate"` — engine cycles through `accent1`–`accent6` across slides. Use for visual variety.
- `"section-keyed"` — accents rotate per section (slides between `section` layout slides share an accent).

**Pre-emit checklist (verify BEFORE outputting JSON):**

1. Every table: logical rows × cols ≤ TDR ceiling (rows ≤ 7, cols ≤ 6, font ≥ 9pt) — see Rule 20. Count multiline cells as N rows.
2. Every fill is semantic (`accent1`, `lt2`, `dk1`, etc.) except documented brand-color allowlist — no mixed hex+semantic on any slide (Rule 12).
3. No sibling shapes in any `shape_grid` with computed gap < 4pt — no stacked tables separated by hairline dividers.
4. Patterns with 4+ peer cells use `cell_accent_mode: "progressive"` (or `"alternate"` for paired layouts) unless visual consistency is intentional — see Cell Accent Variety.
5. Every cell at 60–110% density — compare your text length against `max_chars` from `expand_pattern`'s `cell_budgets[]`. See Text Capacity Awareness.

### Phase 4: REPAIR — Validate, Render, Verify, Fix

Validation is NOT verification. `validate_input` checks JSON structure; it does not judge whether the deck looks right. Contrast auto-fix, sizing choices, overflowing text, and mis-chosen layouts are all visible in pixels and invisible in JSON. **Images are truth.**

1. **Schema + fit check.** Call `validate_input` with `fit_report: true` (MCP) or run `json2pptx validate -fit-report` (CLI). The CLI form `-fit-report=path.json` writes **NDJSON** (one finding per line, no array wrapping); `-fit-report=-` writes NDJSON to stdout; bare `-fit-report` prints a human-readable summary to stderr. Validate exits 0 even with unfittable cells — refusal comes via `strict_fit` on generate. Fix only failing slides, don't regenerate the deck. The fit-report surfaces diagnostics with `fix.kind` hints that are directly actionable. Stable fields for programmatic matching: `code`, `severity` (error/warning/info), `action` (`refuse`/`shrink_or_split`/`review`/`info`), `fix.kind`, `fix.params`. Advisory (human-readable, may change): `message`. See the Layout Finding Codes section below for the full code catalog. Input JSON is validated with `additionalProperties: false` — unknown fields produce warnings identifying the unexpected key and its location.
2. **Generate.** Call `generate_presentation` with `strict_fit: "warn"` (default) or `"strict"` for refuse-on-overflow (MCP), or `json2pptx generate -strict-fit warn|strict` (CLI). The strict-fit ladder: `off` (legacy, silent shrink+truncate); `warn` (shrink + emit fit-findings); `strict` (refuse on overflow with `fix.kind: split_at_row|reduce_text`). Both native layout findings and chart findings participate in the ladder — see the chart finding codes below for which codes promote at which level. On refusal, MCP returns structured diagnostics with `IsError=true`:
   ```json
   {
     "diagnostics": [
       {
         "path": "slides[2].content.body",
         "code": "placeholder_overflow",
         "severity": "error",
         "message": "text overflows placeholder by 42%",
         "fix": { "kind": "reduce_text" }
       }
     ],
     "summary": "generation refused: 1 error-severity finding"
   }
   ```
3. **Render to images.** Call `render_slide_image` (one slide) or `render_deck_thumbnails` (whole deck) over MCP — preferred over the `pptx2jpg -input <out.pptx> -output <dir>/ -density 150` shell-out. Both paths require LibreOffice + ImageMagick on the server's PATH; if unavailable, **say so explicitly** and flag data-dense slides for manual inspection before declaring done. To get a deck-level quality signal, also call `score_deck` — it returns a 0-100 score plus structured findings keyed to the same `code` vocabulary as fit-report.
4. **Inspection checklist (per slide).** Before handing back to the user, confirm:
   - [ ] Text fits its shape or cell — no clipping, no visible overflow.
   - [ ] Chart axes/legends are readable at deck-viewing size.
   - [ ] Every placeholder and grid cell shows the content you intended.
   - [ ] Text color is intentional — no surprise grays from contrast auto-fix (see Rule 16).
   - [ ] Footer and source render where expected; no "Source: Source:" double prefix (see Rule 18).
5. **Repair.** Prefer `repair_slide` (MCP) over hand-editing JSON — it accepts the same `Fix.Kind` vocabulary fit-report emits and patches one slide without regenerating the deck. Pass the deck JSON, the 0-based `slide_index`, and a `fixes` array of `{kind, params}` directives. Returns the patched deck plus post-patch fit findings for the modified slide. Supported `repair_slide` apply-only kinds are a *superset* of the fit-report enum — see "Fix kinds for `repair_slide`" below. Common repairs:
   - Text clipping or overflow → `repair_slide` with `{kind:"reduce_text", params:{max_items|max_length}}`, `{kind:"shorten_title", params:{max_length}}`, or `{kind:"split_at_row", params:{row}}`. For shape_grid cells specifically, `{kind:"reduce_cell_text", params:{cell_path, max_chars}}` truncates a single cell's text to a character budget (with ellipsis). Prefer rewriting content to fit over truncation; use `reduce_cell_text` only when the agent should not rephrase the text (e.g., user-supplied verbatim content). As a last resort, lower font size or increase cell/row allocation in JSON.
   - Wrong layout for the content → `repair_slide` with `{kind:"swap_layout", params:{layout_id}}`.
   - Surprise gray text from contrast auto-fix (visible as a `contrast_autofixed` finding) → swap fill to an accent with ≥3.0 contrast against white, OR switch text color to `dk1`, OR set `"contrast_check": false` if the gray is wrong and the accent is already a compliant color (see Rule 16).
   - For a no-side-effect dry run before regenerating, call `preview_presentation_plan` to inspect layout selection, placeholder mapping, and fit findings without producing a PPTX.

Do not tell the user the deck is done until the checklist passes or you have explicitly flagged what you couldn't verify.

---

## Text Capacity Awareness

Every shape grid cell has a measurable text capacity — the maximum character count that fits at the resolved font size within the cell's physical dimensions. The engine computes this deterministically using embedded font metrics (no OS font dependency). Agent content should target **60–110%** of each cell's capacity.

### Density Bands

| Band | Density % | Status | Severity | Meaning |
|------|-----------|--------|----------|---------|
| Underfilled | < 60% | `underfilled` | info | Cell looks sparse; content doesn't justify the allocated space |
| Optimal | 60–110% | `optimal` | — | Content fits naturally with appropriate whitespace |
| Overflow | > 110% | `overflow` | warning | Text will clip or require aggressive shrinking to fit |

### Workflow Integration

**Phase 1 PLAN.** When choosing patterns, estimate content volume per cell. A 3-cell grid with single-sentence items fits `kpi-3up`; multi-paragraph items need `card-grid` or a 2-column layout. Use `recommend_pattern` with your content volume in mind. For a quick capacity check, read `placeholders[].max_chars` from the compact `list_templates` response — this gives a rough character budget per placeholder without needing `expand_pattern`.

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

**Phase 3 RENDER.** The pre-emit checklist (above) includes: *"Every cell at 60–110% density."* Verify this by checking that your text length per cell falls within the `max_chars` range from `expand_pattern`.

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

### The `bounds_assumption` Field

`expand_pattern` returns `bounds_assumption: "full_content_area"`, indicating that budgets were computed against the full layout content area (below the title placeholder). When `expand_pattern` is called standalone (outside `compose`), this is always accurate. In a future `compose` context where multiple patterns share a slide, budgets will reflect the composed sub-region — the `bounds_assumption` field will change to indicate the reduced area. Always read this field to understand what the budgets represent.

### Related Tools

- **`expand_pattern`** — returns `cell_budgets[]`, `capacity_warnings[]`, and `layout_suggestions[]` for pre-generation density checks and alternative layout recommendations
- **`validate_input`** (with `fit_report: true`) — post-generation findings including `fit_overflow` and `density_exceeded`
- **`repair_slide`** — apply `reduce_text`, `split_at_row`, or `reduce_cell_text` fixes to bring cells into the optimal band (see Phase 4 REPAIR decision sequence above)

---

## Pattern Library

For BMC, KPI grids, 2x2 matrices, timelines, card grids, icon rows, and two-column comparisons, use json2pptx's named patterns. Named patterns expand to validated `shape_grid` structures at generation time, replacing ~600 tokens of boilerplate with ~100 tokens.

- **Browse the catalog:** `list_patterns` (MCP) or `json2pptx patterns list` (CLI)
- **View a pattern's value schema:** `show_pattern` (MCP) or `json2pptx patterns show <name>` (CLI). Grid-shaped patterns include a `text_budget_guide` block with per-configuration `body_max_chars` and `header_max_chars` — use these to size content before calling `expand_pattern`. The response also includes `example_values` — canonical example values showing the expected shape and realistic content for the `values` parameter. Use these as a template when populating pattern values.
- **Validate before generating:** `validate_pattern` (MCP) or `json2pptx patterns validate <name> <values.json>` (CLI)
- **Preview expansion + density pre-flight:** `expand_pattern` (MCP) or `json2pptx patterns expand` (CLI). Returns `density_warnings` for any embedded tables that exceed TDR ceilings (Rule 20) — run this before `generate_presentation` to catch density issues without paying generation cost. Pass `theme_template` (MCP) or `--template` + `--templates-dir` (CLI) for template-aware layout bounds; the response `bounds_source` field indicates `"template"` or `"default_fallback"`. When all populated cells are consistently suboptimal, the response includes `layout_suggestions[]` with alternative patterns and overrides.
- **Cold-start helper:** `recommend_pattern` (MCP) returns the top patterns for a stated intent (e.g., "compare two options", "show 3 KPIs"). Use when you don't know the catalog by heart.

Apply at the slide level via the top-level `pattern` field (XOR with `shape_grid` — never both):

```json
{
  "layout_id": "blank",
  "pattern": {
    "name": "kpi-3up",
    "values": { ... },
    "callout": {"text": "Takeaway", "emphasis": "accent1"}
  }
}
```

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
5. **Verify** post-write by calling `expand_pattern` and checking `cell_budgets[]` — every cell should land in the 60–110% density band.

**Non-grid patterns** (e.g., `pull-quote`, `stat-hero`, single-cell patterns) have no `text_budget_guide`. For those, use per-placeholder budgets from `list_templates` instead. See [Text Capacity Awareness](#text-capacity-awareness) for the full density workflow.

---

## Rules

Non-negotiable. Violating these causes broken or incorrect slides.

### Shape Grid

| # | Rule | Rationale |
|---|---|---|
| 1 | Cell col_spans must sum to column count per row | Engine panics on mismatched grids |
| 2 | `columns: 3` (int) = 3 equal cols; `[10, 90]` = proportional widths. Never `[3]` | `[3]` creates one column at 3% width, not three columns |
| 3 | `bounds` uses percentages (0-100), not points or EMU | `{"x": 5, "y": 18, "width": 90, "height": 72}` = 5% from left, 18% from top |
| 4 | `gap`/`row_gap`/`col_gap` are typographic points, not percentages. Default 8; 1-4 for dense slides | Cumulative: 5-row grid with `row_gap: 10` burns 40pt (~5% height). Tighten gaps before shrinking content |
| 5 | Row `height` is a percentage of `bounds.height` | Rows without height split remaining space equally |
| 6 | One content type per cell: `shape`, `table`, `icon`, `image`, or `diagram` | Combining silently drops content |
| 7 | Body text cells MUST set all 4 insets (6-10pt each) | Without insets, text jams against shape edges |

### Charts

| # | Rule | Rationale |
|---|---|---|
| 8 | `series[i].values` length must equal `len(categories)` | Mismatched arrays produce corrupted charts |
| 9 | Chart types use underscores: `stacked_bar`, `grouped_bar` | Hyphens (`stacked-bar`) silently fail |
| 10 | Don't mix data formats. Single: `{"Q1": 10}`; Multi: `{categories, series}`; Waterfall: `{points}` | Pick one format per chart |

### Layout Finding Codes

Native (non-chart) findings emitted by `validate_input` (with `fit_report: true`) and `generate_presentation` use these codes. They follow the `{path, code, severity, action, message, fix}` envelope. No prefix — the `chart.*` namespace below covers charts and diagrams.

**Pre-flight codes** — emitted when measuring the deck before render:

| Code | When emitted | Default action | Typical `fix.kind` |
|------|-------------|----------------|--------------------|
| `placeholder_overflow` | Body text overflows placeholder after autofit (three-condition gate: overshoot > 15%, autofit off/unavailable, can't fit at min font) | `shrink_or_split` | `reduce_text` |
| `title_wraps` | Title placeholder measures >1 line (informational, distinct from `placeholder_overflow`) | `review` | `reduce_text` |
| `slide_bounds_overflow` | JSON-authored shape center falls outside slide rect (center-based threshold, not corners) | `shrink_or_split` | `reduce_text` |
| `footer_collision` | Authored shape bbox intersects footer area on a layout that declares a footer placeholder | `review` (strict: `refuse`) | `reduce_text` |
| `fit_overflow` | Per-cell: text needs more lines than cell height allows at the declared font | `refuse` | `split_at_row` / `reduce_text` |
| `density_exceeded` | Table rows × cols beyond TDR ceiling at the declared font (Rule 20) | `review` | `split_at_row` |
| `stacked_tables` | Sibling tables in a shape_grid with `row_gap < 4pt` (two-tables-one-grid anti-pattern) | `review` | `split_at_row` |
| `divider_too_thin` | Divider shape height < 4% of slide height | `review` | — |
| `hex_fill_non_brand` | Non-allowlisted `#RRGGBB` fill on a shape | `review` | `use_semantic_color` |
| `mixed_fill_scheme` | Slide mixes semantic (`accent1`, `lt2`) and hex fills (hex-fill mix anti-pattern) | `review` | `use_semantic_color` |
| `cell_underfilled` | Per-cell: text uses <60% of cell character capacity (density bands: <40% = warning, 40-59% = info) | `review` | `add_detail_or_resize` |

**Density-band severity for cell capacity findings** (`fit_overflow` and `cell_underfilled` share the density axis):

| Density band | Code | Severity | Action | Guidance |
|---|---|---|---|---|
| >130% | `fit_overflow` | `error` | `refuse` | Must reduce text — blocks under `strict_fit: "strict"` |
| 110–130% | `fit_overflow` | `warning` | `review` | Should reduce text before shipping |
| 60–110% | *(none)* | — | — | Healthy range — no finding emitted |
| 40–59% | `cell_underfilled` | `info` | `review` | Consider adding detail; not blocking |
| <40% | `cell_underfilled` | `warning` | `review` | Strongly consider adding detail or using a smaller grid |

**How to triage capacity findings:**
- **`info` severity** — consider acting; not blocking under any `strict_fit` mode
- **`warning` severity** — should act before shipping; not blocking under `strict_fit: "warn"` or `"off"`
- **`error` severity** — must act; blocks generation under `strict_fit: "strict"` (MCP returns `IsError=true`)

`strict_fit` interaction: only `error`-severity findings with action `refuse` block generation in strict mode. `cell_underfilled` never blocks because its maximum severity is `warning`.

**Render-time codes** — emitted during `generate_presentation` when the engine adjusted content to fit:

| Code | When emitted |
|------|-------------|
| `text_trimmed` | Trailing paragraphs trimmed to fit placeholder |
| `text_overflow` | Text still overflows placeholder after trimming |
| `readability_trimmed` | Paragraphs trimmed for readability floor |
| `no_autofit_overflow` | Text overflows placeholder that has `noAutofit` set |
| `table_rows_truncated` | Table rows truncated to fit row height |
| `table_font_scaled` | Table font scaled down to the minimum floor |
| `diagram_clamped` | Diagram placeholder dimensions clamped to minimum |
| `diagram_render_failed` | Diagram render failed; placeholder image inserted |
| `column_width_deficit` | Column widths fell back to global floor |
| `pagination_default_threshold` | Pagination used default threshold (no template capacity available) |
| `contrast_autofixed` | Text color auto-replaced for WCAG AA. `action: info`, `fix.kind: replace_color`, `fix.params: {original_color, replacement_color, background_color, contrast_ratio_before, contrast_ratio_after}` |

**Budget summary code** — emitted when more than `DefaultFindingBudget` (5) findings exist on a slide and `verbose_fit:false`:

| Code | When emitted |
|------|-------------|
| `findings_truncated` | Per-slide finding budget exceeded; remaining findings suppressed. `action: info`, `fix.kind: truncation_summary`, `fix.params: {suppressed_count: int, top_codes: ["code:count", ...] sorted by count desc}`. Pass `verbose_fit: true` (MCP) or `--verbose-fit` (CLI) to see all findings without truncation |

**Action semantics (shared with chart codes):**
- `refuse` — with `strict_fit: "strict"`, generation is blocked and MCP returns `IsError=true`; with `warn`, emits finding only
- `shrink_or_split` — content will be adjusted or distributed; strict promotes to `refuse` for content-loss codes
- `review` — informational; agent should inspect but no automatic remediation
- `info` — advisory/telemetry only, never promoted

**`fix.kind` enum** (stable for programmatic matching):

| Kind | Semantics | Params |
|------|-----------|--------|
| `reduce_text` | Shorten text content in the indicated path | — |
| `split_at_row` | Emit `split_slide` at the given row index | `row: int` |
| `use_semantic_color` | Replace hex fill with `accent1`/`lt2`/`dk1`/… | `message?` |
| `replace_color` | Swap one explicit color for another | `from, to` |
| `replace_value` | Replace an invalid value with a suggested one | `suggestion, allowed?` |
| `provide_value` | Required field is missing | `field` |
| `use_one_of` | Value must be one of an allowed set | `allowed` |
| `rename_field` | Unknown field name close to a known one | `suggestion` |
| `remove_field` | Unknown field should be removed | — |
| `add_detail_or_resize` | Cell is underfilled — add more text or use a smaller grid | `current_density_pct: int` |

Chart/diagram codes below introduce their own `fix.kind` values (`reduce_items`, `explicit_scale`, `truncate_or_split`, `align_series`, `increase_canvas`).

**Fix kinds for `repair_slide`.** The apply-only superset accepted by `repair_slide` includes two kinds that fit-report does not emit. Agents emit these when they decide a fix themselves:

| Kind | Semantics | Params |
|------|-----------|--------|
| `shorten_title` | Truncate the title placeholder text | `max_length: int` (default 50) |
| `swap_layout` | Change the slide's `layout_id` | `layout_id: string` (required) |
| `reduce_cell_text` | Truncate a shape_grid cell's text to fit a character budget (adds ellipsis) | `cell_path: string` (required, JSON Pointer e.g. `"/slides/0/shape_grid/rows/1/cells/2"`), `max_chars: int` (required, > 1). Handles markdown emphasis safely. Prefer rewriting content over truncation — use only when the agent should not rephrase the text. |

`repair_slide` also accepts `reduce_text` (`max_items` for bullets, `max_length` for text), `split_at_row` (`row` = rows per page, optional `title_suffix`, `repeat_headers`), and `use_one_of` (`path`, `value`). Unsupported kinds return `{applied: false, message: "kind_not_supported"}`.

### Chart Finding Codes

Charts and diagrams emit structured findings at render time, following the same `{path, code, message, fix}` envelope as native layout findings (see `docs/FIT_FINDINGS.md`). Codes use the `chart.*` prefix.

**Data-integrity codes** — indicate bad input data:

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.invalid_numeric` | NaN/Inf values clamped during render | `replace_value` |
| `chart.zero_sum_pie` | Pie/donut with all-zero or all-negative values | `replace_value` |
| `chart.negative_on_log` | Negative values on a log-scale chart | `explicit_scale` |
| `chart.all_zero_series` | All series values are zero (flat chart) | `replace_value` |
| `chart.capacity_exceeded` | Series/points/categories exceed renderer limits | `reduce_items` |
| `chart.invalid_time_format` | Time-series string cannot be parsed | `replace_value` |

**Content-loss codes** — successful degradation that dropped or truncated payload; promoted under `warn`:

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.legend_overflow_dropped` | Legend entries dropped (area exceeded) | `reduce_items` |
| `chart.overflow_suppressed` | Overflow content suppressed or truncated | `reduce_items` |

(`chart.capacity_exceeded` is also a content-loss code but is grouped with data-integrity above because strict promotes it all the way to `refuse`.)

**Advisory codes** — informational fitting/labeling adjustments; never promoted:

| Code | When emitted | Fix kind |
|------|-------------|----------|
| `chart.auto_log_scale_applied` | Auto-switched to log scale based on data range | `explicit_scale` |
| `chart.tick_thinned` | Axis tick labels thinned to prevent overlap | `reduce_items` |
| `chart.scatter_label_skipped` | Scatter label skipped due to collision | `increase_canvas` |
| `chart.label_truncated` | Label truncated to fit available space | `increase_canvas` |
| `chart.label_ellipsized` | Label shortened with ellipsis | `increase_canvas` |
| `chart.label_clipped` | Label hard-clipped at container boundary | `increase_canvas` |

**Strict-fit promotion ladder for chart codes** (matches `svggen/core/finding_codes.go::promotionTable`):

| Level | `chart.capacity_exceeded` | `chart.legend_overflow_dropped`, `chart.overflow_suppressed` | Data-integrity codes (5) | Advisory codes (6) |
|-------|---------------------------|-----------------------------------------------------|------------------------|--------------------|
| `off` | (no promotion) | (no promotion) | (no promotion) | (no promotion) |
| `warn` | `shrink_or_split` | `shrink_or_split` | (no promotion) | (no promotion) |
| `strict` | `refuse` | `shrink_or_split` | `refuse` | (no promotion) |

Example chart finding in a fit report:

```json
{
  "path": "slides[1].content.chart_value",
  "code": "chart.capacity_exceeded",
  "message": "12 series exceeds max_series=50 — truncated to first 50",
  "severity": "shrink_or_split",
  "fix": { "kind": "reduce_items", "params": { "limit": 50 } }
}
```

### Content and Layout

| # | Rule | Rationale |
|---|---|---|
| 11 | `layout_id` must be a **canonical ID** — not a display name. Use one of: `title`, `content`, `two-column`, `two-column-wide-narrow`, `two-column-narrow-wide`, `blank`, `section`, `closing`, `image-left`, `image-right`, `quote`, `agenda`. Display names like `"Title Slide"` or `"One Content"` are **not valid** `layout_id` values and will fail to resolve | The engine resolves canonical IDs via tag-based matching (see `internal/layout/canonical.go`). Display names returned by `list_templates` `layout_names` are informational only — they show what the template provides, but `layout_id` must use the canonical form |
| 12 | Semantic fills (`accent1`, `lt2`, `dk1`) required; hex `#RRGGBB` forbidden unless in brand-color allowlist. **Never mix semantic and hex fills on the same slide.** Never use raw names like `"blue"` | Semantic colors adapt to template theme; use `{"color": "accent1", "lumMod": 75000, "lumOff": 25000}` for tints. Mixed hex+semantic on one slide breaks visual consistency and is always a bug |
| 13 | `align`: `"l"`, `"ctr"`, `"r"`, `"just"` | NOT `"left"`, `"center"`, `"right"` |
| 14 | `vertical_align`: `"t"`, `"ctr"`, `"b"` | NOT `"top"`, `"middle"`, `"bottom"` |
| 15 | Templates: `forest-green`, `midnight-blue`, `modern-template`, `warm-coral` | Inspect via `list_templates` (MCP) or `json2pptx skill-info` (CLI). Returns `color_roles`, `table_styles[]`, `white_text_safe`, `layout_names`, and `data_format_hints_digest` |

**`placeholder_id` per layout:** `title`/`closing` → `title`, `subtitle`; `content` → `title`, `body`; `two-column` → `title`, `body`, `body_2`; `blank` → `title` only (body goes in `shape_grid`); `section` → `title`, `body` (engine remaps `subtitle` → `body` with a `placeholder_remapped` finding). For authoritative per-template lists, use `json2pptx skill-info` or `list_templates` (MCP).

### Contrast Auto-Fix

| # | Rule | Rationale |
|---|---|---|
| 16 | Engine auto-replaces low-contrast text with dark gray (WCAG AA, ratio < ~3.0). Auto-fixes are now visible — check `fit_findings` for `contrast_autofixed` entries (with before/after ratios) before deciding whether to re-author colors | White on `accent3`-`accent6` → surprise gray. Fix: use `accent1`/`accent2` fill, or `dk1` text, or `"contrast_check": false` (last resort — only when you've verified contrast manually) |

### Silent Traps (no error, broken output)

| # | Wrong | Right | What happens |
|---|---|---|---|
| 17 | `"footer": "text"` (string) | `"footer": {"enabled": true, "left_text": "text"}` | Crash: cannot unmarshal string |
| 18 | `"source": "Source: X"` | `"source": "X"` | Renders "Source: Source: X" — engine prepends prefix |
| 19 | `"chart": {...}` / `"table": {...}` | `"chart_value": {...}` / `"table_value": {...}` | Empty slide — content fields need `_value` suffix |

### Table Density (TDR — enforced, not advisory)

| # | Rule | Rationale |
|---|---|---|
| 20 | **MUST split** if rows > 7 OR cols > 6 OR font_size < 9pt. No exceptions. | Tables exceeding these limits overflow, clip, or become unreadable at presentation-viewing distance. Emit `split_slide` instead of cramming |

**Multiline cell counting.** A table cell containing `\n` or a comma-list with ≥3 items counts as N logical rows where N = max(line_count, ceil(comma_items / 1)). Apply this adjusted row count BEFORE the rows > 7 check. A 5-row table where 3 cells each contain 2 lines = 5 + 3 = 8 logical rows → must split.

**Refusal wording.** When TDR forces a split, emit exactly: *"This table has [N] logical rows × [M] columns; per Rule 20 I cannot fit this — emitting split_slide to distribute rows across slides."* Do not silently shrink fonts below 9pt to avoid the split.

### Anti-patterns

**Two-tables-one-grid.** Sibling tables stacked in the same `shape_grid` with `row_gap < 4pt` or a divider shape between them with height < 4% of slide height. This creates a visual collision — the tables read as one broken table.

Bad — two tables jammed together:
```json
{
  "rows": [
    {"cells": [{"table": {"headers": ["Q1","Q2"], "rows": [["10","20"]]}}]},
    {"height": 2, "cells": [{"shape": {"type": "rect", "fill": "accent1"}}]},
    {"cells": [{"table": {"headers": ["Q3","Q4"], "rows": [["30","40"]]}}]}
  ],
  "row_gap": 2
}
```

Good — separate slides or adequate spacing:
```json
{
  "rows": [
    {"cells": [{"table": {"headers": ["Q1","Q2"], "rows": [["10","20"]]}}]},
    {"height": 8, "cells": [{"shape": {"type": "rect", "fill": "accent1"}}]},
    {"cells": [{"table": {"headers": ["Q3","Q4"], "rows": [["30","40"]]}}]}
  ],
  "row_gap": 6
}
```
Or better: put each table on its own slide.

**Hex-fill mix.** A slide containing both semantic fills (`accent1`, `lt2`, etc.) AND non-allowlisted `#RRGGBB` hex fills. This always indicates a mistake — either commit to semantic colors or to a documented brand palette, never both on one slide.

Bad — mixed fills on one slide:
```json
{
  "cells": [
    {"shape": {"fill": "accent1", "text": "Revenue"}},
    {"shape": {"fill": "#FF6B35", "text": "Costs"}}
  ]
}
```

Good — all semantic:
```json
{
  "cells": [
    {"shape": {"fill": "accent1", "text": "Revenue"}},
    {"shape": {"fill": "accent2", "text": "Costs"}}
  ]
}
```

**Pattern monotony (deck-level).** Generating N slides with the same pattern (e.g., 5 card-grids in a row) produces a visually flat deck. The audience cannot distinguish slides. This is the single most common agent mistake.

Bad — monotonous sequence:
```
Slide 2: card-grid — "Market Segments"
Slide 3: card-grid — "Product Lines"
Slide 4: card-grid — "Competitor Analysis"
Slide 5: card-grid — "Team Structure"
```

Good — varied sequence with rhythm breaks:
```
Slide 2: card-grid     — "Market Segments"
Slide 3: comparison-2col — "Product Lines"
Slide 4: stat-hero     — "Key Differentiator"     ← narrative break
Slide 5: matrix-2x2    — "Competitor Positioning"
Slide 6: icon-row      — "Team Strengths"
```

Rules: no pattern should appear 3+ times consecutively. Insert a narrative-break pattern (stat-hero, pull-quote) every ~5 slides. Use `analyze_deck_rhythm` to detect violations before generating.

**Accent monotony.** Using the default `accent_strategy: "primary"` on a 10+ slide deck makes every shape the same color. Set `"rotate"` or `"section-keyed"` for longer decks, or manually assign different accents to shape fills.

### Cell Accent Variety

**Why it matters.** When every cell in a multi-cell grid uses the same accent color, the slide reads as a monochrome block — the audience cannot visually parse distinct items. Accent variety within a slide creates hierarchy and makes each cell scannable at presentation-viewing distance.

**The three modes.** Grid-shaped patterns expose a `cell_accent_mode` override that controls per-cell accent color variation. The mode operates on the resolved base accent (after `accent_strategy` has picked the slide-level accent):

| Mode | Behavior | When to use |
|------|----------|-------------|
| `uniform` (default) | Every cell uses the same base accent | Timelines, process flows, sequential steps — consistency aids comprehension of order |
| `alternate` | Cells alternate between base and base+1 (wraps at accent6→accent1) | Paired comparisons, two-tier hierarchies, before/after — distinguishes two groups |
| `progressive` | Each cell walks base, base+1, base+2, ... (wraps at accent6→accent1) | 4+ peer cells where differentiation matters — feature lists, benefit grids, KPI dashboards |

**Interaction with `accent_strategy`.** The deck-level `accent_strategy` resolves the base accent per slide (e.g., `section-keyed` assigns one accent per section). `cell_accent_mode` then walks *from* that base within the slide. Example: if `section-keyed` resolves slide 5 to `accent3` and `cell_accent_mode` is `progressive`, cells get `accent3`, `accent4`, `accent5`, `accent6`, `accent1`, `accent2`.

**Which patterns support it.** All grid-shaped patterns (those with multiple peer cells rendered by the shape grid engine) support `cell_accent_mode`. Non-grid patterns (single-cell heroes, axis-bound matrices, fixed-progression layouts) do not expose it because their accent logic is structurally determined. Use `show_pattern` or `list_patterns` to check whether a specific pattern's overrides schema includes `cell_accent_mode`.

**Anti-patterns:**

- All cells same accent on a slide with 4+ peer cells → use `progressive` to differentiate items visually.
- Mixing `alternate` with `section-keyed` accent strategy without validation → verify the result with `analyze_deck_rhythm` to confirm within-slide accent variety reads well against the section-level base.

**Validation loop.** After generating, check `analyze_deck_rhythm` for `within_slide_accent_variety` recommendations. If the tool flags low variety on a grid-heavy slide, set `cell_accent_mode: "progressive"` in the pattern's overrides and re-analyze.

### Charts: Subtitle vs Footnote

Charts accept both `subtitle` and `footnote` fields. Use `subtitle` for contextual text rendered below the chart title (e.g., "FY2024 Q1-Q4"). Use `footnote` for source attribution rendered at the chart bottom. These are separate fields routed to different render positions — do not use `footnote` when you mean `subtitle`.

### Font Availability

The SVG chart renderer (`svggen/`) requires at least one usable font at boot time. If the requested font, system fallbacks (Arial, Helvetica), and the embedded Liberation Sans font all fail to load, the renderer returns an error immediately rather than producing charts with missing text. This is a hard failure — no silent degradation.

### JSON Schema Validation

Input JSON is validated with `additionalProperties: false` at every object level. Unknown keys produce structured warnings identifying the unexpected field and its JSON path. This catches typos (e.g., `chart` instead of `chart_value`) and obsolete fields early, before generation.

---

## Color Roles

Each template exposes `color_roles` in `list_templates` (MCP) / `json2pptx skill-info` (CLI) output — use `primary_fill` / `secondary_fill` for header cells with white text, `body_fill` + `body_text` for card bodies, and check `white_text_safe` before using any accent with `#FFFFFF` text. For tints, use luminance modifiers: `{"color": "accent1", "lumMod": 20000, "lumOff": 80000}` (20% tint with `dk1` text).

**Template-authored accent guidance (`accent_usage_guide`).** Some templates include an `accent_usage_guide` map in their `list_templates` output. When present, it maps accent color names (e.g. `"accent1"`, `"accent3"`) to prose descriptions of each accent's intended role within that template's visual language. When `accent_usage_guide` is present, defer to the template's role descriptions over generic assumptions — do not assume any accent has a fixed semantic role (positive, negative, neutral, subtle, etc.) unless the guide says so. When absent, fall back to the existing `color_roles` `primary_fill`/`secondary_fill`/`body_fill` semantics above.

---

## Deck-Level Defaults

For multi-table decks, set shared styles once in the top-level `defaults` block instead of repeating them on every `table_value`:

```json
{
  "defaults": {
    "table_style": {"style_id": "grid-accent1", "header_background": "accent1"},
    "cell_style": {"align": "l", "vertical_align": "ctr"}
  },
  "slides": [ ... ]
}
```

**Semantics (V1).** Swap-only: any inline field on a table/cell fully replaces the corresponding defaults field for that field (no deep merge). Supported kinds: `table_style`, `cell_style`. See `../../docs/STYLE_DEFAULTS.md` for scope rules and the `@template-default` sentinel. Table styles available per template are listed in `list_templates`'s `table_styles[]` array.

**Per-template named settings.** Beyond per-deck `defaults`, you can register named `table_styles` and `cell_styles` per template via `register_template_setting`, then reference them by name from any deck. List existing names with `list_template_settings{template_name}`. Both write tools (`register_template_setting`, `delete_template_setting`) require `JSON2PPTX_ALLOW_SETTINGS_WRITE=1` on the server and return `SETTINGS_WRITE_DISABLED` otherwise; the read tool is always available. Confirm gating via `get_capabilities().features.template_settings`.

---

## Table Density Reference

Call `table_density_guide` (MCP) or run `json2pptx tables guide` (CLI) for detailed font size and row-count guidance when building table slides in shape grids. **Rule 20 (above) is enforced at generation time** — consult the density guide for sizing recommendations within those hard limits. Pass `{template: "..."}` to scope results to a specific template's `table_styles[]`.

---

## Deck Sizing Guidelines

| Deck type | Slides | Notes |
|---|---|---|
| Executive summary | 5-8 | Title, 3-5 content, closing |
| Strategy / consulting | 12-20 | Full arc: situation, evidence, solution, implementation, ask |
| Board presentation | 8-12 | Concise with data-heavy slides |
| Training / workshop | 15-30 | More content slides, fewer grids |
| Quick update | 3-5 | Title, 1-3 content, next steps |

---

## Icon Names

Call `list_icons` (MCP) or run `json2pptx icons list` (CLI) for all available names. Use `"icon": {"name": "ICON_NAME", "fill": "#FFFFFF"}` inside a shape, or `"icon": {"name": "ICON_NAME"}` as a standalone cell.

---

## Reference

For complete field specifications (connectors, accent bars, callout geometries, speaker notes, footers, backgrounds, theme overrides, patch operations, all chart/diagram types, and more), see `../template-deck/TEMPLATE_GUIDE.md` or run `json2pptx validate-template <path>`.
