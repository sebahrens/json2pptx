# Quality and review

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
pass on every slide.

## What the tools cannot see (look for these yourself)

- **A zoomed axis you did not ask for.** Bars, lines, areas and waterfalls with
  non-negative data now start at zero; if a chart looks zoomed, a `data.y_min`
  was authored — check that the heading says so.
- **A strip whose band is inside the block.** Any pattern, compose block or
  grid that leaves the lower third of the content area empty is reported
  (`SLIDE_UNDERUSED` with `empty_band_pct`; `VERTICAL_IMBALANCE` for a short
  table; `SPARSE_SINGLE_ROW_FLOW` for a flow of short labels:
  `go-slide-creator-i7yju`, `-kgfs1`), and a `kpi_snapshot`, a `value-chain`
  and a `process-flow` of sentence steps are grown to clear it. The check
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
- **The screenshot too small** for its callouts to be legible at projection
  size.

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
