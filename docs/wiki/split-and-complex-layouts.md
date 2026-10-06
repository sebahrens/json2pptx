# Split and complex layouts

One slide, several things — done so it reads as one slide. Every recipe here
was authored through the MCP by an agent that saw only the product, rendered,
inspected on two templates, and then tightened. Each YAML block is a complete
DeckSpec that validates against the current build; the four full decks are in
[`examples/semantic/playbooks/`](../../examples/semantic/playbooks/). Back to
the [hub](README.md); the engine's contract for these kinds is in
[`DECKSPEC.md`](../../skills/generate-deck/DECKSPEC.md).

**When is a split right?** When two or three views prove the *same* action
title: the trend and the number it produces; the bridge and what it means for
the bid; the screenshot and the fact it shows. A second conclusion gets a second
slide. The dominant evidence takes the larger share (60–67%); the narrow side
holds one number and a few short lines, never a paragraph.

## 1. Chart left, number and bullets right — `regions`, `main_left`

The most-used consulting split: the exhibit on the left, the headline number
over two or three bullets on the right. `regions` takes 2–3 typed regions:
`chart`, `stat`, `kpis`, `table`, `timeline`, `image`, `text`. With
`arrangement: main_left`, `regions[0]` is the main region (its `size_pct` is
its width share, default 60) and `regions[1..2]` stack on the right, splitting
that side by their own `size_pct`. One title, one `source`, one `takeaway`
cover the slide.

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Regulator's 2025 review; Harbour Bank management information 2023-2025
slides:
  - kind: regions
    title: Operational losses rose 84% in two years to EUR 11.2M
    arrangement: main_left
    regions:
      - kind: chart
        size_pct: 62
        heading: Operational-risk losses
        unit: EUR M
        chart:
          type: bar
          data:
            categories: ["2023", "2024", "2025"]
            series:
              - name: Losses
                values: [6.1, 8.4, 11.2]
      - kind: stat
        size_pct: 45
        value: EUR 11.2M
        label: Op-risk losses 2025
        context: Peer median about 0.02% of assets, roughly EUR 8.4M for Harbour Bank
      - kind: text
        size_pct: 55
        heading: Why losses are rising
        bullets:
          - Controls are self-assessed by 1,200 owners with no independent testing
          - No integrated reporting, so repeat incidents are not spotted across units
    takeaway: Losses exceed both the EUR 8M appetite and the peer median.
```

Review points: the right column spans the chart's height — the text region
takes the height its bullets need and the stat grows into the rest, centred
— so keep it to one number and two or three bullets; a text region too long
for what the stat can give up is refused on its bullets. A single-series bar
chart accents its last
bar by default — the 2025 bar here — which is what the title argues.

## 2. Bridge beside the implication — `regions`, `columns`, waterfall chart

A margin or P&L walk with the "so what" beside it. There is no `bridge`
region: the bridge is a **`chart` region of `type: waterfall`** whose `data`
is `points`, each with a `type` (`total`, `increase`, `decrease`, `subtotal`).
Use `arrangement: columns` for two side-by-side regions.

```yaml
meta:
  title: Project Falcon
  template: p-style
  date: October 2026
  source: Nordbolt management accounts 2023-2025
slides:
  - kind: regions
    title: EUR 6.5M of the EUR 7.0M EBITDA gain since 2023 is price
    arrangement: columns
    regions:
      - kind: chart
        size_pct: 66
        heading: EBITDA bridge 2023 to 2025
        unit: EUR M
        chart:
          type: waterfall
          data:
            points:
              - {label: EBITDA 2023, value: 24.0, type: total}
              - {label: Price, value: 6.5, type: increase}
              - {label: Volume, value: 3.2, type: increase}
              - {label: Raw material, value: -4.1, type: decrease}
              - {label: Opex, value: 1.4, type: increase}
              - {label: EBITDA 2025, value: 31.0, type: total}
      - kind: text
        size_pct: 34
        heading: What this means for the bid
        body: Nearly all of the margin expansion came from price taken during a raw-material spike. If customers claw back even half of it when steel normalises, run-rate EBITDA falls by about EUR 3M and the multiple is paid on a number that does not hold.
        bullets:
          - Price durability is the first interview question
          - "Volume added EUR 3.2M: real, if share held"
    takeaway: The bid rests on whether EUR 6.5M of price sticks.
```

A `total` that differs from the running sum by more than 0.5% is reported
(`chart.waterfall_total_mismatch`). The axis starts at zero whenever no running
total is negative, so the opening total is drawn in full; `data.y_min` /
`data.y_max` override the axis deliberately (a `y_min` that hides more than
half of the smallest bar is reported as `chart.axis_not_zero`).

## 3. Chart, headline number and a small table — three regions

The business-case staple: the series on the left, the saving as a number over
the three-line table that derives it. A table region holds up to 4 columns and
5 rows; the number sits above it.

```yaml
meta:
  title: Atlas Retail data platform
  template: modern-template
  date: October 2026
  source: Atlas Retail five-year TCO model, September 2026
slides:
  - kind: regions
    title: "Run cost falls from EUR 3.6M to 1.5M a year: EUR 5.5M saved by 2031"
    arrangement: main_left
    regions:
      - kind: chart
        size_pct: 62
        heading: Annual run cost, current vs target
        unit: EUR M
        chart:
          type: bar
          data:
            categories: [Year 1, Year 2, Year 3, Year 4, Year 5]
            series:
              - name: Current
                values: [3.6, 3.7, 3.7, 3.7, 3.7]
              - name: Target
                values: [4.8, 3.1, 1.9, 1.6, 1.5]
      - kind: stat
        size_pct: 35
        value: EUR 5.5M
        label: saved over five years
        context: Year 1 includes EUR 4.8M migration
      - kind: table
        size_pct: 65
        headers: [Five-year TCO, EUR M]
        rows:
          - [Current, "18.4"]
          - [Target, "12.9"]
          - [Saving, "5.5"]
        column_alignments: [left, right]
    takeaway: The year-1 spend of EUR 4.8M pays back by year 3; the saving is EUR 5.5M by 2031.
```

## 4. Status board — `option_matrix` with a `rag` criterion

Risk-appetite dashboards, control results by domain, workstream status: rows
of things with a coloured status and the figures behind it. `option_matrix`
takes **a scale per criterion**: `scale: rag` draws red / amber / green dots,
`scale: text` prints the metric and the limit as they are, `harvey` (the
default) draws 0–4 balls. `recommended` highlights the rows that matter (here
the breached ones) and `highlight_label` names the badge; `decisive_criterion`
tints the column the audience should read first.

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Harbour Bank risk appetite statement; management information Q3 2026
slides:
  - kind: option_matrix
    title: Two of five risk appetite metrics are breached, one is amber
    criteria:
      - {label: Status, scale: rag}
      - {label: Current, scale: text}
      - {label: Limit, scale: text}
    options:
      - {name: Credit, scores: [green, NPL 2.1%, "3%"]}
      - {name: Liquidity, scores: [green, LCR 148%, "110%"]}
      - {name: Operational, scores: [red, EUR 11.2M, EUR 8M]}
      - {name: Conduct, scores: [amber, "37", "30"]}
      - {name: Cyber, scores: [red, "2", "0"]}
    recommended: [Operational, Cyber]
    highlight_label: Breached
    decisive_criterion: Status
    takeaway: Operational and cyber are breached; conduct is drifting towards its limit.
```

Keep `detail` lines off the options when there are five rows and the template
is narrow: the first attempt with per-row details was measured at 5–6pt and the
product's verified fix was to drop the visual (`go-slide-creator-u8orh`);
removing the details kept it at 12pt. Quote numbers ("37") so YAML keeps them
as text. A results board that also needs the counts (controls tested,
exceptions, rating) is the same shape with `text` criteria for the counts — see
[playbook-risk-assurance.md](playbook-risk-assurance.md).

## 5. Before and after, aligned row by row — `comparison`

Two columns whose rows correspond: today vs target, current vs proposed, A vs
B. The kind takes exactly two balanced columns `{header, items[]}` and keeps
rows aligned with hairline rules. Six rows is the comfortable maximum; the
`pattern_overcrowded` note at 10+ cells is advisory when every item is one
line.

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Harbour Bank organisation data, September 2026
slides:
  - kind: comparison
    title: The target model adds 16 FTE to the 2nd line and one integrated report
    columns:
      - header: Today
        items:
          - "1st line: 1,200 control owners self-assess their controls"
          - "2nd line: Risk & Compliance, 28 FTE"
          - "3rd line: Internal Audit, 12 FTE"
          - "Weakness: risk appetite not cascaded below the board"
      - header: Target (June 2027)
        items:
          - "1st line: 1,200 owners trained, controls tested quarterly"
          - "2nd line: Risk & Compliance, 44 FTE (+16)"
          - "3rd line: Internal Audit, 12 FTE, assures the new framework"
          - Appetite cascaded to business-unit limits and KRIs
    takeaway: Same three lines; the 2nd line grows and the reporting becomes one view.
```

For a per-row **connector** (the "from → to" chevron in a centre gutter) add
`connectors: true`; `highlight_column: right` (or `left`, or a header) gives
the argued column a solid header and tinted rows; `highlight_row` tints one
row instead (never both). All three apply to two-column comparisons only — the
KPI before/after in [playbook-technology-and-data.md](playbook-technology-and-data.md)
uses the first two.

## 6. Photo or screenshot with callouts — `image_case`

A picture beside the words about it: a comparable's site, a product
screenshot, an ops console. `image` is `{path | url, alt, fit}` (paths resolve
against the spec's directory on the CLI and `base_dir` over the MCP); `callouts`
are `{label, x, y}` with fractions of the picture; `eyebrow` and `heading` sit
over the `body`; up to 5 `bullets` and 3 `metrics`.

```yaml
meta:
  title: Atlas Retail data platform
  template: modern-template
  date: October 2026
  source: Atlas Retail ops console screenshot, September 2026
slides:
  - kind: image_case
    title: "The ops console shows the breach: 61 of 90 nights, 1.5 h is the target"
    image:
      path: ../../examples/images/ops-console.png
      alt: Nightly load dashboard with the SLA breach banner and the job queue
      fit: contain
    callouts:
      - {label: SLA breach, x: 0.27, y: 0.135}
      - {label: "Job queue (14,860)", x: 0.50, y: 0.54}
    eyebrow: Nightly load dashboard
    heading: What the operations team sees most mornings
    body: The nightly load dashboard flags the SLA breach at the top and a job queue in the centre that has not drained by 09:42.
    bullets:
      - 61 of the last 90 nights missed the 6 am SLA
      - "Target state: a 1.5 h load window and fewer than 3 misses per 90 days"
    metrics:
      - {value: 61 / 90, label: SLA misses today}
      - {value: 1.5 h, label: target load window}
    caption: Ops console, queue health, 09:42 UTC
```

Callouts landed exactly on their fractional points on both templates in the
runs. `image_width_pct` (30–60, default 45) sets the picture column: a wide
`contain` screenshot wants 55–60, and the text column then holds less — cut
the body to one sentence or the eyebrow is squeezed below 12pt and the render
is refused. A missing file is `SEMANTIC_IMAGE_MISSING` and blocks readiness;
never ship a labelled placeholder frame.

## 7. Architecture with cross-cutting rails — `architecture`

Tiers top to bottom, one block per component, and the concerns that run
through every tier as vertical **rails** beside the stack, not as another tier.

```yaml
meta:
  title: Atlas Retail data platform
  template: modern-template
  date: October 2026
  source: Atlas Retail target architecture, September 2026
slides:
  - kind: architecture
    title: Five tiers and two rails make up the target lakehouse
    tiers:
      - {name: Consumption, items: [Power BI, 3 data products, ML feature store]}
      - {name: Serving, items: [Semantic layer, Governed marts]}
      - {name: Processing, items: [Spark batch, Streaming]}
      - {name: Storage, items: [Delta lake, Bronze / silver / gold]}
      - {name: Ingestion, items: [CDC from 14 sources, Event streaming]}
    rails: [Security & governance, FinOps]
    takeaway: Security, governance and FinOps run through every tier; they are built in Foundation, not bolted on.
```

3–6 tiers, 1–12 items per tier (7+ wrap onto two rows), items ≤ 40 characters.

## 8. Long tables: how many rows, and how to split

A native `table` holds up to **10 logical rows including the header** (nine
data rows). Rows past seven render at a compact pitch (the text's own line
height, about 0.3in at 12pt) so an eight-row price list or a seven-finding
register stays on one slide; a row that still cannot fit is refused as
`table_rows_truncated` with the `split_at_row` to use — nothing is silently
dropped. A cell with a newline, or a comma list of three or more items in a
cell of 48+ characters, counts as extra rows.

When a table must split: same headers on both halves, the first keeps the
action title, the second is titled `<same title> (2/2)` so the rhythm checks
treat the pair as one exhibit; keep them adjacent; put the backup detail in an
appendix table instead when the audience does not need every row
(`section` with `appendix: true`, then the table — pages read A1, A2).

```yaml
meta:
  title: Project Falcon
  template: p-style
  date: October 2026
  source: Nordbolt information memorandum; market model desk research, 2026
slides:
  - kind: table
    title: Eight deliverables over four weeks make up the EUR 320k fee
    headers: [Phase, Deliverable, Days, Fee (EUR k)]
    column_alignments: [left, left, right, right]
    rows:
      - [Week 1, Data room review and hypothesis tree, "8", "40"]
      - [Week 1, "Market model v1: size, growth, segments", "12", "80"]
      - [Week 2, Interview guide and target list, "4", "20"]
      - [Week 2, Customer interviews wave 1 (12), "14", "60"]
      - [Week 3, Customer interviews wave 2 (13), "14", "60"]
      - [Week 3, Synthesis workshop with deal team, "4", "20"]
      - [Week 4, Red-flag report, "6", "20"]
      - [Week 4, IC pack and Q&A support, "6", "20"]
      - [Total, "Eight work packages, 68 consultant days", "68", "320"]
    totals_row: true
    takeaway: Interviews are 140k of the 320k; synthesis and IC pack 60k; expenses billed at cost.
```

## 9. One structured finding per slide — raw `labeled-rows`

Audit findings, risk observations and issue logs read best as labelled rows
(WHAT WE FOUND / WHY IT MATTERS / ACTION / OWNER AND DATE), not bullets. No
DeckSpec kind draws this yet; the `labeled-rows` pattern does, through a
`raw_json2pptx` slide — the pattern block is carried verbatim, inline `**bold**`
is honoured, and the slide still takes `source` and `notes`. Get the fields from
`show_pattern` (in the default profile) before writing one.

```yaml
meta:
  title: FY26 IT General Controls Review
  template: forest-green
  date: October 2026
  source: Cobalt Insurance Internal Audit, FY26 ITGC review
slides:
  - kind: raw_json2pptx
    slide:
      slide_type: content
      layout_id: blank-title
      content:
        - placeholder_id: title
          type: text
          text_value: "High: privileged access was not recertified for 3 of 9 applications"
      pattern:
        name: labeled-rows
        values:
          rows:
            - label: WHAT WE FOUND
              sublabel: access management
              body: The annual privileged-access recertification was not performed for 3 of the 9 in-scope applications (claims, policy admin, general ledger). 41 privileged accounts went unreviewed in FY26.
            - label: WHY IT MATTERS
              sublabel: financial reporting risk
              body: "Unreviewed privileged accounts can change financial data and configurations without detection. **This is a SOX key control and the main driver of the red rating for access management.**"
            - label: ACTION
              sublabel: agreed by management
              body: Run a full recertification of all privileged accounts across the 9 applications in Q4 2026, then move recertification into the access tooling delivered in H1 2027 with quarterly cadence.
            - label: OWNER AND DATE
              sublabel: "severity: high"
              body: "Head of IAM. Recertification complete by **31 Dec 2026**; tooling live by 30 Jun 2027; Internal Audit re-tests in Q3 2027."
```

## 10. Heat map with named risks — `risk_heatmap`

Four quadrants with a headline and a description each is `matrix_2x2`
(`x_axis`, `y_axis`, `quadrants`, one optional `highlight`). Named risks rated
low / medium / high on likelihood and impact are `risk_heatmap`: `items` are
1–20 `{name ≤40, likelihood, impact}`, placed on a 3 × 3 (`size: 5` for a
5 × 5). A level is `low` / `medium` / `high` (`very low` … `very high` at
size 5), a number 1–`size` (1 = lowest), or one of the grid's own
`likelihood_levels` / `impact_levels` labels (lowest first, ≤14 chars each).
Every cell is drawn and filled by its likelihood × impact band — neutral,
tint, solid of the template's negative accent — so the colour is never an
authoring choice; risks that share a cell stack one per line. A grid too
crowded for 12pt names reports `BODY_TOO_LONG` at the fullest cell's first
risk: shorten the names, keep the top risks on the map and the rest in a
`table`, or drop the takeaway.

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Harbour Bank top-risk register, Q3 2026
slides:
  - kind: risk_heatmap
    title: Cyber is the one severe risk rated almost certain
    size: 5
    likelihood_levels: [Rare, Unlikely, Possible, Likely, Almost certain]
    impact_levels: [Negligible, Minor, Moderate, Major, Severe]
    items:
      - {name: Cyber attack, likelihood: Almost certain, impact: Severe}
      - {name: Third-party outage, likelihood: Likely, impact: Major}
      - {name: Data breach, likelihood: 4, impact: 4}
      - {name: Model risk, likelihood: 2, impact: 5}
      - {name: Conduct, likelihood: 3, impact: 3}
      - {name: Climate transition, likelihood: 2, impact: 3}
      - {name: Payment fraud, likelihood: 4, impact: 1}
    takeaway: One risk sits in the top corner; two more are major and likely.
```

See [playbook-risk-consulting.md](playbook-risk-consulting.md) for the 3 × 3
in a full deck. The raw `risk-heatmap` pattern takes the same `values` plus
`tier_labels` (the legend wording) and `overrides.show_legend`.

## 11. Roadmap with parallel tracks — `roadmap`

Phase boxes on a time axis plus 0–4 full-width "In parallel" bars for the
workstreams that run alongside every phase. `phases` are 3–6
`{name ≤40, date_label ≤30, description ≤160, milestone?}`; `parallel_tracks`
0–4 strings ≤90; `parallel_label` renames the bar label. Each track is a bar
one line tall. When the phases, milestones, tracks and a `takeaway` band do
not fit at full padding (the shortest area is `modern-template`'s), the rows
give up padding first and the phase names step down no further than 12pt;
three phases with milestones, two tracks and a takeaway fit on every shipped
template. What still cannot fit is reported as `BODY_TOO_LONG` on each
over-long `description` with the character count the area holds, or on the
last track with the number of tracks there is room for.

```yaml
meta:
  title: Harbour Bank ERM uplift
  template: midnight-blue
  date: October 2026
  source: Harbour Bank programme plan, October 2026
slides:
  - kind: roadmap
    title: Four phases and two parallel tracks finish before the 30 June 2027 deadline
    phases:
      - {name: Mobilise, date_label: Jul-Aug 2026, description: "Steering committee chartered; joint team on site"}
      - {name: Design, date_label: Sep-Nov 2026, description: "Appetite cascade, taxonomy and control framework designed"}
      - {name: Build, date_label: Dec 2026-Mar 2027, description: "Controls tested independently; 2nd line hired to 44 FTE"}
      - {name: Embed, date_label: Apr-Jun 2027, description: "First integrated board report; findings evidenced to the regulator"}
    parallel_tracks:
      - Data and reporting platform (Sep 2026 - Jun 2027)
      - 1,200 control owners trained (Dec 2026 - Jun 2027)
    takeaway: Every phase ends before the regulator's deadline; the two tracks start in Design and Build.
```

For bars drawn to scale on a labelled time axis (several workstreams, each
with dated bars), the raw `roadmap-phased` pattern remains the tool.

## 12. Several patterns on one slide — raw `compose`

When a region kind cannot express a part (a KPI column that must be a
`metric-list`, a timeline footer under a chart), the raw slide `compose`
envelope places whole patterns and svggen diagrams in segments:
`direction` (`horizontal` | `vertical`), `segments[]` each with one `pattern`,
one `diagram` (with `alt`) or a nested `compose`, and `size_pct` shares
(≤ 8 segments, depth 2). Each segment is fit-checked in its own rectangle
(`BODY_TOO_LONG` names `segment[i]`); `compose` never paginates. The canonical
example — `list_slide_kinds kinds:["raw_json2pptx"]` returns it as
`composed_example`, rendered as returned:

```yaml
meta:
  title: Launch review
  template: midnight-blue
  date: October 2026
  source: Illustrative
slides:
  - kind: raw_json2pptx
    slide:
      slide_type: content
      layout_id: blank-title
      source: Illustrative
      content:
        - placeholder_id: title
          type: text
          text_value: Revenue grew 75% in four quarters, so the launch can hold a 32% margin
      compose:
        direction: vertical
        gap: 6
        segments:
          - size_pct: 65
            compose:
              direction: horizontal
              gap: 6
              segments:
                - size_pct: 62
                  diagram:
                    type: line
                    title: Quarterly revenue (EUR m)
                    alt: Quarterly revenue rises from EUR 12m in Q1 to EUR 21m in Q4
                    data:
                      categories: [Q1, Q2, Q3, Q4]
                      series:
                        - name: Revenue
                          values: [12, 14, 17, 21]
                - size_pct: 38
                  pattern:
                    name: metric-list
                    values:
                      items:
                        - {label: Revenue growth, value: "+75%"}
                        - {label: Gross margin, value: "32%"}
                        - {label: Markets live, value: "4"}
          - size_pct: 35
            pattern:
              name: timeline-horizontal
              values:
                - {label: "Oct: Design"}
                - {label: "Nov: Pilot"}
                - {label: "Dec: Rollout"}
```

Prefer `regions` whenever its eight region kinds suffice: it is validated field
by field, degrades nowhere, and patches by `path`.

## 13. Cycle beside the narrative — `regions` with a `cycle` region

A loop on its own slide is the `cycle` kind (`phases`, a `style`, one
`highlight`; see [visual-vocabulary.md](visual-vocabulary.md), "The cycle
family"). When the slide also has to say what the loop has achieved, put the
loop in a `cycle` region and the words in a `text` region. The region takes
the kind's own fields under their canonical names (`phases`, `style`,
`center`, `highlight`, `intake`). Give it 50–60% of the width: there the ring
keeps its shape and moves its labels into a numbered legend. `ring`, `nodes`,
`radial`, `concentric` and `intake` (which stacks its lane above the ring)
all fit a column; `figure_eight` needs the full width, so it is refused in a
side-by-side region with the fix in the message — use `ring` there, or
`arrangement: rows`. In a stacked group a cycle region takes at least 60% of
the height. A `text` region in a column beside the cycle is set as one block
(heading, then body) centred on the ring's axis rather than hung from the top.

```yaml
meta:
  title: Northwind operations review
  template: midnight-blue
  date: October 2026
  source: Northwind service desk data, January–September 2026
slides:
  - kind: regions
    title: The monthly improvement loop has cut rework by a third
    arrangement: columns
    regions:
      - kind: cycle
        size_pct: 55
        phases:
          - Plan
          - Do
          - {label: Check, highlight: true}
          - Act
        center: Monthly
      - kind: text
        heading: What changed since January
        bullets:
          - Rework is down 34% across the three service lines
          - Every phase has one named owner and one output
          - Check now reads live defect data, not the month-end report
    takeaway: The loop works because Check is no longer optional.
```

## Choosing between them

| You need | Use |
|---|---|
| Exhibit + number + bullets proving one title | `regions` `main_left` (§1) |
| Bridge / walk + its implication | `regions` `columns`, waterfall chart (§2) |
| Series + saving + the table behind it | `regions` `main_left`, chart + stat + table (§3) |
| Rows with a coloured status and figures | `option_matrix`, `rag` + `text` criteria (§4) |
| Today vs target, row by row | `comparison` with `connectors` / `highlight_column` (§5) |
| Picture and the words about it, pointers on the picture | `image_case` with `callouts` (§6) |
| Layers with concerns running through all of them | `architecture` with `rails` (§7) |
| Up to nine data rows | one `table`; beyond, split `(2/2)` or appendix (§8) |
| One finding, structured | raw `labeled-rows` (§9) |
| 3 × 3 / 5 × 5 risk heat map with names | `risk_heatmap` (§10) |
| Phases plus parallel workstreams | `roadmap` with `parallel_tracks` (§11) |
| Shapes that share one place in a hand-built grid (ring segments, a badge on a shape) | raw `shape_grid` cell `layers` ([INPUT_FORMAT.md](../INPUT_FORMAT.md)) |
| Whole patterns side by side or stacked | raw `compose` (§12) |
| A loop, hub or nested rings beside the words about it | `regions` with a `cycle` region (§13) |
