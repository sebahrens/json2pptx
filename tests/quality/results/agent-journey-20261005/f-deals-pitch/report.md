# f-deals-pitch — Project Falcon CDD proposal (MCP, default profile, p-style then blue-corporate)

## Journey in order

1. **Onboarding (n=1–5, ~60 KB).** `initialize.json` instructions, abridged `tools/list` (23 KB, read twice: list + raw), `get_started task:brief`, `list_templates names` (p-style present), `list_slide_kinds` catalogue.
2. **Planning (n=6).** `plan_deck format:deckspec` on the full brief. The draft was unusable for this brief: bridge -> `pillars`, photo case -> `kpi_snapshot`, pricing schedule -> `timeline`, all titles `__FILL__`, 28 facts unplaced. Discarded.
3. **Kind research (n=7–10, ~33 KB).** `list_slide_kinds kinds:[regions, chart_insight, bridge, image_case]` and `[comparison, option_matrix, table, team, next_steps, title, executive_summary, timeline, roadmap, kpi_snapshot, decision, closing]` with `brief + example`, p-style colour roles, full `render_deck_spec` schema. **Bytes read before authoring the first slide: 100,631.**
4. **Authoring v1 (spec-v1.json).** 11 slides: title, exec summary, S1 `regions`, S2 `regions` with a `bridge` region, S3 `image_case`, `option_matrix`, `comparison`, `timeline`, 8-row `table`, `team`, `next_steps`.
5. **Validate/repair before render (n=11, 12, 15).** v1: main_left needs 3 regions; table max 7 rows -> split at row 4; bullets/eyebrow below 12pt. v2 patch: columns arrangement, split table into two slides, trimmed bullets -> "unknown region kind bridge" (with `bridge` in the available list). v3 patch: chart region `type: waterfall` (from `recommend_visual`'s data contract, n=13–14) -> only a misplaced `alt` left, verified removal.
6. **First render (n=16): `deterministic_ready: true`** after 3 validates + 1 render. Thumbnails all 12 (n=17), four at density 100 (n=19).
7. **Repair round 1 (n=20–21).** Footer truncated the client name on 11 slides; removed `project_code` -> fits. Re-thumbnailed with `known_hashes` (cover unchanged, 11 images), full-deck pass on the final revision.
8. **Completion (n=23).** `submit_visual_review` 12 slides approved with P2/P3 findings on slides 4, 7, 9, 10 -> `visually_reviewed_current_revision`.
9. **Cross-template (n=24–25).** Fork-render the same spec on blue-corporate, all 12 thumbnails inspected.

Where the time went: roughly a third on discovery (kinds, the S2 shape), a third on the validate loop, a third on looking at 43 images. Server time was 20.8 s; the run took 7.8 wall minutes of tool time.

## S1 / S2 / S3

| Slide | Shape used | Attempts | One slide or three things pasted? | Narrow side readable? |
|---|---|---|---|---|
| S1 market + 2.3% + bullets | `regions`, `arrangement: main_left`, `[chart(bar, 60) / stat(2.3%, 40) ; text(bullets, 60)]` | 1 to compile (v1); bullets trimmed once for the 12pt floor | One slide: one title, one source line, chart left, number over bullets right; right column top-heavy (blank band below) | Yes: 12pt+ bullets, checked at density 100 (n019-01) |
| S2 waterfall + "what it means" | `regions`, `arrangement: columns`, `[chart{type: waterfall, points increase/decrease/total} 66 / text{heading, body, bullets} 34]` | **3**: (1) `bridge` region in main_left -> count error; (2) `bridge` region in columns -> unknown region kind (contradictory list); (3) chart region type waterfall -> compiled | One slide; the product **cannot** place the `bridge` kind beside text, the waterfall *chart* is the substitute. Defect: y-axis starts at 22, opening bar a stub, no axis control | Yes: body and two bullets at ~12pt (n019-02) |
| S3 Keel case | `image_case` with `image.path`, `image_side: left`, eyebrow/heading/body/3 bullets, 2 `metrics`, caption | 1 to compile; 1 verified patch (remove `takeaway`) to lift the eyebrow above 9.4pt | One slide; photo left, story and the 9 pts / EUR 40M metrics right | Yes (n019-03) |

## The eight-row pricing schedule

The product said (n=11): "table has 9 rows including the header; the renderer lays out at most 7 — split it across two slides", remediation `split_slide {max_rows: 7, row: 4}`. I split into two `table` slides (weeks 1–2: 4 rows; weeks 3–4: 4 rows + a bold total row, `totals_row: true`). **No row was lost** (images n021-08, n021-09). Cost: two slides whose lower half is empty, both scored 100.

## The eight required elements

| Required element | How expressed | Attempts | Result |
|---|---|---|---|
| S1 market chart + share + bullets | `regions` main_left | 1 | OK, one slide |
| S2 bridge beside narrative | `regions` columns with waterfall chart | 3 | OK as a chart; bridge kind cannot be a region; axis truncated |
| S3 Keel photo + story + metrics | `image_case` | 1 (+1 verified patch) | OK |
| B vs C two-column | `comparison` 2 columns x 3 items | 1 (trimmed from 5 items on an info finding) | OK; sparse |
| Three-option evaluation | `option_matrix` 3 x 4, harvey, recommended B, decisive "Confidence on price risk" | 1 | OK, best-looking slide |
| Eight-row pricing schedule | `table` split into 2 slides at row 4 | 2 | All rows kept; under-filled |
| Team with headshot + initials | `team`, `members[0].photo` | 1 | OK (avatar in circle, MR / C / C initials) |
| Next steps with Friday ask | `next_steps` 5 actions + 2 decisions | 1 | OK |

## Metrics asked for

- Bytes read before the first slide was authored: **100,631** (n=1–10).
- Validate calls to first `deterministic_ready: true`: **3** (n=11, 12, 15); render calls: **1** (n=16).
- Repair rounds after the first render: **1** (footer). Open after review: P2 waterfall axis (slide 4), P2/P3 under-filled tables (9, 10), P3 sparse comparison (7).

### Slides the tools scored clean that looked wrong
- Slide 4 (p-style and blue-corporate): waterfall y-axis from 22 -> `log/images/n019-02.jpeg`, `n021-03.jpeg`, `n025-04.jpeg`. Score 100, no finding.
- Slides 2–12 v4: footer "Meridian Capital…" truncated -> `n017-02.jpeg` … `n017-12.jpeg`. No finding.
- Slides 9–10: 4–5-row tables in the top third -> `n021-08.jpeg`, `n021-09.jpeg`. Score 100.
- blue-corporate slide 12: date column at two sizes, decisions band on the footer line -> `n025-12.jpeg`. No finding.

### Where I had to guess a field or shape
- `bridge` as a region kind: wrong, and the error's available list said it was right (n=12). Resolved by inference from `recommend_visual`'s waterfall data contract (n=13).
- `alt` inside `chart`: wrong; the error named the exact accepted keys and gave a verified patch (n=15).
- Waterfall point types `increase/decrease/total`, `members[].photo`, `image.path`, `arrangement` values, `scale: harvey`: all taken from the kind briefs / data contracts, no guessing needed.

## Cross-template: blue-corporate (same spec, no restyling)
Nothing broke structurally; all 12 slides render with the assets in place. What degraded: spaced all-caps titles wrap every title to two lines and crowd the content (slides 3, 4, 6, 9, 10); the recommended option's name wraps in the highlight row (6); next-steps dates render at two sizes and the decisions band touches the footer (12); the waterfall's two blues are less distinct than p-style's accents (4). Images: `log/images/n025-01.jpeg` … `n025-12.jpeg`.

## Top five things that would make an agent love this tool
1. Let `regions` host `bridge` (or rank a regions/compose candidate first when the intent says "beside"), and make the unknown-region-kind error list the region kinds.
2. Measure the footer chrome like the takeaway and raise a finding instead of ellipsising a client name.
3. Zero baseline for waterfalls by default (or a `y_min`), and a chart finding when autoscaling hides an opening total.
4. Let an 8-row one-line table fit on one slide, or stretch split tables; flag a 4-row table alone on a slide as under-used.
5. Have `plan_deck` route on the kind catalogue's own words (waterfall, photo, schedule, team) and fill the slots with the matched facts.

## Final artefacts
- p-style, reviewed and approved: `/tmp/jj/wave/f-deals-pitch/out/falcon-pstyle-v5.pptx` (content_hash 6e681df9…, deck_id deck_5c883a74e6d586607d4be42e1df30fe8 rev 5). Superseded: `out/falcon-pstyle-v4.pptx`.
- blue-corporate (same spec, fork deck_e85cad8f464baaeacdea0bec5d5fad30): `/tmp/jj/wave/f-deals-pitch/out/falcon-bluecorp.pptx`.
- Specs sent: `spec-v1.json` (full), `spec-v2-patch.json`, `spec-v3-patch.json` (patch ops on the deck_id), `spec-v4.json` (stored spec after the `alt` removal, first ready render), `spec-v5.json` (final; blue-corporate = v5 with `template: blue-corporate`).
- Images inspected (43): `log/images/n013-01..04` (previews), `n017-01..12` (v4), `n019-01..04` (density 100), `n021-01..11` (v5 final pass), `n025-01..12` (blue-corporate).
