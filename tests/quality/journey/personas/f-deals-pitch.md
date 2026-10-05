# f-deals-pitch: a commercial due diligence proposal to a PE fund (MCP, default tool profile)

You are a consulting engagement manager's AI assistant. The deals team hands you this brief:

> Build the proposal deck for "Project Falcon": Meridian Capital Partners is considering acquiring Nordbolt, a European industrial fastener maker (revenue EUR 212M, EBITDA EUR 31M, 14.6% margin). We are pitching a 4-week commercial due diligence. 10–12 slides on `p-style` when `$REPO/templates/p-style.pptx` exists, otherwise `blue-corporate`. Facts: the European fastener market grew from EUR 8.1bn (2021) to EUR 9.4bn (2025), forecast EUR 10.6bn by 2028; Nordbolt holds 2.3% share, number 6 in Europe; top three competitors are Bossard (9.1%), Würth (7.4%) and Fabory (3.8%). Nordbolt's revenue by segment: automotive 41%, construction 27%, machinery 22%, other 10%. Margin bridge 2023→2025: 2023 EBITDA 24.0, price +6.5, volume +3.2, raw material −4.1, opex +1.4 = 31.0. Three scope options for the DD: (A) desktop-only market model, EUR 180k, 3 weeks; (B) market model plus 25 customer interviews, EUR 320k, 4 weeks; (C) B plus a channel-partner survey, EUR 410k, 5 weeks — we recommend B. Workplan (B): week 1 data room and market model, week 2 interviews wave 1, week 3 interviews wave 2 and synthesis, week 4 red-flag report and IC pack. Team: Anna Lindqvist (partner, 18 years industrials), Marco Rossi (manager, 3 prior fastener DDs), two consultants. A comparable: last year's DD on "Project Keel" (fastener distributor) found a 9-point price-gap risk that moved the bid by EUR 40M. Fees for B: market model 120k, interviews 140k, synthesis and IC pack 60k. Pricing schedule has eight line items you may invent sensibly (phase, deliverable, days, fee). The ask: confirm scope B and sign the engagement letter by Friday.

Three slides are **required to be split or complex layouts**, exactly as described, and you must say in your report how you expressed each one and how many attempts it took:

- **S1** — market chart on the LEFT (bar chart 2021/2025/2028), and on the RIGHT a big share number ("2.3%") above three or four short bullets about Nordbolt's position. One title, one source.
- **S2** — the margin bridge as a waterfall taking about two thirds of the width, and on the remaining third a short "what this means for the bid" narrative. If the product cannot place a bridge beside text, say so and show what you did instead.
- **S3** — the Project Keel case: the site photo at `$J/assets/site-photo.png` on one side, the story and the "9 points / EUR 40M" result metrics on the other.

Also required: a two-column comparison of scope options B vs C, the three-option evaluation (criteria: cost, duration, confidence on price risk, confidence on volume risk), the eight-row pricing schedule (eight rows is more than one slide of your table kind may hold: do what the product tells you, do not silently drop rows), the team with the headshot at `$J/assets/headshot.png` for the partner and initials for the others, and a next-steps closer with the Friday ask.

## Do

1. Start the bridge with the default tool profile (no `--tools`). Read `log/initialize.json` and `tools/list`; follow what the product tells you to do first.
2. Plan the storyline, author the DeckSpec, validate, repair, render.
3. Render every slide (`render_deck_thumbnails`), look at every image, repair what looks wrong (three rounds at most), and finish the completion protocol (`submit_visual_review`).
4. Then render the same spec on the other template (`blue-corporate` if you started on `p-style`, else `midnight-blue`) and look at every slide again. Do not restyle the content for it; record what broke.
5. Save every spec revision you sent as `$J/<persona>/spec-vN.json` (or .yaml) and keep the final PPTX paths in `report.md`.

## Measure

- For S1, S2, S3: the kind/field shape you used, attempts until it rendered as intended, whether the result looked like one slide or like three things pasted together, and whether text on the narrow side stayed readable.
- For the eight-row table: what the product said, what you did, and whether any row was lost.
- Bytes read before the first slide was authored; validate and render calls to the first `deterministic_ready: true`.
- Every slide the tools scored clean that looked wrong, with the image path.
- Where you had to guess a field name or a shape and whether the error told you the right one.
