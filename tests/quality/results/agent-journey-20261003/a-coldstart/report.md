# a-coldstart: Q3 FY26 board deck, cold start over MCP (default tool profile)

Both decks finished the completion protocol (`visually_reviewed_current_revision`, `publishable: true`). The warm-coral deck took 4 renders and 3 thumbnail passes; the modern-template copy was refused on the first try and needed content cuts on 3 slides.

## Journey, in order

| Step | Calls (n) | What happened | Size / time |
|---|---|---|---|
| Onboarding | 1-4 | Read `initialize` instructions and tools/list, called `get_started`. Clear on the path; it also lists tools and files I do not have. | tools/list 41.6KB, get_started 12.2KB |
| Discovery | 5-9 | `plan_deck` (kind sequence usable, fact routing and structure not), `list_slide_kinds` catalog, `item_schema` for 10 kinds, one `recommend_visual` (not usable), `list_templates` for warm-coral. | 4.4KB, 22.9KB, 36.7KB, 4.8KB, 2.1KB; all under 0.05s |
| Authoring | - | Wrote a 6.4KB flat DeckSpec, 10 slides, from the examples. No field names guessed; `chart.type` copied from the example. | - |
| Validation | 10-12 | Error 1: two recommended options. Fixed by patch. Then infos: gantt timeline drops `body`, title wraps to 3 lines. | 1.2KB, 4.1KB, 1.6KB |
| Render | 13-14 | First render wrote the file but the gate failed (timeline labels over 36 chars, caused by following the validator's advice). Second render was deterministic-ready. | 11.5KB, 10.2KB; 0.35s each |
| Review 1 | 15 | Looked at all 10 slides. Wrong: gantt timeline (three identical full-width bars), decision slide badging one option under a title recommending two, title orphan, subtitle wrap. | 625KB, 2.6s |
| Repair | 16-19 | Two patch rounds, changed-slide thumbnails, then a full pass. 9 of 10 final image hashes matched ones already inspected; looked at the new one. | 8.6KB + 218KB, 9.7KB + 624KB |
| Completion | 20 | `submit_visual_review` accepted first time, 10/10 images verified. | 3.2KB |
| Second template | 21-27, 29 | Patch `/meta/template` -> refused. `validate_deck_spec` showed 10 errors on 3 slides. Three validate rounds (one made worse by the tool's own advice), render, full thumbnail pass, review accepted. | 9.0KB, 15.7KB, 3.2KB, 0.5KB, 8.5KB, 733KB, 3.0KB |
| Probe | 28 | Confirmed the timeline kind has no `style` field the validator told me to choose. | 2.0KB |

Totals: 26 tool calls, 2.38MB of responses, of which 2.2MB is thumbnails and 177KB is JSON. Wall time about 8 minutes by the system clock; server time is negligible.

Where the effort went: reading schemas (60KB of catalog and item_schema), and the two places the tool's guidance sent me the wrong way (timeline, option matrix on modern-template).

## What the second template took

- 7 calls instead of the 3 it should take (render, thumbnails, review).
- Content cuts: three exec-summary supports shortened, option details removed from the option matrix, two decisions merged into one on next steps.
- modern-template has a shorter content area and all-caps titles; neither is visible before rendering.
- The patch overwrote the stored deck, so the two delivered files differ on slides 1, 6 and 9, and the deck_id now holds the modern-template version.

## Top 5 things that would make an agent love this tool

1. **Make "looks wrong" and "scores 100" disagree less.** The gantt timeline (A1) and the half-empty decision and KPI slides (A13) passed every check. Scale range bars to a time axis and let sparse patterns fill or centre.
2. **Phrase every remedy in DeckSpec fields, and make sure it helps.** Advice named `style`, `show_legend`, `bounds/max_height_pct` and `compose`, none of which the kind exposes, and dropping `highlight_label` increased the overflow (A3, A4).
3. **Report one root cause per slide, the same from render and validate.** "Needs 310pt, holds 304pt" should be the error, with the cheapest cut and its saving, rather than ten per-cell font errors with the cause marked info (A2).
4. **Give budgets up front, per template.** `list_slide_kinds(template=...)` returning per-field character budgets, plus a way to validate one spec against several templates and to render with a template override without mutating the deck (A8, A14).
5. **Trim discovery to the active profile and to canonical fields.** Drop references to absent tools and SKILL.md, enumerate chart types, hide alias keys, and fix `plan_deck` fact routing so its draft can be filled rather than rewritten (A5, A7, A10).

Keep: deck_id + patch, changed_slides -> slide_indices, pixel content hashes with verified review, sub-second validate and render, copy-ready examples, and the default chrome (takeaway band, source, page numbers, highlighted bar).

## Open point for the user (content, not tooling)

The brief says revenue is +14% YoY, but the five-quarter series starts at 41.0, which makes Q3 FY26 vs Q3 FY25 +17.6% (48.2 / 41.0). +14% matches 42.3, the second value. The decks state "+14% YoY" on the summary and KPI slides as given and label the chart Q3 FY25 to Q3 FY26; one of the two needs correcting before the board sees it. Next-step dates are month-level (Oct, Nov, Dec 2026) because the brief gave only the window; the timeline assumes an October 2026 start.

## Artefacts

- `/tmp/jj/j/a-coldstart/out/q3-fy26-board-warm-coral.pptx` (10 slides, sha256 50c2e005..., reviewed and approved, call n=20)
- `/tmp/jj/j/a-coldstart/out/q3-fy26-board-modern-template.pptx` (10 slides, sha256 89f65be6..., reviewed and approved, call n=27)
- `/tmp/jj/j/a-coldstart/findings.json` (17 findings, 8 delights)
- `/tmp/jj/j/a-coldstart/spec_v1.json` (first authored spec; later edits are the patches in `r1.json`-`r4.json`, `mv2.json`, `mv3.json`)
- `/tmp/jj/j/a-coldstart/log/calls.jsonl` and `log/resp-NNN.json`
- Slide images inspected, in `/tmp/jj/j/a-coldstart/slides/`:
  - warm-coral final revision: `img-1791016288420-1.jpeg` to `img-1791016288422-10.jpeg` (slides 1-10)
  - warm-coral earlier defects: `img-1791016216822-9.jpeg` (gantt bars), `img-1791016216822-8.jpeg` (one option badged), `img-1791016216821-5.jpeg` (title orphan)
  - modern-template: `img-1791016361189-1.jpeg` to `img-1791016361192-10.jpeg` (slides 1-10)

Note on isolation: the pattern-preview PNG cited in A6 is a path the server returned; I opened that one file and nothing else in the repository. The bridge's `@file` argument form failed (it parses JSON before checking for `@`), so I passed file contents inline; that is a harness issue, not a product one.
