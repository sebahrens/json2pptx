# c-repair journey: flawed draft -> diagnostics-only repair (default MCP profile)

## What I did, in order
1. Discovery (2 calls): `get_started(brief)`, `list_slide_kinds {}`.
2. Wrote a hurried draft, "Cloud cost reduction programme", for `modern` (`specs/v0.json`). It has 10 slides, not 8: the requested flaws need nine distinct slides plus a title.
3. Repaired using only findings, `next_tool_call`, remediation params and `describe_finding` (15 calls). From round 3 on I used `deck_id` + `patch`.
4. Rendered, looked at all 12 slides, repaired the render-only findings, looked again, submitted the review.
5. Rendered the same final spec (`specs/final.json`) on `midnight-blue` and `p-style`, looked at all 24 images, ran `score_deck` on all three.

Wall time about 10 minutes; 45 tool calls; 1.88 MB of responses, of which 1.58 MB were thumbnails.

## Round-trips
| # | call n | what I sent | result |
|---|---|---|---|
| V1 | 4 | full draft | 5 errors, 12 warnings, no deck_id |
| V2 | 12 | full spec, literal fixes | 2 errors, 5 warnings (4 of them caused by my fixes) |
| V3 | 16 | deck_id + 7 ops | 4 errors, 1 warning |
| V4 | 17 | the 4 suggested patches, literally | same 4 errors |
| V5 | 18 | rename value -> description | 10 errors + 8 infos (second tier appears) |
| V6 | 23 | suggested layout patch, titles, meta.source | ok=true, 4 infos (3 caused by the suggested patch) |
| V7 | 24 | undo layout, cut exec summary, retitle closer | no issues |
| R1 | 25 | render modern | pptx written, deterministic_ready=false (BODY_TOO_LONG) + 2 infos |
| R2 | 27 | shorten supports again | identical finding |
| R3 | 28 | remove one point | deterministic_ready=true, 2 infos |
| R4 | 31 | add detail to slides 4 and 9 | 0 diagnostics, score 100 |

Thumbnails: n=26 (12 slides) and n=32 (12 slides; 3 changed, 9 pixel-identical). Review: n=33, accepted first try, `visually_reviewed_current_revision`.

## Per-finding log
"Msg alone" = did the message tell me what to change and to what.

| Finding (call) | Severity | Msg alone | Repair worked first time | Ping-pong / notes |
|---|---|---|---|---|
| UNKNOWN_ARCHETYPE "consulting" (4) | error | yes | yes | my own guess, not planted |
| UNKNOWN_KIND funnel (4) | error | partially (27 kinds, no nearest) | no | -> REQUIRED steps, UNKNOWN_FIELD stages, TAKEAWAY_REQUIRED (12) -> UNKNOWN_FIELD value x4 (16) |
| PATTERN_DEGRADED exec summary, 6 points (4) | warning | yes for the count | yes for the count | -> budget finding (12) -> 10 fit errors (18) -> render block (25) |
| TAKEAWAY_REQUIRED x6 (4) | warning | yes | yes | the kpi one duplicates the `takeway` typo |
| UNKNOWN_FIELD takeway (4) | error | yes (did_you_mean) | yes | |
| PATTERN_DEGRADED 7 KPIs (4) | warning | yes | yes | dropped 1 KPI |
| CHART_SERIES_LENGTH_MISMATCH (4) | error | yes | yes | no remediation block, describe_finding was enough |
| UNKNOWN_FIELD bulets (4) | error | partially (key list, no suggestion) | yes | |
| PATTERN_DEGRADED comparison "found 3" (4) | warning | no (wrong cause) | n/a | vanished when bulets was fixed |
| SEMANTIC_DENSITY table 10 rows (4) | warning | yes | yes | -> RHYTHM_MONOTONY (12) |
| PATTERN_DEGRADED decision label (4, 12, 16) | warning | yes, one option per call | yes each time | 3 round-trips for 3 labels; path is the list |
| PATTERN_DEGRADED process step (4, 12) | warning | yes, one step per call | yes after I cut all 8 | slide still cramped |
| PATTERN_DEGRADED exec budget (12) | warning | partially (no index, no length, 3 budgets) | no | passed here, failed at 18 |
| RHYTHM_MONOTONY (12) | warning | partially (run not listed) | yes | leaves a divider between table halves, unflagged |
| UNKNOWN_FIELD stages (12) | error | partially | yes | |
| UNKNOWN_FIELD steps[i].value x4 (16) | error | yes | no (suggested patch is a no-op) | 1 wasted round-trip |
| TEXT_BELOW_READABLE_MIN x10 (18) | error | partially ("shorten", budget only in params) | no (suggested patch swaps layout) | -> NO_EXECUTIVE_SUMMARY, BODY_TOO_LONG, SLIDE_TEXT_DENSE (23) |
| FIT.BODY_TOO_LONG 420pt vs 349pt (18) | info | partially | - | path /slides/1/pattern |
| TITLE_NOT_ACTION x3 (18) | info | yes | yes | 6 other topic titles exempt, never shown |
| DATA_WITHOUT_SOURCE x4 (18) | info | yes | yes (one meta.source op) | |
| NO_EXECUTIVE_SUMMARY / BODY_TOO_LONG / SLIDE_TEXT_DENSE (23) | info | no (caused by the tool's own patch) | undone by removing layout | 2 of the 3 are duplicates |
| CLOSING_WITHOUT_NEXT_STEPS (23) | info | yes | yes | first shown on the 6th validate |
| BODY_TOO_LONG at render (25) | warning, action review, blocks gate | partially (3 advices, no numbers) | no (advice 1 failed at 27; advice 3 worked at 28) | render-only; no semantic_path |
| SLIDE_UNDERUSED (25) | info | partially ("add detail") | yes | render-only |
| SPARSE_FILL (25) | info | partially (mentions non-DeckSpec knobs) | yes | render-only |

Summary counts:
- Fix caused a new finding: 5 times (kind change, exec count, table split, suggested layout patch, cutting to cell_max_chars then failing at render).
- Suggested repair failed first time: 4 (unknown-field replace, layout swap, lead/support budget, "shorten the sentences" at render).
- Findings without a path into my spec, or pointing at compiled objects: FIT.BODY_TOO_LONG (`/slides/1/pattern`), NO_EXECUTIVE_SUMMARY (`/slides/1`), render BODY_TOO_LONG (raw_path only), `params.cell_path` on the fit errors, contrast_predicted (`/slides/7/content/Section Number`).
- Severity not matching outcome: render BODY_TOO_LONG is "warning"/"review" yet blocks `deterministic_ready`; the same finding is "info" with ok=true when validating with template=modern (probe n=40). TEXT_BELOW_READABLE_MIN is "error" in the finding, "refuse" in evidence, "review" in describe_finding.
- Duplicates: 10x TEXT_BELOW_READABLE_MIN, 4x UNKNOWN_FIELD value, BODY_TOO_LONG + SLIDE_TEXT_DENSE, takeway -> two findings.
- Render-only findings: BODY_TOO_LONG, SLIDE_UNDERUSED, SPARSE_FILL (and title_wraps / contrast_predicted on p-style).
- Never flagged: six topic-label titles (documented takeaway exemption), chart with no unit, the section divider inside a split table.

## Text cut versus what the message said
| Slide | Draft | Final | What the messages said |
|---|---|---|---|
| Executive summary | 6 points, 1188 chars | 4 points, 270 chars (-77%) | "3-5 points", then "lead <=90, support <=200", then cell_max_chars 49/55, then "needs 337pt, holds 311pt" |
| Process (8 steps) | 1075 chars | 473 chars (-56%) | "a flow box holds 80": accurate |
| Decision labels | 81 / 80 / 78 | 43 / 46 / 49 | "a step holds 60": accurate |
| KPIs | 7 | 6 | "2-6": accurate |
| Table | 9 rows | 5 + 4 rows on two slides | "at most 7 including the header": accurate |

Probe n=41: four points with the long supports (747 chars) misses the floor by 0.5pt on modern, so most of the last cuts were not needed; the binding limit was the number of rows.

## Template switch (same final spec)
- **midnight-blue** (n=34): deterministic_ready=true, 0 diagnostics, score 100. Images: same content, no breakage; exec-summary rows have uneven heights; flow boxes are narrower and wrap to 6 lines.
- **p-style** (n=35): deterministic_ready=true, 2 infos: `title_wraps` on the closing title (max_chars 20, against the ~40 in the kind summary) and `contrast_predicted` on the section number (auto-adjusted, ratio 2.8 -> 3.0). Images: wider content area, bold flow-box text, closing title on two lines, nothing clipped. Applying the title patch literally (20 chars, n=38) cleared it without re-triggering the closing finding.
- After the p-style render, validating the deck_id (n=39) started returning p-style findings: the deck_id keeps the last render's template. My earlier validates had no template.

## score_deck versus the images
`score_deck` returned 100, composition 100, gate passed, no findings, on all three templates (n=43-45). That does not match what I saw. Nothing is broken or clipped, but four slides are weak: the executive summary is four one-line rows in a mostly empty slide, "Savings funnel" uses the left half only, a section divider splits one table in two, and the 8-box flow wraps 2-3 words per line. Half the content slides still have label titles. I approved all 12 in the review and attached those four as P2 findings.

## Top 5 things that would make an agent love this
1. Tell me everything in the first validate, for the template I will render on (or say which checks were skipped).
2. Make every suggested patch correct by construction: rename for unknown fields, replace-with-budget for over-long text, never a layout swap that contradicts an earlier warning.
3. One budget per field, stated once, with index, measured length and limit for every offender in the same response.
4. Severity that predicts the gate, and blocking reasons that name the code and slide.
5. A score that distinguishes "nothing broken" from "good", so 100 means I can stop looking.

## Artefacts
- Specs: `specs/v0.json` (draft), `specs/v1.json`, patch files `specs/a_p2..a_p6_validate.json`, `specs/a_r2..a_r4_render.json`, `specs/final.json`.
- Decks: `out/cloud-cost-modern.pptx` (reviewed revision 4fda4708...), `out/cloud-cost-midnight-blue.pptx`, `out/cloud-cost-p-style.pptx`, `out/cloud-cost-p-style-t20.pptx` (closing title cut to 20 chars).
- Images inspected: `img/modern1/s00-s11.png` (first render), `img/modern2/s00-s11.png` (final), `img/modern_hi/s01.png`, `s10.png`, `img/mb1/sheet0-2.png`, `img/ps1/sheet0-2.png`, `img/ps_hi/s09.png`.
- Raw responses: `r_*.json` / `r_*.txt`; running log `notes.md`; call log `log/calls.jsonl`.
