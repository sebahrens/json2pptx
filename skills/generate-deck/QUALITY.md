# Consulting-quality storyline

Read this before writing any slide. The engine checks fit, contrast and
structure; it cannot tell whether the deck says anything. That part is yours.

## 1. Write the ghost deck first

List only the slide titles, in order, before choosing a single visual. Read
them top to bottom as if they were the whole deck: an executive who reads
nothing else must get the argument. A good sequence runs situation →
complication → resolution, then evidence, plan and the ask. If a title does
not move the argument forward, cut the slide or fold it into another; put
backup material in an appendix section, not in the storyline.

## 2. Every title is an action title

A title states the conclusion the slide proves, as a full sentence:

- at most 15 words, so it holds on two lines;
- carries the number when there is one ("… grew 18% …", not "… grew …");
- is specific to this deck — it would be false in someone else's.

| Topic title (don't) | Action title (do) |
|---|---|
| Revenue Trajectory | Revenue grew 41% this year, and the EMEA launch drove the Q4 step-up |
| Market Overview | A $180B market is shifting to managed AI infrastructure |
| Options | Funding an SMB success pod is the cheapest way to protect retention |
| Next Steps | Approve the pilot budget by 15 March to launch in Q3 |

Cover, section and closing slides may keep short labels; every content slide
gets an action title. If you cannot write one, the slide has no message yet.

## 3. One message per slide

The body proves the title and nothing else. Two conclusions mean two slides.
Delete bullets that restate the title, repeat another slide, or would be true
of any company. Order supporting points by importance, not by discovery.

## 4. Evidence slides say "so what"

Chart, table, matrix, bridge and KPI slides set `takeaway`: one sentence on
what the evidence means or what to do about it — not a restatement of the
title. Label the measure and unit on every chart (RULES.md 10a). Highlight
the one bar, row, cell, line or slice the title is about; leave the rest
neutral (`data.highlight` — series names on line / area / grouped bar,
slice names on pie / donut; RULES.md 10e–10f).

## 5. Source every number

Any slide with data sets `source` (who, what, when: "Company filings, FY24").
State the period and unit. Never invent a figure, unit or source to fill a
visual; if the brief has no number, write a qualitative title instead.
A chart, a table of figures, or a chart / KPI / stat pattern without one
draws `DATA_WITHOUT_SOURCE` (review weight); for estimates, write
"Illustrative" rather than leaving it empty. When one data set feeds the
whole deck, set it once: DeckSpec `meta.source`, raw top-level `source`. A
`chart_value` `footnote` (rendered at the chart's bottom) counts too. Every source renders once, in
the 9pt source zone just above the footer, on the content's left edge — a
`chart-insights-split` or `stat-hero` `values.source` is moved there too.

## 6. Choose the visual from the message

Pick the DeckSpec kind that matches what the title claims, then confirm with
`list_slide_kinds` (and `recommend_visual` on the raw path):

| The title claims… | Use |
|---|---|
| a change over time | `chart_insight` (line or bar) |
| how parts add up to a total, or a walk from A to B | `bridge` |
| a ranking or a comparison of values | `chart_insight` (bar) |
| one headline figure | `stat` |
| several figures of equal weight | `kpi_snapshot` |
| two states or options side by side | `comparison` |
| options scored against criteria | `option_matrix` |
| a recommendation among options | `decision` |
| a sequence of steps | `process` |
| a plan over time | `roadmap` (phases) or `timeline` (dated milestones) |
| a positioning on two dimensions | `matrix_2x2` |
| exact figures: financials, P&L, pricing, a segment split | `table` (`totals_row` for the total line, units in the header) |
| where risks sit by likelihood × impact | `matrix_2x2` (axes Likelihood / Impact, each quadrant listing its risks) |
| the risks and how they are mitigated | `table` (Risk · Likelihood · Impact · Mitigation · Owner), not a card per risk |
| a reporting or governance structure | `org` |
| who is on the team | `team` |
| what the deck covers | `agenda` (or `structure.auto_agenda`) |
| one customer story or case | `image_case` |
| a structure of parts (SWOT, BMC, pillars) | `framework` / `pillars` |
| what people said | `quote` |
| the whole answer up front | `executive_summary` |

Back matter goes in an appendix: `structure.sections[].appendix: true` (or a
`section` slide with `appendix: true`) — the divider is unnumbered and left
out of the agenda. A divider titled Appendix / Backup / Q&A is unnumbered
automatically.

Do not choose a visual because it looks varied; a card grid of topics proves
nothing. Rhythm tools come after the message is right.

## 7. Before you render: self-check

- The ghost deck reads as one argument; no two titles make the same point.
- Every content title is a full sentence of ≤15 words carrying its number.
- Each body proves its title; evidence slides have a `takeaway` and a `source`.
- The executive summary's points match the section titles that follow.
- The closing states the decision or next step, with owner and date
  (`next_steps` kind / `next-steps` pattern, not "Thank you").
- Content slides carry a page number and the deck date (on by default; set
  `meta.date`). No stat-hero or pull-quote added just to vary the rhythm.
