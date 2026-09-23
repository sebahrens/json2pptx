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

Raw decks support clickable text and navigation: add `link: {url: "https://..."}` to a text/bullets content item, `source_link: {url: "https://..."}` beside a slide's `source`, or `link: {slide: N}` to a `shape_grid` shape or overlay badge. `N` is the destination slide in the final deck (1-based). See `docs/INPUT_FORMAT.md` for examples and validation rules.

**Completion rule (single source — same text as `get_started.completion_protocol.rule` and the MCP
server `instructions`):** A deck is done only after every slide of the CURRENT revision has been rendered (render_deck_thumbnails) and looked at by you. A passing deterministic gate, score, or validate result is a precondition for that review, never completion. After a repair, re-render and re-inspect the slides that changed (render_deck_thumbnails with slide_indices, or render_slide_image for a single one), then make one full-deck pass over the final revision: the revision you ship is the one that has to have been seen.

**MCP-only clients:** the server sends this workflow as its `initialize` `instructions` (and
`get_started` echoes it verbatim as `quality_workflow`): call `get_started` first → author a DeckSpec
(`list_slide_kinds` → `validate_deck_spec` → `render_deck_spec`) → render every slide with
`render_deck_thumbnails` and inspect the images → fix at `semantic_path` → never ship exemplar
content. `get_started{task:"brief"}` returns this DeckSpec path as its `fast_path` (`tool:
"render_deck_spec"`, `steps[]`); `make_deck` is a skeleton/wireframe tool only.

MCP hosts that expose prompts can use `deck-from-brief` with a required `brief`
and optional `template` / `slide_budget` (1–100), or `revise-deck` with a required
`goal` and exactly one of `pptx_path` / `deck_json`. Both prompts include the
appropriate `get_started` fast path and the same final-revision visual review rule.

MCP clients can opt into scoped render logs with `logging/setLevel` (`level:
"info"` or `"warning"`). `render_deck_spec` then sends start, finish and failure
events as `notifications/message` for that client session. Start and finish
include `tool` and `slide_count`; a successful finish adds `pptx_path` and
`duration_ms`. Logs also go to
server stderr; without an opt-in, INFO/WARN notifications are suppressed.

MCP `completion/complete` uses the resources already in `resources/list` as
references for enum values. Pass `ref: {type: "ref/resource", uri: "…"}` and
`argument: {name: "…", value: "<prefix>"}`:

| Resource URI | Argument name | Value for |
|---|---|---|
| `json2pptx://templates` | `template` | `render_deck_spec.template` and other template inputs |
| `json2pptx://patterns` | `pattern` | pattern inputs |
| `json2pptx://schema/deckspec` | `kind`, `archetype`, `chart_type` | DeckSpec and chart enums |
| `json2pptx://skill` | `fix_kind`, `finding_code` | repair and fit finding enums |

The `deck-from-brief` prompt also completes its `template` argument with
`ref: {type: "ref/prompt", name: "deck-from-brief"}`. MCP does not define a
tool reference for completion, so use the corresponding resource reference
for a tool argument. Unknown reference/argument pairs return no values.

This skill is split into focused sub-files. SKILL.md (this file) covers preconditions, the 5-tool quick reference, and the workflow overview. Load the sub-files when you need their detail:

| File | Contents |
|---|---|
| [TOOLS.md](TOOLS.md) | Full `json2pptx-mcp` tool catalogue with MANDATORY / SKIPPABLE markers per phase, plus contract-drift, pagination, schema-introspection, and gated-write semantics |
| [WORKFLOW.md](WORKFLOW.md) | 4-phase workflow deep dive (Plan, Vary, Render, Repair), visual inspection, `next_tool_call`, response_fingerprint, idempotency_key |
| [FINDINGS.md](FINDINGS.md) | All finding codes (layout + chart), the `fix.kind` enum, the `repair_slide` apply-only superset, strict-fit promotion ladder |
| [RULES.md](RULES.md) | Rules 1–20 (shape grid, charts, content/layout, contrast, silent traps, table density), anti-patterns, cell accent variety |
| [PATTERNS.md](PATTERNS.md) | Pattern library, `text_budget_guide`, Text Capacity Awareness, density bands, bounds override |

Read `../template-deck/TEMPLATE_GUIDE.md` for the complete field reference (content types, chart types, diagram types, shape grid properties, patch operations).

After authoring a chart or svggen diagram, run `validate_input` with the fit
report enabled. A `RENDER.diagram.text_overlap` finding names the two drawn
labels that collide; shorten one or give the diagram more space.
Timeline activity descriptions are placed after labels staggered below bars.
The same validation step checks editable native OOXML diagrams for dense copy;
reduce items or shorten labels when it reports native text overlap or text below
the readable minimum.

See `examples/four-phase-workflow.md` for a worked end-to-end example of the 4-phase flow.
For a coherent deck that must exercise template layouts, read
[`examples/layout-coverage-deckspec.md`](examples/layout-coverage-deckspec.md).

## Semantic deck specs — the default authoring path

For ordinary business decks, author a compact **DeckSpec** (`meta` + `slides[].kind`) and let
the compiler choose patterns, layouts, accents, and rhythm — a spec is far shorter than the raw
`PresentationInput` it lowers to. Write it as YAML or JSON; both the MCP `spec` arg and the CLI
`--spec` accept either.

DeckSpec also supports a chapter form, mutually exclusive with flat `slides[]`:
`structure: {cover, auto_agenda, sections:[{title, slides:[]}], closing}`. It expands to the cover,
optional agenda, a numbered divider before each non-empty section, the section's content slides,
and the closing slide. Set `meta.chrome.section_crumb: true` to show the current section on content
slides. Divider numbers come from the engine; never write numeric divider labels yourself.

When a request explicitly requires native layout coverage, list canonical IDs in
`meta.required_layouts`. `semantic explain` reports `layout_coverage.{requested,assigned,missing}`
and assigns compatible narrative slides after planning. Missing coverage is blocking: add an
appropriate semantic slide. Do not replace the story with one generated slide per required layout.

**Tools** (MCP `json2pptx-mcp` · CLI `json2pptx semantic <sub>`):

| Step | MCP tool | CLI | Purpose |
|---|---|---|---|
| Discover | `list_deck_archetypes`, `list_slide_kinds` | `semantic schema` | Enumerate `meta.archetype` / `slides[].kind` and each kind's required + typical fields (plus `required_aliases`: required-one-of alias keys, e.g. `kpi_snapshot` accepts `metrics` for `kpis`), the closed per-kind `item_schema`, a copy-ready `example` slide that validates clean, and `compositions[]` — every `pattern` / `layout` the kind's override accepts, with the reason each exists. An override outside that list is ignored and reported as `SEMANTIC_PATTERN_NOT_AVAILABLE`, so read it before varying a monotonous run (go-slide-creator-u5az). `semantic schema` prints the full DeckSpec JSON Schema (draft 2020-12); the same schema (inlined) is the MCP input schema of `spec` on **`validate_deck_spec`** — the one tool that carries it, because it is the one the workflow says to call before rendering. `render_deck_spec` / `compile_deck_spec` / `explain_deck_spec` declare the OUTLINE instead (`meta` + `slides[].kind` from the registered enum) and point here: the full contract was 17KB embedded four times, twice in the core listing alone (go-slide-creator-uhaq). Nothing about what those tools ACCEPT changed — an unknown payload field is reported as `SEMANTIC_UNKNOWN_FIELD` by the compiler whichever tool receives it. |
| Validate | `validate_deck_spec` | `semantic validate` | First check: unknown kinds/archetypes, missing required payload fields, rhythm/density advisories, **plus the fit/content findings of the compiled deck** (it compiles the spec and runs the same collectors as `validate_input`, minus the geometry-airiness advisories a one-slide spec cannot control). Returns the shared finding envelope; `ok=false` ⇒ ≥1 error-severity finding (`--strict off\|warn\|strict` controls advisory severity). |
| Preview plan | `explain_deck_spec` | `semantic explain` | Read-only projection: resolved archetype/template, deck `rhythm` + `rhythm_warnings[]`, and per slide `{index, kind, role, visual_family, density, title, takeaway, pattern, layout}` — **without** compiling or rendering. Use during planning. |
| Render | `render_deck_spec` | `semantic render` | One-call spec → `.pptx`. Strict output validation by default (`output_validation off\|warn\|strict`). Pass `output_filename` to name the artifact; omit it and the name is a slug of `meta.title` plus a short digest of the spec, so two specs never collide and re-rendering an unedited spec is idempotent. Returns `{success, pptx_path, overwrote, publishable, blocking_reasons[], quality_summary, diagnostics[], explanation_summary}`. **`success` says the file was WRITTEN; `publishable` says it is fit to SHIP** — gate your "done" on `publishable`. `diagnostics[]` now carries the SAME fit/content findings `validate_input` reports for the compiled deck (wrapped titles, over-long bodies, placeholder copy…), each with the `semantic_path` to edit, and `quality_summary` carries `structural_score` + `quality_gate` — the same deterministic gate `score_deck` applies, with the headline `score` capped at the structural verdict. |
| Lower to raw | `compile_deck_spec` (`include_compiled_json: true`) | `semantic compile --envelope` | Escape hatch: emit the compiled `PresentationInput` to hand-edit, then drive `validate_input` / `generate_presentation`. Default output is compact (`{ok, slide_count, template, diagnostics[]}`). |

**Deck chrome and per-slide extras.** `meta.chrome` carries the deck furniture every board deck
needs — `{confidentiality, client_name, project_code, footer_date, section_crumb,
page_numbers:{enabled, format, skip[]}}` — and `footer_date` defaults to `meta.date`. `meta.viewing_mode`
(`present` | `read`) and `meta.accent_strategy` (`primary` | `rotate` | `section-keyed`) pass through
to the compiled deck, the spec's choice winning over the tool argument. **Every** slide kind also
accepts `notes` (speaker notes, rendered into the PPTX notes slide) and `source` (the footnote line
under the content) — before this only `chart_insight` could cite anything, so an option matrix or a
financial case had nowhere to put its source (go-slide-creator-zmjs). The source renders **exactly
once** per slide: when the chosen pattern draws its own attribution (as `chart-insights-split` does,
under the chart) the chrome source band stands down rather than printing it a second time. Write the
citation however you like — `"Company filings FY2026"` and `"Source: Company filings FY2026"` both
render as `Source: Company filings FY2026`, never `Source: Source: …` (go-slide-creator-xg48).

```yaml
meta:
  title: "Board update"
  date: "September 2026"
  chrome: {confidentiality: "Strictly confidential", client_name: "Acme Corp",
           page_numbers: {format: "{current} / {total}", skip: [title]}}
slides:
  - {kind: executive_summary, title: "Where we stand",
     points: [{lead: "Growth is ahead of plan.", support: "Revenue grew 41% to $48M."}, ...],
     bottom_line: "Fund an SMB retention pod in Q3.", takeaway: "...",
     notes: "Pause for questions on churn.", source: "Finance close pack, 30 Sep 2026"}
```

**`meta.archetype`** ∈ `board_update`, `qbr`, `sales_pitch`, `strategy_proposal`,
`project_roadmap`, `market_analysis` — biases template choice, default rhythm, and whether a
synthesis/decision slide is expected (call `list_deck_archetypes` for each one's default template +
`executive` flag).

Semantic quality diagnostics include `SEMANTIC_EVIDENCE_VISUAL_MISSING` for evidence-oriented
market analysis without a data-bearing visual, `SEMANTIC_VISUAL_FAMILY_NARROW` for longer decks
using fewer than three non-structural visual families, and the required-layout diagnostics
`SEMANTIC_REQUIRED_LAYOUT_UNKNOWN`, `SEMANTIC_REQUIRED_LAYOUT_DUPLICATE`, and
`SEMANTIC_REQUIRED_LAYOUT_MISSING`.

**`slides[].kind`** — `list_slide_kinds` (CLI `semantic schema`) is the **single source of truth**
for each kind's required + typical fields; the table below mirrors it exactly. Template resolution
order: spec `meta.template` > tool/CLI `template` arg > archetype default.

> ⚠️ **Unknown payload fields are reported, never silently used.** Each kind's payload is a closed
> schema (`additionalProperties: false`): `semantic schema` and the MCP `spec` input schema declare a
> discriminated union (one `Slide_<kind>` variant per kind, pinned by `kind` const, in
> `SlideSpec.oneOf`) listing exactly the fields the compiler reads — including list-entry keys
> (`kpis[]`, `columns[]`, `steps[]`, `phases[]`, `options[]`) and the chart object (`type`, `title`,
> `data`). A misspelled or invented key (`takeawy`, a KPI's `valeu`, a column's `rows`, a flat
> `chart.series`) is still **dropped by the compiler**, so `validate_deck_spec` / `render_deck_spec`
> report it as **`SEMANTIC_UNKNOWN_FIELD`** at the exact path (e.g. `slides[2].kpis[1].valeu`) —
> `warning` under `off`/`warn`, `error` under `strict` — with `fix: {kind: "rename_field", params:
> {from, to, did_you_mean}}` when a known key is close. Treat every such warning as lost content and
> fix it. Use **exactly** the field names in the table (or copy `list_slide_kinds` `example`).

| kind | required | typical / optional | item-object fields (exact) |
|---|---|---|---|
| `title` | `title` | `subtitle`, `eyebrow` | — |
| `section` | `title` | `subtitle` | — |
| `executive_summary` | `title` | `points` (3–5 → `exec-summary` visual; any other count → bullet list), `bottom_line` (the ask, as a labelled callout), `takeaway` (footer one-liner) | each point: string (the conclusion alone) or `{lead, support?}` (`lead` ≤90 chars, `support` ≤200) |
| `kpi_snapshot` | `kpis` (2–6 → cards) | `title`, `takeaway` | each KPI: `{value, label, delta?}` — `value` ≤8 **characters**, `label` ≤40, `delta` ≤12 (e.g. `"+5%"`, renders a small annotation; aliases `sub`/`trend`/`change`). Budgets count characters, so `"€186.4M"` (7) fits; a value past the budget degrades the whole slide to a bullet list and says so as `SEMANTIC_PATTERN_DEGRADED` naming the field and the budget |
| `chart_insight` | `chart` | `insights[]` (1–6 → chart+insights visual; >6 → native `two-column` chart + full insight list), `title`, `source`, `takeaway` (dropped when it is word-for-word the slide's ONLY insight — the same sentence in the bullet and the band reads as a mistake, not emphasis; a takeaway summarising several insights is kept) | chart: `{type, data, title?}`; `data` = `{categories:[…], series:[{name, values:[…]}]}` (bar/line/area) or `{categories:[…], values:[…]}` (pie/donut) — a missing/malformed `data` yields `SEMANTIC_PATTERN_DEGRADED` at `slides[i].chart.data` with `fix.params.{expected_shape, example, from, to, reason}`. **Every series needs exactly one value per category and every value must be an unquoted number**: a short array is `CHART_SERIES_LENGTH_MISMATCH` (error) at `slides[i].chart.data.series[j].values`, a quoted figure or a null is `CHART_VALUE_NOT_NUMERIC` (error) at the offending element. Both used to validate clean and render a chart with empty slots or an empty plot |
| `comparison` | `columns` (2 balanced ≤10 rows each → `comparison-2col`; 3–5 → `stylish-panels`; 2–5 otherwise → `card-grid`; 6+ → bullets) | `title`, `takeaway` | each column: `{header, items[]}` *(or `{header, pros[], cons[]}`)*; every column needs a header to get a visual |
| `table` | `headers` (≤6, alias `columns`), `rows` (≤6 so the table stays within the 7-row budget) | `title`, `column_alignments`, `highlight_column`, `totals_row`, `column_types`, `takeaway` | each row: a list of cell values, or an object keyed by header label; short rows render blank cells. `highlight_column` takes a header NAME or a 0-based index; `column_alignments` are `left`/`center`/`right`. For options scored against criteria use `option_matrix`, not a table of symbols |
| `option_matrix` | `criteria` (2–6, alias `columns`), `options` (2–6, alias `rows`) | `title`, `scale` (`harvey` default / `rag` / `text`), `recommended`, `decisive_criterion`, `highlight_label`, `corner_label`, `takeaway` | each criterion: string or `{label, scale?}` (per-column scale, ≤30 chars); each option: `{name, detail?, scores[]}` — exactly one score per criterion (harvey `0–4` or `none`/`quarter`/`half`/`three-quarter`/`full`; rag `red`/`amber`/`green`; text ≤24 chars; `"-"` for n/a) |
| `team` | `members` (1–8 → `team-bios` cards; aliases `people`, `team`) | `title`, `takeaway` | each member: string (a bare name) or `{name, role, bio?, photo?, photo_label?}`. Every card needs a `role`; name ≤60 chars, role ≤80, bio ≤220 (~2 lines). `photo: {path\|url, alt}` is a real headshot, cover-cropped to the frame (alt defaults to the member's name and role); `photo_label` is the initials badge used **only when there is no photo** (≤8 chars, dropped if longer). Members with and without photos mix on one slide. Past 8 people or any budget it degrades to a bullet list |
| `image_case` | `body` (the story, ≤300 chars; aliases `text`, `story`, `description` — or give `bullets` instead) | `title`, `image`, `eyebrow`, `heading`, `bullets`, `metrics`, `caption`, `image_side`, `image_label`, `takeaway` | a photo or screenshot beside the words about it → `image-text-split`. `image` is a path or url string, or `{path\|url, alt}` (aliases `photo`, `screenshot`); omit it for a dashed placeholder. `eyebrow` ≤30 chars, `heading` ≤80, up to 5 `bullets` ≤140 each, up to 3 `metrics` `{value ≤10, label ≤40}` (both required — a figure with no words is half a claim), `caption` ≤120. `image_side` is `left` (default) or `right`. A picture with nothing said about it is a plain image slide, not a case study, and is refused |
| `framework` | `framework` (`swot` / `porters_five_forces` / `bmc`; aliases `type`, `model`), `sections` | `title`, `takeaway` | `sections` keys the framework's own parts, each a list of short items (≤10 per part, ≤200 chars each). swot: `strengths`, `weaknesses`, `opportunities`, `threats`. porters_five_forces: `rivalry`, `new_entrants`, `substitutes`, `suppliers`, `buyers` — a force may be `{items, intensity}` where intensity is 0.0–1.0 or `"high"`/`"medium"`/`"low"`. bmc: the 9 canvas cells (`key_partners` … `revenue_streams`). **Every part is required** — the visual draws all of them or none — and a framework missing one degrades to bullets grouped under each part's heading. Cells are filled from the deck's accent, not a per-cell rainbow: swot keeps two accents for its positive/negative halves, bmc one (Value Proposition a step deeper), porters_five_forces colours by `intensity`. `style.colors` overrides, in cell order |
| `matrix_2x2` | `quadrants` (exactly 4, clockwise from top left; aliases `cells`, `boxes`, or the named positions `top_left`/`top_right`/`bottom_left`/`bottom_right`), `x_axis`, `y_axis` | `title`, `x_low`, `x_high`, `y_low`, `y_high`, `takeaway` | each quadrant: `"Header"`, `"Header: body"`, or `{header, body?}` — header ≤80 chars, body ≤200. Axis labels ≤60 (aliases `x_axis_label`, `y_axis_label`), axis ends ≤20. Short of four headed quadrants and two named axes, or past any budget, it degrades to a bullet list naming each quadrant's position and both axes with their ends |
| `timeline` | `milestones` (3–7 → `timeline-horizontal`; aliases `stops`, `events`, `timeline`) | `title`, `takeaway` | each milestone: string (a bare label) or `{label, date?, end_date?, body?}`. Label ≤60 chars, date ≤30, body ≤200. A milestone with an `end_date` spans a period, which draws the whole line as range bars instead of dots. Outside 3–7, or past any budget, it degrades to a dated bullet list. For parallel workstreams across phases use `roadmap`, not `timeline` |
| `stat` | `value` (the number, ≤20 chars; aliases `stat`, `number`, `metric`) | `title`, `label`, `unit`, `context`, `source`, `takeaway` | one number made the whole slide → `stat-hero`. `label` is the words beneath it (≤80 chars, aliases `caption`/`subtitle`; defaults to the slide `title`), `unit` a short suffix beside the number (≤10, alias `suffix`), `context` one line under the label (≤120, aliases `detail`/`description`), `source` the footnote (≤80, drawn in the attribution band). Past any budget it degrades to a content slide keeping every word. For several numbers at equal weight use `kpi_snapshot` |
| `agenda` | `sections` (2–10 → numbered `agenda`; 3–6 WITH subtitles → `agenda-with-images` rows; aliases `items`, `agenda`) | `title`, `current`, `takeaway` | each section: string or `{title, subtitle?}`. `current` marks the section the deck is at — its 1-based position or its title — highlighting that row (aliases `current_section`, `highlight`, `active`). Outside those counts it degrades to a numbered bullet list with the current section marked |
| `quote` | `quotes` (one → `pull-quote`; 3–8 → `quote-cluster`; aliases `testimonials`, `voices`) **or** single `quote` string with `attribution` | `title`, `takeaway`, `role` for a single quote | each quote: `{text, name, role?}` (also accepts `quote`/`attribution`/`title` inside an item). The visual needs a speaker name; two quotes, missing names, or text outside pattern budgets degrade to quote bullets with `SEMANTIC_PATTERN_DEGRADED`. Single quote ≤500 chars; cluster text ≤240 each |
| `bridge` | `columns` (3–10 → `waterfall-bridge`) | `title`, `unit`, `caption`, `takeaway` | each column: `{label, type, value?}` with `type` = `total`, `delta`, or `subtotal`. Give numeric `value` on totals and deltas; omit it on subtotals to use the running total (an explicit subtotal must match it). Label ≤40 chars, unit ≤8, caption ≤60. Outside visual budgets it degrades to bullets with every component and `SEMANTIC_PATTERN_DEGRADED`. Use for a P&L walk or additive cost bridge. |
| `pillars` | `pillars` (3–5) | `title`, `objective`, `foundation`, `roof_badges`, `takeaway` | each pillar: `{title, body?}` with `body` a list of bullets. With both `objective` and `foundation`, it renders a `strategy-house` (pillar title ≤60, up to 5 bullets ≤120 each; optional ≤3 roof badges ≤24 each). Without house framing it renders `stylish-panels` (title ≤80, 1–8 bullets ≤200 each). Partial framing or content outside those budgets degrades to bullets preserving every field with `SEMANTIC_PATTERN_DEGRADED`. |
| `org` | `nodes` | `title`, `takeaway` | a flat reporting tree of `{id, name, title?, parent?}` nodes. Exactly one node omits `parent`; every other parent names an existing ID. Up to 7 nodes, 3 reporting levels, and 4 direct reports per node render as an `org_chart` diagram. Larger trees or labels over 40 characters degrade to a complete attributed bullet list with `SEMANTIC_PATTERN_DEGRADED`. |
| `architecture` | `tiers` (3–6 → `arch-stack` visual, alias `layers`) | `title`, `rails` (≤3, alias `side_rails`), `takeaway` | each tier: string or `{label, description?}` — or `{label, items[]}`, whose items are joined into the tier's detail line. Label ≤60 chars, detail ≤120, rail ≤30; a payload outside the tier count or those budgets degrades to a bullet list with every word intact, never truncated |
| `process` | `steps` | `title`, `takeaway` | each step: string or `{label, description?, type?}`. **Steps that carry a `description` render as numbered rows** (`numbered-step-strip`, 3–6 steps; label ≤60 chars, description ≤180) — a bold label over its own detail line. **Bare labels and branching steps** (`type: "decision"`) render as the `process-flow` diagram (3–8 steps, ≤80 chars per box; a description there is appended to the label, because a flow box has nowhere else to put one). Outside both it degrades to bullets |
| `roadmap` | `phases` (3–6 → visual) | `title`, `takeaway` | each phase: string or `{name, date_label?, description?, active?, milestone?, items?}` (items[] sub-bullets are folded into the phase description) |
| `decision` | `title` | `recommendation`, `options[]` (aliases `choices`, `alternatives`), `takeaway` | each option: `"Label"`, `"Label | detail"`, or `{label, detail?}`. **3–6 options render as numbered boxes** (label ≤60 chars, detail ≤180); **exactly 2, each WITH a detail**, render as two cards side by side (label ≤80, detail ≤300). The `recommendation` is the callout band beneath them — the ask, where the room can see it. One option, seven options, or a two-option pair missing a detail falls back to the recommendation as a lead-in over option bullets |
| `closing` | `title` | `subtitle`, `bullets[]`/`points[]` (renders a content slide) | — |
| `raw_json2pptx` | `slide` | — | a raw `PresentationInput` slide, structurally validated then passed through (see note) |

> **`executive_summary` body vs footer:** body points come from `points` (preferred) **or**
> `takeaways` (plural array); `takeaway` (singular string) is the one-line footer insight — distinct
> from the `takeaways` body array.
>
> **`executive_summary` renders the `exec-summary` pattern** at 3–5 points: numbered bold
> conclusions, each with its supporting sentence, separated by rules, over an optional
> `bottom_line` callout. Write each point as `{lead, support}` — `lead` is the conclusion
> (answer-first), `support` the one sentence of evidence. A bare string is the conclusion with no
> support line. Outside 3–5 points, or past the char budgets (`lead` ≤90, `support` ≤200), the
> slide degrades to a bullet list and `validate_deck_spec` says so with `SEMANTIC_PATTERN_DEGRADED` at
> `slides[i].points`; `explain_deck_spec` / `render_deck_spec`'s `explanation_summary` reports
> `pattern: "exec-summary"` only when the visual is what you will actually get.
>
> **Compiler-accepted aliases** (resilience only — prefer the canonical names above): `kpi_snapshot`
> `kpis`↔`metrics` with `{value↔big, label↔small/caption, delta/trend/change↔sub}` (a **blessed required-one-of alias**: a
> spec providing only `metrics` validates and compiles — it appears in `list_slide_kinds`
> `required_aliases` and the schema's required-one-of clause, not just at compile time); `chart_insight` `insights[]`↔`insight`
> (singular string); `comparison` column `header`↔`title`/`label`/`name`; `process` step
> `label`↔`title`/`name`/`step`/`text`/`description`; `roadmap` phase `name`↔`title`/`label`/`phase`,
> `date_label`↔`dates`/`date`/`period`, `description`↔`detail`/`summary` (plus `items`/`bullets` folded in); `decision` option
> `label`↔`title`/`name`. Counts outside a visual's range degrade to a readable content slide (with a
> `SEMANTIC_PATTERN_DEGRADED` advisory) rather than failing. A field present with the **wrong JSON type** for its
> kind (a numeric title, `points`/`steps`/`columns` given as a bare string instead of an array) is
> flagged with a `SEMANTIC_FIELD_TYPE` advisory (`warning` under `warn`, `error` under `strict`) — the
> compiler would otherwise drop the wrong-typed value silently, so wrap single list values in `[ ]` and
> keep scalar fields as strings.
>
> **`SEMANTIC_PATTERN_DEGRADED` vs `SEMANTIC_DENSITY`.** The first means you are losing the visual:
> the content does not fit the pattern its kind promised, so the compiler renders bullets or a plain
> content slide instead. `fix.kind` is `restore_visual` and `fix.params` answer the three questions
> without reading the message — `from` (the pattern refused, `""` when the kind has not committed to
> one), `to` (`content-bullets` / `content-slide` / `native-two-column` / `insights-only`) and
> `reason` (`count_out_of_range`, `budget_exceeded`, `columns_unbalanced`, `scores_incomplete`,
> `score_unreadable`, `chart_data_missing`). Branch on `reason`: `count_out_of_range` wants the list
> brought into range, `budget_exceeded` wants the named text shortened. `SEMANTIC_DENSITY` now means
> only count advice that does **not** cost you the visual — an over-wide or over-tall table, a row
> with fewer cells than its header, an unbalanced comparison a card-grid still draws.
>
> **`raw_json2pptx` structural contract:** the `slide` payload is decoded as a raw `PresentationInput`
> slide and must be a valid one, or validate/compile **blocks** (`SEMANTIC_REQUIRED` / `SEMANTIC_UNKNOWN_FIELD`
> at `slides[i].slide`): it must be a JSON object, carry **no unknown fields** (typo'd keys are reported, not
> silently dropped), set a `slide_type` or `layout_id`, and carry renderable content (`content`, `shape_grid`,
> `pattern`, or `compose`). The `blank` slide_type is the one content-free exception (a deliberate empty canvas).
>
> **It is bound by constrained design mode**, like every other deck: a raw slide that hand-sets a font size or
> a hex fill is refused with `design_mode_violation`, the same verdict `generate_presentation` gives the same
> payload. Set `meta.design_mode: "free"` when those values are deliberate — that is the only reason to set it,
> since the compiler's own output never hand-sets what the template owns (go-slide-creator-rs4h).

**A written deck is not automatically a good deck.** `render_deck_spec` returns `publishable: false` with `blocking_reasons[]` when the render carries an error-severity or `action: refuse` diagnostic, or fails the deterministic quality gate — while still returning `success: true` and the `pptx_path`, so you can open it and see the problem. Treat `publishable: false` as "fix and re-render", never as done; `success` alone only tells you a file exists. The CLI mirrors this: `json2pptx semantic render` exits non-zero on an unpublishable deck under the default `--output-validation strict`.

**Failures set `isError`.** A `render_deck_spec` / `compile_deck_spec` call that produces nothing — template not found, an unparseable spec, a slide the generator refuses — now sets the protocol's `isError` flag as well as `ok: false`, so a harness branching on `isError` cannot mistake it for a finished deck. Tools that *assess* rather than produce (`validate_deck_spec`, `validate_input`, `validate_pattern`) still report an invalid deck as a successful call with `ok: false`: their verdict is the product.

**Iterating produces separate files.** Each `render_deck_spec` call writes its own artifact, so you can render v1, keep it, render v2 and compare without shell access. `overwrote: true` in the response means a file already existed at `pptx_path` and this render replaced it — the only way to hit that is to pass the same `output_filename` twice. Writes to one path are serialized, so the `content_hash` you get back is always true of the bytes that render wrote; that is what `submit_visual_review`'s `pptx_revision` and the thumbnail cache are keyed on.

**Revising costs a patch, not a re-upload.** Every `validate_deck_spec` / `render_deck_spec` /
`explain_deck_spec` response carries `deck_id` — a handle on the spec the server is now holding.
On the next call send `deck_id` **instead of** `spec` (setting both is `AMBIGUOUS_INPUT`), with an
optional `patch`: `[{op, path, value}]` where `op` is `replace` | `add` | `remove` and `path` is a
JSON Pointer into the DeckSpec. `/slides/3/title` retitles slide 4; `/meta/template` restyles the
deck; `add` at `/slides/6` inserts a slide and at `/slides/-` appends one; `remove` at `/slides/2`
drops one. Ops apply in order and atomically — a malformed op is refused naming its index
(`patch[1].path`) and the stored deck is untouched. The patched deck becomes the handle's new
content, so the next call sees it, and the response's `changed_slides: [int]` names the 0-based
slides that differ, which is exactly the list to pass to `render_deck_thumbnails` as `slide_indices`.
Handles are per server process, expire after 1 hour (refreshed on each use), and an expired or
unknown one is an error naming `spec` as the way back — they save bytes, they are not where your
deck lives, so keep your own copy. A `render_deck_spec` driven by a handle and given no `template`
re-uses the one the last render resolved to, so patching a deck cannot silently restyle it.

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

### Patterns DeckSpec cannot reach

A DeckSpec kind compiles to 27 of the 43 registered patterns. The rest are authored with
`kind: raw_json2pptx` (or a hand-written `PresentationInput`), carrying a `pattern` block
verbatim — see the raw skeletons above. `list_slide_kinds` publishes the per-kind
`compositions` list; everything in that list is an override the kind will honour, and anything
outside it is reported as `SEMANTIC_PATTERN_NOT_AVAILABLE` and ignored.

| Pattern | What it draws |
|---|---|
| `before-after` | Two-column before/after with a transition chevron |
| `before-after-compact` | The same, height-capped for brief content |
| `driver-tree` | Value / cost driver tree: root metric → branches → leaves |
| `dual-org-ladder` | Two parallel org columns of paired role cards |
| `hero-detail` | One hero statistic with 2–4 supporting detail cards |
| `horizontal-bar-with-callouts` | Ranked horizontal bars with a per-bar insight callout (callouts optional; omit them all for a plain full-width ranked bar chart) |
| `icon-row` | A horizontal row of icon + caption pairs |
| `journey-maturity-model` | A 3–6 stage maturity ladder with a 'where we are' marker |
| `kpi-inline` | A horizontal inline KPI bar, height-capped for supporting context |
| `process-flow-compact` | A compact process flow, height-capped for short labels |
| `process-grid-2row` | Two parallel process tracks sharing the same phase columns |
| `pyramid` | A stacked trapezoid hierarchy of 3–5 tiers |
| `roadmap-phased` | A phased roadmap with parallel workstreams |
| `scqa-summary` | A 4-row Situation / Complication / Questions / Answer summary |
| `swimlane` | A horizontal swimlane diagram with actors and steps |
| `value-chain` | A horizontal value chain of 4–10 step columns |

`internal/semantic/reach.go` is the source of this table and a test diffs the two, so a pattern
cannot become reachable (or stop being) without this list changing with it.

---

## Quick Pattern Selector

Skim before reading the pattern catalogue. For ambiguous cases, fall through to `recommend_visual`.

```
PICK YOUR LAYOUT
─────────────────────────────────────────────────────────────────
Framing / exec summary
  SCQA narrative                       → scqa-summary
  3-5 bold key messages + support      → exec-summary
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
  options × criteria (Harvey / RAG)    → table-highlight (2-6 × 2-6; highlight_row = recommended)
  before / after transition            → before-after (or -compact)
  2×2 axes positioning                 → matrix-2x2 (axes are low→high arrows; optional x_low/x_high/y_low/y_high end labels, default Low/High)  ·  svggen: matrix_2x2
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
  one photo + case-study text          → image-text-split (image.path/url, 0-3 metrics)
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

1. `numbered-step-strip` — ordered steps with an optional **per-step detail zone** for body text. In `chevron` style a step label has to fit on ONE line inside its arrow: the strip gives up notch depth first and then shrinks the label to the 12pt floor to keep it there, so a 6-step strip renders a blunter arrow rather than "Qualific / ation". A label so long that even the floor wraps is reported as `BODY_TOO_LONG` — shorten it or use fewer steps.
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

Builds, validates, and repairs whole PPTX presentations. Owns templates, layouts, patterns, fit-report, and the `repair_slide` apply-only fix vocabulary. The 5-tool quick reference for this server is in the [MCP Tools (most-used)](#mcp-tools-most-used) section below; the full tool catalogue with phase markers lives in [TOOLS.md](TOOLS.md).

- **Binary path:** `cmd/json2pptx/json2pptx` (built via `make` or `go build ./cmd/json2pptx`)
- **Run as MCP:** `json2pptx mcp [--templates-dir <path>] [--output <path>] [--tools core|all]`
- **Tool annotations:** every tool advertises MCP annotations derived from its classification — `readOnlyHint` (false only for the tools that write artifacts or server state), `destructiveHint` (true only for `register_template_setting` / `delete_template_setting`), `idempotentHint`, `openWorldHint` (true only for the vision-API tool `inspect_slide_images`), and a human `title`. Hosts that gate destructive tools no longer prompt on discovery calls.
- **Tool profile:** by default (`--tools core`) `tools/list` advertises only the 24 core tools (`analyze_deck_rhythm`, `describe_finding`, `examine_template`, `expand_pattern`, `generate_presentation`, `get_capabilities`, `get_input_schema`, `get_started`, `inspect_slide_images`, `list_patterns`, `list_slide_kinds`, `list_templates`, `plan_deck`, `preview_presentation_plan`, `recommend_visual`, `render_deck_spec`, `render_deck_thumbnails`, `render_slide_image`, `repair_slide`, `score_deck`, `show_pattern`, `submit_visual_review`, `validate_deck_spec`, `validate_input`) without `outputSchema` (responses still carry `structuredContent`), keeping the listing under ~92KB (the closed DeckSpec schema is embedded once, on `validate_deck_spec`, and without field prose — call `list_slide_kinds` for per-kind item_schema + examples). Start the server with `--tools all` (or env `JSON2PPTX_MCP_TOOLS=all`) to list the full catalogue — facades (`make_deck`, `auto_repair`), `read_presentation`, preview/wireframe, template settings, `score_candidates`, `apply_deck_patch`, etc. Non-core tools remain callable by name in core mode; `get_capabilities().mcp_tools_available[].in_core_profile` tells you which tools the core profile lists.
- **Guidance is environment-aware too:** `get_started` opens with a `runtime` block — `{render_available, missing_commands[], templates_dir, output_dir, settings_write_enabled}` — so the first call already tells you what this server can do and where it writes. When `render_available` is false (no LibreOffice / ImageMagick), the server's `initialize` instructions carry a `RENDER TOOLING MISSING` line, `get_started` drops the render/inspect steps from both `fast_path` and `sequence`, the last remaining step says to hand back `pptx_path` and declare the deck **UNREVIEWED**, and `completion_protocol.complete_status` becomes `draft_needs_visual_review` — the honest ceiling on such a server. Do not claim the completion rule was met there; say the deck was not looked at.
- **Guidance is profile-aware:** everything the server recommends names a tool the active profile actually lists, because a client that never saw a tool in `tools/list` cannot call it. `get_started{task:"revise"}` returns `fast_path.tool: "render_deck_spec"` in both profiles — the DeckSpec branch (`deck_id` + `patch` → re-render → thumbnails of `changed_slides`), because a deck authored the recommended way is revised by editing its spec, not by repairing the compiled JSON. The raw-deck chain (`validate_input` → `preview_presentation_plan` → `repair_slide` → `generate_presentation`) stays as the second branch in `sequence`; in core mode `read_presentation` is dropped from it and the one-call `auto_repair` facade is named only as something `--tools all` adds. `next_tool_call` follows the same rule: a suggestion that would name a hidden tool is rewritten to the in-profile equivalent (`recommend_pattern` → `recommend_visual` with `content_hints.item_count`, `read_presentation` / `validate_presentation_output` → `render_deck_thumbnails` on the same file) or omitted when the profile has no equivalent — it is never a tool you cannot call.
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
- **`nine_box_talent`** — auto-routed people go in `employees: [{name, performance, potential}]` where `performance`/`potential` accept `"low"`/`"medium"`/`"high"` or numbers `1`/`2`/`3` (1=low, 2=medium, 3=high). Or place people explicitly with `cells: [{position: {row, col}, items}]` (row 0 = top/high potential, col 0 = left/low performance). Full reference: [docs/diagrams/nine_box_talent.md](../../docs/diagrams/nine_box_talent.md).
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

**What `theme_override` actually changes.** The engine rewrites the artifact's theme part (`ppt/theme/themeN.xml`), so an overridden scheme colour or font reaches *everything* that resolves through it: layout placeholders, pattern fills, native shapes, and every `<a:schemeClr>` reference. Chart series colours follow too, because the data palette resolves its scheme names against the post-override theme. Overriding a font the template does not embed is allowed but emits a deck warning — the font may substitute at render time — and an override naming a scheme slot the template's theme does not declare emits a warning and leaves that slot alone.

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
- `design_mode` is **deck-level** (top of the JSON, not inside a slide) and it is a **presentation FIELD, never a tool argument** — `generate_presentation(design_mode: "free")` is rejected with `UNKNOWN_PARAMETER`; put it inside the `presentation` object (`design_mode_violation`'s `next_tool_call` now shows exactly that shape, and `get_capabilities().features.design_mode` advertises it). The CLI flag `--design-mode=constrained|free` overrides this field for ad-hoc runs (`json2pptx generate --design-mode=free --json deck.json`).
- `contrast_check` is **slide-level** (inside each slide object, not on a content item)
- In **constrained** mode, raw hex colors on the documented override surface (`diagram_value.style.colors`, shape fills, etc.) are **refused** (`design_mode_violation`, blocks generation). Raw hex colors embedded in a diagram's **data payload** (e.g. `pyramid` `levels[].color`) are instead **dropped silently** and rendered with the template scheme, with an advisory `CUSTOM_COLOR_DROPPED` finding (MCP: `warning` severity). To honor custom diagram colors, rerun with `design_mode: "free"`.

---

## MCP Tools (most-used)

The five tools below cover the precondition workflow (`recommend_visual` → `show_pattern` → `expand_pattern` → `validate_input` → `generate_presentation`) plus `repair_slide` for fix-up. For the full tool catalogue — including session/discovery (`get_started`, `get_capabilities`, `get_input_schema`, `list_templates`, `resolve_theme`, `examine_template`, …), rhythm/scoring (`analyze_deck_rhythm`, `score_candidates`, `score_deck`), preview/render (`preview_presentation_plan`, `preview_slide_wireframe`, `render_slide_image`, `render_deck_thumbnails`, `inspect_slide_images`), the gated write tools (`register_template_setting`, `delete_template_setting`), and the MANDATORY / SKIPPABLE markers per phase — see **[TOOLS.md](TOOLS.md)**. The six `svggen-mcp` tools (`render_diagram`, `list_diagram_types`, `validate_diagram`, `get_diagram_schema`, `get_capabilities`, `get_started`) are documented under [Connected MCP servers](#connected-mcp-servers).

| Tool | Phase | When to call |
|---|---|---|
| `recommend_visual` | PLAN | First call per slide intent — ranks candidates across layouts, patterns, charts, diagrams, and raw shape_grid. Start here when unsure which visual approach fits. Pass the optional `template` (a template name) to make it **template-aware**: each candidate then carries `template_support: {status: supported\|risky\|unsupported, reasons[], required_layout}` grounded in the template's canonical layouts, derivable layouts, font-aware placeholder capacities, and palette — and candidates needing absent layouts (or violating capacity) are demoted so they no longer rank first. With `template`, each candidate's `example` also carries `layout_id` (the layout it renders on — the matching layout for placeholder candidates, else One Content) and `layout_preview_png_path` (the shipped 320px thumbnail of that layout); placeholder candidates use it as `preview_png_path` (`renderer: "template-preview"`, `metadata_only: false`). Open the thumbnail to see the template before committing to a layout. |

Render image tools honor MCP cancellation during conversion and thumbnail assembly. A cancelled call returns `CANCELLED` without image blocks; retry the render if images are still needed. Pass `_meta.progressToken` to `render_deck_thumbnails` to receive `notifications/progress` from preparation through each selected slide (the notification echoes the token and uses the selected slide count as `total`).

Use `export_deck` on an existing PPTX with `format: "pdf"` for a retained review copy or `format: "notes"` for a Markdown speaker-notes handout. It returns the absolute `output_path`; notes export works without LibreOffice.
| `show_pattern` | PLAN | Per chosen pattern — returns the value schema and `example_values`. When `supports_callout: true`, the response also carries a `callout_schema` fragment for the envelope-level `callout` DTO. **`values` is an object for most patterns and an ARRAY of cells for nine of them** (`icon-row`, `kpi-2up`…`kpi-6up`, `kpi-inline`, `stylish-panels`, `timeline-horizontal`): read `schema.properties.values` for the one you are about to call. `expand_pattern` / `validate_pattern` declare `values` as object-or-array for exactly this reason and name the array patterns in the argument description. |
| `expand_pattern` | PLAN | Per pattern slide — resolve the `shape_grid` plus `density_warnings`, `cell_budgets`, and `bounds_source`. **The grid it returns can be edited and submitted**: it carries `source: "pattern:<name>"`, which exempts the engine's own explicit font sizes from constrained mode's absolute-size rule. Keep the stamp on the grid you submit; drop it and those sizes are refused as if you had written them. Raw hex colours are still refused either way. Density is a **height** ratio (wrapped block height / available height, each paragraph measured at its own size), so >110% means the renderer will autofit-shrink the cell, not that text is clipped; aim for the 35–110% band, and treat `fit_overflow` (clipped even at the smallest shrink) and `TEXT_BELOW_READABLE_MIN` (shrunk below the readable floor) as the actual defects. |
| `validate_input` | RENDER | Cheapest precondition gate — full-deck schema + **pattern value-schema validation** (every slide-level `pattern`, `compose` segment, and nested cell pattern is expanded with the same validator `generate_presentation` runs, so a `maxLength` / `minItems` / unknown-bundled-icon violation is reported here instead of surfacing a round-trip later — **one finding per failing field**, each at its own path such as `/slides/{i}/pattern/values/members/0/role`, with `did_you_mean` for a misnamed field and a copy-ready `example` for a wrong shape; see [Pattern input codes](FINDINGS.md#pattern-input-codes--emitted-by-validate_input--generate_presentation--expand_pattern-before-a-pattern-expands)) + optional `fit_report` + optional `strict_unknown_keys` for fail-fast on typo'd fields + optional `placeholder_policy` (default `warn`) to surface leftover `__FILL__` skeleton tokens. Always run before `generate_presentation`: what it accepts, generate accepts. |
| `generate_presentation` | RENDER | Render the PPTX. Defaults to `output_validation: "strict"` (see [Output Validation Guarantee](#output-validation-guarantee)); `strict_fit` controls overflow promotion (see [FINDINGS.md](FINDINGS.md)); `placeholder_policy` (default `warn`) warns on unresolved `__FILL__` tokens — set it to `"strict"` for publishable/gated output so leftover skeleton tokens block generation. |
| `repair_slide` | REPAIR | Apply targeted fixes to a single slide using the **executable** `Fix.Kind` vocabulary (`get_capabilities().vocabularies.repair_fix_kinds`; includes `set_max_height_pct` to cap a stretched pattern's height). For multi-slide fixes, run `propose_repairs` first to translate findings into ranked directives. Truncating kinds (`reduce_text`, `shorten_title`, `reduce_cell_text`, `reduce_items`, `resize_list`) refuse with `code: "semantic_review_required"` when they would delete a number, unit, negation, or qualifier; list truncations then propose `split_pattern{path, first}` via `next_tool_call`. A finding's fix may instead name an **advisory** kind (`advisory_fix_kinds`: `add_detail_or_resize`, `grow_pattern`, `review`, `truncation_summary`, …) whose remedy is an authoring decision — `repair_slide` answers `{applied: false, code: "advisory_fix_kind", message: <the decision>, alternatives: [executable kinds]}` and `propose_repairs` files it under `advisory[]`; act on the guidance, don't retry the kind. On a clean deck most remaining findings are advisory — that is the loop finishing, not failing. Full tables: [FINDINGS.md](FINDINGS.md#fix-kinds-for-repair_slide--complete-table). |
| `auto_repair` | REPAIR | Server-side `generate→inspect→repair` convergence loop against a tunable `gate` (`min_score`, `max_p0_findings`, `max_p1_findings`, `require_takeaway_on_charts`); replaces hand-coded score→propose→repair→regenerate loops. Default `max_passes` 3; the final deck is always rendered. Returns `final_presentation` (full repaired JSON, always present — feed back into `validate_input`/`generate_presentation`/`repair_slide`), `gate_passed`, the publishability/evidence fields (`publishable`, `manual_review_required`, `blocking_reasons[]`, `content_status`, `evidence_complete`, `output_validation`, `render_evidence?`), and `next_state`/`resume_token` for resuming. **Default deterministic** (static + render-fit only, no rendering/API key); pass `visual_qa:{enabled:true}` to add a vision pass, `allow_degraded_scoring:true` to converge when renders fail. Full contract below: [Output Validation Guarantee](#output-validation-guarantee), [Visual-QA mode](#visual-qa-mode-auto_repair--make_deck), [Resumable convergence](#resumable-convergence-resume_token--next_state); per-field detail in [TOOLS.md](TOOLS.md). Each `trace[]` entry carries the repair funnel — `directives_proposed`, `directives_advisory` (findings whose remedy is an authoring decision), `directives_applied`, `directives_failed[{kind, slide_index, code, reason}]` — so a pass that changed nothing says why; `next_state.completion: "no_progress"` means the gate is unmet and no directive landed. |
| `make_deck` | SKELETON | **Skeleton / wireframe only — not the path to a real deck** (for that, author a DeckSpec and call `render_deck_spec`). One-call facade: hand it an `outline` (natural-language brief) → chains `plan_deck → expand patterns with exemplar content → auto_repair`, returning a **DRAFT** PPTX skeleton. Output is **never publishable** — with no caller content it fills slides with pattern exemplar PLACEHOLDER values, so it always reports `content_status: "exemplar_skeleton"`, `uses_exemplar_content: true`, `publishable: false`, **`gate_passed: false`** (with `"exemplar_content"` leading `gate_reasons[]` and `blocking_reasons[]`), `final_score: 0` and `content_score: 0`; the deterministic layout/fit score is reported separately as `structural_score`. Replace exemplar copy via `repair_slide`, then run visual QA / manual review before shipping. Same `gate`, `next_state`/`resume_token`, and publishability/evidence contract as `auto_repair`; adds `plan.slides[]` (per-slide pattern/role/title, for targeting `repair_slide` without re-planning) and `final_presentation` (full authored+repaired JSON). Optional `style_hints` (`slide_budget`, `audience`, `accent_strategy`, `must_include`), `max_repair_passes`, `allow_degraded_scoring`, `visual_qa`. Use it only when you want a wireframe to look at; author content with a DeckSpec (`render_deck_spec`) or the precondition workflow. Detail in [TOOLS.md](TOOLS.md). |

**Stop when the quality gate passes.** Every `score_deck` response carries a `quality_gate` block — the **machine-readable definition of done**. `auto_repair` and `make_deck` apply the SAME criteria by default, so their `gate_passed` means what `quality_gate.passed` means (go-slide-creator-ie9v): a deck cannot converge in the repair loop and then be refused by the ship check. Passing your own `gate` relaxes the loop's stopping rule — and then `gate_passed` is a stop signal, not a shipping verdict; re-check with `score_deck` before you call it done.

```json
"quality_gate": {
  "passed":   true,
  "reasons":  [],
  "criteria": {
    "min_score":                  80,
    "max_p0_findings":            0,
    "max_p1_findings":            0,
    "require_takeaway_on_charts": true,
    "allow_accent_overload":      false,
    "min_composition_score":      65,
    "max_problem_slides_pct":     40
  }
}
```

**Chart numbers say what the data says.** Data labels pick the precision that keeps them **distinct** rather than rounding to whole numbers — a series `[4.6 … 6.5]` labels every bar differently instead of printing "5, 5, 5, 6, 6, 6, 7" beside a "+6% a year" headline (go-slide-creator-66qb) — and thousands are grouped (`12,400`). Set `data.data_labels.format` to take control; it carries the units too (`"€%.1fM"`, `"%.1f%%"`). Axis ticks still format independently of the labels.

**Diagram text that must shrink says so.** A native diagram shape whose text overflows its box now stores the exact shrink (`<a:normAutofit fontScale="…">`) rather than leaving it to the renderer, so a deck looks the same in PowerPoint as in the thumbnails you inspect — before this, LibreOffice recomputed the fit and PowerPoint rendered at 100% and overflowed (go-slide-creator-wvr0). When that shrink takes the text under the `viewing_mode` readability floor you get `TEXT_BELOW_READABLE_MIN` naming the effective size and the scale (`… renders at 3.4pt … (autofit 28% to fit the shape)`). It is advisory, but on a 12-bullet quadrant it is the deck telling you to split the slide.

**Title length is measured, not counted.** A too-long title is reported against the actual title box of the layout the slide will land on — `title (112 chars) only fits its title placeholder at 60% of the template 45pt size; shorten to ≤ 71 chars` — and that measurement runs whether you write `layout_id`, only `slide_type`, or a DeckSpec slide kind (which carries neither). The 60-character rule of thumb survives only as the fallback when no layout can be resolved (go-slide-creator-t64e).

**What the score measures.** `overall_score` is the mean per-slide score (100 minus each finding's severity weight) pulled down by the SHARE of slides carrying a finding — a deck where most slides have something wrong is worse than its average slide suggests. Slides whose only findings are about airiness (`sparse_layout`, `SPARSE_FILL`, `SLIDE_UNDERUSED`, `cell_underfilled`, `pattern_underfilled`) or a wrapping title (`title_wraps`) do not count toward that share: a KPI slide with three big numbers is supposed to look sparse. `max_problem_slides_pct` applies the same count as a gate criterion (it needs at least 3 affected slides before it can fire, so a short deck is not failed by two blemishes).

**The gate also sees deck rhythm.** `min_composition_score` puts a floor under the `composition` axis — the same rhythm analysis `analyze_deck_rhythm` reports. It was computed and then discarded, so eight consecutive identical `kpi-3up` slides scored 100 and PASSED with composition 55 (go-slide-creator-xx9i); the single most common LLM deck failure, everything looking the same, had no consequence. A failing reason names the diagnostics that dropped it — `composition 55 < min_composition_score 65 (missing_emphasis, pattern_run)` — so you know whether to vary the layouts, add an emphasis slide (`stat-hero`, `pull-quote`), or spread the accents. The criterion is skipped when composition was not measured (the `slide_indices` subset path: three slides have no rhythm).

The gate also sees **content substance**, not just fit: `WEAK_CONTENT` (lorem ipsum / "Click to add title" / "XX%" — `refuse`), `MISSING_TITLE`, `SLIDE_NEARLY_EMPTY` (a content slide under 8 words of body), `DECK_MONOTONY` (4+ consecutive slides of the same shape; 6+ refuses) and `CHART_OVERLOADED` (more categories than a reader can follow). Before these, a deck whose every slide read "Lorem ipsum" scored 99 and passed; the score's rank agreement with blind human grades of the rendered decks went from +0.39 to +0.77 across the 16-deck calibration corpus in `cmd/json2pptx/testdata/calibration/`.

When `quality_gate.passed === true`, the deterministic precondition is met — stop the *score-driven* repair loop (no further `propose_repairs` / `repair_slides_batch` / `auto_repair` calls just to raise the number, no aesthetic polish for its own sake). The gate is **not** completion: now apply the [completion rule](#deck-generation-skill) — render all slides with `render_deck_thumbnails`, inspect every returned image, and repair (then re-render and re-inspect) anything you see wrong. When `passed === false`, `reasons[]` enumerates the unmet criteria in a stable order (`score → P0 → P1 → takeaway → accent_overload → composition → problem_slides`) so you can address the highest-impact issue first. The criteria block is fixed by the server (not configurable on `score_deck`) so the gate cannot be relaxed at call time — agents that need a different threshold should call `auto_repair` (which exposes a tunable gate) instead. The same gate semantics apply inside the visual-QA loop: stop iterating once `quality_gate.passed` flips true on a `score_deck` pass.

**`score_deck` grades a RAW deck.** Pass a semantic DeckSpec (slides carrying `kind`) and it is refused with `INVALID_PARAMETER` pointing at `validate_deck_spec` / `compile_deck_spec`. It used to unmarshal the spec into a deck whose slides were all empty and grade THAT — answering `overall_score 99, quality_gate PASS` for a deck it had never read (go-slide-creator-xx9i).

**`get_capabilities` takes a `sections` projection.** The default —
`[runtime, features, deprecations]`, about 6.5 KB — is what you need to detect drift and decide what the server can do. `sections: ["runtime"]` is 0.4 KB and still carries `schema_version` + `schema_fingerprint`. `sections: ["tools"]` returns the 74 KB tool catalogue, which `tools/list` already sent you, and `sections: ["all"]` is everything. Before this the single call cost 183 KB (~46K tokens) and was the first step `get_started` recommended (go-slide-creator-5pta).

**Discovery is compact by default.** `list_templates`, `list_patterns` and `list_icons` default to `fields: "compact"`; pass `fields: "full"` when you need the whole payload. `list_templates{}` is ~2.7 KB against ~108 KB for full, and its compact projection omits `supported_types` — that block is static per-server data, so fetch `chart_capabilities` / `diagram_capabilities` / `shape_geometries` from their own tools when you need them. Calling with no `fields` used to return the full payload and then advise you to ask for compact (go-slide-creator-dykl).

**Response size.** Tool results carry the payload as `structuredContent`. For a client that negotiated protocol **2025-06-18 or later** the duplicate JSON text copy in `content[0].text` is omitted — sending both doubled every response (389 KB on the wire for 168 KB of information across one pass of the core tools; `list_slide_kinds` alone went 61.6 KB → 19.6 KB). Older clients still get the text copy. If your client reads `content[0].text` despite negotiating a modern protocol, start the server with `--text-fallback=always` (or `JSON2PPTX_MCP_TEXT_FALLBACK=always`); `never` drops it for everyone (go-slide-creator-vxre).

**Compact responses.** Responses are always compact JSON; the server still advertises `experimental.compact_responses: true` and still honours the client capability and the deprecated `MCP_COMPACT_RESPONSES=1` environment variable, but neither changes anything.

---

## Visual-QA mode (auto_repair / make_deck)

`auto_repair` and `make_deck` run a **deterministic** convergence loop by default: they score the deck from static + render-fit findings and **never look at a rendered pixel**. This produces `draft_needs_visual_review`, not a completed deck. Completion is `visually_reviewed_current_revision`: render every slide from the current artifact, inspect every image with a configured provider or recorded host/manual review, repair, then render and inspect the repaired revision again. The response truth-labels the default as `quality_mode: "deterministic"`.

To add the agent-grade visual refinement loop, pass `visual_qa: {enabled: true}`. The phase runs AFTER the deterministic loop: render thumbnails → `inspect_slide_images` → map visual findings to `propose_repairs` → apply → re-render. Visual findings may carry an optional `bbox` (`{x,y,w,h}` as fractions of the slide); `propose_repairs` hit-tests it against the generated shape_grid cell bounds so directives target that cell (`/slides/N/shape_grid/rows/R/cells/C`, threaded into `reduce_cell_text.cell_path`) instead of the whole slide — include `bbox` when you forward host-reviewed findings. Only **P0/P1** visual findings drive automatic repairs (P2/P3 are advisory). Any repairs it applies are reflected in `final_presentation`.

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
- `ANTHROPIC_API_KEY` unset → `inspection_mode: "heuristic"` (pure-Go fallback) instead of vision; `quality.actual` stays `"deterministic+visual_qa"` (visual QA still ran) but a `fallback_reasons[]` entry flags the lower-rigor backend. **Heuristic mode cannot approve a deck**: it checks for blank slides, content/table slides concentrated above a largely unused lower content region, text-like ink against a slide edge, and a standard aspect ratio. The conservative layout-balance check is P2 with `source: "deterministic"`; the broader advisory checks remain P3 with `source: "heuristic"`. Vision mode also merges the deterministic layout-balance result so a clean model response cannot approve a half-empty table. Completion still requires looking at every rendered slide (or a vision provider) and recording the verdict with `submit_visual_review`. Its edge check ignores solid fills — a template's accent rail or takeaway band is decoration, not overflow (go-slide-creator-3pyf).
- **Malformed `visual_qa` is NOT a transparent fallback** — a *present* `visual_qa` that is not an object, or has a wrong-typed field, fails fast with `INVALID_PARAMETER` rather than silently disabling the mode (so you never lose a requested vision pass to a typo). Omit `visual_qa` (or pass `null`) for the deterministic default.
- A wedged renderer or stalled API call can no longer hang the loop: LibreOffice/ImageMagick subprocesses and each vision request run under bounded deadlines. A breach is reported as `LIBREOFFICE_TIMEOUT` / `IMAGEMAGICK_TIMEOUT` (render step, recorded in `notes[]`) or `VISION_TIMEOUT` (per-slide inspection error), each carrying the tool, elapsed time, and a retry/degrade action. The standalone `render_slide_image`, `render_slide_image_from_json`, and `render_deck_thumbnails` tools return the same `*_TIMEOUT` codes; resolve any of them with `describe_finding`.
- Every LibreOffice conversion runs in a profile private to this server process, so a second MCP server process, a parallel agent, or your own open LibreOffice no longer makes renders fail at random (go-slide-creator-0ixs). If one still does, `RENDER_FAILED` says so in words — "LibreOffice produced no PDF ... another LibreOffice instance was running, not that the deck is invalid" — and the conversion has already been retried once. Treat that message as an environment problem and retry the call; do not start editing a deck that is fine.

**Host/manual review completion** — without a vision provider, inspect every rendered slide yourself (or have a human do it) and record the verdict with `submit_visual_review` (`pptx_path`, `pptx_revision` = the generation `content_hash`, one `slides[]` entry per slide with `verdict` and `image_path`/`image_sha256`). **The image is not optional**: each entry needs `index`, `verdict` and one of `image_path` / `image_sha256` — a verdict with no pixels behind it cannot be checked against the artifact's own render and is rejected naming the missing field. `role` is optional and defaults to `"slide"`. Partial coverage or a stale revision is rejected; an all-approved review with no P0/P1 finding and **verified images** flips `evidence.approved` and `status: "visually_reviewed_current_revision"` (`inspection_backend: host|manual`). Re-submit after any regeneration — evidence is bound to the artifact hash.

**The images are the evidence, so they are checked** — submit the `path` / `content_hash` values `render_deck_thumbnails` (or `render_slide_image`) returned for *this* PPTX. Every slide's pixel hash is compared against this server's own render of that slide of that artifact (any density it was rendered at counts), and the response always carries `image_verification` `{status, method, verified_slides, total_slides, reasons[], how_to_verify}`:
- Submitting one slide's image for several slides, or another deck's images, is rejected with `INVALID_PARAMETER` naming which slide the image actually is. Two slides that genuinely render to identical pixels still verify — the rule is "each image is this slide's render", not "all hashes differ".
- When the server has no render of the artifact to compare against (you rendered elsewhere, or the render cache was cleared), the review is still recorded but `image_verification.status: "unverifiable"`, `evidence.pixels_rendered: false`, `status: "reviewed_unverified_images"` — **not** a completion status — and no `visual_evidence` is written to the authoring manifest. Render with `render_deck_thumbnails` and resubmit to reach completion.

**Failed inspection ≠ clean deck** — when a vision call errors (API failure, malformed output, `VISION_TIMEOUT`) or a heuristic decode fails, that slide is *not* inspected, so zero findings does not mean it passed. `inspect_slide_images` projects each failure to an **error-severity** finding (`VISION_INSPECTION_FAILED` / `VISION_TIMEOUT` / `HEURISTIC_INSPECTION_FAILED`, so `findings.ok` is false) and adds top-level `failed_slide_count` + `inspection_status` (`complete` / `partial` / `failed`); never treat an empty findings list as clean unless `inspection_status` is `complete`. In the `visual_qa` loop the same failure sets `visual_qa.inspection_complete: false`, populates `failed_slide_count`, records a `notes[]` entry, and stops the pass from being counted as a clean convergence.

**Maintainer quality benchmark.** `go run ./cmd/qualitybench` defaults its held-out family to the purpose-built side-logo portability fixture outside `templates/`. Its `--templates` entries accept `name=path:family[:heldout]`. Agent runs accept `--parallel N` for explicitly bounded concurrency while keeping deterministic evidence order; the default remains one worker. Supply authoritative `--agent-model` and `--agent-version` values so model-generated labels cannot vary across evidence records. Blind ratings are applied with `--report ... --ratings reviewer-a.csv,reviewer-b.csv`; the release decision counts two distinct `reviewer_type=human` reviewers per run, while optional `llm` ratings remain supplemental. For a repeatability study, `--prepare-agent-ratings` creates two blank blind ballots for each of exactly three agent IDs without launching them; after all six are filled, `--compare-agent-ratings` reports every raw ballot, per-agent run deltas, median-of-agent-means numeric consensus, and strict-majority boolean consensus with 3–3 ties left unresolved. See [docs/QUALITY_BENCHMARK.md](../../docs/QUALITY_BENCHMARK.md).

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
| `ooxml` | `raster_fallback` | `OOXML_LOW_FALLBACK_DPI` (SVG picture's paired PNG is below 96 DPI at its displayed size; advisory for SVG-unaware viewers) |

`OPC_*` and the structural-corruption `OOXML_*` codes (`OOXML_ILLEGAL_XML_CHAR`, `OOXML_SLIDE_COUNT_MISMATCH`, `OOXML_INVALID_TABLE`) are always promoted to `severity: "blocking"`. Other `OOXML_*` codes are advisory `warning`s and do not fail strict mode unless the validator escalates them.

`OOXML_INVALID_TABLE` fires when a table row's `<a:tc>` count does not match the table's `<a:gridCol>` count. It blocks because readers align cells positionally: the row's remaining cells shift left and a merged cell's text is dropped outright (LibreOffice paints it as an empty block). Author merges with `col_span` / `row_span` on the origin cell only — the engine materialises the `hMerge` / `vMerge` continuation cells ECMA-376 requires, so you never write them yourself.

Grid-cell charts and diagrams ship text-preserving, frame-sized PNG fallbacks alongside their SVGs. Their generation requires `rsvg-convert` or `resvg`; without either converter, generation fails instead of silently shipping an unlabeled raster. Small SVG icons still use the lightweight icon fallback path.

### Dropped content also fails strict mode

Strict `output_validation` covers a second class of failure: **content the engine could not place at all**. When a content block targets a `placeholder_id` the resolved layout does not declare, the block is simply absent from the rendered slide. The response reports this three ways:

- a `CONTENT_DROPPED` fit finding at `/slides/{i}/content/{n}` whose `fix.params.cause` is `"placeholder_not_found"` (plus `placeholder_id`, `layout_id`, and the `available` ids) — see [FIT_FINDINGS.md](../../docs/FIT_FINDINGS.md#content_dropped);
- **`success: false`** under `output_validation: "strict"` (the default). Under `warn` / `off` the finding is still emitted and `success` stays `true`;
- the dropped id is **excluded from that slide's `placeholders_used`** and listed in **`placeholders_dropped`** instead. `placeholders_used` reports what was actually populated, never what was merely requested, and `occupancy_pct` is computed from the reduced set.

Other `CONTENT_DROPPED` causes (a slide skipped in `--partial` mode, two visuals colliding on one placeholder) stay advisory and leave `success` untouched. So: read `placeholders_dropped` and the `cause` param, not just `success`, when a slide renders emptier than you expect.

### Validation evidence on the repair facades

`auto_repair` and `make_deck` succeed at the transport layer even when something went wrong, so they expose **explicit evidence and status fields** instead of letting a clean-looking response (a successful tool call with a `path`) imply a publishable deck. Never treat artifact existence or `gate_passed` alone as "done":

- **`deterministic_ready`** (always present, boolean) — everything the engine can decide on its own: the gate passed on complete evidence, the artifact is structurally valid, **and** content is author-supplied. This is the flag the **default path can reach**, and the one to gate "does this deck still need editing?" on. Equivalent to `deterministic_blocking_reasons` being empty.
- **`publishable`** (always present, boolean) — `deterministic_ready` **AND** a current all-slide visual inspection with an explicit approved verdict. **Unreachable on the default path**, which renders no pixels: call `render_deck_thumbnails`, look at every slide, then `submit_visual_review`. Equivalent to `blocking_reasons` being empty. `make_deck` output is **always `publishable: false` and `gate_passed: false`** because its slides are exemplar placeholders (`blocking_reasons[0] == "exemplar_content"`; `final_score`/`content_score` are 0 and the layout/fit score moves to `structural_score`). Gate "can this ship unseen?" on `publishable`; do not read a `false` here as "the deck is broken" when `deterministic_ready` is `true` — it means nobody has looked at it yet.
- **`manual_review_required`** (always present, boolean) — the affirmative inverse of `publishable`: a human or agent must review before the deck ships (gate failed, evidence incomplete, structurally invalid, exemplar content, or simply not yet looked at).
- **`blocking_reasons`** (present only when not publishable, `[string]`) — every reason the deck is not publishable, a **superset of `gate_reasons`** that also folds in incomplete-evidence, exemplar-content and missing-visual-verdict causes. **`deterministic_blocking_reasons`** is the same list minus the visual-review entry: what is still fixable by editing. Branch on that one to decide what to fix.
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

Branch on `ok` (false when any error-severity finding is present), then walk `findings`. Every tool's `outputSchema` is declared as `{anyOf: [<success shape>, <error envelope>]}`, so a validating MCP client accepts both — an error response's `structuredContent` conforms to the same schema its success response does. Each finding carries a namespaced `code` (`<CATEGORY>.<legacy_code>`, e.g. `INPUT.MISSING_PARAMETER`, `TPL.TEMPLATE_NOT_FOUND`, `RENDER.URL_FETCH_FAILED`), the offending JSON `path` and `expected_type` under `evidence`, and — for repairable issues — a structured `remediation.primary.{action, params}`. Arg-validation findings guarantee a non-empty `evidence.path` plus at least one of `evidence.expected_type` or `next_tool_call` so an agent can self-correct without re-reading the schema. The `next_tool_call` usually replays the same tool with the offending field as a placeholder; for shape failures it points at `get_input_schema`, and for unknown identifiers it points at the relevant discovery tool (e.g. `list_templates`, `list_patterns`). Discovery findings also carry list facts in `evidence` (e.g. icon-name `suggestions`) so you can repair without a separate lookup. Common legacy codes (after the namespace prefix) are `MISSING_PARAMETER`, `INVALID_PARAMETER`, `INVALID_JSON`, `INVALID_KEY`, and `INVALID_PATH`.

**A present-but-wrong-typed argument is `INVALID_PARAMETER`, never `MISSING_PARAMETER`.** `describe_finding{code: 12345}` answers "code must be a string, got a number" with `evidence.expected_type` and a `next_tool_call` retry — not "code is required", which would send you to re-add a field that is already there. `MISSING_PARAMETER` means the key is genuinely absent. The same holds for a nested path (`presentation.slides`), and `get_started{task: <non-string>}` is an error rather than a silent fallback (an unknown task STRING still falls back to `brief`, which is the documented default).

**Unknown arguments are rejected, never ignored.** Every MCP tool call is checked against the tool's declared input schema before the handler runs. An argument name the tool does not accept (a typo) fails the call with `INPUT.UNKNOWN_PARAMETER` at `evidence.path` = the bad name; the message lists every accepted argument and, when a close match exists, `fix: {kind: "rename_field", params: {from, to, did_you_mean}}` plus a `next_tool_call` retry. Rename and retry — do not assume the setting was applied.

> **Sibling spellings are accepted, not rejected** (go-slide-creator-r1m3). Three tools name a shared concept differently from the rest, and each is now accepted under the majority spelling and rewritten before the handler runs: `validate_presentation_output` takes `path` but accepts `pptx_path`; `resolve_theme` (and `examine_template`, `list_template_settings`, `register_template_setting`, `delete_template_setting`) take `template_name` but accept `template`; `plan_deck` takes `slide_budget` but accepts `slide_count`. Send either. When both are sent the **declared** name wins.

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

   A Sankey request returns `candidates: []` and `unsupported_visual: "sankey"` because no Sankey renderer is registered. Do not substitute `process_flow` for quantitative flow widths. Pricing plans and subscription tiers route to parallel `card-grid` cards.

   **Template-aware ranking.** Pass `template` (a template name, e.g. `"midnight-blue"`) to vet each candidate against that specific template. Every candidate then carries `template_support: {status, reasons[], required_layout}`:
   - `status: "supported"` — the template natively covers the layout/capability the candidate needs.
   - `status: "risky"` — producible only via a synthesised/derived layout (e.g. a two-column built by splitting a One Content layout), or close to a body-capacity / content-zone limit. Read `reasons[]` for the caveat.
   - `status: "unsupported"` — the candidate needs a canonical or derivable layout the template cannot provide (`reasons[]` names what is missing).

   The engine **demotes** risky/unsupported candidates in the ranking, so the top candidate is feasible for the template whenever any feasible option exists. The displayed `score` is left untouched (it stays the intent-match score); only ordering changes. `required_layout` names the canonical layout or derivable capability the candidate needs (`"Title Slide"`, `"Two Content"`, `"full-image"`, `"grid base"`, …). The same support logic also powers `plan_deck` template-awareness. Without `template`, candidates carry no `template_support` (template-agnostic ranking, unchanged behaviour).

   Compose envelopes accept 2 to **N** top-level segments — the enforced cap is published as `get_capabilities().features.compose.max_segments` (default 8) along with the supported `directions` and `supports_smart_compose` flag. For arrangements that exceed the cap, nest a compose envelope inside a segment instead of flattening: a `SegmentInput` may set `compose` (XOR with `pattern`) to host a child envelope. Nesting depth and total leaf count caps are published under the same `compose` feature block.

   **Diagram segments.** A `SegmentInput` may set `diagram: {type, data, style?}` as a third XOR alternative to `pattern` / `compose`. Diagram segments let a native pattern coexist with an svggen chart/diagram on the same slide without flattening the pattern through a single-cell grid. For native `process_flow` with `data.direction: "vertical"`, boxes stay content-sized (maximum 40% of the diagram width), decision targets use left/right branch lanes (`Yes` left, `No` right), and labels sit beside the connector in the gap. A frame too short to preserve readable step/label geometry reports `diagram.text_overlap`; enlarge it or reduce the flow rather than accepting auto-shrunk copy.

   **Compose-candidate discovery.** `recommend_visual` emits candidates with `category == "compose"` when the intent contains a multi-pattern keyword or the top two pattern candidates declare mutual compose-affinity. The candidate's `placement.composable_with` carries the **specific pair of sibling pattern names** to drop into a `ComposeInput.segments[]`.

   **Envelope-level banner and callout.** A `ComposeInput` may set `banner: {text, emphasis?, accent?}` and/or `callout: {text, emphasis?, accent?}`. These do **not** consume a segment slot. Constraint: validation rejects `banner` when the first segment's pattern is itself banner-leading (currently `strategy-house` and `pull-quote`).

   **Slide-level overlays.** `SlideInput.overlays: []OverlayShape` adds free-floating shapes rendered on top of the grid (arrows, lines, badges). Endpoints are either `{x, y}` percentages or `{anchor_cell: {row, col, at}}` with anchor positions like `"center"`, `"top-left"`, etc. Arrow overlays whose endpoints both target `anchor_cell` with `at: "center"` on text-bearing cells are auto-routed to the cell corners facing the opposite endpoint, keeping arrowheads off the label centers. Arrow stroke colors with poor contrast (<3:1 WCAG AA Large) against an endpoint cell's resolved fill are auto-flipped to white or near-black, whichever has the best worst-case contrast across both endpoints. Overlays are emitted **last** in the shape tree, so they paint above grid cells, charts, icons and images alike — numbered callouts on a screenshot work (go-slide-creator-yomm).
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

`content` is body-capable: the engine populates a body placeholder with your content item. Generation **fails** (instead of silently dropping content) when two content items with different `placeholder_id`s fall back onto the same physical placeholder — e.g. `body_2` on a one-content layout resolving onto `body` alongside a chart; the error names both items and the placeholder. Use `two-column` for `body` + `body_2`. The blank layouts are shape-grid-oriented: there is no body placeholder, so all content must come from `shape_grid` or `pattern`. `blank-title` keeps a title slot; `blank-canvas` reserves nothing. With an authored title, `slide_type: "blank"` selects a title-bearing blank canvas when available; without one, it can select a truly empty canvas. For a specific role, pin `layout_id: "blank-title"` or `"blank-canvas"`. A `shape_grid`/`pattern` without `layout_id` binds the title canvas before heuristic scoring and computes grid bounds below its title.

**Canonical `layout_id` is resolved against the active template.** An explicit canonical name (`title`, `content`, `blank`, `blank-title`, `blank-canvas`, `section`, `closing`, `two-column`, `image-left`, …) resolves only when that template has a matching layout. Check `list_templates.canonical_layout_availability` first: `image-left`, `image-right`, `quote`, and `agenda` are optional, not universal. Detailed discovery's `canonical_layout_ids` maps the available names to concrete layouts. You do **not** pass raw `slideLayoutN` IDs; a concrete ID, generated layout ID (`content-2-50-50`, `grid-2x2`), or known alias (`2-col`, …) is still accepted for compatibility. An unavailable canonical name is an error listing available canonical names and a suggestion when one is appropriate; it does not auto-select another layout.

---

## Workflow: Plan → Vary → Render → Repair

### PRECONDITION: Validate Before You Generate

**You MUST NOT call `generate_presentation` until all of the following have succeeded for the current deck:**

1. **Visual discovery.** For each slide, call `recommend_visual` to determine the best visual approach. If you already know the slide needs a named pattern, `recommend_pattern` or `list_patterns` is sufficient. Do not guess pattern names from memory.
2. **Schema inspection.** For each chosen pattern, call `show_pattern` to retrieve the value schema and `example_values`.
3. **Density pre-flight.** For each pattern slide, call `expand_pattern` with your populated values to confirm density is in the 35–110% optimal band (a height ratio: >110% means the cell will be autofit-shrunk, <35% that it reads as mostly empty).
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

1. **PLAN** — produce a short outline (template, accent strategy, slide-by-slide list of layouts + patterns + accents). Use `plan_deck` for decks >4 slides. Each planned slide carries a canonical `layout` (equal to its `skeleton.layout_id`): slide 0 is `"title"` and the last slide is `"closing"`, both with **no pattern** (`recommended_pattern: ""`, layout-only skeleton with `title` + `subtitle`); content slides are `"blank-title"` + a pattern. `comparison` slots map only to `comparison-2col` / `before-after`, and emphasis patterns (`stat-hero`, `pull-quote`, `kpi-inline`) are capped at ceil(n/5) per deck (`must_include` placements are kept). The brief's facts — clauses with quantities (`+23% revenue`, `churn 4%`) or named entities (`EU expansion`) — are carried verbatim into slide `content_seed`s and per-slide `facts[]` (quantities to KPI/stat/chart slides first); anything that did not fit is listed in the top-level `unplaced_facts[]` — use those numbers as the slide content, and add a slide or fold in every `unplaced_facts` entry rather than dropping it. Facts are cleaned for copy-paste: a brief written as a bulleted list loses its `-` / `*` / `1.` markers, a clause cut inside a bracket drops the half-open parenthetical rather than shipping it, and two facts on one slide are two sentences in the seed, never spliced with a semicolon. **Pass the optional `template` (a template name) to make the plan template-aware:** every planned slide — and each `alternatives[]` entry — then carries `template_support: {status: supported|risky|unsupported, reasons[], required_layout}` from the same shared helper `recommend_visual` uses, so the two tools agree for identical template constraints. A recommended pattern the template cannot host is swapped for a supported alternative during planning, so the plan never assigns an impossible pattern; the result also echoes the vetted `template`. Without `template`, slides carry no `template_support` (template-agnostic plan, unchanged).

   **The plan may be shorter than `slide_budget`.** Each narrative role is bounded by what can actually fill it: the patterns the role may draw on (a `comparison` slot has only `comparison-2col` / `before-after`, so a third comparison slide must repeat one) and what the brief carries for it (one "build vs buy" is one comparison slide, not one per five slides). Surplus slots move to roles with headroom; when none has any, the plan returns fewer slides and says so in the top-level `budget_note` — read it and either accept the shorter deck or give the brief the content the extra slides would have carried. Padding was the cause of the repetition agents used to see: `rhythm_check.repeated_families` is now empty on a full-length plan, and non-empty only alongside a `budget_note`.
2. **VARY** — call `analyze_deck_rhythm` and act on `longest_run`, `accent_balance`, `density_cv`, `composition_score`.
3. **RENDER** — generate the JSON in one pass; verify the pre-emit checklist (Rule 20, semantic fills, gap ≥4pt, accent variety, 35–110% density).
4. **REPAIR** — `validate_input` → `generate_presentation` → `render_deck_thumbnails` (all slides) → inspect every image (`inspect_slide_images` or your own review) → `repair_slide` → re-render. Images are truth; done means every slide of the current revision was rendered and inspected.

**Rendered slides arrive as images you can see.** `render_slide_image`, `render_slide_image_from_json`, and `render_deck_thumbnails` return each slide as a native MCP image content block (`image/jpeg`, max 1280px wide) after a small JSON metadata block (`delivery: "image_content"`, per-slide `index` / `path` / `image_content_index`, no base64). Look at the images directly — no `ANTHROPIC_API_KEY` or `inspect_slide_images` round-trip is needed to see the slides; hand the returned `path`s to `inspect_slide_images` only when you want its categorized findings. Clients that cannot display MCP images can pass `include_base64_json: true` to get the legacy base64-PNG-in-JSON envelope (the CLI `render-slide` / `render-thumbnails` / `render-slide-from-json` subcommands always print that legacy JSON).

**Re-inspect the slides that changed, not the deck.** A 15-slide deck is ~370KB of
base64 per thumbnail pass, and a repair loop that re-pulls all of it to look at one fixed slide
spends nearly all of its context on images it has already seen. `render_deck_thumbnails` takes
**`slide_indices: [int]`** — render ONLY those 0-based slides, one image block each, ascending.
Pass `render_deck_spec` / `validate_deck_spec`'s `changed_slides` verbatim. The response carries
`slide_count` (the deck's size, whatever came back) and `selected` (what did), so a narrowed pass
never reads as a whole deck. An index the deck does not have is an error naming the real count, not
a silent omission; an empty array is an error too — omit the argument to render everything.
`slide_indices` is mutually exclusive with `max_slides`, which caps a prefix rather than naming
slides. `render_slide_image(pptx_path, slide_index)` remains the one-slide form. None of this
relaxes completion: the revision you ship still has to have been seen whole (see the
[completion rule](#deck-generation-skill)).

**One score scale, explicit basis.** Every deck-quality score is 0-100 and names what it measured: `generate_presentation.quality` / `render_deck_spec.quality_summary` carry `basis: "input"` (static heuristics over the input JSON); `score_deck.overall_score` carries `basis: "structural"` and `auto_repair` / `make_deck` `final_score` carry `score_basis: "structural"` (deterministic rules over the generated deck — no pixels). None of them is a visual verdict (`"rendered"` is reserved for pixel-derived scores); look at the rendered slides for that.

For the `repair_slide` fix-kind vocabulary, finding-code catalog, and strict-fit promotion ladder, see [FINDINGS.md](FINDINGS.md).

---

## Pattern Library (overview)

For BMC, KPI grids, 2x2 matrices, timelines, card grids, icon rows, two-column comparisons, accent-banded panels (`stylish-panels`), strategy-house frameworks, executive SCQA summaries (`scqa-summary`), chart-with-takeaway layouts (`chart-insights-split`), ranked horizontal bars with per-bar callouts (`horizontal-bar-with-callouts` — 3–8 bars; each bar binds to one insight via a left accent bar, and a bar without a `callout` gets no tick; leave `callout` off every bar and the callout column is dropped so the bars span the full width), waterfall / bridge bar charts (`waterfall-bridge` — 3–10 columns of total + delta + subtotal bars showing how components reconcile a start total to an end total; floating delta bars and auto-computed subtotals; a currency `unit` such as `"$m"` renders as `$210m` / `−$41m`; a bridge draws no value axis, so set `caption` to state the scale once — a delta too thin to hold its own label gets it in the adjacent spacer, against the bar), value / cost driver trees (`driver-tree` — root metric → 2–4 branches → 1–4 leaves each, with optional per-branch annotations; for **people/role** hierarchies use the svggen `org_chart` diagram type instead), Porter-style value chains (`value-chain` — 4–10 step columns with per-step description and optional highlight), double-track process grids (`process-grid-2row` — two parallel rows of 3–6 phase columns sharing the same N columns, with a dk2 row-label column on the left and per-row accent fill), ordered numbered steps without branching (`numbered-step-strip` — 3–6 steps in `chevron` / `stacked-box` / `toc` styles, each with an optional per-step detail zone; never emits decision diamonds — use `process-flow` when the sequence has decision points), maturity ladders (`journey-maturity-model` — 3–6 stage columns with numbered headers, descriptions, and an optional 'where we are' marker), visual deck previews (`agenda-with-images` — 3–6 numbered agenda rows each with title/subtitle and an image or quote placeholder; `image_label` is a caption, not a switch — a row without one still gets its placeholder, and only a deck with no labels at all drops the column), team / 'Our People' pages (`team-bios` — 1–8 members, each with a headshot (`members[].photo`) or an initials placeholder above name + role + short bio, up to 4 per row), and joint-venture / engagement-team paired-role slides (`dual-org-ladder` — 2–6 paired role rows across two parallel org columns, each column with its own org-name header, optional thin connector line between paired cards), use json2pptx's named patterns. Named patterns expand to validated `shape_grid` structures at generation time, replacing ~600 tokens of boilerplate with ~100 tokens.

**Chart so-what.** `chart-insights-split` also takes `headline` `{value, label}` (big accent number above the insights), `so_what` (tinted callout under them), `unit` / `chart_label` (caption above the chart — defaults to the single series name + unit, e.g. `Revenue ($M)`, because single-series charts show no legend) and adds value labels to bar charts (≤16 points) and single-series line/area charts (≤12 points) by default (`overrides.data_labels` true/false forces it). Lead with the number, end with the implication.

**Content-sized narrative / evaluation patterns.** `exec-summary` states 3–5 bold lead-in conclusions (`points[].lead` ≤90 chars) each with one supporting sentence (`support` ≤200 chars), separated by thin rules, plus an optional `bottom_line` ask below a closing rule — a pointing accent flag labelled BOTTOM LINE followed by the statement in a tinted box, so the conclusion does not read as another pale band — use it for answer-first key-message slides; keep `scqa-summary` for an explicit Situation / Complication / Questions / Answer arc. Rows are sized from the measured text (the type scale steps down 17/14 → 14/12pt before anything would shrink below 12pt), so short summaries stay compact instead of stretching to the full slide. `table-highlight` is the options × criteria evaluation matrix: `criteria` (2–6, strings or `{label, scale}`), `options` (2–6 × `{name, detail?, scores[]}` with one score per criterion), `scale` `harvey` (0–4 or `none`/`quarter`/`half`/`three-quarter`/`full`) | `rag` (`red`/`amber`/`green`, `r`/`a`/`g`) | `text` (≤24 chars), `"-"` for n/a. Set `highlight_row` (+ optional `highlight_label`, e.g. `"Recommended"`) on the recommended option and `highlight_col` on the decisive criterion; a legend row explains the symbols (`legend_labels` [high, mid, low], `show_legend:false` to drop it). `image-text-split` puts one picture beside a text column: `image` `{path | url, alt}` (relative paths resolve against the deck JSON directory, URLs are fetched and cached — same as shape_grid image cells), optional `caption`, `eyebrow` (e.g. `"Case study"`), `heading`, `body` (≤300 chars) and/or ≤5 `bullets`, plus 0–3 `metrics` `{value, label}` under the text; `overrides.image_side` (`left`/`right`), `image_width_pct` (30–60). The picture is cover-cropped to its panel (no distortion); without an `image` the pattern draws a dashed placeholder labelled with `image_label` — replace it before shipping.

**Pictures in `team-bios` and `pull-quote`.** The same `{path | url, alt}` reference works in two more patterns, so an "Our team" page and a customer quote with a headshot no longer need a hand-built `shape_grid`. `team-bios` takes one per person as `members[].photo`; a member without it keeps the initials tile, so photographed and un-photographed people mix on one slide, and `alt` defaults to the member's name and role. `pull-quote` takes `values.image` with `overrides.image_side` (`left` default / `right`) and `overrides.image_width_pct` (15–40, default 25); `alt` defaults to the attribution and role. In both, relative paths resolve against the deck JSON directory and URLs are fetched and cached, exactly as for shape_grid image cells. A `pull-quote` headshot takes a quarter of the width from the quote, so shorten a photographed quote — the readability preflight measures long italic quote prose against the 12pt projected-body floor and reports `TEXT_BELOW_READABLE_MIN` when it shrinks below that floor. Numeric KPI values retain their 18pt floor.

**Tip — chart + narrative on the same slide.** `chart-insights-split` is the canonical "data on the left, interpretation on the right" consulting layout: pass a `chart` (any `types.DiagramSpec` shape, or the `{label: value}` shorthand `chart_value` accepts — e.g. `{"type": "bar", "data": {"Q1": 12, "Q2": 14}}`, normalized to `categories`/`series` in key order) plus 1–6 `insights` bullets. `validate_input` expands the pattern and runs the same svggen check `generate_presentation` does: a chart generate would reject is an error diagnostic plus a `diagram_render_failed` fit finding with `action: refuse`. If you ship the pattern without a `chart`, the engine renders insights full-width and emits `CHART_PLACEHOLDER_EMPTY` (`action: review`) so you know the panel collapsed — supply a chart or swap to an insights-only pattern. Like every warning a pattern raises about its own content (`BODY_TOO_LONG` on an over-budget `team-bios` bio, `MULTIPLE_CURRENT_STAGES` on a maturity model), it reaches you from **every** surface: `validate_input`, `generate_presentation(fit_report=true)`, `score_deck`, `render_deck_spec`, `preview_presentation_plan`, and as a raw `warnings[]` line from `expand_pattern`.

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

**Content-sized pattern blocks.** Patterns size their cards/steps to their content and centre the block vertically in the content area (expanded grids carry `vertical_align: "center"`): `quote-cluster` bubbles and `value-chain` descriptions hug their text (a short quote is centred in its row rather than hanging at the top), `arch-stack` side rails are thin bands with rotated labels instead of full-height columns, `strategy-house` pillars are drawn on the template's subtle surface so the column reads down to the foundation, `kpi-2up`…`kpi-6up` cards and `icon-row` cards rest at 45% of the content height and are raised only when the tallest card's text needs more, never past the content area (an `icon-row` card with a `secondary` chart keeps the full zone — the chart needs the height), `process-flow` steps at 35%, `timeline-horizontal` (default `dots` style = date row + accent-dot axis + label/body under each dot) and `phase-roadmap` rows hug their text, header bands in `before-after(-compact)` / `stylish-panels` are ~1.2× the header line height, `before-after` bodies (default 14pt bullets), `stylish-panels` bodies, `hero-detail` rows and `card-grid` rows hug their text (sparse card text is vertically centred), and height-capped variants (`*-compact`, `kpi-inline`, `numbered-step-strip` chevrons) are centred rather than top-anchored — the centring respects the takeaway/source band. A `bounds` / `max_height_pct` override keeps the old top-anchored, fill-the-box behaviour. For a raw `shape_grid`, set `vertical_align` (`stretch` default, `top`, `center`, `bottom`) together with a row `max_height` to get the same content-sized, centred block. **A native table is content-sized and centred too**: its rows hug their text, and the table block sits at the vertical centre of its placeholder (or of its grid cell) rather than hanging from the top, so a short table no longer leaves the bottom half of the slide blank (go-slide-creator-6jv7).

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
- **Charts (Rules 8–10):** `series[i].values` length equals `len(categories)`; chart types use underscores (`stacked_bar`, NOT `stacked-bar`); don't mix data formats. **Chart styling — the whole surface.** `chart_value.style` accepts exactly `show_values` (draw the value on each bar / point / slice — this is the data-labels switch), `show_legend` (force the legend on, single-series charts included), `colors` (hex per series, overriding the template's data palette), `font_family`, `background` and `value_format`. `chart_value.chart_style` accepts exactly `show_vertical_gridlines` and `show_single_series_legend`, the narrower token overrides — the latter wins over `style.show_legend` when both are set. **Both blocks are closed**: anything else (`palette`, `font_size`, `show_grid`, `subtitle`) is reported as an unknown field by `validate_input`, not ignored. Chart text sizes come from the template's type scale; there is no per-chart font size. Call `get_data_format_hints` for the same list with a line of semantics per key (`chart_style_hints`). **One number format per chart:** `style.value_format` `{style: plain|compact|percent|currency, decimals, prefix, suffix, thousands_sep}` governs the value-axis ticks, the data labels and any in-mark label together — `{"style":"compact","prefix":"€"}` puts `€1.2M` on both the axis and the bars, which no argument could do before (the axis and the labels were formatted by different code). Percent treats values in `[0,1]` as fractions (`0.412` → `41.2%`); values above `1` are preserved as already-scaled percentages and raise `chart.percent_scale_ambiguous`. Currency should set `prefix`; when omitted the renderer uses the generic `¤` marker so it is never a plain-number alias and raises `chart.currency_prefix_defaulted`. Omit `value_format` and the renderer picks one format for both sides: grouped digits (`1,240`) with enough decimals to keep the labels distinct, switching to compact notation (`1.2M`) on a chart with a value axis once the numbers pass 9,999. `decimals` fixes the precision on both sides; `thousands_sep` forces grouping on or off; the axis otherwise keeps the precision its tick step needs (a whole-numbered axis prints `1`, not `1.0`). A misspelled key inside `value_format` is reported as an unknown field, not dropped. **Legend defaults:** bar / line / grouped-bar / area charts with 2–4 series render inline series labels and suppress the legend (executive default); above 4 series the legend reappears. Force the legend back on inside the 2–4 window with `chart_value.style.show_legend: true`. Stacked variants and non-Cartesian charts (pie, donut, scatter, radar, waterfall, funnel, gauge, treemap) keep the previous legend behaviour.
- **Content & layout (Rules 11–15):** `layout_id` must be canonical (`title`, `content`, `blank`, `section`, `closing`, `two-column`, …); semantic fills (`accent1`, `lt2`, `dk1`) required, never mix with hex on one slide; align values are `"l"`/`"ctr"`/`"r"`/`"just"`, vertical align is `"t"`/`"ctr"`/`"b"`.
- **A slide that argues from data needs a takeaway.** `show_pattern` returns `data_visual: true` for the patterns whose job is to make a quantitative claim (`chart-insights-split`, `waterfall-bridge`, `horizontal-bar-with-callouts`, `table-highlight`, `matrix-2x2`). A slide using one — or carrying a chart / chart-shaped `diagram_value` — with an empty `takeaway` reports `takeaway_missing` at `action: review`. A title that is itself the argument suppresses it: six words or more with a verb ("Prioritise the four initiatives in the top-right quadrant"), so you never have to say the same sentence twice.
- **Alt text on every visual.** `chart_value`, `diagram_value`, `table_value`, and a `shape_grid` cell's `diagram` / `table` each take an optional `alt`: one sentence saying what the visual shows, written verbatim into the shape's `cNvPr/@descr`. Omit it and the engine derives one from the payload (`"Bar chart, Quarterly revenue ($M). 4 categories, 1 series (Revenue), values from 34 to 48."`, `"Process flow. 5 steps."`, `"Table, 4 columns by 5 rows. Columns: Segment, FY25 revenue, FY26 revenue, Change."`) and reports `MISSING_ALT_TEXT` at `action: review` — the derivation says what is in the visual, only you can say what it is FOR. Pattern-expanded grid cells are exempt. DeckSpec visuals get an `alt` for free: `chart_insight`, `framework` and `table` compile the slide's `takeaway` (else its `title`) into it, and an `alt` on your own `chart` payload overrides that.
- **Contrast auto-fix (Rule 16):** engine auto-replaces low-contrast text to reach WCAG AA for that text's size; surfaces as `contrast_autofixed` findings. Tinted shape_grid fills (`alpha`, `lumMod`/`lumOff`) are judged by their effective tinted colour, so dark text on a light tint is kept. **Sibling cells are decided together:** when several shape_grid cells start from the same text colour and their fills are one visual family (progressive tints of an accent, say), the replacement is chosen ONCE against the worst fill in the group and applied to all of them — a tinted stack no longer comes out white on one tier, black on the next and grey on the last two. The finding says so (`across N sibling cells`, `fix.params.cells`, `source: "shape_grid_group"`) and there is one per group, not one per cell. A tonal shade of the fill's own hue is preferred over the palette's near-black when the palette misses the bar by a hair. Cells whose fills are NOT alike (a dark card beside a pale one) are left to the per-cell decision: no single colour suits both, and forcing one makes the dark card worse to help the pale one. **Text that states no colour is checked too:** a placeholder inherits its colour from the layout's `lstStyle` or the master's `txStyles`, through the layout's colour-map override, and an inverted layout can turn that into white text on a white background. The engine resolves the inherited colour, pins an explicit one when it fails, and reports it with `fix.params.source: "layout-lstStyle"` or `"master-txStyles"` (go-slide-creator-ucmgr). **On a background YOU set** (`background.color`, or an opaque scrim over a photo) the fix snaps to the template's palette rather than lerping: the template's title colour was chosen against the template's background, not yours, so there is no hue to preserve and a navy title on a black slide becomes `lt1` white (21:1) rather than a mid-grey at exactly the WCAG floor. `validate` predicts it too — `contrast_predicted` with `fix.params.source: "slide_background"`, naming the same colour generate will swap in. Template backgrounds and `shape_grid` cells still lerp: there you chose the fill and the text colour together (go-slide-creator-s7wmh). **Natively-drawn diagrams choose their own text colour per shape:** a `heatmap` prints each cell's value in `lt1` or `dk1` — whichever reads better on that cell's tint — so the darkest tiles are legible on every template (go-slide-creator-vdvs). A `heatmap` also shortens any row/column label that does not fit its box, with an ellipsis, and reports it as `grid_diagram_narrow` naming the grid size — dense grids used to wrap labels over each other into mush with nothing reported (go-slide-creator-3rkpt). Keep labels short, or the grid small, to show them in full. Diagram text does not pass through the placeholder contrast pass, so this is decided where the shape is drawn.
- **Chart annotations and value labels (Cartesian charts).** `bar`, `line`, `area`, `stacked_bar`, `grouped_bar`, and `stacked_area` accept two optional data keys that carry the consulting-standard chart furniture:
  - `annotations` — an array of `{kind, ...}` objects. `reference_line` (`axis` `"x"`/`"y"` default `"y"`, `value`, `label`, `style` `solid`/`dashed`/`dotted`, `color`) draws a target or threshold line; `trendline` (`series`, `method: "linear"`, `label`, `style`, `color`) fits and draws a trend over one series; `callout` (`x`, `y`, `text`, `color`) drops a positioned annotation on the plot.
  - `data_labels` — `{format, show_on}` turns on per-point value labels. `format` is a Go number format (`"$%.0fM"`, `"%.1f%%"`); `show_on` ∈ `all` (default), `last`, `peaks`, `first_last`.

  Both are declared on the chart data object, beside `categories` / `series`:

  ```json
  {
    "type": "bar_chart",
    "data": {
      "categories": ["Q1", "Q2", "Q3", "Q4"],
      "series": [{ "name": "Revenue", "values": [120, 145, 160, 195] }],
      "annotations": [
        { "kind": "reference_line", "axis": "y", "value": 180, "label": "Target: $180M", "style": "dashed" }
      ],
      "data_labels": { "format": "$%.0fM", "show_on": "all" }
    }
  }
  ```

  Non-Cartesian charts (pie, donut, scatter, bubble, waterfall, funnel, gauge, treemap, radar-only schemas) do not declare these keys and reject them as `UNKNOWN_FIELD`.

- **Inline markup is a closed vocabulary.** Text fields render `<b>`, `<i>`, `<u>`, `<sup>` and `<sub>` — nothing else. `<sup>` / `<sub>` become real OOXML baseline shifts, so a footnote marker is `"+210bps<sup>1</sup>"`. Any other tag (`<a>`, `<color>`, `<code>`, `<br>`, `<span>`, …) is passed through to the text run and **prints literally on the slide**; `validate_input` / `validate --fit-report` now report it as `UNSUPPORTED_INLINE_MARKUP` (`action: review`) naming the tag and the JSON path. `get_capabilities().features.supports_inline_markup` is the authoritative list.

- **Sub-bullets:** indent a bullet with a leading tab (or two spaces) per level — `["Revenue grew 18%", "\tEnterprise added $12.8M", "\tSMB gave back $0.7M"]`. The indent sets the paragraph's OOXML level, so the sub-bullet gets the template's smaller size and secondary glyph rather than the parent's, and the whitespace never reaches the slide. Levels are clamped at 4, and nesting past two levels reports `BULLET_NESTING_DEEP` (advisory) because deeper hierarchies stop reading on a projected slide. Works in `bullets_value`, `body_and_bullets_value` and `bullet_groups_value` (whose bullets already start one level under their header).
- **Ordered lists:** write them as `bullets_value: ["1. Freeze the schema", "2. Replay the log", "3. Cut over"]` — numbered from 1 with no gaps. The engine then draws the numbers itself (OOXML auto-numbering) and removes your typed prefixes, so the slide shows ONE marker. A list that is only partly numbered, or starts elsewhere, keeps your text verbatim and prints the number beside the layout's bullet glyph (`• 1. …`); that case reports `NUMBERED_LIST_NOT_APPLIED` with a `renumber_bullets` fix (go-slide-creator-6or2). A single line that merely opens with a number ("2024. A big year") is prose and is left alone.

- **Silent traps (Rules 17–19):** `footer` is an object not a string; content fields need `_value` suffix (`chart_value`, `table_value`).
- **One source convention.** Both source surfaces — slide-level `source` and a pattern's own `values.source` (`chart-insights-split`, `stat-hero`) — render identically: the label `Source: ` added when you did not write it, italic, left-aligned, at the engine's 12pt text floor, in the template's `dk2`. Writing `"Source: …"` yourself is harmless (the prefix is not doubled) and either field may be used; a pattern that draws its own source suppresses the slide band so the line is never printed twice.
- **Table density (Rule 20 — enforced):** MUST split if rows > 7 OR cols > 6 OR font < 9pt. Multiline cells count as N logical rows.
- **No emoji codepoints (hard rule):** emoji glyphs are rejected by pattern validators in `card-grid`, `icon-row`, `herodetail`, etc. Use a bundled SVG icon name or supply a user icon via `path` / `url` / `svg_data`. The monochrome symbols `✓ ✔ ✗ ✘ ★ ☆ ☐ ☑ ☒ © ® ™` are permitted in text (including table cells). See the [Icon Names](#icon-names) section.
- **Anti-patterns:** two-tables-one-grid, hex-fill mix, pattern monotony (no 3-in-a-row), accent monotony, sparse single-row flow (no full-slide `process-flow`/`timeline-horizontal` for 3-6 short labels — see the Sparse-sequence rule).
- **Cell accent variety:** `cell_accent_mode` ∈ {`uniform`, `alternate`, `progressive`} — use `progressive` for 4+ peer cells.

Deterministic geometry findings run on every shape_grid and pattern-expanded grid at validate / preview time: `TEXT_EXCEEDS_SHAPE` (a word wider than the shape's geometry text area — typical for chevron labels; `fix.kind: widen_shape_text_area`, advisory patch; `action: review` below 2× overflow, `shrink_or_split` at least 2×). If `available_pt < minimum_glyph_pt`, shortening cannot work: lower a pointed pattern's `max_height_pct`, change its step type, or widen/change raw-grid geometry. `SPARSE_FILL` (filled shape >10% of the slide with text filling <20% of it) and `SLIDE_UNDERUSED` (grid content covers too little of the safe area — 45% for restrictive authored `bounds` / `max_height_pct`, 22% for content-sized patterns or uncapped grids; `fix.params.band_capped_by` is `author`, `pattern`, or `none`, and only `author` has a cap to loosen) use `fix.kind: add_detail_or_resize` and `action: review`. When a pointed shape already lacks text width, a taller band worsens it; do not raise its cap to address `SLIDE_UNDERUSED`.

For the full finding-code catalog (`fit_overflow`, `cell_underfilled`, `placeholder_overflow`, `chart.*` family, render-time codes like `contrast_autofixed`, `text_trimmed`, `diagram_clamped`, etc.) and the `fix.kind` enums, see [FINDINGS.md](FINDINGS.md).

---

## Color Roles

Each template entry from `list_templates` carries its real canvas: `aspect_ratio` (`4:3`, `16:10`, `16:9`, `21:9`, or a decimal `W.WWW:1` for anything else — computed from the slide size, not assumed) plus `slide_width_in` / `slide_height_in`. Size column counts and text lengths against those, not against an assumed widescreen canvas. Each template also exposes `color_roles` in `list_templates` (MCP) / `json2pptx skill-info` (CLI) output — use `primary_fill` / `secondary_fill` as header-fill candidates, `body_fill` + `body_text` for card bodies, and put normal-sized `#FFFFFF` text on an accent only if it appears in `white_text_safe_body` (4.5:1, also exposed as `white_text_safe`). `white_text_safe_large` (3:1) applies only to text at least 18pt, or 14pt when bold. Avoid `near_background_accents` for marks on the light canvas (less than 2:1 against `lt1`); automatic chart palettes skip them but explicit color choices remain yours. Some templates have no white-safe accent, so `primary_fill` may be a dark theme slot instead. For tints, use luminance modifiers: `{"color": "accent1", "lumMod": 20000, "lumOff": 80000}` (20% tint with `dk1` text).

**Template-authored accent guidance (`accent_usage_guide`).** Some templates include an `accent_usage_guide` map in their `list_templates` output. When present, it maps accent color names (e.g. `"accent1"`, `"accent3"`) to prose descriptions of each accent's intended role within that template's visual language. When `accent_usage_guide` is present, defer to the template's role descriptions over generic assumptions — do not assume any accent has a fixed semantic role (positive, negative, neutral, subtle, etc.) unless the guide says so. When absent, fall back to the existing `color_roles` `primary_fill`/`secondary_fill`/`body_fill` semantics above.

**Semantic palette metadata.** `list_templates` (MCP, `fields=full`) / `json2pptx skill-info` also surface the template's authored palette intent (omitted when the template ships no metadata block):

- `semantic_accents` — maps `positive` / `negative` / `neutral` to accent names; use these when a pattern field expects a `semantic_accent` so reds/greens follow the template's own conventions. Patterns whose fills carry fixed meaning already resolve through this map without being asked: `waterfall-bridge` takes its negative-delta fill from `semantic_accents.negative`, its positive-delta fill from `positive`, and its subtotal fill from `neutral`, falling back to `accent2` / the deck accent / `accent3` only on a template that declares none. An explicit `negative_accent` / `subtotal_accent` override still wins.
- `surface_tints` — maps `subtle` / `paper` / `elevated` / `inverse` surface roles to scheme color names for tinted card/panel backgrounds.
- `data_palette` — ordered scheme color names for chart series (matches what svggen uses), so multi-series charts stay on-brand.
- `metadata_version` and `sha256` — the metadata schema version and a stable content hash of the template file; use `sha256` as a cache key to detect when a template changed under a stable name.

**Getting the deck out of the server.** Every `generate_presentation` / `render_deck_spec` response carries a **`resource_link` content block** alongside the JSON: `json2pptx://deck/<file name>`, media type `application/vnd.openxmlformats-officedocument.presentationml.presentation`. Read it with `resources/read` to get the `.pptx` as a base64 blob whose sha256 equals the response's `content_hash`. Use it whenever `pptx_path` is a path you cannot reach — a containerised or remote server, or a sandbox whose file tools do not see the server's output directory. Four read-only resources also serve, cached by the host and free of per-call tokens, what would otherwise cost a tool call every session: `json2pptx://templates`, `json2pptx://patterns`, `json2pptx://schema/deckspec` and `json2pptx://skill` (this document).

**Slide backgrounds.** `background` takes `image` / `url` (+ `fit`), a solid `color` (a scheme name or hex — how a light deck gets its one dark statement, section break or pull-quote slide, without a full-bleed `shape_grid` cell fighting the layout's title placeholder), and `overlay` `{color, alpha}` — a scrim over the image so text on a photo stays legible (default `dk1` at `0.45`). Text contrast is enforced against whichever of those the audience actually sees: the slide's own `color`, or the scrim when it is opaque enough (alpha ≥ 0.35) to decide the text's background. An image with **no** scrim has no single colour to judge against, so validate reports `TEXT_OVER_IMAGE_UNVERIFIED` rather than guessing — add an overlay, move the text off the picture (`image-text-split`), or set `contrast_check: false` when you know the photo is uniform under the text.

**Bring your own template.** A template the server has not registered reaches the engine as `template_path`, not `template` — `template` takes a registered NAME and rejects a path. `template_path` is accepted by `examine_template`, `list_templates`, `validate_input`, `preview_presentation_plan`, `generate_presentation` (as `presentation.template_path`) and `render_deck_spec` (as a tool argument), and is mutually exclusive with `template` on each. It resolves against the call's `base_dir` (the server's CWD when omitted) and MUST stay inside it after `~`/`$VAR` expansion and symlink evaluation; a path escaping the root is refused with `INVALID_PATH` naming the `base_dir` it escaped. Call `get_started(task: "onboard-template")` for the vetting sequence — `examine_template(template_path, base_dir)` first, because a template missing a canonical family renders those slides on the blank canvas. Copying the `.pptx` into the server's templates directory (`get_capabilities(sections:["runtime"]).runtime.templates_dir`) registers it live with no restart, after which it is a normal `template` addressed by file name without `.pptx`.

**Canonical layout taxonomy.** Discovery output (compact and full) carries the same canonical taxonomy that `examine_template` reports, so you can plan against stable roles instead of raw layout names:

- `canonical_layout_ids` — canonical layout name → concrete layout ID (address layouts by intent). These names go in `layout_id`, NOT `slide_type`: a closing slide is `layout_id: "closing"`, and `slide_type` accepts only its own nine values (`title`, `content`, `section`, `two-column`, `blank`, `chart`, `diagram`, `image`, `comparison`). Passing a layout name as a slide_type is reported as `UNKNOWN_ENUM` with a `rename_field` fix onto `layout_id`.
- Each `layout_summaries[]` entry carries `canonical_type` (e.g. `"Title Slide"`, `"One Content"`, `"Section Divider"`) and per-placeholder `role` (`title`, `eyebrow`, `section_number`, `body`, `image`, `chart`, …) alongside `max_chars`. For a **title** placeholder `max_chars` is the measured capacity at the comfort size for the resolved font — the same threshold the measured title check flags against — not an area estimate.
- `canonical_coverage` — per content-bearing family (`title-slide`, `section-divider`, `one-content`, `qa-closing`): `{present, layouts[]}`. A family with `present: false` means decks needing that slide kind cannot resolve a native layout.
- `derivable_layouts` — `[{name, ready, missing[]}]` for higher-level layouts the engine can synthesise (e.g. `two-content`, `full-image`, grid patterns). When `ready: false`, `missing` names the absent prerequisite.
- Full mode (`fields=full`) adds, per layout, `canonical_type` / `canonical_family` / `canonical_confidence`, and per placeholder `role` / `role_confidence` / `font_size_pt` (the font-size evidence behind the font-aware `max_chars`).
- The full per-layout entries (`examine_template`'s `report.json` `layouts[]`, and discovery `fields=full` `layouts[]`) carry a `tags[]` array of structural/semantic hints. Two matter when placing section titles: `title-at-bottom` (the title slot sits in the lower half of the slide, under a decorative element) and `compact-title` (that bottom title slot only fits a short single-line title). The tag is the signal to read — it is derived from the layout's internal area estimate, not from the measured `max_chars` reported alongside it, so do not try to re-derive it from that number. When a Section Divider carries these, keep the section title to one short line; long titles overflow. (See the template guide for the full tag catalog.)
- `layouts[].content_zone` is the safe area, and it stops **above the footer**: the zone's `bottom_emu` is clamped to `profile_geometry.frame.footer_top_emu` minus 0.1in whenever the layout has a footer — including one it inherits from the master, which is the case its own placeholders say nothing about. Before this it could report a bottom 0.17in INTO the footer band, and content laid out to it landed under the footer text (go-slide-creator-p41d6). The engine's own grid bounds use the same line, so a full-height pattern keeps that clearance too.
- `examine_template` also returns the **template profile** the generator renders chrome from: top-level `profile` (`{template_hash, parser_version, role_bindings, diagnostics[]}`) and per-layout `layouts[].profile_geometry` — `footer_regions[]` (resolved `dt`/`ftr`/`sldNum` rects, layout else master) and `frame` (`content`, `takeaway_band`, `source_band`, `footer_top_emu`, `basis`, `fits`). The takeaway/source band spans the layout's body column and sits above the footer placeholders; body/chart/table placeholders shrink to stop above it. `fits: false` means a `takeaway`/`source` on that layout is skipped at render and preflight emits `chrome_band_no_fit` (`fix.kind: swap_layout`) — put takeaways on a layout where `fits` is true.
- **Readability policy (`viewing_mode`).** Top-level `viewing_mode`: `"present"` (default; projected — 12pt body/card text, 10pt captions such as KPI labels/deltas, 20pt titles) or `"read"` (on-screen/print, lower floors). Text that autofit shrinks below its floor — placeholder text at generate time, `shape_grid` cell text predicted by the fit report — emits `TEXT_BELOW_READABLE_MIN` (`fix.kind: reduce_text`, `fix.params.strategy: shorten|split`, `actual_pt`, `min_pt`). Shorten or split; don't switch to `read` to silence it for a projected deck.
- **Title fit is measured, not counted.** Titles are measured against the resolved title placeholder with the template's inherited title style (master font size, all-caps, line spacing — e.g. modern-template renders titles at 45pt all-caps). A long title that fits by shrinking is written at the reduced size; one that cannot fit even at the minimum autofit size emits `TITLE_OVERFLOW` (`fix.kind: shorten_title`, `fix.params.max_chars`). `validate`, the fit report and the `quality` score use the same measurement **and the same single code**: a title that only fits below ~80% of the template title size is flagged as `title_wraps` escalated to `shrink_or_split`, with a measured `fix.params.max_chars`. Treat both that and `TITLE_OVERFLOW` as must-fix: shorten to ≤ `max_chars`. Nothing else reports a title's length — there is no 60-character rule, and `HEADLINE_TOO_LONG` (12 words) only fires for a title whose placeholder cannot be measured. The `max_chars` reported for a title placeholder is the measured capacity of that box at the comfort size, so `len(title) <= max_chars` means "no title finding".

  **`shorten_title` will not mangle a headline.** It cuts at a word boundary (never mid-word, never mid-rune), drops a dangling colon or trailing function word, and accepts either `max_words` (what `HEADLINE_TOO_LONG` carries) or `max_length` (what this tool documents) — both are supplied so a directive replayed verbatim and a call you construct behave identically. It **refuses** with `code: "semantic_review_required"` when the cut would leave a fragment: fewer than 3 words, or half the headline or more removed. `HEADLINE_TOO_LONG` itself switches its `fix.kind` to `review` when the headline is more than twice the budget, because that is a rewrite, not a trim. Rewrite the headline as a shorter claim and move the detail into the body or `takeaway` — do not keep lowering the budget until it truncates.

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

**Per-cell conditional tint.** A cell may carry `{"content": "On track", "conditional": {"rule": "equals", "threshold": "On track", "fill": "accent3"}}`. Rules: `always` (or omit `rule` — the plain highlight form), `positive`, `negative`, `threshold`/`gte`, `lte`, `between` (`threshold: [0, 5]`), `equals`, `contains`. `threshold` takes a number, a string or a two-number array — whichever the rule compares. **The rule is evaluated against the cell's own content**, so a cell that does not satisfy it keeps the table's normal fill; numbers are read out of the text (`"+4%"`, `"(3.2)"`, `"EUR 1,186.4"`), and a percent sign is a unit rather than a scale (`"50%"` is 50). An unrecognised rule or a threshold the rule cannot use is an `INVALID_PARAMETER` error at `…conditional.rule` / `.threshold` carrying the allowed list and a `did_you_mean` (go-slide-creator-6hlu). See `../../docs/STYLE_DEFAULTS.md`.

**Engine table defaults.** With no style fields set, tables render a styled header (`accent1` fill, bold `lt1` text), right-align numeric columns detected from the data (header included), bold a `Total` / `Sum` / `Grand total` row with a top rule, and size rows to their content instead of stretching them to fill the placeholder. Explicit `header_background` (incl. `"none"`), `use_table_style: true`, a non-default `style_id`, `column_types`, or `column_alignments` override the corresponding default. See `../../docs/STYLE_DEFAULTS.md`.

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

- `chrome.page_numbers.format` supports `{current}` and `{total}` placeholders, and the box is sized from the WIDEST string the format can produce (both numbers at the deck's highest), so `"{current} / {total}"` renders on one line instead of stacking `2 /` over `10`. The box grows leftward from the slide's right edge, capped at 2in and never into the left footer's last inch; past that the page number's type shrinks (to 8pt) rather than wrapping. Default skip set is `["title", "closing"]`. `"title"` and `"closing"` match the canonical layout types (so section dividers are *not* skipped by default — they stay in the numbered body flow); any other value matches a structural layout tag, so e.g. adding `"section-header"` suppresses page numbers on section dividers too.
- `chrome.section_crumb: true` appends the current section title to the footer line on that section's **content** slides (`Confidential — Acme | Sept 2026 | Market context`). Section dividers keep the plain line — they already announce the section in display type — and cover / agenda / closing slides sit outside any section. It only resolves when the deck also sets `structure.sections[].title`; on a flat `slides[]` deck the flag is inert. The crumb goes through the same footer fitting as the rest of the line, so a long section title shrinks and then ellipsizes rather than overflowing.
- Chrome text contrast is enforced against the layout background the slide actually renders, color-map overrides included, so a footer is never drawn in the slide's own background color. When the inherited color fails WCAG AA the engine pins an explicit palette color and reports it as `contrast_autofixed` at `/slides/{i}/chrome` with `fix.params.source: "chrome"` (go-slide-creator-hln7). You do not request this and cannot turn it off per slide; `contrast_check: false` on a slide only governs its placeholder text.

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

**Permitted monochrome symbols.** These render in the template's own text font, in the run's colour, and are NOT rejected: `✓` `✔` `✗` `✘` `★` `☆` `☐` `☑` `☒` `©` `®` `™`. They are the answer for a competitor matrix or a scored table, where a bundled SVG icon is not an option — a table cell takes only strings. Do NOT reach for `√` / `×` as a substitute; they are maths operators and read as a typo. A variation selector after one of them (`✓️`) asks for the colour emoji glyph and is still rejected, as are the emoji-presentation lookalikes `✅` `❌` `⭐` (go-slide-creator-l38d).

Call `list_icons` (MCP) or run `json2pptx icons list` (CLI) for all available icons. **Search by business concept, not by glyph name:** `filter` first tries a substring match on the name, then resolves the query through a curated concept index — `strategy` → `target` / `chess`, `revenue` → `coin` / `cash`, `governance` → `scale`, `compliance` → `certificate` / `shield-check`, `risk` → `alert-triangle`, `milestone` → `flag`, `efficiency` → `gauge`, and ~150 more. Concept hits lead the results even when the substring also matched (so `risk` no longer buries `alert-triangle` behind every icon containing `asterisk`), `matched_via` reports `name` / `synonym` / `synonym+name`, and `concept_matches[]` names which concept produced each icon. Multi-word queries reach the concepts inside them (`cost reduction` → the cost icons). A query matching neither returns `concepts[]`, the full vocabulary, so the next call is informed rather than another guess. Use `"icon": {"name": "ICON_NAME", "fill": "#FFFFFF"}` inside a shape, or `"icon": {"name": "ICON_NAME"}` as a standalone cell. The `"fill"` color override also works with custom SVG icons specified via `"path"`: `"icon": {"path": "icons/custom.svg", "fill": "#FF6600"}`.

**Shrink an overlay icon (`scale`).** When an icon is overlaid on a shape (`shape.icon`, or a cell `icon` with text), the optional `"scale"` field (`0 < scale <= 1`) reduces its footprint relative to the shape — useful for narrow cards where the default top/left overlay icon crowds the text. Example: `"shape": {"fill": "accent1", "text": "Secure", "icon": {"name": "shield", "scale": 0.4}}`. The overlay default is `0.6`; out-of-range or unset values fall back to it. `scale` has no effect on standalone (text-free) icon cells.

**Preview a single icon (`preview_icon`).** Before committing a custom-SVG path/URL or a recolored bundled icon to a deck, call `preview_icon` with the same `IconInput` shape you'd drop into a shape grid cell. It returns `svg_data` (with `fill` applied for non-inline sources), `png_base64` (rasterized preview), `alt`, `source_kind` (`bundled` / `path` / `url` / `inline`), and `qualified_name` for bundled icons — no template load, no full generation cycle. Pass `base_dir` whenever `icon.path` is relative. The `fill` override is ignored for inline `svg_data` (a warning is returned). The CLI counterpart is `json2pptx preview-icon` (accepts `--name`/`--path`/`--url`/`--svg-data` flags or an `--icon` JSON file/`-`).

**Structured write manifests for file-writing CLI commands.** The CLI commands that write files to disk — `preview-icon` (`--out-svg`/`--out-png`), `preview-wireframe` (`--out`), and `preview-patterns` — accept a `--manifest` flag that emits a JSON success manifest to stdout instead of forcing agents to scrape stderr logs. The manifest shape is `{"success": true, "command": "<name>", "artifacts": [{"path", "kind", "bytes", "sha256"}], "warnings": [...]}` where `kind` is a short artifact label (`svg`, `png`, `pattern-preview`). For `preview-icon`/`preview-wireframe` the flag requires at least one `--out*` target; the PNG entry is omitted when rasterization produced no bytes. `generate` already returns this information via `--json-output-report` (`output_path` + `content_hash` + `warnings`; `--json-output` is the deprecated alias), so it needs no `--manifest` flag.

**Canonical identifier.** Each entry in the discovery response (`list_icons` MCP, `json2pptx icons list --json` CLI) exposes a `qualified_name` field in `<set>:<name>` form (e.g. `"outline:chart-pie"`, `"filled:chart-pie"`). Use `qualified_name` directly as `icon.name` in deck JSON. This is required for filled icons — a bare `"chart-pie"` resolves to the outline set; you must write `"filled:chart-pie"`. Outline icons accept either the bare name or the `outline:` prefix. The legacy `names[]` array (bare names) is kept for backward compatibility but does not disambiguate sets.

**Bundled name preflight.** `validate_input` and `generate_presentation` preflight every `icon.name` against the bundled registry. Unknown names emit `ICON_BUNDLED_NAME_UNKNOWN` (severity: error) with `details.suggestions` — for a name that reads as a business concept, the icons that concept maps to (`strategy` → `target`, `chess`, `map-2`); otherwise a ranked list of Levenshtein-closest matches (or qualified cross-set forms when the bare base name only resolves in the non-default set). Concept resolution comes first because edit distance over 5,000 glyph names cannot recover intent — it used to answer `strategy` with `karate` and `revenue` with `venus`. Use `suggestions[0]` to repair the name without a separate `list_icons` round-trip.

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

**KPI icon placement and value fit.** `kpi-2up`…`kpi-6up` default the icon to `left` on landscape cards and `top` on square/narrow cards (explicit `position` wins), size the icon as an accent (≈ a quarter of the card height; an explicit `scale` wins), and shrink `big` uniformly (floor 16pt) so values like `$4.2M` / `12 days` never break across lines — you do not need to lower `big_size` by hand to avoid wraps.

**Accent on icon fill.** Prefer semantic theme colors (`accent1`–`accent6`, `dk1`, `lt1`) for `fill` so the icon adapts to the template's palette. Hex (`#RRGGBB`) is allowed only when the surrounding slide is already on a hex-allowlisted brand palette (see Rule 12 in RULES.md). Do not mix semantic and hex fills on one slide. At generation time scheme names (incl. `tx1`/`bg1` aliases) are resolved to the template's hex before being written into the SVG; a `fill` that is neither a scheme name nor hex is ignored (the icon keeps its default colour). `preview_icon` loads no template, so it only honours hex fills and warns on scheme names. Embedded icons also ship a rasterized PNG fallback for viewers that ignore SVG. When you omit `fill` on a pattern icon (kpi-*, icon-row, card-grid, hero-detail, matrix-2x2), the pattern picks a colour that contrasts with the card (≥ 3:1): `lt1` on solid accent cards, the accent on light cards — so leave `fill` unset unless you need a specific colour.

---

## Reference

For complete field specifications (connectors, accent bars, callout geometries, speaker notes, footers, backgrounds, theme overrides, patch operations, all chart/diagram types, and more), see `../template-deck/TEMPLATE_GUIDE.md` or run `json2pptx validate-template <path>`. Row `connector`s only join **visible** cells (a fill or outline — unboxed text and spacers are skipped) and fan out from a `row_span` parent to each child with elbow routing.
