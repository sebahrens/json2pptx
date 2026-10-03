# Semantic DeckSpec authoring

Read this when authoring or revising a DeckSpec. `list_slide_kinds` is the
live catalogue (one line and the required fields per kind); for a kind you
use, `kinds:["<kind>"]` returns its full summary, typical fields and example,
`fields:["brief"]` its field signatures and text budgets (`"item_schema"`:
canonical fields with descriptions, each naming its `aliases` once;
`"compositions"`: pattern / layout overrides; `"item_schema_full"`: the
closed schema with every alias, for validators).

## Plan the narrative

Write the storyline first ([QUALITY.md](QUALITY.md)). `plan_deck` with
`format:"deckspec"` drafts it from the brief: `deck_spec` holds the kinds
with `__FILL__` titles (and `meta.date`), `slots[]` each slot's `path`,
guidance and routed facts, `unplaced_facts` every clause no slot holds, and
`constraints[]` the instructions about the deck itself ("8 slides", "with an
agenda"). A brief that lists its slides gets one slide per item, in order;
`budget` / `budget_note` say how the slide budget was spent (no agenda or
dividers under 12 slides unless asked). Facts are never truncated.
Send `spec` as a JSON object or a YAML/JSON string, with `meta` and either flat `slides[]` or chapter-based
`structure: {cover, auto_agenda, sections:[{title, slides:[]}], closing}`.
The forms are mutually exclusive. Chapters add numbered section dividers;
`meta.chrome.section_crumb: true` labels their content slides in the footer;
`meta.chrome.tracker: true` sets the section name above each content title. The compiler
generates divider numbers: do not hand-author them. A section with
`appendix: true` (or a `section` slide with `appendix: true`) is back
matter: an unnumbered divider (later chapters keep their numbers), left out
of `auto_agenda`, tracker / crumb "Appendix: <title>". Dividers titled
Appendix, Backup, Annex, Q&A or Thank you are unnumbered without the flag.
Appendix slides skip the rhythm run checks; their page numbers read A1, A2, …
while the main deck stays contiguous and `{total}` counts only it. Keep a narrative,
not one slide per layout or interchangeable cards. `explain_deck_spec` previews the resolved story and visual rhythm
without rendering.

When the request explicitly asks to exercise native layouts, put canonical
layout IDs in `meta.required_layouts` (not a
`recommend_visual.template_support.required_layout` value, which can name a
capability). Inspect
`explain_deck_spec.layout_coverage.{requested,assigned,missing}` and add a
compatible *narrative* slide for each missing layout, never an empty or
unrelated one. Template resolution order is the tool's
`template` argument (that call only; `warnings[]` says it overrode the pin),
then `meta.template`, then `template_path`, then the archetype default; a
`deck_id` stays bound to its first template until a patch to
`/meta/template`. `meta.waivers: [{code, reason}]` waives
`NO_EXECUTIVE_SUMMARY`, `CLOSING_WITHOUT_NEXT_STEPS`, `TITLE_NOT_ACTION` or
`takeaway_missing` (a waived finding is an `info`). Discover archetypes with
`list_deck_archetypes`.

`meta.chrome` controls confidentiality, client/project labels, date, page
numbers and section crumbs. By default a deck gets page numbers (title and
closing skipped), `meta.date` in the footer, and the tracker with sections.
`meta.viewing_mode` and `meta.accent_strategy` choose reading scale and
accent rhythm (schema enums; others are `SEMANTIC_REQUIRED`). Every kind can
carry `notes` and `source` (rendered once in the 9pt source zone, even when
the pattern has its own attribution field; `meta.source` is the deck
default; missing → `DATA_WITHOUT_SOURCE` at `slides[N].source`).
`meta.type_scale`: `compact`, `comfortable` (default) or `presentation`; a
slide's `pattern` is a string, never an object.

## Validation and content budgets

The live kind schema is closed. Unknown fields are dropped by compilation
and surfaced as `SEMANTIC_UNKNOWN_FIELD`; treat even a warning as lost
content. A wrong JSON type is `SEMANTIC_FIELD_TYPE`, not a request to guess
the intended coercion. Numbers in text positions render as plain decimals;
quote literals such as `"1.10"`. `SEMANTIC_REFERENCE_UNRESOLVED` means a
`recommended` / `decisive_criterion` / agenda `current` matched nothing.
Placeholder copy the product wrote (`__FILL__`, a recipe's "Replace with …",
an `<instruction>` value) is a blocking `SEMANTIC_WEAK_CONTENT` in any text
field, `meta` included. Run `validate_deck_spec` before rendering and correct
these at their `path`.

Visuals have content limits. The compiler degrades an out-of-range visual to
readable bullets or another content layout and reports
`SEMANTIC_PATTERN_DEGRADED` at each over-budget item, with the budget in
`remediation.primary.params` (`max_chars`, or `min_items` / `max_items` for a
count). This is not the same as `SEMANTIC_DENSITY`, which advises about
density without losing the visual. Shorten, split or change the count; do not
silently truncate facts. Check `explanation_summary.pattern` after render to confirm
the visual you expected actually landed (a degraded slide reports its compiled
`layout` / `visual_family`).

Text budgets are live data: `list_slide_kinds` `fields:["budgets"]` (or
`["brief"]`) returns `budgets[] {field, max_chars, max_chars_per_line,
max_lines, min_items, max_items, basis, note}`. `basis: "measured"` budgets
(`title`, `subtitle`, `takeaway`) are measured on `template`, else the
tightest across the shipped templates (`budget_basis.templates`), so copy
inside them fits every one; `basis: "fixed"` budgets hold everywhere; `note`
states the tighter lengths at higher item counts, and a finding's `max_chars`
is the length for the slide's own count. Call it with the template you will
render on before writing titles and takeaways. What the numbers do not say:

| Kind | Authoring consequence |
|---|---|
| `executive_summary` | 3–5 points `{lead, support}`; a plain string spans the width. One conclusion band: `bottom_line`, else `takeaway` — both → the `takeaway` goes to the notes (`SEMANTIC_DUPLICATE_CALLOUT`). Past the budgets it degrades to bullets. |
| `kpi_snapshot` | 2–6 KPIs; give each its reference in `comparator` (alias `vs`, "vs plan +4 pts"). A value past the budget degrades the slide; one that fits the budget but not the card is `BODY_TOO_LONG`. |
| `chart_insight` | 1–6 insights sit beside the chart; more use a native chart with the full list. Every series needs one unquoted number per category (`CHART_SERIES_LENGTH_MISMATCH`, `CHART_VALUE_NOT_NUMERIC`). Chart types are short names (`bar`, `line`; `bar_chart` is accepted). |
| `comparison` | 2 balanced columns of ≤10 rows compare; 3–5 columns are panels; 6–12 columns are cards; beyond that, bullets. |
| `table` | ≤6 headers × 6 body rows. Options scored against criteria belong in `option_matrix`, not a generic table. |
| `option_matrix` | 2–6 criteria × 2–6 options on one Harvey, RAG or text scale. `recommended` takes one option (name or 0-based index) or a list; a higher-scoring rival is `SEMANTIC_RECOMMENDATION_OUTSCORED`. |
| `decision` | 3–6 options are numbered boxes; exactly 2, or 7–12, each with a detail, are cards. Mark the recommended option `recommended: true` (or name it in slide-level `recommended`); two or more are a combined recommendation ("Recommended: A and B" when no `recommendation` is written). The ask goes in `recommendation`, the slide's one conclusion band. |
| `team` | 1–8 people, each with a role; a `photo` (path / url, or `{path|url, alt}`) or an initials `photo_label`. |
| `image_case` | A picture with a stated case (body, ≤5 bullets, ≤3 metrics). `callouts: [{label, x, y, units?}]` (≤6) point into the picture — fractions 0–1 from its top-left, or `units: "px"`; a target the crop hides is `OVERLAY_TARGET_CROPPED`, so set `image.fit: "contain"` for screenshots. Without an image a dashed placeholder renders and validation warns `SEMANTIC_IMAGE_MISSING` (`image_label` marks a deliberate one). A `regions` image region draws no callouts. |
| `framework`, `matrix_2x2` | Every canonical part (SWOT, Five Forces, BMC; four headed quadrants and both axes) or the slide degrades to grouped bullets. |
| `timeline`, `roadmap` | 3–7 milestones; one with an `end_date` turns the line into bars drawn to scale. 3–6 phases for a roadmap; parallel workstreams belong in `roadmap`. |
| `stat` | One value; `unit` renders at 40% of its size on the baseline. Several equal-weight figures → `kpi_snapshot`. |
| `agenda` | 2–10 sections; `title` defaults to "Agenda"; `current` bolds one section and dims the rest. |
| `quote` | One named speaker is a pull quote, 3–8 a cluster; two quotes, or missing names, degrade to bullets. |
| `bridge` | 3–10 columns; a total off the running sum by >0.5% is `SEMANTIC_BRIDGE_TOTAL_MISMATCH`. |
| `pillars` | 3–5 pillars. A house needs `objective` and `foundation` (a string, or 1–3 levels, each a band or a row of 2–5 cells) and takes a `beam`; without them it is panels. |
| `org` | One root, ≤7 nodes, 3 levels, ≤4 direct reports per node; larger trees degrade to attributed bullets. |
| `architecture` | 3–6 tiers; a tier's `items` (1–12, ≤40 characters each) are drawn one block each, a `description` as one line. |
| `process` | 3–6 steps with descriptions are numbered rows (label ≤60, description ≤180); 7–8 steps, or bare labels, are flow boxes (label and description together ≤80). A straight sequence is not a branching flowchart. |
| `next_steps` | The closer: 2–6 `actions` `{action, owner, date}` and 0–3 `decisions`. Keep `closing` for a Q&A page; its title budget depends on the template. |

Counts that do not fit yet: `next_steps` with 6 actions and a decisions band,
and `option_matrix` with 6 options, on every shipped template;
`executive_summary` with 5 points and some `pillars` houses on `modern` /
`modern-template`. The finding's verified patch says what fits.

These are authoring budgets, not an alternate schema. Prefer
`list_slide_kinds` canonical field names over the `aliases` it lists.

## Several visuals on one slide

For visuals read together, use `kind: regions`, not a raw compose: 2–3
typed chart / stat / kpis / table / timeline / image / text regions in a
bounded arrangement with `size_pct` shares (budgets: `list_slide_kinds`;
`examples/semantic/regions.yaml`), each rule an error at
`slides[N].regions[k]`. A stacked region holds one visual's worth; leave its
share unset (raised to what its kind reads in). On a readability refusal
raise `size_pct` or cut; on `chart.plot_area_collapsed` put the chart
beside, not above.

## Render and revise

Use `validate_deck_spec` → `render_deck_spec`. `success: true` means a file
was written; `deterministic_ready` that diagnostics, output validation and
the quality gate cleared; `publishable` stays false until a current
all-slide visual verdict is approved. The CLI exits 0 for a clean,
unreviewed render, not for a strict-mode blocker. Resolve `diagnostics[]`,
`deterministic_blocking_reasons[]` and the gate before review, then take the
final verdict from `submit_visual_review`'s current-revision status; a
re-render is a new unreviewed response, and a verdict never clears blockers
(`reviewed_deterministic_blockers`). Edit the spec at a diagnostic's `path`
(a JSON Pointer; the string a patch takes) and keep the DeckSpec as the
source of truth. Validate and render report the same findings for one spec
and template ([FINDINGS.md](FINDINGS.md)): text generation would shrink below
its floor is an `error` at validate too. A refused render (`success: false`)
carries the first blocking finding's patch as its top-level `next_tool_call`:
a verified removal, a rewrite of the named field within its budget, or the
slide's text layout (`/slides/N/layout: "content"`). A finding inside a
`raw_json2pptx` slide points into it (`/slides/N/slide/…`). HTTP:
`POST /api/v1/semantic/render` (`docs/api/README.md`).
After each revision, render and inspect the affected slides, then inspect all
slides of the final revision as required by [SKILL.md](SKILL.md).

Validation, explain, and render return a `deck_id` handle. Send `deck_id`
instead of `spec` (never both), with an optional ordered
`patch:[{op, path, value | from}]` (`replace`, `add`, `remove`, `move`,
`copy`; all apply or none). A slide is addressed by index or by its `id`
(`/slides/s4/title`; a stored deck assigns `s1`, `s2`, … to slides without
one). `changed_slides` lists the 0-based slides that LOOK different — on
render, since the last render — so re-render those; `slide_changes[]`
classifies every affected slide (`edited | inserted | restyled | moved |
renumbered | notes_only | removed`); `stored: false` means the patch was not
kept (a refused render, or `dry_run: true`). `validate_deck_spec` also reads
the store (`read: "spec" | "history" | "diff:A..B" | <slide id>`; `diff:A`
compares A with the current revision), finds or replaces text everywhere
(`find`, `replace`), restores a revision (`restore: N`) and forks
(`fork: true`). Handles are process-local and expire after one hour; retain
the source spec yourself.
Without an explicit `output_filename`, the rendered filename includes a
digest so two distinct specs do not collide. Reusing an explicit name may
overwrite the earlier artifact; check `overwrote`.

`raw_json2pptx` is a deliberate escape hatch inside a DeckSpec. Its
`slide` must be a structurally valid raw slide with renderable content, its
`pattern` values pass the pattern schema, and `meta.design_mode` constrains
it like a raw deck (a raw hex fill or absolute font size is refused in
constrained mode). For a visual beyond semantic reach, query the
chosen kind's `compositions`; an override the kind or payload cannot
take reports `SEMANTIC_PATTERN_NOT_AVAILABLE`. Use a minimal raw slide
(also for a multi-region slide: WORKFLOW.md → spatial planning), not a
lowered deck.

Runnable examples: [examples/semantic](../../examples/semantic/).
Compiler architecture and schema:
[docs/SEMANTIC_COMPILER.md](../../docs/SEMANTIC_COMPILER.md).
