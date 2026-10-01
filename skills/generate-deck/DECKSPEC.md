# Semantic DeckSpec authoring

Read this when authoring or revising a DeckSpec. For live kind names, required
fields, aliases, examples, and supported compositions, call
`list_slide_kinds` (compact by default); request
`kinds:["<chosen-kind>"], fields:["item_schema","compositions"]` only for a
kind you intend to use. `item_schema` is the closed contract; an alias is a
`$ref`. Do not copy a static kind catalog from an older document.

## Plan the narrative

Write the storyline first ([QUALITY.md](QUALITY.md)). `plan_deck` with
`format:"deckspec"` drafts it from the brief: an option evaluation gets
`option_matrix` before `decision`, a chart only for a fact that changes over
time, `cause` only when the brief names a problem, `decision` only when it
asks for one, and a customer update gets highlights and a dated `timeline`.
`deck_spec` holds the kinds with `__FILL__` titles (and `meta.date`); a budget
of 8+ slides comes back as `structure` (auto agenda + 2–4 sections, whose
dividers count toward the budget). `slots[]` gives each slot's `path`,
guidance and routed brief facts: metrics to KPI/stat slots, to-dos and asks
to the plan, decision and next steps, dated milestones to the timeline.
Send `spec` as a JSON object or a YAML/JSON document string (both pass the
advertised schema). Use YAML or JSON with `meta` and either flat `slides[]` or chapter-based
`structure: {cover, auto_agenda, sections:[{title, slides:[]}], closing}`.
The forms are mutually exclusive. Chapters add numbered section dividers;
`meta.chrome.section_crumb: true` labels their content slides in the footer;
`meta.chrome.tracker: true` sets the section name above each content title. The compiler
generates divider numbers: do not hand-author them. A section with
`appendix: true` (or a `section` slide with `appendix: true`) is back
matter: its divider is unnumbered and later chapters keep their numbers,
`auto_agenda` leaves it out, and its slides' tracker / crumb read
"Appendix: <title>". Dividers titled Appendix, Backup, Annex, Q&A or Thank
you are unnumbered without the flag. Appendix slides (after an appendix /
Appendix-titled divider) are outside the rhythm run checks, so backup tables
in a row are fine. With page numbers on, appendix slides read A1, A2, … in the footer
(an "Appendix B: …" divider numbers B1, B2) and the appendix divider shows
none; the main deck keeps contiguous numbers and `{total}` counts only it. `plan_deck` drafts this appendix itself when the brief has
backup material and the budget has room: a last `appendix: true` section
(the `next_steps` close then ends the last chapter) or, flat, an appendix
`section` slide after the close; its slots carry `appendix: true`. Keep an actual narrative
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
numbers, and section crumbs. Without `meta.chrome` a deck renders consulting
chrome by default: page numbers on every slide but the title and closing,
`meta.date` in the footer, and the tracker when the deck has sections.
`chrome.page_numbers.enabled: false` turns the numbers off. `meta.viewing_mode` and
`meta.accent_strategy` choose reading scale and accent rhythm (enum values per
the schema; others are `SEMANTIC_REQUIRED`). Every slide
kind can carry `notes` and `source`; the latter is rendered once, in the 9pt
source zone above the footer, even when the chosen pattern has its own
attribution field (or set `meta.source` once: every data slide without its
own source renders it). A data slide without one draws `DATA_WITHOUT_SOURCE` at
`slides[N].source`. `meta.type_scale` can be
`compact`, `comfortable` (default), or `presentation` for the whole deck; a
slide's `pattern` is a string, never an object.

## Validation and content budgets

The live kind schema is closed. Unknown fields are dropped by compilation
and surfaced as `SEMANTIC_UNKNOWN_FIELD`; treat even a warning as lost
content. A wrong JSON type is `SEMANTIC_FIELD_TYPE`, not a request to guess
the intended coercion. Numbers in text positions render as plain decimals;
quote literals such as `"1.10"`. `SEMANTIC_REFERENCE_UNRESOLVED` means a
`recommended` / `decisive_criterion` / agenda `current` matched nothing. Run
`validate_deck_spec` before rendering and correct
these at their source paths.

Visuals have content limits. The compiler degrades an out-of-range visual to
readable bullets or another content layout and reports
`SEMANTIC_PATTERN_DEGRADED` with `fix.params.{from,to,reason}`. This is not
the same as `SEMANTIC_DENSITY`, which advises about density without losing
the visual. Respond to `count_out_of_range` by changing the visual or count,
and to `budget_exceeded` by shortening or splitting content; do not silently
truncate facts. Check `explanation_summary.pattern` after render to confirm
the visual you expected actually landed (a degraded slide reports its compiled
`layout` / `visual_family`).

The following are decision budgets *not fully expressed* by a compact kind
listing; get exact fields and aliases from `list_slide_kinds`:

| Content shape | Useful visual range and authoring consequence |
|---|---|
| Executive summary | Three to five conclusion/support points use the `exec-summary` pattern (lead column 45% of the width). Hard limits: lead 90 characters, support 200, bottom line 160; past them the compiler degrades to bullets. Readable averages per point (lead / support): 3 points 90 / 200 (90 / 193 with a `bottom_line`); 4 points 90 / 128 (57 / 65 with a `bottom_line`); 5 points 52 / 60. On render the pattern measures the real content area and reports `BODY_TOO_LONG` only when the text does not fit at a readable size. Points with no `support` (plain strings) span the lead across the width. `points` or plural `takeaways` are body content. The slide has one conclusion band: `bottom_line` when given, else `takeaway` — a `takeaway` beside a `bottom_line` goes to the speaker notes with `SEMANTIC_DUPLICATE_CALLOUT`. |
| KPI snapshot | Two to six KPI cards. Keep values at most 12 characters, labels at most 40, deltas at most 12, `comparator` (alias `vs`, e.g. "vs plan +4 pts") at most 24 — give each KPI its reference. A value beyond the hard budget degrades the slide; a value that fits the character budget but cannot fit in the card reports `BODY_TOO_LONG`. |
| Chart insight | One to six insights use chart-plus-insights; more use a native chart with the full insight list. Every series needs exactly one unquoted numeric value per category. A short series is `CHART_SERIES_LENGTH_MISMATCH`; a quoted/null value is `CHART_VALUE_NOT_NUMERIC`. Neither should be shipped as an empty plot. |
| Comparison | Two balanced columns of at most ten rows use a comparison visual; three to five columns use panels; larger content may degrade to cards or bullets. |
| Table | At most six headers and six body rows fit the semantic table budget. Use `option_matrix` for options scored against criteria instead of flattening that structure into a generic table. |
| Option matrix | Two to six criteria by two to six options; one score per criterion on one Harvey, RAG, or text scale. A higher-scoring rival is `SEMANTIC_RECOMMENDATION_OUTSCORED`. |
| Team | One to eight people use biography cards. Name at most 60 characters, role at most 80, bio at most 220, initials label at most 8. Every card needs a role; a `photo` headshot (path/url string or `{path|url, alt}`) and an initials `photo_label` are alternatives. |
| Image case | Story body at most 300 characters, eyebrow 30, heading 80, at most five bullets of 140 each, at most three metrics with value 10 and label 40, caption 120. A picture without a stated case is not an image case. Over budget it degrades to two columns, keeping the picture. Without `image` / `photo` / `screenshot` a dashed "Image placeholder" box renders and validation warns `SEMANTIC_IMAGE_MISSING`; set `image_label` to mark a deliberate placeholder. |
| Framework | SWOT, Five Forces, and BMC need every canonical part; at most ten items per part and 200 characters per item. A missing part degrades the whole framework to grouped bullets. |
| 2×2 matrix | Exactly four headed quadrants and both axes. Quadrant header at most 80 characters, body 200; horizontal axis 16, vertical axis 60, axis ends 11. Missing parts or over-budget copy degrade to named bullets. |
| Timeline | Three to seven milestones, label at most 60, date 30, body 200. Use ranges for periods; use a roadmap for parallel workstreams. Outside the budget, preserve dates in bullets. |
| Hero statistic | One value at most 20 characters, label 80, unit 10, context 120, source 80. The `unit` renders at 40% of the value's size on the value's baseline ("$2.4B" large, "TAM" small). For several equal-weight figures use KPI snapshot. |
| Agenda | Two to ten numbered sections (title ≤100, subtitle ≤120); a subtitle renders as a smaller muted line under its title, and `current` bolds that section and dims the rest. A subtitle over 120 characters (3–6 sections) uses the `agenda-with-images` rows, which have no current-section highlight; outside the range it degrades to a numbered bullet list. |
| Quotation | One named speaker uses a pull quote (at most 500 characters); three to eight named speakers use a cluster (at most 240 each). Two quotes or missing names degrade to quote bullets. |
| Bridge | Three to ten waterfall columns; component label at most 40 characters, unit 8, caption 60. Total/delta values are numeric; an implicit subtotal uses the running total. A total off the running sum by >0.5% is `SEMANTIC_BRIDGE_TOTAL_MISMATCH`. |
| Pillars | Three to five pillars. A house needs both objective and foundation, title at most 60, up to five bullets of 120, and up to three roof badges of 24. Without full framing use panels (title 80, one to eight bullets of 200). |
| Organization | One root, up to seven nodes, three levels, and four direct reports per node; labels at most 40 characters. Larger trees degrade to attributed bullets rather than dropping people. |
| Architecture | Three to six tiers; tier label at most 60 characters, detail at most 120, and side rail at most 30. Out-of-budget content degrades intact to bullets. |
| Process | Three to six described steps use numbered rows (label at most 60, description at most 180). Bare labels or decision branches use `process-flow` when within its range; three to six labels averaging under 40 characters use the shallow `process-flow-compact` band under the title. A straight sequence is not a branching flowchart. |
| Next steps | The closer (`next_steps`, the `plan_deck` deckspec default): two to six `actions` {action ≤90, owner ≤30, date ≤20} and zero to three `decisions` ≤120. Outside the budget it degrades to bullets keeping owner and date. Keep `closing` for a Q&A page only: it renders on the template's own closing layout (not the cover), and a title over 40 characters wraps there (`SEMANTIC_DENSITY`) — keep the ask short and put owner/date in `subtitle`. |
| Roadmap | Three to six phases use a phase visual; otherwise choose a content layout that preserves every milestone. |
| Decision | Three to six options use numbered boxes (label at most 60, detail at most 180). Exactly two *detailed* options use paired cards (label at most 80, detail at most 300). Mark exactly one labeled option recommended and put the ask in `recommendation` — it is the slide's one conclusion band; a `takeaway` beside it goes to the speaker notes (`SEMANTIC_DUPLICATE_CALLOUT`). |

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
unreviewed response rather than refreshing the earlier one. A visual verdict
never clears deterministic blockers (`reviewed_deterministic_blockers`).
Each diagnostic has a `semantic_path` to edit and a
`raw_path` only as fallback. Keep the DeckSpec as the source of truth.
Grid text generation would shrink below its readable floor is an `error`
in `validate_deck_spec` too. If render still refuses (`success: false`), the
refusal is a diagnostic (`code`, `severity`, `semantic_path`,
`evidence.measured`/`allowed`) whose `next_tool_call` — also top level — is a
`validate_deck_spec` patch: rewrite the named field, or for a list, switch
the slide to its native layout (`/slides/N/layout: "content"`). Inside a
`raw_json2pptx` slide its `semantic_path` is the slide's `slides[N].slide`
payload. HTTP integrations get the same render through
`POST /api/v1/semantic/render` (documented in the repository's `docs/api/README.md`).
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
renderable content (except a deliberate blank slide), its `pattern` values
pass the pattern schema, and has the same
`meta.design_mode` constraints as a raw deck. A raw hex fill or absolute
font size in constrained mode is refused; use free mode only when low-level
control is intentional. For a visual beyond semantic reach, query the
chosen kind's `compositions`; an unsupported override reports
`SEMANTIC_PATTERN_NOT_AVAILABLE`. Use a minimal raw slide rather than
lowering the entire deck unnecessarily.

Runnable examples live in [examples/semantic](../../examples/semantic/).
The compiler architecture and schema are in
[docs/SEMANTIC_COMPILER.md](../../docs/SEMANTIC_COMPILER.md).
