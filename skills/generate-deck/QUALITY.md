# Consulting-quality storyline

Read this before writing any slide. The engine checks fit, contrast and
structure; it cannot tell whether the deck says anything. That part is yours.

## 1. Write the ghost deck first

Write the ghost deck: slide titles in order. They must carry the argument
alone: situation → complication → resolution → evidence → plan → ask.
Cut titles that add nothing; move backup detail to an appendix.

## 2. Every title is an action title

An action title is a full-sentence conclusion, ≤15 words, carrying the
number when one exists and specific to this deck.

| Topic title (don't) | Action title (do) |
|---|---|
| Revenue Trajectory | Revenue grew 41% this year, and the EMEA launch drove the Q4 step-up |
| Market Overview | A $180B market is shifting to managed AI infrastructure |
| Options | Funding an SMB success pod is the cheapest way to protect retention |
| Next Steps | Approve the pilot budget by 15 March to launch in Q3 |

Cover, section and closing may use labels; content needs action titles.
A takeaway does not excuse a topic title such as "Monthly spend trend".

## 3. One message per slide

The body proves one message. Two conclusions need two slides; several views
may prove one title. Cut generic or repeated bullets; order by importance.

## 4. Evidence slides say "so what"

Chart, table, matrix, bridge and KPI slides set `takeaway`: one sentence on
what the evidence means or what to do about it — not a restatement of the
title. Label the measure and unit on every chart (RULES.md 10a). Highlight
the one bar, row, cell, line or slice the title is about; leave the rest
neutral (`data.highlight` — series names on line / area / grouped bar,
slice names on pie / donut; RULES.md 10e–10f).

## 5. Source every number

Set `source` (who, what, when), period and unit on data slides
(`DATA_WITHOUT_SOURCE`). Never invent figures or sources; label estimates
"Illustrative". Deck defaults: `meta.source` / raw `source`; chart `footnote`
also counts. Sources render once in the 9pt source zone above the footer.

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
| where risks sit by likelihood × impact | `risk_heatmap` (a "medium" has its own cell) |
| the risks and how they are mitigated | `table` (Risk · Likelihood · Impact · Mitigation · Owner), not a card per risk |
| a reporting or governance structure | `org` |
| who is on the team | `team` |
| what the deck covers | `agenda` (or `structure.auto_agenda`) |
| one customer story or case | `image_case` |
| a structure of parts (SWOT, BMC, pillars) | `framework` / `pillars` |
| what people said | `quote` |
| the whole answer up front | `executive_summary` |

Do not choose a visual because it looks varied; a card grid of topics proves
nothing. Rhythm tools come after the message is right.

## 7. Before you render: self-check

- The ghost deck reads as one argument; no two titles make the same point.
- Each body proves its title; evidence slides have a `takeaway` and a `source`.
- The executive summary's points match the section titles that follow.
- The closing states the decision or next step, with owner and date
  (`next_steps` kind / `next-steps` pattern, not "Thank you").
- Content slides carry a page number and the deck date (on by default; set
  `meta.date`). No stat-hero or pull-quote added just to vary the rhythm.
