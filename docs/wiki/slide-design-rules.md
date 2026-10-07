# Slide design rules: what a partner-level reviewer checks

The craft rules the strategy houses and the Big Four teach their consultants,
stated as tests an agent can run on its own deck, and tied to the engine
feature or finding that enforces (or cannot enforce) each. Back to the
[hub](README.md); the argument-level rules are in
[storyline-and-structure.md](storyline-and-structure.md), the skeletons in
[deck-archetypes.md](deck-archetypes.md), the ten-point render rubric in
[quality-and-review.md](quality-and-review.md).

## 1. The argument: three tests from the Pyramid Principle

**The governing thought.** The deck has exactly one: a sentence someone could
disagree with, not a topic. "Options for the northern site" is a topic;
"Allocate the northern site and drop the extension" is a governing thought. It
is the `executive_summary`'s `bottom_line` and the cover's reason to exist.

**Vertical logic.** Read the deck top-down asking *why?* or *how?* at each
step: the executive summary's `lead` raises the question, the body slide under
it answers, the slide's exhibit proves the slide's title. Where the chain
breaks, either evidence is missing or the slide does not belong. The engine
checks the mechanics (`TITLE_NOT_ACTION`, `takeaway_missing`,
`NO_EXECUTIVE_SUMMARY`), not the logic; `explain_deck_spec` lists every
slide's `title`, `takeaway` and `role` in order so you can read the chain
without rendering.

**Horizontal logic.** Ideas grouped under one parent must be the same kind of
thing, in a deliberate order (time, structure or importance), and summarised
by the parent — MECE: no overlap, no gap, no "other" larger than a named
bucket. Groups of three to five. The body slides under one `executive_summary`
point are such a group; so are the `pillars`, the `options`, the `criteria`.

**The so-what test.** Ask "so what?" of every title until the answer is an
implication for the decision. A title that still permits a further "so what"
is a finding, not an insight: "Losses rose 84%" → "Losses rose 84% and now
exceed the appetite, so the control model must change this year".

## 2. The storyboard before the slides

**Dot-dash.** Round bullets are the top-level ideas — each becomes one slide's
action title; dashes under them are the supporting points — the slide's body
or exhibit. Review the dot-dash with the person steering you before any slide
exists; it is the cheapest point to learn the deck is wrong. In the product:
`plan_deck(format:"deckspec")` returns the slots with `guidance` and
`facts[]`; rewrite its `__FILL__` titles into the ghost deck, read them as
one paragraph, and only then fill the bodies.

**Answer first, unless.** Default to the pyramid (answer on slide two). Build
up (situation → complication → evidence → answer) only when the audience will
reject the answer unheard, when the finding itself is the news (an audit, a
diagnostic), or when the answer is not yet known (a workshop pre-read). Even
then the `executive_summary` leads with the answer; it is the body that walks.

**The executive summary mirrors the deck.** 3–5 `points`, each `lead` a bold
claim that is also the title of the body section it stands for, in the same
order; `support` carries the number. Skimming the leads alone must give the
whole argument. Write it last, test it first.

**The closer is a decision, not a courtesy.** `next_steps` with the ask in
`decisions`, each action with owner and date; the consequence of not acting
belongs in the title ("Approve the EUR 2.4M reallocation by 1 August or the
Q4 saving slips to FY28"). `CLOSING_WITHOUT_NEXT_STEPS` catches the
"Thank you" slide; nothing catches a vague ask — write the decision as a
sentence the chair can read out.

**Appendix discipline.** A slide goes to the appendix when removing it leaves
the title storyline intact but you would still need it to defend a number. It
keeps an action title and a source, and the body slide that leans on it says
so in its `source` or `takeaway` ("detail in A3"). `section` with
`appendix: true` numbers the pages A1, A2 and keeps them out of the rhythm
checks and the `{total}`.

## 3. The slide: title – exhibit – takeaway

| Rule | Test | In the product |
|---|---|---|
| One message per slide | Can you write a single sentence both halves prove? If not, two slides | `regions` / `compose` carry one `title` and one `takeaway` by design |
| Action title | A full sentence, active, with the number where one exists, ≤ two lines; states what the exhibit proves, not what it is about; nothing in the title that is not on the slide | `TITLE_NOT_ACTION` with `reason` (`topic_label`, `stock_label`, `no_verb_or_number`, `too_long`); budgets from `list_slide_kinds fields:["brief"]` (≈67 chars) |
| The exhibit proves the title | Point at the bar, row or box the title names; if you cannot, change the exhibit or the claim | `highlight` on the bar / series / slice / row / column / phase the title argues; one per slide |
| The takeaway concludes, it does not restate | It answers "so what" for this slide in one line, in words the title did not use | `takeaway` ≤ ~98 chars; `insight` on `chart_insight`; `bottom_line`, `recommendation` on the kinds that own one |
| Reading order | Context left / top, conclusion right / bottom; the eye runs a Z | `main_left` puts the exhibit first and the number second; `main_top` puts the numbers first when they *are* the context |
| Source line | Dataset and year, then "team analysis"; definitions in a footnote, not the title | `source` on every evidence slide; `meta.source` as the default; `DATA_WITHOUT_SOURCE` |
| Units once, consistent | Currency, scale (k / M / bn), decimals and period format identical across the deck | `unit` on charts and bridges; one decimal where the source has one |
| Section marker | The reader always knows which chapter they are in | `meta.chrome.tracker: true`, `section_crumb: true`; `agenda` with `current` |
| Status stickers | "Draft", "Preliminary", "Illustrative", "Not exhaustive" when true — and removed when no longer true | `meta.chrome.confidentiality` for the deck-level marking; a slide-level sticker is the `eyebrow` on `title` / `image_case` or a `banner` on a raw compose |
| Page numbers | Always, except the cover | on by default; `{total}` excludes the appendix |

## 4. Text, chart or diagram

- **Numbers compared** → a chart. **A process, structure or relationship** →
  a diagram (`process`, `cycle`, `architecture`, `org`, `pillars`). **A
  judgement across options** → a table device: `option_matrix` with Harvey
  balls (fit, 0–4) or RAG (status). **An argument** → structured text with
  bold lead-ins (`executive_summary`, raw `labeled-rows`, `exec-summary`),
  never bullets of fragments. **One number** → `stat`, never a chart with one
  bar.
- **Chart by the comparison the message makes** (Zelazny): *component* —
  "share of", "almost half" → `donut` / `pie` (≤6 slices) or `stacked_bar`;
  *item* — "larger than", "ranks" → horizontal `bar`
  (`data.orientation: horizontal`), or raw `horizontal-bar-with-callouts`
  when each bar needs its own insight; *time series* — "grew", "declined" →
  `bar` for a few periods, `line` for many, `small_multiples` for one measure
  across 2–6 groups; *frequency* — "in the range", "most are" → columns;
  *correlation* — "varies with" → `scatter` (8+ items; fewer is a table);
  *a walk between two totals* → `bridge` / `waterfall`.
- **Direct labelling.** One series labelled on the chart needs no legend;
  the chart title states the measure and unit, the slide title states the
  message. Auto data labels are on; `style.show_legend` only for 2+ series.
- **Specialist exhibits and their kinds:** waterfall → `bridge` (deltas
  accented, subtotals computed); Harvey balls → `option_matrix` `harvey`
  (one scale per column, legend drawn); 2×2 → `matrix_2x2` (axes with low /
  high ends, four named quadrants, the recommended one `highlight`ed);
  driver tree → raw `driver-tree` (root metric left, MECE branches, unit on
  every node); heat map → `risk_heatmap` (bands fixed by likelihood ×
  impact, never hand-coloured); maturity ladder → raw
  `journey-maturity-model`; marimekko is not drawn — use a `stacked_bar`
  with the segment sizes in the labels, or a `table`.

```yaml
meta:
  title: Design-rule example
  template: forest-green
  date: October 2026
  source: Regulator's 2025 review; management information 2023-2025
slides:
  - kind: chart_insight
    title: Operational losses rose 84% in two years and now exceed the EUR 8M appetite
    chart:
      type: bar
      title: Operational-risk losses (EUR M)
      data:
        categories: ["2023", "2024", "2025"]
        series:
          - name: Losses
            values: [6.1, 8.4, 11.2]
        highlight: ["2025"]
    insight: The appetite was breached in 2024 and nobody was told until the regulator asked.
    insights:
      - 2025 losses are 1.4 times the EUR 8M appetite
      - Peer median is about EUR 8.4M for a bank this size
    takeaway: The breach is two years old; the control model, not the limit, is the problem.
```

## 5. Colour, type and density

- One base colour, one accent, grey for everything else. The accent goes on
  the one thing the title is about; `accent_strategy: primary` keeps it
  constant, `section-keyed` changes it per chapter, `rotate` per slide — use
  the last two only when the chapters or slides are genuinely parallel. Grid
  patterns' `cell_accent_mode: progressive` is for ordered or graded cells,
  never for unordered peers.
- Type comes from the template; never set sizes. A visual that cannot hold
  its text at the template's minimum is refused (`BODY_TOO_LONG`,
  `SEMANTIC_PATTERN_DEGRADED` with `max_chars`); the fix is fewer words or
  another slide, not a smaller font. A `table_font_scaled` info note is a
  prompt to split when the standard is strict.
- **Read or presented.** A deck that is read alone (pre-read, board paper,
  report) carries complete sentences, up to ~250 words on a page, exhibits
  that explain themselves: `meta.viewing_mode: read`. A deck spoken over
  carries one statement per slide and the detail in the appendix:
  `present`. Decide before the first slide; mixing them is the commonest
  reason a deck reads as uneven.
- Rhythm: no three slides of one visual family in a row, no motif on more
  than half the content slides, a light slide (stat, quote, chart) after a
  dense one (table, matrix). `analyze_deck_rhythm(deck_id)` and the
  `SEMANTIC_RHYTHM_*` warnings watch this; `explain_deck_spec` reports
  `rhythm` and `layout_coverage` before a render.

## 6. The twelve things reviewers flag, and where the product catches them

| Failure | Catch | Fix |
|---|---|---|
| Title is a topic ("Market overview") | `TITLE_NOT_ACTION` | Rewrite as the sentence the exhibit proves |
| The chart does not prove the title | not caught — read the render | Change the chart, or the claim; `highlight` the proof |
| Titles do not chain (an orphan slide) | not caught — read the ghost deck | Move it to the appendix or cut it |
| MECE violation (overlap, gap, a large "other") | not caught; `CHART_OVERLOADED` catches 7+ slices | Regroup; `group_small_below_pct` for a pie's tail |
| Several insights on one slide | `SLIDE_TEXT_DENSE`, `pattern_overcrowded`, region refusals | Split by message |
| Analysis before the answer | `NO_EXECUTIVE_SUMMARY`; `missing_executive_summary` in rhythm | `executive_summary` on slide two |
| Decorative colour, mixed fonts | template-driven; `accent_heavy_slide`, `strong_accent_run` in rhythm | One `highlight`; `primary` accent strategy |
| Inconsistent units and terms | not caught | One `unit` per measure; one name per thing across slides |
| Missing source, legend, chart title | `DATA_WITHOUT_SOURCE`; chart heading rules | `source` everywhere; `title` on every chart |
| No ask, "Thank you" closer | `CLOSING_WITHOUT_NEXT_STEPS`, `missing_next_steps` | `next_steps` with `decisions` |
| Placeholder text, stale "Draft" | `SEMANTIC_WEAK_CONTENT` for recipe text; stickers not caught | Search the spec for "Replace with", "TBD", "Draft" before the second-template pass |
| Looks right in the validator, wrong on the page | by design: `deterministic_ready` is not a visual verdict | Render every slide, on two templates; `submit_visual_review` |
