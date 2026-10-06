# Visual vocabulary: message → kind

Pick the visual from the message, never the other way round. The left column
is the sentence you are trying to make the audience accept; the right column
is how json2pptx draws it on the DeckSpec path (kinds are live:
`list_slide_kinds`; a few consulting staples are still raw-path patterns and
say so). Back to the [hub](README.md); the engine's own catalog with budgets
is [`DECKSPEC.md`](../../skills/generate-deck/DECKSPEC.md); the per-slide
recipes are in [split-and-complex-layouts.md](split-and-complex-layouts.md).

## General

| The message is… | Kind | Notes |
|---|---|---|
| "Here is the answer and the three reasons" | `executive_summary` | 3–5 `{lead, support}`, `bottom_line` is the ask. |
| "The situation in numbers" | `kpi_snapshot` (2–6) or `stat` (one) | Give each KPI its `comparator` ("vs plan +4 pts"). |
| "X grew / fell / crossed a line" | `chart_insight` (chart + `insights[]`) or `regions` when the number sits beside it | Bar for discrete periods, line for 6+ points, waterfall for walks; `highlight` the bar the title argues. |
| "Here is how the pieces fit" | `architecture` (tiers + `rails`), `pillars` (3–5; a house with `objective` + `foundation`), `framework` (SWOT, Five Forces, BMC) | Draw every canonical part or the kind degrades to bullets. |
| "A beats B on these criteria" | `option_matrix` (criteria × options, one scale per criterion) | `recommended`, `decisive_criterion`; `rag` for status, `harvey` for fit. |
| "We recommend option 2" | `decision` (3–6 options) | One `recommended: true`, the ask in `recommendation`. |
| "Today vs target" | `comparison` (2 aligned columns) | Rows correspond; keep it to ~6. |
| "This is the sequence" | `process` (3–6 steps with descriptions → numbered rows; 7–8 or bare labels → flow boxes) | A straight sequence, not a branching flowchart. |
| "It repeats: the last phase leads back to the first" | raw `cycle-ring` (4–8 phases) | A lifecycle, PDCA, an operating rhythm, a flywheel. One `highlight` phase at most; `style: arrows` for chasing arrows. Not for a sequence that runs once (use `process`); 3 phases or stations joined by arrows are `cycle-nodes`. |
| "Two loops feed each other" (build ↔ run, plan ↔ deliver) | raw `cycle-figure-eight` (4–8 phases, 2–4 per lobe) | One path round both lobes, numbered along the way; `left_label` / `right_label` name the loops. Needs the slide's width: in a half-width segment it is refused — use `cycle-ring` there, and for one loop. |
| "The work repeats in a loop" | `cycle-nodes` (raw path; 3–8 steps as numbered circles joined by arrows) | Use when the steps return to the start and the hand-offs matter (plan-do-check-act, a feedback loop). Avoid for a sequence that ends — that is `process`. Prefer `cycle-ring` when the phases are one continuous ring. `highlight` one step; in a split half the labels become a legend; a step's `icon` replaces the number in its circle. Pending the `cycle` kind (go-slide-creator-53v5u). |
| "A few one-off steps lead into a cycle that then repeats" | raw `cycle-intake` (1–3 intake steps, 3–8 loop phases) | Onboarding then the service cycle, deal intake then the portfolio review, ingest then the model loop. Intake arrows on the left, an accent arrow into the ring, the phases in one numbered list on the right; one `highlight` phase; `loop_style: nodes` for circles joined by arrows. Not when nothing feeds the loop (`cycle-ring`, `cycle-nodes`) or when the steps never loop back (`process`). With 3 intake steps keep loop labels to one line (22) and, from 6 phases, descriptions to about 22 characters; 8 phases are labels only on a short body. Pending the `cycle` kind (go-slide-creator-53v5u). |
| "When things happen" | `timeline` (3–7 dated stops) · `roadmap` (3–6 phases, 0–4 `parallel_tracks`) | An `end_date` turns a timeline into bars. |
| "Who does what by when" | `next_steps` | 2–6 actions `{action, owner, date}`, 0–3 `decisions`. Always the closer. |
| "Who we are" | `team` (1–8, `photo` or initials `photo_label`) | Role on every card. |
| "What a stakeholder said" | `quote` (1 → pull quote, 3–8 → cluster) | Two quotes degrade to bullets. |
| "Look at this" | `image_case` (picture + body + `callouts`) | For a comparable, a screenshot, a site. |
| "The numbers in full" | `table` (≤ 6 columns, ≤ 9 data rows) | Right-align numbers; `totals_row: true`; backup detail to an appendix. |
| "The walk from A to B" | `bridge` (3–10 columns) or a waterfall `chart` region beside text | Totals must sum (0.5% tolerance). |
| "These layers contain one another" (core → adjacent → ecosystem) | raw `concentric-rings` (3–5 layers, inner → outer) | An onion with a side ladder of labels; one solid ring (`highlight`). Not for a ranked hierarchy (raw `pyramid`), a centre with satellites (`radial-hub`) or a loop (`cycle-ring`). Raw path: carry it as a `raw_json2pptx` slide. |
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

`labeled-rows`, `capability-heatmap`, `roadmap-phased` (dated bars per
workstream), `swimlane`, `value-chain`, `scqa-summary`,
`driver-tree`, `journey-maturity-model`, `cycle-ring`, `cycle-nodes`, `cycle-figure-eight`, `cycle-intake`, `exec-summary` variants, and the
svggen diagrams (gantt, venn, org chart). Carry them as a `raw_json2pptx`
slide inside the DeckSpec — the whole pattern block verbatim from
`show_pattern` — not as a lowered raw deck; `recommend_visual` returns such a
slide ready to run for any pattern or diagram candidate.

`radial-hub` (raw path) — "everything relates to the centre": one hub with
4–8 unordered peers around it (a platform and its capabilities, an
operating-model hub, a stakeholder or ecosystem map). Short labels, at most a
one-line description each; `highlight` one spoke when the title argues it;
give each spoke an `icon` (a bundled name) so the discs are not empty.
Avoid it when the items follow one another (use a `process`, or the cycle
patterns), when they are today/future pairs (`state-shift-hub`) and when there
is no centre (`pillars`, a card grid). In a half-width split it falls back to
a keyed legend; give it 60% of the width or short labels with
`labels: "inside"`.
