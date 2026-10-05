# Pattern Library and Text Capacity

## Pattern Library

Named patterns expand into validated `shape_grid` structures. Choose from
the live `list_patterns` catalog (not enumerated here); `show_pattern` gives
a candidate's `use_when`, `not_when`, value schema, example and text budget.
`recommend_visual` ranks them against layouts, charts and diagrams. Use
a diagram, not a text grid, for data-driven or topologically complex graphics.

- **Browse:** `list_patterns` (MCP) or `json2pptx patterns list` (CLI).
- **Schema:** `show_pattern` (MCP) or `json2pptx patterns show <name>` (CLI) returns the value schema, `example_values` (use them as the template for `values`) and, for grid-shaped patterns, a `text_budget_guide` with per-configuration `body_max_chars` / `header_max_chars`.
- **Validate:** `validate_pattern` (MCP) or `json2pptx patterns validate <name> <values.json>` (CLI) checks values and fit. Pass `theme_template` (MCP) or `--template` + `--templates-dir` (CLI) for the intended template; CLI `--json` returns the findings envelope and exits non-zero on blocking findings.
- **See it:** `recommend_visual` with `preview: true` (TOOLS.md → Composition recipes).
- **Expand + density pre-flight:** `expand_pattern` (MCP) or `json2pptx patterns expand` (CLI) returns `cell_budgets[]` (for a sparse slide-level pattern they describe the composed block generation renders), `capacity_warnings[]`, `density_warnings` for embedded tables over the TDR ceilings (RULES.md Rule 20) and, when all populated cells are suboptimal, `layout_suggestions[]`. With `theme_template` the response's `bounds_source` is `"template"`, else `"default_fallback"`.
- **Per-cell styling:** `cell_overrides` (`{"<index>": {...}}`, index meaning in each pattern's schema) accepts `accent_bar`, `font_size`, `emphasis`, `align`, `vertical_align` and `color`. Text keys restyle every paragraph of that cell's primary text (so `font_size` flattens a header/body hierarchy) — e.g. `strategy-house` index 0 (objective banner), an `agenda` row's title, a `metric-list` item's big value. Use pattern-level `overrides` for deck-consistent sizes.

Apply at the slide level via the top-level `pattern` field (XOR with `shape_grid` — never both):

```json
{"layout_id": "blank-title", "pattern": {"name": "kpi-3up", "values": [
  {"big": "$127M", "small": "Revenue"}, {"big": "43%", "small": "Gross margin"}, {"big": "2.1x", "small": "YoY growth"}]}}
```

KPI `values` is a JSON **array**, one cell per metric: an object `{"big", "small"}` (aliases `value`/`number` → `big`, `label`/`caption` → `small`) or a pipe string `"$127M | Revenue"`; forms may be mixed. Give a KPI its reference: `comparator` (alias `vs`, ≤24 chars, e.g. `"vs plan +4 pts"`) renders under the caption on kpi-Nup cards (`kpi-inline` rejects it).

**Closing.** End on `next-steps` (actions with `owner` / `date`, `decisions` requested), not "Thank you". A repeated `agenda` with `overrides.highlight` marks the current section.

Do NOT hand-roll shape grids when a named pattern exists.

**Callouts.** Patterns with `supports_callout=true` accept an envelope-level `callout: {text, emphasis?, accent?}` — the takeaway band below the pattern (3pt accent bar, 14pt bold dk1 text, no box; budget two lines). `emphasis`: `bold` (default), `italic`/`bold-italic`, `subtle` or `strong` (solid accent). KPI patterns refuse callouts (`callout_unsupported`). exec-summary `bottom_line`, metric-list `callout` and chart-insights-split `so_what` render the same band.

**Accent is restrained by default.** Patterns spend at most one solid accent block, in the template's `color_roles.primary_fill` (the default accent). Structural cells are neutral tints with dark text; accent rules, connectors and numerals mark structure. Many patterns are open by default (text on the canvas between rules) and take an `overrides.style` that restores tiles or solid fills (`tiles`, `panels`, `solid`, `ribbon`, …): each pattern's `show_pattern` overrides schema lists its values. A process flow spends its one solid accent on `steps[].highlight` (at most one) or its single decision. Ask for an accent fill only where it is the slide's one emphasis.

**Chrome bands shrink row budgets.** A `takeaway` (and `source`) band takes 70–100pt off the content zone. `numbered-step-strip` stacked-box / toc rows are measured against what is left and report `BODY_TOO_LONG` when they cannot fit at 12pt: under a takeaway, keep them to labels only from five rows up, or drop the band. When every label and body is one short line, each row is `number | label | detail`.

### Copy budgets and what happens when text does not fit

**Per-pattern copy targets** (BMC cells, driver-tree leaves, comparison rows,
roadmap activities, team bios, metric details, swimlane steps, KPI captions,
…) live in each field's `show_pattern` description, measured at default sizes
for every supported count. Limits: `state-shift-hub` takes 3–4 pairs,
`dual-org-ladder` at most 4 rows.

Budgets assume the template's full content area. On a short or narrow
template, or under a `takeaway`, content-sized patterns (`exec-summary`,
`table-highlight`, `metric-list`, `team-bios`, `pull-quote`, …) first give up
air, headshot size or type down to the 12pt floor, then report
`BODY_TOO_LONG` naming what to drop. `icon-row`, `labeled-rows`,
`process-flow` and `process-flow-compact` grow their cards, rows or band to the
text's written fit before reporting it. `process-flow` lays 7–8 steps on
two rows of four (the second runs back right to left, its chevrons / arrows
mirrored to point left; all-chevron / arrow flows wrap left to right), so a
box holds 80 characters at every step count. `overrides.rows` is `1` or `2`
(default `2` from 7 steps; `2` needs at least 4 steps); `rows: 1` keeps one
row of narrow boxes. `type: "arrow"` steps are block arrows with the label
at the flow's type size.
`process-flow-compact` is one band and rejects `rows`. Validation predicts grid text
(nested cells included) that generation would refuse as an `error`
`TEXT_BELOW_READABLE_MIN` (DeckSpec render refusals: DECKSPEC.md).

KPI values ("$4.2M", "127%") and label words are never wrapped mid-token: a
value shrinks and a label column widens before its label does. A kpi-Nup
value is written at its fitted size, 16pt at the least; one too long for
16pt puts its row at 14pt and reports `BODY_TOO_LONG` (the message quotes
both sizes) at `/slides/N/pattern/values/i/big` with `fix.params.max_chars`.
A label word that cannot fit is `TEXT_EXCEEDS_SHAPE`: shorten or abbreviate. A
KPI value's 12 characters are the hard maximum: `kpi-5up` holds about 11
digits and `kpi-6up` about 9 on the narrowest templates.

**Placement.** Patterns follow the slide size: on a larger slide
(business-template, 14.7 × 8.3in) pattern text and row heights render about
10% larger (12 → 13pt, 14 → 15pt); authored `shape_grid` sizes are kept.
Box patterns are content-sized: `kpi-Nup` cards, `card-grid`
rows, `before-after` panels and `strategy-house` pillars hug their text (≤
1.6× its height). A slide's own pattern block that needs under 75% of the
content area is composed: its text steps up one type-scale step (12→14pt,
14→18pt, KPI figures up to 48pt; skipped if a short label would wrap)
and the block sits at the optical centre; a lone open `kpi-Nup` row also
grows its divider band to half the area. Do not add filler, spacer rows or
`bounds` to "fill" a sparse slide. `pattern.vertical_align` (default `auto`)
takes `top` (under the title, at the pattern's own sizes),
`center`, `bottom` or `stretch`; an explicit value, `bounds` and
`max_height_pct` are honoured as authored; `type_scale: "compact"` keeps the
sizes and only centres. Dense blocks, compose segments, nested cell
patterns and `regions` cells hang from the native body-text line. The
compact variants (`kpi-inline`, `before-after-compact`,
`process-flow-compact`) are for a compose segment or region cell; alone on
a slide they are composed like any sparse block.
`VERTICAL_IMBALANCE` / `HORIZONTAL_IMBALANCE` (25 points each) fire when 40%
or more of the content area stays empty below / beside a block (or between
it and its conclusion band): set `vertical_align` (`center` / `stretch`),
add a supporting zone (`compose`) or pick a pattern that fills the area. A
table counts as its rows and reports from a third empty beneath it: add
rows or pair it with a region. A
remaining `SLIDE_UNDERUSED` means the content is thin — add real detail,
pair it with a second zone, or merge slides. It is not raised for a
`stat-hero` or `pull-quote` at its designed size, nor for a normal
`next-steps` / `numbered-step-strip` list; a lone `process-flow-compact` or
`kpi-inline` reports, as does a KPI row (tiles) over an empty lower third.

---

## Text Capacity Awareness

`density_pct` is a **height** ratio: each paragraph wrapped at its own font size, line heights summed, compared with the cell's text height. `max_chars` is the derived character hint at the cell's dominant size. Target **35–110%**: under 35% is `underfilled` (info), over 110% `overflow` (warning: it clips or shrinks hard).

**Pick a configuration.** `show_pattern` returns `text_budget_guide.target_density` (a *character* target) and `configurations[]` (`columns`, `rows`, `body_max_chars` at 12pt, `header_max_chars` at 16pt). Pick the one whose `body_max_chars` is closest to `planned_chars / 0.85`, write to it, then check every `expand_pattern` `cell_budgets[]` entry's `density_pct` lands in 35–110%. Non-grid patterns (`pull-quote`, `stat-hero`) have no guide: use per-placeholder `max_chars` from `list_templates` `mode="compact"` or `fields="full"`.

- **Underfilled:** add supporting detail; for short content use a sparse pattern (`kpi-3up`, not `card-grid`) or a smaller configuration (2×2, not 3×2).
- **Overflow, or `fit_overflow` / `density_exceeded` after generation:** (1) rewrite shorter; (2) a larger configuration or pattern (`repair_slide` `swap_layout`, or `layout_suggestions[]`, offered when all populated cells are underfilled or all overflow); (3) `repair_slide` `split_at_row` / `reduce_text`; (4) last resort for verbatim content: `reduce_cell_text` (truncates one cell to `max_chars` with an ellipsis), then re-validate with `fit_report: true`.

**Bounds.** A sparse slide-level block is composed automatically (Placement above). To pin a region pass `max_height_pct` (1–99% of the content area) or `bounds` (percent rectangle; wins) to `expand_pattern` or as slide-level `pattern.bounds` / `pattern.max_height_pct`; density math then uses the reduced area and `bounds_assumption` reports `"explicit_override"`. An underfilled `capacity_warnings[]` entry and `density_class_divergence` (average density far below the pattern's `density_class`) carry a `next_tool_call` with a recommended `max_height_pct` or a sparser pattern.
