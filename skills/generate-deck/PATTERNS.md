# Pattern Library and Text Capacity

How to pick a named pattern, size content for its cells, and read the pre-flight density reports.

---

## Pattern Library

Named patterns expand into validated `shape_grid` structures. Choose from
the live `list_patterns` catalog; call `show_pattern` for a candidate's
`use_when`, `not_when`, value schema, example, and text budget. The catalog
changes with the registry, so this guide intentionally does not enumerate it.
Use `recommend_visual` when the visual family itself is undecided (it ranks
patterns, native layouts, charts, and diagrams; the folded `recommend_pattern`
alias is its pattern-only subset). Use a diagram for a data-driven or
topologically complex graphic rather than forcing it into a text grid.

- **Browse:** `list_patterns` (MCP) or `json2pptx patterns list` (CLI).
- **Schema:** `show_pattern` (MCP) or `json2pptx patterns show <name>` (CLI) returns the value schema, `example_values` (use them as the template for `values`) and, for grid-shaped patterns, a `text_budget_guide` with per-configuration `body_max_chars` / `header_max_chars`.
- **Validate:** `validate_pattern` (MCP) or `json2pptx patterns validate <name> <values.json>` (CLI) checks values and fit. Pass `theme_template` (MCP) or `--template` + `--templates-dir` (CLI) for the intended template; CLI `--json` returns the findings envelope and exits non-zero on blocking findings.
- **Expand + density pre-flight:** `expand_pattern` (MCP) or `json2pptx patterns expand` (CLI) returns `cell_budgets[]`, `capacity_warnings[]`, `density_warnings` for embedded tables over the TDR ceilings (RULES.md Rule 20) and, when all populated cells are suboptimal, `layout_suggestions[]`. With `theme_template` the response's `bounds_source` is `"template"`, else `"default_fallback"`.
- **Per-cell styling:** `cell_overrides` (`{"<index>": {...}}`, index meaning in each pattern's schema) accepts `accent_bar`, `font_size`, `emphasis`, `align`, `vertical_align` and `color`. Text keys restyle every paragraph of that cell's primary text (so `font_size` flattens a header/body hierarchy) — e.g. `strategy-house` index 0 (objective banner), an `agenda` row's title, a `metric-list` item's big value. Use pattern-level `overrides` for deck-consistent sizes.

Apply at the slide level via the top-level `pattern` field (XOR with `shape_grid` — never both):

```json
{"layout_id": "blank-title", "pattern": {"name": "kpi-3up", "values": [
  {"big": "$127M", "small": "Revenue"}, {"big": "43%", "small": "Gross margin"}, {"big": "2.1x", "small": "YoY growth"}]}}
```

KPI `values` is a JSON **array**, one cell per metric: an object `{"big", "small"}` (aliases `value`/`number` → `big`, `label`/`caption` → `small`) or a pipe string `"$127M | Revenue"`; forms may be mixed. Give a KPI its reference: `comparator` (alias `vs`, ≤24 chars, e.g. `"vs plan +4 pts"`) renders under the caption on kpi-Nup cards (`kpi-inline` rejects it).

**Closing.** End a consulting deck on `next-steps` (actions with `owner` and `date`, plus `decisions` requested), not a "Thank you" slide. A repeated `agenda` with `overrides.highlight` marks the current section.

Do NOT hand-roll shape grids when a named pattern exists; let the engine handle grid structure, bounds, and gap arithmetic.

**Callouts.** Patterns with `supports_callout=true` accept an envelope-level `callout: {text, emphasis?, accent?}` — the takeaway band below the pattern (flush 3pt accent bar, 14pt bold dk1 text, no box). `emphasis`: `bold` (default), `italic`/`bold-italic`, `subtle` (5% neutral tint) or `strong` (solid accent, measured-contrast text). Plain one-line text (budget two lines). KPI patterns refuse callouts (`callout_unsupported`). exec-summary `bottom_line`, metric-list `callout` and chart-insights-split `so_what` render the same band; their `overrides.takeaway_emphasis` takes `subtle` / `strong`.

**Accent is restrained by default.** Patterns spend at most one solid accent block, in the template's `color_roles.primary_fill` (the default accent). Structural cells are neutral tints with dark text; accent rules, connectors and numerals mark structure. `overrides.style: "solid"` restores accent fills on `roadmap-phased`, `process-flow[-compact]`, `kpi-inline`, `agenda-with-images`, `numbered-step-strip` (number lanes; not `values.style`) and `process-grid-2row`; likewise `labeled-rows` `label_style: "filled"`, `hero-detail` `style: "cards"`, `stylish-panels` `overrides.ribbon: "accent"`. A process flow spends its one solid accent on `steps[].highlight` (at most one) or its single decision. Ask for an accent fill only where it is the slide's one emphasis.

**Chrome bands shrink row budgets.** A `takeaway` (and `source`) band takes 70–100pt off the content zone. `numbered-step-strip` stacked-box / toc rows are measured against what is left and report `BODY_TOO_LONG` ("N stacked-box rows need …pt … the content area holds about …pt") when they cannot fit at 12pt: under a takeaway, keep stacked-box / toc to labels only from five rows up (four toc rows hold a body), or drop the band.

### Copy budgets and what happens when text does not fit

**Per-pattern copy targets** (BMC cells, driver-tree leaves, comparison rows,
roadmap activities, team bios, metric details, swimlane steps, KPI captions,
…) live in each field's `show_pattern` description, measured at default sizes
for every supported count. Limits: `state-shift-hub` takes 3–4 pairs,
`dual-org-ladder` at most 4 rows.

Budgets assume the template's full content area. On a short or narrow
template, or under a `takeaway`, the content-sized patterns
(`chart-insights-split`, `table-highlight`, `exec-summary`, `scqa-summary`,
`bmc-canvas`, `agenda-with-images`, `state-shift-hub`, `metric-list`,
`next-steps`, `matrix-2x2`, `process-grid-2row`, `stylish-panels`,
`team-bios`, `framework-grid`, `contact-directory`, `agenda`,
`before-after[-compact]`, `comparison-2col`, `hero-detail`,
`timeline-horizontal`, `pull-quote`) first give up air, headshot size or type
down to the 12pt floor (a `pull-quote` quote 36 → 20pt), then report
`BODY_TOO_LONG` naming what to drop. `icon-row`, `labeled-rows`,
`process-flow` and `process-flow-compact` grow their cards, rows or band to the
text's written fit before reporting it. Validation predicts grid text
(nested cells included) that generation would refuse as an `error`
`TEXT_BELOW_READABLE_MIN` (DeckSpec render refusals: DECKSPEC.md).

KPI values ("$4.2M", "127%") and label words are never wrapped mid-token: a
value shrinks (a `kpi-6up` "$4.2M" may render near 28pt); a
`process-grid-2row` row-label column widens up to 22% (the `scqa-summary`
label column up to 1.3 : 4) before its label shrinks. What cannot fit at the
floor is reported — `BODY_TOO_LONG` on `values[i].big`, `TEXT_EXCEEDS_SHAPE`
naming the label word: shorten or abbreviate it.

**Placement.** Box patterns are content-sized: `kpi-Nup` cards, `card-grid`
rows, `before-after` panels and `strategy-house` pillars hug their text (at
most 1.6× its height). Pattern blocks hang from the line native body text
starts on, not mid-slide; content under a top-anchored title starts below its
measured lines (tracked, bold or substituted title faces included).
`pattern.vertical_align` (default `auto`) takes `center` / `bottom` /
`stretch`. Leftover space below a short block is intended — do not pad copy.
`SLIDE_UNDERUSED` judges them at 20% ink (`phase-roadmap` /
`timeline-horizontal` 22%, others 29%); `VERTICAL_IMBALANCE` ignores the band
below a body-line block. When it fires, add a supporting zone (`compose`), a
`takeaway`, or merge slides.

---

## Text Capacity Awareness

`density_pct` is a **height** ratio: each paragraph wrapped at its own font size, line heights summed, compared with the cell's text height — computed from embedded font metrics (no OS dependency). `max_chars` is the derived character hint at the cell's dominant size. Target **35–110%**; patterns leave whitespace on purpose, so a well-authored cell fills about half its box.

| Band | Density % | Status | Severity | Meaning |
|------|-----------|--------|----------|---------|
| Underfilled | < 35% | `underfilled` | info | Content doesn't justify the space |
| Optimal | 35–110% | `optimal` | — | Fits with appropriate whitespace |
| Overflow | > 110% | `overflow` | warning | Will clip or need aggressive shrinking |

### Picking a grid configuration with `text_budget_guide`

`show_pattern` returns `text_budget_guide.target_density` (a *character* sizing target) and `text_budget_guide.configurations[]` with `columns`, `rows`, `body_max_chars` (at 12pt) and `header_max_chars` (at 16pt). Estimate characters per cell, choose the configuration whose `body_max_chars` is closest to `planned_chars / 0.85` (~85% density), write to that budget, then call `expand_pattern` and check every `cell_budgets[]` entry lands in 35–110%. Non-grid patterns (`pull-quote`, `stat-hero`) have no guide: use per-placeholder `max_chars` from `list_templates` with `mode="compact"` (or `fields="full"`; the default `fields="compact"` omits them).

`cell_budgets[]` entries carry `cell_index` (zero-based), `row`, `col`, `max_chars`, `actual_chars`, `density_pct`, `status` and `font_size_pt`. Rewrite before rendering — cheaper than repairing after generation. WORKFLOW.md's pre-emit check *"every cell at 35–110% density"* reads `density_pct`.

### Decision rules

- **Underfilled (< 35%):** add supporting detail; for inherently short content (a metric, a status) use a sparse pattern (`kpi-3up`, not `card-grid`); when most cells are underfilled, use a smaller configuration (2×2, not 3×2).
- **Overflow (> 110%), or `fit_overflow` / `density_exceeded` after generation:** (1) rewrite — shorter sentences, fewer bullets; (2) a larger configuration or a higher-capacity pattern (`repair_slide` `swap_layout`, or `layout_suggestions[]`); (3) `repair_slide` `split_at_row` / `reduce_text`; (4) `reduce_cell_text` (truncates one cell to `max_chars` with an ellipsis) only for verbatim content you must not rephrase, then re-validate with `fit_report: true`. `reduce_cell_text` is a last resort, never a substitute for the VARY loop.
- **`layout_suggestions[]`** appears only when **all** populated cells are underfilled or all overflow (alternative patterns + overrides + `reason`); mixed grids need manual adjustment.

### Bounds override: `bounds` and `max_height_pct`

For short content in tall cells, prefer a compact variant (`process-flow-compact`, `before-after-compact`, `kpi-inline`). Otherwise pass `max_height_pct` (1–99, share of the content area; same as `bounds: {x:0, y:0, width:100, height:<value>}`) or `bounds` (percent rectangle; wins over `max_height_pct`) to `expand_pattern` or as slide-level `pattern.bounds` / `pattern.max_height_pct`. Density math then uses the reduced area (no false `cell_underfilled`). `expand_pattern`'s `bounds_assumption` says which area the budgets used: `"full_content_area"` or `"explicit_override"`.

An underfilled `capacity_warnings[]` entry without explicit bounds carries a `next_tool_call` re-expanding with a recommended `max_height_pct` — follow it. `density_class_divergence` fires when average density is far below the pattern's `density_class` (<15% medium, <30% high); its `next_tool_call` suggests a compact variant or a `max_height_pct`.

### `recommend_visual` inputs

- `content_hints.density_hint` (`low` / `medium` / `high`): a matching `density_class` scores higher, a distant one lower.
- `candidates` (2–8 names) ranks an **explicit shortlist** instead of the catalog: every name returns `score`, `rationale` and `confidence_band`, bypassing the 0.5 cutoff, top-K, near-misses and diversity bonus; `category` is auto-resolved (placeholder layout / named pattern / chart / diagram / raw_shape_grid) and unknown names score 0 with a rationale.
