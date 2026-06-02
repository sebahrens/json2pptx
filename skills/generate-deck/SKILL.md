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

For new content-bearing decks, author a semantic **DeckSpec** and render it with
`render_deck_spec` (CLI `json2pptx semantic render`) — see [Semantic deck specs](#semantic-deck-specs--the-default-authoring-path)
below. Drop to raw `PresentationInput` via `generate_presentation` / `json2pptx generate -json`
only when the user needs a feature outside the semantic schema or a targeted raw repair;
raw JSON is also the compiler's own output format.

This skill is split into focused sub-files. SKILL.md (this file) covers preconditions, the 5-tool quick reference, and the workflow overview. Load the sub-files when you need their detail:

| File | Contents |
|---|---|
| [TOOLS.md](TOOLS.md) | Full `json2pptx-mcp` tool catalogue (40+ rows) with MANDATORY / SKIPPABLE markers per phase, plus contract-drift, pagination, schema-introspection, and gated-write semantics |
| [WORKFLOW.md](WORKFLOW.md) | 4-phase workflow deep dive (Plan, Vary, Render, Repair), visual inspection, `next_tool_call`, response_fingerprint, idempotency_key |
| [FINDINGS.md](FINDINGS.md) | All finding codes (layout + chart), the `fix.kind` enum, the `repair_slide` apply-only superset, strict-fit promotion ladder |
| [RULES.md](RULES.md) | Rules 1–20 (shape grid, charts, content/layout, contrast, silent traps, table density), anti-patterns, cell accent variety |
| [PATTERNS.md](PATTERNS.md) | Pattern library, `text_budget_guide`, Text Capacity Awareness, density bands, bounds override |

Read `../template-deck/TEMPLATE_GUIDE.md` for the complete field reference (content types, chart types, diagram types, shape grid properties, patch operations).

See `examples/four-phase-workflow.md` for a worked end-to-end example of the 4-phase flow.

## Semantic deck specs — the default authoring path

For ordinary business decks, author a compact **DeckSpec** (`meta` + `slides[].kind`) and let
the compiler choose patterns, layouts, accents, and rhythm — a spec is far shorter than the raw
`PresentationInput` it lowers to. Write it as YAML or JSON; both the MCP `spec` arg and the CLI
`--spec` accept either.

**Tools** (MCP `json2pptx-mcp` · CLI `json2pptx semantic <sub>`):

| Step | MCP tool | CLI | Purpose |
|---|---|---|---|
| Discover | `list_deck_archetypes`, `list_slide_kinds` | `semantic schema` | Enumerate `meta.archetype` / `slides[].kind` and each kind's required + typical fields (plus `required_aliases`: required-one-of alias keys, e.g. `kpi_snapshot` accepts `metrics` for `kpis`). `semantic schema` prints the full DeckSpec JSON Schema (draft 2020-12). |
| Validate | `validate_deck_spec` | `semantic validate` | First check: unknown kinds/archetypes, missing required payload fields, rhythm/density advisories. Returns the shared finding envelope; `ok=false` ⇒ ≥1 error-severity finding (`--strict off\|warn\|strict` controls advisory severity). |
| Preview plan | `explain_deck_spec` | `semantic explain` | Read-only projection: resolved archetype/template, deck `rhythm` + `rhythm_warnings[]`, and per slide `{index, kind, role, visual_family, density, title, takeaway, pattern, layout}` — **without** compiling or rendering. Use during planning. |
| Render | `render_deck_spec` | `semantic render` | One-call spec → `.pptx`. Strict output validation by default (`output_validation off\|warn\|strict`). Returns `{success, pptx_path, quality_summary, diagnostics[], explanation_summary}`. |
| Lower to raw | `compile_deck_spec` (`include_compiled_json: true`) | `semantic compile --envelope` | Escape hatch: emit the compiled `PresentationInput` to hand-edit, then drive `validate_input` / `generate_presentation`. Default output is compact (`{ok, slide_count, template, diagnostics[]}`). |

**`meta.archetype`** ∈ `board_update`, `qbr`, `sales_pitch`, `strategy_proposal`,
`project_roadmap`, `market_analysis` — biases template choice, default rhythm, and whether a
synthesis/decision slide is expected (call `list_deck_archetypes` for each one's default template +
`executive` flag).

**`slides[].kind`** — `list_slide_kinds` (CLI `semantic schema`) is the **single source of truth**
for each kind's required + typical fields; the table below mirrors it exactly. Template resolution
order: spec `meta.template` > tool/CLI `template` arg > archetype default.

> ⚠️ **Unknown payload fields are silently ignored.** The DeckSpec payload accepts arbitrary keys
> (`additionalProperties: true`), so a misspelled or invented field name — `points` written as
> `bullets`, `kpis` as `metrics_list`, a column's `items` as `rows` — is dropped without error and
> its content never reaches a slide. Use **exactly** the field names in the table. `semantic schema`
> now declares each kind's payload fields as a discriminated union (`$defs.Slide_<kind>`, pinned by
> `kind` const, referenced from `SlideSpec.oneOf`), so a JSON-Schema validator can flag missing
> required or unknown fields — but the compiler itself still ignores unknown keys at render time, so
> validate the spec against the schema rather than relying on the engine to reject mistakes.

| kind | required | typical / optional | item-object fields (exact) |
|---|---|---|---|
| `title` | `title` | `subtitle`, `eyebrow` | — |
| `section` | `title` | `subtitle` | — |
| `executive_summary` | `title` | `points` (body bullets), `takeaway` (footer one-liner) | — |
| `kpi_snapshot` | `kpis` (2–6 → cards) | `title`, `takeaway` | each KPI: `{value, label, delta?}` (`delta` ≤12 chars, e.g. `"+5%"`, renders a small annotation; aliases `sub`/`trend`/`change`) |
| `chart_insight` | `chart` | `insights[]` (1–6 → chart+insights visual), `title`, `source`, `takeaway` | chart: `{type, data, title?}` |
| `comparison` | `columns` (exactly 2, balanced, ≤10 rows each → visual) | `title`, `takeaway` | each column: `{header, items[]}` *(or `{header, pros[], cons[]}`)* |
| `process` | `steps` (3–8 → visual) | `title`, `takeaway` | each step: string or `{label, type?}` |
| `roadmap` | `phases` (3–6 → visual) | `title`, `takeaway` | each phase: string or `{name, date_label?, description?, active?, milestone?, items?}` (items[] sub-bullets are folded into the phase description) |
| `decision` | `title` | `recommendation`, `options[]`, `takeaway` | each option: string or `{label}` |
| `closing` | `title` | `subtitle` | — |
| `raw_json2pptx` | `slide` | — | a raw `PresentationInput` slide, structurally validated then passed through (see note) |

> **`executive_summary` body vs footer:** body bullets come from `points` (preferred) **or**
> `takeaways` (plural array); `takeaway` (singular string) is the one-line footer insight — distinct
> from the `takeaways` body array.
>
> **Compiler-accepted aliases** (resilience only — prefer the canonical names above): `kpi_snapshot`
> `kpis`↔`metrics` with `{value↔big, label↔small/caption, delta/trend/change↔sub}` (a **blessed required-one-of alias**: a
> spec providing only `metrics` validates and compiles — it appears in `list_slide_kinds`
> `required_aliases` and the schema's required-one-of clause, not just at compile time); `chart_insight` `insights[]`↔`insight`
> (singular string); `comparison` column `header`↔`title`/`label`/`name`; `process` step
> `label`↔`title`/`name`/`step`/`text`/`description`; `roadmap` phase `name`↔`title`/`label`/`phase`,
> `date_label`↔`dates`/`date`/`period`, `description`↔`detail`/`summary` (plus `items`/`bullets` folded in); `decision` option
> `label`↔`title`/`name`. Counts outside a visual's range degrade to a readable content slide (with a
> `SEMANTIC_DENSITY` advisory) rather than failing. A field present with the **wrong JSON type** for its
> kind (a numeric title, `points`/`steps`/`columns` given as a bare string instead of an array) is
> flagged with a `SEMANTIC_FIELD_TYPE` advisory (`warning` under `warn`, `error` under `strict`) — the
> compiler would otherwise drop the wrong-typed value silently, so wrap single list values in `[ ]` and
> keep scalar fields as strings.
>
> **`raw_json2pptx` structural contract:** the `slide` payload is decoded as a raw `PresentationInput`
> slide and must be a valid one, or validate/compile **blocks** (`SEMANTIC_REQUIRED` / `SEMANTIC_UNKNOWN_FIELD`
> at `slides[i].slide`): it must be a JSON object, carry **no unknown fields** (typo'd keys are reported, not
> silently dropped), set a `slide_type` or `layout_id`, and carry renderable content (`content`, `shape_grid`,
> `pattern`, or `compose`). The `blank` slide_type is the one content-free exception (a deliberate empty canvas).

**Repair stays in the spec.** `render_deck_spec` maps every render-time fit finding back to the
semantic source you wrote: each `diagnostics[]` entry carries `semantic_path` (the DeckSpec field
to edit), `raw_path` (compiled-pointer fallback), `slide_index`, `action`, and often a
`recommended_edit`. **Edit the spec at `semantic_path` and re-render** — do not patch the compiled
JSON unless you have deliberately dropped to the raw escape hatch.

```yaml
meta: {title: "Q3 Review", archetype: board_update, template: midnight-blue}
slides:
  - {kind: title, title: "Q3 Review", subtitle: "Board update"}
  - {kind: executive_summary, title: "Bottom line", takeaways: [...], takeaway: "..."}
  - {kind: kpi_snapshot, title: "Where we stand", kpis: [...]}
  - {kind: chart_insight, title: "Revenue", chart: {...}, insight: "..."}
  - {kind: decision, title: "Recommendation", options: [...], recommendation: "..."}
```

**Worked spec examples + schema.** Complete runnable DeckSpecs live in
[`../../examples/semantic/`](../../examples/semantic/) (`qbr.yaml`, `sales_pitch.yaml`, plus
`invalid.yaml` showing the findings a malformed spec returns). Run `json2pptx semantic schema` for
the authoritative DeckSpec JSON Schema (the same enum `list_slide_kinds` / `list_deck_archetypes`
expose), and see [`../../docs/SEMANTIC_COMPILER.md`](../../docs/SEMANTIC_COMPILER.md) for the
compiler's normalize → validate → compile pipeline.

**Raw authoring (escape hatch).** When you need raw `PresentationInput` directly — a feature
outside the semantic schema, or a targeted raw repair — five fillable JSON skeletons live in
[`examples/skeletons/`](examples/skeletons/README.md): pick the one matching your archetype, copy
it, replace the `__FILL_*__` tokens. Skeletons pre-encode rhythm, accent strategy, and required
`takeaway` fields so you do not re-derive them per deck.

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
  value / cost driver tree             → driver-tree (NOT for people/roles — use svggen org_chart)
  one big number (± details)           → stat-hero / hero-detail
  2-6 KPIs (or supporting bar)         → kpi-2up … kpi-6up, kpi-inline
Compare 2 options / states
  side-by-side text                    → comparison-2col
  before / after transition            → before-after (or -compact)
  2×2 axes positioning                 → matrix-2x2  ·  svggen: matrix_2x2
Process / sequence   ⚠ sparse-sequence rule below — NOT one row of 3-6 boxes on a bare slide
  short ordered steps (3-6, no branch) → numbered-step-strip (chevron / stacked-box / toc; never diamonds)
  ordered steps + a description each    → value-chain (4-10) · or numbered-step-strip detail zone
  flowchart WITH branching / decisions → process-flow (or -compact)  ← reserve for real branches
  two parallel / aligned tracks (3-6)  → process-grid-2row
  planned phases anchored to dates      → phase-roadmap
  workstreams × phases                  → roadmap-phased
  cross-actor / cross-functional        → swimlane
  true calendar milestones (real dates) → timeline-horizontal
  maturity ladder (3-6 stages, current state) → journey-maturity-model
  gantt schedule data                  → svggen: gantt
Cause-effect / structure
  fishbone / Ishikawa                  → svggen: fishbone
  architecture / tech stack            → arch-stack
  narrowing hierarchy / panels         → pyramid / stylish-panels
People / org / distribution
  org hierarchy (top-down)             → svggen: org_chart
  team bios with photos                → team-bios
  joint venture / engagement team pairs → dual-org-ladder
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

### Refined consulting layouts (prefer over generic card-grid / process-flow)

`card-grid` is a fallback, not a destination. When the content carries a recognizable
consulting shape, reach for the refined family first — `recommend_visual` now biases these
above `card-grid` even when you describe the slide with generic "cards"/"grid" wording, so
trust the higher-scoring refined candidate rather than defaulting to a tile grid:

| Content shape | Refined layout (use this) | Not |
|---|---|---|
| Ranked / weighted scorecard (vendors, options, drivers) | `horizontal-bar-with-callouts` | card-grid, kpi-Nup |
| Value / cost decomposition (metric = sum of branches) | `driver-tree` | card-grid |
| Governance / strategy pillars (+ foundation) | `strategy-house` · `stylish-panels` | card-grid |
| Capability / digital maturity, staged progression | `journey-maturity-model` | card-grid, plain bullets |
| Described phase plan (phases + dates + descriptions) | `phase-roadmap` | card-grid, process-flow |
| Operational sequence, each step with a description | `value-chain` | process-flow, card-grid |
| Pillars / capabilities as titled bullet blocks | `stylish-panels` | card-grid |
| Table of contents / agenda | `agenda` · `numbered-step-strip` (style:toc) | card-grid, process-flow |

Only fall back to `card-grid` for genuinely flat catalog content (N titled tiles with no
ranking, decomposition, sequence, or hierarchy). See the sparse-sequence rule below for why a
lone strip of boxes also fails — refined families avoid that by carrying per-item mass.

**`card-grid` visual styles + surface overrides.** `overrides.style` selects the card
treatment: `filled` (default, solid accent cards with light text), `accent-stripe`,
`numbered-badge`, `icon-card`, `tinted` (alternating lt1/lt2), and `soft-card` (a single
pale surface with dark text and an explicit no-border line). Independently of style, four
generic surface overrides apply on top of any style:

- `card_fill` — repaint every card with a scheme color (`lt2`) or, in `design_mode: "free"`,
  a raw hex like `"#FFF5ED"`. Constrained mode rejects raw hex; use a scheme color instead.
- `line_color` + `line_width` (0–12 pt) — draw an explicit card border. `line_color` takes
  precedence over `border`.
- `border` — keyword shortcut: `none` (explicit no border), `subtle` (thin dk1 hairline),
  `accent` (1 pt accent-colored border).

Pair `style: "soft-card"` (or `numbered-badge`) with `card_fill` for a pale brand surface
that keeps dark, contrast-safe text — e.g. `{"style":"soft-card","card_fill":"#FFF5ED"}`.

### Sparse-sequence rule (hard)

**Never fill a whole slide with a single row of 3-6 boxes** — a lone `process-flow`
or `timeline-horizontal` strip stranded in a sea of whitespace reads as unfinished.
`process-flow` and `timeline-horizontal` earn a full slide only when they carry their
own weight:

- `process-flow` → **only when the sequence actually branches** (decision diamonds, merges).
  A straight 1→2→3→4 chain is not a flowchart.
- `timeline-horizontal` → **only for true calendar milestones** with real dates, not for
  generic ordered steps.

For a short ordered sequence, pick (in rough order of preference):

1. `numbered-step-strip` — ordered steps with an optional **per-step detail zone** for body text.
2. `agenda` / `agenda-with-images` — when it reads as a table-of-contents / section list.
3. `value-chain` — steps that each carry a one-line description.
4. `phase-roadmap` — steps anchored to dates/phases.
5. `process-grid-2row` — when there are **two aligned tracks** sharing the same columns.
6. A hand-built `shape_grid` **lane + detail zone** (skeleton below) when no named pattern fits.

**Lane + detail-zone skeleton.** A top "lane" row of numbered step boxes plus an aligned
detail row below — the two rows share the same column count so steps and details line up
vertically, and the detail row gives the slide real vertical mass instead of one floating strip:

```json
{
  "layout_id": "blank",
  "content": [{"placeholder_id": "title", "type": "text", "text_value": "How the model selects a tool"}],
  "shape_grid": {
    "gap": 14,
    "rows": [
      {
        "height": 16,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "1  Intake",  "bold": true, "color": "lt1", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "2  Route",   "bold": true, "color": "lt1", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "3  Execute", "bold": true, "color": "lt1", "align": "ctr", "vertical_align": "ctr"}}},
          {"shape": {"geometry": "rect", "fill": "accent1", "text": {"content": "4  Verify",  "bold": true, "color": "lt1", "align": "ctr", "vertical_align": "ctr"}}}
        ]
      },
      {
        "auto_height": true,
        "cells": [
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Capture the request and the context it needs.", "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 8, "inset_right": 8, "inset_bottom": 8}}},
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Pick the tool family that fits the intent.",    "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 8, "inset_right": 8, "inset_bottom": 8}}},
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Run the tool and collect the result.",         "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 8, "inset_right": 8, "inset_bottom": 8}}},
          {"shape": {"geometry": "rect", "fill": "lt2", "text": {"content": "Check the output before returning it.",        "align": "l", "vertical_align": "t", "inset_top": 10, "inset_left": 8, "inset_right": 8, "inset_bottom": 8}}}
        ]
      }
    ]
  }
}
```

> Constrained mode: omit absolute `size` (template manages it) and use scheme colors
> (`lt1`, `accent1`, …), never raw hex — the validator rejects both. This exact slide
> passes `json2pptx validate`.

Prefer `numbered-step-strip` (with its detail zone) over hand-rolling this; reach for the raw
`shape_grid` lane only when you need a layout no named pattern covers.

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

**Diagram data-shape gotchas** (always copy from `get_diagram_schema.example_values` rather than guessing — these three are easy to get wrong and fail silently):

- **`matrix_2x2`** — points go in `points` (not `items`); `x`/`y` are on a **0-100 scale** by default (origin bottom-left, quadrant split at 50). 0-1 values collapse into the bottom-left corner — for normalized coords set `x_max`/`y_max` to `1`. To skip coordinates, use `quadrants: [{position, title, items}]`. Full reference: [docs/diagrams/matrix_2x2.md](../../docs/diagrams/matrix_2x2.md).
- **`nine_box_talent`** — auto-routed people go in `employees: [{name, performance, potential}]` where `performance`/`potential` are the **strings** `"low"`/`"medium"`/`"high"` (numbers are ignored and dump everyone in the center cell). Or place people explicitly with `cells: [{position: {row, col}, items}]` (row 0 = top/high potential, col 0 = left/low performance). Full reference: [docs/diagrams/nine_box_talent.md](../../docs/diagrams/nine_box_talent.md).
- **`porters_five_forces`** — each force needs a canonical `type` (`rivalry`, `new_entrants`, `substitutes`, `suppliers`, `buyers`) and an `intensity` from `0.0` to `1.0` (not `position`/`level`). An object-keyed form (top-level `rivalry`/`supplier_power`/`buyer_power`… keys, synonyms accepted) also works; an unrecognized shape renders a blank diagram. Full reference: [docs/diagrams/porters_five_forces.md](../../docs/diagrams/porters_five_forces.md).

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
- In **constrained** mode, raw hex colors on the documented override surface (`diagram_value.style.colors`, shape fills, etc.) are **refused** (`design_mode_violation`, blocks generation). Raw hex colors embedded in a diagram's **data payload** (e.g. `pyramid` `levels[].color`) are instead **dropped silently** and rendered with the template scheme, with an advisory `CUSTOM_COLOR_DROPPED` finding (MCP: `warning` severity). To honor custom diagram colors, rerun with `design_mode: "free"`.

---

## MCP Tools (most-used)

The five tools below cover the precondition workflow (`recommend_visual` → `show_pattern` → `expand_pattern` → `validate_input` → `generate_presentation`) plus `repair_slide` for fix-up. For the full 40+ tool catalogue — including session/discovery (`get_started`, `get_capabilities`, `get_input_schema`, `list_templates`, `resolve_theme`, `examine_template`, …), rhythm/scoring (`analyze_deck_rhythm`, `score_candidates`, `score_deck`), preview/render (`preview_presentation_plan`, `preview_slide_wireframe`, `render_slide_image`, `render_deck_thumbnails`, `inspect_slide_images`), the gated write tools (`register_template_setting`, `delete_template_setting`), and the MANDATORY / SKIPPABLE markers per phase — see **[TOOLS.md](TOOLS.md)**. The six `svggen-mcp` tools (`render_diagram`, `list_diagram_types`, `validate_diagram`, `get_diagram_schema`, `get_capabilities`, `get_started`) are documented under [Connected MCP servers](#connected-mcp-servers).

| Tool | Phase | When to call |
|---|---|---|
| `recommend_visual` | PLAN | First call per slide intent — ranks candidates across layouts, patterns, charts, diagrams, and raw shape_grid. Start here when unsure which visual approach fits. Pass the optional `template` (a template name) to make it **template-aware**: each candidate then carries `template_support: {status: supported\|risky\|unsupported, reasons[], required_layout}` grounded in the template's canonical layouts, derivable layouts, font-aware placeholder capacities, and palette — and candidates needing absent layouts (or violating capacity) are demoted so they no longer rank first. |
| `show_pattern` | PLAN | Per chosen pattern — returns the value schema and `example_values`. When `supports_callout: true`, the response also carries a `callout_schema` fragment for the envelope-level `callout` DTO. |
| `expand_pattern` | PLAN | Per pattern slide — preview the resolved `shape_grid` plus `density_warnings`, `cell_budgets`, and `bounds_source`. Confirm density lands in the 60–110% optimal band before generating. |
| `validate_input` | RENDER | Cheapest precondition gate — full-deck schema + optional `fit_report` + optional `strict_unknown_keys` for fail-fast on typo'd fields + optional `placeholder_policy` (default `warn`) to surface leftover `__FILL__` skeleton tokens. Always run before `generate_presentation`. |
| `generate_presentation` | RENDER | Render the PPTX. Defaults to `output_validation: "strict"` (see [Output Validation Guarantee](#output-validation-guarantee)); `strict_fit` controls overflow promotion (see [FINDINGS.md](FINDINGS.md)); `placeholder_policy` (default `warn`) warns on unresolved `__FILL__` tokens — set it to `"strict"` for publishable/gated output so leftover skeleton tokens block generation. |
| `repair_slide` | REPAIR | Apply targeted fixes to a single slide using the `Fix.Kind` vocabulary fit-report emits. For multi-slide fixes, run `propose_repairs` first to translate findings into ranked directives. |
| `auto_repair` | REPAIR | Server-side `generate→inspect→repair` convergence loop against a tunable `gate` (`min_score`, `max_p0_findings`, `max_p1_findings`, `require_takeaway_on_charts`); replaces hand-coded score→propose→repair→regenerate loops. Default `max_passes` 3; the final deck is always rendered. Returns `final_presentation` (full repaired JSON, always present — feed back into `validate_input`/`generate_presentation`/`repair_slide`), `gate_passed`, the publishability/evidence fields (`publishable`, `manual_review_required`, `blocking_reasons[]`, `content_status`, `evidence_complete`, `output_validation`, `render_evidence?`), and `next_state`/`resume_token` for resuming. **Default deterministic** (static + render-fit only, no rendering/API key); pass `visual_qa:{enabled:true}` to add a vision pass, `allow_degraded_scoring:true` to converge when renders fail. Full contract below: [Output Validation Guarantee](#output-validation-guarantee), [Visual-QA mode](#visual-qa-mode-auto_repair--make_deck), [Resumable convergence](#resumable-convergence-resume_token--next_state); per-field detail in [TOOLS.md](TOOLS.md). |
| `make_deck` | COLD-START | One-call facade: hand it an `outline` (natural-language brief) → chains `plan_deck → expand patterns with exemplar content → auto_repair`, returning a **DRAFT** PPTX skeleton. Output is **never publishable** — with no caller content it fills slides with pattern exemplar PLACEHOLDER values, so it always reports `content_status: "exemplar_skeleton"`, `uses_exemplar_content: true`, `publishable: false` even when the gate passes; replace exemplar copy via `repair_slide`, then run visual QA / manual review before shipping. Same `gate`, `next_state`/`resume_token`, and publishability/evidence contract as `auto_repair`; adds `plan.slides[]` (per-slide pattern/role/title, for targeting `repair_slide` without re-planning) and `final_presentation` (full authored+repaired JSON). Optional `style_hints` (`slide_budget`, `audience`, `accent_strategy`, `must_include`), `max_repair_passes`, `allow_degraded_scoring`, `visual_qa`. Prefer this for cold starts; switch to the precondition workflow when authoring per-slide content yourself. Detail in [TOOLS.md](TOOLS.md). |

**Stop when the quality gate passes.** Every `score_deck` response carries a `quality_gate` block — the **machine-readable definition of done**:

```json
"quality_gate": {
  "passed":   true,
  "reasons":  [],
  "criteria": {
    "min_score":                  80,
    "max_p0_findings":            0,
    "max_p1_findings":            0,
    "require_takeaway_on_charts": true,
    "allow_accent_overload":      false
  }
}
```

When `quality_gate.passed === true`, the deck has cleared the ship-quality bar — **stop**. Do not chain another `propose_repairs` / `repair_slide` / `repair_slides_batch` / `auto_repair` call, do not render thumbnails to "double-check", and do not spend more tokens on aesthetic polish. When `passed === false`, `reasons[]` enumerates the unmet criteria in a stable order (`score → P0 → P1 → takeaway → accent_overload`) so you can address the highest-impact issue first. The criteria block is fixed by the server (not configurable on `score_deck`) so the gate cannot be relaxed at call time — agents that need a different threshold should call `auto_repair` (which exposes a tunable gate) instead. The same gate semantics apply inside the visual-QA loop: stop iterating once `quality_gate.passed` flips true on a `score_deck` pass.

**Compact responses.** The server advertises `experimental.compact_responses: true` in its `initialize` response; compaction itself is controlled by client opt-in (the client sends `experimental.compact_responses: true` in its capabilities) or the deprecated `MCP_COMPACT_RESPONSES=1` environment variable.

---

## Visual-QA mode (auto_repair / make_deck)

`auto_repair` and `make_deck` run a **deterministic** convergence loop by default: they score the deck from static + render-fit findings and **never look at a rendered pixel**. This is fast and free but cannot catch purely visual defects (overlap, contrast, misalignment) the way a vision pass can. The response truth-labels this as `quality_mode: "deterministic"`.

To add the agent-grade visual refinement loop, pass `visual_qa: {enabled: true}`. The phase runs AFTER the deterministic loop: render thumbnails → `inspect_slide_images` → map visual findings to `propose_repairs` → apply → re-render. Only **P0/P1** visual findings drive automatic repairs (P2/P3 are advisory). Any repairs it applies are reflected in `final_presentation`.

**Requested vs actual** — `quality_mode` reports the regime that **actually ran**, and the always-present `quality` object spells out the gap: `{requested, actual, inspection_mode?, fallback_reasons[]?}`. `requested` is request-derived (`"deterministic+visual_qa"` whenever you set `visual_qa.enabled=true`); `actual` is `"deterministic+visual_qa"` only when a visual pass actually inspected slides and degrades back to `"deterministic"` when it was skipped. `quality_mode` aliases `quality.actual`, so it never overstates rigor — a requested-but-skipped phase reports `quality_mode: "deterministic"`, and `quality.fallback_reasons[]` says why (render tools unavailable, render failure, or missing-API-key heuristic fallback). Read `quality.actual`/`quality.inspection_mode`, **not** `quality.requested`, before claiming a vision pass happened.

```jsonc
"visual_qa": {
  "enabled": true,         // default false — the whole mode is opt-in
  "model": "claude-...",   // optional vision-model override
  "audit_palette": true,   // also run the deterministic palette ΔE audit
  "max_passes": 1,         // visual render→inspect→repair iterations, clamped [1,3]
  "density": 50            // thumbnail DPI for inspection, clamped [25,150]
}
```

**Preconditions and cost** (echoed back in `visual_qa.requirements`): rendering needs `libreoffice` + `magick` on PATH; vision inspection issues one Claude vision call per slide (default `claude-haiku-4-5-20251001`) and requires `ANTHROPIC_API_KEY`.

**Transparent fallbacks** — a *well-formed* `visual_qa` request never errors out the call; it degrades and records the reason in `quality.fallback_reasons[]`:
- Render tools missing → `visual_qa.inspection_mode: "skipped"`, `quality.actual: "deterministic"` (the requested visual phase ran on nothing), with an explanatory `notes[]` entry; the deterministic deck is preserved.
- `ANTHROPIC_API_KEY` unset → `inspection_mode: "heuristic"` (pure-Go fallback, advisory P3 findings) instead of vision; `quality.actual` stays `"deterministic+visual_qa"` (visual QA still ran) but a `fallback_reasons[]` entry flags the lower-rigor backend.
- **Malformed `visual_qa` is NOT a transparent fallback** — a *present* `visual_qa` that is not an object, or has a wrong-typed field, fails fast with `INVALID_PARAMETER` rather than silently disabling the mode (so you never lose a requested vision pass to a typo). Omit `visual_qa` (or pass `null`) for the deterministic default.
- A wedged renderer or stalled API call can no longer hang the loop: LibreOffice/ImageMagick subprocesses and each vision request run under bounded deadlines. A breach is reported as `LIBREOFFICE_TIMEOUT` / `IMAGEMAGICK_TIMEOUT` (render step, recorded in `notes[]`) or `VISION_TIMEOUT` (per-slide inspection error), each carrying the tool, elapsed time, and a retry/degrade action. The standalone `render_slide_image`, `render_slide_image_from_json`, and `render_deck_thumbnails` tools return the same `*_TIMEOUT` codes; resolve any of them with `describe_finding`.

**Failed inspection ≠ clean deck** — when a vision call errors (API failure, malformed output, `VISION_TIMEOUT`) or a heuristic decode fails, that slide is *not* inspected, so zero findings does not mean it passed. `inspect_slide_images` projects each failure to an **error-severity** finding (`VISION_INSPECTION_FAILED` / `VISION_TIMEOUT` / `HEURISTIC_INSPECTION_FAILED`, so `findings.ok` is false) and adds top-level `failed_slide_count` + `inspection_status` (`complete` / `partial` / `failed`); never treat an empty findings list as clean unless `inspection_status` is `complete`. In the `visual_qa` loop the same failure sets `visual_qa.inspection_complete: false`, populates `failed_slide_count`, records a `notes[]` entry, and stops the pass from being counted as a clean convergence.

**Atomic artifact updates** — each visual-repair pass is staged: the engine applies the repairs, re-renders, and **rolls the in-memory edits back if that re-render fails**, so the returned `final_presentation` and the PPTX at `path` always advance together. `visual_qa.artifact_consistent` (always present) reports the guarantee: `true` means `final_presentation` matches the PPTX you'd inspect or ship. It is `false` only in the defensive case where a re-render failed *and* the rollback could not be performed — then `final_presentation` is ahead of the PPTX, a blocking `notes[]` entry explains it, and you must **not** ship that artifact (re-run `auto_repair` / `generate_presentation` from the returned JSON instead). A rolled-back pass leaves `artifact_consistent: true` and records the revert in `notes[]`.

`visual_qa.passes[]` records each iteration: `{pass, inspection_mode, failed_slide_count, inspection_status, thumbnail_paths[], visual_findings[], proposed_repairs[], repairs_applied[]}` — `repairs_applied[]` lists only repairs that survived in the final deck (a reverted pass reports an empty array). The `visual_qa` block also carries `inspection_complete` and a roll-up `failed_slide_count` (see *Failed inspection ≠ clean deck* above). When `audit_palette: true`, `visual_qa.palette_audit` carries `{available, violations, findings, note?}`. Discover the mode at runtime via `get_capabilities` → `features.quality_modes`.

---

## Output Validation Guarantee

**The zero "needs repair" contract.** `generate_presentation` (MCP) and `json2pptx generate` (CLI) default `output_validation` / `--output-validation` to `strict`. In strict mode the engine runs the full OPC + OOXML validator (`internal/pptx.ValidateOutputFile`) against the freshly-written `.pptx` and **refuses to return success on any blocking finding**. Every successful generate response therefore implies a structurally clean file — agents do not need a separate `validate_presentation_output` call to confirm.

A blocking finding means PowerPoint or Keynote would show the "we found a problem with some content, do you want us to repair" prompt when opening the file. The validator covers:

| Phase | Validator | Sample codes |
|-------|-----------|--------------|
| `opc` | `structural` | `OPC_MISSING_PART`, `OPC_DANGLING_REL`, `OPC_DUPLICATE_REL_ID`, `OPC_MISSING_ELEMENT`, `OPC_MALFORMED_XML`, `OPC_MISSING_CONTENT_TYPE`, `OPC_MISSING_CONTENT_TYPE_OVERRIDE` |
| `ooxml` | `ooxml_content` | `OOXML_INVALID_COLOR`, `OOXML_INVALID_SCHEME`, `OOXML_DUPLICATE_ID`, `OOXML_INVALID_TABLE`, `OOXML_ZERO_EXTENT`, `OOXML_ILLEGAL_XML_CHAR`, `OOXML_SLIDE_COUNT_MISMATCH`, `OOXML_EMPTY_REQUIRED_ATTR` |

`OPC_*` and the two structural-corruption `OOXML_*` codes (`OOXML_ILLEGAL_XML_CHAR`, `OOXML_SLIDE_COUNT_MISMATCH`) are always promoted to `severity: "blocking"`. Other `OOXML_*` codes are advisory `warning`s and do not fail strict mode unless the validator escalates them.

### Validation evidence on the repair facades

`auto_repair` and `make_deck` succeed at the transport layer even when something went wrong, so they expose **explicit evidence and status fields** instead of letting a clean-looking response (a successful tool call with a `path`) imply a publishable deck. Never treat artifact existence or `gate_passed` alone as "done":

- **`publishable`** (always present, boolean) — the single authoritative ship-as-is flag. `true` only when the gate passed on complete evidence, the artifact is structurally valid, **and** content is author-supplied. Equivalent to `blocking_reasons` being empty. `make_deck` output is **always `publishable: false`** because its slides are exemplar placeholders. Stop and review/replace before shipping whenever this is `false`.
- **`manual_review_required`** (always present, boolean) — the affirmative inverse of `publishable`: a human or agent must review before the deck ships (gate failed, evidence incomplete, structurally invalid, or exemplar content).
- **`blocking_reasons`** (present only when not publishable, `[string]`) — every reason the deck is not publishable, a **superset of `gate_reasons`** that also folds in incomplete-evidence and exemplar-content causes. Branch on this to decide what to fix.
- **`content_status`** (always present) ∈ {`author_supplied`, `exemplar_skeleton`} and **`uses_exemplar_content`** (boolean) — content provenance. `make_deck` always reports `exemplar_skeleton` / `true`; `auto_repair` always `author_supplied` / `false`. Exemplar content is never publishable, no matter how cleanly it scores.
- **`artifact_status`** (always present) ∈ {`generated`, `generated_invalid`} — whether the on-disk PPTX passed final structural output validation. **`validation_status`** (always present) ∈ {`passed`, `passed_degraded`, `failed`} folds the gate result with evidence completeness.
- **`evidence_complete`** (always present, boolean) — the authoritative clean-evidence flag. `true` only when the render pass that produced the score completed **and** the final structural output validation passed. A `gate_passed: true` with `evidence_complete: false` is a *degraded* pass, never a clean one.
- **`output_validation`** (always present, `{ran, valid, blocking[]}`) — the final `pptx.ValidateOutputFile` run on the on-disk deck, executed after any visual-QA re-render. Blocking structural findings reopen the gate (`gate_passed: false`) even if the convergence loop had satisfied it. A corrupt or unreadable file yields `valid: false` rather than a silent pass.
- **`render_evidence`** (present only on failure, `{complete:false, stage, detail, degraded}`) — emitted when a per-pass render failed (`stage` ∈ `convert`/`tempdir`/`generate`). Its presence means the score reflects static analysis only, and an explicit **`RENDER_EVIDENCE_INCOMPLETE`** finding (action `refuse`) is in the finding set blocking the gate.

By default an incomplete render blocks the gate. Pass **`allow_degraded_scoring: true`** to converge on static analysis alone — the `RENDER_EVIDENCE_INCOMPLETE` finding drops to advisory (`review`), `render_evidence.degraded` is set, and the gate may pass — but `evidence_complete` stays `false` and final output validation still blocks regardless. `score_deck` accepts the same `allow_degraded_scoring` flag and attaches `render_evidence` (only on incomplete renders) to its response.

### Resumable convergence (resume_token / next_state)

`auto_repair` and `make_deck` are synchronous: one call runs the whole pass loop and returns. To let you inspect a partial result and pick up where it stopped — instead of restarting from scratch — every response carries a **`next_state`** block, and both tools accept a **`resume_token`** argument.

**`next_state`** (always present) =
`{completion, resumable, resume_token, next_action, passes_run, next_pass?, max_passes, artifact_path, remaining_findings[]}`.

- **`completion`** classifies how the loop stopped: `converged` (gate met on complete evidence — *not* resumable, nothing to do), `converged_degraded` (gate met on degraded/static-only evidence), `max_passes_exhausted` (budget ran out with the gate unmet), `no_progress` (a pass applied no repairs — the loop stalled), or `render_incomplete` (the backing render did not complete). It is the one field that distinguishes a partial/degraded result from a clean convergence.
- **`resumable`** is `true` for every status except a clean `converged` run. **`next_action`** is a one-line suggested move; **`remaining_findings[]`** (capped) echoes the findings still open after the last pass.
- **`resume_token`** is the handle to continue this session (per-process, expires after 1 hour).

**To resume:** call the same tool again with `resume_token` set. The saved post-repair deck, accumulated `trace`, and content provenance are reloaded from the session, and the loop continues at `next_state.next_pass` with **continuous global pass numbering — completed passes are never re-run**. On a resume call: `presentation` (auto_repair) / `outline` (make_deck) are ignored; `gate` and `max_passes` (or `max_repair_passes` for make_deck) may be overridden to relax bounds or grant a fresh budget of additional passes; `base_dir`, `visual_qa`, `allow_degraded_scoring`, and `output_filename` are inherited (the last is still overridable). make_deck preserves the original `plan` across the resume without re-planning. An unknown/expired token returns `RESUME_TOKEN_NOT_FOUND`; a token issued by the other tool returns `RESUME_TOKEN_MISMATCH`.

Typical use: a first call exhausts `max_passes` with `completion: "max_passes_exhausted"` → inspect `remaining_findings`, then resume with a higher `max_passes` (or a relaxed `gate`) to converge from the current deck instead of regenerating it. A `render_incomplete` result is resumable once render tooling (libreoffice + magick) is available, or set `allow_degraded_scoring`.

### MCP error envelope (FindingEnvelope)

Every MCP tool error — argument validation (missing required field, wrong type, malformed JSON), template/asset resolution failures, strict-fit refusals, slide-level validation errors — returns an error `CallToolResult` (`isError: true`) whose structured content is the shared **FindingEnvelope**. The same shape is emitted by `validate_input`, `repair_slide`, the `validate-template` CLI command (under its `findings` key, alongside the structural `theme`/`layouts`/`capabilities` fields), and the other diagnostic-bearing surfaces:

```json
{
  "schema_version": "1.0",
  "tool": "json2pptx",
  "subcommand": "mcp",
  "ok": false,
  "summary": "1 error",
  "findings": [
    {
      "id": "input-1",
      "code": "INPUT.MISSING_PARAMETER",
      "category": "INPUT",
      "severity": "error",
      "message": "fixes is required (expected array)",
      "evidence": {"path": "fixes", "expected_type": "array"},
      "example_value": [{"kind": "reduce_text", "params": {"max_items": 5}}],
      "next_tool_call": {
        "tool": "repair_slide",
        "args_template": {"fixes": "<provide value>"}
      },
      "describe_command": "json2pptx describe-finding MISSING_PARAMETER"
    }
  ]
}
```

`validate-template` carries its findings under `findings` rather than as an error result: the structural report (`theme`, `layouts`, `capabilities`) always returns, and template-validation issues ride in the envelope with `TPL.*` codes — `TPL.TEMPLATE_METADATA_PARSE`, `TPL.TEMPLATE_METADATA_VERSION`, `TPL.TEMPLATE_ASPECT_RATIO_INVALID`, `TPL.TEMPLATE_LAYOUT_HINT_INVALID`, `TPL.TEMPLATE_SECTION_NUMBER_NAMING` (all warnings), and `TPL.TEMPLATE_ERROR` (error). Run `json2pptx describe-finding <code>` (legacy code, e.g. `TEMPLATE_SECTION_NUMBER_NAMING`) for remediation steps.

`examine-template` (CLI: `json2pptx examine-template <template.pptx> --out <dir>`) is the deepest read-only template diagnostic: it writes a directory (`report.json`, `report.md`, `theme.json`, `conformance.json`, `canonical_roles.json`, per-layout `layouts/slideLayoutN__<canonical>.{json,xml,svg,png}`, and `master/`) describing exactly what a user-provided template supports. `report.json` nests the shared envelope under its `findings` key (alongside `canonical_coverage`, `derivable_layouts`, and `layouts[]` with font-aware `max_chars`, exact bounds in inches, z-order, and content zones). It adds one finding code: **`TPL.LAYOUT.MISSING_ROLE`** (warning), emitted once per absent content-bearing canonical family (`title-slide`, `section-divider`, `one-content`, `qa-closing`); the finding's `evidence.family` names the missing family and `canonical_coverage.<family>.present` is `false`. Use it to vet a new template before authoring against it; the annotated SVG overlays show the same numbers as `report.json` for a browser sanity-check. (PNGs require LibreOffice + ImageMagick; the SVG overlays are always written.) The `--gate` flag turns the same examination into a CI gate: it writes a `gate.json` verdict (`{template, passed, violations[]}`) and exits non-zero when a template has an untagged layout, incomplete canonical coverage, a title placeholder not named exactly `title`, a section divider without a `Section Number` frame, or any error-severity finding — the contract every bundled template must satisfy (see `docs/TEMPLATE_SPEC.md` → CI Template Gate; enforced by `.github/workflows/templates.yml`).

Branch on `ok` (false when any error-severity finding is present), then walk `findings`. Each finding carries a namespaced `code` (`<CATEGORY>.<legacy_code>`, e.g. `INPUT.MISSING_PARAMETER`, `TPL.TEMPLATE_NOT_FOUND`, `RENDER.URL_FETCH_FAILED`), the offending JSON `path` and `expected_type` under `evidence`, and — for repairable issues — a structured `remediation.primary.{action, params}`. Arg-validation findings guarantee a non-empty `evidence.path` plus at least one of `evidence.expected_type` or `next_tool_call` so an agent can self-correct without re-reading the schema. The `next_tool_call` usually replays the same tool with the offending field as a placeholder; for shape failures it points at `get_input_schema`, and for unknown identifiers it points at the relevant discovery tool (e.g. `list_templates`, `list_patterns`). Discovery findings also carry list facts in `evidence` (e.g. icon-name `suggestions`) so you can repair without a separate lookup. Common legacy codes (after the namespace prefix) are `MISSING_PARAMETER`, `INVALID_PARAMETER`, `INVALID_JSON`, `INVALID_KEY`, and `INVALID_PATH`.

Primary-tool **runtime** failures (those that fire after arguments parse, when the named resource is absent or broken) follow the same contract. A missing `template` (`TEMPLATE_NOT_FOUND`) carries `evidence.template_name` plus the `available_templates` list and points `next_tool_call` at `list_templates`. A missing or malformed PPTX path on `read_presentation` / `validate_presentation_output` (`FILE_NOT_FOUND`, `INVALID_PATH`) carries `evidence.path` + `expected_type` + `evidence.file_path` and replays that same tool. An existing-but-unreadable file (`READ_FAILED`) and a validator that errors (`VALIDATION_FAILED`) cross-point `next_tool_call` at the sibling introspection tool (`validate_presentation_output` ↔ `read_presentation`) so you can investigate in one hop; an out-of-range `slide_index` (`INVALID_SLIDE_INDEX`) carries `evidence.slide_count`.

When you do not recognize a `code`, run the finding's `describe_command` — `json2pptx describe-finding <code>` — or call the `describe_finding` MCP tool. It resolves **every** code the pipeline emits (the fit/pattern codes from `list_patterns`/`get_capabilities`, the `chart.*` and string-literal codes, and the whole diagnostics taxonomy: `MISSING_PARAMETER`, `TEMPLATE_NOT_FOUND`, `RENDER_FAILED`, `INTERNAL`, …) to `{code, summary, severity, when_emitted, remediation_steps[], example_before, example_after, related_codes[]}`. It accepts the bare legacy code **or** the dotted namespaced form straight off the envelope (`INPUT.MISSING_PARAMETER`, `FIT.placeholder_overflow`) — the prefix is stripped before lookup, so the `describe_command` runs verbatim. The CLI also accepts the code positionally (`json2pptx describe-finding MISSING_PARAMETER`) as well as via `-code`. Unknown codes return a structured error whose `fix.params.allowed` enumerates the full known vocabulary.

### Output validation error envelope

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
  "repairable": false,
  "repair_unavailable_reason": "output-validation findings are structural OPC/OOXML problems with no auto-derivable repair_slide directive; inspect each finding's code, scope, and source_path, then construct the appropriate repair_slide fix ...",
  "next_tool_call": {
    "tool": "describe_finding",
    "args_template": {"code": "OUTPUT_VALIDATION_ERROR"}
  }
}
```

`repairable` is always `false` here: output-validation findings carry no fix directive, so the engine cannot pre-fill an executable `repair_slide` call (it has no way to know the replacement color, target layout, etc.). `next_tool_call` therefore points at `describe_finding` — a directly-executable call that resolves the finding's meaning and remediation steps — rather than at `repair_slide` with an empty `fixes` array, which `repair_slide` rejects. `args_template.code` is the first blocking finding's own `code` when that code is in the describe vocabulary, otherwise the umbrella `OUTPUT_VALIDATION_ERROR` code (as shown above, since the specific `OPC_*`/`OOXML_*` codes are not individually registered). You still construct the actual repair from the preserved `findings[]` context.

Every finding carries a `scope` field classifying responsibility:

| `scope` | Meaning | Agent response |
|---------|---------|----------------|
| `source` | The bug is in the input JSON (bad color, malformed table). | Repair via `repair_slide` (e.g. `replace_color`, `use_semantic_color`). |
| `template` | The bug is in the `.pptx` template (missing layout part, dangling rel). | Switch templates or report; cannot be fixed via `repair_slide`. |
| `generator` | The bug is in the engine. | Report — do not retry; an automated repair is unlikely to help. |

### Responding to a validation error

1. **Inspect every blocking finding's `code` and `scope`.** `scope: "source"` is repairable; `template` and `generator` usually are not.
2. **Run the `next_tool_call` first — it is `describe_finding`, not `repair_slide`.** The envelope never advertises a `repair_slide` call (`repairable: false`) because output-validation codes carry no auto-derivable fix params. Invoking `describe_finding` with the supplied `code` returns the finding's `remediation_steps[]`. You then build the `repair_slide` directive yourself from the preserved `findings[]` context — `slide_index` comes from `findings[].slide_index`.
3. **Look up unfamiliar codes** via `describe_finding` (or in `internal/pptx/output_validator.go` — `opcCodeMap`, `ooxmlCodeMap`) before guessing a remedy. Each finding's `message` field also explains why it fired.
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

   **Template-aware ranking.** Pass `template` (a template name, e.g. `"midnight-blue"`) to vet each candidate against that specific template. Every candidate then carries `template_support: {status, reasons[], required_layout}`:
   - `status: "supported"` — the template natively covers the layout/capability the candidate needs.
   - `status: "risky"` — producible only via a synthesised/derived layout (e.g. a two-column built by splitting a One Content layout), or close to a body-capacity / content-zone limit. Read `reasons[]` for the caveat.
   - `status: "unsupported"` — the candidate needs a canonical or derivable layout the template cannot provide (`reasons[]` names what is missing).

   The engine **demotes** risky/unsupported candidates in the ranking, so the top candidate is feasible for the template whenever any feasible option exists. The displayed `score` is left untouched (it stays the intent-match score); only ordering changes. `required_layout` names the canonical layout or derivable capability the candidate needs (`"Title Slide"`, `"Two Content"`, `"full-image"`, `"grid base"`, …). The same support logic also powers `plan_deck` template-awareness. Without `template`, candidates carry no `template_support` (template-agnostic ranking, unchanged behaviour).

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
| `blank-title` | `title` only | Slide content is a `shape_grid` or `pattern` rendered below a slide title — no body placeholder |
| `blank-canvas` | none | Fully custom canvas with **no** title — all content is a `shape_grid`/`pattern`; nothing reserved for a title |
| `blank` (legacy) | `title` only | Backward-compatible alias that resolves to `blank-title` (falls back to the empty Blank only if the template has no Blank + Title). Prefer `blank-title`/`blank-canvas` in new decks |

`content` is body-capable: the engine populates a body placeholder with your content item. The blank layouts are shape-grid-oriented: there is no body placeholder, so all content must come from `shape_grid` or `pattern`. `blank-title` keeps a title slot; `blank-canvas` reserves nothing. Setting `slide_type: "blank"` (or omitting `layout_id` when `shape_grid`/`pattern` is present) triggers auto-selection of a blank layout with title and computed grid bounds — equivalent to `blank-title`.

**Canonical `layout_id` is resolved against the active template.** An explicit canonical name (`title`, `content`, `blank`, `blank-title`, `blank-canvas`, `section`, `closing`, `two-column`, `image-left`, …) is resolved by tag-based matching to whichever concrete `slideLayoutN` the chosen template provides — you do **not** pass raw `slideLayoutN` IDs, and the same canonical name stays portable across templates (e.g. `blank` → the template's Blank + Title, falling back to its empty Blank). A concrete `slideLayoutN` ID, a generated layout ID (`content-2-50-50`, `grid-2x2`), or a known alias (`2-col`, …) is passed through unchanged. If a name resolves to nothing, it is left as-is and falls through to the normal layout lookup / auto-selection path.

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

1. **PLAN** — produce a short outline (template, accent strategy, slide-by-slide list of layouts + patterns + accents). Use `plan_deck` for decks >4 slides. **Pass the optional `template` (a template name) to make the plan template-aware:** every planned slide — and each `alternatives[]` entry — then carries `template_support: {status: supported|risky|unsupported, reasons[], required_layout}` from the same shared helper `recommend_visual` uses, so the two tools agree for identical template constraints. A recommended pattern the template cannot host is swapped for a supported alternative during planning, so the plan never assigns an impossible pattern; the result also echoes the vetted `template`. Without `template`, slides carry no `template_support` (template-agnostic plan, unchanged).
2. **VARY** — call `analyze_deck_rhythm` and act on `longest_run`, `accent_balance`, `density_cv`, `composition_score`.
3. **RENDER** — generate the JSON in one pass; verify the pre-emit checklist (Rule 20, semantic fills, gap ≥4pt, accent variety, 60–110% density).
4. **REPAIR** — `validate_input` → `generate_presentation` → `render_slide_image` / `render_deck_thumbnails` → `inspect_slide_images` → `repair_slide`. Images are truth.

For the `repair_slide` fix-kind vocabulary, finding-code catalog, and strict-fit promotion ladder, see [FINDINGS.md](FINDINGS.md).

---

## Pattern Library (overview)

For BMC, KPI grids, 2x2 matrices, timelines, card grids, icon rows, two-column comparisons, accent-banded panels (`stylish-panels`), strategy-house frameworks, executive SCQA summaries (`scqa-summary`), chart-with-takeaway layouts (`chart-insights-split`), ranked horizontal bars with per-bar callouts (`horizontal-bar-with-callouts` — 3–8 bars; each bar binds to one insight via a left accent bar), waterfall / bridge bar charts (`waterfall-bridge` — 3–10 columns of total + delta + subtotal bars showing how components reconcile a start total to an end total; floating delta bars and auto-computed subtotals), value / cost driver trees (`driver-tree` — root metric → 2–4 branches → 1–4 leaves each, with optional per-branch annotations; for **people/role** hierarchies use the svggen `org_chart` diagram type instead), Porter-style value chains (`value-chain` — 4–10 step columns with per-step description and optional highlight), double-track process grids (`process-grid-2row` — two parallel rows of 3–6 phase columns sharing the same N columns, with a dk1 row-label column on the left and per-row accent fill), ordered numbered steps without branching (`numbered-step-strip` — 3–6 steps in `chevron` / `stacked-box` / `toc` styles, each with an optional per-step detail zone; never emits decision diamonds — use `process-flow` when the sequence has decision points), maturity ladders (`journey-maturity-model` — 3–6 stage columns with numbered headers, descriptions, and an optional 'where we are' marker), visual deck previews (`agenda-with-images` — 3–6 numbered agenda rows each with title/subtitle and an image or quote placeholder), team / 'Our People' pages (`team-bios` — 1–8 members, each with a photo placeholder above name + role + short bio, up to 4 per row), and joint-venture / engagement-team paired-role slides (`dual-org-ladder` — 2–6 paired role rows across two parallel org columns, each column with its own org-name header, optional thin connector line between paired cards), use json2pptx's named patterns. Named patterns expand to validated `shape_grid` structures at generation time, replacing ~600 tokens of boilerplate with ~100 tokens.

**Tip — chart + narrative on the same slide.** `chart-insights-split` is the canonical "data on the left, interpretation on the right" consulting layout: pass a `chart` (any `types.DiagramSpec` shape) plus 1–6 `insights` bullets. If you ship the pattern without a `chart`, the engine renders insights full-width and emits `CHART_PLACEHOLDER_EMPTY` (`action: review`) so you know the panel collapsed — supply a chart or swap to an insights-only pattern.

**Tip — executive problem framing.** `scqa-summary` lays out the classic consulting Situation / Complication / Questions / Answer arc as a 4-row, 20%/80% split. Each row's body accepts either a string or a 1–4 item array of bullets, so the same pattern works for both terse one-liners and dense multi-bullet content.

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

**KPI `values` is always a JSON array of cells**, one per metric (`kpi-2up`…`kpi-6up`, `kpi-inline`). Each cell is either:

- an object `{"big": "$127M", "small": "Revenue"}` — `big` is the headline number (≤ 8 chars), `small` is the caption. The intuitive aliases `value`/`number` (→ `big`) and `label`/`caption` (→ `small`) are also accepted. An optional `sub` (≤ 12 chars) renders a small delta/trend annotation between the number and the caption (e.g. `{"big": "$50M", "small": "Revenue", "sub": "+5%"}`); the aliases `delta`/`trend`/`change` map to `sub`. Put the sign in the value itself (`"+5%"` / `"-0.4%"`) — the annotation stays in the card's light text color rather than green/red so it remains legible on the accent fill.
- a pipe-delimited string shorthand `"$127M | Revenue"` (exactly one ` | ` separator).

Both forms validate and expand identically; you may mix them within one `values` array. Passing a single bare cell instead of an array is tolerated (wrapped into a one-element list), so the result is a clear "exactly N cells" count error rather than an unmarshal failure.

**Do NOT hand-roll shape grids when a named pattern exists.**

See [PATTERNS.md](PATTERNS.md) for the full pattern workflow: catalog browsing, `text_budget_guide`, Text Capacity Awareness (density bands, decision rules), bounds override (`bounds`, `max_height_pct`), and density-class divergence warnings.

---

## Rules (overview)

Non-negotiable. Full catalog with rationale and examples in [RULES.md](RULES.md). Highlights:

- **Shape grid (Rules 1–7):** col_spans must sum per row; `bounds` are percentages; gaps are typographic points; one content type per cell (except `composite`/`pattern`/`grid` slots); body text cells need all 4 insets.
- **Charts (Rules 8–10):** `series[i].values` length equals `len(categories)`; chart types use underscores (`stacked_bar`, NOT `stacked-bar`); don't mix data formats. **Per-slide chart-style overrides:** add `chart_value.chart_style: {show_vertical_gridlines: true}` or `{show_single_series_legend: true}` to flip an executive token default for one chart — omit the block to keep the deck defaults. **Legend defaults:** bar / line / grouped-bar / area charts with 2–4 series render inline series labels and suppress the legend (executive default); above 4 series the legend reappears. Force the legend back on inside the 2–4 window with `chart_value.style.show_legend: true`. Stacked variants and non-Cartesian charts (pie, donut, scatter, radar, waterfall, funnel, gauge, treemap) keep the previous legend behaviour.
- **Content & layout (Rules 11–15):** `layout_id` must be canonical (`title`, `content`, `blank`, `section`, `closing`, `two-column`, …); semantic fills (`accent1`, `lt2`, `dk1`) required, never mix with hex on one slide; align values are `"l"`/`"ctr"`/`"r"`/`"just"`, vertical align is `"t"`/`"ctr"`/`"b"`.
- **Contrast auto-fix (Rule 16):** engine auto-replaces low-contrast text with dark gray; surfaces as `contrast_autofixed` findings.
- **Silent traps (Rules 17–19):** `footer` is an object not a string; never prefix `source` with "Source: "; content fields need `_value` suffix (`chart_value`, `table_value`).
- **Table density (Rule 20 — enforced):** MUST split if rows > 7 OR cols > 6 OR font < 9pt. Multiline cells count as N logical rows.
- **No emoji codepoints (hard rule):** emoji glyphs are rejected by pattern validators in `card-grid`, `icon-row`, `herodetail`, etc. Use a bundled SVG icon name or supply a user icon via `path` / `url` / `svg_data`. See the [Icon Names](#icon-names) section.
- **Anti-patterns:** two-tables-one-grid, hex-fill mix, pattern monotony (no 3-in-a-row), accent monotony, sparse single-row flow (no full-slide `process-flow`/`timeline-horizontal` for 3-6 short labels — see the Sparse-sequence rule).
- **Cell accent variety:** `cell_accent_mode` ∈ {`uniform`, `alternate`, `progressive`} — use `progressive` for 4+ peer cells.

For the full finding-code catalog (`fit_overflow`, `cell_underfilled`, `placeholder_overflow`, `chart.*` family, render-time codes like `contrast_autofixed`, `text_trimmed`, `diagram_clamped`, etc.) and the `fix.kind` enums, see [FINDINGS.md](FINDINGS.md).

---

## Color Roles

Each template exposes `color_roles` in `list_templates` (MCP) / `json2pptx skill-info` (CLI) output — use `primary_fill` / `secondary_fill` for header cells with white text, `body_fill` + `body_text` for card bodies, and check `white_text_safe` before using any accent with `#FFFFFF` text. For tints, use luminance modifiers: `{"color": "accent1", "lumMod": 20000, "lumOff": 80000}` (20% tint with `dk1` text).

**Template-authored accent guidance (`accent_usage_guide`).** Some templates include an `accent_usage_guide` map in their `list_templates` output. When present, it maps accent color names (e.g. `"accent1"`, `"accent3"`) to prose descriptions of each accent's intended role within that template's visual language. When `accent_usage_guide` is present, defer to the template's role descriptions over generic assumptions — do not assume any accent has a fixed semantic role (positive, negative, neutral, subtle, etc.) unless the guide says so. When absent, fall back to the existing `color_roles` `primary_fill`/`secondary_fill`/`body_fill` semantics above.

**Semantic palette metadata.** `list_templates` (MCP, `fields=full`) / `json2pptx skill-info` also surface the template's authored palette intent (omitted when the template ships no metadata block):

- `semantic_accents` — maps `positive` / `negative` / `neutral` to accent names; use these when a pattern field expects a `semantic_accent` so reds/greens follow the template's own conventions.
- `surface_tints` — maps `subtle` / `paper` / `elevated` / `inverse` surface roles to scheme color names for tinted card/panel backgrounds.
- `data_palette` — ordered scheme color names for chart series (matches what svggen uses), so multi-series charts stay on-brand.
- `metadata_version` and `sha256` — the metadata schema version and a stable content hash of the template file; use `sha256` as a cache key to detect when a template changed under a stable name.

**Canonical layout taxonomy.** Discovery output (compact and full) carries the same canonical taxonomy that `examine_template` reports, so you can plan against stable roles instead of raw layout names:

- `canonical_layout_ids` — canonical layout name → concrete layout ID (address layouts by intent).
- Each `layout_summaries[]` entry carries `canonical_type` (e.g. `"Title Slide"`, `"One Content"`, `"Section Divider"`) and per-placeholder `role` (`title`, `eyebrow`, `section_number`, `body`, `image`, `chart`, …) alongside `max_chars`.
- `canonical_coverage` — per content-bearing family (`title-slide`, `section-divider`, `one-content`, `qa-closing`): `{present, layouts[]}`. A family with `present: false` means decks needing that slide kind cannot resolve a native layout.
- `derivable_layouts` — `[{name, ready, missing[]}]` for higher-level layouts the engine can synthesise (e.g. `two-content`, `full-image`, grid patterns). When `ready: false`, `missing` names the absent prerequisite.
- Full mode (`fields=full`) adds, per layout, `canonical_type` / `canonical_family` / `canonical_confidence`, and per placeholder `role` / `role_confidence` / `font_size_pt` (the font-size evidence behind the font-aware `max_chars`).
- The full per-layout entries (`examine_template`'s `report.json` `layouts[]`, and discovery `fields=full` `layouts[]`) carry a `tags[]` array of structural/semantic hints. Two matter when placing section titles: `title-at-bottom` (the title slot sits in the lower half of the slide, under a decorative element) and `compact-title` (that bottom title slot only fits a short single-line title — visible-title `max_chars` < 35). When a Section Divider carries these, keep the section title to one short line; long titles overflow. (See the template guide for the full tag catalog.)

**Read-only discovery (no preview cache writes).** Template discovery generates layout-preview PNG **cache files** as a side effect (under `~/.cache/json2pptx/layout-previews`, only when LibreOffice + ImageMagick are present). Every `list_templates` / `skill-info` response carries a `side_effects` block — `{preview_cache_writes, read_only, preview_cache_dir, disable_with}` — so you can tell whether the call touched the filesystem. When you only need template metadata in a read-only planning context, pass `read_only: true` (MCP `list_templates`) or `--no-preview` (CLI `skill-info`): preview generation is skipped, no cache files are written, `preview_png_path` is omitted from `layout_summaries[]` / `layouts[]`, and `side_effects.preview_cache_writes` is `false`. All other metadata (theme, color roles, canonical taxonomy, semantic palette) is unaffected. `get_capabilities` reports `list_templates` with `writes_files: true` for this reason.

---

## Deck-Level Defaults

For multi-table decks, set shared styles once in the top-level `defaults` block instead of repeating them on every `table_value`:

```json
{
  "defaults": {
    "table_style": {"style_id": "@template-default", "header_background": "accent1"},
    "cell_style": {"align": "l", "vertical_align": "ctr"}
  },
  "slides": [ ... ]
}
```

**Semantics (V1).** Swap-only: any inline field on a table/cell fully replaces the corresponding defaults field for that field (no deep merge). Supported kinds: `table_style`, `cell_style`. See `../../docs/STYLE_DEFAULTS.md` for scope rules and the `@template-default` sentinel. Table styles available per template are listed in `list_templates`'s `table_styles[]` array — each entry's `id` is a `{8-4-4-4-12}` OOXML GUID.

**`style_id` validation.** A `style_id` must be empty, the `@template-default` sentinel, or a well-formed OOXML table style GUID (e.g. one of the `id` values from `list_templates`'s `table_styles[]`). Any other value — a friendly name, a typo, or a string containing XML metacharacters such as `"&<` — is rejected with an `INVALID_PARAMETER` validation error and is never emitted into the deck. A well-formed GUID the template does not declare is allowed but yields the advisory `unknown_table_style_id` warning.

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

- `chrome.page_numbers.format` supports `{current}` and `{total}` placeholders. Default skip set is `["title", "closing"]`. `"title"` and `"closing"` match the canonical layout types (so section dividers are *not* skipped by default — they stay in the numbered body flow); any other value matches a structural layout tag, so e.g. adding `"section-header"` suppresses page numbers on section dividers too.
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

**Shrink an overlay icon (`scale`).** When an icon is overlaid on a shape (`shape.icon`, or a cell `icon` with text), the optional `"scale"` field (`0 < scale <= 1`) reduces its footprint relative to the shape — useful for narrow cards where the default top/left overlay icon crowds the text. Example: `"shape": {"fill": "accent1", "text": "Secure", "icon": {"name": "shield", "scale": 0.4}}`. The overlay default is `0.6`; out-of-range or unset values fall back to it. `scale` has no effect on standalone (text-free) icon cells.

**Preview a single icon (`preview_icon`).** Before committing a custom-SVG path/URL or a recolored bundled icon to a deck, call `preview_icon` with the same `IconInput` shape you'd drop into a shape grid cell. It returns `svg_data` (with `fill` applied for non-inline sources), `png_base64` (rasterized preview), `alt`, `source_kind` (`bundled` / `path` / `url` / `inline`), and `qualified_name` for bundled icons — no template load, no full generation cycle. Pass `base_dir` whenever `icon.path` is relative. The `fill` override is ignored for inline `svg_data` (a warning is returned). The CLI counterpart is `json2pptx preview-icon` (accepts `--name`/`--path`/`--url`/`--svg-data` flags or an `--icon` JSON file/`-`).

**Structured write manifests for file-writing CLI commands.** The CLI commands that write files to disk — `preview-icon` (`--out-svg`/`--out-png`), `preview-wireframe` (`--out`), and `preview-patterns` — accept a `--manifest` flag that emits a JSON success manifest to stdout instead of forcing agents to scrape stderr logs. The manifest shape is `{"success": true, "command": "<name>", "artifacts": [{"path", "kind", "bytes", "sha256"}], "warnings": [...]}` where `kind` is a short artifact label (`svg`, `png`, `pattern-preview`). For `preview-icon`/`preview-wireframe` the flag requires at least one `--out*` target; the PNG entry is omitted when rasterization produced no bytes. `generate` already returns this information via `--json-output-report` (`output_path` + `content_hash` + `warnings`; `--json-output` is the deprecated alias), so it needs no `--manifest` flag.

**Canonical identifier.** Each entry in the discovery response (`list_icons` MCP, `json2pptx icons list --json` CLI) exposes a `qualified_name` field in `<set>:<name>` form (e.g. `"outline:chart-pie"`, `"filled:chart-pie"`). Use `qualified_name` directly as `icon.name` in deck JSON. This is required for filled icons — a bare `"chart-pie"` resolves to the outline set; you must write `"filled:chart-pie"`. Outline icons accept either the bare name or the `outline:` prefix. The legacy `names[]` array (bare names) is kept for backward compatibility but does not disambiguate sets.

**Bundled name preflight.** `validate_input` and `generate_presentation` preflight every `icon.name` against the bundled registry. Unknown names emit `ICON_BUNDLED_NAME_UNKNOWN` (severity: error) with `details.suggestions` — a ranked list of Levenshtein-closest matches (or qualified cross-set forms when the bare base name only resolves in the non-default set). Use `suggestions[0]` to repair the name without a separate `list_icons` round-trip.

**Local asset path preflight.** `validate_input`, `generate_presentation`, `preview_presentation_plan`, `render_slide_image_from_json`, `score_deck`, `auto_repair`, and `make_deck` resolve every relative local asset path before generation. The CLI uses each input file's own directory as the base, so `json2pptx validate <deck.json>` and `json2pptx generate -json <deck.json>` agree on what a relative asset path points to; `json2pptx validate` also accepts an explicit `--base-dir <abs-dir>` override (mirroring the MCP `base_dir`), and `validate` of stdin falls back to the process CWD. MCP tools take an explicit `base_dir` parameter (absolute path to an existing directory), falling back to the server's process CWD when omitted. The best-deck tools resolve assets with the same helper and contract as `generate_presentation`: `score_deck` resolves before its render+score pass, and `auto_repair`/`make_deck` resolve once before the convergence loop so every repair pass embeds the same assets a direct `generate_presentation` would. A missing relative asset short-circuits all of these with the per-surface diagnostics below, exactly as `generate_presentation` does. **Always send `base_dir` from MCP** so the same JSON works regardless of how the server was launched — otherwise relative paths in your deck are silently coupled to the server's working directory. A malformed `base_dir` (relative, missing, or not a directory) is rejected with `INVALID_PARAMETER` (`path: "base_dir"`) before any per-asset findings. Coverage spans `icon.path` (shape grid icons), `image_value.path` (content images), `cells[].image.path` (shape grid cell images), and `background.image` (slide background). Each broken reference becomes its own structured finding so agents see every failure in one pass:

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
