# Partner review: json2pptx output (p-style, with midnight-blue for comparison)

Verdict: the storyline in State of AI is close to board-ready (action titles, three sections, exec summary up front). What gives it away as machine-made is the finishing: four different "so what" treatments, outlined boxes, big empty tinted panels, and charts that don't show the point their title makes. Most fixes are engine defaults, not content.

## P0: fix before any client sees it

**1. One takeaway component, restyled (the user's flag #1)**
- Evidence: state-of-ai 9, 12, 19 (salmon fill plus a 1pt orange outline across the full width, pressed against the footer); state-of-ai 6 (grey panel with grey left rule, "So what:"); patterns-smoke 20 (orange chevron "BOTTOM LINE" tag plus salmon bar); patterns-smoke 23 (solid orange full-bleed bar).
- Problem: there are four styles for the same idea. The one used most (the outlined salmon box) looks like a form-validation error. It carries no more visual weight than the body text and has no breathing room above the footer.
- Engine change: use a single `takeaway` renderer for slide `takeaway`, `chart-insights-split` so-what, `exec-summary` bottom line and `metric-list` callout. Default style: no outline, no tint. A 2–3pt accent rule on the left or top, then bold text in dk1 or accent at title-minus-4pt, sitting ≥0.15in above the footer zone. Optionally allow a light lt2 tint with no border. Retire the chevron tag and the full-saturation bar as defaults. Reserve vertical space for the takeaway in layout so it never collides with the footer.
- Impact: this is the biggest single lift in perceived quality, and it makes the "so what" consistent across every slide that has one.

**2. No outlines on filled shapes (the user's flag #2)**
- Evidence: state-of-ai 9/12/19 (takeaway), 13 (2x2 quadrants are white boxes with grey borders), 20 (top quote row outlined, bottom row grey-filled), 18 (milestone box); board-deck 4 and 9 (matrix and swimlane cells); patterns-smoke 4, 9, 16, 18.
- Problem: filled shape plus stroke is 2005 PowerPoint. The outlined and filled cells on patterns-smoke 16 and state-of-ai 20 look like a rendering bug rather than a design choice.
- Engine change: default `line: none` on every filled shape in patterns. Separate elements with whitespace or a single hairline rule (0.5pt lt2) between rows, never boxes. Add a lint `FILLED_SHAPE_OUTLINED` for user overrides.
- Impact: the output looks current straight away. It also removes the "outline versus fill" inconsistency within a slide.

**3. Charts don't show the point the title makes**
- Evidence: state-of-ai 9 (the title says engineering and support lead, but all five bars are the same grey, with no accent on the two leaders); state-of-ai 12 (the waterfall's reduction bar is black, its start and end bars are the same salmon as its increases, and it has no total highlight); state-of-ai 6 (every bar is accent, so nothing is emphasised).
- Problem: partner rule: the slide's one message must be visible in the data ink. Here the emphasis is either absent or spread across everything.
- Engine change: charts default to neutral (lt2/grey) with an `highlight` index or series in accent. On `horizontal-bar-with-callouts`, `highlight: [0,1]`. On `waterfall-bridge`, start and end totals in dk2, decreases in accent, increases in neutral, and the endpoint labels bolded. On a single-series column chart, only the latest or last bar is in accent by default.
- Impact: charts become self-explanatory in a single glance.

**4. Sources and units missing on data slides**
- Evidence: state-of-ai 5 (four KPIs, no source or base), 9 (%, of what? no n, no source), 12 (illustrative note floats top-right), 19; board-deck 2/3/8; consulting-layouts 8. Only state-of-ai 6 and 15 carry a source.
- Problem: a board deck with unsourced numbers gets sent back.
- Engine change: a slide-level `source` and `notes` field rendered in a fixed footnote zone (bottom-left, 8–9pt, above the footer), used by every pattern. Emit a fit finding `DATA_WITHOUT_SOURCE` (warning) when a slide contains a chart, KPI pattern or numeric table with no source. Move chart unit captions ("Cost per resolved ticket, $, illustrative") top-left under the title.
- Impact: meets the basic house standard, and pushes authors toward it.

## P1: partner would mark up

**5. Empty tinted panels and poor vertical fill**
- Evidence: state-of-ai 5 (KPI tiles about 60% empty grey), 17 (text sits in the middle of large pink panels next to an oversized chevron), 21 (pillars three-quarters empty grey); board-deck 3/8; patterns-smoke 1/2/7; consulting-layouts 5/6.
- Engine change: size tiles to their content (content-height plus padding) and centre the group vertically in the body zone. Otherwise cap tile height at about 1.6x the content. For `strategy-house`, size pillars to their bullets. Cap the `before-after` chevron at about 0.4in wide. Raise `BODY_SPARSE` when fill is under 35%.

**6. Tracker and section navigation**
- Evidence: state-of-ai 4/10/16 section dividers (a huge "01" on salmon, fine), but content slides carry no section tracker; the agenda (slide 2) uses three large grey blocks.
- Engine change: optional `chrome.tracker` renders the current section name small and accent-coloured above the title, taken from the preceding section slide. Restyle `agenda` as number plus text rows with hairline rules and no filled grey blocks (the style exec-summary already uses on state-of-ai 3). Highlight the current section when the agenda repeats.

**7. Action titles are not enforced**
- Evidence: board-deck (all titles are topics: "Revenue Highlight", "Operating Metrics", "Efficiency Gains"); consulting-layouts and patterns-smoke ("KPI 3-Up", "Service Tiers (with cell_overrides)").
- Engine change: a `TITLE_NOT_ACTION` info finding when a title is under 5 words or has no verb or number. SKILL.md guidance: a full-sentence claim of at most 2 lines. Rewrite the shipped examples, because agents copy them.

**8. Label contrast and colour discipline**
- Evidence: consulting-layouts 2/5/6/7 and board-deck 9 (black text on saturated orange); consulting-layouts 3/5 (three orange shades carry no meaning); state-of-ai 11/13 (black blocks and giant orange block arrows on a 2x2 with no items plotted).
- Engine change: auto-select lt1 text on accent fills. Warn-only is not enough for pattern-generated fills. Use at most one accent plus neutrals per slide by default, with tint ramps only where they encode an ordinal (heatmap). Replace the 2x2 block arrows with thin axis lines and labels.

**9. Tables**
- Evidence: state-of-ai 14 (orange header text, zebra stripes, table fills 30% of the slide); state-of-ai 19 and patterns-smoke 21 (black header bars).
- Engine change: default consulting table: bold dk1 header over a 1pt rule, no header fill, hairline row rules, no zebra, numbers right-aligned, first column bold. Stretch row height to fill, or anchor the table to the top and put a takeaway under it. Use dk2 for header fills, never pure black.

## P2: polish

**10. Typography mixing:** state-of-ai 7/8 use serif bullets at title size while every other body text is sans. Fix the placeholder body font to the theme minor font. Also, state-of-ai 8's column headers are rendered as bullets; column headers should be bold and unbulleted.

**11. Closing slide:** "Thank You / Questions and discussion" (state-of-ai 22, board-deck 10). A consulting deck closes on next steps and decisions requested. Ship a `next-steps` closer (owner, date, decision) and discourage "Thank You" in SKILL.md.

**12. Footer placement:** the footer is centred and the page number sits on its own at the far right (state-of-ai throughout). Left-align confidentiality and date, and put the page number right. The board deck has no footer or page numbers at all; enable chrome by default.

**13. Contact and team text wrapping:** patterns-smoke 27 breaks names across lines ("Jane / Smith") in a narrow column. Allow a minimum text width, or wrap the title rather than the name.

**14. The midnight-blue template's double vertical stripes** (blue and yellow at the left edge on every slide) crowd the title and KPI grid (board-deck 3). This is a template issue: the content margin should clear the decoration.

## Debate response

Constraint noted: content-sized patterns keep their middle vertical anchoring.

### Verdicts on the designer's items

| # | Designer item | Verdict | Why |
|---|---|---|---|
| 1 | Takeaway band restyle | AGREE-WITH-CHANGE | Adopt their spec, with one change: use no fill (not a 4% tint). A tinted slab near the bottom is exactly what the user dislikes. See converged spec C1. |
| 2 | One callout token set | AGREE | This is the same as my item 1. Keep a strong solid fill only as an explicit opt-in. |
| 3 | No strokes; tints and white gutters | AGREE | Same as my item 2. Hairline rules survive only as row separators in lists and tables. |
| 4 | Top-anchor all patterns | DISAGREE (deferred) | The user constraint says keep middle anchoring. Solve the "void" with content-sized heights and a height cap instead of re-anchoring. |
| 5 | 5-step type scale, 11pt body minimum | AGREE-WITH-CHANGE | Agree on the scale, but let dense tables and footnotes go to 10pt and 9pt. Board packs are dense. |
| 6 | Serif only for display; two-column headers | AGREE | Same as my item 10. |
| 7 | KPI typography, true minus, flush accent bar | AGREE | This adds a partner-level point: KPIs also need a comparator (vs plan or prior year) and a source line. |
| 8 | One neutral ramp and one accent ramp, no mid-tints | AGREE | Same colour discipline as my item 8. Converged in C4. |
| 9 | No black fills | AGREE | Same as my item 9 (dk2 headers). I accept "no header fill, rule under the header" as the default instead. |
| 10 | Clip-art geometry | AGREE | Same as my items 8 and 5 (2x2 arrows, chevron). The roof-notch bug is a P0 defect. |
| 11 | Chart defaults | AGREE | Same as my item 3: neutral bars, accent on the highlight bar, no y-axis when bars are labelled. Also keep the unit in the chart caption. |
| 12 | Letter-spaced caps labels | AGREE (P2) | Cosmetic. |
| 13 | Title and subtitle widows | AGREE (P2) | Cosmetic. |
| 14 | Agenda: numerals and rules, no tiles | AGREE | Same as my item 6. The agenda must also support a current-section highlight so it can double as a tracker. |
| 15 | Left-align inside cards | AGREE | Consulting house standard. |
| 16 | Compact legend | AGREE | |
| 17 | Footer aligned to left margin | AGREE | Same as my item 12. Page number stays at the right. |

### Converged proposal

**C1. Takeaway / so-what: one component, no box.**
- Always:
  - no stroke
  - 3pt accent1 left bar at full text height
  - 12pt inset
  - 14pt bold dk1 text, top-anchored in its zone
  - at least 16pt gap above and at least 12pt gap to the footer
  - width equal to the content width (not full bleed)
- Default fill: none.
- Allowed variant: lt2/neutral 4% tint. Never a peach accent tint.
- `emphasis: "strong"`: a solid accent1 fill with lt1 text, explicit opt-in only.
- Retire the chevron "BOTTOM LINE" tag and the outlined peach band.
- Use this component for the slide `takeaway`, the chart-insights `so_what`, the exec-summary bottom line and the metric-list banner.

**C2. Outlines: line none on every filled shape.**
- Separate items with 4–6pt white gutters over a neutral tint, or with 0.5pt horizontal rules in lists and tables.
- The 2x2 and BMC are built from gutters, not borders.
- A lint flags outlined fills in user overrides.

**C3. No dk1/black fills.**
- Table headers: no fill, bold dk1, 1.5pt rule under.
- Chains and stacks: a neutral 16% tint, with the one highlighted step in accent1.
- Waterfall: neutral for the non-highlighted deltas, dk2 or neutral 60% for totals, accent for the step the title talks about.

**C4. Colour discipline (merging my "light text on accent" with their "neutral + one accent").**
- The default fill for all pattern surfaces is the neutral ramp (4/8/16%) with dk1 text.
- Accent1 at 100% is used for exactly one emphasised element per slide: the highlight bar, the recommended option, the current phase, or a strong callout.
- Wherever accent1 at 100% is a fill, the text colour is chosen by measured contrast (on p-style orange that means lt1 white, bold). This kills the black-on-orange labels.
- The accent 10–20% tint only marks a highlighted row or band.
- Remove the 40–70% mid-tints.
- Ordinal ramps (heatmap) are the only multi-shade use.

**C5. Emptiness (keeping middle anchoring).**
- Cap stretch at about 1.5x content height so tiles don't balloon, and centre the group as today.
- Use the freed space for the takeaway zone.

**C6. Sources and chart defaults (my item 4 plus their item 11).**
- A fixed footnote zone at 9pt, bottom-left above the footer.
- `DATA_WITHOUT_SOURCE` warning.
- Neutral bars plus one highlight.
- No y-axis or gridlines when bars are labelled.

### Remaining disagreements

1. **Top-anchoring (designer item 4):** deferred per the user constraint. I would not re-anchor. I would cap heights instead.
2. **Takeaway fill:** the designer allows a 4% neutral wash by default; I want no fill by default, because the user dislikes the band look. This is minor: both sides agree on no stroke and no peach.
3. **Minimum body size:** the designer says 11pt; I would allow 10pt in tables and dense matrices.
4. **Not covered by the designer and still needed:** action-title lint, section tracker, next-steps closer, and KPI comparators. These are storyline items rather than styling, but they are what a partner sends the deck back for.
