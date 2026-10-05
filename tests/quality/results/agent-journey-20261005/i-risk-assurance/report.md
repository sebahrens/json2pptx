# i-risk-assurance: FY26 ITGC audit-committee readout (MCP, default profile)

Enjoyment 7/10. 44 tool calls, 10.6 wall minutes, 4 validates and 1 render to the first `deterministic_ready: true` (call n=23), three repair rounds, completion status `visually_reviewed_current_revision` (call n=46).

## Journey in order

1. **Onboarding (n=1-3).** `initialize` instructions and `get_started(task: brief)` gave a clear sequence: plan_deck -> list_templates -> list_slide_kinds -> validate -> render -> thumbnails -> submit_visual_review, plus the completion rule. Read in one pass.
2. **Planning (n=4).** `plan_deck` produced a 12-slide skeleton (7 content + 5 structural) but left 24 facts unplaced, including all four required complex layouts. I kept its structure idea (exec summary, appendix section) and authored from the brief.
3. **Discovery (n=5-17).** `list_slide_kinds` with my chosen kinds gave copy-ready examples. `recommend_visual` with previews chose the RAG board (table-highlight / option_matrix) and the maturity chart (grouped_bar / chart_insight) well, but for "one finding on its own structured slide" it returned only a bullet layout; I found labeled-rows via the hidden `list_patterns` and a candidate shortlist. The option_matrix `scale` vocabulary (harvey | rag | text) was only in `item_schema`.
4. **Authoring + validation (n=18-22).** v1 had 3 blocking errors: the 7-row register exceeds the 7-row table cap (split required), the 5-option RAG board cannot hold option details, and later a chevron label too long for a 5-step strip (twice). Patches by deck_id fixed them in under a second each.
5. **Render + review (n=23-25).** First render was `deterministic_ready` with score 100. Thumbnails of all 14 slides, then density-100 renders of the five dense slides. Everything was legible; the two half-empty register slides and the y-axis-from-2 trend chart were the visible problems the tools scored clean.
6. **Repairs (n=26-37).** Round 1: turned the 3-row register remainder into a regions slide (table + 4 KPIs) - blocked twice (rows arrangement truncates, kpi-4up overflow), then rendered with tiny KPIs. Round 2: replaced the KPI region with a single stat ("7 of 7") - good. Round 3: added items to the roadmap phases - first blocked because items are folded into the description's 160-char limit; shortened, rendered, still a sparse lower third.
7. **Completion (n=38, 46).** Full-deck pass on the final revision, `submit_visual_review` with all 14 approved and three P2/P3 findings recorded -> `visually_reviewed_current_revision`.
8. **p-style (n=39-44).** Re-validated and rendered the final spec on p-style; all 14 slides inspected. My saved spec had drifted from the patched stored deck (slide 7), so one re-render. On p-style the footer chrome is truncated with an ellipsis and the second trend series is a very pale orange.

Where the time went: roughly a third in discovery (recommend_visual previews are 120-250 KB each and I ran six), a third in the register split and its regions repair, the rest in authoring and inspection.

## The four required complex slides (S1-S4)

| Slide | Shape used | Attempts | Outcome |
|---|---|---|---|
| S1 results-by-domain board | `option_matrix` kind (table-highlight pattern), criteria `Controls tested` and `Exceptions` with `scale: text`, `Rating` with `scale: rag`, `decisive_criterion: Rating` | 2 (details dropped after FIT.BODY_TOO_LONG) | RAG survived as red / amber / green dots with a legend on both templates (n025-01.jpeg, n041-04.jpeg) |
| S2 seven-finding register | `table` kind; product said max 7 rows incl. header -> split 4 + 3. Second half became a `regions` slide (table 64% + stat 36%) after two repair rounds | 1 validate to be told to split; 3 attempts on the second half | No finding dropped; the 4-row table fills only the top third (n025-03.jpeg); the 3-row half now sits beside a "7 of 7 actions accepted" stat (n033-01.jpeg) |
| S3 trend left, opinion right | `regions` kind, `arrangement: main_left`: line chart 60% / `stat` "Partially effective" / `text` with two bullets | 1 | Narrow column readable at 36% (n025-02.jpeg); chart y-axis starts at 2, no axis option to fix it |
| S4 maturity today vs FY28 | `chart_insight` with `grouped_bar`, `orientation: horizontal`, five domains as categories, Today vs FY28 target series; the product offered no row-aligned ladder (journey-maturity-model is a stage ladder, dual-org-ladder takes 2-4 rows) | 1 | Five domains aligned row by row with data labels (n024-10.jpeg) |

**Structured finding slides:** `raw_json2pptx` with the `labeled-rows` pattern (WHAT WE FOUND / WHY IT MATTERS / ACTION / OWNER AND DATE). The product did not offer it: `recommend_visual` returned only the bullet `content` layout (n=11); I found it through `list_patterns` (hidden) and a candidate shortlist (n=13). Result n024-08.jpeg, n024-09.jpeg.

**Appendix:** understood. `section` with `appendix: true` rendered unnumbered (n024-13.jpeg) and the statistics table is paged "A1" (n024-14.jpeg).

**Slides scored clean that looked wrong:** register halves (n025-03.jpeg, n024-07.jpeg) - half empty; trend chart baseline at 2 (n025-02.jpeg); roadmap lower third empty (n025-04.jpeg, n037-01.jpeg); p-style footer truncation and pale series (n041-02.jpeg, n041-05.jpeg).

## Five things that would make an agent love this tool

1. Measure tables against the content area instead of a fixed 7-row cap (or auto-split with repeated headers) - the register split cost two slides and two repair rounds.
2. Surface labeled-rows (and a `finding` kind) when the intent is a single issue / finding / risk card; today recommend_visual answers with bullets.
3. Publish region capacities for the `regions` kind and degrade the kpis region gracefully; three blind attempts for one table + KPI slide.
4. Return the stored spec after patches (or write it beside the pptx) so the deck I re-render elsewhere is the one I approved.
5. Make the least destructive patch the "verified fix" (drop details before dropping the visual), collapse repeated symptoms, and state character budgets in TEXT_EXCEEDS_SHAPE.

## Final artefacts

- forest-green (approved revision 216c2c8a...): `/tmp/jj/wave/i-risk-assurance/out/fy26-it-general-controls-review-3084a201.pptx`
- p-style: `/tmp/jj/wave/i-risk-assurance/out/fy26-it-general-controls-review-6ea9e9fc.pptx`
- Specs sent: `spec-v1.json` (first validate), `spec-v2.json` (split + details dropped), `spec-v3.json` (label / takeaway / subtitle patches, first render), `spec-v4.json` (regions table + kpis), `spec-v5.json` (final: stat region + roadmap items), `spec-v5-pstyle.json` (same on p-style). Patches as sent: `patch2.json` ... `patch8.json`.
- Images inspected: `log/images/n024-01..14.jpeg` (first full render), `n025-01..05.jpeg` (density 100), `n030-01`, `n033-01`, `n037-01` (repair rounds), `n038-01..14` (final full pass), `n041-01..14` and `n044-01` (p-style); previews `n009-*`, `n010-*`, `n011-01`, `n013-*`, `n014-*`, `n015-*`.
