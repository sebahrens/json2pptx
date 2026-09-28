# Raw PresentationInput path

For features DeckSpec cannot express, existing raw decks, or low-level repairs.
Ordinary DeckSpec authoring and final visual review follow [SKILL.md](SKILL.md).

## Discover, validate, generate

Call `get_started` first; check `runtime`. Use `recommend_visual` if undecided,
then `list_patterns` and `show_pattern` for live schemas and `example_values`.
Pattern `values` may be an object or array. `expand_pattern` returns editable
`shape_grid`, density warnings and cell budgets. Keep its
`source: "pattern:<name>"` stamp: template-derived sizes may otherwise fail
constrained mode.

Run `validate_input` with `fit_report:true` before
`generate_presentation`. Use `strict_unknown_keys` when authoring fresh
JSON so misspelled fields fail early. Replace every `__FILL__` skeleton token
with authored content; use `placeholder_policy:"strict"` for a publishable
run. `design_mode` is a **deck field**, `contrast_check` is a **slide field**,
and typed content uses the matching `*_value` property declared by
`get_input_schema`. In constrained mode use template scheme colors and
template-managed type sizes. Use `design_mode:"free"` only when explicit
hex colors or absolute sizing are intentional.

For a reusable deck use canonical `layout_id` values discovered from
`list_templates` / `examine_template`, not a template-specific
`slideLayoutN` identifier. A pinned layout number is appropriate only for
an inspected, fixed template. The content-as-array shape, placeholder IDs,
and shape-grid properties are in
[TEMPLATE_GUIDE.md](../template-deck/TEMPLATE_GUIDE.md); inspect the live
template to verify actual bounds. A raw deck may use top-level
`structure` instead of `slides[]`, but not both.

Raw content supports URL and internal links. Put `link:{url:"https://…"}`
on a text/bullet item, `source_link:{url:"https://…"}` beside a slide source,
or `link:{slide:N}` on a grid shape or overlay badge. `N` is the destination
slide number in the **final** deck (1-based); validate after pagination or
reordering. See [INPUT_FORMAT.md](../../docs/INPUT_FORMAT.md).

## Diagnose and repair

`generate_presentation` defaults to strict output validation. A successful
response implies the written PPTX passed the blocking OPC and OOXML checks;
it says nothing about visual polish. `CONTENT_DROPPED` with
`fix.params.cause:"placeholder_not_found"` (target placeholder missing) or
`"placeholder_occupied"` (another block already fills it) is a refuse-class
drop and strict mode fails generation. Read
`placeholders_dropped`, not just `placeholders_used`, when a slide looks
empty. If a finding code is unfamiliar, use `describe_finding`; for
repairable kinds query
`get_capabilities().vocabularies.repair_fix_kinds`.

Actual source loss refuses every fit mode, including `warn` and `off`.
Preserve required content and the requested slide count; refused inputs and
stale outputs are not approved. Use `describe_finding` for repair guidance.
So does grid text below its role floor
(`TEXT_BELOW_READABLE_MIN`); a lone axis "1" is a caption.

Apply precise `repair_slide` fixes to one raw slide per call; with a raw
`deck_id` each call is small. `propose_repairs` plans several slides. A finding can recommend an advisory kind whose remedy is an
authoring decision; do not retry it as an executable fix. Text-reduction
repairs may refuse with `semantic_review_required` when they would remove
a number, unit, negation, or qualifier; split or rewrite deliberately.
Read [FINDINGS.md](FINDINGS.md) only if the live `describe_finding` response
does not cover the code. `score_deck.quality_gate.passed` stops structural
repair, but does not replace slide-image inspection.

The raw `deck_id` returned by generation is a short-lived server handle.
It can replace a `presentation` payload on later preview, repair, score,
rhythm, and regenerate calls. Keep your own source JSON; a
`read_presentation` extraction is for inspection, not an authoritative
round-trip `PresentationInput`; it includes shapes inside groups (native
diagrams such as swot / pestel / bmc), connectors, `pictures[]` and
`hyperlinks`. A `FONT_SUBSTITUTED` warning means renders may wrap where fit
findings did not; trust the image. `apply_deck_patch` is an atomic structural
transform for insertion, removal, replacement, move, duplicate, or existing
field replacement; validate and inspect the resulting deck before shipping.

## Images, charts, diagrams, and icons

For a local relative asset, send an explicit absolute `base_dir` to MCP
calls. CLI validation resolves relative to the input file directory.
Validation checks image, background, and icon paths; unsafe traversal,
symlink escapes, missing files, unset environment variables, oversized
assets, bad remote types, and unsafe SVG XML have distinct findings. Do not
assume a URL will fetch or an SVG is safe because it looks plausible.

Use bundled icon names from `list_icons`, qualified by set when necessary;
do not put emoji codepoints in deck JSON. A pattern icon slot may take a
bundled-name string or a full `IconInput` with exactly one of `name`,
`path`, `url`, or `svg_data`. Inline SVG can avoid a file roundtrip;
its `fill` override is ignored because it is already styled. Preview a
custom or recolored icon with `preview_icon` if appearance matters.
Template scheme colors are the portable default for icon fills.

The separate `svggen-mcp` server owns diagram construction. Use its
`get_started` and `get_diagram_schema` for the chosen diagram type, then
`validate_diagram` and `render_diagram`. To match the deck palette, call
`resolve_theme` with the same template and any `theme_override`, then
pass its `theme_colors` array to each diagram's
`style.theme_colors`. Use `validate_input` with the fit report enabled
after embedding a chart or diagram. A diagram collision or unreadable
native text is a content/layout problem: shorten labels or give the diagram
more space, then render the slide to pixels again.

## Raw planning

The storyline rules (QUALITY.md) are the same on this path. `plan_deck`
(default `format: "raw"`) returns ordered slides with a canonical `layout`
(`title` first, `closing` last, `blank-title` + a pattern between), a
`recommended_pattern`, `suggested_pattern_fallback`, narrative role and
`content_seed`, with brief facts verbatim in `facts[]` and the rest in
`unplaced_facts[]`. Comparison slots use only `comparison-2col` /
`before-after`; emphasis patterns are capped at ceil(n/5). Each slide's
`skeleton` is a partial `SlideInput`: copy it, replace every `__FILL__`,
review the typed choices in its `__CHOOSE__` speaker note and delete that
note. Leftover tokens are `unresolved_placeholder` warnings; run the
publishable pass with `placeholder_policy: "strict"` so they block.

Per slide intent, `recommend_visual` ranks layouts, patterns, charts,
diagrams and compose envelopes (`recommend_pattern` only when you already
need a named pattern). Take canonical layout IDs from `list_templates`; for
placeholder capacity request `mode="compact"` (`layout_summaries[].placeholders[].max_chars`)
or `fields="full"`.

## Rhythm before generation

`analyze_deck_rhythm` is a static check (no PPTX): `per_slide[]` fingerprints
(pattern, density class, accent role, `within_slide_accent_variety`) and
`aggregates` — `longest_run` (target ≤ 2), `repetition_index` (< 0.5),
`accent_balance` (no accent > 80%), `density_cv` (> 0.1 on 4+ slides),
`density_distribution` — plus `composition_score` and `recommendations` with
`recommended_break_patterns`. Act on it: swap the middle slide of a run of 3
to a suggested break pattern; add a stat/quote break when `density_cv` is
flat; set `cell_accent_mode: "progressive"` on a 5+ cell slide with one
accent; add detail or use a smaller grid when underfilled cells pass 30%;
iterate until the score is ≥ 70.

**Pre-emit checklist.** Tables within Rule 20 (rows ≤ 7, cols ≤ 6, font ≥
9pt, multiline cells counted); every fill semantic, never mixed with hex
(Rule 12); no `line` on a filled shape (`FILLED_SHAPE_OUTLINED`: separate
with the `gap` and neutral `dk1` tints, e.g. `{"color":"dk1","lumMod":4000,
"lumOff":96000}`); one solid accent per slide (peer cells neutral, accent as
a 3pt `accent_bar`); no sibling shapes closer than 4pt; every pattern cell at
35–110% `density_pct` from `expand_pattern` (PATTERNS.md).

## Generate and repair

`validate_input` with `fit_report: true` (CLI `json2pptx validate
--fit-report`, `--json` for the envelope) exits clean even with unfittable
cells; refusal comes from `strict_fit` on `generate_presentation`: `off`
(silent shrink/truncate), `warn` (default: shrink and report), `strict`
(refuse on overflow with `fix.kind` `split_at_row` / `reduce_text`, as a
FindingEnvelope with `IsError=true`). Every `findings` array is sorted by
(severity desc, slide index asc, code asc); deck-level findings precede slide
0, so work top-down. `preview_presentation_plan` dry-runs layout selection,
placeholder mapping and fit without writing a PPTX.

`repair_slide` takes the raw `deck_id` (or the deck JSON), `slide_index` and
`fixes: [{kind, params}]`; a handle repair persists and returns
`changed_slides` (`return_deck: true` for the JSON). Overflow: preserve
evidence — `split_bullets` (`max_items`) for native bullet columns,
`split_at_row` for tables, `shorten_title` only when meaning survives;
`reduce_text` / `reduce_cell_text` truncate and are last resorts. Wrong
layout → `swap_layout`. Surprise grey from `contrast_autofixed` → an accent
with ≥ 3:1 against white, `dk1` text, or `contrast_check: false` only after
verifying contrast yourself. For vision findings, `repair_slide` with
`{kind: "autofix_visual", params: {category}}` tries the mapped kinds in
order (e.g. `text_overflow` → `reduce_cell_text`, `split_at_row`,
`reshape_grid`).

## Finish

Render with `render_deck_thumbnails`, then apply the per-slide rubric, the
submission and the three-round loop cap in [WORKFLOW.md](WORKFLOW.md) → Phase
4 — the review protocol is identical on both paths. If the server lacks
render tooling, say the artifact is **UNREVIEWED**.
