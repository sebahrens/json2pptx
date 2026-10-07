# Layout catalog: every kind, pattern, chart and diagram, and how to see it

What the engine can draw, in one place, so a visual is chosen from the full
menu and not from the three an agent remembers. The live source is always the
product (`list_slide_kinds`, `list_patterns`, `show_pattern`,
`list_templates fields:"full"`); this page is the map of it. Back to the
[hub](README.md); the message-first index is
[visual-vocabulary.md](visual-vocabulary.md), the slide-level recipes are in
[split-and-complex-layouts.md](split-and-complex-layouts.md), and the segment
rules are in [slide-estate-and-segments.md](slide-estate-and-segments.md).

## 1. How to explore

| You want | Call | Returns |
|---|---|---|
| The kind catalogue, one line each | `list_slide_kinds()` | 29 kinds with `summary`, `required_fields`, `typical_fields` |
| Fields, budgets, forms, a copy-ready example | `list_slide_kinds(kinds:["decision"], fields:["example","budgets","compositions"])` | `budgets[] {field, max_chars, max_items, basis}`, `compositions[] {pattern, layout, reason}`, `example` |
| How a kind renders, before authoring | `list_slide_kinds(kinds:["cycle","pillars"], preview:true, template:"…")` | up to 4 kinds as images (`previews[].image_content_index`); the `cycle` kind returns one image per style |
| Candidates for a sentence you want to prove | `recommend_visual(intent, template, preview:true)` | ranked `candidates[]`, each with a `data_contract` and a runnable `next_tool_call`; `deckspec` form when a kind exists, raw `raw_json2pptx` slide otherwise; `candidates:["compose:a+b"]` previews a split |
| The pattern catalogue (58) | `list_patterns(fields:"full")` | `groups[]` by category; `use_when`, `cells`, `composes_with`, `role_on_slide`, `density_class` |
| One pattern's contract | `show_pattern(name)` | `schema` (values / overrides / cell_overrides), `text_budget_guide`, `example_values`, `use_when` / `not_when`, `composes_with` |
| Chart and diagram types with their data shapes | `list_templates(template, fields:"full")` | `chart_capabilities[]`, `diagram_capabilities[]` (the CLI counterpart is `skill-info`) |
| A template's layouts as pictures | `list_templates(template, fields:"full")` → `layout_summaries[]`; thumbnails under `templates/previews/<template>/<layoutID>.png` | 320px PNG per layout, committed |
| A gallery of every pattern on every template | CLI `json2pptx preview-patterns -templates-dir templates -output assets/pattern-previews` | local PNGs (gitignored); the MCP never returns pattern previews as paths |

`show_pattern`, `list_slide_kinds`, `recommend_visual` and `plan_deck` are in
the default `deckspec` profile; `list_patterns`, `expand_pattern`,
`explain_deck_spec` and `list_deck_archetypes` are hidden but callable by
name (`get_started(tool:"explain_deck_spec")` returns the schema).

## 2. The 29 DeckSpec kinds

Every kind takes `title`, `takeaway`, `source`, `notes`, `id`, and — where a
row names alternatives — the composition overrides `pattern:` / `layout:`
(one of `list_slide_kinds fields:["compositions"]`; anything else is
`SEMANTIC_PATTERN_NOT_AVAILABLE`). "Degrades to" is what the compiler emits
when the content leaves the visual's bounds, always with a finding at the
field that caused it.

| Kind | Draws | Forms the content selects | Degrades to |
|---|---|---|---|
| `title` | cover: `title` ≤42, `subtitle` ≤44, optional `eyebrow` kicker | — | — |
| `section` | numbered divider (01, 02 …); `appendix: true` or a title like Appendix / Backup / Q&A is unnumbered and uncounted | — | — |
| `agenda` | the contents page: `sections` (2–10, strings or `{title, subtitle}`), `current` (1-based or title) bolds the live section and dims the rest | a subtitle over 120 chars → `agenda-with-images` rows (3–6) | numbered bullets |
| `executive_summary` | 3–5 `points[] {lead, support}` over a `bottom_line` band (`exec-summary`) | — | bullets |
| `kpi_snapshot` | 2–6 `kpis[] {value ≤12, label, delta, comparator ≤24}` (`kpi-Nup`) | count picks `kpi-2up` … `kpi-6up`; values hold 11 digits at 5, 9 at 6 | bullets |
| `stat` | one `value` ≤20 with `label`, `context` (`stat-hero`) | — | content slide |
| `chart_insight` | `chart {type, title, data}` beside 1–6 `insights[]` and / or one `insight` so-what callout (`chart-insights-split`, 65/35; 75/25 when the text is sparse) | `layout: two-column` for a native chart and a longer list | — |
| `bridge` | 3–10 `columns[] {label ≤40, type: total \| delta \| subtotal, value}`; `unit` ≤8, `caption` ≤60 (`waterfall-bridge`) | a subtotal may omit `value` | bullets |
| `table` | `headers` (≤6) × `rows` (≤9 data rows; 10 logical with the header), `column_alignments`, `column_types`, `highlight_column`, `totals_row` | — | `table_rows_truncated` with `split_at_row`, never a silent cut |
| `comparison` | `columns[] {header, items[]}` | 2 balanced columns → `comparison-2col` (`connectors`, `highlight_column`, `highlight_row`); 3–5 → `stylish-panels`; 6–12 → `card-grid` | bullets |
| `option_matrix` | 2–6 `criteria` (`{label, scale: harvey \| rag \| text}`) × 2–6 `options[] {name, detail ≤80, scores[]}`, `recommended` (one or a list), `decisive_criterion`, `highlight_label`, `corner_label` (`table-highlight`) | — | scored bullets |
| `decision` | `options` + `recommendation` band | 3–6 → numbered boxes (`numbered-step-strip`); exactly 2 or 7–12, each with a `detail` → cards (`card-grid`) | recommendation lead-in over bullets |
| `matrix_2x2` | `x_axis` ≤16, `y_axis` ≤60, `quadrants` (4, clockwise from top left, or `top_left` … by position), `x_low` / `x_high` / `y_low` / `y_high` ≤11, one `highlight` | — | bullets naming each quadrant |
| `risk_heatmap` | 1–20 `items[] {name ≤40, likelihood, impact}` on 3 × 3 or `size: 5`; `likelihood_levels` / `impact_levels` relabel the axes | — | bullets |
| `framework` | `framework: swot \| porters_five_forces \| bmc` with `sections` keyed by part (swot: strengths, weaknesses, opportunities, threats; five forces: rivalry, new_entrants, substitutes, buyer_power, supplier_power; bmc: the nine canvas blocks) | bmc → `bmc-canvas`; swot / five forces → native diagram | grouped bullets when a part is missing |
| `architecture` | 3–6 `tiers[] {name, items[] ≤12}` + 0–3 `rails` (`arch-stack`) | — | bullets |
| `pillars` | 3–5 `pillars[] {title, body[]}` | alone → `stylish-panels`; with `objective` + `foundation` (string, or 1–3 levels, each a band or a row of cells) → `strategy-house` with `beam` and `roof_badges` | bullets |
| `cycle` | `phases` (+ `center`, `highlight`, `intake`, `left_label` / `right_label`, `left_count`) | `style`: `ring` 4–8 (default), `nodes` 3–8, `intake` 1–3 + 3–8, `figure_eight` 4–8 (full width only), `radial` 4–8 (`center` required), `concentric` 3–5 | numbered list |
| `process` | `steps` (strings or `{label, description, type}`) | with descriptions → numbered rows (3–6, `numbered-step-strip`); bare labels → flow (3–8, `process-flow`; 7–8 bend onto two rows); `type: decision` steps → diamonds in the flow; short labels → `process-flow-compact` band | bullets |
| `timeline` | 3–7 `milestones[] {label ≤60, date ≤30, end_date, body ≤200}` (`timeline-horizontal`) | an `end_date` on any stop turns the line into range bars | dated bullets |
| `roadmap` | 3–6 `phases[] {name, date_label, description \| items, milestone, active}` + 0–4 `parallel_tracks`, `parallel_label`, `current_phase` (`phase-roadmap`) | — | bullets |
| `org` | `nodes[] {id, name, title, parent}`; one root, ≤7 nodes, ≤3 levels (`org_chart`) | — | indented list |
| `team` | 1–8 `members[] {name, role, bio, photo \| photo_label}` (`team-bios`) | — | bullets |
| `quote` | `quotes[] {text, name, role}`; or the one-quote shorthand `quote` + `attribution` + `role` | 1 → `pull-quote`; 3–8 → `quote-cluster` | bullets (2 quotes always do) |
| `image_case` | `image {path \| url, alt, fit}`, `callouts[] {label, x, y}`, `eyebrow`, `heading`, `body`, ≤5 `bullets`, ≤3 `metrics`, `caption`, `image_width_pct` 30–60, `image_side` (`image-text-split`) | — | content slide |
| `next_steps` | 2–6 `actions[] {action ≤90, owner ≤30, date ≤20}` + 0–3 `decisions` (≤120), `decisions_label` (`next-steps`) | owner / date columns drop when no action has one | bullets |
| `closing` | the template's closing layout: short title + subtitle, or a few bullets | — | — |
| `regions` | one title over 2–3 typed regions (`chart`, `stat`, `kpis`, `table`, `timeline`, `image`, `text`, `cycle`), `arrangement` one of six | see [slide-estate-and-segments.md](slide-estate-and-segments.md) | — |
| `raw_json2pptx` | a raw slide verbatim: a `pattern`, a `compose` envelope or a `shape_grid` | the escape hatch for the 22 raw-only patterns and the svggen / native diagrams | — |

Three kinds that are easy to miss and change a deck's quality: `agenda`
(a real contents page with the current section marked — use it on 12+ slide
decks instead of hand-numbered bullets), `decision` with two detailed options
(a pair of cards is the cleanest "A or B" slide), and `process` steps with
`type: decision` (the one branching flow the engine draws).

```yaml
meta:
  title: Catalog examples
  template: midnight-blue
  date: October 2026
  source: Illustrative
slides:
  - kind: agenda
    title: Three questions, one decision
    current: 2
    sections:
      - {title: Where the cost went, subtitle: "The EUR 7.0M EBITDA walk, 2023 to 2025"}
      - {title: What holds and what does not, subtitle: "Price durability, volume, raw materials"}
      - {title: The bid we recommend, subtitle: "Scope, price and the conditions"}
  - kind: decision
    title: Co-delivery is the only option that transfers capability inside the deadline
    options:
      - label: Advisory only
        detail: EUR 0.9M. Bank staff deliver; the regulator's June 2027 date is at risk if hiring slips.
      - label: Co-delivery
        detail: EUR 2.1M. Joint team from day one; 16 FTE hired into the second line by March 2027.
        recommended: true
    recommendation: Approve co-delivery at EUR 2.1M and the hiring plan behind it.
  - kind: process
    title: Every change passes one risk gate before release
    steps:
      - Request
      - Impact assessment
      - {label: "Risk above appetite?", type: decision}
      - Approve
      - Release
      - Post-release check
    takeaway: The gate is the only step that can send a change back.
  - kind: pillars
    title: Four pillars take risk governance from "needs improvement" to effective
    objective: Risk governance rated effective by June 2027
    beam: One integrated risk report to the board, monthly
    pillars:
      - {title: Appetite, body: [Cascaded to BU limits, KRIs with owners]}
      - {title: Controls, body: [Independent testing, Quarterly attestation]}
      - {title: Second line, body: [44 FTE, Challenge recorded]}
      - {title: Data, body: [One loss database, Incident taxonomy]}
    foundation:
      - 1,200 control owners trained by June 2027
      - [People, Platform, Policies]
    takeaway: The beam is the deliverable the board sees every month.
```

Decks of six slides or more need an `executive_summary`; these catalog
snippets stay short on purpose.

```yaml
meta:
  title: Catalog examples (continued)
  template: midnight-blue
  date: October 2026
  source: Illustrative
slides:
  - kind: bridge
    title: EUR 6.5M of the EUR 7.0M EBITDA gain since 2023 is price
    unit: EUR M
    caption: 2023 to 2025
    columns:
      - {label: EBITDA 2023, type: total, value: 24.0}
      - {label: Price, type: delta, value: 6.5}
      - {label: Volume, type: delta, value: 3.2}
      - {label: Raw material, type: delta, value: -4.1}
      - {label: Opex, type: delta, value: 1.4}
      - {label: EBITDA 2025, type: total, value: 31.0}
    takeaway: If half the price gain unwinds, run-rate EBITDA falls by about EUR 3M.
  - kind: chart_insight
    title: Compute is 48% of run cost, so FinOps guardrails protect the saving
    chart:
      type: donut
      title: Run cost by component (%)
      data:
        categories: [Compute, Storage, Licences, Support, Network]
        values: [48, 21, 17, 9, 5]
        highlight: [Compute]
    insight: Every 10% of idle compute removed is EUR 150k a year.
    insights:
      - Compute scales with the load window; the 1.5 h target halves it
      - Licences are fixed until the 2028 renewal
  - kind: timeline
    title: Two of three clearers migrated; the third has eleven months
    milestones:
      - {label: Mandate published, date: Mar 2024}
      - {label: Wave 1 live, date: Jun 2025, body: Two clearers migrated}
      - {label: Wave 2, date: Nov 2025, end_date: Jun 2026, body: Third clearer and the custody book}
      - {label: Deadline, date: May 2027}
```

## 3. The 58 patterns, by family

"Via" names the kind that compiles to the pattern; **raw** means the pattern
has no kind and is authored as a `raw_json2pptx` slide with the block from
`show_pattern(name)` (`recommend_visual` returns such a slide ready to run).
Counts and budgets live in the pattern's `schema` and `text_budget_guide`;
the one-line descriptions with every override are in the repository's
`CLAUDE.md` table.

**Opening, summary and closing**

| Pattern | Via | Draws |
|---|---|---|
| `exec-summary` | `executive_summary` | 3–5 bold lead-ins, each with one supporting sentence, rules between, optional bottom-line band (`overrides.takeaway_emphasis: bar \| band`) |
| `scqa-summary` | raw | Situation / Complication / Question / Answer as four tab-labelled rows, the Answer emphasised |
| `agenda`, `agenda-with-images` | `agenda` | numbered contents list; rows with an image slot per section |
| `text-sidebar` | raw | foreword page: 1–4 paragraphs + ≤6 bullets beside a tinted or accent sidebar carrying one key message (`sidebar_side`, `sidebar_style`, `sidebar_width_pct`) |
| `next-steps` | `next_steps` | numbered action rows (action / owner / date) over a "Decisions requested" band |

**Numbers**

| Pattern | Via | Draws |
|---|---|---|
| `kpi-2up` … `kpi-6up` | `kpi_snapshot` | big-number cards with caption and optional `comparator`; an open row alone on a slide grows its dividers to half the area |
| `kpi-inline` | raw, **segment only** | one content-sized KPI bar; alone on a slide it reports `SLIDE_UNDERUSED` |
| `stat-hero` | `stat` | one oversized number, label, context |
| `hero-detail` | raw | one dominant metric (`hero {value, label}`) over 2–4 `details[] {title, body}` cards |
| `metric-list` | raw | vertical "by the numbers": 3–7 `items[] {label, value, detail}`, right-aligned accent values, one `highlight` row, takeaway band |
| `chart-insights-split` | `chart_insight` | chart panel + insights column with optional headline number and so-what |
| `horizontal-bar-with-callouts` | raw | 3–8 ranked `bars[] {label, value, callout}` with a per-bar insight callout column (`unit`, `max_value`); the callout column drops when none is given |
| `waterfall-bridge` | `bridge` | floating delta bars with computed subtotals |

**Comparison and decision**

| Pattern | Via | Draws |
|---|---|---|
| `comparison-2col` | `comparison` (2 cols) | aligned rows under optional headers; `connectors`, `highlight_column` / `highlight_row`, `overrides.style: tiles` |
| `before-after` | raw | two states with a transition chevron: `before {header, items[]}`, `after {…}`; `overrides.style: panels`, `overrides.emphasis` |
| `before-after-compact` | raw, **segment only** | the same for brief lists, content-sized |
| `stylish-panels` | `comparison` (3–5) / `pillars` | titled columns with an accent rule and open bullets; one `highlight` column; `overrides.style: ribbon` |
| `card-grid` | `comparison` (6–12) / `decision` (2, 7–12) | 2–12 titled cards; a short last row stays short (7 = 4 + 3); `overrides.style: filled` |
| `table-highlight` | `option_matrix` | criteria × options with Harvey balls, RAG dots or text, recommended row, decisive column, legend |
| `matrix-2x2` | `matrix_2x2` | four tinted quadrant fields with dark axis bars; `overrides.style: open \| tiles` |
| `capability-heatmap` | raw | 3–8 function columns of 1–6 activity cells filled by rating tier (2–4 tiers) with a legend; `overrides.header_fill: accent` |
| `risk-heatmap` | `risk_heatmap` | likelihood × impact grid, risks placed by band |

**Structure and frameworks**

| Pattern | Via | Draws |
|---|---|---|
| `arch-stack` | `architecture` | 3–6 tier lanes with a pentagon label tab, one block per component, side rails |
| `strategy-house` | `pillars` (+ objective + foundation) | roof, optional beam, 3–5 pillars, 1–3 foundation levels |
| `pyramid` | raw | 3–5 stacked trapezoid tiers (`cell_accent_mode` per tier) |
| `framework-grid` | raw | 2–6 dimension rows (`rows[] {label, cards[] {title, body}}`), a pentagon tab pointing into a pale band of 1–4 cards; one `highlight` row; `overrides.style: open \| tiles` |
| `bmc-canvas` | `framework: bmc` | the nine-cell Business Model Canvas |
| `labeled-rows` | raw | 2–6 rows of a keyword tab (WHAT / WHY / HOW; `label_style: tab \| tinted \| filled \| text`) beside 1–4 lines of body |
| `driver-tree` | raw | `root {label, unit}` → 2–4 `branches[] {label, unit, leaves[], annotation}` with elbow connectors |
| `value-chain` | raw | 4–10 step columns with per-step descriptions and one highlight; grows when alone |
| `concentric-rings` | `cycle` (`concentric`) | 3–5 nested rings with a side label ladder |

**Sequence and time**

| Pattern | Via | Draws |
|---|---|---|
| `process-flow` | `process` (bare labels / `type: decision`) | interlocking chevrons with numerals, diamonds for decisions, one solid step; `overrides.style: tinted \| solid`, `overrides.rows` |
| `process-flow-compact` | `process` (short labels) / raw, **segment only** | one content-sized band of arrows |
| `numbered-step-strip` | `process` (with descriptions) / `decision` (3–6) | `chevron` / `stacked-box` / `toc` styles, per-step detail, optional `icon` |
| `process-grid-2row` | raw | two parallel lanes (`lanes[]`) of 3–6 phase chevrons with `column_headers` and `outcomes` |
| `swimlane` | raw | actor lanes as pale bands, steps as pentagons in time order, arrows only on lane changes, `highlight: [lane, step]` |
| `timeline-horizontal` | `timeline` | dots, chevron or `gantt` style on a labelled axis |
| `phase-roadmap` | `roadmap` | one band of phase chevrons over date / description panels, milestones, parallel tracks |
| `roadmap-phased` | raw | workstream rows with `bars` from `start` to `end` on a shared time axis, lanes for overlaps, `milestone` markers, `current_phase` |
| `journey-maturity-model` | raw | ascending staircase of 3–6 stages in a tonal ladder, the `current` stage solid with a "We are here" pointer |

**Loops and hubs**

| Pattern | Via | Draws |
|---|---|---|
| `cycle-ring` | `cycle` (`ring`) | 4–8 ring segments with numbered badges, labels outside; under about 450pt of width a numbered legend beside the ring |
| `cycle-nodes` | `cycle` (`nodes`) | 3–8 circles joined by curved arrows; `overrides.labels: legend \| inside`, `direction`, `arrows: none` |
| `cycle-intake` | `cycle` (`intake`) | 1–3 intake arrows feeding a loop that starts at 9 o'clock; stacks in a narrow area |
| `cycle-figure-eight` | `cycle` (`figure_eight`) | two coupled lobes; needs ≥580pt of width — refused in a side-by-side segment with `swap_pattern` → `cycle-ring` |
| `radial-hub` | `cycle` (`radial`) | solid hub + 4–8 spoke satellites; `overrides.labels: inside \| legend` |
| `state-shift-hub` | raw | central hub with 3–4 numbered today / future pairs on an arc |

**People, voice and pictures**

| Pattern | Via | Draws |
|---|---|---|
| `team-bios` | `team` | 1–8 member cards with photo or initials |
| `contact-directory` | raw | 1–4 groups of up to 24 people in rows of 3–5, headshot or initials disc + name + title |
| `dual-org-ladder` | raw | two org columns of 2–4 paired roles with a centre arrow (joint venture, engagement team); `overrides.style: lines \| tiles` |
| `pull-quote` | `quote` (1) | italic quote with attribution and optional headshot |
| `quote-cluster` | `quote` (3–8) | 3-column grid of attributed quotes; `overrides.style: bubble \| tile` |
| `image-text-split` | `image_case` | photo cover-cropped beside eyebrow / heading / body / bullets / metrics |
| `icon-row` | raw | 3–5 icons over captions; `overrides.style: tile` |

Raw-only, for the record (22): `before-after`, `before-after-compact`,
`capability-heatmap`, `contact-directory`, `driver-tree`, `dual-org-ladder`,
`framework-grid`, `hero-detail`, `horizontal-bar-with-callouts`, `icon-row`,
`journey-maturity-model`, `kpi-inline`, `labeled-rows`, `metric-list`,
`process-grid-2row`, `pyramid`, `roadmap-phased`, `scqa-summary`,
`state-shift-hub`, `swimlane`, `text-sidebar`, `value-chain`.

A raw-only pattern inside a DeckSpec:

```yaml
meta:
  title: Vendor selection
  template: forest-green
  date: October 2026
  source: Atlas Retail RFP scoring, September 2026
slides:
  - kind: raw_json2pptx
    slide:
      slide_type: content
      layout_id: blank-title
      content:
        - placeholder_id: title
          type: text
          text_value: Vendor B leads on the two criteria that decide migration risk
      pattern:
        name: horizontal-bar-with-callouts
        values:
          unit: pts
          max_value: 100
          bars:
            - {label: Vendor B, value: 84, callout: "Strongest migration tooling; reference site live in 11 months"}
            - {label: Vendor A, value: 76, callout: "Lowest licence cost; weakest on skills availability"}
            - {label: Vendor C, value: 61, callout: "Best APIs; no regulated-industry reference in Europe"}
            - {label: Vendor D, value: 42, callout: "Lagging on security certifications"}
```

## 4. Charts (16) and diagrams (21)

Charts render as native SVG with a PNG fallback. Author them in a
`chart_insight`, a `chart` region, or a `diagram` compose segment, with
`{type, title, data}`:

| Comparison (Zelazny) | Types | Data shape |
|---|---|---|
| Time series, few periods | `bar`, `grouped_bar`, `stacked_bar` | `{categories, series[{name, values}]}`; `data.highlight` names the bar(s) or series the title argues (default: the last period); `data.orientation: horizontal` for ranked items |
| Time series, many points | `line`, `area`, `stacked_area` (`data.as_area` draws an area series as bars) | same; markers drop past 12 points |
| One measure, 2–6 groups, same periods | `small_multiples` | same; one panel per series on a shared y domain (`data.y_scale: independent` opts out) |
| Component of a whole | `pie`, `donut` (≤6 slices; `group_small_below_pct` folds the tail), `treemap` | `{categories, values}` |
| Correlation | `scatter`, `bubble` | `{points[{x, y, size, label}]}` |
| A walk between totals | `waterfall` | `{points[{label, value, type: total \| increase \| decrease \| subtotal}]}`; totals checked to 0.5% |
| Stages, funnels, gauges | `funnel` (`steps`), `gauge` (`bullet` style), `radar` | per type; see `chart_capabilities` |

Diagrams: svggen `timeline`, `venn`, `org_chart`, `gantt`, `matrix_2x2`,
`fishbone`; native OOXML `process_flow`, `pyramid`, `swot`,
`porters_five_forces`, `house_diagram`, `business_model_canvas`,
`value_chain`, `nine_box_talent`, `kpi_dashboard`
(`metrics[].good_direction`), `heatmap`, `pestel`, `panel_layout`,
`icon_columns`, `icon_rows`, `stat_cards`. All 21 render inside a compose
segment or a grid cell as well as alone. Where a kind or pattern draws the
same family (`framework`, `pillars`, `matrix_2x2`, `cycle`, `org`), prefer it:
it is validated field by field and patched by path.

## 5. Chrome, emphasis and reading scale

- `meta.accent_strategy`: `primary` (one accent throughout, the default),
  `rotate` (accent steps per slide), `section-keyed` (one accent per chapter).
  Within a slide, emphasis is always one thing: a `highlight` phase, bar,
  row, column or option; grid patterns take `cell_accent_mode: uniform |
  alternate | progressive`.
- `meta.viewing_mode`: `present` (spoken over, larger type, less on a page) or
  `read` (a pre-read or leave-behind; denser, complete sentences).
- `meta.chrome`: `confidentiality`, `client`, `project_code`, `date`, page
  numbers, `tracker: true` (section name above every content title),
  `section_crumb: true` (section in the footer). Page numbers, the date and
  the tracker are on by default; `CHROME_TRUNCATED` reports what a narrow
  footer dropped.
- `meta.archetype` (`board_update`, `qbr`, `sales_pitch`,
  `strategy_proposal`, `project_roadmap`, `market_analysis`) sets a default
  template and whether the rhythm checks expect a synthesis slide; see
  [deck-archetypes.md](deck-archetypes.md).
