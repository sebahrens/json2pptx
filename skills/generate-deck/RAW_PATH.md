# Raw PresentationInput path

For features DeckSpec cannot express, existing raw decks, or low-level repairs.
Ordinary DeckSpec authoring and final visual review follow [SKILL.md](SKILL.md).

## Discover, validate, generate

Use `recommend_visual` if undecided,
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
`get_input_schema`. In constrained mode use scheme colors and template
type sizes; hex is refused everywhere, tables and nested grids included.
Use `design_mode:"free"` only when explicit hex colors or absolute sizing
are intentional.

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

Raw content supports links: `link:{url:"https://…"}` on a text/bullet item,
`source_link:{url:"https://…"}` beside a slide source, or `link:{slide:N}`
on a grid shape or overlay badge (`N` is the 1-based slide number in the
**final** deck). See [INPUT_FORMAT.md](../../docs/INPUT_FORMAT.md).

## Diagnose and repair

`generate_presentation` defaults to strict output validation: success means
the PPTX passed the blocking OPC / OOXML checks, not that it looks right.
Refuse-class `CONTENT_DROPPED` (FINDINGS.md) refuses the deck before a file
is written. Under `output_validation:"warn"`/`"off"` it is written without
the block; read `placeholders_dropped` then.

Actual source loss refuses every fit mode, including `warn` and `off`, and so
does grid text below its role floor (`TEXT_BELOW_READABLE_MIN`; a lone axis
"1" is a caption). Preserve required content and the requested slide count;
refused inputs and stale outputs are not approved.

Apply precise `repair_slide` fixes to one raw slide per call (small with a
raw `deck_id`); `propose_repairs` plans several. Advisory kinds and
`semantic_review_required` refusals are authoring decisions, not retries
([FINDINGS.md](FINDINGS.md), when `describe_finding` does not cover a code).

The raw `deck_id` from generation is a short-lived handle that can replace
the `presentation` payload on preview, repair, score, rhythm and regenerate
calls. Keep your own source JSON: `read_presentation` is for inspection, not
a round-trip `PresentationInput`. A `FONT_SUBSTITUTED` warning means renders
may wrap where fit findings did not; trust the image. `apply_deck_patch` is an atomic structural
transform for insertion, removal, replacement, move, duplicate, or existing
field replacement; validate and inspect the resulting deck before shipping.

## Images, charts, diagrams, and icons

For a local relative asset, send an absolute `base_dir` to MCP calls (CLI
resolves relative to the input file). `base_dir` bounds relative paths only:
an absolute image, background or icon path is read wherever it points unless
the server sets `ALLOWED_IMAGE_PATHS`. Asset paths expand only `$HOME`,
`$BRAND_ASSETS`, `$JSON2PPTX_*`. A picture that cannot be embedded is
refuse-class `IMAGE_ASSET_UNAVAILABLE` at the authored field and blocks
`deterministic_ready`; `score_deck` fetches image `url`s like generation
(`URL_FETCH_FAILED` refuses).

A `slide_type: "image"` slide on a template without a picture layout fills
One Content's body with the whole picture; cover discarding over 30% reports
`IMAGE_HEAVY_CROP`, and a photo over the footer band drops that slide's
chrome (`CHROME_OVER_IMAGE`). Image-cell `geometry` / `fit`: RULES.md 6d;
screenshot callouts (`anchor_image`): RULES.md 6f.

Use bundled icon names from `list_icons` (set-qualified when needed), never
emoji. A pattern icon slot takes a bundled-name string or an `IconInput`
with exactly one of `name`, `path`, `url`, `svg_data`; inline SVG ignores a
`fill` override (already styled). Check custom or recolored icons with
`preview_icon`; template scheme colors are the portable icon fill.

The separate `svggen-mcp` server builds diagrams: its `get_started` and
`get_diagram_schema`, then `validate_diagram` and `render_diagram`. For the
deck palette, pass `resolve_theme`'s `theme_colors` (same template and
`theme_override`) to each diagram's `style.theme_colors`; `validate_input`
judges contrast against the deck's `theme_override` colours too. Run
`validate_input` with the fit report after embedding one; a collision means
shorter labels or more space.

Native types (`swot`, `porters_five_forces`, `pestel`,
`business_model_canvas`, `value_chain`, `nine_box_talent`, `kpi_dashboard`,
`process_flow`, `heatmap`, `pyramid`, `house_diagram`, `panel_layout`; aliases
`icon_columns` / `icon_rows` / `stat_cards`) also render as shapes in a
`shape_grid` cell or `compose` segment. Text under 7pt is refused as
`DIAGRAM_REGION_TOO_SMALL` (`fix.params.min_width_emu` / `min_height_emu`):
enlarge it, use `diagram_value` or cut items. A too-wide word is
`TEXT_EXCEEDS_SHAPE` (`fix.params.words`); text shrunk to fit,
`TEXT_BELOW_READABLE_MIN` (`max_items_per_*`, `max_chars_per_item`). A
`kpi_dashboard` metric where lower is better takes
`good_direction: "down"`.
Diagram/chart data keys are strict at every level: an undrawn key is
`unknown_key`.

Diagrams use one hue by default (accent1 and its tints, neutral surfaces;
framework diagrams take accent-tinted cards only with `style.colors`). To make
one quadrant carry the message, set `data.highlight_quadrant` (index 0-3,
`"top-left"`-style position, or the quadrant's label); give points a `series`
only when they really belong to different groups.

## Raw planning

The storyline rules (QUALITY.md) are the same on this path. `plan_deck`
(default `format: "raw"`) returns ordered slides with a canonical `layout`
(`title` first, `closing` last, `blank-title` + a pattern between), a
`recommended_pattern`, narrative role and `content_seed`, with brief facts
verbatim in `facts[]` and the rest in `unplaced_facts[]` (a metric written
as three or more values routes to `chart-insights-split`); region clauses plan
one `composition` slide (misses in `unsupported_regions[]`). Each slide's
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
`density_distribution`, `motif_runs` (3+ consecutive content slides in one
visual motif, whatever patterns drew them; target none), `motif_share`,
`dominant_motif` (one motif on more than half of 4+ content slides) — plus
`composition_score` and `recommendations` with `recommended_break_patterns`
and a `code` ("slide N" in a `message` is 1-based, one more than its
`slide_index`). `per_slide[].motif` is what the slide looks like (`tiles`,
`open-columns`, `open-list`, `flow`, `chart`, …); an explicit style changes
it (`stylish-panels` `ribbon` is `tiles`). Swap the middle slide of a run of 3 (`break_run`;
`break_motif_run` when the patterns differ but the look does not) to a
suggested pattern; keep `(1/2)` / `(2/2)` parts adjacent
(`continuation_interrupted`); add detail or a smaller grid when
underfilled cells pass 30% (`underfilled_cells`). Narrative checks:
`missing_executive_summary` (6+ slides), `missing_next_steps` (no
next-steps close), `missing_sections` (13+ content slides, no divider or
agenda), `evidence_missing_takeaway_or_source`, `bullets_heavy` (3+
bullets-only slides). A `compose` slide's `pattern` names its structure
(`compose:h[chart+compose:v[kpi+pull-quote]]`); reordered, re-split or
same-family swaps (`kpi-3up` → `kpi-4up`) still form a run. Accent checks: `accent_heavy_slide`,
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
layout → `swap_layout`. Surprise grey from `contrast_autofixed`: RULES.md
16. For vision findings, `{kind: "autofix_visual", params: {category}}`
tries the mapped kinds in order.

## Finish

Render, then review as in [WORKFLOW.md](WORKFLOW.md) → Phase 4.
