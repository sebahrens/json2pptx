# Playbook: technology and data

Platform business cases, architecture reviews, cloud and data migrations,
vendor selections, data-governance and AI-readiness proposals. The worked
deck is a steering-committee business case to replace an on-premise warehouse
with a cloud lakehouse:
[`examples/semantic/playbooks/tech-data-platform-business-case.yaml`](../../examples/semantic/playbooks/tech-data-platform-business-case.yaml)
(12 slides, `modern-template`, cross-rendered on `p-style`). Back to the
[hub](README.md).

## Audience and the ask

A steering committee that funds in tranches: a CIO, a CFO, business sponsors.
They want the pain quantified, the target drawn, the vendor choice defended,
the money and the saving over five years, the plan with its parallel
dependencies, and the KPIs they will hold you to. The ask is a year-1 budget
and a contract. Tone: operational numbers (SLA misses, load windows, defect
rates), architecture as a picture, every cost line sourced to the model.

## Storyline skeleton

```
1. From a 2009 warehouse to a cloud lakehouse
2. Approve EUR 4.8M now: the lakehouse saves EUR 5.5M in five years          executive_summary
3. The 2009 warehouse missed its 6 am SLA on 61 of the last 90 days          kpi_snapshot (6 KPIs)
4. The ops console shows the breach: 61 of 90 nights, 1.5 h is the target    image_case with callouts
5. Five tiers and two rails make up the target lakehouse                     architecture
6. Databricks scores best on migration risk and skills availability           option_matrix
7. Run cost falls from EUR 3.6M to 1.5M a year: EUR 5.5M saved by 2031        regions (chart + stat + table)
8. Compute is 48% of run cost, so FinOps guardrails protect the saving        chart_insight (donut)
9. Three phases decommission the warehouse by the end of 2028                roadmap (milestones + parallel_tracks)
10. Four KPIs move: the load window drops from 9.5 h to 1.5 h                 comparison (connectors, highlight_column)
11. Approve EUR 4.8M for year 1 and the Databricks contract                   decision
12. Four actions start Foundation in January 2027                             next_steps
```

Pain (3–4) → target (5) → choice (6) → money (7–8) → plan (9) → outcome (10)
→ ask (11–12).

## Slide-by-slide blueprint

| # | Kind | What goes in | Watch |
|---|---|---|---|
| 3 | `kpi_snapshot` | Sources, jobs, load window, SLA misses, defect rate, reconciliation effort | Six KPIs hold about 9 characters per value; captions one line. |
| 4 | `image_case` | Console screenshot, `fit: contain`, `image_width_pct: 55`, two `callouts`, eyebrow / heading / one-sentence body, 2 bullets, 3 `metrics` | Callout `x`,`y` are fractions of the picture; a wider picture leaves less room for text. |
| 5 | `architecture` | 5 tiers, 2–3 items each; `rails: [Security & governance, FinOps]` | Items ≤ 40 characters; rails read as vertical bands. |
| 6 | `option_matrix` | 3 vendors × 4 criteria (harvey), `recommended`, `decisive_criterion: Migration risk`, a `detail` per vendor | Scores 0–4; the recommended row is badged. |
| 7 | `regions` `main_left` | 2-series bar (current vs target, 5 years) + `stat` (saving) + 3-row `table` (TCO) | Table region ≤ 4 columns, ≤ 5 rows. |
| 8 | `chart_insight` | Donut of cost drivers + 3 `insights` | Four slices, named; percentages in the labels. |
| 9 | `roadmap` | Foundation / Migrate / Optimise as `phases` with `milestone`s, `parallel_tracks` for literacy and FinOps, a `takeaway` | Tracks are one-line bars; on `modern-template` — the shortest content area — the rows give up padding, never type size, so three phases, milestones, two tracks and the takeaway all fit. Keep descriptions to two lines (about 70 characters at three phases). |
| 10 | `comparison` | Two columns, four aligned rows, `connectors: true`, `highlight_column: right` | The improvement is visible as a chevron per row and a filled target column. |
| 11 | `decision` | Do nothing / lakehouse / alternatives, one `recommended: true`, `recommendation` with the amount | Always include "do nothing" with its cost. |
| 12 | `next_steps` | Contract, landing zone, pilot sources, literacy programme — owner and date each | Dates before the Foundation phase starts. |

## The technology-specific visuals

**Architecture with rails** — the kind draws it; keep tier names short and
put the concern that spans every layer in `rails`, never as a bottom tier.

**Before / after with connectors.** The `comparison` kind aligns the rows;
`connectors: true` draws the per-row today → target chevron and
`highlight_column: right` fills the target column:

```yaml
meta:
  title: Atlas Retail data platform
  template: modern-template
  date: October 2026
  source: Atlas Retail data operations logs; target KPIs from the business case model
slides:
  - kind: comparison
    title: "Four KPIs move: the load window drops from 9.5 h to 1.5 h"
    columns:
      - header: Today
        items:
          - "Nightly load window: 9.5 h"
          - "SLA misses: 61 per 90 days"
          - "Product-record defects: 23%"
          - "Reconciliation effort: 60% of 4 FTE"
      - header: Target after migration
        items:
          - "Load window: 1.5 h"
          - "SLA misses: under 3 per 90 days"
          - "Product-record defects: under 5%"
          - "Reconciliation effort: -70%"
    connectors: true
    highlight_column: right
    takeaway: The migration is held to these four numbers at the Q3 2028 re-test.
```

**TCO on one slide** — the three-region slide in
[split-and-complex-layouts.md](split-and-complex-layouts.md) §3. The number
beside the chart is the saving; the table is what the CFO will check.

**Screenshot with callouts** — §6 there. For data-platform decks the
screenshot is often the strongest slide: it shows the audience what the
operations team sees. Keep the text to the fact the picture proves.

**Build and run as one loop.** When the operating model is two loops that
feed each other (build ↔ run, plan ↔ deliver), the `cycle` kind with
`style: figure_eight` draws one path round both; it needs the slide's full
width, so use `style: ring` in a half-width region. Name the hand-over phase in
`highlight`:

```yaml
meta:
  title: Atlas Retail data platform
  template: modern-template
  date: October 2026
slides:
  - kind: cycle
    style: figure_eight
    title: Release is where the build loop hands over to the run loop
    left_label: Build
    right_label: Run
    phases:
      - label: Plan
        description: Rank the backlog by customer value
      - label: Code
        description: Small changes, reviewed in a day
      - label: Test
        description: Automated checks gate every merge
      - label: Release
        description: Ship behind a feature flag
      - label: Operate
        description: On-call owns the service level
      - label: Monitor
        description: Usage and incidents feed the plan
    highlight: Release
```

## Variants

| Engagement | Spine changes |
|---|---|
| Architecture review / technical debt | Current `architecture` → issues `option_matrix` (`rag` severity, text effort) → target `architecture` → `comparison` current vs target → `roadmap`. |
| Cloud migration | Landscape `kpi_snapshot` (apps, servers, spend) → waves `roadmap` + parallel tracks → run cost `regions` (bar + stat + table) → risks `option_matrix` (`rag`) → `decision` on wave 1. |
| Data governance / data quality | Defect KPIs `kpi_snapshot` → operating model `pillars` (ownership, standards, tooling, culture) → roles `org` → quick wins `next_steps`. |
| AI readiness / use-case prioritisation | Use cases `matrix_2x2` (value × feasibility, `highlight` the first wave) → platform `architecture` → `roadmap` → `decision`. |
| Vendor selection | Requirements `table` → `option_matrix` (criteria weighted in the title) → TCO `regions` → `decision`. |

## Review points for this audience

1. Every cost figure on slides 7–8 traces to the model named in `source`;
   the five-year total appears in the executive summary.
2. The architecture names real components, not categories ("Power BI", not
   "BI tool").
3. The "do nothing" option is on the decision slide with its run-rate cost.
4. KPI targets (slide 10) reappear in `next_steps` or the roadmap as the
   re-test point.
5. `modern-template` sets titles in capitals: 60 characters already wrap to
   two lines; keep titles tight and check the p-style render for the serif
   wrap.
