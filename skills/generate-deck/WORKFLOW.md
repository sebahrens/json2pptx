# Workflow: Plan → Vary → Render → Repair

The default path authors a semantic **DeckSpec**. Raw `PresentationInput`
steps (skeletons, pre-emit checklist, `validate_input` / `strict_fit`,
`repair_slide`) are in [RAW_PATH.md](RAW_PATH.md); the review protocol below
applies to both paths. Storyline rules are in [QUALITY.md](QUALITY.md).

---

## Phase 1: PLAN — storyline first

1. **Ghost deck.** Write the action titles alone, in order, and read them as
   the whole argument (QUALITY.md). Present them to the user unless they asked
   for the finished deck directly.
2. **Draft the spec.** For more than four slides call `plan_deck` with
   `format: "deckspec"`: it returns `deck_spec` (kinds drafted from the
   brief — `structure` chapters for 8+ slides), `slots[]` with each slot's
   `path`, guidance and routed brief facts, and `unplaced_facts[]` (place
   them; do not drop them). Replace every `__FILL__` (titles, `meta.date`)
   and fill the fields from `list_slide_kinds`.
3. **Template.** `list_templates` gives names, `canonical_layout_ids` and
   `color_roles`. Set `meta.template`; leave `meta.accent_strategy` at
   `primary` unless Phase 2 → Accent monotony says otherwise. Fill with
   `color_roles.primary_fill` (not always `accent1`) and set text on an
   accent fill to `color_roles.ink_on_accent[accentN].ink`. With
   `fields="full"`, `accent_usage_guide` gives each accent's role; when it
   carries `accent_usage_guide_derived: true` the template authored none and
   the lines are contrast facts only — keep to `primary_fill`, the
   `semantic_accents`, and never use a `near_background_accents` entry for text.
   `grid` gives the template's margin, columns, gutter and the content frame
   lines content should start on.
4. **Chapters.** Use `structure.sections` (with `auto_agenda` for two or more
   sections) instead of hand-made dividers; the engine numbers them.
5. **Chrome.** Page numbers (title and closing skipped) and the date are on
   by default; set `meta.date`, and add confidentiality / client in
   `meta.chrome` when the deck needs them.

The outline you show the user:

```
Deck: [title] · Template: [name]
  1. [kind] — [action title] — [facts it carries]
  2. …
```

### Layout coverage requests

When the brief asks for a complete deck that also showcases layouts, plan the
argument before applying the coverage constraint:

1. Write the story and section outline without counting layouts as slides.
2. Choose a slide budget that supports the subject; "complete" usually needs
   repeated content layouts and more slides than required layouts.
3. Put chapter boundaries in `structure.sections`.
4. Add canonical IDs to `meta.required_layouts`, run `explain_deck_spec`,
   review every `layout_coverage.assigned` entry and resolve `missing` by
   adding a compatible narrative slide.
5. Keep DeckSpec as the source; use `raw_json2pptx` only for the smallest
   visual the semantic kinds cannot express.
6. A `blank-canvas` content slide carries its title into the raw slide's
   `headline`; the rest of the canvas holds the visual.

Reject the plan before rendering when it produces an isolated numeric divider,
an untitled visual canvas, one slide per required layout, or a deck made only
of bullets and coloured rectangles. Evidence-oriented decks need at least one
data-bearing visual and enough distinct visual families to separate evidence,
analysis, process and chapter structure.

---

## Phase 2: VARY — check the sequence

The compiler picks patterns and layouts per kind; your job is the sequence.
`explain_deck_spec` (full profile) previews each slide's resolved visual
without rendering, and `analyze_deck_rhythm` accepts the `deck_id`.

- No visual family three times in a row (alternating `kpi-3up` and `kpi-4up`
  is still one run; composed slides with the same regions are one run,
  differently composed ones are not); follow a dense slide (table, grid) with a light one
  (stat, quote, section).
- Every evidence slide has a `takeaway` and a `source`.
- Clear `analyze_deck_rhythm`'s narrative codes (`missing_executive_summary`,
  `missing_next_steps`, `missing_sections`,
  `evidence_missing_takeaway_or_source`, `bullets_heavy`) and accent codes
  (`accent_heavy_slide`, `strong_accent_run`) — RAW_PATH.md lists them.
- `composition_score < 70` or `longest_run ≥ 3` → change the kind of the
  middle slide, not its content.

### Pattern monotony (deck-level)

The most common agent mistake: the same pattern slide after slide (card-grid,
card-grid, card-grid, …) makes a flat deck. Break runs with a different
family that fits the content — e.g. card-grid → comparison-2col → stat-hero →
matrix-2x2 → icon-row. Use a stat or quote slide only when the message is a
single number or a stakeholder voice — never as filler. Appendix back matter
(after a divider titled Appendix / Backup / Annex, or in a
`sections[].appendix: true` section) is exempt: backup pages never form a run
for `break_run`, `DECK_MONOTONY` or `SEMANTIC_RHYTHM_MONOTONY`.

### Accent monotony

This is the one place the skill sets accent strategy; other guides defer here.

- `primary` (default) is the safe choice for any length: one brand accent;
  keep grid cells `uniform` unless they show ordered or graded data.
- `section-keyed` gives each chapter its own accent — use it when the deck has
  `structure.sections` / section dividers.
- `rotate` cycles pattern slides through the template's **safe** accents only
  (light body text readable on the fill, not the negative accent, not a grey
  or pastel slot under 2:1 on `lt1`), keyed to each pattern's content so
  inserting a slide recolours nothing; every `kpi-*` pattern shares one accent. Validation
  reports the excluded accents once per deck (`ROTATED_ACCENT_UNREADABLE`,
  info, at `/accent_strategy`). Use it only when that safe set has three or
  more accents and the deck has many pattern slides; otherwise it adds noise,
  not variety.

Do not switch strategy just to move `accent_balance`; judge the rendered slides.

---

## Phase 3: RENDER

`validate_deck_spec` → `render_deck_spec`. Fix blocking diagnostics at their
`semantic_path`; `SEMANTIC_PATTERN_DEGRADED` means the visual you asked for
did not land (DECKSPEC.md). `deterministic_ready` is a precondition for
review, never approval. Keep the returned `deck_id`: revisions are
`deck_id` + `patch`, and `changed_slides` names what to re-render.

---

## Phase 4: REPAIR — review every slide

Validation is not verification. Contrast auto-fix, wrapping, clipped text and
the wrong visual show only in pixels. **Images are truth.**

1. **Render.** `render_deck_thumbnails` for every slide of the current
   revision; pass `density: 100` for the review pass (the 50 DPI default is a
   thumbnail, too coarse to read 10pt text). When a response is truncated,
   request the remaining slides with `slide_indices` — every slide needs an
   image you looked at. `preview_slide_wireframe` is structural only and
   never counts as visual review. No render tooling
   (`get_started.runtime.render_available: false`) → deliver as
   **UNREVIEWED** and say so.
2. **Score each slide against the rubric.** No deck size is exempt — a
   one-slide deck is reviewed like a forty-slide one.

   | # | Check | Fails when | Severity |
   |---|---|---|---|
   | 1 | Action title | a topic label, > 2 lines, or no number where one exists | P1 |
   | 2 | Body proves the title | content argues something else or restates the title | P1 |
   | 3 | Readable | clipped, overflowing or overlapping text; text under ~10pt at viewing size | P0 |
   | 4 | Contrast | text hard to read on its fill; surprise grey from auto-fix (RULES.md 16) | P1 |
   | 5 | Aligned | edges, baselines or columns visibly off the grid | P2 |
   | 6 | No orphans | a lone word on a line, a one-item bullet list, an empty cell or placeholder | P2 |
   | 7 | Balanced whitespace | content crammed in one corner or a strip floating in white space | P2 |
   | 8 | Meaningful accents | colour that encodes nothing, or highlights the wrong item | P2 |
   | 9 | Evidence | chart without units/axis label, data without `source`, no `takeaway` | P1 |

3. **Record it.** Every failed check is a finding on that slide in
   `submit_visual_review` (`{severity, category, description, location}`,
   category = the check name). A P0/P1 finding makes the deck verdict
   `changes_requested`; approve a slide only when it passes all nine.
4. **Repair.** Edit the spec at the finding's `semantic_path` (or the field
   the rubric names), send `deck_id` + `patch`, re-render `changed_slides`,
   re-check those slides, then make one full pass over the final revision
   and submit that review. Deterministic repairs are in RAW_PATH.md;
   [FINDINGS.md](FINDINGS.md) covers finding semantics.
5. **Loop cap.** Stop after **three** repair rounds. If slides still fail,
   hand back the deck with the open findings listed per slide rather than
   looping or declaring it done.

Do not tell the user the deck is done until the final revision's review is
submitted and approved, or you have said exactly what could not be verified.

---

## Automated vision QA (`inspect_slide_images`)

`inspect_slide_images` runs the same Claude-vision QA agent as `testrand qa`:
pass rendered images (`{index, path | png_base64, slide_type?, title?}`;
`slide_type` and `title` improve precision) and get per-slide findings whose
`suggested_fixes[]` map to `repair_slide` kinds, plus a `findings`
FindingEnvelope (P0/P1 → `error`, so `findings.ok` is false exactly when a
slide needs repair). Without `ANTHROPIC_API_KEY` it degrades to
`mode:"heuristic"` (blank / edge-overflow / aspect-ratio only, all P3).
`image_quality`, `aspect_ratio` and `border_style` findings have no automatic
fix: surface them. Vision flags about 60% false positives on layout issues in
correct decks, so treat P2/P3 as advisory. It supplements the rubric above; it
never replaces your own look at every slide.
