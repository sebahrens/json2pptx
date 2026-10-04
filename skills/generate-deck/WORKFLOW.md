# Workflow: Plan → Vary → Render → Repair

The default path authors a semantic **DeckSpec**. Raw `PresentationInput`
steps (skeletons, pre-emit checklist, `validate_input` / `strict_fit`,
`repair_slide`) are in [RAW_PATH.md](RAW_PATH.md); the review protocol below
applies to both paths.

---

## Phase 1: PLAN — storyline first

1. **Ghost deck.** Write the action titles alone, in order, and read them as
   the whole argument (QUALITY.md). Present them to the user unless they asked
   for the finished deck directly.
2. **Draft the spec.** For more than four slides call `plan_deck` with
   `format: "deckspec"` (response fields: DECKSPEC.md → Plan the narrative;
   `structure` chapters for 8+ slides). Place every `unplaced_facts[]`
   entry, replace every `__FILL__` (titles, `meta.date`) and fill the
   fields from `list_slide_kinds`. Region clauses ("left a line
   chart; upper right a KPI") draft a `regions` slide whose slot lists each
   region's `path`, `role`, `kind` and clause; `unsupported_regions[]` names
   what it could not draft.
3. **Template.** `list_templates` gives names (`fields:"names"` alone is
   enough to pick one), `canonical_layout_ids` and `color_roles`. Set
   `meta.template`; leave `meta.accent_strategy` at `primary` unless Phase 2
   → Accent monotony says otherwise. Fill with `color_roles.primary_fill`
   (not always `accent1`); text on an accent fill uses
   `color_roles.ink_on_accent[accentN].ink`. With `fields="full"`,
   `accent_usage_guide` gives each accent's role (with
   `accent_usage_guide_derived: true` keep to `primary_fill` and the
   `semantic_accents`, never a `near_background_accents` entry for text).
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
argument first, without counting layouts as slides; then add canonical IDs
to `meta.required_layouts` and resolve each `layout_coverage.missing` entry
with a compatible narrative slide (DECKSPEC.md). A `blank-canvas` content slide carries its title into the raw
slide's `headline`.

### One slide or several: spatial planning

Views that each prove the *same* action title (the trend, its KPIs, the
dated plan) may share a slide as regions; a conclusion the title does not
state gets its own slide (QUALITY.md §3). One visual stays the default.
Prefer `kind: regions` (DECKSPEC.md). When its region kinds cannot express
the slide (pattern panels, quotes, native diagrams), make only that slide
`raw_json2pptx` and start from `list_slide_kinds` `kinds:["raw_json2pptx"]`
→ `composed_example` (line chart beside a `metric-list` KPI column above a
`timeline-horizontal` footer; renders as returned — swap title, data,
source). Do not re-derive the envelope.

| Need | Use |
|------|-----|
| Several regions, each a whole pattern or chart, proving one title | slide `compose`: each `segments[]` entry holds one `pattern`, one `diagram` (svggen; give it `alt`) or a nested `compose` |
| One small block inside a grid or pattern cell (`kpi-3up` in a `matrix-2x2` quadrant) | cell-level `pattern` / `grid` (RULES.md 6c) |
| More table rows than one slide holds | raw `split_slide` (`split.by: "table.rows"`) paginates; it never combines views, and `compose` never paginates |

- `direction: "horizontal"` (side by side) or `"vertical"` (stacked); nest a
  `compose` for chart | KPIs above a footer (≤ 8 segments, depth 2, 12
  leaves; `get_capabilities().features.compose`).
- `size_pct` is a segment's share of width / height; omitted shares split
  equally, `smart_compose: true` balances by content. The dominant evidence
  gets the largest share. `gap` is in points (default 8).
- One action title and one slide `source` cover every region; each chart
  region's title names measure and unit.

Each segment is fit-checked in its own rectangle (`BODY_TOO_LONG` names
`segment[i]`). Cut copy or add share rather than shrink type, and inspect
the thumbnail.

Reject the plan before rendering when it produces an isolated numeric divider,
an untitled visual canvas, one slide per required layout, or a deck made only
of bullets and coloured rectangles.

---

## Phase 2: VARY — check the sequence

The compiler picks patterns and layouts per kind; your job is the sequence.
`explain_deck_spec` (full profile) previews each slide's resolved visual
without rendering, and `analyze_deck_rhythm` accepts the `deck_id`.

- No visual family and no motif three times in a row (`longest_run`,
  `motif_runs`: `kpi-4up` → `stylish-panels` → `icon-row` are three rows of
  open columns, whatever the patterns), and no motif on more than half the
  content slides (`dominant_motif`); follow a dense slide (table, grid) with
  a light one (stat, quote, section).
- Clear `analyze_deck_rhythm`'s narrative codes (`missing_executive_summary`,
  `missing_next_steps`, `missing_sections`,
  `evidence_missing_takeaway_or_source`, `bullets_heavy`), run codes
  (`break_run`, `break_motif_run`, `motif_dominant`,
  `continuation_interrupted`) and accent codes (`accent_heavy_slide`,
  `strong_accent_run`) — RAW_PATH.md lists them.
- `composition_score < 70` or `longest_run ≥ 3` → change the kind of the
  middle slide, not its content.

### Pattern monotony (deck-level)

Break runs with a family that fits the content (card-grid →
comparison-2col → stat-hero → matrix-2x2 → icon-row); a stat or quote only
for a single number or a stakeholder voice, never as filler. A split exhibit
titled `… (1/2)` / `… (2/2)` (or `(cont.)`) is one slide for every run check;
keep the parts adjacent — a slide between them is reported
(`continuation_interrupted`, `SEMANTIC_RHYTHM_CONTINUATION_SPLIT`). Appendix
back matter is exempt from `break_run`, `DECK_MONOTONY` and
`SEMANTIC_RHYTHM_MONOTONY` (whose message and `evidence.run` name the run).

### Accent monotony

This is the one place the skill sets accent strategy; other guides defer here.

- `primary` (default) is the safe choice for any length: one brand accent;
  keep grid cells `uniform` unless they show ordered or graded data.
- `section-keyed` gives each chapter its own accent — use it when the deck has
  `structure.sections` / section dividers.
- `rotate` cycles pattern slides through the template's **safe** accents only
  (readable under light text, not the negative accent, not a grey or pastel
  slot), keyed to each pattern's content so inserting a slide recolours
  nothing. Validation reports the excluded accents once per deck
  (`ROTATED_ACCENT_UNREADABLE`, info). Use it only when that safe set has
  three or more accents and the deck has many pattern slides.

Do not switch strategy just to move `accent_balance`; judge the rendered slides.

---

## Phase 3: RENDER

`validate_deck_spec` → `render_deck_spec`; fix blockers at their `path`.
The first validation reports everything knowable, even when a slide does not
compile: fix every finding in one pass, then re-validate (`warnings[]` says
what was assumed and which checks wait). `templates: ["modern", …]` also
reports the spec on other templates. `deterministic_ready` is not
approval. Retain `deck_id`; revise with `patch` and re-render
`changed_slides`.

---

## Phase 4: REPAIR — review every slide

Validation is not verification. Contrast auto-fix, wrapping, clipped text and
the wrong visual show only in pixels. **Images are truth.**

1. **Render.** `render_deck_thumbnails` for every slide of the current
   revision with `density: 100` (the 50 DPI default is too coarse for 10pt
   text). If a response is truncated, request the rest with `slide_indices` —
   every slide needs an image you looked at; `preview_slide_wireframe` never
   counts. No render tooling (`get_started.runtime.render_available: false`)
   → deliver as **UNREVIEWED** and say so.
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
   | 10 | Native slide composition | decorative web UI dominates the content (RULES.md → Native slide composition) | P1 |

3. **Record it.** Every failed check is a finding on that slide in
   `submit_visual_review` (`{severity, category, description, location}`:
   `severity` P0–P3 and `category` from the tool schema's enums — whitespace
   is `layout_balance`; any other value is rejected and nothing is
   recorded). `slides[].verdict` is `approved`, `changes_requested` or
   `inconclusive`. A P0/P1 finding makes the deck verdict
   `changes_requested`; approve only when all ten pass. For check 10, name
   the needless UI element and a repair that preserves its content.
4. **Repair.** Edit the spec at the finding's `path` (or the field the
   rubric names), send `deck_id` + `patch`, re-render and re-check
   `changed_slides` (pass them, or slide ids, as `slide_indices`), then
   review the whole final revision once and submit. On a repeat pass send
   the `content_hash` values you hold as `known_hashes` (combines with
   `slide_indices`; an unmatched hash is ignored): a slide whose pixels did
   not change returns `{index, id, content_hash, unchanged: true}` with no
   image and no `path`. The image you hold is still that slide; submit its
   `content_hash` as `image_sha256`. CLI: `json2pptx render-thumbnails
   deck.pptx --out-dir slides/ --known-hashes <hash>,<hash>`.
   Deterministic repairs: RAW_PATH.md; finding semantics:
   [FINDINGS.md](FINDINGS.md).
5. **Loop cap.** Stop after **three** repair rounds. If slides still fail,
   hand back the deck with the open findings listed per slide rather than
   looping or declaring it done.

Do not tell the user the deck is done until the final revision's review is
submitted and approved, or you have said exactly what could not be verified.

---

## Automated vision QA (`inspect_slide_images`)

`inspect_slide_images` (CLI `json2pptx inspect`) runs the Claude-vision QA
agent on rendered images (`{index, path | png_base64, slide_type?, title?}`):
per-slide findings whose `suggested_fixes[]` map to `repair_slide` kinds,
plus a `findings` FindingEnvelope (P0/P1 → `error`). Without
`ANTHROPIC_API_KEY` it is `mode:"heuristic"` (blank / edge-overflow /
aspect-ratio, all P3). About 60% of layout flags on correct decks are false
positives, so P2/P3 are advisory. It supplements your own look at every
slide, never replaces it.
