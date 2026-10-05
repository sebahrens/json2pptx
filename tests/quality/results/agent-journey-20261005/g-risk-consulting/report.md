# g-risk-consulting — journey report

Harbour Bank ERM uplift proposal, 11 slides, `midnight-blue` then forked to `p-style`. 28 tool calls, 7.9 wall minutes of server session, 2 refused renders, enjoyment 7/10.

## What I did, in order

1. **Onboarding (n=1–3).** `initialize.json` and `get_started(task: brief)` gave a clear sequence: plan_deck → list_templates → list_slide_kinds → validate → render → thumbnails → submit_visual_review, plus the completion protocol.
2. **plan_deck (n=4).** Disappointing: with `format: deckspec` it mapped the losses-chart slide to `executive_summary`, the heat map to `table`, the next-steps closer to `process` (then appended its own `next_steps`), and left 27 brief facts unplaced even though the brief named the slides they belong to. I kept only its slide order.
3. **Discovery (n=5–15).** `list_slide_kinds` with `kinds:[…]` and `fields:["item_schema"]` was the real source of truth (budgets, aliases, examples). `recommend_visual` with `preview:true` for the heat map showed heatmap / matrix-2x2 / capability-heatmap / table-highlight images (n=9) — that is how I found both the S1 and S3 shapes. For the status-board intent it ranked kpi-5up/kpi-inline/metric-list (n=10); table-highlight (option_matrix, scale rag) only surfaced when I named it in `candidates` (n=12). The roadmap kind's schema has no parallel tracks (n=14); `roadmap-phased` is raw-only (n=15).
4. **Authoring + validate (n=16–17).** spec-v1: 2 blocking errors, both on the status board (option_matrix): details too long, and a 288pt-vs-253pt height claim with 5–6pt measured text whose "verified" fix was `layout: content` (bullets). Removing the five short details (the other verified patch) cleared everything in one call. Validate also told me to drop the takeaway, then warned `SEMANTIC_TAKEAWAY_REQUIRED` when I did.
5. **First render (n=18).** `deterministic_ready: true` on the first render (2 validates, 1 render to ready). Thumbnails (n=19) — looked at all 11.
6. **Repair round 1 (n=21–24).** Fixes: swap the 2×2 point matrix for a capability-heatmap used as a likelihood × impact grid, shorten KPI comparators, remove the roadmap milestone, retitle two slides. The render refused twice (n=21, n=22) on raw-pattern labels at 9.6pt whose error paths pointed at generated grid cells (`/slides/9/pattern/rows/3/cells/0`) rather than my fields; each refusal exposed only the next failing shape. Third attempt (n=23) ok; known_hashes re-render returned only the 4 changed slides (n=24).
7. **Completion (n=26).** submit_visual_review with the 11 content hashes (7 from the first render, 4 from the re-render) → `visually_reviewed_current_revision`. One of three repair rounds used.
8. **p-style (n=27–28).** One `fork` + `patch meta.template` call, deterministic_ready, looked at all 11 slides. Larger render of the status board at density 100 (n=29); score_deck 99/100 (n=30).

## S1–S4

| Slide | Shape used | Attempts | Status / highlight survived? | Narrow column readable? | Degraded to bullets? |
|---|---|---|---|---|---|
| S1 risk-appetite dashboard | `option_matrix` (table-highlight), criteria Status (scale rag) / Current (text) / Limit (text), `recommended: ["Operational","Cyber"]`, `highlight_label: "Breached"`, `decisive_criterion: Status` | 2 (details had to be removed; the verified alternative was `layout: content`) | Yes: green/amber/red dots, two breached rows tinted with a "Breached" badge and left bar on both templates (n029-01, n028-04). On p-style the column and row tints share a hue (A13). | text 12pt+ confirmed at density 100 | No |
| S2 losses chart + headline | `regions`, `arrangement: main_left`, chart (62%) + stat "EUR 11.2M" with peer-median context + text region with 2 bullets | 1 | n/a | Yes (n019-05, n028-05) | No |
| S3 heat map | Attempt 1: raw `matrix_2x2` diagram with six labelled points — all six placed, but the four "medium" risks sat on the axis lines (n019-06). Attempt 2 (kept): raw `capability-heatmap` with Low/Medium/High-likelihood columns and High/Medium/Low-impact tiers; all six named risks land in the right likelihood column with the right impact colour (n024-02, n028-06). | 2 | Yes, colour = impact, legend present | n/a | No |
| S4 three lines today vs target | `comparison` with two columns of six items aligned row by row | 1 | n/a | Yes (n019-07, n028-07); `pattern_overcrowded` info (14 cells > 8) but it rendered cleanly | No |

How I found the heat map: `recommend_visual(intent: "likelihood x impact risk heat map with six named risks…", preview: true)` (n=9) ranked `heatmap` diagram (numbers only), `matrix-2x2` kind, `capability-heatmap`, `table-highlight`; a shortlist call (n=13) gave the `matrix_2x2` diagram's `points[{label,x,y}]` contract. No single form takes named items in a 3×3 likelihood × impact grid (A8).

Validate/render to first `deterministic_ready: true`: 2 validates (n=16, n=17), 1 render (n=18).

## Slides the tools scored clean that looked wrong

- Rev 3 slide 6 (first heat map): only an info `diagram.text_overlap`; four risk markers sat exactly on the quadrant dividers — `log/images/n019-06.jpeg`.
- Rev 3 slide 10 (first roadmap): clean, but the milestone forced a tall third lane with a tiny diamond label — `log/images/n019-10.jpeg`.
- Slide 3 kpi-4up: flagged `fit_overflow` on every pass yet renders with the lower half empty (the opposite problem) — `log/images/n024-01.jpeg`.
- p-style slides 3, 4, 9, 10: wide empty band under one-line titles, nothing flagged — `log/images/n028-03.jpeg`, `log/images/n028-10.jpeg`.

## Top five things that would make an agent love this tool

1. A plan_deck that routes a brief's named slides and enumerated facts to the right kinds (A1).
2. recommend_visual understanding status / RAG / limit vocabulary, and a risk heat-map form that takes named items per likelihood × impact cell (A2, A8).
3. Parallel tracks on the DeckSpec roadmap kind so a consulting roadmap never needs the raw path (A3).
4. Raw-pattern refusals that list every unreadable field at once, at its `pattern.values` path with the offending text (A5, A11).
5. Remediations ordered by how much of the visual they preserve — never lead with "switch to content layout" when dropping five short details fixes it (A4, A7).

## Final artefacts

- Final midnight-blue PPTX: `/tmp/jj/wave/g-risk-consulting/out/harbour-bank-erm-uplift-proposal-c4496453.pptx` (deck_id `deck_917a70a046a599ad1314c29f5cefefd6`, revision 4, content_hash `17937cc860ca1e821b3f851f99743e2cc34b1d9e2b5de72eaeddee0dea6fbafc`, status `visually_reviewed_current_revision`)
- First render (rev 3): `/tmp/jj/wave/g-risk-consulting/out/harbour-bank-erm-uplift-proposal-40b05065.pptx`
- p-style PPTX: `/tmp/jj/wave/g-risk-consulting/out/harbour-bank-erm-p-style.pptx` (deck_id `deck_d84b14c281bf67404e5048cc8b7e68e3`)
- Specs: `spec-v1.json` (as authored), `spec-v2.json` (stored rev 3 after the validate patches), `spec-v3.json` (stored rev 4, final), `spec-v4-pstyle.json` (rev 4 with template p-style, sent as a fork patch)
- Patches sent: `patch-r1.json`, `patch-r1b.json`, `patch-r1c.json`; review args `review-args.json`
- Images inspected: `log/images/n009-01..04` (heat-map previews), `n010-01..04` (status-board previews), `n019-01..11` (rev 3, all slides), `n024-01..04` (rev 4 changed slides), `n028-01..11` (p-style, all slides), `n029-01` (status board at density 100)
