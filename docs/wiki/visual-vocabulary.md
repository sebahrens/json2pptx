---
title: "Visual vocabulary: message \u2192 kind"
type: documentation
summary: "json2pptx guide to visual vocabulary"
status: active
last_updated: "2026-10-25"
tags: [json2pptx, deck-authoring]
---

## Visual vocabulary: message → kind

Pick the visual from the message, never the other way round. The left column
is the sentence you are trying to make the audience accept; the right column
is how json2pptx draws it on the DeckSpec path (kinds are live:
`list_slide_kinds`; a few consulting staples are still raw-path patterns and
say so). Back to the [hub](README.md); the engine's own catalog with budgets
is [`DECKSPEC.md`](https://github.com/sebahrens/json2pptx/blob/main/skills/generate-deck/DECKSPEC.md); the per-slide
recipes are in [split-and-complex-layouts.md](split-and-complex-layouts.md),
and the full menu with every form, pattern, chart and diagram is
[layout-catalog.md](layout-catalog.md).

Repeated `recommend_visual` calls and candidate shortlists retain each
composition's score, rationale and region order for the same intent and
layout context. Equal-score pairs have a deterministic order; declared
body/band placement preferences still apply.

## Inline evidence and footnote markers

Raw `shape_grid` text supports `<b>bold</b>`, `<i>italic</i>`,
`<u>underline</u>`, `<sup>superscript</sup>` and `<sub>subscript</sub>` in
each `content` string, including paragraphs. Tags may be nested; other HTML
is not supported. Use emphasis for the evidence words in a quote, and a
superscript number to connect those words to a nearby fact or source card.
The marker is visual text: it does not create a footnote link automatically.

This complete DeckSpec keeps a quoted message on the left and its numbered
evidence on the right. The runs remain editable in PowerPoint.

```yaml
meta:
  title: Evidence behind the message
  template: midnight-blue
  design_mode: free
  source: Pilot review, October 2026
slides:
  - kind: raw_json2pptx
    slide:
      slide_type: content
      layout_id: blank-title
      content:
        - placeholder_id: title
          type: text
          text_value: Every pilot case needs an accountable owner
      shape_grid:
        columns: [60, 40]
        vertical_align: auto
        rows:
          - min_height: 220
            max_height: 220
            cells:
              - shape:
                  geometry: rect
                  text:
                    content: '"We need <b>one owner</b><sup>1</sup> for each case, with an <i>independent review</i><sup>2</sup> before closure."'
                    size: 18
              - shape:
                  geometry: rect
                  text:
                    paragraphs:
                      - content: '1. One accountable owner'
                        bold: true
                        size: 14
                      - content: 'The intake register assigns a named owner to every case.'
                        size: 12
                      - content: '2. Independent review'
                        bold: true
                        size: 14
                      - content: 'A second reviewer records the decision before closure.'
                        size: 12
```

## General

| The message is… | Kind | Notes |
|---|---|---|
| "Here is the answer and the three reasons" | `executive_summary` | 3–5 `{lead, support}`, `bottom_line` is the ask. |
| "The situation in numbers" | `kpi_snapshot` (2–6) or `stat` (one) | Give each KPI its `comparator` ("vs plan +4 pts"). |
| "X grew / fell / crossed a line" | `chart_insight` (chart + `insights[]`) or `regions` when the number sits beside it | Bar for discrete periods, line for 6+ points, waterfall for walks; `highlight` the bar the title argues. |
| "Before vs after, one measure" | `stat` ("6.6 → 0.9", label "wrong per 100"), or `regions` with the `stat` beside a two-bar `chart` only when the bars differ visibly | Two bars of 93 and 99 at a zero baseline look equal; let the number carry it, or plot the complement ([slide-design-rules.md §4](slide-design-rules.md#4-text-chart-or-diagram)). |
| "Here is how the pieces fit" | `architecture` (tiers + `rails`), `pillars` (3–5; a house with `objective` + `foundation`), `framework` (SWOT, Five Forces, BMC) | Draw every canonical part or the kind degrades to bullets. |
| "A beats B on these criteria" | `option_matrix` (criteria × options, one scale per criterion) | `recommended`, `decisive_criterion`; `rag` for status, `harvey` for fit. |
| "We recommend option 2" | `decision` (3–6 options as numbered boxes; exactly 2, or 7–12, each with a `detail`, as cards) | One `recommended: true`, the ask in `recommendation`. |
| "Today vs target" | `comparison` (2 aligned columns; 3–5 columns → titled panels; 6–12 → cards) | Rows correspond; keep it to ~6; `connectors: true` for the per-row arrow. |
| "The chapters, and where we are" | `agenda` (2–10 `sections`, `current`) | A real contents page; pair it with `structure.sections` on 12+ slides. |
| "This is the sequence" | `process` (3–6 steps with descriptions → numbered rows; 7–8 or bare labels → flow chevrons; a step with `type: decision` → a diamond) | Beside a stat or text, a `process` region takes 3–6 numbered steps. A standalone process has one path with at most a yes / no gate, not a branching flowchart; actors in lanes → raw `swimlane`. |
| "It repeats", "everything relates to the centre", "these contain one another" | `cycle` (3–8 `phases`, one `style`) | One rule picks the style: one ordered loop → `ring` (default; 4–8 segments) or `nodes` (3–8 circles joined by arrows, when the hand-offs matter); one-off steps that feed the loop → `intake` (1–3 `intake` steps); two loops that feed each other, on a full-width slide → `figure_eight` (`left_label` / `right_label`); unordered peers around a centre → `radial` (`center` is the hub); things that contain one another → `concentric` (3–5, innermost first). One `highlight` phase; `ring` / `figure_eight` also take per-phase `status: done` or `planned` (dashed outline), independently of highlight. Not for a sequence that runs once (`process`) or a ranked hierarchy (`pyramid`). See "The cycle family" below. |
| "When things happen" | `timeline` (3–7 dated stops) · `roadmap` (3–6 phases, 0–4 `parallel_tracks`) | An `end_date` turns a timeline into bars. |
| "Who does what by when" | `next_steps` | 2–6 actions `{action, owner, date}`, 0–3 `decisions`. Always the closer. |
| "Who we are" | `team` (1–8, `photo` or initials `photo_label`) | Role on every card. |
| "What a stakeholder said" | `quote` (1 → pull quote, 2–8 → cluster) | Two voices use paired columns; missing names degrade to bullets. |
| "Look at this" | `image_case` (picture + body + `callouts`) | For a comparable, a screenshot, a site. |
| "Use this complete SVG or PNG as the slide" | Raw `PresentationInput`: `blank` + one-cell `shape_grid` | Explicit bounds `0, 0, 100, 100`; nested `image.fit: "contain"`; leave cell `fit` unset. [Full-slide recipe](slide-estate-and-segments.md#7-a-complete-slide-supplied-as-svg-or-png). |
| "The numbers in full" | `table` (≤ 6 columns, ≤ 9 data rows) | Right-align numbers; `column_widths: [1, 3]` gives glossary definitions more room; `totals_row: true`; backup detail to an appendix. |
| "The walk from A to B" | `bridge` (3–10 columns) or a waterfall `chart` region beside text | Totals must sum (0.5% tolerance). |
| "Where things sit on two axes" | `matrix_2x2` | Four named quadrants; both axes labelled. |
| "The deck's chapters" | `structure.sections` + `auto_agenda` (12+ slides) | Never hand-number dividers. |

## Consulting pitch and deals

| Message | Kind / pattern | Example title |
|---|---|---|
| Market size and the target's share | `regions` `main_left`: bar chart + `stat` + `text` | "The market adds EUR 1.2bn by 2028, but Nordbolt holds only 2.3% of it" |
| What drove the margin | `regions` `columns`: waterfall chart + `text` | "EUR 6.5M of the EUR 7.0M EBITDA gain since 2023 is price" |
| Why us: the comparable | `image_case` with 2–3 `metrics` | "On Project Keel the same test moved the bid by EUR 40M" |
| Scope options | `option_matrix` (harvey) then `comparison` for the two finalists | "Scope B is the only option that prices both risks inside the window" |
| Workplan | `timeline` (weeks) or `roadmap` (phases) | "Four weeks from data room to IC pack" |
| Fees | `table` with `totals_row` | "Eight deliverables over four weeks make up the EUR 320k fee" |
| Team | `team` | "A team that has done three fastener diligences before" |
| Competitive position | `matrix_2x2` or `chart_insight` (horizontal bar of shares) | "Three players hold 20% of a fragmented market" |
| Investment case (IC paper) | `kpi_snapshot` (entry multiple, IRR, MOIC) → `bridge` (value creation) → `option_matrix` (exit routes) | — |

## Risk consulting

| Message | Kind / pattern | Example title |
|---|---|---|
| Regulatory findings in numbers | `kpi_snapshot` (findings by severity, deadline) | "14 regulatory findings must be closed in nine months" |
| Risk appetite dashboard | `option_matrix` with `rag` + `text` criteria | "Two of five risk appetite metrics are breached, one is amber" |
| Loss trend and its cause | `regions` `main_left`: bar + `stat` + `text` | "Operational losses rose 84% in two years to EUR 11.2M" |
| Likelihood × impact with named risks | `risk_heatmap` (3 × 3, or `size: 5`) | "Cyber is the only high-likelihood, high-impact risk" |
| Three lines of defence today vs target | `comparison` | "The target model adds 16 FTE to the 2nd line and one integrated report" |
| Target operating model | `pillars` with `objective` and `foundation` (the house) | "Four pillars take risk governance from 'needs improvement' to effective" |
| Delivery options | `decision` (advisory / co-delivery / outsource) | "Co-delivery at EUR 2.1M balances speed, cost and capability transfer" |
| Programme plan with parallel tracks | `roadmap` with `parallel_tracks` | "Four phases and two parallel tracks finish before the deadline" |
| Control maturity by domain | `chart_insight` grouped horizontal bar (today vs target) or raw `journey-maturity-model` for one ladder | "Access and third-party controls must climb two maturity levels by FY28" |

## Technology and data

| Message | Kind / pattern | Example title |
|---|---|---|
| The platform is failing quietly | `kpi_snapshot` (SLA misses, load window, defect rate) | "The 2009 warehouse missed its 6 am SLA on 61 of the last 90 days" |
| Evidence on the screen | `image_case` with `callouts` | "The ops console shows the breach" |
| Target architecture | `architecture` with `rails` | "Five tiers and two rails make up the target lakehouse" |
| Vendor selection | `option_matrix` (harvey, `decisive_criterion`) | "Databricks scores best on migration risk and skills availability" |
| TCO / run cost | `regions` `main_left`: 2-series bar + `stat` + 3-row `table` | "Run cost falls from EUR 3.6M to 1.5M a year" |
| Where the cost goes | `chart_insight` donut + `insights[]` | "Compute is 48% of run cost, so FinOps guardrails protect the saving" |
| Migration plan | `roadmap` with `milestone`s and `parallel_tracks` | "Three phases decommission the warehouse by the end of 2028" |
| KPI before / after | `comparison` with `connectors: true`, `highlight_column: right` | "Four KPIs move: the load window drops from 9.5 h to 1.5 h" |
| Data flow / process | `process` (≤ 8 steps) or svggen `swimlane` (raw) | — |
| Delivery governance | `org` (≤ 7 nodes) | — |

## Risk assurance (internal audit, SOX, ITGC)

| Message | Kind / pattern | Example title |
|---|---|---|
| Scope and approach | `process` (plan → walkthroughs → design → testing → reporting) | "The review followed five steps from scoping to reporting" |
| Results by domain | `option_matrix`: `text` criteria for counts, `rag` for the rating | "Two of five domains are rated red" |
| Trend and the opinion | `regions` `main_left`: line chart + `stat` ("Partially effective") + `text` | "Exceptions fell for a third year, but the opinion stays partially effective" |
| Findings register | `table` (title, severity, owner, due date; ≤ 9 rows per slide) | "All seven findings have an owner and a date" |
| One finding in full | raw `labeled-rows` (what / why / action / owner-date) | "High: privileged access was not recertified for 3 of 9 applications" |
| Maturity today vs target | `chart_insight` grouped horizontal bar | "Access and third-party controls must climb two maturity levels" |
| Remediation plan | `roadmap` (quick wins → tooling → re-test) | "Remediation runs in three phases and is re-tested in Q3 2027" |
| The committee's ask | `next_steps` with `decisions` | "Four actions start in October; two decisions are requested today" |
| Testing statistics | appendix `table` | "Appendix: testing statistics by domain" |

## Chart choice in one breath

Discrete periods → bar (one series; accent the last or the argued bar). Many
points or several series over time → line. Parts of a whole (3–6 parts) → donut
or stacked bar; never a pie with more than six slices. A walk between two
totals → waterfall. Ranked things → horizontal bar. Two measures per item →
scatter only when there are 8+ items; otherwise a table. Small multiples when
one measure across 2–6 groups shares the periods. Every chart: unit in the
heading, source on the slide, no legend when one series is labelled.

## Patterns that still need the raw path

Patterns without a kind include: `labeled-rows` (one structured finding),
`scqa-summary`, `text-sidebar` (a foreword page), `before-after` and
`before-after-compact` (two states with a chevron), `framework-grid` (levers
by dimension), `capability-heatmap`, `driver-tree`, `horizontal-bar-with-callouts` (ranked bars, one insight each),
`hero-detail`, `metric-list`, `kpi-inline`, `process-grid-2row` (two lanes),
`swimlane`, `roadmap-phased` (dated bars per workstream),
`journey-maturity-model`, `state-shift-hub`, `dual-org-ladder` (engagement /
JV team), `contact-directory`, `icon-row`. Some diagrams (such as gantt) also need
raw hosting; venn, fishbone, PESTEL, nine-box, heatmap, value-chain and pyramid
have semantic hosts, listed in [the additions catalog](consulting-visual-additions.md). Each is a row
in [layout-catalog.md](layout-catalog.md) §3. Carry them as a
`raw_json2pptx` slide inside the DeckSpec — the whole pattern block verbatim
from `show_pattern` — not as a lowered raw deck; `recommend_visual` returns
such a slide ready to run for any pattern or diagram candidate. (`exec-summary`
is *not* raw-only: `executive_summary` compiles to it, and
`overrides.takeaway_emphasis` is reachable through the kind's `pattern`
override.)

Three more messages the kinds cover and agents under-use: "the chapters,
with where we are" → `agenda` (`sections`, `current`); "A or B" → `decision`
with exactly two detailed options (a pair of cards); "the flow has one
yes / no gate" → `process` with a step of `type: decision` (the diamond in
the flow). The forms every kind can take are in
[layout-catalog.md](layout-catalog.md) §2.

## The cycle family

`cycle` draws six pictures from one payload — `phases` in order, an optional
`center`, the one phase to `highlight` — and `style` names which:

| The content is | `style` | Count | Notes |
|---|---|---|---|
| One loop: the last phase leads back to the first (lifecycle, PDCA, operating rhythm, flywheel) | `ring` (default) | 4–8 | Ring segments with numbered badges; `center` names the loop. Three phases are drawn as `nodes`. |
| One loop whose hand-offs matter | `nodes` | 3–8 | Circles joined by arrows; a one-line `center`. |
| One-off steps, then a loop (onboarding then the service cycle) | `intake` | 1–3 `intake` + 3–8 | Naming `intake` steps selects it. In a narrow region the lane stacks above the ring. |
| Two loops that feed each other (build ↔ run, plan ↔ deliver) | `figure_eight` | 4–8, 2–4 per loop | `left_label` / `right_label` title the loops. Needs the full slide width: in a side-by-side region it is refused — use `ring` there. |
| Unordered peers around one idea (hub and spoke, ecosystem) | `radial` | 4–8 | `center` is required: it is the hub. |
| Things that contain one another (core → adjacent → ecosystem) | `concentric` | 3–5 | Innermost first; the core takes the accent unless `highlight` names a layer. |

Outside a style's count or text budgets the slide degrades to a numbered list
and the finding at `phases` says where the content belongs. A field a style
does not draw (`center` on a figure eight, `left_label` on a ring) is an error
rather than a silent drop. Avoid the family when the items follow one another
once (`process`), when they are today/future pairs (raw `state-shift-hub`)
and when there is neither a loop nor a centre (`pillars`, `comparison`). Beside
a narrative use a `cycle` region (split layouts §13).

## Additional consulting forms

Use [Consulting visual additions](consulting-visual-additions.md) for RACI,
status boards, issue/decision trees, stakeholder placement, progress against
target, Mekko, combination and butterfly charts. Prefer semantic kinds when
listed there; inspect `list_slide_kinds` or `show_pattern` for exact fields.
For two or four titled groups side by side, use `comparison-2col` with two
headers and aligned group rows; `bullet_groups` remains a single-placeholder
stack.
