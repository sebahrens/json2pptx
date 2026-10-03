# b-discovery: finding the right layout for 14 slide intents (midnight-blue)

Two sessions against `/tmp/j2p mcp`: default tool profile (100 calls, `log/`) and `--tools all` (31 calls, `log-all/`). Call numbers below are `D n=` (default) and `A n=` (all). Machine clock: 10:27 to 10:36.

Caveats on "without prior knowledge": I did not read the repo. My harness context did include the project's CLAUDE.md, which lists pattern names and one-line descriptions (not their data shapes), so pattern *names* were not entirely new to me. I opened the `preview_png_path` files that `recommend_visual` returned, since the product handed me those paths; they live under the repo's `assets/pattern-previews/`.

## Journey, default profile

1. `initialize` instructions, `tools/list` (41.6 KB, 12 tools), `get_started` (12.2 KB), `list_slide_kinds` (22.9 KB, 27 kinds, no pictures).
2. `recommend_visual` once per intent with `template: midnight-blue` (D n=5..18, 71.9 KB total).
3. `list_slide_kinds` with `item_schema` for 13 kinds (59 KB) and for `raw_json2pptx` (2.7 KB; `slide` is an opaque object).
4. One single-slide DeckSpec per intent and per competing option: `validate_deck_spec` -> `render_deck_spec` -> `render_deck_thumbnails`, then looked at every image (24 slides).

Time went into (i) guessing `pattern.values` for patterns that have no schema in this profile, and (ii) rendering the runner-up options to see whether the top recommendation was really the best.

## Per-intent results (default profile)

Columns: (a) calls/bytes to know which visual; (b) to know shape and limits; (c) picture before authoring; (d) attempts to a valid render; best = was recommend_visual's #1 the best rendered option.

| # | Intent | (a) recommend_visual | (b) shape + limits | (c) preview | (d) attempts | #1 was best? |
|---|---|---|---|---|---|---|
| 1 | strategy house, 4 pillars, 2 enabling layers | 1 call, 9.1 KB: strategy-house 0.97, house_diagram 0.95 | pattern: none; found kind `pillars` in item_schema (~3.3 KB) | none | 2 (foundation array rejected, D n=22; string OK n=23) | Yes, but two layers are not expressible; house_diagram `floors` silently drops content (n=24, n=27) |
| 2 | five option cards | 1 call, 1.5 KB: card-grid only | none for card-grid; `comparison` + `pattern: card-grid` (2.7 KB) | file path; stale (blue 3x2) | comparison: 1. raw card-grid: 4 validates. `decision`: impossible at 5 options (n=36, n=47) | Yes (card-grid), reached via `comparison` |
| 3 | sales funnel with conversion rates | 1 call, 5.7 KB: chart funnel 0.97 with contract + runnable call | in the candidate | none | 1 | Yes |
| 4 | org chart of 7 | 1 call, 3.1 KB: org_chart 0.96 with contract | in the candidate; kind `org` 2.7 KB | none | 1 | Yes |
| 5 | line chart + KPI + 3-milestone timeline | 1 call, 10.7 KB: compose with **bar** chart and range-bar timeline | in the candidate; `regions` kind 11.7 KB found separately | none | 1 (regions), 1 (compose verbatim) | No: `regions` kind is the exact match and is never recommended |
| 6 | before / after of a process | 1 call, 6.1 KB: before-after 0.95 | none; guessed `{before:{header,items}, after:{...}}` | file path; style differs | 1 (guess was right) | Yes; fills only the top half |
| 7 | five-stage maturity, where we are | 1 call, 4.4 KB: journey-maturity-model 0.96 | none; guessed | none | validate OK, render failed on `values.current` (n=64), then OK with `stages[].current` (n=68) | Yes |
| 8 | value chain, one highlighted step | 1 call, 6.2 KB: value-chain pattern 0.96 = value_chain diagram 0.96 | pattern: none (guessed `steps[].highlight`); diagram: contract | none | pattern 1; diagram 2 (validate OK, render refused `highlight`) | Yes (pattern); the tie is not explained, and only the pattern can highlight |
| 9 | swimlane across three teams | 1 call, 6.3 KB: **team-bios 0.92** first, swimlane 4th | none; guessed `lanes[{actor,steps}]` | file path; no arrows, different style | 1, but the auto-drawn arrows describe the wrong flow | No |
| 10 | 2x2 with 6 plotted items | 1 call, 3.9 KB: matrix-2x2 pattern 0.95 = matrix_2x2 diagram 0.95 | diagram contract documents `points` (0-100 scale) | pattern only; looks broken | 1 | Tie; only the diagram plots items, and nothing says so |
| 11 | Gantt for 6 workstreams | 1 call, 4.5 KB: **roadmap-phased 0.97** above gantt 0.92 | gantt contract in candidate | roadmap-phased only | 1 each | No: gantt is the Gantt; roadmap-phased is a text grid |
| 12 | eight customer quotes | 1 call, 2.4 KB: quote-cluster 0.96 (cap 3-8) | kind `quote` 3.9 KB | none | 1 | Yes |
| 13 | 3 vendors x 5 criteria + recommendation | 1 call, 5.6 KB: **bar chart 0.85** first, table-highlight 0.80 last | kind `option_matrix` 5.8 KB | none | 1 | No: the last-ranked candidate is the right one |
| 14 | screenshot with two callouts | 1 call, 2.3 KB: image-text-split 0.76, image 0.88 (risky) | kind `image_case` 5.1 KB | layout only | 1 | Not expressible: picture beside bullets, no callouts |

Limits learned only from an error: `decision` needs exactly one `recommended: true` (in the schema text, but past where I had read); `decision` cannot take 5 options on this template; card-grid `cells` lives in `values` and each needs `header`; maturity `current` is per stage; value_chain diagram items take no `highlight`; `pillars.foundation` is one string.

## Default vs `--tools all`

| | default | `--tools all` |
|---|---|---|
| tools / `tools/list` | 12 / 41.6 KB | 49 / 335 KB (179 KB is outputSchema) |
| pattern catalogue | names only via recommend_visual; 22 patterns have no kind and no schema | `list_patterns` 17.9 KB (50 of 51 on page 1), `show_pattern` 5.5-9.6 KB each |
| data shape + limits for a pattern | guess, learn from errors | closed JSON Schema, min/max items, per-count char budgets, example_values |
| wrong key | misleading count error, wrong path | `validate_pattern`: did_you_mean + patch + pointer to show_pattern |
| picture before authoring | stale PNG file path for 15 of 26 recommended patterns | still none in show_pattern/list_patterns; example_values -> `render_slide_image_from_json` works (2 calls, ~55 KB, 1.1 s) |
| chart/diagram types | only those recommend_visual happens to return | `get_data_format_hints` 14.8 KB, 37 types |
| recommend_visual | same response, byte for byte (same fingerprint) | same; does not point to show_pattern |

What the default-profile agent misses: the exact shape of every non-kind pattern; overrides such as card-grid `style`, per-cell `recommended` and `vertical_align: stretch` (which turned the half-empty five-card slide into a full one); and the swimlane idiom of one step per column, visible only in show_pattern's example, which fixed the arrows (`img/i09-swimlane.jpg` vs `img/a09-swimlane.jpg`).

Is the full catalogue navigable? Yes for patterns: list_patterns' `use_when` lines ("prefer X when ...") are good signposts and show_pattern is authoritative. Less so for raw shape_grid (`get_input_schema`, 25 KB, has no field descriptions) and for knowing which of 49 tools matter; nothing connects recommend_visual -> show_pattern -> single-slide render. Intent 14 stayed out of reach in both profiles, and the attempt exposed two render defects (invisible callout shapes; "2." rendered as "1.").

## Top 5 things that would make an agent love this

1. Every recommend_visual candidate carries its data shape, limits and a runnable example, patterns included, and names the DeckSpec kind when one exists. Put show_pattern in the default profile.
2. One-call current preview of any kind/pattern/diagram on my template as an image block; retire the stale PNGs.
3. validate_deck_spec catches everything render_deck_spec refuses, and the first error on a raw pattern names the unknown key, the right path and the expected shape.
4. Fix the ranking for literal intents (swimlane, Gantt, options x criteria) and honour the chart type asked for; say when a tie hides a real difference (diagram plots points, pattern does not).
5. No silent content loss and no gate pass on placeholder text: house_diagram floors/sections, "Replace with the action title...", "2." -> "1.".

## Artefacts

- Final 14-slide deck (best option per intent): `/tmp/jj/j/b-discovery/out-all/final-14-intents.pptx`; contact sheet `/tmp/jj/j/b-discovery/img/final-contact.jpg`; slides `img/final-01.jpg` .. `final-14.jpg`. `deterministic_ready` is false only for NO_EXECUTIVE_SUMMARY (it is a layout sampler, not a storyline). No visual verdict was submitted.
- Per-intent single-slide decks: `/tmp/jj/j/b-discovery/out/i01-*.pptx` .. `i14-*.pptx` (24 files), images in `/tmp/jj/j/b-discovery/img/` (all inspected).
- All-profile experiments: `img/preview-*.jpg`, `img/a02-cardgrid-*.jpg`, `img/a09-swimlane.jpg`, `img/a14-callouts.jpg`, `out-all/i14-callouts.pptx`.
- Specs: `/tmp/jj/j/b-discovery/specs/`; recommend_visual responses: `rv/1.json` .. `rv/14.json`; tool definitions: `tools-default.json`, `tools-all.json`.
- Logs: `log/calls.jsonl` (default), `log-all/calls.jsonl` (all). Both bridge servers (the four PIDs I started) were stopped at the end.

Bridge note (not a product issue): `mcpd.py call ... '@file'` crashes because it runs `json.loads` on the argument before checking for `@`; I passed file contents inline instead.
