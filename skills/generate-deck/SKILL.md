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

This skill is split into focused sub-files. SKILL.md (this file) covers preconditions, MCP tools, and the workflow overview. Load the sub-files when you need their detail:

| File | Contents |
|---|---|
| [WORKFLOW.md](WORKFLOW.md) | 4-phase workflow deep dive (Plan, Vary, Render, Repair), visual inspection, `next_tool_call`, response_fingerprint |
| [FINDINGS.md](FINDINGS.md) | All finding codes (layout + chart), the `fix.kind` enum, the `repair_slide` apply-only superset, strict-fit promotion ladder |
| [RULES.md](RULES.md) | Rules 1–20 (shape grid, charts, content/layout, contrast, silent traps, table density), anti-patterns, cell accent variety |
| [PATTERNS.md](PATTERNS.md) | Pattern library, `text_budget_guide`, Text Capacity Awareness, density bands, bounds override |

Read `../template-deck/TEMPLATE_GUIDE.md` for the complete field reference (content types, chart types, diagram types, shape grid properties, patch operations).

See `examples/four-phase-workflow.md` for a worked end-to-end example of the 4-phase flow.

**Start from a skeleton, not a blank slate.** Five canonical fillable JSON skeletons live in [`examples/skeletons/`](examples/skeletons/README.md) — pick the one matching your deck archetype, copy it, and replace the `__FILL_*__` tokens. Skeletons pre-encode the rhythm rules, accent strategy, and required `takeaway` fields so you do not re-derive them per deck.

| Archetype | Skeleton | When to reach for it |
|---|---|---|
| Board / executive update | [`exec-summary.json`](examples/skeletons/exec-summary.json) | Headline number → evidence → decision → close (6 slides) |
| QBR / performance review | [`data-heavy.json`](examples/skeletons/data-heavy.json) | Chart-dominant; each chart slide carries a takeaway (7 slides) |
| Today-vs-target / vendor selection | [`comparison.json`](examples/skeletons/comparison.json) | Three comparison frames: transformation, capability table, 2x2 (5 slides) |
| Program walkthrough / delivery plan | [`process-roadmap.json`](examples/skeletons/process-roadmap.json) | Process flow + swimlane + phased timeline + metrics (6 slides) |
| Investor / sales pitch | [`pitch.json`](examples/skeletons/pitch.json) | Problem → solution → traction → ask, classic arc (9 slides) |

---

## Connected MCP servers

This skill talks to **two** independent MCP servers. Both must be reachable for the workflow described below to function end-to-end.

### `json2pptx-mcp` — deck-level engine

Builds, validates, and repairs whole PPTX presentations. Owns templates, layouts, patterns, fit-report, and the `repair_slide` apply-only fix vocabulary. This is the server that exposes every tool listed in the [MCP Tools](#mcp-tools-prefer-over-cli-shell-outs) table below.

- **Binary path:** `cmd/json2pptx/json2pptx` (built via `make` or `go build ./cmd/json2pptx`)
- **Run as MCP:** `json2pptx mcp [-templates-dir <path>] [-output-dir <path>]`
- **Use when:** generating, validating, planning, scoring, repairing, or introspecting a full deck or any template / pattern / icon / shape catalog.

```json
{
  "mcpServers": {
    "json2pptx": {
      "command": "/absolute/path/to/json2pptx",
      "args": ["mcp", "-templates-dir", "/absolute/path/to/templates"]
    }
  }
}
```

### `svggen-mcp` — diagram and chart renderer

Standalone SVG renderer with its own diagram/chart registry. **Distinct connectable server** — not a sub-tool of `json2pptx-mcp`. Use it when you want a rendered SVG (or PNG) for a single diagram or chart, or to validate a diagram payload in isolation before embedding it (e.g., via `shape_grid` cell `icon.svg_data` — see [Icon Names](#icon-names)).

> **Standalone use:** if the consumer is a raw SVG/PNG and not a PPTX, load the focused [`../render-diagram/SKILL.md`](../render-diagram/SKILL.md) instead — it covers the six svggen-mcp tools, the `validate → dry_run → render` flow, the `theme_colors` copy contract, and the error envelopes without the deck-level surface area below.

- **Binary path:** `svggen/cmd/svggen-mcp/svggen-mcp` (built via `cd svggen && go build ./cmd/svggen-mcp`)
- **Run as MCP:** `svggen-mcp` (stdio transport, no flags required)
- **Use when:** rendering or validating an isolated diagram/chart; obtaining raw SVG markup to embed inline in a `shape_grid` cell.

```json
{
  "mcpServers": {
    "svggen": {
      "command": "/absolute/path/to/svggen-mcp"
    }
  }
}
```

**`svggen-mcp` tool table** (the only tools served by this binary):

| Purpose | Tool | Notes |
|---|---|---|
| Render a diagram or chart to SVG or PNG | `render_diagram` | Requires `type` + `data` (JSON object). Optional `style` (JSON object) and `format` (`"svg"` default or `"png"`). Returns the rendered SVG markup as text or base64 PNG. Use this output as a `shape_grid` cell's `icon.svg_data` for inline embedding. |
| List all supported diagram/chart types | `list_diagram_types` | Returns the type registry as an array of `{name, aliases?}` objects. `name` is the canonical registered ID (e.g., `bar_chart`, `pie_chart`); `aliases` enumerates other accepted names (e.g., `["bar"]`, `["pie"]`) that resolve to the same renderer. Prefer the canonical `name` in new code; the short aliases (`bar`, `line`, `pie`, etc.) remain accepted everywhere `render_diagram` takes a `type`. Call once per session to discover what `render_diagram` accepts. |
| Validate a diagram/chart payload | `validate_diagram` | Returns `{valid, errors}` envelope. Use BEFORE `render_diagram` when you want structured errors instead of a render failure. |
| Get the JSON Schema for a diagram/chart type | `get_diagram_schema` | Returns the input schema for a specific `type`, plus `example_values` with both `minimal` (smallest valid input) and `realistic` (representative shape and content) — mirrors `show_pattern.example_values` so you can copy a working example instead of guessing field names from `list_diagram_types`. The legacy top-level `example` field is retained as a back-compat alias for `example_values.realistic`. |
| Detect svggen-mcp contract drift | `get_capabilities` | Returns `{schema_version, tool_list:[{name, description}], chart_types:[], diagram_types:[], chart_capabilities:[...], diagram_capabilities:[...], deprecations:[], features:{dry_render, structured_errors}}`. `schema_version` is sourced from the svggen library version (single source). Call once per session and compare `schema_version` to the value you cached; a change means the rendering or validation contract may have shifted. Distinct from `json2pptx-mcp.get_capabilities` — this one is scoped to the svggen registry. |
| Discover the recommended call sequence for a task | `get_started` | Returns `{task, sequence:[{tool, when_to_call}], available_tasks, notes}`. Pass `task` to scope the sequence: `"render"` (default), `"preflight-render"` (validate before rendering, including a `dry_run` pass), or `"embed-in-deck"` (render SVG markup for inline `shape_grid` `icon.svg_data`). Unknown values fall back to `"render"`. Use as your first call to avoid reverse-engineering the workflow from SKILL.md. Distinct from `json2pptx-mcp.get_started` — this one is scoped to svggen-mcp's six tools. |

When a `validate_diagram` call returns errors, the per-error `fix.kind` values come from the chart-finding enum (`align_series`, `truncate_or_split`, `replace_value`, `explicit_scale`, `reduce_items`) — see FINDINGS.md.

**Keeping the SVG palette in sync with the deck template (one-shot copy):**

Call `resolve_theme` once per deck and pass its `theme_colors` array straight through to every `render_diagram` call. No hand-pivoting from the `colors` map — typos in scheme names would otherwise be silent.

```jsonc
// 1) json2pptx-mcp.resolve_theme({"template_name": "midnight-blue"}) →
{
  "template": "midnight-blue",
  "colors": { "accent1": "#1F4E79", "accent2": "#2E75B6", "dk1": "#000000", "lt1": "#FFFFFF", "...": "..." },
  "theme_colors": [
    { "name": "accent1", "rgb": "#1F4E79" },
    { "name": "accent2", "rgb": "#2E75B6" },
    { "name": "dk1",     "rgb": "#000000" },
    { "name": "lt1",     "rgb": "#FFFFFF" }
  ]
}

// 2) Copy theme_colors verbatim into svggen-mcp.render_diagram:
{
  "type": "bar_chart",
  "data": { /* ... */ },
  "style": {
    "theme_colors": [
      { "name": "accent1", "rgb": "#1F4E79" }
    ]
  }
}
```

If the deck applies a `theme_override`, pass that same object as `resolve_theme`'s `theme_override` argument so the array reflects the post-override palette.

---

## Minimum Valid Deck

The smallest complete input showing the content-as-array shape and key deck/slide-level fields:

```json
{
  "template": "midnight-blue",
  "design_mode": "constrained",
  "slides": [
    {
      "layout_id": "title",
      "contrast_check": true,
      "content": [
        { "placeholder_id": "title", "type": "text", "text_value": "Hello World" },
        { "placeholder_id": "subtitle", "type": "text", "text_value": "A minimal deck" }
      ]
    }
  ]
}
```

**Key scope rules:**
- `design_mode` is **deck-level** (top of the JSON, not inside a slide). The CLI flag `--design-mode=constrained|free` overrides this field for ad-hoc runs (`json2pptx generate --design-mode=free --json deck.json`).
- `contrast_check` is **slide-level** (inside each slide object, not on a content item)

---

## MCP Tools (prefer over CLI shell-outs)

When operating through the MCP server, prefer these tools over shelling out to the CLI. All tools below are served by **`json2pptx-mcp`**; the six `svggen-mcp` tools (`render_diagram`, `list_diagram_types`, `validate_diagram`, `get_diagram_schema`, `get_capabilities`, `get_started`) are documented in the [Connected MCP servers](#connected-mcp-servers) section above.

| Purpose | MCP tool | CLI equivalent |
|---|---|---|
| **First call — ordered MCP workflow keyed to your task** (`brief`/`revise`/`validate-only`). Returns `sequence:[{tool, when_to_call}]` so you do not have to derive the workflow from the flat 35+ tool catalog. | `get_started` | `json2pptx get-started` |
| **Detect API drift** — fetch `schema_version`, live tool list, deprecations, feature flags. Both `json2pptx-mcp` and `svggen-mcp` expose the same standardized fields. | `get_capabilities` | (CLI inlines) |
| **Authoritative input schema** — JSON Schema for `PresentationInput` with all nested types, `x-field-scope` annotations, and inline enums. Digest-cacheable. | `get_input_schema` | `json2pptx input-schema` |
| Introspect templates, patterns, layouts, `canonical_layout_ids`, `color_roles`, `table_styles`, `white_text_safe`, `data_format_hints_digest` | `list_templates` | `json2pptx skill-info` |
| Fetch data-format hints by digest (paginated) | `get_data_format_hints` | (CLI inlines in skill-info) |
| Resolve a template's theme colors (semantic name → hex, including tint modifiers). Returns both `colors` (map for lookups) and `theme_colors` (array ready to copy into svggen-mcp's `render_diagram` under `style.theme_colors`). Accepts optional `theme_override`. | `resolve_theme` | (CLI inlines) |
| **Plan a deck** — given a brief, returns an ordered slide outline with per-slide pattern recommendations, narrative roles, content seeds, and accent rotation. Enforces rhythm rules. Recommended starting point for any deck > 4 slides. Each slide also carries `suggested_pattern`, `suggested_pattern_fallback`, and a `skeleton` (partial `SlideInput` JSON with `__FILL__` tokens for every agent-supplied string) — copy the skeleton and replace tokens rather than re-deriving the slide structure from the prose `content_seed`. | `plan_deck` | (CLI inlines) |
| **Visual decision aid** — rank candidates across layouts, patterns, charts, diagrams, and raw shape_grid for a slide intent. **Start here** when unsure which visual approach to use. | `recommend_visual` | `json2pptx recommend-visual` |
| Recommend a named pattern given an intent (pattern-only subset of recommend_visual) — **legacy, prefer `recommend_visual`** for new code. | `recommend_pattern` | (CLI inlines) |
| Discover preset shape geometries (chevron, homePlate, callout, action_button, ...) grouped by category, with optional substring search | `get_shape_catalog` | (CLI inlines) |
| Validate input JSON (schema + optional `fit_report` + optional `strict_unknown_keys` for fail-fast on typo'd fields) | `validate_input` | `json2pptx validate [--fit-report] [--strict-unknown-keys]` |
| Preview the planned generation (layout selection, placeholder mapping, fit findings) without rendering. Each `resolved_slides[].shape_grid_resolution.cells[]` carries the per-cell wireframe rectangle. For compose slides, `expanded_compose` exposes per-segment geometry. Fit findings attributable to a child segment carry `segment_index`. Surfaces `COMPOSE_HORIZONTAL_TRUNCATION` and `COMPOSE_SEGMENT_BOUNDS_IGNORED` warnings. Accepts optional `strict_unknown_keys` for fail-fast on typo'd fields. | `preview_presentation_plan` | `json2pptx preview [--strict-unknown-keys]` |
| **Render one slide's resolved plan as an annotated wireframe (SVG + base64 PNG) without LibreOffice or ImageMagick** — paints the slide frame, layout placeholders, `shape_grid` cells, occupancy %, and per-cell fit-finding badges. Inputs mirror `preview_presentation_plan` plus required `slide_index` and optional `format` ∈ {`svg`, `png`, `both`}. | `preview_slide_wireframe` | (MCP-only) |
| Generate the PPTX (accepts `strict_fit` + `fit_report` + optional `strict_unknown_keys` for fail-fast on typo'd fields) | `generate_presentation` | `json2pptx generate [--strict-unknown-keys]` |
| Apply targeted fixes to a single slide (uses the `Fix.Kind` vocabulary fit-report emits) | `repair_slide` | (CLI inlines) |
| **Translate fit/visual QA findings into ranked `repair_slide` fix directives** — accepts FitFinding-shaped and visualqa-shaped findings (mixed input is fine). Returns directives grouped by slide and ranked by severity/action, plus a per-slide `batch_tool_call` ready to submit to `repair_slide`. Does **not** mutate the deck. Findings with no repair mapping (e.g. `image_quality`, `aspect_ratio`, `border_style`) are returned under `unmapped[]`. | `propose_repairs` | (MCP-only) |
| Score a generated deck (0-100 with structured findings) — accepts optional `slide_indices: [int]` to render + score only the listed slides | `score_deck` | (CLI inlines) |
| **Rank candidate slide_jsons for one slot without rendering** — predicts each candidate's score from static analysis plus a rhythm penalty for pattern runs at the target slide position. | `score_candidates` | `json2pptx score-candidates` |
| **Inspect rendered slide images with Claude vision** — returns structured findings whose `suggested_fixes[]` are pre-mapped to `repair_slide` fix kinds. When `ANTHROPIC_API_KEY` is set the report uses vision; when unset it degrades to a pure-Go heuristic pass (all findings P3). | `inspect_slide_images` | `testrand qa` |
| Render one slide to a PNG image (preferred over `pptx2jpg` shell-out) | `render_slide_image` | `pptx2jpg` |
| **Render one slide directly from its JSON** — skips full-deck generation by wrapping the slide in a synthetic single-slide deck. Cached by `(slide_json + template_content + density)`. Pass `overlay=true` to composite a diagnostic overlay on top of the rendered PNG. | `render_slide_image_from_json` | `json2pptx render-slide-from-json` |
| Render the whole deck as thumbnails (preferred over `pptx2jpg` shell-out) | `render_deck_thumbnails` | `pptx2jpg` |
| **Read back a generated PPTX as structured JSON** — best-effort extraction of placeholders, shapes, tables, and speaker notes. | `read_presentation` | (CLI inlines) |
| **Validate a generated PPTX file** — runs OPC package integrity + OOXML content checks against an on-disk `.pptx`. | `validate_presentation_output` | (CLI inlines) |
| Browse pattern catalog | `list_patterns` | `json2pptx patterns list` |
| Show a pattern's value schema | `show_pattern` | `json2pptx patterns show <name>` |
| Validate a pattern's input values | `validate_pattern` | `json2pptx patterns validate` |
| Expand a pattern (preview the `shape_grid` + run table-density checks; returns `density_warnings`, `bounds_source`). Pass `theme_template` (MCP) or `--template` (CLI) for template-aware layout bounds. | `expand_pattern` | `json2pptx patterns expand` |
| **Batch-expand N patterns against the agent's content under a single template load.** Returns each candidate's full expansion + `cell_budgets[]` + `capacity_warnings[]` + `layout_suggestions[]`. Use after `recommend_pattern` to compare candidates head-to-head. | `expand_patterns` | (MCP-only; CLI users loop `json2pptx patterns expand`) |
| Analyze deck rhythm — pattern runs, density variation, accent balance, composition score (lightweight, pre-generation) | `analyze_deck_rhythm` | `json2pptx analyze-rhythm` |
| Table density reference (TDR) — font size + row-count guidance per template/style. CLI `--json` emits the same envelope as the MCP tool (use `--template` / `--style-id` to scope). | `table_density_guide` | `json2pptx tables guide [--json] [--template <name>] [--style-id <id>]` |
| Icon catalog | `list_icons` | `json2pptx icons list` |
| Chart capability metadata (limits, density behavior, label strategy per type) | `get_chart_capabilities` | (CLI inlines in skill-info) |
| Diagram capability metadata (max nodes, overflow behavior, required fields per type) | `get_diagram_capabilities` | (CLI inlines in skill-info) |
| List named `table_styles`/`cell_styles` registered for a template (read-only) | `list_template_settings` | (CLI inlines) |
| Register a named `table_style` or `cell_style` (**write — gated**) | `register_template_setting` | (CLI inlines) |
| Delete a named template setting (**write — gated**) | `delete_template_setting` | (CLI inlines) |

**Contract drift detection.** Call `get_capabilities` once per session to fetch `schema_version`, the live tool list, deprecated fields, and feature flags (`features.strict_fit`, `compact_responses`, `fit_report`, `strict_unknown_keys`, `named_patterns`, `template_settings`). Compare `schema_version` against the value you cached last session — a major bump means breaking changes and you should re-read this skill. Prefer `features.strict_fit` and `features.named_patterns` over hardcoding mode lists.

**Compact responses (server-driven).** The server unconditionally advertises `experimental.compact_responses: true` in its `initialize` response. There is no client-side opt-in.

**Write tools are gated.** `register_template_setting` and `delete_template_setting` require the `JSON2PPTX_ALLOW_SETTINGS_WRITE=1` environment variable on the server. Without it, both return `SETTINGS_WRITE_DISABLED`. Check `get_capabilities().features.template_settings` before attempting writes.

**Digest protocol.** `list_templates` returns `data_format_hints_digest` instead of the inline hints payload. Reuse the digest across calls; fetch the full hints only when the digest changes via `get_data_format_hints{digest: "..."}`. The same digest protocol applies to `get_input_schema{digest: "..."}`.

**Pagination (discovery tools).** `list_templates`, `list_patterns`, and `list_icons` accept optional `cursor` and `page_size` arguments (default 50, max 200). The response envelope echoes `total_count`, `page_size`, and `next_cursor` when more entries remain.

**Input schema introspection.** Call `get_input_schema` to discover the full `PresentationInput` JSON Schema derived from the live Go structs. Each field is annotated with `x-field-scope` (`deck`, `slide`, `content`, `shape`, or `split`). Enum-constrained fields include inline `enum` arrays. The `slides[]` item schema is a `oneOf` between a regular `SlideInput` and a `SplitSlideInput` envelope (discriminator: `type == "split_slide"`), so agents can author either variant directly from schema output. `SlideInput` also carries type-level `anyOf` (`layout_id` OR `slide_type` is required) and `allOf` (`pattern` / `shape_grid` / `compose` are mutually exclusive visual-envelope alternatives). `ContentInput` encodes the typed-value discriminator as an `allOf` of `if`/`then` branches: setting `type` to `text` requires `text_value` and forbids the other typed `*_value` fields; `type: bullets` requires `bullets_value`; and so on through `body_and_bullets`, `body_and_lead`, `bullet_groups`, `table`, `chart`, `diagram`, and `image`. The legacy raw `value` field remains accepted for backward compatibility and is unconstrained by the discriminator.

**Chart and diagram capabilities.** `list_templates` includes `chart_capabilities` and `diagram_capabilities` arrays. Each entry carries an optional `aliases` array listing alternate names. Some diagram types have `status: "stub"` indicating the renderer exists but is not yet production-hardened.

**Isolated diagram validation.** The separate `svggen-mcp` server exposes `validate_diagram` for checking a diagram payload in isolation. Per-error `next_tool_call` routes shape errors to `get_diagram_schema` and constraint errors back to `validate_diagram`.

---

## Visual Decision Ladder

When building a slide and unsure which visual approach to use, follow this decision order:

1. **`recommend_visual`** — the unified entry point. Ranks candidates across *all* categories (placeholder layouts, named patterns, charts, diagrams, compose envelopes, raw shape_grid). Start here.

   Compose envelopes accept 2 to **N** top-level segments — the enforced cap is published as `get_capabilities().features.compose.max_segments` (default 8) along with the supported `directions` and `supports_smart_compose` flag. For arrangements that exceed the cap, nest a compose envelope inside a segment instead of flattening: a `SegmentInput` may set `compose` (XOR with `pattern`) to host a child envelope. Nesting depth and total leaf count caps are published under the same `compose` feature block.

   **Diagram segments.** A `SegmentInput` may set `diagram: {type, data, style?}` as a third XOR alternative to `pattern` / `compose`. Diagram segments let a native pattern coexist with an svggen chart/diagram on the same slide without flattening the pattern through a single-cell grid.

   **Compose-candidate discovery.** `recommend_visual` emits candidates with `category == "compose"` when the intent contains a multi-pattern keyword or the top two pattern candidates declare mutual compose-affinity. The candidate's `placement.composable_with` carries the **specific pair of sibling pattern names** to drop into a `ComposeInput.segments[]`.

   **Envelope-level banner and callout.** A `ComposeInput` may set `banner: {text, emphasis?, accent?}` and/or `callout: {text, emphasis?, accent?}`. These do **not** consume a segment slot. Constraint: validation rejects `banner` when the first segment's pattern is itself banner-leading (currently `strategy-house` and `pull-quote`).

   **Slide-level overlays.** `SlideInput.overlays: []OverlayShape` adds free-floating shapes rendered on top of the grid (arrows, lines, badges). Endpoints are either `{x, y}` percentages or `{anchor_cell: {row, col, at}}` with anchor positions like `"center"`, `"top-left"`, etc. Arrow overlays whose endpoints both target `anchor_cell` with `at: "center"` on text-bearing cells are auto-routed to the cell corners facing the opposite endpoint, keeping arrowheads off the label centers. Arrow stroke colors with poor contrast (<3:1 WCAG AA Large) against an endpoint cell's resolved fill are auto-flipped to white or near-black, whichever has the best worst-case contrast across both endpoints.
2. **`recommend_pattern`** — use only when you already know you need a named pattern and want to pick the best one.
3. **`list_patterns` / `show_pattern`** — use when you already know the pattern name and need its value schema.

**Do not jump straight to `recommend_pattern`** unless you are certain the slide needs a named pattern.

### `blank` vs `content` — Choosing the Right Layout

| Layout | Placeholders | Use when |
|--------|-------------|----------|
| `content` | `title` + `body` | Slide content goes into the body placeholder — text, bullets, charts, tables, or diagrams |
| `blank` | `title` only | Slide content is a `shape_grid` or `pattern` — no body placeholder; all visual content is rendered as positioned shapes below the title |

`content` is body-capable: the engine populates a body placeholder with your content item. `blank` is shape-grid-oriented: there is no body placeholder, so all content must come from `shape_grid` or `pattern`. Setting `slide_type: "blank"` (or omitting `layout_id` when `shape_grid`/`pattern` is present) triggers auto-selection of a blank layout with title and computed grid bounds.

---

## Workflow: Plan → Vary → Render → Repair

### PRECONDITION: Validate Before You Generate

**You MUST NOT call `generate_presentation` until all of the following have succeeded for the current deck:**

1. **Visual discovery.** For each slide, call `recommend_visual` to determine the best visual approach. If you already know the slide needs a named pattern, `recommend_pattern` or `list_patterns` is sufficient. Do not guess pattern names from memory.
2. **Schema inspection.** For each chosen pattern, call `show_pattern` to retrieve the value schema and `example_values`.
3. **Density pre-flight.** For each pattern slide, call `expand_pattern` with your populated values to confirm density is in the 60–110% optimal band.
4. **Input validation.** Once the full deck JSON is assembled, call `validate_input` (with `fit_report: true`) to catch schema errors, unknown keys, scope mistakes, and fit issues.

The six most common first-attempt failures (wrong content shape for a pattern, missing geometry fields, misspelled overrides, wrong row format, scope confusion, field-name typos) are all caught by steps 1–4 above before any PPTX is produced. Skipping these is a workflow violation; at minimum, always run `validate_input`.

**The sequence in practice:**

```
recommend_visual (per slide intent) →  pick visual approach (layout, pattern, chart, diagram)
show_pattern (per pattern)          →  learn value schemas + example_values
expand_pattern (per pattern slide)  →  confirm density, get cell_budgets
validate_input (full deck JSON)     →  catch schema + fit errors
generate_presentation               →  only after steps above pass
```

### 4-phase overview

Full details for each phase live in [WORKFLOW.md](WORKFLOW.md). One-line summary:

1. **PLAN** — produce a short outline (template, accent strategy, slide-by-slide list of layouts + patterns + accents). Use `plan_deck` for decks >4 slides.
2. **VARY** — call `analyze_deck_rhythm` and act on `longest_run`, `accent_balance`, `density_cv`, `composition_score`.
3. **RENDER** — generate the JSON in one pass; verify the pre-emit checklist (Rule 20, semantic fills, gap ≥4pt, accent variety, 60–110% density).
4. **REPAIR** — `validate_input` → `generate_presentation` → `render_slide_image` / `render_deck_thumbnails` → `inspect_slide_images` → `repair_slide`. Images are truth.

For the `repair_slide` fix-kind vocabulary, finding-code catalog, and strict-fit promotion ladder, see [FINDINGS.md](FINDINGS.md).

---

## Pattern Library (overview)

For BMC, KPI grids, 2x2 matrices, timelines, card grids, icon rows, two-column comparisons, accent-banded panels (`stylish-panels`), strategy-house frameworks, executive SCQA summaries (`scqa-summary`), and chart-with-takeaway layouts (`chart-insights-split`), use json2pptx's named patterns. Named patterns expand to validated `shape_grid` structures at generation time, replacing ~600 tokens of boilerplate with ~100 tokens.

**Tip — chart + narrative on the same slide.** `chart-insights-split` is the canonical "data on the left, interpretation on the right" consulting layout: pass a `chart` (any `types.DiagramSpec` shape) plus 1–6 `insights` bullets. If you ship the pattern without a `chart`, the engine renders insights full-width and emits `CHART_PLACEHOLDER_EMPTY` (`action: review`) so you know the panel collapsed — supply a chart or swap to an insights-only pattern.

**Tip — executive problem framing.** `scqa-summary` lays out the classic consulting Situation / Complication / Questions / Answer arc as a 4-row, 20%/80% split. Each row's body accepts either a string or a 1–4 item array of bullets, so the same pattern works for both terse one-liners and dense multi-bullet content.

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

**Do NOT hand-roll shape grids when a named pattern exists.**

See [PATTERNS.md](PATTERNS.md) for the full pattern workflow: catalog browsing, `text_budget_guide`, Text Capacity Awareness (density bands, decision rules), bounds override (`bounds`, `max_height_pct`), and density-class divergence warnings.

---

## Rules (overview)

Non-negotiable. Full catalog with rationale and examples in [RULES.md](RULES.md). Highlights:

- **Shape grid (Rules 1–7):** col_spans must sum per row; `bounds` are percentages; gaps are typographic points; one content type per cell (except `composite`/`pattern`/`grid` slots); body text cells need all 4 insets.
- **Charts (Rules 8–10):** `series[i].values` length equals `len(categories)`; chart types use underscores (`stacked_bar`, NOT `stacked-bar`); don't mix data formats.
- **Content & layout (Rules 11–15):** `layout_id` must be canonical (`title`, `content`, `blank`, `section`, `closing`, `two-column`, …); semantic fills (`accent1`, `lt2`, `dk1`) required, never mix with hex on one slide; align values are `"l"`/`"ctr"`/`"r"`/`"just"`, vertical align is `"t"`/`"ctr"`/`"b"`.
- **Contrast auto-fix (Rule 16):** engine auto-replaces low-contrast text with dark gray; surfaces as `contrast_autofixed` findings.
- **Silent traps (Rules 17–19):** `footer` is an object not a string; never prefix `source` with "Source: "; content fields need `_value` suffix (`chart_value`, `table_value`).
- **Table density (Rule 20 — enforced):** MUST split if rows > 7 OR cols > 6 OR font < 9pt. Multiline cells count as N logical rows.
- **No emoji codepoints (hard rule):** emoji glyphs are rejected by pattern validators in `card-grid`, `icon-row`, `herodetail`, etc. Use a bundled SVG icon name or supply a user icon via `path` / `url` / `svg_data`. See the [Icon Names](#icon-names) section.
- **Anti-patterns:** two-tables-one-grid, hex-fill mix, pattern monotony (no 3-in-a-row), accent monotony.
- **Cell accent variety:** `cell_accent_mode` ∈ {`uniform`, `alternate`, `progressive`} — use `progressive` for 4+ peer cells.

For the full finding-code catalog (`fit_overflow`, `cell_underfilled`, `placeholder_overflow`, `chart.*` family, render-time codes like `contrast_autofixed`, `text_trimmed`, `diagram_clamped`, etc.) and the `fix.kind` enums, see [FINDINGS.md](FINDINGS.md).

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

**Per-template named settings.** Beyond per-deck `defaults`, you can register named `table_styles` and `cell_styles` per template via `register_template_setting`, then reference them by name from any deck. List existing names with `list_template_settings{template_name}`. Both write tools (`register_template_setting`, `delete_template_setting`) require `JSON2PPTX_ALLOW_SETTINGS_WRITE=1` on the server and return `SETTINGS_WRITE_DISABLED` otherwise; the read tool is always available.

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

**No-emoji policy (hard rule).** **Never emit emoji codepoints anywhere in deck JSON** — not in `icon` fields, not in pattern values (`card-grid` cells, `icon-row` items, `herodetail` etc.), not in shape text, not in titles, bullets, headers, captions, or table cells. Emoji glyphs (`🚀`, `📈`, `✅`, `⚡`, etc.) and pictographic characters in the Unicode emoji range are rejected by pattern validators **and** by a centralized boundary validator in `validate_input` / `generate_presentation` that emits the `no_emoji_violation` diagnostic code (severity: error, action: refuse) with a JSON path to the offending field. Use a **bundled SVG icon name** (preferred) or supply a user icon via `path` / `url` / `svg_data`. Plain Unicode symbols outside the emoji range (e.g. arrows like `→`, `←`) are still allowed in text but should not appear in icon fields.

Call `list_icons` (MCP) or run `json2pptx icons list` (CLI) for all available icons. Use `"icon": {"name": "ICON_NAME", "fill": "#FFFFFF"}` inside a shape, or `"icon": {"name": "ICON_NAME"}` as a standalone cell. The `"fill"` color override also works with custom SVG icons specified via `"path"`: `"icon": {"path": "icons/custom.svg", "fill": "#FF6600"}`.

**Canonical identifier.** Each entry in the discovery response (`list_icons` MCP, `json2pptx icons list --json` CLI) exposes a `qualified_name` field in `<set>:<name>` form (e.g. `"outline:chart-pie"`, `"filled:chart-pie"`). Use `qualified_name` directly as `icon.name` in deck JSON. This is required for filled icons — a bare `"chart-pie"` resolves to the outline set; you must write `"filled:chart-pie"`. Outline icons accept either the bare name or the `outline:` prefix. The legacy `names[]` array (bare names) is kept for backward compatibility but does not disambiguate sets.

**Bundled name preflight.** `validate_input` and `generate_presentation` preflight every `icon.name` against the bundled registry. Unknown names emit `ICON_BUNDLED_NAME_UNKNOWN` (severity: error) with `details.suggestions` — a ranked list of Levenshtein-closest matches (or qualified cross-set forms when the bare base name only resolves in the non-default set). Use `suggestions[0]` to repair the name without a separate `list_icons` round-trip.

**Local asset path preflight.** `validate_input` and `generate_presentation` resolve every relative local asset path against the JSON input directory (CLI) or server CWD (MCP) before generation. Coverage spans `icon.path` (shape grid icons), `image_value.path` (content images), `cells[].image.path` (shape grid cell images), and `background.image` (slide background). Each broken reference becomes its own structured finding so agents see every failure in one pass:

| Code | Surface |
|---|---|
| `ICON_PATH` | `icon.path` resolution failure (missing file, symlink escape, traversal) |
| `IMAGE_PATH` | `image_value.path` or shape-grid cell `image.path` resolution failure |
| `BACKGROUND_IMAGE_PATH` | `slide.background.image` resolution failure |

All findings carry `details.input_value`, `details.slide_index`, and a JSON Pointer `path` (e.g. `/slides/0/content/0/image_value/path`) so the offending node round-trips through jq/jsonpath. Unsupported extensions (anything outside `.png .jpg .jpeg .gif .svg .bmp .tiff .tif .webp` for images; `.svg` for icons) are rejected before disk I/O. Use absolute paths if your asset lives outside the input directory.

**Accepted `IconInput` sources (exactly one per icon).** Set exactly one of:

| Source | Field | Example |
|---|---|---|
| Bundled icon | `name` | `{"name": "chart-pie", "fill": "#FFFFFF"}` |
| Local file (SVG / image) | `path` | `{"path": "icons/custom.svg", "fill": "#FF6600"}` |
| Remote SVG / image | `url` | `{"url": "https://example.com/logo.svg"}` |
| Inline SVG markup | `svg_data` | `{"svg_data": "<svg…>…</svg>", "alt": "…"}` |

**Inline SVG (`svg_data`).** When you already have SVG markup — e.g. the output of `svggen-mcp.render_diagram` — embed it directly in a cell without a filesystem roundtrip:

```json
"icon": {"svg_data": "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 100 100\">…</svg>", "alt": "Pie chart: A 60%, B 40%"}
```

`fill` is ignored for `svg_data` (your SVG is assumed pre-styled). The optional `alt` field sets accessibility text; when omitted, alt falls back to a value derived from `name` or `path`, or `"icon"` for inline SVG. Pattern fields that accept icons (e.g. `card-grid`'s cell `icon`, `icon-row` items, `herodetail`'s `icon`) follow the same rule: bundled name or a loadable source — never an emoji glyph.

**Accent on icon fill.** Prefer semantic theme colors (`accent1`–`accent6`, `dk1`, `lt1`) for `fill` so the icon adapts to the template's palette. Hex (`#RRGGBB`) is allowed only when the surrounding slide is already on a hex-allowlisted brand palette (see Rule 12 in RULES.md). Do not mix semantic and hex fills on one slide.

---

## Reference

For complete field specifications (connectors, accent bars, callout geometries, speaker notes, footers, backgrounds, theme overrides, patch operations, all chart/diagram types, and more), see `../template-deck/TEMPLATE_GUIDE.md` or run `json2pptx validate-template <path>`.
