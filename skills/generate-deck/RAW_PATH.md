# Raw PresentationInput path

Read this only when the requested slide cannot be expressed by DeckSpec, when
editing an existing raw deck, or when making a targeted low-level repair.
These preconditions do **not** apply to ordinary semantic DeckSpec authoring.
Use [SKILL.md](SKILL.md) for the final-revision visual-inspection rule.

## Discover, validate, generate

Call `get_started` first and check its `runtime`. For a raw slide, use
`recommend_visual` if the visual form is undecided, then `list_patterns`
and `show_pattern` for the selected pattern's live value schema and
`example_values`. Pattern `values` can be an object *or* an array;
do not guess. `expand_pattern` returns the editable `shape_grid`, density
warnings and cell budgets. Preserve its `source: "pattern:<name>"` stamp
when editing an expanded grid: it marks template-derived font sizing;
removing it can make those same sizes invalid in constrained mode.

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
`fix.params.cause:"placeholder_not_found"` means the target placeholder did
not exist and strict mode fails generation. Read
`placeholders_dropped`, not just `placeholders_used`, when a slide looks
empty. If a finding code is unfamiliar, use `describe_finding`; for
repairable kinds query
`get_capabilities().vocabularies.repair_fix_kinds`.

Apply precise `repair_slide` fixes to a single raw slide. For findings on
several slides, `propose_repairs` plus `repair_slides_batch` avoids repeated
round trips. A finding can recommend an advisory kind whose remedy is an
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
diagrams such as swot / pestel / bmc) with bounds in slide coordinates. `apply_deck_patch` is an atomic structural
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

## Finish

Render with `render_deck_thumbnails`, inspect **every** slide image, repair
anything visibly wrong, and repeat. Re-render changed slides promptly; the
final full-deck pass must cover the current revision. If the server lacks
render tooling, say the artifact is **UNREVIEWED**. The detailed four-phase
workflow, progress notifications, idempotency, and visual-review protocol
are in [WORKFLOW.md](WORKFLOW.md).
