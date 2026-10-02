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

Two raw-only patterns cover pages DeckSpec kinds do not: `contact-directory`
(key contacts / "who to call": grouped rows of circular headshots, names and
titles, up to 24 people) and `text-sidebar` (prose introduction or foreword
beside one large key-message panel). Prefer `comparison-2col` /
`before-after` to `slide_type: "comparison"` (`COMPARISON_PREFER_PATTERN`).

For a reusable deck use canonical `layout_id` values discovered from
`list_templates` / `examine_template`, not a template-specific
`slideLayoutN` identifier. A pinned layout number is appropriate only for
an inspected, fixed template. A raw `shape_grid.columns` is a whole number
1-24 or an array of at most 24 widths; `validate` rejects anything else
(`INVALID_GRID`). The content-as-array shape, placeholder IDs,
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
empty.

Actual source loss refuses every fit mode, including `warn` and `off`, and so
does grid text below its role floor (`TEXT_BELOW_READABLE_MIN`; a lone axis
"1" is a caption). Preserve required content and the requested slide count;
refused inputs and stale outputs are not approved.

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
findings did not; trust the image (Calibri measures with the embedded, metric-identical Carlito on every host and is not reported). `apply_deck_patch` is an atomic structural
transform for insertion, removal, replacement, move, duplicate, or existing
field replacement; validate and inspect the resulting deck before shipping.

## Images, charts, diagrams, and icons

For a local relative asset, send an explicit absolute `base_dir` to MCP
calls. CLI validation resolves relative to the input file directory.
`base_dir` bounds relative asset paths only: an absolute image, background
or icon path is read wherever it points unless the server sets
`ALLOWED_IMAGE_PATHS` (which `icon.path` also obeys). Asset paths expand only
`$HOME`, `$BRAND_ASSETS`, `$JSON2PPTX_*`. Unsafe traversal, symlink escapes,
missing files, unset variables, oversized assets, bad remote types, and unsafe
SVG XML have distinct findings; a picture that cannot be embedded is
refuse-class `IMAGE_ASSET_UNAVAILABLE` at the authored field and blocks
`deterministic_ready`. `score_deck` fetches image `url`s like generation
(`URL_FETCH_FAILED` refuses); a `deck_id` keeps the authored `url`,
re-fetched on each render.

A `slide_type: "image"` slide on a template without a picture layout fills
One Content's body with the whole picture (no crop); cover discarding over 30%
reports `IMAGE_HEAVY_CROP`, and a photo over the footer band drops that
slide's chrome (`CHROME_OVER_IMAGE`). A shape-grid `image` cell accepts
`geometry: "ellipse"` for a circular frame and `fit: "contain"` to keep a
screenshot whole (RULES.md 6d). Numbered callouts on a screenshot use overlay
`kind: "callout"` with an `anchor_image` target in source-image fractions or
pixels, so leaders stay on their pixels through crop and layout changes; a
target the crop hides reports `OVERLAY_TARGET_CROPPED` (RULES.md 6f).

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

A `swot` or `porters_five_forces` diagram also renders as native shapes in a
`shape_grid` cell (`{"diagram": {"type": "swot", ...}}`), so it can sit on
`blank-title` under the same title as pattern slides; the other native-only
types (pestel, bmc, value_chain, ...) still need a body placeholder.

Diagrams use one hue by default: timeline bars/milestones, matrix_2x2
points and org_chart levels stay in accent1 and its tints (org levels take
one accent each only under `accent_strategy` `rotate` / `section-keyed`),
and matrix_2x2 quadrants share a neutral wash. To make
one quadrant carry the message, set `data.highlight_quadrant` (index 0-3,
`"top-left"`-style position, or the quadrant's label); give points a `series`
only when they really belong to different groups.

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
need a named pattern). A compose candidate's `next_tool_call.args_template.spec.slides[0].slide`
is a ready raw slide (`layout_id: "blank-title"`, title, `compose`); its
`composition.regions[].data_contract.field_path` names each region's sample
content. Take canonical layout IDs from `list_templates`; for
placeholder capacity request `mode="compact"` (`layout_summaries[].placeholders[].max_chars`)
or `fields="full"`.

## Rhythm before generation

`analyze_deck_rhythm` is a static check (no PPTX): `per_slide[]` fingerprints
(pattern, density class, accent role, `within_slide_accent_variety`) and
`aggregates` — `longest_run` (target ≤ 2), `repetition_index` (< 0.5),
`accent_balance` (no accent > 80%), `density_cv` (> 0.1 on 4+ slides),
`density_distribution` — plus `composition_score` and `recommendations` with
`recommended_break_patterns` and a `code`. Act on it: swap the middle slide
of a run of 3 (`break_run`) to a suggested break pattern — suggestions follow
the slide's content (numbers → KPI / stat, options → comparison, dates →
timeline; never a timeline without dates); add detail or use a smaller grid
when underfilled cells pass 30% (`underfilled_cells`). Narrative checks:
`missing_executive_summary` (6+ slides), `missing_next_steps` (no
next-steps close), `missing_sections` (10+ content slides, no divider or
agenda), `evidence_missing_takeaway_or_source`, `bullets_heavy` (3+
bullets-only slides). A `compose` slide's `pattern` names its structure —
direction, region families, a `*` on a region holding ≥ 12.5 points over an
equal share, nested envelopes in place, e.g. `compose:v[kpi*+pull-quote]` or
`compose:h[chart+compose:v[kpi+pull-quote]]` — so differently composed
slides do not form a run, while the same regions reordered, re-split 55/45
or swapped within a family (`kpi-3up` → `kpi-4up`) still do; its
`break_run` alternatives skip the run's region families. Accent checks: `accent_heavy_slide`,
`strong_accent_run` (see RULES.md). `accent_balance` counts pattern slides
by their resolved accent (`accent_strategy` or `overrides.accent`).
Iterate until the score is ≥ 70 and the narrative codes are gone.

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
