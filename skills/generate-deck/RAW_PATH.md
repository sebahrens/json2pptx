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

`generate_presentation` defaults to strict output validation: success means
the PPTX passed the blocking OPC / OOXML checks, not that it looks right.
Refuse-class `CONTENT_DROPPED` (FINDINGS.md) fails generation; read
`placeholders_dropped`, not just `placeholders_used`, when a slide looks
empty.

Actual source loss refuses every fit mode, including `warn` and `off`, and so
does grid text below its role floor (`TEXT_BELOW_READABLE_MIN`; a lone axis
"1" is a caption). Preserve required content and the requested slide count;
refused inputs and stale outputs are not approved.

Apply precise `repair_slide` fixes to one raw slide per call (small with a
raw `deck_id`); `propose_repairs` plans several. Advisory kinds and
`semantic_review_required` refusals are authoring decisions, not retries
([FINDINGS.md](FINDINGS.md), when `describe_finding` does not cover a code).
`score_deck.quality_gate.passed` stops structural repair, not slide-image
inspection.

The raw `deck_id` from generation is a short-lived handle that can replace
the `presentation` payload on preview, repair, score, rhythm and regenerate
calls. Keep your own source JSON: `read_presentation` is for inspection, not
a round-trip `PresentationInput` (it includes grouped shapes such as native
swot / pestel / bmc diagrams, connectors, `pictures[]` and `hyperlinks`). A `FONT_SUBSTITUTED` warning means renders may wrap where fit
findings did not; trust the image (Calibri measures with the embedded, metric-identical Carlito on every host and is not reported). `apply_deck_patch` is an atomic structural
transform for insertion, removal, replacement, move, duplicate, or existing
field replacement; validate and inspect the resulting deck before shipping.

## Images, charts, diagrams, and icons

For a local relative asset, send an absolute `base_dir` to MCP calls (CLI
resolves relative to the input file). `base_dir` bounds relative paths only:
an absolute image, background or icon path is read wherever it points unless
the server sets `ALLOWED_IMAGE_PATHS` (`icon.path` obeys it too). Asset paths expand only
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
slide's chrome (`CHROME_OVER_IMAGE`). Image-cell `geometry` / `fit`: RULES.md
6d; screenshot callouts (`anchor_image`): RULES.md 6f.

Use bundled icon names from `list_icons` (set-qualified when needed), never
emoji. A pattern icon slot takes a bundled-name string or an `IconInput`
with exactly one of `name`, `path`, `url`, `svg_data`; inline SVG ignores a
`fill` override (already styled). Check custom or recolored icons with
`preview_icon`; template scheme colors are the portable icon fill.

The separate `svggen-mcp` server builds diagrams: its `get_started` and
`get_diagram_schema`, then `validate_diagram` and `render_diagram`. For the
deck palette, pass `resolve_theme`'s `theme_colors` (same template and
`theme_override`) to each diagram's `style.theme_colors`. Run
`validate_input` with the fit report after embedding a chart or diagram; a
collision or unreadable native text means shorter labels or more space, then
render the slide again.

Native types (`swot`, `porters_five_forces`, `pestel`,
`business_model_canvas`, `value_chain`, `nine_box_talent`, `kpi_dashboard`,
`process_flow`, `heatmap`, `pyramid`, `house_diagram`, `panel_layout`; aliases
`icon_columns` / `icon_rows` / `stat_cards`) also render as editable shapes in
a `shape_grid` cell or `compose` segment, sized to it
(`get_diagram_capabilities`: `grid_cell_support`, `pipeline` `native_ooxml`;
svggen `svg`). Text under the 7pt floor is refused with
`DIAGRAM_REGION_TOO_SMALL` (`fix.params.min_width_emu` / `min_height_emu`):
enlarge the region, use a body placeholder (`diagram_value`) or cut items;
dense canvases need most of the slide. A word too wide for its shape reports
`TEXT_EXCEEDS_SHAPE` (`fix.params.words`). Data keys are strict: an undrawn
key (pyramid `levels[].title`, not `label`) is `unknown_key`, fix `rename_field`.

Diagrams use one hue by default: timeline bars/milestones, matrix_2x2
points and org_chart levels stay in accent1 and its tints (org levels take
one accent each only under `accent_strategy` `rotate` / `section-keyed`),
and matrix_2x2 quadrants share a neutral wash. To make
one quadrant carry the message, set `data.highlight_quadrant` (index 0-3,
`"top-left"`-style position, or the quadrant's label); give points a `series`
only when they really belong to different groups.

## Raw planning

The storyline rules (QUALITY.md) are the same on this path. Region clauses
plan one `composition` slide (a `shape_grid` skeleton with `regions[]` cell
paths; misses in `unsupported_regions[]`). `plan_deck`
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
need a named pattern); a compose candidate's
`next_tool_call.args_template.spec.slides[0].slide` is a ready raw slide
(TOOLS.md). Take canonical layout IDs from `list_templates`; for
placeholder capacity request `mode="compact"` (`layout_summaries[].placeholders[].max_chars`)
or `fields="full"`.

## Rhythm before generation

`analyze_deck_rhythm` is a static check (no PPTX): `per_slide[]` fingerprints
(pattern, density class, accent role, `within_slide_accent_variety`) and
`aggregates` — `longest_run` (target ≤ 2), `repetition_index` (< 0.5),
`accent_balance` (no accent > 80%), `density_cv` (> 0.1 on 4+ slides),
`density_distribution` — plus `composition_score` and `recommendations` with
`recommended_break_patterns` and a `code`. Swap the middle slide of a run
of 3 (`break_run`) to a suggested pattern (picked from content: numbers → KPI,
options → comparison, dates → timeline); add detail or a smaller grid when
underfilled cells pass 30% (`underfilled_cells`). Narrative checks:
`missing_executive_summary` (6+ slides), `missing_next_steps` (no
next-steps close), `missing_sections` (10+ content slides, no divider or
agenda), `evidence_missing_takeaway_or_source`, `bullets_heavy` (3+
bullets-only slides). A `compose` slide's `pattern` names its structure
(direction, region families, `*` = region ≥ 12.5 points over an equal share,
nesting: `compose:v[kpi*+pull-quote]`, `compose:h[chart+compose:v[kpi+pull-quote]]`);
reordered, re-split or same-family swaps (`kpi-3up` → `kpi-4up`) still form a
run, and `break_run` skips the run's families. Accent checks: `accent_heavy_slide`,
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
(refuse on overflow with `fix.kind` `split_at_row` / `reduce_text`, a
FindingEnvelope with `IsError=true`). Work `findings` top-down (FINDINGS.md
order). `preview_presentation_plan` dry-runs layout selection, placeholder
mapping and fit without writing a PPTX.

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
