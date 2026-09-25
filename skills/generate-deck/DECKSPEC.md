# Semantic DeckSpec authoring

Read this when authoring or revising a DeckSpec. For live kind names, required
fields, aliases, examples, and supported compositions, call
`list_slide_kinds` (compact by default); request
`kinds:["<chosen-kind>"], fields:["item_schema","compositions"]` only for a
kind you intend to use. `validate_deck_spec` carries the authoritative closed
schema. Do not copy a static kind catalog from an older document.

## Plan the narrative

Use YAML or JSON with `meta` and either flat `slides[]` or chapter-based
`structure: {cover, auto_agenda, sections:[{title, slides:[]}], closing}`.
The forms are mutually exclusive. Chapters add numbered section dividers;
`meta.chrome.section_crumb: true` labels their content slides. The compiler
generates divider numbers: do not hand-author them. Keep an actual narrative
instead of creating one slide per layout or a sequence of interchangeable
cards. `explain_deck_spec` previews the resolved story and visual rhythm
without rendering.

When the request explicitly asks to exercise native layouts, put canonical
layout IDs in `meta.required_layouts`. Do not copy a
`recommend_visual.template_support.required_layout` value into that array:
it can name a capability rather than a canonical layout. Inspect
`explain_deck_spec.layout_coverage.{requested,assigned,missing}` and add a
compatible *narrative* slide for each missing layout. Do not solve coverage by
adding empty or unrelated slides. Template resolution order is
`meta.template`, then the tool's `template` argument, then the archetype
default. Discover archetypes with `list_deck_archetypes`.

`meta.chrome` controls confidentiality, client/project labels, date, page
numbers, and section crumbs. `meta.viewing_mode` and
`meta.accent_strategy` choose reading scale and accent rhythm. Every slide
kind can carry `notes` and `source`; the latter is rendered once even when
the chosen pattern has its own attribution band. `meta.type_scale` can be
`compact`, `comfortable` (default), or `presentation`; use a
`pattern.overrides.type_scale` only for a slide that needs a different scale.

## Validation and content budgets

The live kind schema is closed. Unknown fields are dropped by compilation
and surfaced as `SEMANTIC_UNKNOWN_FIELD`; treat even a warning as lost
content. A wrong JSON type is `SEMANTIC_FIELD_TYPE`, not a request to guess
the intended coercion. Run `validate_deck_spec` before rendering and correct
these at their source paths.

Visuals have content limits. The compiler degrades an out-of-range visual to
readable bullets or another content layout and reports
`SEMANTIC_PATTERN_DEGRADED` with `fix.params.{from,to,reason}`. This is not
the same as `SEMANTIC_DENSITY`, which advises about density without losing
the visual. Respond to `count_out_of_range` by changing the visual or count,
and to `budget_exceeded` by shortening or splitting content; do not silently
truncate facts. Check `explanation_summary.pattern` after render to confirm
the visual you expected actually landed.

The following are decision budgets *not fully expressed* by a compact kind
listing; get exact fields and aliases from `list_slide_kinds`:

| Content shape | Useful visual range and authoring consequence |
|---|---|
| Executive summary | Three to five conclusion/support points use the `exec-summary` pattern. Each lead is at most 90 characters and support at most 200; outside the range or budget, the compiler degrades to bullets. `points` or plural `takeaways` are body content; singular `takeaway` is the footer insight. |
| KPI snapshot | Two to six KPI cards. Keep values at most 12 characters, labels at most 40, deltas at most 12. A value beyond the hard budget degrades the slide; a value that fits the character budget but cannot fit in the card reports `BODY_TOO_LONG`. |
| Chart insight | One to six insights use chart-plus-insights; more use a native chart with the full insight list. Every series needs exactly one unquoted numeric value per category. A short series is `CHART_SERIES_LENGTH_MISMATCH`; a quoted/null value is `CHART_VALUE_NOT_NUMERIC`. Neither should be shipped as an empty plot. |
| Comparison | Two balanced columns of at most ten rows use a comparison visual; three to five columns use panels; larger content may degrade to cards or bullets. |
| Table | At most six headers and six body rows fit the semantic table budget. Use `option_matrix` for options scored against criteria instead of flattening that structure into a generic table. |
| Option matrix | Two to six criteria by two to six options. Each option needs exactly one score per criterion; use the chosen Harvey, RAG, or short text scale consistently. |
| Team | One to eight people use biography cards. Name at most 60 characters, role at most 80, bio at most 220, initials label at most 8. Every card needs a role; a photo and an initials label are alternatives. |
| Image case | Story body at most 300 characters, eyebrow 30, heading 80, at most five bullets of 140 each, at most three metrics with value 10 and label 40, caption 120. A picture without a stated case is not an image case. |
| Framework | SWOT, Five Forces, and BMC need every canonical part; at most ten items per part and 200 characters per item. A missing part degrades the whole framework to grouped bullets. |
| 2×2 matrix | Exactly four headed quadrants and both axes. Quadrant header at most 80 characters, body 200; horizontal axis 16, vertical axis 60, axis ends 11. Missing parts or over-budget copy degrade to named bullets. |
| Timeline | Three to seven milestones, label at most 60, date 30, body 200. Use ranges for periods; use a roadmap for parallel workstreams. Outside the budget, preserve dates in bullets. |
| Hero statistic | One value at most 20 characters, label 80, unit 10, context 120, source 80. For several equal-weight figures use KPI snapshot. |
| Agenda | Two to ten numbered sections; three to six with subtitles can use illustrated rows. Outside the range, use a numbered list, preserving the current-section marker. |
| Quotation | One named speaker uses a pull quote (at most 500 characters); three to eight named speakers use a cluster (at most 240 each). Two quotes or missing names degrade to quote bullets. |
| Bridge | Three to ten waterfall columns; component label at most 40 characters, unit 8, caption 60. Total/delta values are numeric; an implicit subtotal uses the running total. |
| Pillars | Three to five pillars. A house needs both objective and foundation, title at most 60, up to five bullets of 120, and up to three roof badges of 24. Without full framing use panels (title 80, one to eight bullets of 200). |
| Organization | One root, up to seven nodes, three levels, and four direct reports per node; labels at most 40 characters. Larger trees degrade to attributed bullets rather than dropping people. |
| Architecture | Three to six tiers; tier label at most 60 characters, detail at most 120, and side rail at most 30. Out-of-budget content degrades intact to bullets. |
| Process | Three to six described steps use numbered rows (label at most 60, description at most 180). Bare labels or decision branches use `process-flow` when within its range. A straight sequence is not a branching flowchart. |
| Roadmap | Three to six phases use a phase visual; otherwise choose a content layout that preserves every milestone. |
| Decision | Three to six options use numbered boxes (label at most 60, detail at most 180). Exactly two *detailed* options use paired cards (label at most 80, detail at most 300). Mark exactly one labeled option recommended and put the ask in `recommendation`. |

These are authoring budgets, not an alternate schema. For a kind absent from
this table, use the runtime example and schema. `list_slide_kinds` also
returns `required_aliases`, but prefer its canonical field names in new
content.

## Render and revise

Use `validate_deck_spec` → `render_deck_spec`. A render's
`success: true` means a file was written. `deterministic_ready` means
diagnostics, output validation, and the quality gate cleared; `publishable`
is false on a fresh render until a current all-slide visual verdict is
approved. The CLI exits successfully for a clean, unreviewed render but
not for a deterministic blocker under strict mode. Resolve `diagnostics[]`,
`deterministic_blocking_reasons[]`, and the quality gate before review.
After inspecting the images, use `submit_visual_review` and its returned
current-revision status for the final verdict; re-rendering creates a new
unreviewed response rather than refreshing the earlier one.
Each diagnostic has a `semantic_path` to edit and a
`raw_path` only as fallback. Keep the DeckSpec as the source of truth.
After each revision, render and inspect the affected slides, then inspect all
slides of the final revision as required by [SKILL.md](SKILL.md).

Validation, explain, and render return a `deck_id` handle. You can send
`deck_id` instead of `spec`, with an optional ordered JSON-Pointer
`patch:[{op,path,value}]` (`add`, `remove`, `replace`). Do not send
both `spec` and `deck_id`. Patch application is atomic and
`changed_slides` identifies the 0-based slides to re-render first. Handles
are process-local and expire after one hour; retain the source spec yourself.
Without an explicit `output_filename`, the rendered filename includes a
digest so two distinct specs do not collide. Reusing an explicit name may
overwrite the earlier artifact; check `overwrote`.

`raw_json2pptx` is a deliberate escape hatch inside a DeckSpec. Its
`slide` must be a structurally valid raw slide with a layout/type and
renderable content (except a deliberate blank slide), and has the same
`meta.design_mode` constraints as a raw deck. A raw hex fill or absolute
font size in constrained mode is refused; use free mode only when low-level
control is intentional. For a visual beyond semantic reach, query the
chosen kind's `compositions`; an unsupported override reports
`SEMANTIC_PATTERN_NOT_AVAILABLE`. Use a minimal raw slide rather than
lowering the entire deck unnecessarily.

Runnable examples live in [examples/semantic](../../examples/semantic/).
The compiler architecture and schema are in
[docs/SEMANTIC_COMPILER.md](../../docs/SEMANTIC_COMPILER.md).
