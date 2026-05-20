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

This skill is split into focused sub-files. SKILL.md (this file) covers preconditions, the 5-tool quick reference, and the workflow overview. Load the sub-files when you need their detail:

| File | Contents |
|---|---|
| [TOOLS.md](TOOLS.md) | Full `json2pptx-mcp` tool catalogue (40+ rows) with MANDATORY / SKIPPABLE markers per phase, plus contract-drift, pagination, schema-introspection, and gated-write semantics |
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

## Quick Pattern Selector

Skim before reading the pattern catalogue. For ambiguous cases, fall through to `recommend_visual`.

```
PICK YOUR LAYOUT
─────────────────────────────────────────────────────────────────
Framing / exec summary
  SCQA narrative                       → scqa-summary
  strategic pillars + base (+ roof)    → strategy-house
Data + interpretation
  chart + so-what bullets              → chart-insights-split
  ranked bars + per-bar insight        → horizontal-bar-with-callouts
  P&L walk / cost bridge               → waterfall-bridge
  one big number (± details)           → stat-hero / hero-detail
  2-6 KPIs (or supporting bar)         → kpi-2up … kpi-6up, kpi-inline
Compare 2 options / states
  side-by-side text                    → comparison-2col
  before / after transition            → before-after (or -compact)
  2×2 axes positioning                 → matrix-2x2  ·  svggen: matrix_2x2
Process / sequence
  linear steps (3-8)                   → process-flow (or -compact)
  phases + dates                       → phase-roadmap
  workstreams × phases                 → roadmap-phased
  cross-actor / cross-functional       → swimlane
  date-based milestones                → timeline-horizontal
  value / supply chain (4-10 steps)    → value-chain
  maturity ladder (3-6 stages, current state) → journey-maturity-model
  gantt schedule data                  → svggen: gantt
Cause-effect / structure
  fishbone / Ishikawa                  → svggen: fishbone
  architecture / tech stack            → arch-stack
  narrowing hierarchy / panels         → pyramid / stylish-panels
People / org / distribution
  org hierarchy (top-down)             → svggen: org_chart
  team bios with photos                → team-bios
  treemap / venn / funnel              → svggen: treemap, venn, funnel
Catalog / agenda / quote
  N×M titled cards                     → card-grid
  3-5 icon + caption pairs             → icon-row
  full 9-cell Osterwalder BMC          → bmc-canvas
  plain numbered agenda                → agenda
  agenda with images or quotes         → agenda-with-images
  single pull-quote                    → pull-quote
  3-8 stakeholder quote bubbles        → quote-cluster
─────────────────────────────────────────────────────────────────
RULE: prefer diagram types (svggen-rendered) over shape_grid patterns
when a data-driven or topologically complex diagram is needed.
Use shape_grid patterns for structural/text layouts.
```

---

## Connected MCP servers

This skill talks to **two** independent MCP servers. Both must be reachable for the workflow described below to function end-to-end.

### `json2pptx-mcp` — deck-level engine

Builds, validates, and repairs whole PPTX presentations. Owns templates, layouts, patterns, fit-report, and the `repair_slide` apply-only fix vocabulary. The 5-tool quick reference for this server is in the [MCP Tools (most-used)](#mcp-tools-most-used) section below; the full 40+ tool catalogue with phase markers lives in [TOOLS.md](TOOLS.md).

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

## MCP Tools (most-used)

The five tools below cover the precondition workflow (`recommend_visual` → `show_pattern` → `expand_pattern` → `validate_input` → `generate_presentation`) plus `repair_slide` for fix-up. For the full 40+ tool catalogue — including session/discovery (`get_started`, `get_capabilities`, `get_input_schema`, `list_templates`, `resolve_theme`, …), rhythm/scoring (`analyze_deck_rhythm`, `score_candidates`, `score_deck`), preview/render (`preview_presentation_plan`, `preview_slide_wireframe`, `render_slide_image`, `render_deck_thumbnails`, `inspect_slide_images`), the gated write tools (`register_template_setting`, `delete_template_setting`), and the MANDATORY / SKIPPABLE markers per phase — see **[TOOLS.md](TOOLS.md)**. The six `svggen-mcp` tools (`render_diagram`, `list_diagram_types`, `validate_diagram`, `get_diagram_schema`, `get_capabilities`, `get_started`) are documented under [Connected MCP servers](#connected-mcp-servers).

| Tool | Phase | When to call |
|---|---|---|
| `recommend_visual` | PLAN | First call per slide intent — ranks candidates across layouts, patterns, charts, diagrams, and raw shape_grid. Start here when unsure which visual approach fits. |
| `show_pattern` | PLAN | Per chosen pattern — returns the value schema and `example_values`. When `supports_callout: true`, the response also carries a `callout_schema` fragment for the envelope-level `callout` DTO. |
| `expand_pattern` | PLAN | Per pattern slide — preview the resolved `shape_grid` plus `density_warnings`, `cell_budgets`, and `bounds_source`. Confirm density lands in the 60–110% optimal band before generating. |
| `validate_input` | RENDER | Cheapest precondition gate — full-deck schema + optional `fit_report` + optional `strict_unknown_keys` for fail-fast on typo'd fields. Always run before `generate_presentation`. |
| `generate_presentation` | RENDER | Render the PPTX. Defaults to `output_validation: "strict"` (see [Output Validation Guarantee](#output-validation-guarantee)); `strict_fit` controls overflow promotion (see [FINDINGS.md](FINDINGS.md)). |
| `repair_slide` | REPAIR | Apply targeted fixes to a single slide using the `Fix.Kind` vocabulary fit-report emits. For multi-slide fixes, run `propose_repairs` first to translate findings into ranked directives. |

**Compact responses.** The server advertises `experimental.compact_responses: true` in its `initialize` response; compaction itself is controlled by client opt-in (the client sends `experimental.compact_responses: true` in its capabilities) or the deprecated `MCP_COMPACT_RESPONSES=1` environment variable.

---

## Output Validation Guarantee

**The zero "needs repair" contract.** `generate_presentation` (MCP) and `json2pptx generate` (CLI) default `output_validation` / `--output-validation` to `strict`. In strict mode the engine runs the full OPC + OOXML validator (`internal/pptx.ValidateOutputFile`) against the freshly-written `.pptx` and **refuses to return success on any blocking finding**. Every successful generate response therefore implies a structurally clean file — agents do not need a separate `validate_presentation_output` call to confirm.

A blocking finding means PowerPoint or Keynote would show the "we found a problem with some content, do you want us to repair" prompt when opening the file. The validator covers:

| Phase | Validator | Sample codes |
|-------|-----------|--------------|
| `opc` | `structural` | `OPC_MISSING_PART`, `OPC_DANGLING_REL`, `OPC_DUPLICATE_REL_ID`, `OPC_MISSING_ELEMENT`, `OPC_MALFORMED_XML`, `OPC_MISSING_CONTENT_TYPE`, `OPC_MISSING_CONTENT_TYPE_OVERRIDE` |
| `ooxml` | `ooxml_content` | `OOXML_INVALID_COLOR`, `OOXML_INVALID_SCHEME`, `OOXML_DUPLICATE_ID`, `OOXML_INVALID_TABLE`, `OOXML_ZERO_EXTENT`, `OOXML_ILLEGAL_XML_CHAR`, `OOXML_SLIDE_COUNT_MISMATCH`, `OOXML_EMPTY_REQUIRED_ATTR` |

`OPC_*` and the two structural-corruption `OOXML_*` codes (`OOXML_ILLEGAL_XML_CHAR`, `OOXML_SLIDE_COUNT_MISMATCH`) are always promoted to `severity: "blocking"`. Other `OOXML_*` codes are advisory `warning`s and do not fail strict mode unless the validator escalates them.

### Error envelope shape

When strict validation fails, the tool returns an error `CallToolResult` (`isError: true`) whose structured content is:

```json
{
  "summary": "output validation failed: 1 blocking, 0 warning finding(s)",
  "findings": [
    {
      "code": "OOXML_INVALID_COLOR",
      "severity": "blocking",
      "path": "ppt/slides/slide3.xml",
      "phase": "ooxml",
      "validator": "ooxml_content",
      "slide_index": 2,
      "source_path": "/slides/2/shape_grid/cells/4/style/fill",
      "scope": "generator",
      "message": "..."
    }
  ],
  "next_tool_call": {
    "tool": "repair_slide",
    "args_template": {"slide_index": 2, "fixes": []}
  }
}
```

Every finding carries a `scope` field classifying responsibility:

| `scope` | Meaning | Agent response |
|---------|---------|----------------|
| `source` | The bug is in the input JSON (bad color, malformed table). | Repair via `repair_slide` (e.g. `replace_color`, `use_semantic_color`). |
| `template` | The bug is in the `.pptx` template (missing layout part, dangling rel). | Switch templates or report; cannot be fixed via `repair_slide`. |
| `generator` | The bug is in the engine. | Report — do not retry; an automated repair is unlikely to help. |

### Responding to a validation error

1. **Inspect every blocking finding's `code` and `scope`.** `scope: "source"` is repairable; `template` and `generator` usually are not.
2. **Use `next_tool_call.args_template` as a starting point.** When every blocking finding pins to one slide, `slide_index` is populated; otherwise it is `-1` and you must fill it in from `findings[].slide_index`. The `fixes` array is empty because output-validation codes do not share a single canonical fix kind.
3. **Look up unfamiliar codes** in `internal/pptx/output_validator.go` (`opcCodeMap`, `ooxmlCodeMap`) before guessing a remedy. Each finding's `message` field also explains why it fired.
4. **Pick the right `repair_slide` directive** based on the finding's `code` and `source_path`. Common mappings:
   - `OOXML_INVALID_COLOR` / `OOXML_INVALID_SCHEME` → `replace_color` or `use_semantic_color`
   - `OOXML_ILLEGAL_XML_CHAR` → `reduce_text` after stripping the offending byte
   - `OOXML_DUPLICATE_ID` → regenerate the slide (call `generate_presentation` again; this is usually a generator bug worth reporting)
5. **Submit the repair**, then re-run `generate_presentation`. The strict gate runs again on the new output.

### Override modes

| Mode | Behavior | Use when |
|------|----------|----------|
| `strict` (default) | Run validation; block on any blocking finding. | Always, unless you have a specific reason to override. |
| `warn` | Run validation; surface findings in the `output_validation_findings[]` array on the success envelope; never block. | Diagnosing template issues without losing the generated file. |
| `off` | Skip validation entirely. | One-off renders where you accept the "needs repair" risk. |

Set the override per-call: MCP `{"output_validation": "warn"}` or CLI `--output-validation=warn`.

### Where the codes live

- Code definitions and severity classification: `internal/pptx/output_validator.go` (`opcCodeMap`, `ooxmlCodeMap`, `blockingOOXMLCodes`).
- Validator implementation: `internal/pptx/output_validator.go` (`OutputValidator.Validate`) composes the structural OPC `Validator` (`internal/pptx/validator.go`) and the `OOXMLValidator` (`internal/pptx/ooxml_validate.go`).
- Corpus headless-open regression test: `cmd/json2pptx/corpus_headless_test.go` opens every `examples/*.json` deck in headless LibreOffice and fails CI on any repair warning.

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

For BMC, KPI grids, 2x2 matrices, timelines, card grids, icon rows, two-column comparisons, accent-banded panels (`stylish-panels`), strategy-house frameworks, executive SCQA summaries (`scqa-summary`), chart-with-takeaway layouts (`chart-insights-split`), ranked horizontal bars with per-bar callouts (`horizontal-bar-with-callouts` — 3–8 bars; each bar binds to one insight via a left accent bar), waterfall / bridge bar charts (`waterfall-bridge` — 3–10 columns of total + delta + subtotal bars showing how components reconcile a start total to an end total; floating delta bars and auto-computed subtotals), Porter-style value chains (`value-chain` — 4–10 step columns with per-step description and optional highlight), maturity ladders (`journey-maturity-model` — 3–6 stage columns with numbered headers, descriptions, and an optional 'where we are' marker), visual deck previews (`agenda-with-images` — 3–6 numbered agenda rows each with title/subtitle and an image or quote placeholder), and team / 'Our People' pages (`team-bios` — 1–8 members, each with a photo placeholder above name + role + short bio, up to 4 per row), use json2pptx's named patterns. Named patterns expand to validated `shape_grid` structures at generation time, replacing ~600 tokens of boilerplate with ~100 tokens.

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

## Deck Chrome & Section Structure

Two opt-in top-level fields raise the deck from a flat slide list to a structured presentation with persistent chrome. Both are surfaced under `get_capabilities.features` (`deck_chrome`, `page_numbers`, `section_structure`, `section_crumb`) with version + one-line usage hints so you can capability-gate without re-reading this section.

**`chrome` — deck-wide footer chrome (since `2.8.0`).** Composites a footer line from `confidentiality`, `client_name`, `project_code`, and `footer_date`, and overlays slide numbers via `chrome.page_numbers`. Chrome is auto-suppressed on title and closing slides.

```json
{
  "chrome": {
    "confidentiality": "Strictly confidential",
    "client_name": "Acme Corp",
    "project_code": "Aurora",
    "footer_date": "May 2026",
    "page_numbers": {
      "enabled": true,
      "format": "{current} / {total}",
      "skip": ["title", "closing"]
    },
    "section_crumb": true
  }
}
```

- `chrome.page_numbers.format` supports `{current}` and `{total}` placeholders. Default skip set is `["title", "closing"]`.
- `chrome.section_crumb: true` surfaces the current section title in the footer — it only resolves when the deck also sets `structure.sections[].title`.

**`structure` — deck-level section grammar (since `2.7.0`).** Replaces a flat `slides[]` list with named sections plus an optional cover, closing, and auto-generated agenda. The engine expands `structure` into a flat slide sequence with auto section dividers.

```json
{
  "structure": {
    "cover":   {"layout_id": "slideLayout1", "content": [...]},
    "closing": {"layout_id": "slideLayout1", "content": [...]},
    "auto_agenda": true,
    "sections": [
      {"title": "Situation",     "slides": [...]},
      {"title": "Recommendation", "slides": [...]}
    ]
  }
}
```

- `structure` is **mutually exclusive** with a top-level `slides` — pick one.
- `auto_agenda: true` inserts an agenda slide listing every section title after the cover (requires ≥ 2 sections).
- Pair with `chrome.section_crumb: true` so the running section title appears in the footer for every content slide.

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

**Preview a single icon (`preview_icon`).** Before committing a custom-SVG path/URL or a recolored bundled icon to a deck, call `preview_icon` with the same `IconInput` shape you'd drop into a shape grid cell. It returns `svg_data` (with `fill` applied for non-inline sources), `png_base64` (rasterized preview), `alt`, `source_kind` (`bundled` / `path` / `url` / `inline`), and `qualified_name` for bundled icons — no template load, no full generation cycle. Pass `base_dir` whenever `icon.path` is relative. The `fill` override is ignored for inline `svg_data` (a warning is returned). The CLI counterpart is `json2pptx preview-icon` (accepts `--name`/`--path`/`--url`/`--svg-data` flags or an `--icon` JSON file/`-`).

**Canonical identifier.** Each entry in the discovery response (`list_icons` MCP, `json2pptx icons list --json` CLI) exposes a `qualified_name` field in `<set>:<name>` form (e.g. `"outline:chart-pie"`, `"filled:chart-pie"`). Use `qualified_name` directly as `icon.name` in deck JSON. This is required for filled icons — a bare `"chart-pie"` resolves to the outline set; you must write `"filled:chart-pie"`. Outline icons accept either the bare name or the `outline:` prefix. The legacy `names[]` array (bare names) is kept for backward compatibility but does not disambiguate sets.

**Bundled name preflight.** `validate_input` and `generate_presentation` preflight every `icon.name` against the bundled registry. Unknown names emit `ICON_BUNDLED_NAME_UNKNOWN` (severity: error) with `details.suggestions` — a ranked list of Levenshtein-closest matches (or qualified cross-set forms when the bare base name only resolves in the non-default set). Use `suggestions[0]` to repair the name without a separate `list_icons` round-trip.

**Local asset path preflight.** `validate_input`, `generate_presentation`, `preview_presentation_plan`, and `render_slide_image_from_json` resolve every relative local asset path before generation. The CLI uses the JSON input directory as the base; MCP tools take an explicit `base_dir` parameter (absolute path to an existing directory), falling back to the server's process CWD when omitted. **Always send `base_dir` from MCP** so the same JSON works regardless of how the server was launched — otherwise relative paths in your deck are silently coupled to the server's working directory. A malformed `base_dir` (relative, missing, or not a directory) is rejected with `INVALID_PARAMETER` (`path: "base_dir"`) before any per-asset findings. Coverage spans `icon.path` (shape grid icons), `image_value.path` (content images), `cells[].image.path` (shape grid cell images), and `background.image` (slide background). Each broken reference becomes its own structured finding so agents see every failure in one pass:

| Code | Surface |
|---|---|
| `ICON_NOT_FOUND` | `icon.path` file does not exist after resolution |
| `ICON_PATH_EXT_INVALID` | `icon.path` extension is not `.svg` |
| `ICON_PATH_TRAVERSAL` | `icon.path` contains `..` components (rejected pre-clean) |
| `ICON_PATH_SYMLINK_ESCAPE` | `icon.path` is relative but resolves outside the base directory via a symlink |
| `ICON_PATH` | Other `icon.path` resolution failures (symlink loop, permission denied, etc.) |
| `IMAGE_PATH` | `image_value.path` or shape-grid cell `image.path` resolution failure |
| `BACKGROUND_IMAGE_PATH` | `slide.background.image` resolution failure |
| `ASSET_PATH_ENV_UNSET` | Any local asset path references `$VAR`/`${VAR}` whose environment variable is not set; `details.env_variable` names the missing var |
| `ASSET_TOO_LARGE` | Any local asset (`icon.path`, `image_value.path`, shape-grid cell `image.path`, `background.image`) or inline `icon.svg_data` exceeds the soft (warning) or hard (error) cap for its media kind. `details` carries `size_bytes`, `soft_cap_bytes`, `hard_cap_bytes`, `exceeded_cap` (`soft`/`hard`), and `media_kind` (`svg`/`raster`). See `get_capabilities.features.asset_limits` for active thresholds and override env vars. |
| `URL_FETCH_FAILED` | Any `url` field (background, image, icon, shape.icon, content image_value) could not be downloaded, exceeded the 50 MB cap, or returned the wrong content type |
| `SVG_INVALID_ROOT` | Remote SVG fetched successfully but the document's root element is not `<svg>` (e.g. HTML, generic XML); cached only after passing strict validation |
| `SVG_UNSAFE_XML` | Remote SVG declares a `<!DOCTYPE ...>` or `<!ENTITY ...>`; rejected pre-cache as an XXE / billion-laughs carrier regardless of payload |
| `SVG_PARSE_ERROR` | Remote SVG is malformed XML (unbalanced tags, references to undeclared entities, non-XML content, empty body) |

All findings carry `details.input_value` (local paths) or `details.input_url` (URLs), `details.slide_index`, and a JSON Pointer `path` (e.g. `/slides/0/content/0/image_value/path`) so the offending node round-trips through jq/jsonpath. Unsupported extensions (anything outside `.png .jpg .jpeg .gif .svg .bmp .tiff .tif .webp` for images; `.svg` for icons) are rejected before disk I/O. Use absolute paths if your asset lives outside `base_dir`, or pass a `base_dir` that contains every referenced asset. `get_capabilities` lists the tools that honor `base_dir` under `features.base_dir`.

**Path expansion.** Local asset paths (`icon.path`, `image_value.path`, shape-grid cell `image.path`, `background.image`) honor two convenience expansions before resolution: a leading `~/` (or bare `~`) expands to the invoking user's home directory, and `$VAR` / `${VAR}` expand via the server's environment. Unset environment variables yield an `ASSET_PATH_ENV_UNSET` finding rather than silently collapsing to an empty string; traversal and symlink protections still apply against the expanded path.

**Asset size caps.** Local SVG/raster files and inline `icon.svg_data` markup are preflighted against per-media-kind size caps. SVG inputs default to a 2 MB soft cap (warning) and 25 MB hard cap (blocking); raster inputs default to 8 MB / 25 MB. A soft-cap breach emits `ASSET_TOO_LARGE` at warning severity and still commits the resolved path so generation proceeds; a hard-cap breach emits it at error severity and the resolved path is not committed (the field stays at its input value so the caller can surface what the agent submitted). Override the thresholds per process with `JSON2PPTX_MAX_SVG_SOFT_BYTES`, `JSON2PPTX_MAX_SVG_HARD_BYTES`, `JSON2PPTX_MAX_RASTER_SOFT_BYTES`, `JSON2PPTX_MAX_RASTER_HARD_BYTES`. Active values and override env-var names are surfaced under `get_capabilities.features.asset_limits` so agents can pre-validate locally.

**URL preflight.** `url` fields on `background`, `image_value`, shape-grid cell `image`, cell `icon`, and nested `shape.icon` are downloaded and validated by both CLI and MCP before generation. SSRF-blocked, unreachable, or content-mismatched URLs surface as one `URL_FETCH_FAILED` per offending field rather than aborting the request, so a deck with several broken remote assets reports them all in one validate / generate call. Remote SVGs additionally pass a strict XML safety check before reaching the cache: payloads whose root is not `<svg>` surface as `SVG_INVALID_ROOT`, payloads that declare a `<!DOCTYPE ...>` or `<!ENTITY ...>` (XXE / billion-laughs carriers) surface as `SVG_UNSAFE_XML`, and malformed XML surfaces as `SVG_PARSE_ERROR`.

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

`fill` is ignored for `svg_data` (your SVG is assumed pre-styled); setting both emits a non-blocking `ICON_FILL_IGNORED_ON_INLINE` warning — either pre-color the inline markup, or switch to `name`/`path` with `fill`. The optional `alt` field sets accessibility text; when omitted, alt falls back to a value derived from `name` or `path`, or `"icon"` for inline SVG. Pattern fields that accept icons (e.g. `card-grid`'s cell `icon`, `icon-row` items, `herodetail`'s `icon`) follow the same rule: bundled name or a loadable source — never an emoji glyph.

**Polymorphic icon slot on pattern cells.** `card-grid`, `kpi-2up`…`kpi-6up`, `kpi-inline`, `matrix-2x2`, `icon-row`, and `hero-detail` accept either a bundled-name string shorthand or a full `IconInput` object. Both are equivalent — pick the form that fits your need:

```jsonc
// Bundled-name shorthand (most common)
{"icon": "rocket"}

// Custom SVG on disk with a fill recolor
{"icon": {"path": "logo.svg", "fill": "#FF0000", "alt": "Acme logo"}}

// Pre-styled inline SVG (no disk I/O, fill is ignored)
{"icon": {"svg_data": "<svg xmlns=\"http://www.w3.org/2000/svg\">…</svg>"}}

// Remote SVG (downloaded via the URL preflight)
{"icon": {"url": "https://cdn.example.com/icons/widget.svg"}}
```

A bare string is classified at parse time: bundled name → `name`, `http(s)://` or `data:` → `url`, `<svg…>` → `svg_data`, path with `/` and `.svg`/`.png`/`.jpg` → `path`. Unknown short strings stay in `name` and are rejected by the bundled-name preflight (`ICON_BUNDLED_NAME_UNKNOWN`). Setting two of `name`/`path`/`url`/`svg_data` in the object form fails validate with `invalid_shape`. The patterns above bumped their `version` to `2` when this slot landed.

**Accent on icon fill.** Prefer semantic theme colors (`accent1`–`accent6`, `dk1`, `lt1`) for `fill` so the icon adapts to the template's palette. Hex (`#RRGGBB`) is allowed only when the surrounding slide is already on a hex-allowlisted brand palette (see Rule 12 in RULES.md). Do not mix semantic and hex fills on one slide.

---

## Reference

For complete field specifications (connectors, accent bars, callout geometries, speaker notes, footers, backgrounds, theme overrides, patch operations, all chart/diagram types, and more), see `../template-deck/TEMPLATE_GUIDE.md` or run `json2pptx validate-template <path>`.
