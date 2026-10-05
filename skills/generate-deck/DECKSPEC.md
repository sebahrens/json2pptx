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
`constraints[]` the deck instructions ("8 slides", "with an agenda"). A
sentence naming a kind (bridge, heat map, team, options, roadmap, a series)
becomes that kind, in brief order, facts in its fields: edit, do not
retype. "Ask:" is the one closer; the cover title is the brief's deck name.
A brief listing its slides gets one slide per item, in order; `budget_note`
says how the budget was spent (no agenda or dividers under 12 slides unless
asked). Facts are never truncated.
Send `spec` (a JSON object or a YAML/JSON string) with `meta` and either flat `slides[]` or chapter-based
`structure: {cover, auto_agenda, sections:[{title, slides:[]}], closing}`,
never both. Chapters add numbered section dividers;
`meta.chrome.section_crumb: true` puts the section in the footer,
`meta.chrome.tracker: true` above each content title. Divider numbers are
generated: do not hand-author them. A section (or `section` slide) with
`appendix: true` is back matter: an unnumbered divider (later chapters keep
their numbers), outside `auto_agenda`, crumb "Appendix: <title>". Dividers
titled Appendix, Backup, Annex, Q&A or Thank you need no flag. Appendix
slides skip the rhythm run checks and number A1, A2, …; `{total}` counts the
main deck only. Keep a narrative, not one slide per layout or
interchangeable cards. `explain_deck_spec` previews story and rhythm
without rendering.

When the request explicitly asks to exercise native layouts, put canonical
layout IDs in `meta.required_layouts` (not a
`recommend_visual.template_support.required_layout` value, which can name a
capability). Inspect
`explain_deck_spec.layout_coverage.{requested,assigned,missing}` and add a
compatible *narrative* slide for each missing layout, never an empty or
unrelated one. Template precedence is the same on every DeckSpec tool
(validate, render, compile, explain; CLI `semantic … --template`): the
call's `template` > `meta.template` > `template_path` > the `deck_id`'s
bound template > the archetype default. A `template` that differs from
`meta.template` replaces it for that call only (`warnings[]` says so);
patch `/meta/template` to keep it. `explain_deck_spec` is read-only: with
`template` on an unbound `deck_id` it plans on that template without
binding the deck (`warnings[]` says so); validate / render bind it. With
no template and no `meta.archetype` it omits `template` and warns; a render
then fails `TEMPLATE_NOT_FOUND`.
`meta.waivers: [{code, reason}]` waives
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

The kind schema is closed: unknown fields are dropped and surfaced as
`SEMANTIC_UNKNOWN_FIELD` (even a warning is lost content); a wrong JSON type
is `SEMANTIC_FIELD_TYPE`, not a request to guess a coercion. Numbers in text
positions render as plain decimals; quote literals such as `"1.10"`.
`SEMANTIC_REFERENCE_UNRESOLVED`: a `recommended` / `decisive_criterion` /
`highlight_column` / agenda `current` matched nothing. Placeholder copy the
product wrote (`__FILL__`, "Replace with …", an `<instruction>` value) is a
blocking `SEMANTIC_WEAK_CONTENT` anywhere, `meta` included. Run
`validate_deck_spec` before rendering and fix findings at their `path`.

Visuals have content limits: an out-of-range visual degrades to readable
bullets or another content layout, reported as `SEMANTIC_PATTERN_DEGRADED` at
each over-budget item with the budget in `remediation.primary.params`
(`max_chars`, or `min_items` / `max_items` for a count). `SEMANTIC_DENSITY`
only advises; the visual stays. Shorten, split or change the count; never
truncate facts. After render, `explanation_summary.pattern` confirms the
visual landed (a degraded slide reports its compiled `layout` /
`visual_family`).

Text budgets are live data: `list_slide_kinds` `fields:["budgets"]` (or
`["brief"]`) returns `budgets[] {field, max_chars, max_chars_per_line,
max_lines, min_items, max_items, basis, note}`. `basis: "measured"` budgets
(`title`, `subtitle`, `takeaway`) are measured on `template`, else the
tightest shipped template (`budget_basis.templates`); `basis: "fixed"`
budgets hold everywhere; `note` states the tighter lengths at higher item
counts, and a finding's `max_chars` is the length for the slide's own count.
Call it with your template before writing titles and takeaways. What the
numbers do not say:

| Kind | Authoring consequence |
|---|---|
| `executive_summary` | 3–5 points `{lead, support}`; a plain string spans the width. Conclusion band: `bottom_line` (SKILL.md). Past the budgets it degrades to bullets. |
| `kpi_snapshot` | 2–6 KPIs; give each its reference in `comparator` (alias `vs`, "vs plan +4 pts"). A value's 12 characters are the hard maximum, not the fit: five KPIs hold about 11 digits and six about 9 on the narrowest templates (the `kpis[].value` budget `note`). Past 12 the slide degrades; a value that will not fit one line is `BODY_TOO_LONG` at `/slides/N/kpis/i/value` with `max_chars`: shorten it or show fewer KPIs. |
| `chart_insight` | Needs one stated implication: `insights[]` (1–6 beside the chart; more use a native chart), a scalar `insight`, or just `takeaway`, which is then the 18pt so-what callout beside a 75%-wide chart (not repeated in the band; not `takeaway_missing`). Every series needs one unquoted number per category (`CHART_SERIES_LENGTH_MISMATCH`, `CHART_VALUE_NOT_NUMERIC`). Chart types are short names (`bar`, `line`; `bar_chart` is accepted). |
| `comparison` | 2 balanced columns of ≤10 rows compare; 3–5 columns are panels; 6–12 cards; beyond that, bullets. Two columns take `connectors: true` (a per-row today → target badge), `highlight_column` (`left` / `right` / a header) or `highlight_row` (0-based index or cell text) — one highlight, not both; 3+ columns ignore them. |
| `table` | ≤6 headers × 9 body rows (10 logical rows with the header); one-line rows past 7 render at a compact pitch, nothing hidden. A row that cannot fit is `table_rows_truncated` (error): split at its `split_at_row`, titles `… (1/2)` / `… (2/2)`. Options scored against criteria belong in `option_matrix`, not a generic table. |
| `option_matrix` | 2–6 criteria × 2–6 options; `scale` per criterion (`{label, scale}`: `harvey` 0–4, `rag`, `text`) — a status board is `rag` + `text` columns. `recommended` takes one option (name or 0-based index) or a list; a higher-scoring rival is `SEMANTIC_RECOMMENDATION_OUTSCORED`. |
| `decision` | 3–6 options are numbered boxes; exactly 2, or 7–12, each with a detail, are cards. Mark the recommended option `recommended: true` (or name it in slide-level `recommended`); two or more read "Recommended: A and B" when no `recommendation` is written. The ask goes in `recommendation`. |
| `team` | 1–8 people, each with a role; a `photo` (path / url, or `{path|url, alt}`) or an initials `photo_label`. |
| `image_case` | Picture + body, ≤5 bullets, ≤3 metrics. ≤6 `callouts: [{label, x, y, units?}]`: fractions 0–1 or `units: "px"`; `OVERLAY_TARGET_CROPPED` → `image.fit: "contain"`. `image_width_pct` 30–60 (default 45): a wide `contain` screenshot wants 55–60, else it is letterboxed. No image: a draft renders, but `SEMANTIC_IMAGE_MISSING` blocks readiness (`image_label` only labels the frame). `regions` images have no callouts. |
| `framework`, `matrix_2x2` | Every canonical part (SWOT, Five Forces, BMC; four headed quadrants and both axes) or the slide degrades to grouped bullets. |
| `timeline`, `roadmap` | 3–7 milestones; one with an `end_date` turns the line into bars drawn to scale. 3–6 phases for a roadmap (`milestone` ≤60 marks a phase); parallel workstreams go in `parallel_tracks` (0–4, ≤90 each; `parallel_label` ≤24): bars under the phases, not a raw slide. |
| `stat` | One value; `unit` renders at 40% of its size on the baseline. Several equal-weight figures → `kpi_snapshot`. |
| `agenda` | 2–10 sections; `title` defaults to "Agenda"; `current` bolds one section and dims the rest. |
| `quote` | One named speaker is a pull quote, 3–8 a cluster; two quotes, or missing names, degrade to bullets. |
| `bridge` | 3–10 columns; a total off the running sum by >0.5% is `SEMANTIC_BRIDGE_TOTAL_MISMATCH`. |
| `pillars` | 3–5 pillars. A house needs `objective` and `foundation` (a string, or 1–3 levels, each a band or a row of 2–5 cells) and takes a `beam`; without them it is panels. |
| `org` | One root, ≤7 nodes, 3 levels, ≤4 direct reports per node; larger trees degrade to attributed bullets. |
| `architecture` | 3–6 tiers; a tier's `items` (1–12, ≤40 characters each) are drawn one block each, a `description` as one line. |
| `process` | 3–6 steps with descriptions are numbered rows (label ≤60, description ≤180); 7–8 steps (on two rows), or bare labels, are flow boxes (label and description together ≤80). A straight sequence is not a branching flowchart. |
| `next_steps` | The closer: 2–6 `actions` `{action, owner, date}` and 0–3 `decisions`. Keep `closing` for a Q&A page; its title budget depends on the template. |

Prefer `list_slide_kinds` canonical field names over the `aliases` it lists.

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
`deterministic_blocking_reasons[]` and the gate before review (a verdict
never clears blockers). Edit the spec at a diagnostic's `path`
(a JSON Pointer; the string a patch takes) and keep the DeckSpec as the
source of truth. Validate and render report the same findings for one spec
and template ([FINDINGS.md](FINDINGS.md)): text generation would shrink below
its floor is an `error` at validate too. A refused render (`success: false`)
carries the first blocking finding's patch as its top-level `next_tool_call`:
a verified removal, a rewrite of the named field within its budget, or the
slide's text layout (`/slides/N/layout: "content"`). A finding inside a
`raw_json2pptx` slide carries the raw code at the field inside it
(`INVALID_SLIDE` at `/slides/N/slide/overlays/0`).

Validation, explain, and render return a `deck_id` handle. Send `deck_id`
instead of `spec` (never both), with an optional ordered
`patch:[{op, path, value | from}]` (`replace`, `add`, `remove`, `move`,
`copy`; all apply or none). A slide is addressed by index or by its `id`
(`/slides/s4/title`; a stored deck assigns `s1`, `s2`, … to slides without
one; CLI `semantic render` assigns the same ids, lists them in `slides[]`
and records them in `<deck>.pptx.authoring.json` (`slide_ids`), so
`render-slide --slide-id s3` and `render-thumbnails --slides s4,0` work
unless `--no-manifest` was set). `changed_slides` lists the 0-based slides that LOOK different — on
render, since the last render — so re-render those; `slide_changes[]`
classifies every affected slide (`edited | inserted | restyled | moved |
renumbered | notes_only | removed`); `stored: false` means the patch was not
kept (a refused render, or `dry_run: true`). `validate_deck_spec` also reads
the store (`read: "spec" | "history" | "diff:A..B" | <slide id>`; `diff:A`
compares A with the current revision; each side is read on the template
it was validated or rendered on, so a call's `template` change shows as
`restyled` slides and `summary` ends `(template A → B)`), finds or replaces text everywhere
(`find`, `replace`), restores a revision (`restore: N`) and forks
(`fork: true`). Handles are process-local and expire after one hour; retain
the source spec yourself.
The default filename carries a digest of the spec; an explicit
`output_filename` may overwrite an earlier artifact (check `overwrote`).

`raw_json2pptx` is a deliberate escape hatch inside a DeckSpec. Its
`slide` must be a structurally valid raw slide with renderable content, its
`pattern` values pass the pattern schema, and `meta.design_mode` constrains
it like a raw deck (a raw hex fill or absolute font size is refused in
constrained mode). For a visual beyond semantic reach, query the
chosen kind's `compositions`; an override the kind or payload cannot
take reports `SEMANTIC_PATTERN_NOT_AVAILABLE`. Use a minimal raw slide
(also for a multi-region slide: WORKFLOW.md → spatial planning), not a
lowered deck.

Compiler architecture and schema:
[docs/SEMANTIC_COMPILER.md](../../docs/SEMANTIC_COMPILER.md).

`table.totals_row` is boolean; agenda `current` is a 1-based integer or a section title.
