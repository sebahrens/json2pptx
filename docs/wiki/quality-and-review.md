---
title: "Quality and review"
type: documentation
summary: "json2pptx guide to quality and review"
status: active
last_updated: "2026-10-17"
tags: [json2pptx, deck-authoring]
---

## Quality and review

What "done" means, what the engine checks for you, and what only your eyes
can check. Back to the [hub](README.md). The contract is the completion rule
in [`SKILL.md`](../../skills/generate-deck/SKILL.md) and the rubric in
[`WORKFLOW.md`](../../skills/generate-deck/WORKFLOW.md); this page is the
practice.

## The three gates, in order

1. **Deterministic.** `validate_deck_spec` / `render_deck_spec` report no
   `error` (`ok: true`, `deterministic_ready: true`). Warnings cleared too,
   unless you can say why one stays.
2. **Structural.** `score_deck(deck_id)` passes its `quality_gate`: action
   titles, an executive summary, a closer with next steps, takeaways and
   sources on evidence slides, text density. A passing score stops
   score-driven repair; it says nothing about how the slides look (a lorem
   deck once scored 99).
3. **Visual.** Every slide of the revision you ship rendered
   (`render_deck_thumbnails`) and looked at, scored against the rubric below,
   recorded with `submit_visual_review`, and `status:
   "visually_reviewed_current_revision"`. Then the same on the second
   template.

A deck that clears 1 and 2 with topic titles, a bridge whose opening bar is a
stub, or a footer cut off with an ellipsis is not finished. All three were
scored 100 in the journey runs (the footer is now reported: `CHROME_TRUNCATED`).

## The ten-point rubric (per slide)

| # | Check | Fails when |
|---|---|---|
| 1 | Action title | a label, more than two lines, or no number where one exists |
| 2 | Body proves the title | the exhibit argues something else or restates the title |
| 3 | Readable | clipped, overflowing or overlapping text; text under ~10pt at viewing size |
| 4 | Contrast | text hard to read on its fill; unexpected grey from the auto-fix |
| 5 | Aligned | edges, baselines or columns visibly off |
| 6 | No orphans | a lone word on a line, a one-item list, an empty cell |
| 7 | Balanced | content crammed in one corner, or a strip floating in white space |
| 8 | Meaningful accents | colour that encodes nothing, or highlights the wrong item |
| 9 | Evidence | chart without unit, data without `source`, no `takeaway` |
| 10 | Native composition | web-UI decoration (nested cards, pills, buttons) over the content |

Record a failed check as `{severity, category, description, location}` on
that slide; P0/P1 make the deck `changes_requested`. Approve only when all ten
pass on every slide. The deck-level checks a partner runs before these —
governing thought, vertical and horizontal logic, the executive summary
mirroring the body, the ask on the last slide — are in
[slide-design-rules.md](slide-design-rules.md) §1–2 and §6;
`explain_deck_spec(deck_id)` (hidden, callable by name) lists every slide's
title, takeaway and role in order so the chain can be read without a render.

## Measured layout review

The engine now reports `BASELINE_MISALIGN` (same-role peers >2pt apart),
`TYPE_HIERARCHY_FLAT`, `PANE_FILL_IMBALANCE` (comparable prose panes >0.4
apart), `EXHIBIT_TOO_SMALL` (sole image/chart <35%), `GUTTER_INCONSISTENT`
(automatic gaps >4pt from the deck median), `NUMBER_FORMAT_INCONSISTENT`,
`TITLE_ORPHAN`, `DETACHED_LABEL`, `CONTINUATION_UNDERFILLED` and
`STATUS_COLORS_INDISTINCT`. These are advice with measured evidence, not
proof of a bad slide. Authored gaps, different units, unknown geometry and
unrelated text roles must not be treated as comparable peers.

`analyze_deck_rhythm` exposes layout ID, pane count, visual side, split and
known visual area share. Three same-side exhibits or three small exhibits
in a row raise `same_side_run` / `low_visual_share_run`. It also advises on
`takeaway_restates_title` (token overlap >0.6), `takeaway_heavy_deck`
(>50% of content slides) and `layout_variety_high` (>6 families in ≤20
content slides). A takeaway should add an inference, not repeat the title;
use an intentional family change to serve the argument.

## What still needs visual inspection

- **A zoomed axis you did not ask for.** Bars, lines, areas and waterfalls with
  non-negative data now start at zero; if a chart looks zoomed, a `data.y_min`
  was authored — check that the heading says so.
- **A strip whose band is inside the block.** Any pattern, compose block or
  grid that leaves the lower third of the content area empty is reported
  (`SLIDE_UNDERUSED` with `empty_band_pct`; `VERTICAL_IMBALANCE` for a short
  table; `SPARSE_SINGLE_ROW_FLOW` for a flow of short labels:
  `go-slide-creator-i7yju`, `-kgfs1`), and a pattern alone on a slide that
  needs under 70% of the area is scaled as a whole — rows up to 1.6×, type up
  to two steps — to clear it (`go-slide-creator-cyyiy`; compose segments and
  region cells are never scaled). The check
  measures where the last row ends, not how full the rows are: two bullets
  in a tall column, or a body-placeholder slide of two short lines, still
  pass. Look at them, and give a thin slide its second zone.
- **A footer that lost a field.** A footer line wider than the template's slot
  drops `project_code`, then the date, and `CHROME_TRUNCATED` says so
  (`go-slide-creator-m2tlt`) — a review finding that never blocks. Decide
  whether the dropped field matters, and check the second template.
- **Titles that wrap to three lines** on the serif template, or to two lines of
  capitals on `modern-template` / `blue-corporate`.
- **A pale second series** on a line chart on some templates; check the data
  palette on the second render.
- **Row highlight and column tint sharing a hue** on `p-style` status boards.
- **Screenshot detail legibility.** Area share is measurable; the size of
  evidence inside the image still needs inspection.
- **Unlabelled diagram shapes.** A `shape_grid` ellipse or diamond with no
  text (a BPMN start event, a gateway) passes every check; label it
  ("Start", "Cleared?") or use `process` with a `type: decision` step.
- **Captions set as bold body text.** A full-width bold paragraph between
  exhibits flattens the hierarchy; captions are regular weight, one line,
  and sit under the exhibit they explain.
- **A run of custom grids.** Three or more consecutive `raw_json2pptx` grids
  built from the same grey boxes read as one long slide even when the
  content differs, and a geometry fingerprint can flag repetition without understanding
  whether the technical content warrants it. Break it with a kind (`stat`,
  `chart_insight`, `regions`) or move the technical ones to the appendix.
- **Empty bands under one-line titles.** Most kinds and `regions` start
  content at the native body line; under a one-line title, look for a band
  the content could use. `executive_summary` starts below the measured title
  text, including wrapped titles.

## The second-template pass

Render the final `deck_id` with `template: "p-style"` (when present) or
another shipped template without touching the content. What breaks is
information: a title budget, a table that needed the compact pitch, chrome
that no longer fits. Fix the spec so it holds on both; do not fork a
per-template copy. Look at every slide again — the journey runs found a
truncated footer, a two-size date column and a wrapped title only on the
second template.

## Repair discipline

- Patch at the finding's `path` with `deck_id` + `patch`; take a
  `patch_verified` fix as given.
- Prefer the repair that keeps the visual: shorten → drop a detail line → add
  a region or slide → change the kind. Dropping to bullets (`layout: content`)
  is the last resort, even when a verified fix offers it first.
- Three repair rounds, then hand back the open findings per slide. Looping
  past three rounds produced worse decks, not better ones.
- Re-render only `changed_slides` (`known_hashes`), then one full pass over
  the final revision before you submit.

## Evidence to keep

For a deck you will defend: the final spec (read it back with
`validate_deck_spec {deck_id, read: "spec"}`), the `content_hash` of the
reviewed PPTX, the per-slide images you looked at, and the
`submit_visual_review` response. The journey harness under
[`tests/quality/journey/`](../../tests/quality/journey/) shows the full
evidence shape (`calls.jsonl`, `resp-NNN.json`, `images/`).
