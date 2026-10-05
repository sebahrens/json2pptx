# h-tech-data: Atlas Retail lakehouse business case (MCP, `--tools all`)

## Journey in order

1. **First contact** (`initialize.json`, `tools/list`): 48 tools, 224,455 bytes for the tool list. The server instructions said call `get_started` first; `get_started(task: brief)` (n=3, 12.5 KB) laid out the DeckSpec path: plan_deck -> list_slide_kinds -> validate_deck_spec -> render_deck_spec -> render_deck_thumbnails -> submit_visual_review.
2. **plan_deck** (n=4, format deckspec): unusable for this brief. It truncated my first sentence into the deck title ("...640 sto..."), left 13 facts unplaced (all four after-KPIs, the cost split, the screenshot, both parallel tracks), routed architecture tiers into the exec summary and options slots, and spent 5 of 13 slides on cover/agenda/dividers. I discarded it and planned 12 flat slides myself.
3. **Kind discovery**: catalog (n=5, 5.9 KB), copy-ready examples for 13 kinds (n=6, 12.8 KB), then field schemas with budgets and compositions (n=11, 26.7 KB) after one UNKNOWN_PARAMETER on `compositions` (n=10; it is a `fields` value, not an argument). The item schemas told me `roadmap` has no parallel tracks and `comparison` has no connector/highlight option, so I read `show_pattern` for comparison-2col, before-after, phase-roadmap and image-text-split (n=13-15, n=23).
4. **recommend_visual with previews** for S2 (n=8) and the cost driver (n=9): about 230 KB each. S2 top candidate arch-stack, used. Cost-driver top candidate driver-tree (needs leaves per branch I do not have), not used; I took donut (#4) via chart_insight. The preview images (arch-stack with vertical rails, comparison rows aligned, driver-tree, pie) genuinely helped me choose: I saw the rails before authoring.
5. **Author v1 -> validate** (n=16): 1 error (a blank table header was dropped, so headers=1 vs 2 cells), 4 warnings. **v2 -> validate + render** (n=17/18): 3 errors: the S1 three-row table truncated 2 of 3 rows ("split at row 1"), an unlocated 9.5pt "card-title" on the raw roadmap slide, option-matrix detail 42/32 chars. **v3** (n=19/20): clean, `deterministic_ready: true` on the 2nd render after 3 validates (first-ready at call n=20).
6. **Thumbnails + inspection** (n=21, 12 images; n=22 slides 4 and 7 at density 100). Repair round 1 (v4, n=24/25): shortened the two callout labels that were covering the screenshot and folded a floating KPI comparator into its label. Only slides 3 and 4 changed hash. **submit_visual_review** (n=26) -> `visually_reviewed_current_revision`.
7. **p-style**: same spec with template p-style (n=27) rendered `deterministic_ready` with no edits; all 12 thumbnails inspected (n=28), nothing to repair.
8. Extra tool check: `analyze_deck_rhythm` on the final deck (n=29) scored composition 100 and recommended fixing "100% of cells (2/2) underfilled" at slide_index -1.

Where the time went: roughly a third in discovery (reading kind schemas and four pattern schemas to find the raw-only knobs), a third in the three validate/render loops, a third in looking at 44 images (previews, two thumbnail passes, two large renders, the p-style pass).

## S1-S4: shape, attempts, outcome

| Slide | Shape used | Attempts | Outcome |
|---|---|---|---|
| S1 five-year run cost | `regions`, arrangement `main_left`: chart region (bar, 2 series, 62%) left; `stat` "EUR 5.5M" (35%) over a 3-row `table` (65%) right; one title, one source | 3 validates (blank header dropped; table rows truncated; then clean) | Chart has data labels and legend; the stat and the three-row table are legible at density 100 (n022-02). The narrow column stayed readable once the stat/table split was 35/65 and the table heading was dropped. |
| S2 target architecture | `architecture` kind -> arch-stack, 5 tiers with items, `rails: ["Security & governance", "FinOps"]` | 1 | Rails render as two vertical bands beside the stack, not as a tier (n021-05, n028-05). recommend_visual's #1 candidate, and the preview matched the render. |
| S3 ops console | `image_case`, `fit: contain`, two fractional callouts (0.27/0.135 "SLA breach", 0.50/0.54 "Job queue (14,860)"), body + 3 bullets + 3 metrics | 2 (v3, then shorter labels in v4) | Callout dots land exactly on the Queue health header and the delayed payments.settle row on both templates (n022-01, n028-04). The screenshot renders at about a third of the slide width with the lower half of its column empty; no width control in the kind. |
| S4 KPI before/after | `raw_json2pptx` with comparison-2col, headers Today / Target after migration, 4 rows, `overrides.connectors: true, highlight_column: right` | 1 | Rows aligned by hairline rules, a chevron connector per row, target column highlighted (n021-10, n028-10). Not reachable through the `comparison` kind. |

Also required and delivered: option matrix with Harvey balls and Databricks highlighted (slide 6), donut cost split with insights (slide 8), phase-roadmap with two parallel-track bars via raw_json2pptx (slide 9), action-titled executive summary (slide 2), next-steps closer with decisions band (slide 12).

## `--tools all` versus the default

- First contact: 224,455 bytes for tools/list (plus 12.5 KB get_started).
- Used 11 of 48 tools. The extra that earned its place: `show_pattern` (the only way to learn `parallel_tracks` and `connectors`). `analyze_deck_rhythm` added nothing. Registry/cache/export/raw-repair tools were noise for a brief-driven run.
- Validate and render calls to first `deterministic_ready: true`: 3 validates, 2 renders (call n=20).

## Slides the tools scored clean that looked wrong

- Slide 4 (score 100): small screenshot, half-empty image column, callout pills covering the picture at v3 (`log/images/n021-04.jpeg`, `n022-01.jpeg`); improved but not fixed in v4 (`n025-04.jpeg`).
- Slide 3 (score 100): comparator "SLA is 6 am" orphaned below its card at v3 (`n021-03.jpeg`); fixed in v4 by removing it. The six cards still leave a large empty band (`n025-03.jpeg`).
- Inverse case: slide 10 carried a `pattern_overcrowded` info on every run and looks fine (`n021-10.jpeg`).

## Top five things that would make an agent love this tool

1. Make the kinds cover their own patterns: `roadmap.parallel_tracks`, `comparison` connectors/highlight (or a `before_after` kind), `image_case.image_width_pct`. Each raw_json2pptx detour cost a show_pattern read and a schema I had to carry verbatim.
2. A plan_deck that recognises explicit slide asks (architecture, option matrix, year series, screenshot, phases with parallel tracks) and never emits an ellipsised title as content.
3. Findings that name the cell: `TEXT_BELOW_READABLE_MIN` and `table_rows_truncated` should carry the value path and a concrete knob (size_pct, which text) instead of "shorten the text" / "split at row 1".
4. recommend_visual gated by data shape, with a preview count parameter; today it is 230 KB a call and ranks gauge for a stack and driver-tree for a four-way split.
5. Match spec revisions by slide id so the known_hashes shortcut works and a two-slide repair does not return twelve images and a fresh deck_id. The per-slide content_hash already makes this verifiable; the server could use it too.

## Final artefacts

- Final spec (modern-template): `/tmp/jj/wave/h-tech-data/spec-v4.json` (revisions: `spec-v1.json`, `spec-v2.json`, `spec-v3.json`, `spec-v4.json`; render payloads `render-v2.json`, `render-v3.json`, `render-v4.json`, `render-v4-pstyle.json`)
- PPTX modern-template (reviewed, approved): `/tmp/jj/wave/h-tech-data/out/atlas-retail-data-platform-modernisation-f708933f.pptx` (content_hash f38fa435...)
- PPTX p-style: `/tmp/jj/wave/h-tech-data/out/atlas-retail-data-platform-modernisation-3dcb8f04.pptx` (content_hash 77505f73...)
- Earlier modern-template render (v3): `/tmp/jj/wave/h-tech-data/out/atlas-retail-data-platform-modernisation-0c7806e3.pptx`
- Slide images inspected: previews `log/images/n008-01..04.jpeg`, `n009-01..04.jpeg`, `n012-01..02.jpeg`; v3 full pass `n021-01..12.jpeg`; density-100 `n022-01..02.jpeg`; v4 changed slides `n025-03.jpeg`, `n025-04.jpeg` (other ten hash-identical to n021); p-style full pass `n028-01..12.jpeg`.
- Visual review record: `review-v4.json` -> call n=26 `visually_reviewed_current_revision`.
