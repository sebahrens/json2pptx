# The slide estate: splitting it into segments

How one slide's area is divided, what may stand in each part, at which widths
a visual still draws as intended, and the order in which to reach for the
mechanisms. Every number below is the engine's (file references in the
margin notes of `internal/semantic/slides/regions.go` and
`cmd/json2pptx/compose.go`); the worked recipes are in
[split-and-complex-layouts.md](split-and-complex-layouts.md). Back to the
[hub](README.md).

**The rule before the mechanics.** A split is a layout for *one* message that
needs two or three views to be believed — the exhibit and the number it
produces, the walk and its implication, the picture and the fact in it. If
the second part has its own so-what, it is a second slide. The dominant
evidence takes 55–67% of the width; the narrow side carries one number and
two or three short lines, never a paragraph; one title, one `takeaway`, one
source line cover the whole.

## 1. The ladder: which mechanism, in which order

```
1. One kind draws the whole message?            → that kind (29 of them; layout-catalog.md)
2. The kind has a form you want instead?        → pattern: / layout: composition override
3. Two or three typed parts prove one title?    → kind: regions  (2–3 regions, 6 arrangements, depth 1)
4. A part must be a whole pattern or diagram?   → raw_json2pptx with compose (≤8 segments, depth 2, ≤12 leaves)
5. One block inside a cell of a hand-built grid?→ shape_grid cell pattern: / grid: / layers
6. A table that will not fit?                   → split it: (2/2) slide or appendix; never a smaller font
```

Stop at the first rung that works: each one down is validated less finely
and patched less precisely. `regions` is checked field by field and patched
by JSON Pointer; `compose` is checked per segment rectangle; a `shape_grid`
is checked as geometry.

## 2. `kind: regions` — the typed split

### Arrangements

| `arrangement` | Regions | Geometry | `size_pct` means |
|---|---|---|---|
| `columns` (default) | 2–3 | left to right | each region's width share |
| `rows` | 2–3 | top to bottom | each region's height share |
| `main_left` | exactly 3 | `regions[0]` fills the left column; `[1]` and `[2]` stack on the right | main: its width (default 60); the two others: their share of the right column's height |
| `main_right` | exactly 3 | mirror of `main_left` | as above |
| `main_top` | exactly 3 | `regions[0]` is a full-width band on top; `[1]` and `[2]` sit side by side under it | main: its height (default 60); the others: their width share |
| `main_bottom` | exactly 3 | mirror of `main_top` | as above |

There is no nesting: a region is a flat typed object and cannot hold
`regions`. Everything compiles to one `blank-title` shape grid under the
slide title.

### Shares

- `size_pct` is 15–85. Unset values share what the set ones leave, weighted
  by kind (`stat` 2, `chart` and `cycle` 4, everything else 3), so a chart
  beside a stat with nothing set lands near 67/33.
- All set: they must sum to 100 (±1). Set ones that leave an unset region
  under 15% are refused; both cases are `SEMANTIC_DENSITY` at
  `slides[N].regions` ("region sizes do not fit").
- In a **vertical** group (`rows`, the stack of `main_left` / `main_right`,
  the band of `main_top` / `main_bottom`) unset shares are raised to readable
  minimums: `stat` 30% (35 with `context`), `kpis` 30%, `timeline` 45%,
  `table` 15% × (rows + 1), `image` 25%, `text` 15%, `cycle` 60%, a
  decorated `chart` 45% and more with a title, several series or stacking;
  any region with a `heading` +10. The chart yields first (drawn short and
  reported `chart.plot_area_collapsed`); a squeezed `text`, `stat` or
  `timeline` is refused rather than shrunk. Leave stacked shares unset unless
  the render shows a reason.

### The eight region kinds and their contracts

| Region `kind` | Required | Also takes | Budget | Refused |
|---|---|---|---|---|
| `chart` | `chart {type, data}` | `heading` ≤60, `unit` ≤12, `alt`, `source` | any of the 16 chart types | a waterfall's totals off by >0.5% |
| `stat` | `value` ≤20 | `label` ≤80, `unit` ≤10, `context` ≤120 | one number | — |
| `kpis` | `kpis[]` 2–4 `{value, label, delta, comparator}` | `heading` | the `kpi-Nup` card fit of the column | one KPI ("one number is a stat region") |
| `table` | `headers`, ≥1 row | `rows`, `column_alignments` | ≤4 columns × 6 logical rows (5 data rows) | more rows: a `table` slide |
| `timeline` | `milestones` 3–7 | `heading` | label ≤60, date ≤30, body ≤200; `end_date` → bars | — |
| `image` | `image {path \| url, alt, fit}` | `caption` ≤120 | cover (default) or contain | `callouts` → use `image_case` |
| `text` | `body` or `bullets` | `heading` | body ≤400 runes, ≤6 bullets | a column too long for what its neighbour can give up |
| `cycle` | `phases` | `style`, `center`, `highlight`, `intake`, `left_label`, `right_label`, `left_count` | the style's phase counts (ring 4–8, nodes 3–8, intake 3–8, figure_eight 4–8, radial 4–8, concentric 3–5) | `figure_eight` anywhere but a full-width region |

Unknown region kinds are refused with the translation: `bridge` → a
`chart` region of `type: waterfall`; `kpi_snapshot` → `kpis`;
`chart_insight` → `chart`; `image_case` → `image`. A region's own `source`
joins the slide's source line.

### What a region does with its height

A `chart` or an `image` fills its rectangle under a top heading. A `text`
region takes the height its lines need and a `stat` grows into what is left,
centred. Beside a `cycle` the text is set as one block centred on the ring's
axis; a `kpis`, `stat`, `timeline` or `table` beside a cycle is centred on
the same line. In `main_left` / `main_right` with a cycle as the main region
the two stacked regions meet in the middle of their column.

### Recipe: KPI strip over the exhibit — `main_top`

The steering-committee opener: the three numbers across the top, the trend
and the plan beneath them, one claim over all three.

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Harbour Bank programme office, October 2026
slides:
  - kind: regions
    title: The programme is amber on schedule and green on spend after month three
    arrangement: main_top
    regions:
      - kind: kpis
        size_pct: 32
        kpis:
          - {value: 14 of 22, label: milestones met, comparator: plan 16}
          - {value: EUR 1.9M, label: spent to date, comparator: budget EUR 2.1M}
          - {value: "31", label: second-line hires in post, comparator: target 44}
      - kind: chart
        size_pct: 60
        heading: Milestones met vs plan
        chart:
          type: line
          data:
            categories: [Jul, Aug, Sep]
            series:
              - name: Plan
                values: [4, 10, 16]
              - name: Actual
                values: [4, 9, 14]
            highlight: [Actual]
      - kind: text
        heading: Why two milestones slipped
        bullets:
          - Hiring ran four weeks behind the plan in August
          - Data platform vendor contract signed 3 September, not 1 August
    takeaway: Both slips are recovered by December if the October hiring wave lands.
```

### Recipe: picture beside the number — `main_right` with an `image` region

```yaml
meta:
  title: Atlas Retail data platform
  template: modern-template
  date: October 2026
  source: Atlas Retail ops console, September 2026
slides:
  - kind: regions
    title: The nightly load missed its 6 am SLA on 61 of the last 90 days
    arrangement: main_right
    regions:
      - kind: image
        size_pct: 58
        image:
          path: ../../examples/images/ops-console.png
          alt: Nightly load dashboard with the SLA breach banner
          fit: contain
        caption: Ops console, 09:42, queue not drained
      - kind: stat
        value: 61 / 90
        label: nights over the 6 am SLA
        context: Target after migration is fewer than 3 per 90 days
      - kind: text
        bullets:
          - Finance closes on stale data two days in three
          - The queue is the 2009 warehouse, not the network
    takeaway: The breach is structural; tuning has been tried twice.
```

## 3. Raw `compose` — whole patterns and diagrams in segments

Inside a `raw_json2pptx` slide (or a raw deck), `compose` replaces `pattern`
and `shape_grid` (the three are exclusive):

```
compose:
  direction: horizontal | vertical
  gap: <pt>              # default: the template's grid gutter
  smart_compose: true    # only when no size_pct is set: shares from probed density
  banner:  {text, emphasis: bold|italic|bold-italic, accent}   # band above, no segment slot
  callout: {text, emphasis: bold|italic|bar|band|subtle, accent} # band below, no segment slot
  segments:
    - size_pct: 60
      pattern: {name, values, overrides, cell_overrides, callout, vertical_align}
    - size_pct: 40
      diagram: {type, title, alt, data}
    - compose: {direction, gap, segments}     # one nested level
```

Limits: 2–8 segments per envelope, depth 2 (one nested envelope inside a
segment), 12 leaf segments across the tree, exactly one of `pattern` /
`compose` / `diagram` per segment, set `size_pct` summing to ≤100 with the
rest split equally. A segment is expanded inside its own rectangle (less a
4pt inset) so width-sensitive patterns see their real width; a horizontal
segment is exactly its share minus one gap per neighbour. Per-segment
findings are addressed `segments[i].pattern.…`. `compose` never paginates. A
`banner` is refused over a `strategy-house` or `pull-quote` first segment
(they are banner-like already); a segment's own `bounds` / `max_height_pct`
is ignored with `COMPOSE_SEGMENT_BOUNDS_IGNORED`. Relative image paths inside
segments resolve against the deck's `base_dir`. Native diagrams
(`process_flow`, `swot`, `house_diagram`, `kpi_dashboard`, …) are valid
`diagram` segments, not only svggen charts.

### Recipe: a wide loop over a KPI band — vertical compose

The only way a `figure_eight` shares a slide: it keeps the full width and a
content-sized band sits under it.

```yaml
meta:
  title: Platform operating model
  template: midnight-blue
  date: October 2026
  source: Platform team, Q3 2026 retrospective
slides:
  - kind: raw_json2pptx
    slide:
      slide_type: content
      layout_id: blank-title
      content:
        - placeholder_id: title
          type: text
          text_value: Build and run share one loop, and the loop closed 37 incidents at source this quarter
      compose:
        direction: vertical
        segments:
          - size_pct: 72
            pattern:
              name: cycle-figure-eight
              values:
                left_label: Build
                right_label: Run
                phases:
                  - {label: Plan}
                  - {label: Develop}
                  - {label: Release}
                  - {label: Operate, highlight: true}
                  - {label: Observe}
                  - {label: Learn}
          - size_pct: 28
            pattern:
              name: kpi-inline
              values:
                - {big: "37", small: incidents fixed at source}
                - {big: 11 d, small: median lead time}
                - {big: 99.95%, small: availability}
        callout:
          text: Observe feeds Plan directly; nothing waits for the quarterly review.
          emphasis: bar
```

### Recipe: ring in legend mode beside a structured finding — horizontal compose

Under about 450pt of width a `cycle-ring` moves its labels into a numbered
legend of its own accord; pair it with a content-sized pattern on the other
side.

```yaml
meta:
  title: Control cycle
  template: forest-green
  date: October 2026
  source: Cobalt Insurance Internal Audit, FY26
slides:
  - kind: raw_json2pptx
    slide:
      slide_type: content
      layout_id: blank-title
      content:
        - placeholder_id: title
          type: text
          text_value: The control cycle breaks at Test, where three of nine applications were skipped
      compose:
        direction: horizontal
        gap: 12
        segments:
          - size_pct: 50
            pattern:
              name: cycle-ring
              values:
                center: {label: Quarterly}
                phases:
                  - {label: Design}
                  - {label: Operate}
                  - {label: Test, highlight: true}
                  - {label: Report}
                  - {label: Remediate}
          - size_pct: 50
            pattern:
              name: labeled-rows
              values:
                rows:
                  - {label: FOUND, body: "Privileged-access recertification not performed for 3 of 9 applications; 41 accounts unreviewed."}
                  - {label: WHY, body: "Unreviewed privileged accounts can change financial data undetected; a SOX key control."}
                  - {label: ACTION, body: "Full recertification by 31 Dec 2026; quarterly cadence in the access tooling from H1 2027."}
```

## 4. Content-sized, filling and growing: what a block does with spare room

| Behaviour | Patterns | Alone on a slide | In a segment or region |
|---|---|---|---|
| **Band, segment only** | `kpi-inline`, `before-after-compact`, `process-flow-compact` | stays a band and reports `SLIDE_UNDERUSED` — use the full variant | the intended home; hangs from the body line |
| **Content-sized box** | `card-grid`, `before-after`, `comparison-2col`, `strategy-house`, `process-flow`, `arch-stack`, every `kpi-Nup` | cards hug their text; empty area past 20% is `SLIDE_UNDERUSED` | content height, not stretched |
| **Content-sized strip** | `phase-roadmap`, `timeline-horizontal` | threshold 22% | — |
| **Hero** | `stat-hero`, `pull-quote` | exempt from the lower-third rule | — |
| **Grows alone** | an open `kpi-Nup` row (dividers to half the area); `value-chain` and `process-flow` with sentence-length steps (up to 1.6×) | grown, no finding | not grown |
| **Zoomed alone** | every other pattern needing under about 75% of the area | rows to 1.6×, type up to two steps, optically centred | never — segments and cells hang from the body line |

A `process-flow` of short labels alone reports `SPARSE_SINGLE_ROW_FLOW`
instead of growing; a block that leaves the lower third empty is
`SLIDE_UNDERUSED`; a 40% empty band is `VERTICAL_IMBALANCE` /
`HORIZONTAL_IMBALANCE`. Pairing a content-sized block with a second zone is
usually the fix the finding names.

## 5. Segment-fit matrix: what survives at which width

The shipped templates give about 824–828pt of content width. A 50% segment
or `columns` region is therefore ~400–410pt after the gap and insets; 60% is
~490pt; 70% is ~570pt.

| Visual | Full width (~825pt) | 60–70% (~490–570pt) | 50% (~400pt) | 30–40% (~240–330pt) |
|---|---|---|---|---|
| `cycle-ring` | labels beside the ring | legend beside the ring (under 450pt) | legend | legend; descriptions dropped with a warning |
| `cycle-nodes` | labels outside | legend when the label columns fall under 120pt | legend | legend |
| `cycle-intake` | lane feeding the ring at 9 o'clock | lane stacked above the ring | stacked | avoid |
| `cycle-figure-eight` | draws | **refused** (needs 580pt) — `fit_overflow`, `swap_pattern` → `cycle-ring`; a DeckSpec `cycle` region errors at `style` | refused | refused |
| `radial-hub` | labels beside satellites | `legend` fallback (A, B, C …) | legend | avoid |
| `concentric-rings` | rings + side ladder | labels only in a half-width cell | labels only | avoid |
| `chart` (bar / line / waterfall) | — | the exhibit's natural home | fine for ≤6 categories | sparkline-sized; one series |
| `kpis` region / `kpi-Nup` | 2–6 | 2–4 | 2–3 | one `stat` instead |
| `kpi-5up` / `kpi-6up` values | 11 / 9 digits on the narrowest template | do not place in a segment | — | — |
| `table` region | — | 4 × 5 | 3 × 5 | 2 × 4 |
| `text` region | — | 400 runes / 6 bullets | 2–3 bullets or 2 sentences | one number + one line |
| `chart-insights-split` | 65/35 (75/25 when sparse) | — | — | — |
| `process-flow`, 3–6 steps | one row | one row, short labels | 3–4 steps | `process-flow-compact` band |
| `process-flow`, 7–8 steps | two rows | avoid | avoid | avoid |
| `labeled-rows`, `metric-list`, `hero-detail` | fine | fine | fine (3 rows / 3–4 metrics) | 2–3 items |
| `image` | — | the picture's home | a square crop | a thumbnail; use `image_case` instead |

Two consequences worth remembering: a loop beside words is always a legend
layout (give the ring 50–60%, not less, and keep the words to one block); and
anything that needs the full width — figure eight, a two-row flow, a
7+ column table — goes **above or below** its companion (`rows`,
`main_top`, `main_bottom`, a vertical `compose`), never beside it.

## 6. Previewing a split before authoring

`recommend_visual(intent:"…", candidates:["compose:cycle-ring+labeled-rows"],
template, preview:true)` renders the pairing; a plain "beside / next to /
the remaining third" intent returns `regions` first with a `compose` form
below it, each with a `composition {direction, regions[] {name, size_pct,
position, data_contract}}` and a runnable `next_tool_call`.
`list_slide_kinds(kinds:["regions"], preview:true)` shows the default
`main_left`. The playbooks' decks under
[`examples/semantic/playbooks/`](../../examples/semantic/playbooks/) and
[`examples/circular-layouts.json`](../../examples/circular-layouts.json)
(slides 3, 7, 9, 11) are validated splits to copy from.
