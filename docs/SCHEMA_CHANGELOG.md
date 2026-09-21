# Schema Changelog

Tracks backward-incompatible and notable additions to the JSON input schema,
MCP tool surface, and Fix.Kind vocabulary. Agents compare `schema_version`
(from `get_capabilities`) across sessions to detect contract drift.

## Unreleased

### Added

- **Agenda-with-images paired copy targets (go-slide-creator-tp23k.12).**
  `agenda-with-images` now warns when a subtitle exceeds the readable room
  left by its title on a five- or six-row agenda with image labels. Titles up
  to about 65 characters allow about 150 subtitle characters; longer titles
  allow about 75 at five rows and no subtitle at six. Three or four rows, and
  agendas without image labels, retain the 160-character subtitle maximum.
  The title and subtitle descriptions include the paired limits. Measurements
  used all four bundled templates. Schema version advances to 4.88.0.

- **Horizontal-bar callout copy targets (go-slide-creator-tp23k.11).**
  `horizontal-bar-with-callouts` now emits `BODY_TOO_LONG` with the bar index
  and a measured readable target when a callout exceeds about 181 characters
  at six bars or 121 at seven or eight. Three to five bars retain the
  200-character schema maximum; labels and units remain readable at their
  existing maxima. The callout field describes these row-count budgets.
  Measurements used all four bundled templates. Schema version advances to
  4.87.0; the sparse input maximum remains 200.

- **KPI delta validation parity (go-slide-creator-rbkl9).** `kpi-inline`
  and `kpi-2up` through `kpi-6up` now enforce the existing 12-character
  `sub` schema maximum. The `delta`, `trend` and `change` input aliases
  normalize to `sub` and receive the same `max_length` diagnostic with a
  `values[i].sub` path. Schema version advances to 4.86.0; the schema
  fingerprint is unchanged.

- **Inline KPI combined-copy budgets (go-slide-creator-tp23k.10).**
  `kpi-inline` now emits `BODY_TOO_LONG` for dense icon cells when the
  caption exceeds the readable target implied by KPI count, number length
  and delta length. Five or six cells without icons, and up to four cells
  with icons, retain the 40-character caption limit. At five cells, an icon
  with an eight-character number holds about 16 caption characters without
  a delta and none with one; at six cells the limit can fall to 11 or zero.
  Schema guidance keeps the 40-character sparse maximum. Measurements used
  all four templates. Schema version advances to 4.85.0; the input
  fingerprint is unchanged.

- **Six-step chevron copy target (go-slide-creator-tp23k.9).** A
  `numbered-step-strip` with six chevrons now emits `BODY_TOO_LONG` naming
  any label above the measured 47-character readable target. Three to five
  chevrons and all `stacked-box` / `toc` labels remain readable at the
  60-character schema maximum; optional detail bodies stay readable at 180.
  The existing `TEXT_EXCEEDS_SHAPE` finding for labels that certainly break
  mid-word remains. Measurements used all four templates. Schema version
  advances to 4.84.0; the input fingerprint is unchanged.

- **Swimlane step budgets (go-slide-creator-tp23k.8).** `swimlane` now emits
  `BODY_TOO_LONG` with the lane, step and measured text target for each
  supported step-column/lane-count shape. The schema carries the 7×5 table
  while retaining the 80-character limit for sparse grids. Actor labels
  remain readable at the 40-character schema maximum. Measurements used all
  four templates. Schema version advances to 4.83.0; the input fingerprint
  is unchanged.

- **Team-bio card-count budget (go-slide-creator-tp23k.7).**
  `team-bios` replaces its fixed 24-word bio warning with measured
  member-count guidance: about 220 bio characters with 1–4 members and 141
  with 5–8. The second card row reduces text height; headshots do not change
  the text zone. The schema retains 220 as the sparse-card maximum and fit
  reports name the member and target. Measurements used all four templates.
  Schema version advances to 4.82.0; the input fingerprint is unchanged.

- **Phased-roadmap activity budgets (go-slide-creator-tp23k.6).**
  `roadmap-phased` now emits `BODY_TOO_LONG` with the workstream, phase and
  measured activity-pill target for each supported phase/workstream count.
  The schema carries the 7×5 table while retaining the 80-character limit
  for sparse grids. Phase labels and workstream names remain readable at
  their existing schema maxima in the four bundled templates. Schema version
  advances to 4.81.0; the input fingerprint is unchanged.

- **Comparison row budgets (go-slide-creator-tp23k.5).** `comparison-2col`
  now emits `BODY_TOO_LONG` with the row, side and measured copy target.
  The budget counts body rows plus one when headers are present: up to four
  effective rows hold about 200 characters per cell, five hold 196, six or
  seven hold 131, and eight or more hold 66. The schema description carries
  the table; the 200-character cap remains for sparse comparisons. Headers
  retain their 60-character limit. Measurements used all four templates.
  Schema version advances to 4.80.0; the input fingerprint is unchanged.

- **Driver-tree copy budgets (go-slide-creator-tp23k.4).** `driver-tree`
  now warns with measured leaf, branch-label and annotation targets based on
  total leaf rows, annotation-column presence and per-branch row span. Trees
  with 15–16 leaf rows warn to aggregate or split: even short copy is below
  the readable floor at default sizes. The schema describes the budgets while
  preserving the existing maximum lengths for sparse trees. Measurements used
  all four bundled templates. Schema version advances to 4.79.0; the input
  fingerprint is unchanged.

- **BMC cell copy budgets (go-slide-creator-tp23k.3).** `bmc-canvas`
  describes approximate per-bullet readable limits for each of its four cell
  geometries and emits `BODY_TOO_LONG` with the cell, bullet, and target when
  copy exceeds that limit. The narrow middle and bottom cells advise at most
  seven bullets. The schema keeps the 200-character bullet maximum for cells
  with enough space. Budgets were measured across all four bundled templates.
  Schema version advances to 4.78.0; the input fingerprint is unchanged.

- **Pull-quote readability role (go-slide-creator-tp23k.2).** The shape-grid
  readability collector now recognizes the quote row of a `pull-quote` pattern
  or its source-stamped expanded grid. That row is assessed as prose at the
  12pt presentation floor, rather than as a numeric KPI at 18pt solely because
  its authored size is at least 24pt. The schema-maximum 500-character quote
  with headshot is readable at the measured 15.1pt; a quote below 12pt still
  gets `TEXT_BELOW_READABLE_MIN`, and other display values keep the 18pt floor.
  Schema version advances to 4.77.0; the fingerprint is unchanged.

- **MCP enum completions (go-slide-creator-zhaw.3).** `initialize` advertises
  completions. `completion/complete` now filters current template, pattern,
  slide kind, archetype, chart type, fix kind and fit finding code vocabularies
  by prefix. Standard MCP completion references are prompts or resources, so
  the existing `json2pptx://templates`, `json2pptx://patterns`,
  `json2pptx://schema/deckspec` and `json2pptx://skill` resources carry the
  tool-argument vocabularies; `deck-from-brief.template` completes through
  its prompt reference. Unknown reference/argument pairs return an empty list.
  Schema version advances to 4.76.0; the raw input and tool-name fingerprint
  is unchanged.

- **MCP logging (go-slide-creator-zhaw.2).** `initialize` advertises logging,
  and `logging/setLevel` lets each client opt into contextual render start,
  finish and warning messages through `notifications/message`. INFO/WARN render
  events still reach stderr. The SDK filters each notification at that
  session's chosen level; its default error level suppresses these events
  until opt-in. Schema version advances to 4.75.0; the raw input and tool-name
  fingerprint is unchanged.

- **MCP workflow prompts (go-slide-creator-zhaw.1).** `prompts/list` now
  advertises `deck-from-brief` and `revise-deck`; `prompts/get` returns a
  ready-to-use user message with the task-specific `get_started` fast path and
  the shared final-revision visual review rule. The first prompt accepts a
  required brief plus optional template and slide budget. The second accepts
  a revision goal plus exactly one of a PPTX path or deck JSON. Schema
  version advances to 4.74.0; the raw input and tool-name fingerprint is
  unchanged.

- **External and slide-jump hyperlinks (go-slide-creator-repp.1,
  go-slide-creator-repp.2).** Text and bullets accept `link: {url}`;
  slide source attribution accepts `source_link: {url}`. A `shape_grid`
  shape or overlay badge accepts `link: {slide: N}` to navigate to a
  numbered slide in the final deck. External URLs use hyperlink relationships
  with `TargetMode=External`; slide jumps use slide relationships. Invalid URLs,
  ambiguous targets and out-of-range slide numbers are rejected. Schema
  version advances to 4.73.0.

- **`export_deck` PDF and notes handouts (go-slide-creator-c7gm.1,
  go-slide-creator-c7gm.2).** Given an existing PPTX, `format: "pdf"` retains
  LibreOffice's PDF conversion without rasterizing it; `format: "notes"`
  writes a Markdown handout with one titled section and speaker notes per
  slide, without LibreOffice. The response includes an absolute `output_path`
  and file size. Content-hashed filenames keep deck revisions separate.
  `json2pptx export` exposes the same formats on the CLI. Schema version
  advances to 4.72.0 and the MCP tool-name fingerprint changes.

- **Cancellable image renders and thumbnail progress (go-slide-creator-o7ii.1,
  go-slide-creator-o7ii.2).** The render image
  handlers pass the MCP call context through LibreOffice, ImageMagick, cache
  lookup, and thumbnail assembly. A queued call can leave the LibreOffice
  serialization slot when cancelled; an interrupted call returns `CANCELLED`
  and no image payload. `render_deck_thumbnails` emits
  `notifications/progress` only when `_meta.progressToken` is supplied: an
  initial conversion/loading update followed by one update per returned slide,
  with `total` equal to the number selected for delivery. Existing Go render entry points retain background
  context wrappers; new `*Context` variants accept cancellation. Schema version
  advances to 4.71.0; the raw input fingerprint is unchanged.

- **Recommendation routing for pricing and Sankey requests (go-slide-creator-0hb2j).**
  Pricing plan/tier intents favor `card-grid`; a bare `tier` no longer implies
  architecture. `recommend_pattern` and `recommend_visual` now return
  `candidates: []` with `unsupported_visual: "sankey"` when asked for a Sankey
  diagram, including in explicit shortlist mode, because no Sankey renderer is
  registered. Schema version advances to 4.70.0; the raw input fingerprint is
  unchanged.

- **DeckSpec kind `org` (go-slide-creator-zgvs).** A flat `nodes[]` reporting
  tree compiles to the `org_chart` diagram for up to seven nodes, three levels,
  and four direct reports per node. The compiler rejects duplicate IDs,
  missing parents and cycles at semantic paths; larger trees degrade to a
  complete attributed list with `SEMANTIC_PATTERN_DEGRADED` so svggen cannot
  silently prune named people. The node-label overprint fix is already present.
  Schema version advances to 4.69.0; the raw input fingerprint is unchanged.

- **DeckSpec kind `pillars` (go-slide-creator-r1k1).** Three to five named
  pillars compile to `strategy-house` when `objective` and `foundation` are
  supplied together, or `stylish-panels` without house framing. Pattern limits
  and malformed item fields are checked at semantic paths; a fallback preserves
  all text and reports `SEMANTIC_PATTERN_DEGRADED`. The closed schema and
  `list_slide_kinds` include a copy-ready example. Schema version is 4.68.0;
  the raw PresentationInput fingerprint is unchanged.

- **DeckSpec kind `bridge` (go-slide-creator-exo2).** Ordered additive
  `columns: [{label, type, value?}, …]` compile to `waterfall-bridge` for 3–10
  columns. `type` is `total`, `delta`, or `subtotal`; totals and deltas require
  numeric values, and a subtotal may use the running total (an explicit value
  must agree with it). Optional `unit`
  and `caption` pass through. Outside the visual's count or text budgets, the
  compiler preserves each component as a bullet and emits
  `SEMANTIC_PATTERN_DEGRADED`; malformed columns receive a field-path error.
  The closed schema and `list_slide_kinds` expose a copy-ready example. Schema
  version advances to 4.67.0; the raw PresentationInput fingerprint is unchanged.

- **Cell `max_height` for `shape_grid` (go-slide-creator-1j7jo).** A cell can
  set a maximum rendered height in points; the renderer centers shorter cells
  in their row. Horizontal compose retains each segment's row cap on its own
  cells, so a compact KPI row no longer stretches beside a full-height hero.

- **DeckSpec kind `quote` (go-slide-creator-ze1p).** A single attributed quote
  compiles to `pull-quote`; 3–8 attributed quotes compile to `quote-cluster`.
  Use `{kind:"quote", quote:"…", attribution:"…", role:"…"}` for one voice or
  `{kind:"quote", quotes:[{text:"…", name:"…"}, …]}` for a cluster. Two quotes,
  missing attributions and over-budget copy use a content slide with quote
  bullets and a `SEMANTIC_PATTERN_DEGRADED` finding. The closed per-kind schema,
  `list_slide_kinds` example and pattern reachability table include the kind.

- **`CHART_SERIES_LENGTH_MISMATCH` and `CHART_VALUE_NOT_NUMERIC`
  (go-slide-creator-pcrp).** `{categories: [Q1,Q2,Q3,Q4], series: [{name: Rev,
  values: [10]}]}` validated clean and rendered one bar over four ticks;
  `values: ["1", "two"]` rendered an empty plot with a format-error label. Both
  came back `ok: true` with a quality score of 100, so the deterministic gate an
  agent is told to run before rendering could not see the two most common chart
  mistakes.
  - `validate_deck_spec` / `semantic validate` now emit both as **errors**, at
    `slides[i].chart.data.series[j].values` and `…values[k]` respectively.
  - svggen refuses the same two shapes in its per-chart `Validate`, so
    `validate_input`, `validate --fit-report` and `generate_presentation` report
    them as `refuse` fit findings and validate agrees with generate.
  - Covers the categorical charts (bar, line, area, stacked bar/area, grouped
    bar, radar). `scatter_chart` / `bubble_chart` are exempt — they carry point
    objects in the same `values` field — and a time series is exempt from the
    length check only, not the numeric one.

- **Pattern `PostExpandWarnings` reach every surface (go-slide-creator-wn4v).**
  `BODY_TOO_LONG`, `CHART_PLACEHOLDER_EMPTY` and the other warnings a pattern
  emits about the content it was just given were read only by
  `preview_presentation_plan`. A `chart-insights-split` slide with no chart
  rendered 75% empty while `validate_input`, `generate_presentation` and
  `score_deck` all reported no issues, and `docs/PATTERNS.md` claimed otherwise.
  - They are now collected into the shared fit-finding set, so they appear in
    `validate_input`, `validate --fit-report`,
    `generate_presentation(fit_report=true)`, `generate --json-output-report`,
    `score_deck`, `render_deck_spec` and `preview_presentation_plan` alike, as
    review-severity findings scoped to `/slides/N/pattern`.
  - `expand_pattern`, `expand_patterns` and the `patterns expand` CLI gain a
    `warnings[]` array carrying the raw `"<CODE>: message"` lines — the tool for
    inspecting what a pattern does with your values now reports the pattern's
    own objection to them.
  - The collector re-expands the pattern even when the slide already carries a
    `shape_grid`, so the generate path (where the grid *is* the expansion) is
    covered rather than skipped.

- **`PATTERN_CONTENT_MISMATCH` (go-slide-creator-h339i).** Every check asked
  whether content *fits*; none asked whether it *belongs*. The calibration
  corpus's `B07_wrong_pattern` draws three KPIs as a timeline and a four-month
  plan as a 2×2 — every slide structurally sound, every detector silent, and a
  human grade of 30/100.
  - New `review`-severity finding with a `swap_pattern` fix (`params: {from,
    to}`), emitted from the authored `slides[].pattern.values` at preflight.
  - Three rules, each requiring **every** item to match: a timeline whose stops
    are all measurements against non-period labels; a 2×2 whose four quadrant
    headers are all points in time; a flow or pyramid whose every item is a
    short label carrying a figure.
  - Deliberately narrow: a bare number is a period, not a measure; a timeline
    of metrics against real periods is a trend; one named stop clears the rule;
    a step that mentions a figure in prose is still a step. Verified silent on
    all 19 bundled example decks and 15 of the 16 calibration decks.
  - `B07_wrong_pattern` now fails the quality gate for this reason and has been
    removed from `staticPathMisses`.

- **`plan_deck` → `budget_note` (go-slide-creator-whp97).** Role allocation
  scaled every slot count with the slide budget alone, so a brief with one
  comparison ("compare build vs buy") produced three comparison slots at a
  20-slide budget — and the comparison role may only use `comparison-2col` /
  `before-after`, so two of those three had to repeat whatever the variety cap
  said. The repeats were reported in `rhythm_check.repeated_families`, which
  described the problem rather than avoiding it.
  - Each role is now bounded by the pattern families it can draw on AND by what
    the brief carries for it. Surplus slots move to roles with headroom.
  - When nothing can absorb the surplus the plan returns **fewer slides than
    `slide_budget`** and explains it in the new top-level `budget_note` string
    (omitted when the plan used the whole budget).
  - The variety cap also searches the whole registry instead of a 12-candidate
    shortlist, and never moves a `comparison` slot outside its family.
  - Net effect on the planner's own metric briefs at budget 20:
    `repeated_families` goes from up to three over-used families to empty.

- **Real pictures in `team-bios` and `pull-quote` (go-slide-creator-hdpq).** An
  "Our team" page and a customer quote with a headshot — two of the most common
  slides in a pitch or credentials deck — could only be built as a raw
  `shape_grid`, abandoning the pattern's own sizing, because neither pattern had
  anywhere to put an image.
  - `team-bios` gains `values.members[].photo` and `pull-quote` gains
    `values.image`, both the `{path | url, alt}` reference `image-text-split`
    already uses: relative paths resolve against the deck JSON directory, URLs
    are fetched and cached, and the picture is cover-cropped to its frame.
  - A `team-bios` member without a photo keeps the initials tile, so
    photographed and un-photographed people mix on one slide; `photo_label` now
    applies only to that fallback. `pull-quote` without an image is unchanged.
  - `pull-quote` also gains `overrides.image_side` (`left` default / `right`)
    and `overrides.image_width_pct` (15–40, default 25). The headshot is a
    sibling column of the quote, not a wrapper around it, so the readability
    preflight still measures the quote text.
  - Alt text defaults to the person: the member's name and role, or the
    quote's attribution and role.
  - Both patterns' `version` goes to 2.
  - A photographed quote has a quarter less width, so a 500-character quote
    beside a headshot renders smaller than one without; the schema description
    says so and `TEXT_BELOW_READABLE_MIN` reports it.

### Added

- **`waterfall-bridge` gains `values.caption` (go-slide-creator-2fq1).** A
  bridge draws no value axis, so the scale was only ever implied by the bar
  labels. `caption` (optional, <=60 chars) renders once above the bars, right-
  aligned in `dk2` — "EUR millions", "$m, constant FX". It takes its band off
  the bar row, so a deck without one expands and renders byte-identically.

### Added

- **`get_started` steps carry `args_template` (go-slide-creator-bxve).** Every
  step was `{tool, when_to_call}` with no arguments, so agents called the tools
  exactly as the prose named them — `list_templates{}` at 153KB instead of 44KB
  with `fields:"compact"`, `get_capabilities{}` at 183KB instead of the ~6KB
  default projection, `list_patterns{}` at 70KB instead of 31KB. The error-path
  `next_tool_call` blocks already carried `args_template`, so the shape existed;
  it was missing from the one response every agent reads first.
  - Each step now carries the arguments worth sending: the cheap projections,
    the flags a gate is weak without (`fit_report`), and `"<…>"` placeholders
    naming where a path or content hash comes from.
  - The hints live in one table keyed by tool, so a tool named in several
    sequences reads the same way everywhere, and a test checks every key
    against that tool's real input schema — a hint naming a parameter that does
    not exist is worse than no hint.
  - `get_started` gains `verbose` (default false). `quality_workflow` repeated
    the MCP `initialize` instructions verbatim, which every client already
    received, so it is now opt-in; `completion_protocol` still carries the same
    rule in structured form. The CLI passes `verbose` because a CLI caller
    never saw the instructions.
  - An unrecognised `task` still answers with the `brief` workflow — blocking
    an agent's first call helps nobody — but it now says so in `task_warning`
    instead of echoing `task: "brief"` silently.

- **`purge_render_cache`, and the render cache is now bounded
  (go-slide-creator-dpys).** Every `render_*` call wrote content-addressed PNGs
  under the temp dir, and the documented cleanup was "removed by
  InvalidateCache or OS temp cleanup" — but `InvalidateCache` had no callers
  anywhere in the repo, nothing capped size or age, and no tool exposed it. One
  review session left 418 files / 13MB behind; on a long-lived MCP server the
  cache only grew.
  - Renders now sweep the cache: artifacts are evicted after **24h** unused,
    then oldest-first once the total exceeds **500 MiB**. The sweep is
    stat-based and throttled to once per 5 minutes, so a burst of renders pays
    for one walk.
  - `get_capabilities().runtime` gains `render_cache_bytes` (current usage) and
    `render_cache_policy` (the rule). Both are required fields.
  - New `purge_render_cache` tool / `json2pptx purge-render-cache` command for
    the deliberate reclaim, with `all: true` to ignore the bounds. Returns
    `{bytes_before, bytes_reclaimed, bytes_after, scope, policy}`.
  - `ArtifactCleanupPolicy` — echoed as `cleanup` on every path-returning
    render response — now states the bound that exists instead of naming a
    function nothing called.
  - `SchemaVersion` 4.63.0 -> 4.64.0 (new tool, new required runtime fields).

- **Sibling argument names are accepted on the three tools that spelled them
  differently (go-slide-creator-r1m3).** `validate_presentation_output` takes
  `path` while five other PPTX tools take `pptx_path`; `resolve_theme` takes
  `template_name` while eleven tools take `template`; `plan_deck` takes
  `slide_budget` while every slide-counting response field is `slide_count`.
  Each mismatch cost an agent a round-trip.
  - The majority spelling is now accepted on each and rewritten to the declared
    name before the strict-argument check and the handler run, so handlers read
    one name and the schema advertises one name. `template` is also accepted on
    `examine_template`, `list_template_settings`, `register_template_setting`
    and `delete_template_setting`.
  - The declared name still works and WINS when both are sent: a caller
    honouring the schema must not have its value discarded.
  - A genuinely unknown argument is still rejected with `UNKNOWN_PARAMETER`,
    `did_you_mean` and a `rename_field` patch.
  - A registration-time test asserts that any tool taking a pptx path accepts
    `pptx_path`, any tool taking a template accepts `template`, and any tool
    taking a slide count accepts `slide_count`, so the next tool cannot
    reintroduce the cost.

### Changed

- **Pattern bounds now affect expansion sizing (go-slide-creator-a2ldx).**
  `bounds` and `max_height_pct` are resolved before a named pattern chooses
  card height, font size, and flow geometry. Explicit `bounds` remain relative
  to the slide and are clamped to the template's content zone; `max_height_pct`
  remains relative to that content area. Compose segment bounds are still
  ignored with a warning, including during sizing.

- **Pattern expansion uses the template body font in generation and preview
  (go-slide-creator-ex74s, go-slide-creator-o1yei).** Generation previously
  passed theme colors but lost `BodyFont`; preview passed neither. Both now
  pass the same template colors and body font to font-sensitive pattern sizing
  and readable fill selection.

- **Horizontal compose segments keep independent rows (go-slide-creator-rbjas).**
  The expanded shape grid now hosts each segment in a nested grid spanning its
  allocated columns. A multirow segment can retain its own row heights, gaps,
  and vertical alignment beside a single-row segment. Preview cells and text
  and geometry preflight traverse those grids; segment `col_range` still
  describes the same parent columns.

- **`contrast_predicted` covers placeholders whose colour is inherited
  (go-slide-creator-j4364).** A layout that leaves its placeholder colour to
  the slide master's `txStyles` reported an empty `FontColor`, and the contrast
  preflight — which pairs `FontColor` with the slide's own background — skipped
  it entirely. So a deck with `background.color` on a layout like
  midnight-blue's Section Divider got a `contrast_autofixed` swap at generate
  time and silence at validate time.
  - `PlaceholderInfo` gains `InheritedFontColor`: what the placeholder actually
    renders at, resolved through the layout's `clrMapOvr` the way the
    render-time pass resolves it. `FontColor` keeps its narrower meaning — "the
    layout declared this" — because `examine_template` reports it to agents as
    the template's own declaration.
  - The colour map is not optional: modern-template's Section Divider maps
    `tx1` -> `lt1`, so resolving against the theme alone would predict the
    opposite of what renders.
  - No new findings on the bundled examples (all 19 checked, before and after).

- **Peer structural fills are one accent at two luminances
  (go-slide-creator-at7ij).** `dual-org-ladder` coloured its two org headers
  `accent1` / `accent2` and `process-grid-2row` its two rows
  `accent1` / `accent3`. Templates pick accent2 and accent3 for contrast with
  accent1, not kinship, so forest-green rendered green beside bright orange and
  green beside bright blue; midnight-blue, navy beside red. Neither is a
  contrast failure — the halves are just peers (two orgs in one engagement, two
  tracks of one process) and two brand hues say otherwise.
  - Both now default the second fill to the first accent at `lumMod 65000`.
    The schema defaults for `accent_b` and `row2_color` are empty rather than
    a scheme name, and describe the derivation.
  - An authored `accent_b` / `row2_color` is used verbatim: this changes
    defaults, not decks that made a choice.
  - Text colour still comes from the measured contrast of the actual tinted
    fill, so the light labels on both rows stay readable.

- **`recommend_pattern` admits it is pattern-only (go-slide-creator-m2u2).**
  `recommend_pattern("org chart of the leadership team")` returned `team-bios`
  at 0.97 **high** — the `org_chart` diagram is not in its universe, and
  nothing in the response or the tool description said so. A high band on an
  answer the tool cannot see is worse than no answer.
  - The recommender now asks the multi-category scorer the same intent. When a
    chart or diagram lands within 0.08 of the top pattern, the response gains
    `beyond_patterns: {category, name, score}`, every `high` band is capped to
    `medium`, and `next_tool_call` points at `recommend_visual` with the same
    intent.
  - Frameworks that exist as both a pattern and a native diagram
    (`bmc-canvas` / `business_model_canvas`, `process-flow` / `process_flow`)
    are exempt: redirecting there is the same picture under another name.
  - The tool description now leads with "patterns only" and names
    `recommend_visual`.
  - An intent a pattern genuinely answers is untouched: `agenda for the
    session` and `three big number KPIs` keep their `high` band and emit
    neither field.

- **Dense heatmap labels are shortened instead of overlapping
  (go-slide-creator-3rkpt).** The builder divided the available height by the
  row count with no floor and drew each label at full length into a box one
  cell tall, so a 14x14 grid of long labels rendered every row label on top of
  its neighbours and broke the column headers mid-word — and nothing reported
  it.
  - Labels are now measured against the box they will be drawn into and
    shortened with an ellipsis. The budget is a line count with a floor of one
    line, so a row shorter than a single line still gets one readable line
    rather than three overlapping ones.
  - `GRID_DIAGRAM_NARROW` carries the report, naming the grid size and how
    many labels were shortened, with `fix.kind: reduce_text` and
    `fix.params: {diagram_type, rows, columns, cells, labels_shortened}`.
  - A heatmap whose labels already fit is untouched and silent: a 5x4 renders
    byte-identically.

- **Taxonomy frameworks use the deck's accent instead of a rainbow
  (go-slide-creator-w0kj).** `business_model_canvas` coloured its nine cells
  accent1–6 plus repeats, `pestel` its six accent1–6, and `swot` its four
  accent1–4. Templates define accent3–6 as unrelated hues, so a forest-green
  deck rendered a nine-colour canvas with a cyan "Customer Relationships" box,
  and warm-coral got a cyan "Threats" quadrant. Colour carried no information
  and overrode the deck's `accent_strategy`.
  - BMC and PESTEL now use one hue — the deck's accent1 — with the BMC Value
    Proposition a step deeper, since the canvas privileges it.
  - SWOT keeps two accents for the one contrast it actually encodes:
    Strengths/Opportunities against Weaknesses/Threats.
  - `porters_five_forces` is unchanged; its colour tracks each force's
    `intensity`, which is information.
  - **`style.colors` now applies to these three diagram types**, which
    previously ignored it. Entries apply in cell order and a short list
    repeats, so the old rotation is one `style.colors` away and a single
    colour recolours the whole framework. A `#RRGGBB` entry is used at full
    strength; a scheme name keeps the standard cell tint.

- **kpi-Nup's 45% is documented as a base, not a cap
  (go-slide-creator-4uxi).** `kpiMaxCardHeightFrac` was passed to `clampPt` as
  the *lower* bound, so the "cap" was a floor and the row rose to whatever the
  tallest card needed. Renamed `kpiBaseCardHeightFrac`, and SKILL.md /
  docs/PATTERNS.md now state the invariant that is actually true. No behaviour
  change: a tighter ceiling was considered and rejected because a row cannot
  exceed the content area anyway, so capping below it would only clip cards
  whose text genuinely needs the height.

- **A thin delta's value label now sits against its bar
  (go-slide-creator-2fq1).** A bar too small to hold one line of value text
  already moved its label into the adjacent spacer, anchored to the bar's
  edge — but the text box's own ~3.6pt inset then held it further clear. On a
  6pt bar that read as a number floating level with nothing while every other
  label sat inside its bar. The near-side inset is now collapsed, so the label
  reads as belonging to the bar it labels. In-bar labels are unchanged.

- **`SEMANTIC_PATTERN_DEGRADED` splits the fallback advisories off
  `SEMANTIC_DENSITY` (go-slide-creator-kjc8l).** Eight slide kinds reported the
  same thing — "you asked for a visual, you are getting bullets" — as
  `SEMANTIC_DENSITY`, sharing the code with plain count-range advice. The code
  was a misnomer for the budget cases (a KPI value twenty characters too long
  is not a density problem) and an agent could not tell a fallback from a
  suggestion without parsing the message.
  - Every advisory that announces a fallback now carries
    `SEMANTIC_PATTERN_DEGRADED`: `agenda`, `team`, `stat`, `architecture`,
    `timeline`, `matrix_2x2`, `framework`, `image_case`, `decision`, `process`,
    `roadmap`, `executive_summary`, `kpi_snapshot`, `option_matrix`,
    `chart_insight` and the `comparison` cases that no comparison pattern can
    hold.
  - Each carries `fix.kind: "restore_visual"` with
    `fix.params.{from, to, reason}` — `from` is the pattern refused (`""` when
    the kind has not committed to one), `to` is `content-bullets` /
    `content-slide` / `native-two-column` / `insights-only`, and `reason` is
    `count_out_of_range` / `budget_exceeded` / `columns_unbalanced` /
    `scores_incomplete` / `score_unreadable` / `chart_data_missing`. The two
    `chart_insight` data findings keep their `provide_value` fix and gain the
    same three params, so the contract is uniform however the finding was
    raised.
  - `SEMANTIC_DENSITY` keeps only the advisories that do NOT cost the slide its
    visual: an over-wide or over-tall table, a row with fewer cells than its
    header, an unbalanced comparison a card-grid still draws, a dropped
    `table-highlight` badge.
  - `restore_visual` joins `get_capabilities().vocabularies.advisory_fix_kinds`.
    `describe_finding("SEMANTIC_PATTERN_DEGRADED")` returns remediation steps
    that branch on `reason`.
  - Severity and strictness are unchanged (warning under `warn`, error under
    `strict`), as are the messages and paths. No finding appears or disappears —
    only the code and the fix change.

- **`agenda-with-images` keeps the placeholder column rectangular
  (go-slide-creator-jodu).** `image_label` was read as a per-row switch: a row
  without one dropped its placeholder and spanned its title across the gap, so
  a five-item agenda whose last row had no preview rendered four tinted boxes
  and a hole, and the grid read as ragged.
  - The column is now all-or-nothing. One `image_label` anywhere earns every
    row a placeholder; rows without a label get the same tinted box with no
    caption.
  - No `image_label` on any item still collapses the column for every row and
    renders byte-identically to before.
  - `image_label` is documented as what it is — the caption inside the
    placeholder, not the switch that creates it. The `show_pattern` schema
    description says so.

- **`horizontal-bar-with-callouts` drops the callout column when no bar has a
  callout (go-slide-creator-i0x0).** `callout` has always been optional, but the
  layout was not: bars with nothing to say still reserved 40% of the width for
  an empty column and drew the per-bar accent tick beside it, so the slide came
  out as a row of floating coloured dashes next to bars squeezed into the left
  60%.
  - No bar carries a callout -> the outer grid is a single 100% column and the
    bars span the full content width. The label gutter keeps its absolute width
    (22% of the old 60% column = 13.2% of the full width), so every reclaimed
    point goes to the bars and the longest bar reaches the axis.
  - Some bars carry a callout -> the two-column split is unchanged, but the
    accent tick renders only on rows that actually have an insight to anchor.
  - No input change: a deck where every bar has a callout expands and renders
    byte-identically. `max_value` already defaulted to the largest bar value;
    what made the top bar look short was the empty column beside it.

- **An unknown icon name in a diagram panel is reported
  (go-slide-creator-puki).** `panels: [{title: "Simplify", icon: "layers"}]`
  rendered a panel with no icon while its siblings kept theirs, and the only
  trace was a server log line: `WARN panel native shapes: icon not embedded`.
  A shape_grid icon *cell* already reported the same typo, so one mistake had
  two fates depending on which surface the author used.
  - `ICON_BUNDLED_NAME_UNKNOWN` is now emitted for `panels[].icon` too, at
    `…diagram_value.data.panels[N].icon` (and at the cell's own path inside a
    shape_grid or pattern-expanded grid), carrying the same Levenshtein
    suggestions and a `replace_value` fix.
  - `action: review`, not the cell path's error: the panel still renders,
    minus its icon — which is exactly why it went unnoticed.
  - Reaches `validate_input`, `validate --fit-report`, `score_deck` and the
    `generate --json-output-report` fit findings.

- **Pattern ink follows the contrast fixer's order (go-slide-creator-1sel).**
  `readableTextOn` picked whichever of `lt1`/`dk1` had MORE contrast, so a light
  accent got `dk1` — pure black — while the generator's own contrast fixer
  swapped the card's text on the same fill to `dk2`. A black icon sat beside
  brand-dark text on one card, and the off-brand black
  go-slide-creator-sis2 removed from text was still reachable through every
  pattern that asked.
  - Both now try `lt1` → `dk2` → `dk1` and take the first that clears the bar,
    which is exactly `pickThemeTextColor`'s order. Text uses the 4.5:1 text bar,
    icons the 3:1 graphical one.
  - When nothing clears, the highest-contrast candidate still wins, so no case
    gets worse.
  - Verified on the reported deck: the accent3 card's icon and caption are both
    `#1B2A4A` (midnight-blue's dk2), and no icon in the deck is `#000000`.

- **Every fit finding carries `severity` (go-slide-creator-dwkkf).**
  `generate_presentation`'s `fit_findings` had no severity at all while
  `validate_input` reported the same finding as `info`, so a client filtering by
  severity got different answers from two tools about one fact.
  - `FitFinding` now marshals a derived `severity` (`refuse` → `error`,
    `shrink_or_split` → `warning`, everything else → `info`) on every surface
    that serialises one.
  - The mapping moved next to the actions themselves and
    `diagnostics.SeverityForAction` delegates to it, so the envelope and a raw
    `fit_findings` array cannot drift apart again.
- **`UNKNOWN_PARAMETER` suggests a synonym (go-slide-creator-dwkkf).** The
  server's instructions promise a `did_you_mean`, but
  `show_pattern{pattern: "agenda"}` got only "Accepted arguments: name":
  "pattern" and "name" share neither spelling nor tokens, so neither existing
  rule fired. A small same-meaning table (`pattern` → `name`, `prompt` →
  `brief`, `deck` → `presentation`, …) is consulted last, and only when the
  tool actually accepts the target.

- **Embedded templates keep their name (go-slide-creator-qbks).** A server
  started without `--templates-dir` materialised its template as
  `json2pptx-template-356087472.pptx`, and everything downstream that reads a
  template's identity off its base name saw a template called
  `json2pptx-template-356087472`. `recommend_visual` therefore returned no
  `layout_preview_png_path` at all and every candidate stayed
  `metadata_only: true` — on the documented default invocation.
  - The random part now goes in the temp DIRECTORY name
    (`<tmp>/json2pptx-template-XXXX/midnight-blue.pptx`), so the file keeps the
    template's own name and the embedded layout previews resolve.
  - Fixes every name-derived lookup at once rather than only the preview path.

- **`value-chain` fits its step labels (go-slide-creator-vo0j1).** The label
  size was fixed at 12pt regardless of the step count, so a ten-step chain
  rendered "Manufactur / ing" and "Decommissi / oning end- / of-life" — the
  bundled `examples/value-chain.json` shipped it.
  - The pattern now measures each label against its own column width, shrinks
    one shared size to fit, and reports what still cannot fit as a blocking
    `TEXT_EXCEEDS_SHAPE` from `PostExpandWarnings`.
  - The shrink floor is the RENDERER's floor (`shapegrid.MinTextSizePt`, 12pt).
    A first cut used 9pt and changed nothing: the shape_grid renderer raises any
    authored size below its floor back to 12, so the fit promised a size the
    render never used.
  - `examples/value-chain.json`'s ten-step slide is shortened accordingly and
    now renders with no mid-word break.

- **A measured mid-word break fails the quality gate (go-slide-creator-rxkt).**
  `TEXT_EXCEEDS_SHAPE` was advisory from every source, so `score_deck` returned
  `quality_gate.passed = true` on a deck whose chevrons rendered as
  "Internationalisatio / n programme".
  - A pattern raising the code from its own `PostExpandWarnings` — it has
    MEASURED that the text cannot fit at its readable floor — now carries
    `action: "shrink_or_split"`, which the default gate counts as P1 and
    refuses. `numbered-step-strip` is the first: it emits the code once it has
    surrendered every notch depth and shrunk the label to 12pt.
  - The deterministic geometry detector's own `TEXT_EXCEEDS_SHAPE` stays
    `review`. It estimates the box from authored sizes, and the estimate runs
    tight: a word measured 15% wider than its box renders on one line in
    `examples/process-grid-2row.json`. Blocking on the estimate would fail decks
    that are correct.
  - Verified: the reported deck now fails the gate naming the P1 finding, and
    passes again once the labels are shortened. No bundled example or
    calibration deck raises the blocking form.

- **`score_candidates` can tell candidates apart (go-slide-creator-sbqm).** The
  tool for choosing between alternative slides scored a real slide 100, a
  near-empty one 95 and an unreadable one 90 — a spread an agent reads as "all
  three are fine, take the first".
  - Each candidate now carries `axes: {fit, content, rhythm}`, the same
    severity weights split by what they judge, so two candidates with the same
    total can still be told apart and an agent can see WHICH dimension a
    candidate lost on.
  - `blocking_findings` counts refuse-action findings, and a candidate with any
    is ranked from **50** rather than 100: a slide the engine would refuse to
    render is not a choice. Measured on the reported trio: 95 / 20 / 5.
  - `slide_score` is unchanged and stays the number `score_deck` reports for the
    same slide; `score` is the ranking number.
  - New top-level `tie`: when the leading candidates score identically it says
    so, names what the tool cannot judge (which visual reads better) and points
    at `render_slide_image` / `inspect_slide_images`. Rank 1 on a tie is input
    order, not a verdict.

- **A misshapen chart list names the key, not the block
  (go-slide-creator-xp1x).** `data: {categories: [a,b], series: {Rev: [1,2]}}`
  reported "chart.data declares no data series (found keys \"categories\",
  \"series\")" — a message that contradicted itself and told the agent to
  replace the whole data block rather than retype one field.
  - A `series` (or, for pie/donut, `values`) that is present but is not a JSON
    array now reports `SEMANTIC_FIELD_TYPE` at
    `slides[i].chart.data.series`, naming what it is ("an object (keys
    \"Rev\")") and the shape it must take.
  - The original message is kept for the case it was written for: no such key
    at all.

- **The fact-loss guard proposes only a split the pattern accepts
  (go-slide-creator-qtjl).** `reduce_items` / `resize_list` refuse to drop an
  item carrying a figure and propose `split_pattern` instead. The proposal was
  unconditional, so on a 4-point `exec-summary` — which needs at least 3 points
  — following the refusal's own `next_tool_call` produced `points must contain
  at least 3 items, got 1`. The remediation the refusal advertised led into a
  hard error.
  - The guard now validates both halves before proposing: it keeps the point it
    was going to suggest when that is legal, falls back to the most balanced
    legal point otherwise, and when the pattern has no legal split at all
    returns no `next_tool_call` and a message naming the constraint and the
    alternatives.
  - `split_pattern` itself already refused an illegal point; it now shares one
    half-builder with the legality probe, so what the guard promises and what
    the fix does cannot drift.

- **Embedded chart and diagram fonts are subsetted (go-slide-creator-i0ep).**
  Every SVG the engine embeds carried the *whole* font family as base64 — 547 KB
  of Calibri regular plus 552 KB of its bold — re-embedded per chart. A
  six-chart deck shipped 6.6 MB of media for about 5 KB of actual drawing, and
  a 40-slide deck of charts would have been ~45 MB.
  - The canvas SVG renderer now subsets each embedded face to the glyphs that
    chart draws. `examples/charts.json` goes from 3.4 MB to 380 KB (each chart
    SVG 1.10 MB → ~20 KB), and the bundled example decks together from 14 MB to
    2.5 MB.
  - Renders are unchanged: every chart and diagram SVG was rasterised before and
    after and compared, with differences confined to glyph-edge antialiasing.
  - No JSON or response change.

- **Native `panel_layout` and `stat_cards` are sized to their content
  (go-slide-creator-5smrk).** A columns panel group gave every body rectangle
  the whole remaining placeholder height and a stat-card grid gave every card
  an equal share of it, so one sentence rendered as a bordered box around
  roughly 80% empty space and a two-line stat card as a tile three times
  taller than its own text.
  - Each body is now measured at the panel's own width; the band is sized to
    the tallest body in the row (so the panels keep a shared baseline) and the
    whole group is centred vertically in the placeholder. Stat cards are sized
    the same way, from the hero, caption and delta lines measured at their own
    font sizes.
  - Content that needs the full height keeps it: the sizing only ever shrinks,
    and a dense panel renders exactly as before. The measured height carries
    half a line of slack so a substituted font face cannot clip the last line.
  - No JSON, response or finding-code change — `panels[]` is authored exactly
    as before.

- **One title-length rule, measured (go-slide-creator-jcph).** Three
  contradictory numbers used to answer "is this headline too long?" on one
  deck: `title too long (106 chars, max 60)` from the quality score,
  `headline is 13 words; trim to 12 or fewer` from the content lint, and
  `max_chars: 29` in `validate_input`'s placeholder metadata for a layout that
  renders ~100 characters comfortably. An agent asked to shorten all titles had
  no single target, and a visibly two-line 54-character title still scored 100.
  - The measured verdict (`title_wraps` escalated to `shrink_or_split`, or
    `TITLE_OVERFLOW`) is now the only title-length finding. `validate` emits the
    identical record — same code, path and message — so the findings envelope
    carries it once; `max_length` no longer appears for titles.
  - The 60-character score fallback is deleted. A title that cannot be measured
    scores clean and is covered by `HEADLINE_TOO_LONG`, which in turn stands
    down for any title the measurement covered.
  - `max_chars` reported for a **title** placeholder is now the measured
    capacity of the box at the comfort size for the resolved font, not the
    geometric area estimate — in `validate_input` / `generate -dry-run`
    (`slides[].placeholders[]`), discovery (`layout_summaries[]`, full-mode
    `layouts[]`) and `examine_template` (`report.json` `layouts[]`). It is a
    property of the box, not of the current text, so `len(title) <= max_chars`
    means "no title finding". Body and other placeholders keep the estimate.
  - The `compact-title` layout tag is unchanged and remains the signal for a
    short section title; it is not derivable from the reported `max_chars`.

- **Porter's five forces no longer invents an intensity
  (go-slide-creator-ceodq).** A force with no `intensity` defaulted to `0.5`,
  so the renderer printed "Medium (50%)" under it and gave it the accent3
  tint. A chart nobody scored looked exactly like one scored medium across the
  board, and the colour coding — the whole point of the intensity — said
  nothing.
  - An unstated intensity is now distinct from `0.5`: the force is drawn on the
    template's neutral surface with **no intensity line at all**. A partially
    scored chart therefore shows which forces were assessed and which were not,
    on the slide itself.
  - A second contrast defect on the same slide: the intensity line was painted
    in the box's own scheme colour on the box's own tint of it, measuring
    1.55:1 on the accent3 tile. It is now picked against the fill a reader sees
    (5.63 / 7.08 / 10.11 after the fix).
  - The DeckSpec `framework` kind passes a stated intensity through, including
    the words `"high"` / `"medium"` / `"low"`, and does not invent one either.

### Added

- **`alt` on charts, diagrams and tables (go-slide-creator-6e8h).** Every chart
  was embedded with `descr="Revenue ($M) (bar_chart)"` — the title and the type
  name — and native diagram groups and table frames carried no `descr` at all,
  so a screen reader announced the shape of the picture and nothing in it, or
  nothing whatsoever.
  - New optional `alt` field on `chart_value`, `diagram_value`, `table_value`,
    and the `diagram` / `table` keys of a `shape_grid` cell. One sentence
    saying what the visual shows; it is written verbatim into the shape's
    `cNvPr/@descr`.
  - Without an `alt`, the engine now derives one from the payload instead of
    naming the type: `"Bar chart, Quarterly revenue ($M). 4 categories,
    1 series (Revenue), values from 34 to 48."` for a chart,
    `"Process flow. 5 steps."` for a structural diagram, `"Table, 4 columns by
    5 rows. Columns: Segment, FY25 revenue, FY26 revenue, Change."` for a table.
  - `MISSING_ALT_TEXT` covers two more surfaces (six in total): a content-level
    chart/diagram and a content-level table with no `alt`, plus the `diagram`
    and `table` cells of an author-written `shape_grid`. Still advisory
    (`review`) — the visual always renders. Pattern-expanded grid cells are
    exempt; the author has no field to annotate there.
  - DeckSpec visuals get an `alt` for free: `chart_insight`, `framework` and
    `table` compile the slide's `takeaway` (else its `title`) into the visual's
    `alt`, which an `alt` on the author's own `chart` payload overrides.

- **`get_started` says what the server can actually do
  (go-slide-creator-a7fh).** Started without LibreOffice or ImageMagick, the
  whole surface looked identical: `initialize`'s instructions were
  byte-for-byte the same, `tools/list` still advertised
  `render_deck_thumbnails`, and `get_started` still handed back a workflow
  ending in "render every slide and look at it". Only
  `get_capabilities.runtime` knew — 183KB into the session — and the agent
  found out when the MANDATORY completion step failed.
  - Every `get_started` response now carries a **`runtime`** block
    (~200 bytes): `{render_available, missing_commands[], templates_dir,
    output_dir, settings_write_enabled}`. It also hands an agent the output
    directory on its first call.
  - When rendering is unavailable, the server's `initialize` **instructions**
    gain a `RENDER TOOLING MISSING` line, `get_started` drops the render /
    inspect steps from `fast_path` and `sequence`, the step that now ends the
    path says to deliver `pptx_path` and declare the deck UNREVIEWED, and
    `completion_protocol.complete_status` becomes `draft_needs_visual_review`.
    A path that never rendered (`validate-only`) is left exactly as it was.
  - On a server with the toolchain installed, nothing changes.

- **DeckSpec kind `architecture` (go-slide-creator-162os).** A 10-slide
  product-launch deck needed `raw_json2pptx` for its platform stack, which drops
  the author back into the raw pattern dialect (values-object vs values-array,
  `name` vs `label`) with none of the DeckSpec `semantic_path` diagnostics.
  - `{tiers: [{label, description?|items?}], rails?}` compiles onto the
    **arch-stack** pattern: 3–6 tiers top-first, up to 3 cross-cutting rails
    beside them. `layers` aliases `tiers`, `side_rails` aliases `rails`, and a
    tier's `items` are joined into its detail line.
  - Outside the pattern's budgets (tier count, a 60-char label, a 120-char
    detail, a 30-char rail) the slide degrades to a bullet list carrying every
    word, rather than being truncated into a `maxLength` or blocking the render.
    `validate_deck_spec` says which budget it broke.
  - `explain_deck_spec` advertises `arch-stack` only when the payload will
    actually compile to it, so explain and compile agree.
  - The core `tools/list` budget rises from 96KB to 104KB (99,172 bytes today):
    every kind is embedded in both spec tools. Shrinking that schema is filed
    separately (go-slide-creator-gvi6z).

- **`chart_value.style.value_format`: one number format per chart
  (go-slide-creator-e2ck9).** Within one chart the axis and the data labels were
  formatted by different code with different rules: on `[1240, 865, 413]` the
  axis printed `1000 / 1200 / 1400` while the bars printed `1,240 / 865 / 413`,
  and past 9,999 the axis switched to compact notation while the labels kept
  grouping digits. Nothing reached both — `data.data_labels.format` formatted
  the labels only — so a EUR deck could not get `€1.2M` on the axis at all.
  - **`style.value_format`** `{style: plain|compact|percent|currency, decimals,
    prefix, suffix, thousands_sep}` is applied to the value-axis ticks, the data
    labels and any in-mark label alike. Accepted on `chart_value.style` and on a
    diagram's `style`. The keys are closed and walked by the unknown-key check,
    so a typo inside the block is reported rather than dropped.
  - **The default now agrees with itself.** One formatter is derived per chart
    from its own values and used by every side: grouped digits with enough
    decimals to keep the labels distinct, switching to compact notation on a
    chart WITH a value axis once the numbers pass 9,999 (the axis's own
    long-standing rule, now shared). Charts with no axis to agree with (funnel,
    gauge, treemap) keep grouped digits. The axis still takes its decimal places
    from its tick step, not from the data, so a whole-numbered axis prints `1`
    and not `1.0`.
  - Renders that change: axis ticks over 999 now group (`1,000` where it said
    `1000`), and a bar/line chart's labels compact when its axis does
    (`1.2M` where it said `1,240,000`). A caller-supplied
    `data_labels.format` is still honoured verbatim and never rewritten.

- **`render_deck_thumbnails` can render just the slides that changed
  (go-slide-creator-2018).** Its only narrowing knob was `max_slides`, a prefix
  cap, so a repair loop that fixed one slide of fifteen re-pulled all fifteen
  thumbnails — measured at 346KB of base64 per pass, nearly all of it images the
  agent had already seen.
  - **`slide_indices: [int]`** renders only those 0-based slides, one image block
    each, ascending, de-duplicated. Pass `render_deck_spec` /
    `validate_deck_spec`'s `changed_slides` verbatim. Mutually exclusive with
    `max_slides` (`AMBIGUOUS_INPUT`). An index the deck does not have is an
    `INVALID_PARAMETER` naming the real slide count, never a silent omission; an
    empty array is refused rather than read as "the whole deck".
  - The response gains **`slide_count`** (the deck's size, whatever came back)
    and **`selected`** (the indices that did), so a narrowed pass cannot be
    mistaken for a full one.
  - Measured on the 15-slide deck: full pass 346,278 response bytes / 15 images;
    `slide_indices: [4, 9]` 39,643 bytes / 2 images.
  - CLI parity: `json2pptx render-thumbnails --slides 1,3`.

- **Deck handles: a revision is a patch, not a re-upload
  (go-slide-creator-voxp).** Every spec tool was stateless, so the agent was the
  only place the deck lived and paid for that on every call: a measured 15-slide
  DeckSpec loop resent the whole 3,695-byte spec on 5 of 8 calls, and a one-line
  title fix cost a full spec re-upload plus a full-deck re-render plus 15
  re-inspected thumbnails.
  - `validate_deck_spec`, `render_deck_spec` and `explain_deck_spec` now return
    **`deck_id`** — a handle on the spec the server holds. `spec` is no longer
    required on any of them: send `deck_id` instead. Setting both is
    `AMBIGUOUS_INPUT`; an unknown or expired handle is `INVALID_PARAMETER`
    naming `spec` as the way back. Handles are per server process, live 1 hour,
    and refresh on each use.
  - **`patch`**: `[{op, path, value}]` applied to the stored spec before the
    call acts on it. `op` is `replace` | `add` | `remove`; `path` is a JSON
    Pointer into the DeckSpec (`/slides/3/title`, `/meta/template`, `add` at
    `/slides/6` to insert and at `/slides/-` to append, `remove` at `/slides/2`
    to drop). Ops apply in order; a malformed one is refused naming its index
    and leaves the stored deck untouched. Requires `deck_id` — a `patch`
    alongside a literal `spec` is refused rather than silently ignored.
  - **`changed_slides: [int]`** on all three responses: the 0-based slides the
    patch changed, compared by per-slide content digest, which is the `slides`
    list to pass to `render_deck_thumbnails`. Inserting or removing a slide
    reports the shifted tail, because it moved.
  - A handle remembers the template its last render resolved to, so a
    `render_deck_spec` driven by `deck_id` with no `template` cannot silently
    restyle the deck. A YAML spec is stored as canonical JSON (and its
    remembered filename follows), so YAML decks patch like JSON ones.
  - The stateless form is unchanged: every one of these arguments is optional,
    and a server without a handle store simply omits `deck_id`.

- **`get_started(task="revise")` leads with the DeckSpec
  (go-slide-creator-voxp).** Its `fast_path` was the raw-deck repair chain
  (`auto_repair`, or `repair_slide` where the core profile hid it) and named no
  DeckSpec at all, so a deck authored the recommended way had no documented
  revise path. `fast_path.tool` is now `render_deck_spec` in both profiles, with
  `deck_id` + `patch` → re-render → thumbnails of `changed_slides` as its
  `steps`; the raw-deck chain remains the second branch in `sequence`, and the
  notes now say which path belongs to which kind of deck (`repair_slide`'s fixes
  edit compiled slide JSON and do not travel back into a spec).

- **MCP resources: the deck is deliverable without a filesystem path
  (go-slide-creator-fx52).** `resources/list`, `resources/read` and
  `resource_link` all returned `-32601`, and `initialize` advertised
  `tools` only. The whole workflow's one deliverable came back as
  `pptx_path: /abs/path/on/the/server`, so on a containerised server, a remote
  deployment, or any sandbox whose file tools cannot see the server's output
  directory, the agent could not hand the user the deck it had just made.
  - The server now advertises the **resources capability** (no subscriptions —
    a generated deck is immutable once written; no list-changed — the static
    set is fixed at startup).
  - **`json2pptx://deck/{filename}`** serves a generated `.pptx` as a base64
    blob with the OOXML presentation media type. The blob's sha256 equals the
    response's `content_hash`. The URI names a file inside the output directory
    and nothing else: the name is reduced to a base name, so a traversal, an
    absolute path or a non-`.pptx` is refused.
  - **`generate_presentation` and `render_deck_spec` append a `resource_link`
    content block** pointing at that URI, so a host can offer a download
    without reading the blob. It is additive — the JSON payload is unchanged —
    and a failed call carries none.
  - **Four read-only resources** serve what agents otherwise pay a tool call
    for every session, cacheable by the host: `json2pptx://templates`,
    `json2pptx://patterns`, `json2pptx://schema/deckspec` and
    `json2pptx://skill` (SKILL.md, now embedded in the binary).

- **Slide backgrounds: `background.color` and `background.overlay`
  (go-slide-creator-uy5s).** `background` was `{image, url, fit}` only, so the
  standard "one dark slide in a light deck" — a statement, a section break, a
  pull quote — could only be faked with a full-bleed `shape_grid` cell, which
  then fought the layout's own title placeholder for the same space. And an
  image background had no scrim, so a hero photo rendered the template's dark
  title over whatever the picture happened to be.
  - **`background.color`** takes a scheme name (`dk2`, `accent1`, …) or a hex
    value and fills the slide via `<p:bg><p:bgPr><a:solidFill>`. A scheme name
    stays a scheme reference, so it follows the template's theme.
  - **`background.overlay`** `{color, alpha}` is a full-bleed scrim over the
    background image — the same shape a `shape_grid` image cell already
    accepted, one level up where a hero slide needs it. Defaults: `dk1` at
    `0.45`. It is inserted first in the shape tree, so it paints above the
    background and below every placeholder, grid cell and picture.
  - **Text contrast is enforced against what the audience sees**: the slide's
    own `color`, or the scrim when its alpha is at least 0.35, falling back to
    the layout's fill as before. A deck's dark statement slide now gets its
    title checked against the dark fill rather than against a layout background
    nobody can see.
  - **New finding `TEXT_OVER_IMAGE_UNVERIFIED`** (`review`, fix
    `provide_value`): a slide puts text on a background image with no overlay.
    A photo has no single colour, so no contrast verdict is possible — the
    engine says so instead of guessing one. Silent on picture-only slides and
    on slides that set `contrast_check: false`.

- **New slide kind `table` (go-slide-creator-e4h1).** A plain data table — the
  financials, the segment split, the pricing tiers — had no kind, so the most
  ordinary business slide there is needed `raw_json2pptx`; `kind: table` came
  back "unknown slide kind".
  - `headers` (alias `columns`) and `rows`. A row is a list of cell values or an
    object keyed by header label; short rows render blank cells rather than
    dropping a column, and cells keep the author's own numeric literal.
  - `column_alignments` (`left` / `center` / `right`), `highlight_column` (a
    header NAME or a 0-based index), `totals_row`, `column_types`. With none of
    them set, no style block is emitted and the template's own table style
    renders verbatim.
  - Compiles to a native table content block, so it inherits the engine's table
    machinery (banding, totals emphasis, font scaling, the fit report's density
    findings). `validate_deck_spec` reports the renderer's 6-column / 7-row
    budget and any row whose cell count does not match the headers, at
    authoring time rather than after a render.
  - Without a header row there is no table to build, so the rows degrade to
    bullets labelled by whatever headers exist.

- **New slide kind `option_matrix` (go-slide-creator-6o1r).** The DeckSpec path
  compiled to 5 of 43 patterns, and the options × criteria evaluation matrix —
  the slide a recommendation is actually argued on — was not one of them. Three
  reviewers independently reached for it and had to drop to `raw_json2pptx` with
  a hand-written pattern block.
  - `criteria` (2–6, alias `columns`): a string label or `{label, scale?}` for a
    per-column scale. `options` (2–6, alias `rows`): `{name, detail?, scores[]}`
    with exactly one score per criterion.
  - `scale` is `harvey` (default), `rag` or `text`; `recommended` and
    `decisive_criterion` take a NAME (matched against `options[].name` /
    `criteria[].label`) or an index, and highlight that row / column.
    `highlight_label` badges the recommended row.
  - Compiles to `table-highlight`. Outside the pattern's bounds — or with a
    score the scale cannot read — it degrades to a bullet list that still
    carries every score, labelled by criterion, and `validate_deck_spec` says
    which rule was missed at the field the author wrote (`criteria` vs
    `columns`).
  - A `highlight_label` past the pattern's 24-char budget is dropped rather than
    costing the whole matrix — the row it marks stays highlighted — and
    validation names the budget.

- **A comparison of more than two columns is a visual (go-slide-creator-3bgf).**
  `kind: comparison` only had a pattern for exactly two balanced columns.
  Everything else — the standard consulting three-option slide, a four-way
  competitor comparison, an unbalanced pair — collapsed to one run-on bullet per
  column (`"Parcel automation: EUR 60M capex; Payback 3.5 yrs; Low risk"`) on a
  60%-empty page, while `validate_deck_spec` only warned, so an agent following
  `ok: true` shipped it. The engine had the right patterns; only a hand-written
  `raw_json2pptx` block could reach them.
  - **3–5 columns → `stylish-panels`**: each column becomes a titled panel with
    its own bullet list, one bullet per item, rather than a joined line.
  - **2–5 columns `stylish-panels` cannot hold** (an unbalanced pair, a column
    with more than 8 items or an over-long one) **→ `card-grid`**: one titled
    card per column, items joined into the card body.
  - 2 balanced columns within the 10-row cap still compile to `comparison-2col`;
    6+ columns, or any column without a header, still degrade to bullets.
  - `SEMANTIC_DENSITY` no longer warns about a degradation that no longer
    happens: for 3–5 columns it is silent, and where it does fire it names the
    counts that have a visual and points at `raw_json2pptx` +
    `table-highlight` for a wider matrix. `explain_deck_spec` and
    `render_deck_spec`'s `explanation_summary` report the pattern actually
    emitted, and `compositionCandidates` lists all three.

- **`executive_summary` renders the exec-summary pattern (go-slide-creator-ku6t).**
  The kind ALWAYS compiled to a bullet list, so the exec-summary pattern shipped
  in round 1 (go-slide-creator-ycbn) was unreachable from the recommended
  authoring path — the deck's most important slide was the flattest one on it.
  - `points` now accepts `{lead, support}` objects as well as strings (aliases
    `point`/`statement`/`title`/`headline`/`text` for the lead,
    `detail`/`description`/`evidence`/`body` for the support). 3–5 points
    compile to the `exec-summary` pattern: numbered bold conclusions, each with
    its supporting sentence, separated by rules.
  - **`bottom_line`** is a new payload field — the recommendation / ask. In the
    bullet fallback it rides as the last bullet rather than being dropped.
  - The `exec-summary` pattern's bottom line was **redesigned**: it was a
    full-width tinted rectangle with a left stripe, which read as a near-twin of
    the generator's takeaway band sitting right below it — two similar pale
    rectangles stacked, neither reading as the conclusion. It is now a rule that
    closes the point list, a saturated accent flag (a `homePlate` labelled
    BOTTOM LINE, its point aimed into the text) and the statement in its own
    tinted box.
  - Any other point count, or a point past the pattern's budgets (`lead` ≤90
    chars, `support` ≤200), degrades to the bullet list as before —
    never silently: `validate_deck_spec` reports `SEMANTIC_DENSITY` at
    `slides[i].points` naming the degradation, and the explain projection
    advertises `pattern: "exec-summary"` only when the visual is what the
    render will actually produce.

- **Bring-your-own template: `template_path` on every tool that takes a
  template (go-slide-creator-ydbk).** A template reached the engine only by
  NAME, looked up in the server's templates dir and the embedded set, so an
  agent holding a client's `.pptx` had no supported way in: an absolute path in
  `template` failed `TPL.TEMPLATE_NOT_FOUND` listing the names it is not,
  `template_path` was an unknown key, and `examine_template` — the one tool that
  did take a path — was not in the core profile. BYO onboarding needed an
  operator.
  - **`presentation.template_path`** is a new deck-level field on
    `generate_presentation`, `validate_input` and `preview_presentation_plan`;
    **`template_path`** is a new tool argument on `render_deck_spec` and
    `list_templates`. Mutually exclusive with `template` (`AMBIGUOUS_INPUT`);
    either one satisfies the template requirement.
  - Containment matches `examine_template`: the path resolves against the call's
    `base_dir` (the server CWD when absent) and must stay inside it after
    `~`/`$VAR` expansion and symlink evaluation. An escape is `INVALID_PATH`
    naming the `base_dir`, a missing file `FILE_NOT_FOUND`, a non-`.pptx`
    `INVALID_PARAMETER`. `base_dir` is now declared on `list_templates` and
    `render_deck_spec` for this.
  - **CLI:** a deck's `template_path` resolves against the deck JSON's own
    directory, the same frame relative asset paths use, so a deck and its
    template travel together. No `base_dir` guard there — the caller is the user
    running the binary.
  - **`examine_template` joined the core profile.** It is the only tool that can
    vet an unregistered `.pptx`, and a core agent never saw it.
    `coreToolListByteBudget` moved from 80KB to 88KB to fit it (2.9KB).
  - **`get_started(task: "onboard-template")`** is a new task:
    `examine_template` → `describe_finding` → `list_templates` →
    `generate_presentation` → `render_deck_thumbnails`, with notes covering the
    containment rule and the live install-by-copy path. It has no `fast_path`
    facade — nothing can decide for the agent that a template is fit.
  - **`TEMPLATE_NOT_FOUND` redirects** when the value looks like a path: it now
    says `template` takes a name, names `template_path`, and reports the server's
    `templates_dir`, with a `rename_field` fix carrying the value to move.

- **DeckSpec carries deck chrome, speaker notes and per-slide sources
  (go-slide-creator-zmjs).** `get_started("brief")` and SKILL.md send agents to
  `render_deck_spec` as the path for a new deck, but the spec could not express
  a confidentiality line, page numbers, speaker notes, or a source on any kind
  but `chart_insight`: `meta.chrome` failed with *unknown meta field* and the
  deck did not compile, while a slide-level `source` was accepted and reported
  as DROPPED. Every board deck has all three, so the recommended path produced
  unshippable decks and agents had to abandon it for raw
  `generate_presentation`.
  - **`meta.chrome`** `{confidentiality, client_name, project_code, footer_date,
    section_crumb, page_numbers:{enabled, format, skip[]}}` mirrors
  `PresentationInput.chrome` and compiles straight through; `footer_date`
    defaults to `meta.date`.
  - **`meta.viewing_mode`** and **`meta.accent_strategy`** pass through, with the
    spec's own value winning over the tool/CLI argument.
  - **`notes`** and **`source`** are now payload fields on EVERY slide kind,
    mapping to `speaker_notes` / `source` on the compiled slide. `chart_insight`
    keeps its own `source` handling.
  - The DeckSpec JSON Schema (and therefore the MCP `spec` input schema) declares
    all of it. The core profile's tools/list grew from ~70.6KB to ~74.8KB, so
    `coreToolListByteBudget` moved from 72KB to 80KB.

  Verified end-to-end through `render_deck_spec`: the PPTX carries
  `notesSlide1/2.xml`, "Source: Finance close pack, 30 Sep 2026" under the body,
  and the footer "Strictly confidential — Acme Corp | September 2026" with
  `{current} / {total}` page numbers.

### Added

- **Ordered lists render as ordered lists (go-slide-creator-6or2).**
  `bullets_value: ["1. First do this", "2. Then do that"]` rendered as
  `• 1. First do this` — the layout's bullet glyph AND the author's typed
  number. OOXML auto-numbering was already implemented but only the
  `shape_grid` text path ever asked for it, so the routine ordered list
  (steps, rankings, priorities) had no clean form in a placeholder at all.
  - A `bullets` / `body_and_bullets` list whose entries are numbered **from 1
    with no gaps** is now rendered with `<a:buAutoNum type="arabicPeriod"/>`
    and the typed prefixes removed from the text. One marker, drawn by
    PowerPoint.
  - Anything else keeps the author's text verbatim and reports
    **`NUMBERED_LIST_NOT_APPLIED`** (`review`) naming how many bullets carry a
    prefix, with a **`renumber_bullets`** fix. A single line that merely opens
    with a number ("2024. A big year") is prose and is exempt.
  - New executable repair kind **`renumber_bullets`**
    (`params: {path, strip?}`): renumbers the list from 1, or removes the
    prefixes when the list was never ordered.

### Changed

### Fixed

### Added

- **`plan_deck` caps how often one pattern family repeats, and reports what it
  cannot fix (go-slide-creator-8fq4).** A 20-slide plan came back with
  `comparison-2col` four times and `arch-stack` three times — spread out, so
  the run-breaker (which only looks at neighbours) never saw them — and a
  10-slide plan for a metrics brief used four different `kpi-*` patterns, which
  the same blindness allowed because the names differ.
  - The rhythm pass now caps a pattern FAMILY at two slides per deck. Families
    group what an audience reads as the same slide: `kpi-2up`…`kpi-6up` and
    `kpi-inline` are one, and a pattern's `-compact` variant is one with its
    full-size self.
  - The cap is best-effort by design: a role with few candidate patterns (four
    comparison slots in a 20-slide deck) can run out of alternatives, and
    swapping in a pattern that does not suit the role would be worse. What it
    cannot fix it reports, as **`rhythm_check.repeated_families`** —
    `["comparison-2col x3"]` — so the repetition is visible in the plan instead
    of in the render.
  - Replacements never spend the deck's emphasis budget: the emphasis cap has
    already run by then.

- **One source convention across slides and patterns (go-slide-creator-7eib).**
  `slide.source` rendered as `Source: <text>` at 8pt in a raw `#888888`,
  right-aligned in the chrome band, while `chart-insights-split` drew its own
  `values.source` verbatim at 9pt `dk1` on the left and `stat-hero` at 10pt
  centred. One deck showed its sources in three sizes, two alignments and two
  colours depending on which pattern owned the slide — and an author who wrote
  `"Source: …"` into the slide field got `Source: Source: …` on a pattern
  slide.
  - `internal/patterns.SourceNoteText` is now the single rule for the prefix,
    and `SourceNoteSizePt` / `SourceNoteScheme` / `SourceNoteAlign` the single
    look: italic, left, `dk2`, 12pt.
  - 12pt is decided by the tighter surface: shape_grid text is floored at 12pt
    by the renderer, so a pattern asking for 9 drew 12 anyway. The old 8pt band
    was also reported as barely legible at 100%.
  - The raw `#888888` is gone: the band now uses the template's own `dk2`, so
    the source line stays in the palette like everything else.

- **Funnel stages stay readable, and stop implying a valence
  (go-slide-creator-6i6j).** A real SaaS funnel (12,400 / 3,100 / 890 / 212)
  drew each stage proportionally, so the bottom two were 2-4px slivers with
  external leader lines, and the stage colours came from the categorical accent
  rotation — "MQL" in the template's alert red, "Won" in its positive green,
  a reading the data does not carry.
  - **`width_mode`** (`clamped` default, `proportional`, `equal`). Clamped
    interpolates between a 25%-of-plot floor and the full width, so ordering is
    preserved and every stage is still a shape with room for its label. In
    clamped mode an unset `neck_width` becomes 0.55 rather than a point, which
    is what pushed the smallest stage's label out onto a leader line.
  - **`show_conversion`** (default **true**) writes the stage-to-stage
    conversion under each stage's label — "25% of Visitors" — which is the
    number a funnel exists to show. `show_percentage` is unchanged and remains
    percentage-of-first-stage.
  - Stages now take a **single-hue ramp of accent1** (darkest at the top)
    instead of the categorical rotation. Caller-supplied `colors` are still
    honoured exactly.

- **Scatter and bubble stop labelling every point (go-slide-creator-daqp).**
  Six series of nine points drew all 54 labels, and the collision pass could not
  see across series because it ran INSIDE the per-series loop — the placed-label
  set was reset for each one, so labels from different series were laid straight
  over each other.
  - The label pass now runs once for the whole chart, with one shared collision
    set, after every series is drawn.
  - Above **15 labelled points** no labels are drawn at all: past that they
    cover the plot they describe. `chart.scatter_label_skipped` says how many
    were dropped (severity `warning`, fix `reduce_items`), and
    `style.show_values` forces them for a caller who wants them anyway.
  - A collision that remains under the threshold is still reported, but now as
    ONE finding for the chart with a count, rather than one per label — the
    per-label flood was being truncated by the finding cap, which told an agent
    nothing.
  - bubble_chart is a scatter with variable point sizes, so it reports the same
    way; it used to emit nothing at all.

- **`process-flow` chevron and arrow steps keep their label out of the point
  (go-slide-creator-czk4).** A step of `type: "chevron"` laid its text out to
  the bounding box, so the first and last characters were drawn into the notch
  and the tip, and the row's connector arrow ran straight through the shape.
  - Pointed steps (`chevron`, `arrow`) now draw at a 30% point rather than the
    OOXML default 50%, inset their text past the notch on both sides, and — when
    EVERY step is pointed — drop the connector, which was saying the same thing
    the shapes already said, through the shape.
  - A row of pointed steps is also capped at half its step width: the notch is a
    fraction of the shorter side, so a tall chevron eats its own label. At four
    steps that took the label from 110pt of a 198pt shape to 132pt, which is
    the difference between "Board sign-off" on two lines and "Boar / d / sign- /
    off" on four.
  - A flow of plain steps is untouched: same heights, same connectors, no
    insets.

- **`shape_grid.source` — an expanded pattern can be fed back into generate
  (go-slide-creator-c3po).** The documented workflow is
  `expand_pattern -> inspect -> tweak -> generate`, and the last step was
  impossible: the expanders emit explicit font sizes, constrained design mode
  refuses those from an author, and one `scqa-summary` expansion produced
  twenty `design_mode_violation` errors the agent had no way to remove without
  switching the whole deck to `design_mode: free` (which also unlocks raw hex
  and disables the other guards).
  - `shape_grid` gains **`source`**, which the engine stamps as
    `"pattern:<name>"` on every expansion. A grid carrying it is exempt from the
    absolute-font-size rule, because those sizes are the engine's own.
  - The waiver is narrow on purpose: raw hex colours are still refused on a
    stamped grid (the expanders emit none), and a `source` naming a pattern that
    is not registered waives nothing.
  - Verified end to end: the reported `scqa-summary` expansion embedded in a
    constrained deck validates clean, generates, and renders; the same grid with
    `source` removed reports the original twenty violations.

- **`auto_repair` / `make_deck` report `deterministic_ready` beside
  `publishable` (go-slide-creator-z0cx).** `publishable` was documented as "the
  single authoritative ship-as-is flag" and was false for every deck ever run:
  the default mode renders no pixels, so "lacks complete all-slide vision
  inspection" was always in `blocking_reasons` — including for decks that pass
  every deterministic check. A flag that is constant false decides nothing.
  - **`deterministic_ready`** (new) is everything the engine decides on its own:
    the gate passed on complete evidence, the artifact is structurally valid,
    and the content is author-supplied. It is the flag the default path can
    reach.
  - **`deterministic_blocking_reasons`** (new) is `blocking_reasons` minus the
    visual-review entry: what is still fixable by editing the deck.
  - `publishable` is unchanged in meaning (`deterministic_ready` AND a visual
    verdict) but now says so, and its blocking reason names the two calls that
    supply the verdict — `render_deck_thumbnails` then `submit_visual_review`.
  - Gate "does this deck still need editing?" on `deterministic_ready`, and
    "can it ship unseen?" on `publishable`.

- **A chart whose x-axis labels were ellipsized drew every bar in the same slot
  (go-slide-creator-4xsi).** `AdaptXLabels` ellipsized long category labels **in
  place**, and the caller built its categorical scale from the result — while
  the series still looked each position up by the ORIGINAL label. Every lookup
  missed, `CategoricalScale.Scale` returned its range minimum, and all 14 bars
  of a P&L bridge stacked in the first 45 points of a 900-point canvas, with
  their value labels piled on the y-axis and the footnote pushed off the
  canvas.
  - `XLabelLayout.Categories` is now the identity a scale is keyed by and stays
    untouched; the ellipsized text goes to `DisplayLabels`, which the axis was
    already using to print. The label band is measured from the display text,
    so an ellipsized chart still gets the smaller bottom margin it earned.
  - A non-zero waterfall step now draws at least 2pt tall: a -0.8 on a
    3,000-unit axis was a fraction of a pixel, so a real step was invisible.
  - Both are pinned by tests that fail against the old code.

- **Pattern schema maxima are measured and pinned; card-grid says what its
  shape actually holds (go-slide-creator-0g6p).** A per-field `maxLength` can
  state only one number, so for a pattern whose cell count varies it is
  necessarily the budget of the smallest grid.
  - `cmd/json2pptx.TestSchemaMaximaStayReadable` builds, for every registered
    pattern, the largest payload its own schema permits, and measures the
    smallest size any cell would render at — the same prediction the fit report
    gives an agent — on all four bundled templates. The result is pinned per
    pattern and the gate fails both when a number gets worse and when it
    improves without the pin following.
  - The measurement: 33 of 43 patterns permit schema-legal content that renders
    below the 12pt floor, down to 2.6pt (card-grid) and 3.8pt (bmc-canvas). Ten
    are clean at their maxima.
  - **card-grid** now emits `BODY_TOO_LONG` from `PostExpandWarnings` naming the
    budget for the grid's own shape — "a 4x3 grid holds about 60 per card" —
    where `TEXT_BELOW_READABLE_MIN` says only that the text will shrink. The
    measured table (1x1–2x2 → 300, 3x2 → 220, 4x2 → 160, 3x3 → 100, 4x3 → 60,
    5x3 → 40, denser → 20) is in the `body` field's own schema description.
  - No `maxLength` was lowered: a 300-character body is exactly right in a 2x1,
    so shrinking the cap to fit a 5x5 would break readable decks to warn about
    unreadable ones.

- **`process` stops flattening a step's description into its label
  (go-slide-creator-61up).** `{label, description}` was concatenated into
  `"Approve — Board approves tranche 1"` and fed to `process-flow`, whose boxes
  centre that one string at ~9pt reversed out of solid accent: four identical
  blocks in a thin band, no step numbers, and the label and the detail
  indistinguishable.
  - Steps that carry a **description** now compile to **numbered-step-strip** —
    numbered rows, a bold label over its own detail line — in `stacked-box`
    style for 3–4 steps and `chevron` for 5–6.
  - **Bare labels and branching steps** (`type: "decision"`) keep
    `process-flow`: the diamonds are the reason that pattern exists. A
    description on a branching step is still appended to the label, because a
    flow box has nowhere else to put one, and losing it would be worse.
  - Outside both (fewer than 3 steps, more than 8, or a label past its budget)
    it degrades to bullets and `SEMANTIC_DENSITY` names the bound.

- **`decision` compiles to a visual (go-slide-creator-4ndv).** The ask is the
  slide a board deck exists for, and `kind: decision` compiled to a bold
  paragraph over a column of dashes — the plainest page in the deck.
  - 3–6 options now render as **numbered-step-strip** (stacked-box): numbered
    boxes, each a label with an optional detail. Exactly 2 options, each WITH a
    detail, render as two **card-grid** cards side by side.
  - The `recommendation` moves into the pattern's **callout band** beneath the
    options, which is where the ask belongs; the slide's own `takeaway` is
    untouched and still sits below it.
  - An option is `"Label"`, `"Label | detail"` (or an em/en dash) or
    `{label, detail?}`; `choices` and `alternatives` alias `options`.
  - One option, seven options, a pair missing a detail, or text past the
    budgets (label ≤60 chars, detail ≤180 in the strip; ≤80 / ≤300 in the
    cards) keeps the content slide, and `SEMANTIC_DENSITY` says which bound
    broke — "has one option; a decision slide needs at least two to be a
    choice".
  - `examples/semantic/qbr.yaml`'s decision slide is rewritten as
    label + detail, which is what the visual wants and better authoring anyway.

- **Which patterns DeckSpec can reach is now published and pinned
  (go-slide-creator-4fr1).** DeckSpec is the recommended path but compiles to a
  subset of the pattern registry, and nothing said which subset — three
  reviewers independently mis-modelled slides because "unknown slide kind" was
  the only signal and it arrived after the spec was written.
  - `internal/semantic/reach.go` maps every registered pattern to the kind that
    compiles to it, or to nothing. SKILL.md gains a **"Patterns DeckSpec cannot
    reach"** table listing the 21 that need `raw_json2pptx`, and a test diffs
    the table, its counts, and `get_started`'s against the registry — so a new
    pattern cannot ship without someone saying whether a spec author can reach
    it.
  - `list_slide_kinds` now returns `compositions` for every kind, not just some:
    `executive_summary`, `decision`, `title`, `section` and `closing` returned
    an empty list because they had no entry.
  - Four kinds advertised a **cross-kind** composition (`table-highlight` on a
    `table`, `kpi-3up` on a `stat`, `phase-roadmap` on a `timeline`,
    `pull-quote` on an `image_case`). `validateCompositionOverride` reads that
    list, so asking for one validated clean and then silently did nothing —
    exactly the failure the override reporting exists to prevent. They are
    removed; the advice moved into each kind's summary.
  - The `raw_json2pptx` example now shows a **pattern slide** (a `pull-quote`,
    one of the unreachable 21) rather than the bullets every other kind already
    produces: the escape hatch's reason to exist is a pattern no kind compiles
    to.

- **DeckSpec kind `image_case` (go-slide-creator-q31s).** A customer story or a
  product screenshot beside the words about it — the case study slide every
  proposal ends with — had no kind, so an agent that wanted one dropped to
  `raw_json2pptx`.
  - `{body|bullets}` plus an optional `image` compiles onto
    **image-text-split**, with `eyebrow`, `heading`, up to 5 `bullets` and up to
    3 result `metrics`. `image` is a path or url string or `{path|url, alt}`
    (aliases `photo`, `screenshot`); omitted, the pattern draws its dashed
    placeholder. `image_side` (`left` / `right`) is passed as a pattern
    override.
  - A metric needs both its figure and its words: a `{value}` with no `label`,
    or the reverse, is half a claim and is dropped rather than rendered blank.
  - A picture with nothing said about it is a plain image slide, not a case
    study; the kind refuses it, as the pattern does.
  - Past the column's budgets (body ≤300 chars, eyebrow ≤30, heading ≤80,
    bullets ≤5 × ≤140, metrics ≤3) it degrades to a content slide carrying the
    eyebrow, heading and story as text and the bullets, metrics and an
    `Image: <caption>` line as bullets — the picture cannot come with it, so the
    caption stands in for it rather than the slide pretending there was never
    an image.

- **DeckSpec kind `framework` (go-slide-creator-anzx).** SWOT, Porter's five
  forces and the Business Model Canvas are named things with fixed parts. What
  goes in them is not a design choice, and an author writing one should not have
  to know whether the engine draws it as a diagram or as a shape grid.
  - `{framework, sections}` takes all three. `framework` names which (`swot`,
    `porters_five_forces`, `bmc`, with the spellings an author reaches for —
    `five_forces`, `business model canvas`, `SWOT` — resolving to them);
    `sections` keys the framework's own parts, each a list of short items.
  - `bmc` compiles to the **bmc-canvas** pattern, supplying each cell's
    canonical heading so the canvas reads as the canvas. `swot` and
    `porters_five_forces` compile to the **native OOXML diagram** of that name —
    the first DeckSpec kind to reach a diagram rather than a pattern — on a
    `diagram` slide, because `blank-title` has no body placeholder for one and
    the content would be dropped.
  - A five-forces payload may state a force's `intensity` (0.0–1.0, or
    `"high"` / `"medium"` / `"low"`). One the author did NOT state is left to
    the renderer rather than invented here.
  - **Every part is required.** A SWOT missing its threats is not a SWOT, so an
    incomplete framework degrades to bullets grouped under each part's heading
    rather than rendering an empty quadrant, and `SEMANTIC_DENSITY` names the
    parts that are missing. A framework this kind does not draw (a PESTEL, say)
    keeps every section the payload carries, under headings made from its own
    keys.

- **DeckSpec kind `matrix_2x2` (go-slide-creator-ykjh).** Two axes and four
  quadrants — impact against effort, reach against cost — is one of the handful
  of shapes a strategy deck always reaches for, and the spec had no kind for it.
  - `{x_axis, y_axis, quadrants: [...]}` compiles onto **matrix-2x2**. The
    quadrants list is read **clockwise from the top left**, which is how a 2x2
    is read; `cells` / `boxes` alias it, and the named positions
    (`top_left`, `top_right`, `bottom_left`, `bottom_right`) work instead of a
    list. A quadrant is `"Header"`, `"Header: body"`, or `{header, body?}`.
  - Axis labels accept the pattern's own `x_axis_label` / `y_axis_label` as
    aliases; `x_low` / `x_high` / `y_low` / `y_high` label the axis ends.
  - Short of four headed quadrants and two named axes, or past a budget
    (header ≤80 chars, body ≤200, axis ≤60, axis end ≤20), it degrades to a
    bullet list that names each quadrant's position and both axes with their
    ends — a quadrant stripped of where it sits on the axes has lost the point
    of the slide — and `SEMANTIC_DENSITY` names what broke.

- **DeckSpec kind `timeline` (go-slide-creator-wrsb).** Dated milestones on a
  line are not a phased roadmap, and the spec only had the latter: an author
  with five dates and no workstreams had to bend them into phases or drop to
  `raw_json2pptx`.
  - `{milestones: [...]}` compiles onto **timeline-horizontal**: 3–7 stops,
    each a label with an optional date and a line of detail. `stops` /
    `events` / `timeline` alias `milestones`, and a milestone is a string (a
    bare label) or `{label, date?, end_date?, body?}` with the usual spellings
    accepted (`when` / `start` for the date, `until` / `end` for its end).
  - A milestone carrying an `end_date` spans a period rather than marking a
    point, so the slide compiles with `style: gantt` — the pattern rejects an
    end date in any other style, and range bars are what the author asked for.
  - Outside 3–7 stops, or past a stop's text budgets (label ≤60 chars, date
    ≤30, body ≤200), it degrades to a dated bullet list (`Mar 2024 — Label:
    body`, a range rendered `Jan 2025–Mar 2025`) and `SEMANTIC_DENSITY` names
    what broke.
  - `roadmap` is unchanged and still compiles to `phase-roadmap`.

- **DeckSpec kind `stat` (go-slide-creator-2hkc).** One number made the whole
  slide — the oldest move in a pitch deck — and the spec had no kind for it, so
  an agent dropped to `raw_json2pptx` or wrote the number into a title.
  - `{value, label}` compiles onto **stat-hero**, with `unit`, `context` and
    `source` around it. `stat` / `number` / `metric` alias `value`,
    `caption` / `subtitle` alias `label`, `suffix` aliases `unit`, and
    `detail` / `description` alias `context`.
  - `label` defaults to the slide `title`: an author who wrote only a title
    meant it as the words beneath the number, and a bare number with no words
    is not a slide.
  - `source` is left to the slide's attribution band rather than repeated as a
    bullet on the degrade path — the double-print go-slide-creator-xg48 fixed
    for `chart_insight`.
  - Past the hero's text budgets (value ≤20 chars, unit ≤10, label ≤80,
    context ≤120, source ≤80) it degrades to a content slide carrying every
    word, and `SEMANTIC_DENSITY` names which budget broke and by how much.
  - A `stat` slide plans as the big-number family, so three of them in a row —
    or one beside a `kpi_snapshot` — reads as monotony to
    `analyze_deck_rhythm`, which is what it is.
  - `metric_hero` and `big_number` are not accepted spellings, but
    `SEMANTIC_UNKNOWN_KIND` now names `stat` for them instead of handing back
    the full kind list.

- **DeckSpec kind `team` (go-slide-creator-13lj).** "Our people" is a fixture
  of every proposal and engagement deck, and the spec had no kind for it.
  - `{members: [...]}` compiles onto **team-bios**: 1–8 cards with a name, a
    role, an optional bio and an initials badge. `people` / `team` alias
    `members`, and a member is a string (a bare name) or
    `{name, role, bio?, photo_label?}` with the usual spellings accepted.
  - Every card needs a role — a card with a blank line where the title belongs
    reads as missing data — so a roster without one degrades rather than
    rendering half-empty cards.
  - Past 8 people, or past a card's text budgets, it degrades to a bullet list
    and `SEMANTIC_DENSITY` names which budget broke and by how much.
  - A `photo_label` the badge cannot hold is dropped rather than truncated:
    half a set of initials means nothing.

- **DeckSpec kind `agenda` (go-slide-creator-3rvk).** Every deck opens with a
  contents page and the spec had no kind for it, so an agent on the recommended
  path dropped to `raw_json2pptx` or shipped a bullet list.
  - `{sections: [...], current?}` compiles onto the existing agenda patterns.
    Sections carrying a `subtitle` become **agenda-with-images** rows (3–6);
    plain sections become the numbered **agenda** list (2–10). The choice
    follows the content, because whether subtitles were written is the only
    thing that distinguishes the two visuals.
  - `current` marks the section the deck is at — a 1-based position or the
    section's own title. It highlights the row in the numbered list, and is
    marked in the row's subtitle in agenda-with-images (which has no highlight
    override), so the signal is never silently dropped.
  - `items` / `agenda` alias `sections`; `current_section` / `highlight` /
    `active` alias `current`. A section is a string or `{title, subtitle?}`,
    with the usual spellings of the title accepted.
  - Outside those counts it degrades to a numbered bullet list — with the
    current section still marked — and says so as `SEMANTIC_DENSITY`.

### Changed

- **A chart_insight's takeaway is dropped when it repeats the slide's only
  insight, and a sparse insights column gives the chart room
  (go-slide-creator-pyxn).** A lone insight promoted into the takeaway printed
  the same sentence twice on one slide — once as the only Key Insight bullet
  and once verbatim in the band. Meanwhile the insights column took 35% of the
  width for one short bullet while the chart was squeezed into 55%.
  - The bullet is the slide's own content, so the band is what gives way. A
    takeaway that summarises SEVERAL insights is kept: it says something the
    bullets do not.
  - `chart-insights-split`'s default split widens from 65/35 to **75/25** when
    the insights column is sparse — at most two bullets, ≤140 characters in
    total, and no headline or so-what. `overrides.chart_width_pct` still pins
    the ratio and beats both defaults.

### Added

- **`SEMANTIC_PATTERN_NOT_AVAILABLE` (go-slide-creator-u5az).** A slide's
  `pattern` / `layout` override was a silent no-op for anything outside its
  kind's (undisclosed, 1–4 entry) alternatives list:
  `{kind: "comparison", pattern: "table-highlight"}` compiled to
  `comparison-2col` with `ok: true` and no finding, so an agent acting on
  `analyze_deck_rhythm`'s "break this run" advice could not tell its variation
  had been dropped.
  - The override is still ignored — the compiler keeps its own choice — but it
    is now reported at `slides[i].pattern` / `.layout` with
    `fix.params.allowed` carrying the list, as a warning under `warn` and an
    error under `strict`.
  - **`list_slide_kinds` publishes `compositions[]`** per kind
    (`{pattern, layout, reason}`), so the allowed set is discoverable before
    authoring rather than only from a rejection.

### Fixed

- **Constrained design mode is enforced on the DeckSpec path too
  (go-slide-creator-rs4h).** The same raw slide — a `shape_grid` with
  `shape.text.size` and hex fills — was refused by `generate_presentation`
  with blocking `design_mode_violation` findings and accepted in silence by
  `render_deck_spec` when wrapped in `kind: raw_json2pptx`. A compiled deck is
  stamped `design_mode: "constrained"`, but the check that enforces it ran only
  on the raw path, so the guarantee was not a guarantee on the recommended one
  — and an agent learned opposite rules depending on which door it came
  through.
  - `validate_deck_spec`, `compile_deck_spec`, `render_deck_spec` and the three
    `semantic` CLI subcommands now run the same validation over the compiled
    deck. Measured on the reported slide: 4 violations from the raw path, 0
    from the spec path; now 4 from both.
  - **`meta.design_mode`** accepts `"constrained"` (default) or `"free"`.
    There was no opt-out on this path at all, so a spec using the escape hatch
    deliberately had no way to say so. Only the escape hatch needs it: the
    compiler's own output never hand-sets what the template owns, and a test
    pins that.

- **Capability metadata and data-format hints agree, and say true things
  (go-slide-creator-umji).** Two agent-facing surfaces describe the same
  payload — `get_capabilities`' diagram entries and `get_data_format_hints` —
  and they were written independently, so they disagreed. An agent building a
  payload from the wrong one got a diagram with no data in it.
  - **`timeline`** said `required_fields: ["values"]`; the renderer reads
    `events` (alias `activities`) and has never read `values`.
  - **`heatmap`** required `values`, `row_labels` and `col_labels`; only the
    grid is required — an unlabelled heatmap renders.
  - **`nine_box_talent`** required `employees`; either `employees` or `cells`
    renders, so neither is required alone.
  - **`stat_cards`, `icon_columns`, `icon_rows`** are advertised as diagram
    types but had no hints entry at all, so their payload had to be guessed
    from `optional_fields`. They have one now, naming the panel_layout mode
    each resolves to.
  - **`timeline`'s overflow text** promised "error if <3 or >7 stops"; it lays
    out every stop it is given. **`kpi_dashboard`'s** now says its `max_nodes`
    IS enforced (metrics past it are dropped with `CONTENT_DROPPED`), which is
    what the engine does.
  - **porters_five_forces** accepted an intensity of 1.5 or -0.2 and printed
    "High (150%)" / "Low (-20%)" from a field its own hint documents as
    0.0-1.0. It is clamped to that range.
  - A test walks every ready chart and diagram type and fails when a hints
    entry is missing or its required keys disagree with the capability's — the
    two tables cannot drift apart again without CI saying so.

- **The CLI accepts a structure-only deck (go-slide-creator-m1kg).** A deck
  using the documented top-level `structure` block (mutually exclusive with
  `slides`) validated clean and rendered over MCP, but
  `json2pptx generate -json deck.json` failed with "at least one slide is
  required": the slide-count guard ran at parse time while the structure
  expansion ran later. Any agent or CI script driving the CLI — the documented
  counterpart of `generate_presentation` — could not use sections at all.
  - The guard now accepts a deck that carries either `slides` or `structure`,
    and the expansion checks its own result: a structure that produces no
    slides is its own error rather than passing silently.

- **The autofit prediction runs on the layout the engine will actually pick,
  and reports readability alongside trimming (go-slide-creator-nlrg, partial).**
  `collectTextAutofitPreflightFindings` keyed on an explicit `layout_id`, so a
  deck that lets the engine choose its layout — most decks, and every deck the
  semantic compiler emits — got no `text_trimmed`, no `readability_trimmed` and
  no readability verdict at all from `validate`. It now uses the same predicted
  layout the measured-title check uses.
  - `TEXT_BELOW_READABLE_MIN` can now come from `validate`: the prediction
    carries the deck's `viewing_mode` and the text's role, and is judged by the
    same `NewReadabilityFinding` the renderer calls.
  - A trim prediction no longer masks the readability verdict. The renderer
    trims and THEN judges the size it ended up with, so both can be true of one
    placeholder, and `generate` reported both where `validate` reported at most
    one.
  - **Still not at parity**, and the bead stays open for it: the preflight
    measures plain paragraphs, while the renderer also applies the layout's
    inherited line spacing, space-before and bullet left margins. On the
    reported fixture that is one autofit step (60% predicted against 50%
    rendered), which is the difference between "at the floor" and "under it".

- **exec-summary rows share a first baseline (go-slide-creator-kol0).** The
  lead and its support were both centre-anchored, so whenever a lead wrapped to
  two lines the support floated between them; across five rows the right column
  visibly stair-stepped, where a consulting exec summary aligns the first
  baseline of every pair.
  - All three cells of a row are top-anchored now, and the smaller text carries
    a top inset equal to the difference in ascent, so "top-anchored" means
    "same baseline" rather than "same box edge".
  - Measured on a wrapping lead at 110dpi: the support's first ink was 17px
    below the lead's and is now 2px (the cap-height difference between 17pt
    bold and 14pt regular); the number went from 14px below to level.

- **The no-emoji policy permits the monochrome symbols a deck needs
  (go-slide-creator-l38d).** `IsEmoji` treated U+2600–27BF wholesale as emoji,
  so a feature-comparison table written with `✓` and `✗` — the most common
  encoding for a competitor matrix — was refused with one blocking error per
  cell. The remediation the message offered (a bundled SVG icon) cannot go in
  a table cell, which takes only strings, so the only way through was `√` /
  `×`, which reads as a typo. `Acme®` and `Windows™` were refused too.
  - Permitted now: `✓ ✔ ✗ ✘ ★ ☆ ☐ ☑ ☒ © ® ™`
    (`emoji.TypographicSymbols` / `emoji.AllowedSymbols()`). They are
    monochrome glyphs in ordinary text fonts, drawn in the run's own colour.
  - Everything pictographic stays blocked, including the emoji-presentation
    lookalikes `✅ ❌ ⭐`, and a variation selector after a permitted symbol
    (`✓` + U+FE0F) is still a violation because it asks for the colour glyph.
  - The policy message now names the permitted set, so an agent inside a table
    cell is told what it CAN write.

- **heatmap cell values are readable on the dark tiles
  (go-slide-creator-vdvs).** Every value was printed in `dk1`, so "92" on the
  saturated end of a sequential scale was near-black on dark blue or dark
  green. Measured worst case per template, dk1 on the cell it sits on:
  warm-coral 2.25, midnight-blue 2.67, modern-template 4.04, forest-green
  4.10 — all under the 4.5:1 bar the engine enforces everywhere else. Diagram
  text never passed through the contrast pass, which only sees placeholder and
  shape-grid text.
  - Each cell's value is now `lt1` or `dk1`, whichever reads better against
    the colour that cell's tint actually produces (scheme colour + lumMod /
    lumOff, resolved through the template theme). Every cell of the reported
    fixture clears 4.5:1 on all four bundled templates.
  - It is per cell, not a blanket flip: warm-coral's saturated orange keeps
    dark text, because black reads better on it than white does.
  - Without a theme to measure against, the historical `dk1` stands.

- **`audit_palette` works on a machine that installs LibreOffice as `soffice`
  (go-slide-creator-rdql).** The tool demanded a binary literally named
  `libreoffice` plus `pdftoppm`, so on macOS/Homebrew — where the CLI is
  `soffice` and the render stack uses ImageMagick — it returned
  `required binary "libreoffice" not found on PATH` while every other render
  tool worked. The whole palette-drift capability was dead on the most common
  dev platform.
  - It now renders through `internal/render` (`DeckPNGs` / `DependencyStatus`),
    which resolves `libreoffice` **or** `soffice`, rasterises with ImageMagick,
    and brings the timeouts, private LibreOffice profile and render cache with
    it. `--keep` / `--tmp` still hand back PNGs that outlive the render.
  - `preview_patterns` and `internal/layoutpreview` kept their own copies of
    the same resolution; they now call `render.OfficeCommand()` /
    `render.ImageMagickCommand()`, so there is one place that knows
    libreoffice/soffice and magick/convert are the same tool under two names.
  - A test walks the repo and fails on any non-render package that shells out
    to one of those binaries by name.
  - **Note the swapped dependency:** the audit rasterises with ImageMagick now,
    not poppler's `pdftoppm`, because that is what the rest of the engine uses.
    ImageMagick is resolved as `magick` (v7) **or** `convert` (v6) — by the
    rasteriser and by `DependencyStatus` alike, so a box cannot be told
    rendering is available and then fail on the binary name.

- **A content-sized table is centred in its placeholder
  (go-slide-creator-6jv7).** Table rows are sized for their text rather than
  stretched to fill the placeholder, so a 5-row table on a One Content layout
  rendered in the top half and left the bottom ~45% of the slide blank. Nothing
  reported it: the placeholder itself counts as full, so `SLIDE_UNDERUSED`
  never fired, and the deck scored 87 with the quality gate passing.
  - The graphic frame now sits at the vertical centre of its bounds when the
    table is shorter than them; the whitespace is shared above and below
    instead of pooling at the bottom. A table that fills (or overflows) its
    bounds does not move, and neither does one rendered without bounds.
  - The same applies to a table in a `shape_grid` cell, which is its own box.

- **Unknown keys inside a pattern cell are reported (go-slide-creator-4cqh).**
  `PATTERN_UNKNOWN_FIELD` caught a bad key at the top of `values` /
  `overrides`, but not one inside a cell: `{"big": "$4.2M", "small": "ARR",
  "bogus": "z"}` on kpi-3up validated clean. Every pattern whose values are a
  list of cells declares the element as `oneOf{shorthand string, object}`, and
  the inspector asked the `oneOf` — which names no properties — instead of its
  object branch, so it concluded nothing there could be unknown.
  - The inspector now resolves a `oneOf` to the branch matching the node's JSON
    kind, so `values[0].bogus` and `values.cells[2].bodySize` are reported with
    the same `did_you_mean` / `allowed` remediation as any other dropped key.
  - **`json2pptx patterns validate` (MCP `validate_pattern`) runs the inspector
    too** and exits non-zero on a dropped key. It previously printed "valid"
    for a payload whose field was being discarded — the exact case an agent
    hits when it misspells an override and silently gets template defaults.
  - Tolerated aliases and shorthands are unaffected; no example deck gains a
    finding.

- **value-chain picks its highlight by measured contrast
  (go-slide-creator-ah5s).** The pattern painted its steps `dk2` and the
  highlighted step `accent2`. Those two slots are 3.21:1 apart on
  midnight-blue and **1.48:1** on warm-coral, where the highlighted step was
  indistinguishable from its neighbours — the slide's one semantic signal,
  gone, on a template the pattern was never tuned on.
  - The default highlight is now the first of `accent1` … `accent6`, `lt2`
    whose effective colour clears **3:1** against the step fill for this
    template. Measured: warm-coral `accent1` 3.53 (was `accent2` 1.48),
    modern-template `accent5` 3.12 (was `accent2` 2.73), midnight-blue
    `accent2` 3.21 and forest-green `accent2` 5.28 (both unchanged).
  - An authored `highlight_color` is still honoured, and reported as
    **`LOW_CONTRAST_HIGHLIGHT`** (`review`) when it measures below the bar.
  - Without a theme to measure against, the historical `accent2` default
    stands, so an expansion with no template is unchanged.

- **pull-quote separates the quote from its attribution
  (go-slide-creator-36ny).** The two were consecutive paragraphs in ONE cell
  with no spacing, which cost three things at once: the attribution's baseline
  sat directly under the quote's descenders (on a long quote the two lines' ink
  crossed), the cell's autofit shrank the attribution along with the quote —
  to ~7pt on a 500-character quote — and the accent rule ran the full height of
  a box whose bottom third was empty.
  - The quote and the attribution are now two rows of the pattern's grid:
    separate text bodies, so the quote's autofit no longer touches the
    attribution, with a ~0.6em gap under the quote (the quote cell's bottom
    inset).
  - The attribution never renders below **11pt**, whatever `attr_size` says and
    however long the quote is.
  - Both rows are content-sized, so the accent rule stops where the text does
    instead of stretching past it.
  - The rule is a column spanning both rows rather than a per-cell accent bar,
    so the row gap cannot break it in half. `accent_side: "none"` still draws
    no rule, and `"right"` puts the column on the right.

- **`panel_layout` `layout: "stat_cards"` no longer inverts the card
  (go-slide-creator-3j88).** With `panels: [{title: "ARR", body: "EUR 184m"}]`
  — the shape `get_data_format_hints` documents — the card drew "ARR" at 32pt
  and the money as the small caption, the exact inverse of the sibling
  `stat_cards` diagram type fed `{title, value}`.
  - The hero is now the statistic: an explicit `value` wins, otherwise a short
    `body` carrying a digit (`"EUR 184m"`, ≤24 characters) is the 32pt number
    with `title` as its caption. A prose body keeps its place under the title,
    and a short body with no digit (`{title: "Phase 1", body: "Discovery"}`) is
    a label pair, not a statistic, so it is unchanged.
  - The two paths now render identical cards for the same content.
  - `get_data_format_hints`' `panel_layout` description states the rule and
    that `layout` belongs inside `data`.

- **Table conditional formatting accepts a text threshold, and the rule is
  actually evaluated (go-slide-creator-6hlu).** `conditional.threshold` was a
  `float64`, so the natural RAG rule
  `{"rule":"equals","threshold":"On track","fill":"accent3"}` aborted the WHOLE
  deck at parse time — with a Go type error that named the wrong struct
  (`TableCellInput`), pointed at no JSON path and offered no fix. The only
  working form was a per-cell `positive`/`negative` with a hand-picked fill.
  - `threshold` now takes a **number, a string, or a two-number array**
    (`between`). An unusable shape is a finding, never a parse abort.
  - The rule vocabulary is closed and documented:
    `always` (the default when `rule` is omitted — the plain highlight form),
    `positive`, `negative`, `threshold`/`gte`, `lte`, `between`, `equals`,
    `contains`.
  - **The rule is now evaluated against the cell's own content.** It used to
    only pick a default fill, so a cell tagged `negative` was tinted red
    whatever it said. A cell that does not satisfy its rule keeps the table's
    normal fill. Numbers are read out of the cell text (`"+4%"`, `"(3.2)"`,
    `"EUR 1,186.4"`), and a percent sign is a unit, not a scale.
  - An unknown rule, or a threshold the rule cannot use, is an
    `INVALID_PARAMETER` error at `…rows[r][c].conditional.rule` / `.threshold`
    carrying the allowed list, a `did_you_mean` for a typo, and a `fix`.

- **Pattern length budgets count characters, not bytes
  (go-slide-creator-5ok4).** Every `maxLength` in `internal/patterns` is written
  in characters and its error says "chars", but the checks measured `len(s)`.
  `"€186.4M"` is 7 characters and 9 bytes, so it failed kpi-2up's 8-character
  big-number budget as "9 chars" — an arithmetic contradiction an author could
  only escape by trial and error. Every euro sign, umlaut or en-dash cost 2–3
  units of the budget, which fired constantly on European content.
  - All 125 budget checks (guard and reported count alike) now use a rune
    count, in `internal/patterns` and in the semantic layer's own fit
    predicates (`option_matrix`, `comparison` panels/cards, `architecture`).
  - **A KPI snapshot no longer degrades silently.** A value past the budget
    still compiles to a bullet list — but validate, compile and render all
    report it (`SEMANTIC_DENSITY` at `slides[i].kpis`, naming the lost pattern,
    the field and the budget: `kpi-3up: values[0].big exceeds maxLength 8 (20
    chars)`), and the explain plan stops advertising `kpi-Nup` for a payload
    the compiler will not render as cards. `explain_deck_spec` and
    `compile_deck_spec` now agree on this kind, as they already did for
    `architecture`, `chart_insight`, `comparison` and `option_matrix`.
  - A count outside 2–6 likewise plans no pattern instead of clamping to
    `kpi-2up` / `kpi-6up`.

- **Contrast on an author-set background snaps to the palette instead of
  lerping to the WCAG floor (go-slide-creator-s7wmh).** A slide that darkens
  its own background (`background.color: dk1`, or an opaque scrim over a
  photo) keeps the template's title colour, which was chosen against the
  template's background, not this one. The contrast pass treated that colour's
  hue as intentional and lerped it toward white just far enough to clear the
  floor: midnight-blue's navy title on black came out `#4E5A72` at exactly
  3.0, and warm-coral's `#3E2723` came out `#685552` — legible by the
  measurement, barely visible on the slide.
  - Replacement now depends on the background's **provenance**. An
    author-introduced background has no hue intent to preserve, so the fix
    flips to the first template text colour that clears the bar (`lt1`, `dk2`,
    `dk1`, then a tonal shade of the background). Measured: midnight-blue
    `#1B2A4A → #FFFFFF` (1.5 → 21.0), warm-coral `#3E2723 → #FFFFFF`
    (1.5 → 21.0).
  - **Unchanged on a template background** — that pairing is the template's
    own and still lerps — and unchanged for `shape_grid` cells, where the
    author chose the fill and the text colour together.
  - `validate` now predicts this case as well: a `contrast_predicted` finding
    with `fix.params.source: "slide_background"` naming the same replacement
    generate emits. Previously no prediction existed for placeholder text on
    an author background — the deck validated clean and reported a swap only
    after rendering.

- **The full DeckSpec schema is embedded once, not four times
  (go-slide-creator-uhaq).** The ~17KB closed per-kind `oneOf` was the input
  schema of `spec` on all four spec tools: twice in the core `tools/list` and
  four times in the full one, saying the same thing each time. Every new slide
  kind was paid for twice on connect.
  - It now lives on **`validate_deck_spec`** alone — the tool the workflow
    already says to call before rendering, and the one where a
    schema-validating client can check a deck before anything is produced.
  - `render_deck_spec`, `compile_deck_spec` and `explain_deck_spec` declare the
    **outline**: `meta`, and `slides[]` objects whose `kind` is the registered
    enum. Their `spec` description points at `list_slide_kinds` (per-kind
    `item_schema` + copy-ready example) and at `validate_deck_spec`.
  - **What they accept is unchanged.** The per-kind contract is enforced by the
    compiler, not the input schema, so an unknown payload field is still
    reported as `SEMANTIC_UNKNOWN_FIELD` by every tool that validates.
  - Core `tools/list`: 99,172 → 85,131 bytes (-14%), budget lowered from 104KB
    to 92KB. `render_deck_spec`'s definition drops from 21,592 to 6,085 bytes;
    the full profile drops ~45KB.

- **One definition of done: `auto_repair`'s default gate IS the ship gate
  (go-slide-creator-ie9v).** `score_deck`'s gate is 80 / 0 P0 / 0 P1;
  `auto_repair`'s default was 75 / 0 / 2, and both tools return a field called
  `gate_passed`. A deck could converge in the repair loop at 79 and be refused
  by the ship check at 80 — and the `brief` and `revise` fast paths lead to the
  two different gates, so an agent that stopped at `auto_repair` shipped decks
  the project's own gate rejects.
  - `auto_repair` (and `make_deck`, which shares its gate) now default to
    `min_score: 80, max_p0_findings: 0, max_p1_findings: 0`. The looser numbers
    remain available as an explicit `gate` argument — and the tool description
    says that relaxing them makes `gate_passed` a loop stop signal rather than a
    shipping verdict.
  - The response now echoes the criteria it judged against as **`gate`**, the
    way `score_deck` has always echoed `quality_gate.criteria`, so a default
    verdict is distinguishable from a relaxed one.

- **The completion rule now says what to re-inspect after a repair
  (go-slide-creator-2018).** It read "After any repair, re-render and
  re-inspect", which alongside "render ALL slides" implied a full-deck pass per
  repair. It now reads: re-render and re-inspect the slides that changed
  (`render_deck_thumbnails` with `slide_indices`, or `render_slide_image` for
  one), then make one full-deck pass over the final revision — the revision you
  ship is the one that has to have been seen. The machine contract is unchanged:
  `submit_visual_review` still requires a complete, current-revision review.

### Fixed

- **`examine_template`'s content zone reached into the footer band
  (go-slide-creator-p41d6).** For every layout with a footer the reported
  `content_zone.bottom_emu` was 6,515,100 while the same response's
  `profile_geometry.frame.footer_top_emu` said 6,356,350 — 0.17in of overlap,
  on 21 layouts across three templates. The zone was derived from the layout's
  own placeholders; the footer is resolved from the layout ELSE the master, so a
  layout that inherits it reported no footer at all. An agent computing bounds
  from the zone wrote content under the footer text.
  - `content_zone.bottom_emu` is now clamped to `footer_top_emu` minus 0.1in
    whenever the layout has a footer, and left exactly as it was when it does
    not.
  - The render side uses the same line: the shape-grid content zone takes its
    footer from the layout's resolved `footer_regions` (highest chrome rect)
    rather than whichever utility placeholder came first in document order or a
    fixed 0.4in bottom margin. On a 7-row agenda the lowest grid shape moves
    from 0.024in BELOW the footer line to 0.125in above it.

- **Sibling grid cells flipped between three text colours
  (go-slide-creator-tnx3e).** An arch-stack whose tiers are progressive tints of
  one accent came out with white on the darkest tier, pure `#000000` on the
  next and `#373545` on the last two: the contrast pass decided one cell at a
  time, and each decision was defensible alone.
  - Cells that start from the same text colour AND whose fills are one visual
    family (pairwise contrast ≤ 3:1) are now decided **once**, against the worst
    fill in the group, and the colour is applied to every member — including the
    ones that would have passed, since a tier keeping white while its
    neighbours go dark is the inconsistency itself.
  - The candidate order prefers a tonal shade of the fill's own hue over the
    palette's near-black, so the reported stack lands on `#35283D` rather than
    `#000000`.
  - One finding per group instead of one per cell: `source:
    "shape_grid_group"`, `fix.params.cells`, a message reading
    `across N sibling cells`, and a path of `/slides/{i}/shape_grid`.
  - Cells whose fills are NOT alike (a dark card beside a pale one) keep their
    per-cell decisions: no single colour suits both, and forcing one would make
    the dark card worse to help the pale one.

- **One finding code arrived at two severities (go-slide-creator-7xyy).**
  `density_exceeded` came back from a single `validate_input` response as four
  warnings AND four infos — the same four table facts, emitted once by the
  validator and once by the fit report, at different severities — and the
  visual-QA checker kept its own action→severity map that read `review` as a
  warning while every envelope read it as info.
  - Severity now comes from one place: `diagnostics.SeverityForAction`, used by
    the envelopes and by `score_deck`'s per-slide findings. An emitter that
    states no action takes the one the code DECLARES in the finding registry
    (`describe_finding`'s `severity`), so a code cannot be demoted by the
    detector that happened to raise it.
  - Findings with the same `(code, path, message)` are collapsed to one entry,
    keeping the record that carries a remediation. On the reported deck the
    envelope goes from 10 findings to 6, with every fact stated once.
  - Score and gate are unaffected: they weight the finding's `action`, not the
    severity label.

- **A wrong-typed argument was reported as missing (go-slide-creator-6072).**
  A sweep of every core tool with a wrong-typed first argument found
  `describe_finding{code: 12345}` answering "code is required",
  `plan_deck{brief: 12345}` answering "brief is required", and four more. The
  typed accessors cannot tell absent from present-but-wrong-typed, and every
  handler turned the failure into MISSING_PARAMETER — telling an agent to
  re-add a field that is sitting in the call it just sent.
  - The shared `argRequired` helper now looks the argument up (dotted paths
    included) and emits **`INVALID_PARAMETER`** with
    `"<path> must be a string, got a number"`, `evidence.expected_type` and a
    `next_tool_call` retry when the key is present; `MISSING_PARAMETER` is left
    to mean what it says. All 62 call sites route through it.
  - **`get_started{task: <non-string>}`** was the one silent ignore left in the
    surface — it answered with the brief workflow as though nothing had been
    asked. It is now `INVALID_PARAMETER` naming the accepted tasks. An unknown
    task STRING still falls back to `brief`: that is a documented default.
  - **`list_patterns{fields: <non-string>}`** now carries the same
    `expected_type` / `example_value` / `next_tool_call` its identical sibling
    `list_templates` always did.
  - Pinned by a sweep test over the whole catalogue: for all 50 required
    arguments across 52 registered tools, a wrong-typed value is never reported
    as missing.

- **`chrome.page_numbers` wrapped `2 / 10` onto two lines
  (go-slide-creator-pss1z).** The slide-number box was widened to a flat 0.5in —
  "enough for three digits" — and the page-number text never went near the
  fitter that sizes the left footer, so on modern-template every numbered slide
  showed `2 /` over `10` in the corner.
  - The box is now measured from the widest string the format can produce (both
    `{current}` and `{total}` at the deck's highest number, since `{current}` is
    a field PowerPoint fills in at display time) with the same font metrics the
    left footer is fitted with, and grows leftward from its fixed right edge.
  - The growth is capped at 2in and stops short of the left footer's last inch;
    when the format still does not fit, the number's type shrinks on the
    footer's own ladder (10.5pt → 8pt) rather than wrapping. Both slide-number
    shapes now carry `wrap="none"` as a backstop.
  - The left footer is laid out against the widened box, so the two cannot
    overlap. A template whose slide-number box is already wide enough is
    untouched.

- **`slide_type: "closing"` now says where the value belongs
  (go-slide-creator-ejh5u).** `closing` appears in `canonical_layout_ids`, in
  `chrome.page_numbers.skip`, as a DeckSpec kind and in `get_started`'s notes,
  so authors write it as a `slide_type` — and got back `UNKNOWN_ENUM` listing
  nine values that do not contain the word, with no hint that the working
  spelling is `layout_id: "closing"` one field over. (One reviewer hard-coded
  per-template layout ids instead.)
  - The finding for a slide_type that is a canonical LAYOUT name now reads
    `"closing" is a canonical LAYOUT name, not a slide_type: move it to
    layout_id` and carries a **`rename_field`** fix (`from: slide_type`,
    `to: layout_id`) that `repair_slide` applies mechanically. The rule is
    general, not a `closing` special case: every canonical layout name gets it
    (`quote`, `agenda`, `image-left`, `blank-canvas`, …), while a genuine typo
    keeps the plain `use_one_of` error.
  - `get_input_schema` now describes `slide_type` as the auto-selection hint it
    is and says layout names belong in `layout_id`.
  - `repair_slide`'s `rename_field` left the OLD field set when renaming a
    slide-level key, because the renamed JSON was decoded onto the existing
    slide. A renamed `slide_type` therefore sat next to the `layout_id` it had
    just created and the finding came straight back; the slide is now replaced
    wholesale.

- **`expand_pattern` / `validate_pattern` declared `values` as an object, but
  nine patterns need an array (go-slide-creator-ldu3d).** `kpi-2up`…`kpi-6up`,
  `icon-row`, `stylish-panels` and `timeline-horizontal` take an ARRAY of cells
  in `values`; the tools' `inputSchema` said `{"type":"object"}`. A client that
  validates tool arguments against `inputSchema` could not call those patterns
  at all, and the server accepted them only because it does not enforce its own
  advertised schema.
  - `values` is now `{"oneOf":[{"type":"object"},{"type":"array"}]}` on both
    tools, and the description names the array patterns — the list is generated
    from the registry, so it cannot drift from the schemas the validator
    enforces.

- **Text that stated no colour was never contrast-checked
  (go-slide-creator-ucmgr).** The contrast pass rewrote scheme colours it found
  in the slide's own XML, so text carrying no colour at all was invisible to it:
  `examples/basic-deck.json` on `modern-template` put five white bullets on a
  white section divider and the deck reported nothing. The colour came from the
  master's `bodyStyle` (`schemeClr tx1`) through the layout's `clrMapOvr`
  (`tx1` -> `lt1`), two files the pass never opened.
  - The inherited colour is now resolved the way the renderer resolves it —
    the layout's matching placeholder `lstStyle`, then the master's `txStyles`,
    each mapped through the layout's colour-map override — and checked against
    the background the slide actually shows (a slide's own background included).
  - When it fails WCAG for that text's size, an explicit colour is written onto
    the slide's runs and a `contrast_autofixed` finding is reported at
    `/slides/{i}` with **`fix.params.source: "layout-lstStyle"` or
    `"master-txStyles"`** — two new values in that field's vocabulary.
  - A shape that states a colour anywhere is left to the existing pass, so one
    change is never reported as two swaps. Measured: `basic-deck` on
    `midnight-blue`, `warm-coral` and `forest-green` renders byte-identical;
    only `modern-template`, the template with the inverted layout, changes.

- **A log-scale bar chart labelled its bars with the logarithm
  (go-slide-creator-e2ck9).** The label site read the plotted Y coordinate,
  which on a log scale is log10(value), so a bar worth 490,000 was labelled
  "6". It now reads the raw value the chart already recorded for the purpose.

- **`validate --fit-report` runs the same collector as every other surface, and
  stops refusing diagrams that render (go-slide-creator-r87g).** A reviewer
  running the CLI fit report over a sweep of deliberately broken decks got "no
  issues found" on defects `score_deck` reported, and refuse-class findings on
  four of the shipped example decks for charts that render correctly.
  - **The CLI assembled its own subset of the collectors.** Every detector added
    since arrived as a separate "must show up in validate too" change, so the
    two surfaces drifted: the CLI was missing the substance, geometry,
    alt-text, content-lint and background collectors entirely. It now calls
    `collectFitFindings`, the same function `score_deck` and `validate_input`
    use.
  - **Natively-rendered diagrams are no longer dry-rendered through svggen.**
    Half the diagram catalogue (`swot`, `business_model_canvas`, `value_chain`,
    `heatmap`, the panel family, `pyramid`, `process_flow`, …) is drawn as
    OOXML shapes by the generator. Asking svggen about one got "unknown diagram
    type", reported as a REFUSE claiming the deck would show a grey "Data
    unavailable" placeholder. A type neither renderer owns still refuses.
  - **The chart dry run now normalizes the authored data first**, through the
    same `ToDiagramSpec` conversion the render path uses. It used to validate
    the shorthand the author wrote rather than the payload the renderer builds
    from it, so a waterfall authored as `{"Revenue": 100, "Costs": -40}` came
    back "requires 'points' array".
  - **`CHART_OVERLOADED` reads the structured data form.** It counted the data
    map's KEYS, so a 15-slice pie authored as `{categories: [...], values:
    [...]}` — the shape SKILL.md documents — counted as two categories and
    sailed through the legibility ceiling.

- **The chart styling surface is documented, closed, and no longer half inert
  (go-slide-creator-z72f).** `chart_value.style` was published in the input
  schema as a bare `{"type":"object"}` and appeared in no agent-facing surface,
  so the only way to find the data-labels switch was to guess — and guesses
  that followed the repo's own `docs/diagrams/*.md` failed.
  - **`style` and `chart_style` are now typed and closed** (`$defs/ChartStyle`,
    `$defs/ChartStyleOverrides`, `additionalProperties: false`), and
    `validate` walks both. `style.palette`, `style.show_grid` and
    `chart_value.subtitle` are reported as unknown fields instead of being
    dropped in silence — even under `--strict-unknown-keys` they were invisible
    before.
  - **`style.font_size` is removed.** It was never copied onto the style the
    renderer reads, so it did nothing; chart text sizes come from the
    template's type scale. Setting it is now an unknown-field finding rather
    than silence.
  - **`style.show_legend` means what it says.** It used to disable direct
    labels and nothing else, so on a single-series chart — where the legend is
    suppressed by default — it did nothing at all, and the working knob was the
    differently-named `chart_style.show_single_series_legend`. An explicit
    `show_legend` now forces the legend on every chart; the narrower
    `chart_style` override still wins when both are set.
  - **`get_data_format_hints` / `json2pptx data-format-hints` gained
    `chart_style_hints`**: every key both blocks accept, one line of semantics
    each, plus the rules that belong to neither key alone. `docs/diagrams/*.md`
    now says up front that it documents the svggen request envelope, not a
    deck's `chart_value`.

- **`SLIDE_UNDERUSED` stops penalising content-sized patterns, and its fix is
  actionable (go-slide-creator-up04).** It compared a pattern's ink bounding box
  with the whole content zone and demanded 45%, so the deliberately-shorter
  bands go-slide-creator-7km8 produced were flagged for being the height that
  change gave them: on three hand-crafted decks every firing was a false
  positive. The fix told the agent to "remove bounds / max_height_pct caps" it
  had never set — the height came from inside the pattern, so the advice could
  not be acted on.
  - The threshold now depends on who chose the band's height. **45%** when the
    slide caps it (`bounds` / `max_height_pct`) — an authoring decision, so
    loosening it is advice the agent can take. **22%** when the pattern derived
    the height from its content — it is supposed to be shorter than the zone, so
    only a genuine sliver (a four-stop `timeline-horizontal` at 10%) is
    reported.
  - `fix.params` gains **`band_capped_by`** (`author` / `pattern`) and
    **`threshold_pct`**, and the hint differs per case: raise the cap, or —
    when there is no cap — add detail, pair the block with a supporting zone
    using `compose`, or choose a denser pattern.
  - The near-empty placeholder slide this finding never covered is already
    `SLIDE_NEARLY_EMPTY`'s business; the two no longer need to overlap.

- **Inline SVG icons are no longer reported as unsupported text markup
  (go-slide-creator-6o1r).** `UNSUPPORTED_INLINE_MARKUP` scanned every string in
  a deck, including `icon.svg_data` — the documented way to supply an inline
  icon — and told the author its `<svg>` / `<path>` / `<circle>` tags "print
  literally on the slide". They do not: `svg_data` never reaches the run
  builder. The finding fired on every deck using an inline icon or a
  Harvey-ball `table-highlight`, and there was nothing the author could do about
  it. The scan now skips fields that carry markup by contract, at any nesting
  depth; markup in authored text is still reported.

- **`examine_template.aspect_ratio` is measured, not assumed
  (go-slide-creator-ydbk).** It defaulted to the literal `"16:9"` whenever a
  template carried no json2pptx metadata block — which is every bring-your-own
  template, the exact case `examine_template` exists for. A 10×7.5in corporate
  `.pptx` reported itself as widescreen to the agent about to size columns and
  text for it. The ratio now comes from `template.AspectRatio(width, height)`
  over the slide the template declares (the same function discovery adopted in
  go-slide-creator-r9nn); a metadata `aspect_ratio` still overrides it.

- **Pattern input failures are path-addressed, one finding per field, with no Go
  type names (go-slide-creator-20jm).** Every pattern failure used to collapse
  into a single `INPUT.INVALID_SLIDE` (generate) or `GRID.PATTERN_ERROR`
  (validate) whose message was whatever `encoding/json` or `Pattern.Validate`
  happened to say, at the path `/slides/{i}/pattern`: *"invalid values: json:
  cannot unmarshal string into Go struct field ProcessFlowValues.steps of type
  patterns.ProcessFlowStep"*. Several wrong fields arrived newline-joined inside
  one message, so an agent could fix one per round trip.
  - Each failure is now its own finding at its own path
    (`/slides/0/pattern/values/members/0/title`; a nested cell pattern keeps its
    coordinates, `/slides/0/shape_grid/rows/0/cells/1/pattern/values/...`), with
    a fix whose `params.path` is the same deck-absolute pointer and
    `next_tool_call: show_pattern{name}`.
  - **New finding code `PATTERN_UNKNOWN_FIELD`** (`refuse`, fix `rename_field`
    with `did_you_mean`, else `remove_field` with `allowed[]`): a key the pattern
    never reads, so its content never reaches the slide. Detection is empirical —
    the payload is decoded with the pattern's own decoder and a key is reported
    only when its content is demonstrably absent from the decoded value — so the
    tolerated aliases (a KPI cell's `{value, label}`, the `"$4.2M | ARR"`
    shorthand) stay silent and free-form objects (a chart's map-form `data`) are
    never judged. `did_you_mean` prefers a property the caller has not already
    filled: `title` → `role`, `label` → `name`, `columns` → `rows`,
    `date` → `date_label`.
  - **`invalid_shape`** is now also emitted by pattern input inspection and names
    the expected shape in schema terms with a copy-ready `fix.params.example`
    built from the caller's own value (`{"label":"A"}` for a bare `"A"`), or
    `fix.params.unwrap_key` when the payload wraps the array `values` is
    (`{"stops": [...]}`, `{"items": [...]}`).
  - **`UNKNOWN_PATTERN`** carries `did_you_mean` for the names agents actually
    write: pattern-name matching folds number words to digits and drops
    separators, so `kpi-four-up` → `kpi-4up` and `matrix2x2` → `matrix-2x2`
    resolve where they previously returned no suggestion at all.
  - A shape error no longer drags a `wrong_pattern` swap suggestion along with
    it: `{"items": [...]}` on `kpi-3up` used to decode as one empty cell and
    advise *"content shape (1 items) matches a different pattern; consider
    kpi-2up or stat-hero or team-bios"*. It now reports the wrapper key, once.

- **`get_capabilities` gained a `sections` projection and shed 93% of its
  default payload (go-slide-creator-5pta).** `get_started`'s first step is
  `get_capabilities`, and SKILL.md repeats it — so the most expensive call in
  the server was the first one every agent made. It returned **183,242 wire
  bytes (~46K tokens)**, of which `tool_list` alone was 55,475 B: a verbatim
  duplicate of what `tools/list` had already sent. The field an MCP agent
  actually needs (`runtime.render_available` / `output_dir`, 326 B) sat behind
  all of it, and the inputSchema was `{"type":"object","properties":{}}` —
  there was no way to ask for less.
  New **`sections`** argument (string array or comma-separated string):
  `runtime`, `features`, `deprecations`, `vocabularies`, `registry`, `tools`,
  `error_codes`, `cli`, `all`. The **default is
  `[runtime, features, deprecations]`**. `tools` is out of the default because
  `tools/list` is the authoritative catalogue, and `cli` because CLI-only
  commands are not callable over MCP. `schema_version`, `tool_version`,
  `changelog_url` and `runtime.schema_fingerprint` are in **every** projection,
  so drift detection works in the smallest one, and **`sections_included`**
  echoes what you got so an omitted section is distinguishable from an empty
  one.
  Measured (structuredContent only): `get_capabilities{}` **88,697 B →
  6,588 B**; `sections:["runtime"]` **416 B**; `sections:["tools"]` returns the
  full 74 KB catalogue; `sections:["all"]` restores the previous payload. The
  CLI `capabilities` subcommand is unprojected and unchanged.
  `get_started`'s three step hints now name the cheap projection.

- **Discovery tools default to the compact projection
  (go-slide-creator-dykl).** `list_templates{}` measured **153,266 B** on the
  wire and then *warned* that the caller should have asked for compact;
  `list_patterns{}` measured 69,692 B and did the same. `get_started` tells the
  agent to call `list_templates` with no arguments, and the error-path
  `next_tool_call` suggestions use `args_template: {}` — so the expensive
  default was exactly the one agents hit. The server paid first and advised
  afterwards.
  `fields` now defaults to `"compact"` on **`list_templates`, `list_patterns`
  and `list_icons`**, and the "fields parameter omitted" deprecation hint is
  gone — there is nothing left to deprecate. `fields="full"` restores the
  previous payload unchanged.
  **`supported_types` is omitted from the compact projection.** It is static
  per-server data (`diagram_capabilities` 5,561 B + `chart_capabilities`
  5,340 B + `shape_geometries` 2,694 B = 14,388 B) that was attached to every
  `list_templates` response regardless of projection, dwarfing the 1,930 B of
  actual per-template payload. `fields="full"` still carries it, and
  `get_chart_capabilities` / `get_diagram_capabilities` / `get_shape_catalog`
  serve it on demand.
  Measured on the wire (structuredContent only, after
  go-slide-creator-vxre): `list_templates{}` **53,987 B → 2,713 B**,
  `list_patterns{}` **28,137 B → 13,169 B**, both with no warning;
  `list_templates{fields:"full"}` unchanged at 107,752 B.

- **MCP responses stopped shipping every payload twice
  (go-slide-creator-vxre).** `MCPSuccessResult` put the whole object in
  `content[0].text` AND in `structuredContent`. One pass over the 18 callable
  core tools weighed **221,206 B of text + 168,072 B of structuredContent =
  389 KB** for 168 KB of information, **53,134 B of it pure two-space
  indentation**. The bloat scaled with the input too: a 1 MB title produced a
  2,003,105 B response because both copies echo it.
  Two changes. **Compact JSON is now the default** — the text block is a
  machine-read fallback, not a human document; `JSON2PPTX_MCP_PRETTY=1` indents
  it for reading raw transcripts by hand. And for a client that negotiated
  protocol **2025-06-18 or later** the duplicate text copy is **omitted**,
  because structuredContent is part of that protocol's tool-result contract.
  Older clients (2024-11-05, 2025-03-26) and sessions with no recorded
  initialize keep it — the safe direction.
  Measured on the wire: `list_slide_kinds` 61,606 B → **19,620 B**,
  `get_started` 16,728 B → **7,919 B**. On protocol 2024-11-05 the text copy is
  still present and still 49% smaller (38,195 B → 19,550 B) from compaction
  alone.
  New server flag **`--text-fallback=auto|always|never`** (env
  `JSON2PPTX_MCP_TEXT_FALLBACK`), default `auto`. Use `always` if your client
  reads `content[0].text` despite negotiating a modern protocol — the MCP spec
  says a tool with an outputSchema SHOULD include the text copy, and `auto`
  deviates from that SHOULD deliberately, on the grounds that a client
  negotiating 2025-06-18 can read structuredContent.
  The `experimental.compact_responses` capability and `MCP_COMPACT_RESPONSES=1`
  are now no-ops, kept so existing clients do not break; the drift-guarded
  sentence describing the handshake was updated in all four locations.

- **Chart data labels stopped contradicting their own data
  (go-slide-creator-66qb).** Every value label went through a hardcoded
  `"%.0f"`. A seven-bar series `[4.6, 4.9, 5.2, 5.5, 5.8, 6.2, 6.5]` printed
  **three distinct labels for seven bars**, denying its own axis and its own
  "+6% a year" headline; an ARR line `[61.2 … 86.4]` printed 61 64 65 66 71 75
  81 86 on a slide whose bullet read "from EUR 66.0M to EUR 86.4M". Only the
  waterfall auto-detected fractions.
  When the caller leaves `data_labels.format` unset, the renderer now picks the
  precision that keeps the labels **distinct** (up to two decimals) — the same
  rule for bar, line, area, funnel and waterfall. The two reported series label
  as 4.6/4.9/5.2/5.5/5.8/6.2/6.5 and 61.2/64.0/…/86.4. An explicit format is
  never overridden.
- **Thousands are grouped.** `Leads: 12400` reads `Leads: 12,400`, a bar reads
  `1,240`. Grouping finds the first digit run, so a format carrying its own
  units survives: `"€%.0fM"` on 12400 gives `€12,400M`.
- The `data_labels.format` description now says it carries the units too
  (`"€%.1fM"`, `"%.1f%%"`) and what the default does.
  **Not covered:** axis ticks still format independently of the labels, and
  there is no single `style.value_format` knob reaching both — split out as
  go-slide-creator-e2ck9.

- **`table-highlight`'s legend stopped labelling green dots "does not meet"
  (go-slide-creator-z0up).** `thLegendCell` emitted three swatches per scale but
  indexed the caller's `legend_labels` with the same `i` for both, so a deck
  mixing harvey and RAG columns showed six swatches carrying duplicated words —
  the green RAG dot wearing the Harvey "low" label. The RAG scale now has its
  own **`legend_labels_rag`** (same `[HIGH, MID, LOW]` order, i.e.
  `[green, amber, red]`, default `["Green", "Amber", "Red"]`).
- **Each scale gets its own legend row.** Six entries on one row gave each label
  a twelfth of the table and the text autofit down to ~2pt. One scale per row
  gives each label a third, and the reserved row height went from 32pt to 64pt
  — at 32pt the legend cell resolved to ~18pt after the table's proportional
  squeeze, leaving a 10pt nested row. The reported 6×6 deck now renders both
  legend rows at the authored 12pt.
- **The `[high, mid, low]` order is documented.** It lived only in a Go comment,
  so the gallery deck supplied `["Nicht erfüllt", "Teilweise erfüllt", "Voll
  erfüllt"]` — exactly backwards — and got a legend that said the opposite of
  its own chart. The schema descriptions for `legend_labels` and
  `legend_labels_rag`, `docs/PATTERNS.md` and `skills/generate-deck/PATTERNS.md`
  now spell it out.

- **Native diagrams store the autofit shrink, so the file renders the same in
  PowerPoint as in LibreOffice (go-slide-creator-wvr0).** Every native diagram
  (swot, business_model_canvas, value_chain, pestel, house, porters, nine_box,
  kpi, heatmap, panel_layout, process_flow, pyramid, stylish-panels, and every
  shape_grid cell) wrote `<a:normAutofit/>` with no `fontScale`. LibreOffice
  recomputes the shrink, so our own renders showed overflowing text merely
  small; PowerPoint applies the **stored** scale — 100% when the attribute is
  absent — and only recomputes when a human edits the shape, so the same box
  overflowed for the person who opened the deck. It also silently invalidated
  every visual-QA loop that renders through `soffice`: the pixels an agent
  inspected were not the pixels the client saw.
  `pptx.GenerateShape` now measures the body and writes the computed
  `fontScale` / `lnSpcReduction`. On the reported SWOT stress slide the two
  overflowing quadrants come out at 52% and 28%; the three that fit keep the
  bare element. Verified by pinning the text at the stored scale with
  `noAutofit` and re-rendering: everything sits inside its box, where the same
  test against the pre-fix prediction spilled four bullets out of two
  quadrants.
  The prediction lives in `internal/textfit.AutofitScale`, which
  `internal/textcapacity` now delegates to as well — one implementation, so the
  number the engine reports and the number it stores cannot drift. It counts
  paragraph `spcAft`, which the glyph measurement never saw: twelve bullets at
  6pt space-after carry 72pt of spacing, and ignoring it predicted 66% for a
  block that needed 28%.
- **An autofit shrink that leaves text unreadable is now reported.** When the
  stored scale takes a shape's smallest text under the `viewing_mode`
  readability floor, generation emits the existing `TEXT_BELOW_READABLE_MIN`
  (advisory, `action: review`) naming the effective size and the shrink —
  `body text renders at 3.4pt, below the 12pt minimum for viewing_mode
  "present" (autofit 28% to fit the shape)`. Across the 34 bundled examples
  this adds 9 advisory findings, each one true.

- **`reduce_text{max_items}` stopped silently deleting a fact-bearing bullet
  group (go-slide-creator-sx53).** The protected-fact guard covered pattern
  arrays and bullet lists, but the `bullet_groups` group truncation had none:
  `max_items: 3` on four groups returned `applied: true` and removed the only
  group carrying a number — the exact failure the guard exists to prevent, one
  content shape over. Dropping a group removes its `group_label`, `header`,
  `body` AND every bullet under it, so the guard now reads all of them, refuses
  with `code: "semantic_review_required"`, and still applies under
  `confirm_semantic_change: true`. Fact-free truncations are unaffected.

- **Titles are measured without an explicit `layout_id`
  (go-slide-creator-t64e).** The measured title check bailed out whenever a
  slide carried no `layout_id`, so the same 112-character title produced
  "title (112 chars) only fits its title placeholder at 60% of the template 45pt
  size; shorten to ≤ 71 chars" with `layout_id: "content"` and the legacy
  "title too long (112 chars, max 60)" with `slide_type: "content"` — scoring
  85 and 70 for the same deck. Worse, the semantic compilers emit `slide_type`
  and no `layout_id`, so on the DeckSpec path — the one `get_started`
  recommends — a 166-character title passed `validate_deck_spec` with no title
  finding at all, and `render_deck_spec` reported only the character heuristic
  inside `quality_summary`.
  `validate_input`, the fit report, the generate `quality` score,
  `validate_deck_spec` and `render_deck_spec` now measure against the layout the
  heuristic selector will actually choose, resolved in deck order with the same
  running context generation uses. The two spellings of the same slide produce
  identical findings and identical scores.
  Placeholder-existence validation deliberately still uses the author's own
  `layout_id`: the generator auto-maps placeholder IDs, so validating against a
  predicted layout would invent `placeholder_not_found` errors.

- **Deck monotony now has a consequence (go-slide-creator-xx9i).**
  `score_deck` computed the `composition` (rhythm) axis and then evaluated the
  quality gate purely on fit findings. Eight consecutive identical `kpi-3up`
  slides scored **100 and PASSED** with composition 55 and diagnostics
  `[pattern_run, missing_emphasis]`; seven identical bullet slides, composition
  50, same verdict. The information needed to catch the single most common LLM
  deck failure — everything looks the same — was already in the response and
  changed nothing.
  `QualityGateCriteria` gains **`min_composition_score`** (default **65**,
  between the graded monotony decks at 50-60 and every well-built deck at 100).
  The reason names the diagnostics that dropped it —
  `composition 55 < min_composition_score 65 (missing_emphasis, pattern_run)` —
  and sits after `accent_overload` in the documented reason order. `0` disables
  it, and it is skipped when composition was not measured (the `slide_indices`
  subset path).
- **Rhythm analysis stopped calling every hand-built grid the same layout.**
  A slide with a raw `shape_grid` and no named pattern was fingerprinted as the
  literal string `"shape_grid"`, so six structurally different grids in a row
  read as a six-slide `pattern_run`. Harmless while composition was advisory;
  with the gate above it would have blocked genuinely varied decks —
  `examples/sovereign-ai-strategy.json`, 25 slides of hand-built grids, scored
  composition 15 on three such phantom runs. Raw grids are now fingerprinted by
  their shape (`shape_grid:2-2` vs `shape_grid:3`), so identical layouts still
  count as a run and different ones do not. That deck now scores 100.
- **`score_deck` refuses a semantic DeckSpec instead of grading an empty deck.**
  A DeckSpec (slides carrying `kind`) unmarshals into a `PresentationInput`
  whose slides are all empty, and the tool graded that: `overall_score 99`,
  `quality_gate: passed` — for a deck it had never read, with a composition
  score describing its own misreading. It now returns `INVALID_PARAMETER`
  naming `validate_deck_spec` / `compile_deck_spec`.

- **`matrix_2x2`'s coordinate-free `quadrants` form renders as four lists, not a
  scatter plot (go-slide-creator-s27x).** data-format-hints promotes
  `quadrants: [{position, title, items}]` as what an agent without numbers
  should use. It invented an (x, y) for every item so the scatter renderer could
  draw it, and with two items per quadrant the labels already overprinted each
  other and the quadrant caption — "Data platform ●" over "Major projects" over
  "ERP upgrade ●". Each quadrant is now a heading followed by a bulleted list
  filling its own rectangle: no markers, the title is always the topmost line,
  and nothing in one quadrant can collide with anything in another.
- **Quadrant captions moved out of the point cluster.** In `points` mode the
  captions were centred in each quadrant — exactly where points land — so
  "ERP upgrade" printed straight through "Major projects". They now sit in each
  quadrant's outer top corner.
- **Points outside the axis range are clamped and reported.** `x: -10` / `x: 120`
  were plotted wherever the scale put them — outside the plot frame, over the
  axis titles, off the canvas — with nothing reported. They are clamped to the
  axis and reported as the new **`chart.point_out_of_range`** (warning, fix kind
  `replace_value`, params `{label, axis, value, clamped, axis_min, axis_max}`).
- **Rotated labels no longer steal the next element's text-anchor.**
  `fixSVGTextAlignment` pairs each emitted `<text>` with the alignment `DrawText`
  recorded, BY INDEX, and its regex matched only the positioned form
  (`<text x="…">`). Every rotated label — `<text transform="… rotate(-90)">`,
  which carries no `x` — was skipped while its recorded alignment stayed in the
  slice, so from the first rotated label onward every element took the PREVIOUS
  one's anchor and baseline. On a 2x2 matrix (which always draws a rotated
  y-axis title) that hung the first quadrant caption half outside its own
  quadrant, and on a scatter chart it gave the title the wrong baseline. Four
  golden SVGs move, each to its own correct attribute.

- **`org_chart` node labels no longer overprint each other
  (go-slide-creator-s5ur).** Every node drew its name and its job title at
  almost the same baseline: in the reported chart the name sat at y=103.23 in
  12.08px and the title at y=113.53 in 10.44px — 10.31px of separation for glyph
  boxes whose half-heights sum to 11.26 — so "Anna Becker" and "CEO" touched, on
  all four templates. The cause was `titleY = nameY + nameFontSize*0.9`: a
  fraction of ONE font size, which says nothing about how tall the other line
  is. The two- (or three-) line block is now laid out from its own centre with a
  real baseline-to-baseline leading of `max(upper, lower) * 1.15`, which always
  clears the touching threshold of `(upper + lower) / 2`.
- **Depth pruning stopped destroying job titles.** When an org chart is too deep
  for legible boxes the deepest level is removed and the surviving parent is
  marked `+N reports` — and that marker was written OVER the parent's real
  title, so a 25-node tree rendered as "Director 1 / +3 reports" and "VP Area 1"
  was gone from the deck. The count is now its own line under the title, which
  is preserved.
- **The `org_chart` capability described only half its overflow behaviour.** It
  said "siblings >9 collapsed to +N more indicator" beside `max_nodes: 50` /
  `max_depth: 20`, with nothing about depth pruning — so an agent budgeting
  against `max_nodes` had no idea that a 25-node, three-level tree renders as 7
  boxes (3 siblings per parent, nowhere near the 9 limit). `OverflowBehavior`
  and `docs/diagrams/org_chart.md` now describe both mechanisms and say plainly
  that `max_nodes` is a ceiling, not a guarantee.

- **Slide overlays paint on top of images and charts
  (go-slide-creator-yomm).** Numbered callouts on a screenshot — one of the most
  requested product/demo slides — could not be built. Overlay shapes were folded
  into `SlideSpec.RawShapeXML`, which is inserted at the **start** of the
  spTree, while media pics and native-SVG icons are inserted later. In the
  generated XML the order was badge, badge, arrow, …, `<p:pic>`, so the
  screenshot painted over everything annotating it: badge 2 (dead centre on the
  image) was invisible, badge 1 showed only the sliver hanging past the image's
  left edge, and the arrow's head — which lands on the image by design —
  disappeared with them.
  Overlays now travel in their own `SlideSpec.OverlayShapeXML` and are inserted
  at the END, after media pics and icons. Grid cell shapes still go in first, so
  overlays stay above those too.
  The overlay arrowhead needed no change: `<a:tailEnd type="triangle"/>` was
  already emitted and renders correctly — it was simply hidden under the
  picture, like everything else.

- **A slide's `source` renders exactly once, and without stuttering
  (go-slide-creator-xg48).** The original report — a `chart_insight` slide with
  8 insights losing its `source` when it fell back to two-column — was fixed by
  the universal-field work in go-slide-creator-zmjs, which sets a slide-level
  source for every kind. That fix exposed two adjacent defects on the same
  slides, both visible in the render:
  `chart_insight`'s pattern path puts the source in the
  `chart-insights-split` values and draws it under the chart, so also setting
  the slide-level source printed the attribution **twice** — once under the
  chart and once in the chrome source band. `applyUniversalSlideFields` now
  stands down when the compiled slide's pattern already carries a non-empty
  `source`.
  And the source band labelled its line unconditionally, so an author who wrote
  `"Source: Company filings FY2022-FY2026"` — the natural way to write it —
  shipped `Source: Source: Company filings FY2022-FY2026`. The label is now
  added only when the citation does not already start with one
  (case-insensitively).

- **Failed DeckSpec renders set `isError`, and a written deck now says whether
  it is shippable (go-slide-creator-swak).** Two halves of one problem: the
  agent could not tell, from any field it was told to branch on, that something
  had gone wrong.
  `isError` is the only protocol-level failure signal, and it was absent on
  every DOMAIN failure of the semantic tools — template not found, an
  unparseable spec, an unknown diagram type — while argument-level failures on
  the *same* tools set it, and `generate_presentation` / `validate_input` /
  `score_deck` / `render_*` set it for the equivalent `TEMPLATE_NOT_FOUND`. A
  harness branching on `isError` treated a deck that was never written as done
  and went on to render thumbnails of a path that did not exist.
  `render_deck_spec` and `compile_deck_spec` now set `isError` whenever their
  payload reports `ok: false` / `success: false`. The structured payload is
  untouched — diagnostics and `explanation_summary` are exactly as valuable on a
  failure. Tools that ASSESS rather than produce (`validate_deck_spec`,
  `validate_input`, `validate_pattern`) deliberately keep reporting an invalid
  deck as a successful call with `ok: false`: their verdict is the product, and
  the repo's contract tests pin it.
  Separately, a render could return `ok: true` with an `action: refuse`
  diagnostic buried in `diagnostics[]` and a quality gate that had already
  failed — `quality_summary.score: 85`, `pptx_path` set, exit 0. `success` means
  "the file was written" and other tools' parity depends on that, so the answer
  is a second verdict rather than an overloaded first one: the response gained
  **`publishable`** and **`blocking_reasons[]`**. A deck is publishable when
  nothing in its diagnostics is error-severity or `action: refuse` AND the
  deterministic quality gate passed. `json2pptx semantic render` exits non-zero
  on an unpublishable deck under the default `--output-validation strict`.

- **`render_deck_spec` renders no longer destroy each other
  (go-slide-creator-tngh).** `generate_presentation` takes `output_filename`;
  the recommended new-deck tool did not, so every DeckSpec render wrote
  `<output_dir>/output.pptx`. Two renders in one session returned success with
  different `content_hash` values and the *same* `pptx_path`, and the file held
  only one of the decks — so one caller's `content_hash` matched nothing on
  disk. That is worse than a lost file: `submit_visual_review` requires
  `pptx_revision` to equal the artifact's sha256 and `render_deck_thumbnails`
  keys its cache on file content, so an MCP-only agent could not complete, could
  not see why, and had no shell to copy files between calls.
  `render_deck_spec` now accepts `output_filename` with the same sanitisation as
  `generate_presentation` (path components stripped, `.pptx` appended), and
  defaults to `slug(meta.title)-<8 hex of the spec digest>.pptx` — two different
  specs never collide, and re-rendering an unedited spec is idempotent. The
  response gained `overwrote`, true when a file already existed at `pptx_path`.
  Writes to one output path are now serialized across the whole server (the
  content hash is read back off the file, so an interleaved write produced a
  hash that was never on disk), which covers every render tool, not just this
  one.

- **`chrome.section_crumb` does something now (go-slide-creator-ynfv).**
  `get_capabilities().features.section_crumb` reported
  `{supported: true, version: "2.8.0"}` and SKILL.md repeated the claim, but
  `ChromeInput.SectionCrumb` was parsed and then referenced nowhere outside the
  capabilities descriptor: `composeChromeLine` ignored it and `FooterConfig`
  held one deck-wide string with nowhere to vary. A deck with
  `structure.sections[].title` and `chrome.section_crumb: true` rendered
  "Confidential — Acme | Sept 2026" on all ten slides. An agent
  capability-gating on `get_capabilities` would have believed it shipped a
  section tracker.
  `expandStructure` now stamps each content slide with the section it came
  from, and `FooterConfig.LeftTextBySlide` carries a per-slide footer line, so
  slides 4-5 read "… | Market context" and slide 7 "… | Where we win".
  Dividers keep the plain line — they announce the section in 60pt type — and
  cover / agenda / closing slides sit outside any section. Decks that do not
  set the flag, and decks that set it without a `structure` block, produce
  exactly the bytes they did before.

- **Deck chrome is no longer drawn in the slide's own background color
  (go-slide-creator-hln7).** On modern-template's section divider, the footer
  "Confidential — Project ATLAS | Northwind Corp" rendered white on white — the
  first half invisible — and the page number "3 / 10" did not appear at all. The
  layout fills its background with `schemeClr tx1` under
  `<a:overrideClrMapping tx1="lt1">`, and chrome is injected as literal
  `schemeClr tx1`, so text and background resolved to the same color. No finding
  was emitted anywhere, because contrast enforcement only ever looked at
  placeholders and shape grids.
  Two things are fixed. The layout's color map override is now applied wherever
  the engine resolves a scheme color — the background, the slide's own text, and
  chrome — so it reasons about the colors the slide will actually show; read
  literally, that background was dk1 (near-black). And chrome goes through
  contrast enforcement: when the inherited color falls below WCAG AA against
  that background, it is pinned to an explicit color from the template's text
  palette and reported as `contrast_autofixed` at `/slides/{i}/chrome` with
  `fix.params.source: "chrome"` (here `#FFFFFF → #2C3932`, ratio 1.0 → 12.1).
  When nothing in the palette reads better, no swap is claimed — the response
  carries a warning naming the layout and the ratio.
  Blast radius is the layouts that actually invert their map: across all 34
  `examples/` decks on all four bundled templates, every deterministic deck
  generates byte-identically except where chrome was previously unreadable.

- **Concurrent renders no longer fail at random
  (go-slide-creator-0ixs).** `internal/render` was the one LibreOffice call site
  in the repo that did not pass `-env:UserInstallation`
  (`cmd/pptx2jpg`, `internal/layoutpreview` and `internal/qualitybench` all
  did), so every json2pptx process shared one LibreOffice profile. A second
  process converting at the same moment exits 0 and writes no PDF — silently,
  with nothing in its log. Measured: four MCP servers each calling
  `render_slide_image` on the same deck, **2 of 4 returned
  `RENDER.RENDER_FAILED`**, every run; rendering all 34 `examples/` decks four
  at a time produced images for **17 of 34**. The in-process mutex could not
  help, because the collision is between processes. Each process now converts in
  its own profile (created once, reused — a cold profile costs LibreOffice
  ~0.55s), and a conversion that still produces no PDF is retried once against a
  throwaway profile. After: **4/4** servers, **34/34** decks, and the same holds
  with a foreground `soffice` holding the default profile open. Renders are
  byte-identical to before (RMSE 0 across 20 slides on midnight-blue and
  warm-coral). The two developer commands that shared the same defect,
  `preview-patterns` and `audit-palette`, were fixed with it.
- **`RENDER_FAILED` now names the cause an agent cannot see.** The old message
  was `PDF not created at /var/folders/.../render-deck-NNN.pdf`, which reads
  like a broken deck and sent agents off editing content that was fine. It now
  reads `LibreOffice produced no PDF at <path> after 2 attempts (profile
  <dir>). This usually means another LibreOffice instance was running, not that
  the deck is invalid: close any open LibreOffice and retry this call`, with
  LibreOffice's own stderr appended when it said anything. `describe_finding
  RENDER_FAILED` carries the matching remediation step.

- **The heuristic visual pass stopped inverting the signal
  (go-slide-creator-3pyf).** With no `ANTHROPIC_API_KEY`,
  `inspect_slide_images` falls back to pure-Go checks — and its edge-band check
  reported 14 `text_overflow` findings on the BEST deck of the calibration set
  and 10 on the worst. Every one was the template's own decoration ("100.0% of
  pixels in the left edge" is midnight-blue's accent rail; "16.7% in the right
  edge" is its takeaway band), and it fired on blank "Thank you" slides. Since
  `get_started`'s NO VISION PROVIDER path routes agents here, an agent without a
  key learned to ignore the visual channel. The check now requires ink that
  alternates with the background like glyphs do (≥4 transitions per scan line);
  a solid fill crossing the band is decoration and is not reported. On the
  evidence renders: G1 14 findings → **0**, B08 10 → **0**, B05 8 → **0**, and
  B03's blank-slide finding is unchanged. `inspect_slide_images`' description
  now states plainly that heuristic mode cannot approve a deck.

- **Tools carry real MCP annotations (go-slide-creator-ccqn).** Every tool
  shipped mcp-go's `NewTool()` defaults — `readOnlyHint:false`,
  `destructiveHint:true`, `idempotentHint:false`, `openWorldHint:true`, no
  title — so `get_started`, `get_capabilities`, `list_templates`,
  `describe_finding` and every `validate_*` were advertised as destructive,
  non-idempotent and open-world. Hosts that gate destructive tools prompted on
  every discovery call. Annotations are now derived from the existing
  classification metadata: `readOnlyHint = !mutates_state && (!writes_files ||
  cache_writes_only)`, `destructiveHint = mutates_state` (only the two
  template-settings writers), `idempotentHint = !mutates_state`,
  `openWorldHint = api_key_dependency` (only `inspect_slide_images`), plus a
  human `title`. 42 of 52 tools are now read-only. New classification field
  `cache_writes_only` marks `list_templates`, whose only writes are a
  regenerable layout-preview cache.

- **design_mode_violation's next_tool_call is executable (go-slide-creator-g0er).**
  The refusal suggested `{tool: "generate_presentation", args_template:
  {design_mode: "free"}}`; following it verbatim failed with
  `UNKNOWN_PARAMETER` because `design_mode` is a field of the PRESENTATION, not
  a tool argument — and the word appeared nowhere in `tools/list` or
  `get_capabilities`, so the only escape was guessing where it belonged. The
  suggestion now nests it (`{presentation: {design_mode: "free", slides: …}}`),
  `generate_presentation`'s `presentation` schema declares `design_mode` with
  its enum, and `get_capabilities().features.design_mode` advertises the field
  with a usage hint that says it is not a tool argument.

- **The recommended new-deck path can see (go-slide-creator-05wn).**
  `render_deck_spec` reported only what generation happened to emit: one
  `executive_summary` slide with a 100-word body came back with
  `diagnostics: null` and `quality_summary.score: 100`, while `validate_input`
  on the SAME compiled deck reported `FIT.BODY_TOO_LONG`. On an 11-slide deck it
  reported 3 diagnostics against validate_input's 8 (five wrapped titles among
  them). There was also no gate on the semantic surface — `quality_summary`
  returned 100 for 15 of 16 calibration decks, including a lorem deck — so the
  gate SKILL.md calls "the machine-readable definition of done" sat off the
  recommended path behind `compile_deck_spec` + `score_deck`.
  - `render_deck_spec` now runs `collectFitFindings` over the compiled deck and
    reports the findings (deduped, canonically sorted) as diagnostics.
  - `quality_summary` gains **`structural_score`** and **`quality_gate`**, and
    its headline `score` is capped at the structural score.
  - `validate_deck_spec` compiles the spec and runs the same collectors,
    excluding the geometry-airiness advisories (`sparse_layout`, `SPARSE_FILL`,
    `SLIDE_UNDERUSED`, `cell_underfilled`, `SLIDE_NEARLY_EMPTY`, …) that a
    one-slide spec cannot control.
  - The compiler's source map now registers the placeholder-addressed spelling
    of each content link (`/slides/0/content/body`) alongside the indexed one,
    so a collected finding carries the `semantic_path` an agent edits —
    `BODY_TOO_LONG` on the repro now reports `slides[0].points`.

- **repair_slide resolves a pattern slide's cell path (go-slide-creator-qnrb).**
  The visual-QA bbox hit test produces cell paths for pattern slides, and
  `propose_repairs` handed back a ready-made `repair_slide` call — which
  returned `{applied: false, "slide has no shape_grid"}`, dead-ending the
  visual-QA → repair loop on exactly the slides the paths were built for.
  `reduce_cell_text` now expands the pattern, reads the addressed cell, finds
  the `pattern.values` string that produced it, and shortens that value
  (dropping the stale expanded grid). Both spellings of the path are accepted
  (`/slides/N/shape_grid/rows/R/cells/C` and `/slides/N/pattern/...`). Text
  composed at expansion, which matches no single value, refuses with
  `code: "wrong_kind_for_target"` and `did_you_mean: "replace_value"` rather
  than a blank failure — and a string that appears in several values refuses as
  ambiguous instead of editing the wrong one.

- **Layout selection no longer puts content on a divider (go-slide-creator-ujya).**
  On `modern-template`, a two-column slide (a chart beside eight insights — the
  shape the `chart_insight` density fallback compiles to) scored "Section
  Divider" 0.83 and "Title Slide" 0.82 over "Two Content" 0.15. The insights
  landed in the divider's "Section Number" placeholder, rendered at ~54pt in
  four overlapping lines, and ran off the slide. Three scoring defects:
  - `scoreCapacity` gave layouts credit without checking they had enough content
    placeholders. The rule the slot path already applied (heavy penalty per
    missing placeholder) is now general: a side-by-side slide needs two content
    placeholders, any content slide needs one.
  - Bullet overflow could drive a structurally correct layout to 0. It is now
    floored at 0.35 once the layout has the placeholders the slide needs — too
    much text in the right structure beats the right amount in the wrong one,
    and the text-fit findings report the overflow separately.
  - The narrow-diagram penalty no longer fires on an explicit `two-column` /
    `comparison` slide, whose author asked for the split. Diagram types that
    need full width (business model canvas, org chart) keep it.
  - A new direct penalty keeps divider / title / blank layouts out of
    content-bearing slides, and the chart-insights fallback now pins the
    canonical `two-column` layout instead of relying on the heuristic.

  All four bundled templates now select "Two Content" for that slide; verified
  in the render (chart left, all eight insights right, takeaway inside the body
  column).

- **score_deck measures the deck, not just the codes (go-slide-creator-q7ar).**
  Across 16 calibration decks graded blind from their renders, `overall_score`
  correlated with the human grade at +0.39 and 10 of 13 defective decks passed
  the quality gate — including one whose every slide reads "Lorem ipsum" /
  "Click to add title" / "XX%" (score 99), eight identical KPI slides (100),
  four slides with no title (100) and five slides carrying a single one-word
  bullet (100). The score was `100 - sum(severity weights)`, so it only ever
  measured the codes that happened to exist.
  - New finding codes: **`WEAK_CONTENT`** (`refuse`; the raw-path twin of the
    compiler's `SEMANTIC_WEAK_CONTENT`, which only ever saw DeckSpec input —
    go-slide-creator-7ucp),
    **`MISSING_TITLE`**, **`SLIDE_NEARLY_EMPTY`**, **`DECK_MONOTONY`**
    (`refuse` at 6+ consecutive same-shape slides) and **`CHART_OVERLOADED`**.
    All five are describable via `describe_finding`.
  - `overall_score` is now pulled down by the SHARE of slides carrying a
    finding. Slides whose only findings are about airiness (`sparse_layout`,
    `SPARSE_FILL`, `SLIDE_UNDERUSED`, `cell_underfilled`,
    `pattern_underfilled`) or a wrapping title (`title_wraps`) are exempt from
    that count — a KPI slide is supposed to look sparse — and the penalty needs
    at least 3 affected slides.
  - `quality_gate.criteria` gains **`max_problem_slides_pct`** (default 40).
  - `sparse_layout` is no longer emitted for grids expanded from a named
    pattern: it compares authored bounds with estimated TEXT height, and a
    pattern's bounds are the pattern's choice — it called a clean three-card KPI
    slide "6% filled".
  - New regression fixture `cmd/json2pptx/testdata/calibration/`: the 16 graded
    decks plus their blind grades, with `TestCalibrationRanking` asserting the
    rank correlation and the gate verdicts, and
    `TestCalibrationDefectClassesAreDetected` pinning each deck's defect code.

  Result on the corpus: correlation **+0.77**, all 3 good decks pass the gate,
  11 of 13 defect decks fail it. The two that still pass are documented in the
  test: `B05_low_contrast_grid` (its defect survives only in the render, where
  vision sees it) and `B06_table_9_columns` (the static pass sees `review`; the
  full `score_deck` render pass raises a P0 and fails it).

- **auto_repair now repairs, and says what it tried (go-slide-creator-wmfo).**
  The tool `get_started(task="revise")` advertises as the fast path returned
  `repairs_applied: []` on all 16 calibration decks and all 34 examples — an
  unchanged deck, a red gate and no next step — and nothing in the response
  distinguished "nothing was wrong" from "every directive was rejected". Three
  causes, all fixed:
  - `split_pattern`, the directive the fit report proposes for an overfull
    pattern slide, refused with *"slide has no shape_grid to split"* because
    findings describe the problem, not the field to split. It now infers the
    repeated values array (the longest one; a tie is left alone), resizes a
    declared `columns`/`rows` to match the halves, and **refuses** when a half
    would violate the pattern's contract instead of producing a deck that fails
    to generate.
  - A pass applied at most one repair per SLIDE, so a slide with twenty overfull
    cells needed twenty passes against a budget of three. It now applies one
    repair per TARGET (`cell_path` / `path`); a structural repair that changes
    the slide count ends the pass so the next one re-derives findings against
    the new deck.
  - `trace[]` entries gain **`directives_proposed`**, **`directives_advisory`**,
    **`directives_applied`** and **`directives_failed[{kind, slide_index, code,
    reason}]`**, and `repairs_applied` is always an array (it serialized as
    `null` on a zero-repair pass).

  On the `B02_tiny_text_overstuffed` calibration deck: before, 1 pass, 0 repairs,
  `gate_passed: false`; after, 3 passes, 2 splits applied, score 48 → 64 → 76,
  converged. On a 20-cell overstuffed grid: one pass, 20 repairs, score 0 → 100.

- **Pattern slides are measured by the text detectors (go-slide-creator-adur).**
  `generateFitReport`, `collectReadabilityFindings` and the structural pass all
  walked `slide.ShapeGrid`, which a slide-level named pattern only gets at
  generation time — so the entire text-density / autofit / readability family
  silently skipped the surface SKILL.md tells agents to author through. The same
  content authored as a pattern reported one finding; authored as the pattern's
  own expanded `shape_grid` it reported ten. Across the 45-pattern MAX decks:
  125 findings → **399**.
  - `collectFitFindings` now expands `slides[].pattern` once (alongside the
    existing `compose` expansion) and every detector runs over that grid.
  - Findings on an expanded pattern are rooted at
    `/slides/N/pattern/rows/R/cells/C/...`, matching what the geometry detectors
    already emitted.
  - A `reduce_cell_text` fix on a pattern slide is replaced by the advisory
    `rewrite_field`, keeping the measured `max_chars` and naming the `pattern`:
    there is no cell in the deck JSON to edit, so the remedy is shortening the
    pattern's values.
  - The row-level `fit_overflow` ("row content ~27pt exceeds max_height 25pt")
    is now `review` rather than `refuse`. Every occurrence across the bundled and
    gallery decks is a 4–8% overshoot, and the per-cell checks already refuse
    text that is clipped after autofit.
  - `TEXT_BELOW_READABLE_MIN` is only reported when the predicted autofit shrink
    is 0.85 or harsher. The shrink is a prediction from an estimated cell box:
    on `examples/phase-roadmap.json` it predicts 12pt → ~10.6pt on cells that
    render at about 12pt, and reporting inside that error bar turned clean
    pattern decks into gate failures. Text *authored* below the floor is exact
    and always reported.

- **Shape-grid text density is measured, not counted
  (go-slide-creator-lmpu, go-slide-creator-yj77).** `fit_overflow` is the
  dominant reason a deck fails the quality gate, and it fired on cells that
  render correctly: the model compared a cell's character count against a
  single-font-size capacity — the size of its **largest** paragraph. So
  `examples/business-model-canvas.json` scored 62 and failed the gate with 4
  `refuse` findings while every bullet of those cells is fully visible, and a
  `stat-hero` cell (a 120pt number above three small support lines) reported
  911% "overflow". The inverse was just as bad: most cells of most patterns
  reported `underfilled`.
  - `Density.DensityPct` for a shape_grid cell is now a **height ratio**: every
    paragraph is wrapped at its own font size, the line heights are summed, and
    the block is compared with the height the cell offers. New fields
    `required_height_pt`, `available_height_pt`, `lines`, `autofit_scale` and
    `fits` on `textcapacity.Density`.
  - `Budget.FontPt` / the reported `@ Npt` is the **dominant** size (the one
    carrying the most characters), not the largest. `max_chars` survives as a
    derived sizing hint at that size.
  - An unsized cell is measured at the size it renders at
    (`shapegrid.DefaultTextSizePt`, 14pt) instead of a legacy 11pt budget
    default.
  - `fit_overflow` on a shape_grid cell is emitted only when the text does not
    fit **even at the smallest autofit shrink** the renderer applies (20%) —
    text that is actually clipped. A cell the renderer merely shrinks into is
    reported by `TEXT_BELOW_READABLE_MIN` (`review`) if the shrink pushes it
    below the readable floor, and not at all if it stays readable. The message
    now reads `"text needs Npt of height in a cell that offers Mpt (D% at Ppt);
    even the renderer's smallest autofit shrink leaves it clipped"`.
  - `TEXT_BELOW_READABLE_MIN` on a grid cell carries
    `reduce_cell_text{cell_path, max_chars}` instead of the unreachable
    `reduce_text`.
  - The autofit prediction has a single implementation
    (`textcapacity.AutofitScaleFor`), shared by the fit report and the
    readability check.

  - The **underfill band moved from 60% to 35%** (`textcapacity.UnderfilledPct`).
    A 60% floor is far too high for a height ratio: across the 42-pattern
    realistic corpus the median cell fills 48% of its box, so the median
    well-authored slide was reported `underfilled`.

  Effect on the bundled examples: `business-model-canvas` 62 → **90** and the
  gate passes; `varied-pitch-deck` stays 95 / passing; `sovereign-ai-strategy`
  90 → 91 (its remaining P0s are real table-cell overflows). No example gains a
  failure. Across the realistic pattern corpus (42 patterns / 298 cells):
  optimal 52 → **182** cells, underfilled 211 → **94**, overflow 35 → **22**
  (worst 911% → 171%), and patterns with a majority of cells reported
  underfilled 27 → **11**.

- **The two defect classes that block the gate now have working repairs
  (go-slide-creator-9zof).** `BODY_TOO_LONG`'s own `next_tool_call` is
  `repair_slide{reduce_text, {current_words, max_words}}`; applied verbatim it
  returned `applied: false, "no text content found to reduce on this slide"` on
  every slide, because `reduce_text` honored only `max_items` and `max_length`.
  On a `shape_grid` deck, `propose_repairs` mapped 60 of 60 `fit_overflow`
  findings to `reduce_text` and `repair_slides_batch` applied 0 of them, because
  grid-cell text is only reachable by `reduce_cell_text`. Either way the loop
  stalled with a permanent residual.
  - `reduce_text` now accepts **`max_words`** and **`max_chars`** (alias
    `max_length`) and applies them to `text_value`, `bullets_value`,
    `body_and_bullets_value` and `bullet_groups_value`. A word/char budget is
    distributed across bullets in proportion to their length — every bullet
    survives, shortened at a word boundary with a single `…` — rather than
    truncating the list. The protected-fact guard still refuses
    (`semantic_review_required`) when a trim would drop a number, unit,
    negation, or qualifier. A budget-less directive now says so instead of
    blaming the deck.
  - `fit_overflow` on a shape_grid cell emits
    `reduce_cell_text{cell_path, max_chars}` instead of an unreachable
    `reduce_text`; the row-level overflow finding emits the advisory
    `increase_row_height` (a row is not a text target — its cells carry the
    executable fixes).
  - `propose_repairs` retargets any `reduce_text` fix whose finding path points
    inside a shape_grid cell to `reduce_cell_text` with the cell path.
  - `repair_slide` answers a directive it cannot route with the new
    `code: "wrong_kind_for_target"`, a **`did_you_mean`** field, and a
    `next_tool_call` carrying the corrected directive.
  - `next_tool_call` is now emitted for every executable fix kind: the
    suggestion builder had its own hand-maintained kind list, which omitted
    `reduce_cell_text`, so grid-cell findings shipped `next_tool_call: null`.
  - `describe_finding` covers the new repair-result codes
    `advisory_fix_kind` and `wrong_kind_for_target`.

- **Fix kinds are explicitly executable or advisory
  (go-slide-creator-ui4c).** 506 of 814 fix-carrying findings across a 50-deck
  corpus named a `fix.kind` that `repair_slide` cannot apply — and they were the
  findings that fire on the *good* decks (`cell_underfilled` →
  `add_detail_or_resize`, `sparse_layout` → `grow_pattern`, `SPARSE_FILL` /
  `SLIDE_UNDERUSED` → `add_detail_or_resize`, `table_font_scaled` → `review`,
  `density_exceeded` → `reduce_columns`). `repair_slide` answered
  `kind_not_supported` and `propose_repairs` filed them under `unmapped[]` as
  `fix_kind_not_repairable:<kind>`, so the documented loop terminated with zero
  directives and the only remaining signal was prose in `fix.params.hint`.
  - New registry `internal/patterns/fix_kinds.go` is the single vocabulary:
    every kind is `executable` (repair_slide applies it) or `advisory` (the
    remedy is an authoring decision), and each advisory kind carries guidance
    plus the executable `alternatives` that address the same defect.
  - `get_capabilities().vocabularies` gains **`advisory_fix_kinds`**;
    `repair_fix_kinds` is now derived from the registry (same values).
  - `repair_slide` returns `code: "advisory_fix_kind"` with the guidance as
    `message` and a new `alternatives[]` field for a registered advisory kind.
    An unregistered kind still returns `kind_not_supported`.
  - `propose_repairs` gains **`advisory[]`**
    `{reason, kind, guidance, alternatives[], code, slide_index, path, message,
    params}` and `summary.advisory_findings`; only unknown kinds and
    fix-less findings remain in `unmapped[]`.
  - New executable fix kind **`set_max_height_pct`**
    (`params: {max_height_pct}`) caps `slides[i].pattern.max_height_pct` so an
    underfilled pattern stops stretching — the mechanical remedy that the
    underfill / overtall-lane findings already recommended in prose but that no
    directive could express.

- **submit_visual_review binds the review to the artifact's own pixels
  (go-slide-creator-jltp).** `submit_visual_review` is the documented completion
  path when no vision provider is configured, and only the PPTX sha256 was
  checked — so a review whose six slides all pointed at `slide-01.png`, and one
  built from a different deck's images, both returned
  `status: "visually_reviewed_current_revision"`. The completion contract was an
  honour system. Each slide's pixel hash is now compared against this server's
  own render of that slide of that exact artifact (the render cache keys by file
  hash + density, so every density the deck was rendered at counts):
  - A recycled or foreign image is rejected with `INVALID_PARAMETER`, naming
    which slide the image actually is. Slides that genuinely render to identical
    pixels still verify — the rule is per-index identity, not hash uniqueness.
  - New response field `image_verification`
    `{status: verified|unverifiable, method, verified_slides, total_slides,
    reasons[], how_to_verify}` (required).
  - New `status` value **`reviewed_unverified_images`**: the review approves the
    deck but this server has no render of the artifact to compare against.
    `evidence.pixels_rendered` stays `false`, the completion status is withheld,
    and no `visual_evidence` is written to the authoring manifest.
  - `evidence.pixels_rendered` is now a claim about the reviewed artifact rather
    than an unconditional `true`.

- **Recommendations stay inside the active tool profile
  (go-slide-creator-mvny).** In the default `core` profile
  `get_started{task:"revise"}` returned `fast_path.tool: "auto_repair"` with
  `read_presentation` in `falls_back_to` and as sequence step 2 — three tools
  `tools/list` does not advertise in that profile. A client model cannot emit a
  call to a tool it was never shown, so the RECOMMENDED path for the entire
  revise task was uncallable; the note suggesting the operator restart with
  `--tools all` did not make it callable. Fixed on three surfaces:
  - `get_started` is profile-aware. In core, `revise` returns
    `fast_path.tool: "repair_slide"` with the loop spelled out in `steps`
    (`validate_input` → `preview_presentation_plan` → `repair_slide` →
    `generate_presentation` → `render_deck_thumbnails`), and
    `read_presentation` is dropped from `sequence`. Under `--tools all` the
    `auto_repair` facade and the inspection hop are unchanged.
  - `next_tool_call` suggestions pass through a profile filter: one naming a
    hidden tool is rewritten to the in-profile equivalent
    (`recommend_pattern` → `recommend_visual` with `hints.item_count`;
    `read_presentation` / `validate_presentation_output` →
    `render_deck_thumbnails` on the same file) or omitted when the profile has
    no equivalent. `get_input_schema` (1.2KB) is now a core tool instead, since
    it is the suggestion on the most common failures (`INVALID_JSON`,
    `UNSUPPORTED_MODE`) — core is 23 tools.
  - Core tool descriptions no longer name hidden tools: `repair_slide`'s
    `expected_revision` cited `propose_repairs`, `list_templates` sent agents to
    `get_data_format_hints` for full hints, and `get_started`'s own description
    described the `auto_repair` / `make_deck` / `read_presentation` workflow. Each
    now renders for the active profile.

### Added

- **Icon search by business concept (go-slide-creator-3ojy).** `list_icons`
  offered only a case-insensitive substring filter on glyph names, so business
  vocabulary missed entirely: `strategy`, `revenue`, `customer`, `efficiency`,
  `governance`, `compliance`, `innovation`, `cost`, `profit`, `roadmap`,
  `milestone`, `process`, `security` and `quality` all returned zero hits, while
  `risk` returned eight icons whose only connection was containing `asterisk`
  and `team` returned `brand-teams` / `ironing-steam`. A curated index of 142
  business concepts now maps each to the 1–3 bundled icons that express it.
  Concept hits LEAD the results even when the substring filter also matched, so
  `risk` opens with `alert-triangle`. New response fields: `matched_via`
  (`name` / `synonym` / `synonym+name`), `concept_matches[]` naming which concept
  produced each icon, and `concepts[]` — the full vocabulary — when a query
  matches nothing at all. Multi-word queries reach the concepts inside them
  (`cost reduction` → the cost icons). `ICON_BUNDLED_NAME_UNKNOWN`'s suggestions
  consult the same index first, so `strategy` now suggests `target` / `chess` /
  `map-2` rather than `karate`, and `revenue` suggests `coin` / `cash` rather
  than `venus`; genuine typos still fall through to edit distance
  (`chart-bra` → `chart-bar`, `rockt` → `rocket`).

- **Every tool `outputSchema` now admits the error envelope
  (go-slide-creator-vtqo).** MCP 2025-06-18 requires a tool declaring an
  `outputSchema` to return conforming `structuredContent`, and clients may
  validate — but every error result carries the diagnostics `FindingEnvelope`,
  which matched no success schema, so all 11 tested error responses were
  schema-invalid and a validating client rejected them instead of showing the
  diagnostics. All 52 declared schemas are now wrapped as
  `{$defs?, anyOf: [<success>, <error envelope>]}` (any `$defs` is hoisted to
  the wrapper root so `#/$defs/…` references keep resolving), so structured
  errors — which agents depend on — stay exactly as they were and now conform.
  Three success responses were also invalid and are fixed:
  `get_capabilities.runtime.render_missing_commands` and
  `analyze_deck_rhythm.aggregates.pattern_runs` / `.recommendations` serialised
  as `null` from nil slices where the schema said array, and
  `show_pattern.example_values` was declared object-only although a
  list-valued pattern's example is an array.

- **`LAYOUT_UNRESOLVABLE` finding (go-slide-creator-9svyz).** New review-severity
  code (fix kind `swap_layout`, params `slide_type` / `candidates[]` /
  `fallback_layout_id`) emitted by `validate_input`, `validate --fit-report` and
  preview for every slide whose `slide_type` no layout in the template can host.
  Neither surface ran layout resolution before, so a template missing a role
  returned `valid: true` with no findings and `generate` then failed the whole
  deck at the first affected slide.

### Fixed

- **A template missing a layout role no longer fails the whole deck
  (go-slide-creator-9svyz).** Auto-selection failure aborted generation at the
  first affected slide of seven, and the error named the INTERNAL coerced slide
  type ("diagram") rather than the `slide_type` the author wrote. A `pattern` /
  `shape_grid` / `compose` slide needs only a canvas, so it now falls back to the
  template's Blank+Title (else Blank) layout with a per-slide warning, which
  means every affected slide is reported rather than just the first; other slides
  still refuse, now naming the authored `slide_type`.

- **`shorten_title` no longer truncates titles into fragments
  (go-slide-creator-28zf).** It byte-sliced at `max_length`, producing
  "Workstream 1: Our comprehensive enterprise-wide di" — cut mid-word — while
  the deck's score went UP for having meaningless titles. It now cuts at a word
  boundary (never mid-word, never mid-rune), drops a dangling colon or trailing
  function word, and **refuses** with `semantic_review_required` when the cut
  would leave a fragment: fewer than 3 words, or half the headline or more
  removed. It also accepts `max_words` as well as `max_length`, so the directive
  `HEADLINE_TOO_LONG` carries and a call constructed from `repair_slide`'s own
  description behave identically — that finding now supplies both params, and
  switches its `fix.kind` to `review` when the headline is more than twice the
  budget, because that is a rewrite rather than a trim.

- **Title autofit no longer crushes line spacing into a collision
  (go-slide-creator-g5h7).** `bakeTitleFit` applied its line-spacing reduction to
  the TEMPLATE's own spacing rather than to single spacing, so a template that
  already ships tight leading compounded it: modern-template's ~80% times a 20%
  reduction wrote `<a:spcPct val="64000"/>` while keeping `sz=2700` — a 17.3pt
  line pitch for 27pt all-caps glyphs, which LibreOffice renders with the lines
  physically overlapping. The baked pitch is now floored at 85% of single
  spacing (the reported collisions were at 76% and 64%), and a title that no
  longer fits at that floor falls through to the existing measured-overflow path
  and surfaces `TITLE_OVERFLOW` instead of being silently crushed.

- **Contrast autofix targets the ratio the text size requires
  (go-slide-creator-9ux4).** Every swap targeted the WCAG AA *large text* ratio
  of 3.0:1, on the stated assumption that presentation text is almost always
  ≥18pt or ≥14pt bold — false for the 11pt supporting line a card carries. Swaps
  landed at exactly 3.0 and the text rendered as barely-visible grey-on-grey. The
  threshold is now derived from the run's own size and weight: 4.5:1 for normal
  text, 3:1 only at ≥18pt (or ≥14pt bold). A shape grid's text body takes its
  threshold from its SMALLEST run, so an 11pt label beside a 28pt KPI value is
  fixed for the label. `ContrastPreflightPair` gains `TextPt` / `Bold` so
  `contrast_predicted` stays identical to the render-time swap. Measured on the
  reported cases: `#CCCCCC` on `#DDDDDD` now lands at 4.56 (was 3.0), `#444444`
  on `#333333` at 4.55, `#9ACD9A` on `#8FBC8F` at 4.51.

- **`cell_underfilled` is one finding per slide, and exempts metrics and labels
  (go-slide-creator-xpz8).** It was emitted once per cell with no aggregation —
  by far the most common finding in the system (395 occurrences over 50 decks) —
  so a slide of KPI cards accumulated 20+ review-weight findings and bottomed
  its score out at 0, while the one genuinely broken layout on the same deck cost
  5 points. Three changes: cells whose role is a metric value or a short
  label/caption are exempt (a KPI card holding `$12.4M` is correct, not
  underfilled); the rest fold into one finding per slide listing each in
  `fix.params.cells[]`; and the finding is advisory (`action: info`) unless the
  grid carries under 30% of its text capacity across 3+ cells, measured as total
  characters over total capacity rather than by counting sparse cells — a card
  grid whose bodies are one deliberate sentence each is a design, not a defect.
  Measured on the bundled examples: `kpi-big-numbers` 26 findings → 0,
  `consulting-layouts` 53 → 7 (1 escalated), `visual-maturity-stress-test`
  81 → 7, `sovereign-ai-strategy` 71 → 10.

- **Unknown chart/diagram types get a vocabulary, and a grey placeholder blocks
  (go-slide-creator-rrjj).** Asking for an unregistered type (combo, sankey,
  choropleth, or a typo like `barchart`) produced a slide-sized grey
  "Data unavailable" box while `generate_presentation` answered `success: true`
  with a score in the 90s, burying the reason in `warnings[]`. Three changes:
  svggen's unknown-type error is now structured — `did you mean "bar_chart"?`
  plus the full allowed list, with the suggestion suppressed for genuinely
  unregistered types so it never guesses misleadingly; `diagram_render_failed`
  is now `action: refuse` at the render-time placeholder path (it was `review`,
  so the deck shipped) and is predicted at validate time for content surfaces as
  well as grid ones; and the grid-cell error names the author-meaningful row and
  column instead of the internal shape-allocator id ("grid cell 204").

- **Silent table row truncation now blocks (go-slide-creator-oaif).** A 12-row
  table shipped 9 rows plus a literal "…and 3 more rows" cell — three regions'
  financials absent from the deck — reported at `action: review` with the
  quality score still 100 and the gate still passing. `table_rows_truncated` is
  now `action: refuse` at both the pre-generation predictor and the render-time
  site (and `severity: refuse` in the describe catalogue), and its
  `fix.params` gains `split_at_row` so the repair is fully specified. Two
  plumbing gaps are closed with it: the strict-fit gate and the CLI
  `validate --fit-report` read only `generateFitReport`, which omitted the table
  predictor, so the refusal never reached either — both now include it. And
  refuse-class findings carry a hard, uncapped penalty in the input quality
  score (a truncating deck scores 75, not 100) instead of only the generic
  warning penalty, which caps at 0.2.

- **Venn items render, and a Venn beyond 3 circles is refused
  (go-slide-creator-onop).** The exclusive item list got 30% of a circle's
  radius — room for barely one short line — so a circle with 7 items rendered
  none of them and the slide showed three labelled circles and nothing else,
  with no finding. The budget is now 55% of the radius (7 items per circle render
  on a full canvas), and anything that still does not fit raises the new
  warning-severity `diagram.items_dropped` (fix kind `reduce_items`, carrying
  `dropped_count` / `total_count` / `rendered` / `circle`). Separately, more than
  3 circles used to be silently truncated to the first 3 — passing 5 rendered 3
  with no notice; `VennDiagram.Validate` now refuses it with a message naming
  `matrix_2x2` and a card grid as alternatives.

- **Org charts report the levels they drop (go-slide-creator-pwcg).** A
  21-person `org_chart` rendered as 5 boxes reading "+4 reports" — every named
  person below level 1 gone — with no warning and no finding.
  `collapseSiblings` reported the nodes it hid, but `pruneDeepestLevel`, which
  is what fires on a deep chart, reported nothing. It now emits the new
  warning-severity `diagram.org_chart_depth_pruned` (fix kind `reduce_items`,
  carrying `removed_count` and `kept_depth`) so the misrepresentation is
  visible and the author is told to split the chart across slides.

- **`theme_override` reaches the artifact (go-slide-creator-p327).** The override
  was applied only to the in-memory palette used for chart styling;
  `ppt/theme/themeN.xml` was copied from the template unmodified, so every
  `<a:schemeClr val="accent1">` in a layout, pattern or native shape still
  resolved to the template's colour. A deck asking for `accent1: "#6A1B9A"`
  rendered in the template's navy with zero warnings, while `resolve_theme`
  reported the override as applied. The generator now patches the theme part's
  `<a:clrScheme>` slots and `majorFont` / `minorFont` typefaces before the
  package is written, and the chart data palette resolves against the
  post-override theme. `ApplyOverride`'s advisories (a font the template does not
  embed) now surface as deck warnings on both the CLI and MCP paths, and an
  override naming a slot the theme does not declare warns instead of silently
  doing nothing. The ten `slog.Warn("… will not reflect overrides")` sites are
  removed — they no longer hold.

### Added

- **`<sup>` / `<sub>` inline markup + `UNSUPPORTED_INLINE_MARKUP`
  (go-slide-creator-510u).** `supports_inline_markup` advertised `[b, i, u]` and
  every other tag was passed through to the text run verbatim, so `<sup>`,
  `<a>`, `<color>` and `<code>` printed as literal XML-ish text on the slide with
  nothing reported. Superscript and subscript are now rendered (OOXML
  `a:rPr baseline="30000"` / `"-25000"`) and the capability list grows to
  `[b, i, u, sup, sub]`. Any tag outside it now raises the new advisory finding
  `UNSUPPORTED_INLINE_MARKUP` (`action: review`, fix kind `remove_key`, carrying
  both the `unsupported` and `supported` vocabularies) from `validate_input` and
  `validate --fit-report`. The scan reuses the no-emoji policy's traversal, now
  shared as `internal/policy/textwalk`, so it reaches every authored string.

- **`describe_finding` covers the codes it claimed to (go-slide-creator-7zrt).**
  `design_mode_violation` — the code most likely to block a generation — returned
  `INPUT.UNKNOWN_FINDING_CODE`, and the error inlined all 150+ known codes
  (~3.8KB) instead of a suggestion. It and `CUSTOM_COLOR_DROPPED` are now in the
  catalogue with remediation naming the deck-level `design_mode` field, along
  with 16 further codes that a new coverage test found equally undescribable
  (`REQUIRED`, `FILE_READ_ERROR`, `PATCH_ERROR`, `INVALID_STRUCTURE`,
  `STRUCTURE_AND_SLIDES`, `COMPOSE_SEGMENT_EXPAND_FAILED`, `no_emoji_violation`,
  `grid_violation`, `style_collision`, `redundant_field`,
  `legacy_authoring_form`, `kind_not_supported`, `semantic_review_required`,
  `duplicate_title`, `pattern_run`, `density_monotony`, `missing_emphasis`,
  `accent_dominance`). Unknown codes now answer with
  `fix.params.did_you_mean` and fall back to the full `allowed` list only when
  nothing is close — the typo response drops from ~3.8KB to ~0.5KB.

- **`list_templates` reports the real slide size and a real aspect ratio
  (go-slide-creator-r9nn, go-slide-creator-ccpv).** Two discovery defects:
  `aspect_ratio` was hardcoded to `"16:9"` and only overridden by template
  metadata, so a 10x7.5in (4:3) or 17.5x7.5in (21:9) template was reported as
  widescreen — and `fields=compact` exposes nothing but the ratio. It is now
  computed from the slide size via the newly exported
  `template.AspectRatio(width, height)`, which also recognises `4:3`, `16:10`
  and `21:9` and falls back to a decimal `W.WWW:1` form; explicit metadata still
  wins. Each entry additionally carries `slide_width_in` / `slide_height_in`.
  Separately, with a custom `--templates-dir` every embedded template was listed
  under its `os.CreateTemp` filename (`json2pptx-template-2729383514`, different
  on every call and rejected by `generate` as `TEMPLATE_NOT_FOUND`); entries now
  carry the logical name.

- **`waterfall-bridge` fills follow the template's `semantic_accents`
  (go-slide-creator-noa7).** `negative_accent` defaulted to the literal
  `accent2` and never consulted template metadata, so on modern-template
  (`negative: accent1`, a red) negative delta bars rendered `accent2` — its cool
  blue — while the declared negative colour sat unused. Negative bars now default
  to `semantic_accents.negative`, subtotals to `neutral`, and positive deltas to
  `positive`; the old literals remain the fallback for templates declaring no
  semantic accents, and explicit `negative_accent` / `subtotal_accent` overrides
  (or an author-chosen `cell_accent_mode`) still win. The two override
  descriptions no longer advertise a fixed literal default.

- **`kpi_dashboard` honours `unit` and `change` (go-slide-creator-hu58).** The
  documented metric shape is `{label, value, unit?, change?, trend?}`, but the
  native card builder read only `label` / `value` / `delta` / `trend`, so the
  unit and the delta — the two things an executive KPI card exists for — never
  reached the slide and no warning said so. `unit` is now attached to the hero
  value on the correct side (`184.2 EUR m`, `118%`, `$210m`), and `change` is
  read as the documented key with `delta` kept as a synonym. The declared
  capacity (`max_nodes` 12) is now enforced: extra metrics are dropped with a
  `CONTENT_DROPPED` finding naming the count instead of being crushed into
  unreadable slivers.

- **Merged table cells render their text (go-slide-creator-chvf).** A `col_span`
  / `row_span` cell produced one `<a:tc gridSpan="N">` and nothing for the grid
  columns it covered, so a row carried fewer `<a:tc>` than the table had
  `<a:gridCol>`. ECMA-376 requires an explicit `<a:tc hMerge="1"/>` /
  `<a:tc vMerge="1"/>` per covered column; without them LibreOffice dropped the
  second merged cell's text and painted an empty block — so two-tier financial
  headers came out blank. `TableInput.ToTableSpec` now materialises those
  continuation cells (consuming an author-supplied empty filler where one
  exists, so hand-padded rowspans are not doubled), which activates the
  generator's existing `IsMerged` branches. `OOXML_INVALID_TABLE` — the
  row-cell-count check, which already existed but was advisory — is now promoted
  to blocking, so strict `output_validation` can no longer ship this.

- **`chart.plot_area_collapsed` svggen finding (go-slide-creator-k478).** New
  warning-severity finding with fix kind `shorten_labels`, emitted when rotated
  x-axis labels would claim so much of a chart that the plot is squeezed into a
  sliver. The label band is now capped so the plot keeps at least 55% of its
  height, and the finding carries `needed_px`, `capped_px`, `total_categories`
  and `longest_label_len` so an agent can shorten the categories or split the
  slide. It reaches the fit-finding stream through the existing chart
  dry-render merge.

- **Chart `annotations` and `data_labels` (go-slide-creator-pizh).** svggen has
  always implemented reference lines, fitted trendlines, positioned callouts,
  and per-point value labels, but `commonChartFields()` did not declare the two
  data keys, so strict schema validation rejected them with `UNKNOWN_FIELD` and
  the deck rendered a grey "Data unavailable" placeholder while still reporting
  `success: true`. Both keys are now declared on every Cartesian chart schema
  (`bar_chart`, `line_chart`, `area_chart`, `stacked_bar_chart`,
  `grouped_bar_chart`, `stacked_area_chart`) and surfaced in `skill-info`
  `data_format_hints[].optional_keys`. `annotations[]` is
  `{kind: reference_line|trendline|callout, axis, value, label, style, color,
  series, method, x, y, text}` (`kind` required); `data_labels` is
  `{format, show_on: all|last|peaks|first_last}`.

- **`slides[].placeholders_dropped` (go-slide-creator-lhq6).** New optional
  array on each `generate_presentation` / `json2pptx generate` slide-resolution
  entry: the placeholder IDs the slide targeted that the resolved layout does
  not declare. Their content was NOT rendered. `placeholders_used` now reports
  only placeholders that actually received content (and `occupancy_pct` is
  computed from that reduced set) instead of echoing the requested IDs.

### Changed

- **Dropped content fails strict output validation (go-slide-creator-lhq6).**
  Content targeting a non-existent `placeholder_id` now emits a
  `CONTENT_DROPPED` fit finding carrying `fix.params.cause =
  "placeholder_not_found"` (plus `placeholder_id`, `layout_id`, `available`),
  and `success` is `false` under `output_validation: "strict"` (the default).
  Under `warn` / `off` the finding is emitted but `success` stays `true`. Other
  `CONTENT_DROPPED` causes (partial-mode slide skips, visual collisions) remain
  advisory.

### Fixed

- **`horizontal-bar-with-callouts` value labels (go-slide-creator-d6zo).** Bar
  values concatenated the raw number and the unit, so a currency/magnitude unit
  rendered as `1240.5EUR M`; and a bar short relative to `max_value` neither fit
  its label nor moved it, clipping `96.4` into stacked lines and dropping `3.8`
  entirely. Both patterns that carry numeric labels now share one formatter
  (`patterns.FormatMagnitudeLabel`): thousands separators, at most one decimal,
  currency symbols before the number (`$210m`), word units after it separated by
  a thin space (`1,240.5 EUR M`), short magnitude suffixes kept tight (`12bn`).
  A label needing more than 80% of its bar's width is now rendered in dark text
  just right of the bar end instead of inside it. `waterfall-bridge` adopts the
  same formatter, so its word units gain the thin-space separator.
- **`generate_presentation`'s inline pattern example is schema-true
  (go-slide-creator-e5tm).** The `presentation` parameter description carried a
  hand-written `{"pattern":{"name":"kpi-3up","values":{"items":[{"label","value"}]}}}`
  example — a shape no pattern accepts. Copying it passed `validate_input` and
  then failed `generate_presentation` with `INPUT.INVALID_SLIDE`. The snippet is
  now generated from the registry's own `ExemplarValues()` at tool-definition
  time, so it cannot drift from the schema, and a test runs every complete JSON
  example in the tool descriptions through the real pattern validator.
- **Icon cells keep the default text padding (go-slide-creator-jzb1).**
  `resolveIconOverlay` returned text insets of `{iconWidth+gap, 0, 0, 0}` (icon
  left) or `{0, iconZoneH, 0, 0}` (icon top). `buildTextBody` writes all four
  `lIns/tIns/rIns/bIns` as soon as one is non-zero, so those zeros were emitted
  verbatim and overrode PowerPoint's default 0.1in/0.05in padding: card text sat
  flush against the fill edge on every icon-bearing pattern (`card-grid`,
  `hero-detail`, `icon-row`, `kpi-2up`…`kpi-6up`, `kpi-inline`, `matrix-2x2`
  with icons). `iconOverlayLayout.TextInsets` is now the complete inset set —
  defaults on every side, plus the icon reservation on the icon's own axis.
  Expanded `shape_grid` output for these patterns changes accordingly.
- **`validate` / `validate_input` now run pattern value-schema validation
  (go-slide-creator-xvek).** Named-pattern values were never checked against the
  pattern's own schema during validation, so a deck reported VALID (`ok: true`,
  "no issues") was then refused by `generate` with `INPUT.INVALID_SLIDE` —
  `kpi-3up: values[0].small exceeds maxLength 40`, `exec-summary` `minItems`,
  `icon-row` unknown bundled icon names, missing required fields. The validate
  path now runs the same expansion `generate` runs over every slide-level
  `pattern`, `compose` envelope, and cell-level nested pattern, reporting
  failures as `PATTERN_ERROR` errors at `/slides/{i}/pattern`,
  `/slides/{i}/compose`, or `/slides/{i}/shape_grid` carrying generate's own
  message. `preflight` shares the same helper and now covers nested cell
  patterns too.
- **Last slide with visual content no longer lands on the Closing layout
  (go-slide-creator-lhq6).** Layout scoring withheld the last-slide "closing"
  bonus only by `slide_type`, so a `content` slide carrying an image, chart,
  diagram, table, or slot content was assigned a title+subtitle closing layout
  and the visual was silently dropped. Visual and slot-addressed content now
  always require a full content area.
- **Cover slide no longer resolves to the Closing layout
  (go-slide-creator-g5sb).** `isTitleSlideLayout` excluded only `blank-title`,
  so on a template whose Closing layout precedes Title Slide in master order
  (ties are broken by layout order) the deck opened on its closing slide. It
  now mirrors the canonical `"title"` rule and excludes `closing` too.

## 4.60.0 (2026-09-18)

### Added

- **`shape_grid.vertical_align` (go-slide-creator-7km8).** New optional grid
  field: `"stretch"` (default for raw grids — rows are re-scaled to fill the
  bounds, the previous behaviour), `"top"`, `"center"`, `"bottom"`. It takes
  effect when at least one row sets `max_height`: the capped, content-sized
  row block keeps its height and is placed inside the bounds instead of being
  stretched. For content-relative, top-anchored bounds (pattern height caps
  such as `before-after-compact`, `process-flow-compact`, `kpi-inline`,
  `numbered-step-strip` chevrons) `center`/`bottom` also position the capped
  box inside the content area (which already excludes the takeaway/source
  band). Invalid values are a validate error. Named patterns now expand with
  `vertical_align: "center"`; a `bounds` / `max_height_pct` override resets it
  to `stretch` so user-positioned blocks stay where they were put.

### Changed

- **Content-sized pattern heights (go-slide-creator-7km8).** `kpi-2up`…`kpi-6up`
  cards are capped at 45% of the content height (`max_height` on the row),
  `process-flow` steps at 35%, and the block is centred. `timeline-horizontal`
  default `dots` style now draws a real timeline: optional date row, an axis of
  accent dots joined by line connectors, and label/body text under each dot
  (previously full-height filled boxes); `chevron` / `gantt` rows are capped.
  `phase-roadmap` description and milestone rows hug their text.
  `before-after` / `before-after-compact` / `stylish-panels` header bands are
  sized to the header text (~1.2× line height + padding) instead of 25–30% /
  20% of the grid; their bodies, `hero-detail` rows and `card-grid` rows
  (except rows holding a secondary chart) hug their text, and sparse card
  text is vertically centred (go-slide-creator-3i7c). `before-after` body
  default size 12 → 14pt. Expanded
  `shape_grid` output for these patterns changes shape accordingly.

### Added — named patterns (go-slide-creator-xyph / e53n / ycbn)

- **`table-highlight`** — options × criteria evaluation matrix (2–6 × 2–6)
  with Harvey-ball (0–4), RAG or short-text cells, `highlight_row` /
  `highlight_col`, legend row, `overrides.rag_colors`.
- **`image-text-split`** — one image (`path` / `url` / placeholder) beside
  eyebrow + heading + body / bullets and 0–3 result metrics
  (`overrides.image_side`, `image_width_pct`). Its `values.image.path` is
  resolved against the deck directory and `values.image.url` fetched like a
  shape_grid image cell (`patterns.ImageAssetPattern`).
- **`exec-summary`** — 3–5 bold lead-in statements with supporting sentences
  and an optional `bottom_line` bar.

All three size their rows in points (`min_height` = `max_height`) and are
centred by the pattern `vertical_align` default.

### Changed — chart-insights-split so-what (go-slide-creator-pzrs)

- New optional values `headline {value, label}`, `so_what`, `chart_label`,
  `unit` and overrides `data_labels`, `headline_size`. The chart now carries
  a series / unit caption (derived from a single series name + unit) and
  value labels by default on bar charts (≤16 points) and single-series
  line / area charts (≤12 points). With a headline or so-what the insights
  cell expands to a nested column grid; the source row is pinned at 30pt.
  Expanded `shape_grid` output changes accordingly.

### Changed — shape_grid images and contrast preflight

- **Grid image cells cover-fill (go-slide-creator-e53n).** A raster `image`
  cell without `fit` now fills its whole cell and is centre-cropped
  (`a:srcRect`) instead of being squashed into a centred square; explicit
  `fit` values keep the square frame and are cover-cropped into it.
- **`contrast_predicted` judges tints (go-slide-creator-xyph).** Object-form
  fills with `lumMod` / `lumOff` / `alpha` are composed into their effective
  colour before the check, removing false predictions for dark text on light
  accent tints.

### plan_deck deck-quality fixes (go-slide-creator-xmpb)

#### Changed

- **`plan_deck` title/closing slides carry a layout, not a pattern.** Every
  `slides[]` entry gains a required `layout` field (canonical `layout_id`, always
  equal to `skeleton.layout_id`): `"title"` for slide 0, `"closing"` for the last
  slide, `"blank-title"` for every pattern slide. Title and closing slides now
  have `recommended_pattern: ""` / `suggested_pattern: ""`, no `alternatives` or
  predictions, and a layout-only skeleton (`title` + `subtitle` entries, no
  `pattern`). With `template`, their `template_support` vets the Title Slide /
  Closing layout family. Pattern skeletons' `layout_id` is now `"blank-title"`
  (was `"section"` / `"blank"` by role). make_deck's `plan.slides[]` likewise
  reports `recommended_pattern: ""` for those two slides.
- **Comparison slots map only to the comparison family** (`comparison-2col`,
  `before-after`).
- **Emphasis cap.** Emphasis patterns (`stat-hero`, `pull-quote`, and now
  `kpi-inline`) are capped at ceil(n/5) per n-slide plan; excess ones are demoted
  (must_include placements are kept). `rhythm_check.emphasis_count` /
  `has_emphasis` count all three; `pattern_variety` ignores title/closing slides.

#### Added (go-slide-creator-kndv)

- **`plan_deck` carries brief facts into the plan.** Quantity and named-entity
  clauses from the brief (e.g. `+23% revenue`, `churn 4%`, `EU expansion is on
  track`) are routed verbatim to pattern slides: each slide gains an optional
  `facts[]` array and its `content_seed` is prefixed with those facts. The result
  gains a required top-level `unplaced_facts[]` array (always present, `[]` when
  empty) listing facts no slide had capacity for. make_deck's derived slide
  titles pick up the facts through `content_seed`.

### DeckSpec fast path + server instructions (go-slide-creator-o8kl, go-slide-creator-f6kq, go-slide-creator-09e0)

#### Changed

- **`get_started{task:"brief"}` fast path is now the DeckSpec path.**
  `fast_path.tool` is `render_deck_spec` (was `make_deck`) and the new
  `fast_path.steps[]` lists `list_slide_kinds` → `validate_deck_spec` →
  `render_deck_spec` → `render_deck_thumbnails`. `make_deck` is repositioned as a
  skeleton/wireframe tool. `falls_back_to` still mirrors the raw `sequence`.
  `render_deck_spec` is now classified `kind: "workflow_facade"`
  (`primitive_alternatives`: `validate_deck_spec`, `compile_deck_spec`,
  `validate_input`, `generate_presentation`).
- **One completion rule everywhere.** `completion_protocol.rule`, the get_started
  notes, SKILL.md and the server instructions now share one text: render ALL
  slides (`render_deck_thumbnails`) and inspect every image; a passing gate /
  score is a precondition, never completion. SKILL.md's old "stop … do not
  render thumbnails" rule is removed.

#### Added

- **MCP server `instructions`.** The `initialize` response carries the 5-step
  quality workflow (get_started → DeckSpec → render + inspect all slides → fix
  at `semantic_path` → never ship exemplar content).
- **`get_started.quality_workflow`** (always present) echoes the same text
  (single Go const, so the two cannot drift).

### Strict MCP arguments (go-slide-creator-s9uq)

#### Changed

- **Unknown MCP tool arguments are rejected.** Every tool call is checked against
  the tool's declared input schema by a server-level middleware; an undeclared
  argument name now fails the call with the new code **`UNKNOWN_PARAMETER`**
  (`INPUT` namespace, `path` = the argument) instead of being silently ignored.
  The message lists the accepted arguments; when a close name exists the
  diagnostic carries `fix: {kind: "rename_field", params: {from, to,
  did_you_mean}}` and a `next_tool_call` retry (e.g. `plan_deck` `slide_count` →
  `slide_budget`). `make_deck` keeps accepting the legacy `max_passes` alias.

### Closed DeckSpec schema (go-slide-creator-h8o7, go-slide-creator-dg8f)

#### Changed

- **`spec` on `validate_deck_spec` / `render_deck_spec` / `compile_deck_spec` /
  `explain_deck_spec` now carries the real DeckSpec JSON Schema** (inlined — no
  `$ref`s) instead of `{"type":"object","properties":{}}`: `slides.items.oneOf`
  has one `Slide_<kind>` variant per kind, each `additionalProperties: false`
  with closed list-entry and chart object schemas. `json2pptx semantic schema`
  and `GET /api/v1/semantic/schema` emit the same closed variants (previously
  `additionalProperties: true`).
- **Unknown payload keys are diagnosed.** A slide payload key, list-entry key, or
  chart key the compiler never reads now yields `SEMANTIC_UNKNOWN_FIELD` at the
  exact path (warning; error under `strict`; not suppressed by `off`) with a
  `rename_field` fix (`params.{from, to, did_you_mean}`) when a close key exists.
- **Chart data hints point at `slides[i].chart.data`.** The chart advisory
  (`SEMANTIC_DENSITY`) moved from the non-existent `slides[i].chart.series` path
  to `slides[i].chart.data`; its message shows the expected shape and its `fix`
  (`provide_value`) carries `params.{path, expected_shape, example}`. Pie/donut
  `{categories, values}` data is accepted. A flat `chart.series` (never read by
  the compiler) no longer counts as data.

#### Added

- **`list_slide_kinds` entries gain `item_schema`** (the closed per-kind slide
  schema) **and `example`** (a copy-ready slide, including `kind`, that
  validates with zero findings under strict).

### make_deck exemplar gate (go-slide-creator-htwq)

#### Changed

- **`make_deck` never reports a passing gate on exemplar content.** Its slides
  are pattern exemplar placeholders, so the response now forces
  `gate_passed: false`, prepends the machine token `"exemplar_content"` to both
  `gate_reasons[]` and `blocking_reasons[]`, and reports `final_score: 0` plus
  a new `content_score: 0`. The deterministic layout/fit score the loop computed
  moves to the new always-present **`structural_score`** field. Previously a QBR
  brief could return `final_score: 98, gate_passed: true` for off-topic
  placeholder copy. `auto_repair` (author-supplied content) is unchanged.

#### Added

- **`submit_visual_review` MCP tool** — records a host/manual all-slide visual
  review (`pptx_path`, `pptx_revision`, `slides[{index, verdict, image_path|image_sha256,
  role?, findings?}]`, `reviewer?`, `revision?`). Validated by
  `ReviewRecord.ValidateCompletion` (all slides, pixel hashes, current revision);
  partial/stale reviews are rejected. Returns quality `evidence` with
  `inspection_backend: host|manual` and `status`
  (`visually_reviewed_current_revision` | `draft_needs_visual_review`); updates the
  `.authoring.json` manifest's `visual_evidence` (new `reviewer` field) when present.
  `QualityEvidence.approved` now accepts `provider`/`host`/`manual` backends in
  addition to `vision` (heuristic still never approves).

#### Changed

- **`repair_slide` `reduce_items` / `resize_list` fact-loss guard.** Dropping
  pattern-values items that contain a number, unit, negation, or qualifier is
  refused with `code: "semantic_review_required"` and a `next_tool_call`
  proposing `repair_slide` → `split_pattern{path, first}`; pass
  `confirm_semantic_change: true` to override. `split_pattern` accepts an
  optional `path` that splits a `pattern.values` array across two slides.
- **Visual findings carry an optional `bbox`** (`{x,y,w,h}`, fractions of the
  slide) on `inspect_slide_images` findings and `propose_repairs` visual input.
  `propose_repairs` hit-tests it against the generated shape_grid cell bounds
  (patterns are expanded first) and targets the element path
  (`/slides/N/shape_grid/rows/R/cells/C`, also set as `reduce_cell_text`
  `cell_path`) instead of the whole slide; no bbox or no hit keeps the slide path.

### Deck-quality fixes (lanes L4, L6)

#### Added

- **`matrix-2x2` axis direction.** Optional `values.x_low`, `x_high`, `y_low`,
  `y_high` (strings, ≤20 chars; default `"Low"` / `"High"`) label the ends of
  each axis. Both value forms (named quadrants and positional `quadrants`)
  accept them. The axes now render as arrows pointing to the high end (x →
  right, y → up) instead of flat header bars. Additive; existing inputs are
  unchanged apart from the new rendering.

- **`examine_template` exposes the template profile** (go-slide-creator-fw42).
  The report gains a top-level `profile` object (`{template_hash,
  parser_version, role_bindings, diagnostics[]}`) and a per-layout
  `layouts[].profile_geometry` object (`{layout_id, footer_regions[], frame}`).
  `footer_regions[]` are the resolved `dt`/`ftr`/`sldNum` rectangles
  (`{type, x_emu, y_emu, w_emu, h_emu}`, layout first, else master); `frame` is
  the chrome frame the generator renders into — `{canvas, content,
  takeaway_band, source_band, footer_top_emu, has_footer, basis, fits}` with
  `basis ∈ {layout, reference_layout, slide_fallback}`. The profile is built
  once per template content hash (+ parser version) and is the same object the
  generator uses for takeaway/source placement. Additive; no field removed.
- **Fit finding `chrome_band_no_fit`** (go-slide-creator-7m9v). `review`
  action, `fix.kind: swap_layout` (`params.layout_id` = One Content layout).
  Emitted when a slide's takeaway/source band cannot be placed on its layout;
  the band is skipped at render instead of overlapping title/footer chrome.

- **`recommend_visual` template preview refs** (go-slide-creator-aruv). With a
  `template`, `candidates[].example` gains `layout_id` and
  `layout_preview_png_path` — the shipped 320px thumbnail
  (`templates/previews/<template>/<layout_id>.png`, embedded in the binary and
  materialised into the user cache for embedded templates) of the layout the
  candidate renders on. Placeholder candidates now resolve slide types
  (`title`, `section`, `content`, `two-column`, `image`, `blank`) to their
  canonical layout and use the thumbnail as `preview_png_path`
  (`renderer: "template-preview"`, `metadata_only: false`). Additive.

#### Changed

- **Takeaway / source band geometry is layout-derived** (go-slide-creator-7m9v).
  The band spans the layout's body column and sits above its footer
  placeholders (was: a fixed 0.5in slide-percentage margin overlapping the
  master footers). Content placeholders shrink to end above the band. The
  takeaway text is 14pt bold (was 12pt).

### Typography, contrast and readability (deck-quality lane L7)

#### Added

- **Top-level `viewing_mode`** (`"present"` default | `"read"`) selecting the
  readability policy, and the **`TEXT_BELOW_READABLE_MIN`** fit finding
  (`action: review`, `fix.kind: reduce_text`, `fix.params: {strategy, role,
  actual_pt, min_pt, viewing_mode}`) emitted for placeholder autofit (generate)
  and shape_grid cells (fit report / preflight). Schema fingerprint updated;
  SchemaVersion bump left to the merge (coordinator instruction).

- **`TITLE_OVERFLOW` fit finding** (`action: shrink_or_split`, `fix.kind: shorten_title`,
  `fix.params: {current_chars, max_chars, font_pt, min_font_pt}`). Emitted by
  `generate` when a title cannot fit its resolved title placeholder at the
  minimum autofit size, measured with the master's inherited title style.

#### Changed

- **Title checks unified on measured fit.** `validate` title diagnostics,
  the fit report and the generate `quality` score now measure titles against the
  resolved title placeholder and inherited title style. `title_wraps` escalates
  to `action: shrink_or_split` with `fix.kind: shorten_title`,
  `fix.params: {current_chars, max_chars, fit_scale_pct}` when the title only
  fits below the comfort size; `TITLE_OVERFLOW` is also emitted by preflight.
  Validate title warnings now carry `fix.kind: shorten_title` (was
  `shrink_text`) with a measured `max_chars`. Quality-score title issues read
  "title (N chars) only fits its title placeholder at P% …" instead of
  "title too long (N chars, max 60)" when the template is known.
- **Default table styling.** Tables without explicit style fields now render an
  `accent1` bold/`lt1` header, right-aligned detected numeric columns, an
  emphasised `Total`/`Sum` row (bold + top rule) and content-driven row heights.
  No schema change; explicit style fields opt out (see `docs/STYLE_DEFAULTS.md`).
- **textfit glyph widths fixed.** textfit built canvas font faces at
  `fontPt*ptToMM` (canvas takes points), under-measuring every string ~2.83x.
  All measured fit (placeholder autofit, title fit, table cell measurement,
  shape_grid capacity budgets) now uses real widths, so `max_chars` budgets,
  `fit_overflow` / `cell_underfilled` density bands and autofit scales are
  roughly 2.8x stricter than before. The viewing-mode policy no longer acts as
  a shrink floor in `textfit.Calculate`.
- Title placeholders that fit by shrinking now carry the reduced size as an
  explicit run `sz` (plus `lnSpc` when line spacing is reduced) with a bare
  `<a:normAutofit/>`, instead of `<a:normAutofit fontScale=…>`.

## 4.59.0 (2026-09-18)

### Changed

- **Render tools return MCP ImageContent (go-slide-creator-cn3h).**
  `render_slide_image`, `render_slide_image_from_json`, and
  `render_deck_thumbnails` now deliver pixels as native MCP `image` content
  blocks (`image/jpeg`, downscaled to max 1280px wide, quality 80) so the client
  model can actually see the slides. `content[0]` is a compact JSON metadata
  block (also `structuredContent`) with **no base64**: `delivery:
  "image_content"`, per-slide `index`, `path` (content-addressed full-resolution
  PNG, always materialized), `width`/`height`, `content_hash`,
  `image_content_index`, `image_mime_type`, `image_width`, `image_height`.
  `render_deck_thumbnails` hoists the deck-wide `source_hash` / `cleanup` to the
  top level. New input `include_base64_json` (bool, default false) restores the
  legacy base64-PNG-in-JSON envelope unchanged; the CLI render subcommands always
  use it.

- **MCP tool profiles (go-slide-creator-vdxa).** `json2pptx mcp` now takes
  `--tools core|all` (env `JSON2PPTX_MCP_TOOLS`), default `core`. The core
  profile's `tools/list` advertises ~21 tools (cap 23, incl. preview_presentation_plan
  and inspect_slide_images) without `outputSchema` (~69KB incl. the compact closed DeckSpec
  schema, vs ~215KB); `all` advertises the full, unchanged catalogue. Every tool
  is still registered, so non-core tools remain callable by name. New
  `get_capabilities().mcp_tools_available[].in_core_profile` boolean. The tool
  NAME set is unchanged, so the schema fingerprint is unchanged.

### Fixed

- **Stale tool counts and CLI flag in docs (go-slide-creator-og5i).** The
  `get_started` / `make_deck` descriptions, SKILL.md, TOOLS.md, and README no
  longer hard-code tool counts ("45-tool", "37-tool", "40+", "51 tools"), and
  SKILL.md's `json2pptx mcp` line uses the real `--output` flag (not the
  non-existent `-output-dir`). `TestSkillDocCLIFlagsExist` now verifies every
  `json2pptx <cmd> --flag` shown in SKILL.md / TOOLS.md / WORKFLOW.md against the
  command's real flag set, and `TestNoHardcodedToolCounts` rejects new counts.

### Changed (value scale)

- **One 0-100 score scale with an explicit basis (go-slide-creator-n1t7).**
  `generate_presentation.quality.score` / `slide_scores[].score` (CLI
  `--json-output` too) and `render_deck_spec.quality_summary.score` move from
  0.0-1.0 to the shared **0-100** integer-valued scale (a former `0.87` is now
  `87`) and gain `basis: "input"`. `score_deck` gains `basis: "structural"`;
  `auto_repair` / `make_deck` gain `score_basis: "structural"`. Basis enum:
  `input` (static input-JSON heuristics) | `structural` (deterministic rules over
  the generated deck, no pixels) | `rendered` (reserved for pixel-derived
  scores). The `generate_presentation` output schema's stale `quality_score`
  definition (overall/variety/coverage/structure) is replaced by the real shape,
  and `render_deck_spec.quality_summary` now references it. `score_deck` and
  `render_deck_spec` descriptions state exactly what each score measured.
  Agents comparing `quality.score` against 0-1 thresholds must rescale.

## 4.58.0 (2026-05-30)

### Added

- **Semantic compiler MCP tools — the compact `DeckSpec` authoring surface.** Six
  new MCP tools expose `internal/semantic` so an agent can author a NEW deck from
  a compact semantic spec instead of the raw `PresentationInput` model. They are
  thin adapters over the same entry points the `json2pptx semantic` CLI uses, so
  the two surfaces cannot drift.
  - `validate_deck_spec` — validate a `DeckSpec` and return the shared
    `FindingEnvelope` (`{schema_version, tool, subcommand, ok, summary, findings[]}`).
  - `compile_deck_spec` — lower a `DeckSpec` to raw `PresentationInput`. Returns a
    **compact** result (`{ok, slide_count, template, diagnostics[]}`) by default;
    pass `include_compiled_json: true` to also receive the full compiled JSON
    under `compiled_json`.
  - `render_deck_spec` — compile and render straight to a `.pptx`. Returns
    `{ok, success, pptx_path, template, slide_count, content_hash, duration_ms,
    quality_summary, warnings[], diagnostics[], explanation_summary}` — render
    findings are mapped back to the semantic source paths the author wrote.
  - `explain_deck_spec` — project the compiler's planned decisions (archetype,
    template, per-slide kind/role/visual_family/density/pattern/layout) and
    deck-rhythm warnings without compiling or rendering.
  - `list_deck_archetypes` — enumerate deck archetypes
    (`{archetype, summary, default_template, executive}`).
  - `list_slide_kinds` — enumerate slide kinds
    (`{kind, summary, required_fields, typical_fields}`).
  - `spec` accepts a JSON object (the `DeckSpec`) or a raw YAML/JSON string. The
    raw tools (`generate_presentation`, `validate_input`, …) remain available; the
    semantic tools are the recommended default for new decks.
  - The `json2pptx semantic` CLI command group now has MCP parity (the
    `validate|compile|render|explain|schema` subcommands map to these tools), so it
    is no longer classified as CLI-only.

## 4.57.0 (2026-05-22)

### Added

- **Requested-vs-actual quality reporting on `auto_repair` and `make_deck`.** Both
  facades now return an always-present **`quality`** object so an agent can tell
  the inspection regime it *asked for* apart from the one that *actually ran* —
  previously `quality_mode` was request-derived and could claim
  `"deterministic+visual_qa"` even when visual QA was skipped (render tools
  unavailable).
  - `quality` = `{requested, actual, inspection_mode?, fallback_reasons[]?}`.
    `requested` is request-derived (`visual_qa.enabled`); `actual` reflects what
    truly ran and degrades to `"deterministic"` when a requested visual-QA phase
    inspected no slide; `inspection_mode` ∈ {`vision`, `heuristic`, `skipped`};
    `fallback_reasons[]` explains any divergence (render tools unavailable, render
    failure, missing-API-key heuristic fallback).
  - **`quality_mode` is now an alias of `quality.actual`** — it reports the regime
    that ACTUALLY ran rather than the one requested. A requested-but-skipped
    visual-QA phase now reports `quality_mode: "deterministic"` (was
    `"deterministic+visual_qa"`). The value stays within the existing enum, so it
    is schema-shape-compatible, but it is now truthful.
  - `quality` is added to both facade output schemas (always required).

### Changed

- **Malformed `visual_qa` now fails fast.** A *present* but malformed `visual_qa`
  argument (a scalar/array instead of an object, or a wrong-typed field) returns
  an `INVALID_PARAMETER` error instead of silently disabling visual QA, so an
  agent that requested pixel/vision inspection is never told it ran deterministic
  checks without warning (REL-007). An **absent** or `null` `visual_qa` block
  still defaults to deterministic with no error.

Backward-compatible for well-formed callers: existing fresh-run calls gain a
populated `quality` block and a now-truthful `quality_mode`. No new tools;
`PresentationInput`/`Fix.Kind` surfaces and the schema fingerprint are
unaffected.

## 4.56.0 (2026-05-22)

### Added

- **Resumable per-pass state on `auto_repair` and `make_deck`.** Both convergence
  facades now return an always-present **`next_state`** block and accept an
  optional **`resume_token`** argument, so an agent can inspect a partial result
  and continue the loop from where it stopped instead of restarting from scratch.
  - `next_state` =
    `{completion, resumable, resume_token, next_action, passes_run, next_pass?, max_passes, artifact_path, remaining_findings[]}`.
    `completion` ∈ {`converged`, `converged_degraded`, `max_passes_exhausted`,
    `no_progress`, `render_incomplete`} classifies how the loop terminated so a
    partial/degraded result is never mistaken for a clean convergence;
    `remaining_findings[]` (capped at 25) echoes the still-open findings.
  - Passing `resume_token` reloads the saved post-repair deck, accumulated
    `trace`, and content provenance from a per-process session (1-hour TTL) and
    continues at `next_pass` with **continuous global pass numbering — completed
    passes are never re-run**. On resume, `presentation` (auto_repair) / `outline`
    (make_deck) are ignored; `gate` and `max_passes`/`max_repair_passes` may be
    overridden; `base_dir`, `visual_qa`, `allow_degraded_scoring`, and
    `output_filename` are inherited. make_deck preserves its `plan` across the
    resume without re-planning. Unknown/expired tokens return
    `RESUME_TOKEN_NOT_FOUND`; cross-tool tokens return `RESUME_TOKEN_MISMATCH`.
  - `next_state` is added to both facade output schemas (always required);
    `resume_token` is added as an input parameter. `presentation` (auto_repair)
    and `outline` (make_deck) are no longer schema-`required` because a resume
    call does not supply them — they remain required for a fresh run and are
    enforced at the handler.

  Backward-compatible: existing fresh-run calls are unchanged (they simply gain a
  populated `next_state`). No new tools; `PresentationInput`/`Fix.Kind` surfaces
  and the schema fingerprint are unaffected.

## 4.55.0 (2026-05-22)

### Added

- **Guarded path-based template inspection on `examine_template`.** The MCP tool
  previously accepted only `template_name` (resolved through the registered/embedded
  template lookup), so an MCP-only agent could not inspect a newly supplied template
  file until it had been installed in the server's template directory — even though
  the CLI `examine-template` accepts any local path. `examine_template` now accepts
  **one of** two mutually-exclusive forms:
  - `template_name` (string) — registered/embedded template, unchanged default form.
  - `template_path` (string) — a local `.pptx` path, resolved against the new
    optional `base_dir` (string; falls back to the server CWD when absent). The
    resolved file must, after `~`/`$ENV` expansion and symlink evaluation, be a
    regular `.pptx` contained within `base_dir`. A path escaping that allowed root
    (a `..` traversal, or an absolute/symlinked path pointing outside) returns a
    clear `INVALID_PATH` forbidden-path diagnostic; a missing file returns
    `FILE_NOT_FOUND`; a non-`.pptx` extension or a directory returns
    `INVALID_PARAMETER`. Supplying both forms returns `AMBIGUOUS_INPUT`; supplying
    neither returns `MISSING_PARAMETER`.
  - `examine_template` is added to the `get_capabilities` `features.base_dir` list.
  - The report shape is unchanged; its findings envelope already folds the
    validate-template metadata diagnostics. The standalone validate-template verdict
    (the boolean `valid` + capabilities roll-up) and the template-check
    `ConformanceReport` (which the CLI merges into `conformance.json`) remain
    CLI-only — the `cli_only_commands` entries for `validate-template` and
    `template-check` in `get_capabilities` now document the path workflow and this
    limitation.

  Backward-compatible: existing `template_name` calls are unchanged. The schema
  fingerprint and tool-name set are unaffected (input params + a capability list
  entry, not new tools or `PresentationInput`/`Fix.Kind` surfaces).

## 4.54.0 (2026-05-22)

### Added

- **`icon.scale` plumbed through the public JSON surface.** Shape-grid icons
  (`cells[].icon` and nested `shape.icon`, plus the polymorphic pattern icon
  slot) now accept an optional `scale` (`0 < scale <= 1`) that shrinks an icon
  overlaid on a shape; `convertGridCell` and `IconRef.Resolve` copy it into
  `shapegrid.IconSpec`, where `iconOverlayBounds` already honored it (out-of-range
  or unset values fall back to the `0.6` overlay default). Backward-compatible:
  decks without `scale` are unchanged. Schema fingerprint and `schema_version`
  are unaffected (the field lives on `IconInput`, which is outside the fingerprint
  surface).

- **Visual-QA inspection failures surface as diagnostics.** A slide whose visual
  inspection *failed* (an API/transport/decode error or malformed model output
  in `mode: "vision"`, a vision deadline, or an undecodable image in
  `mode: "heuristic"`) was stored only in `SlideResult.error` and dropped by the
  finding projection, so a run where every vision call failed looked like a clean
  inspection with zero findings. `inspect_slide_images` now:
  - Projects each failed slide to an **error-severity** finding —
    `RENDER.VISION_INSPECTION_FAILED`, `RENDER.VISION_TIMEOUT`, or
    `RENDER.HEURISTIC_INSPECTION_FAILED` — carrying the failure mode in
    `evidence.source` and the image in `evidence.image_path`. Because it is
    error-severity, `findings.ok` is now `false` whenever inspection failed, even
    if no visual *defects* were returned.
  - Adds top-level **`failed_slide_count`** (integer) and **`inspection_status`**
    (`complete` | `partial` | `failed`) so an agent can distinguish a clean
    inspection from a backend failure without scanning findings. Both are
    required fields.
- **`auto_repair` / `make_deck` `visual_qa` phase records failed inspections.**
  Each `passes[]` entry adds `failed_slide_count` + `inspection_status`, and the
  `visual_qa` block adds `inspection_complete` (false when any pass had inspection
  failures) and a roll-up `failed_slide_count`. A pass whose inspection failed no
  longer counts its zero actionable findings as a clean convergence: it records a
  `notes[]` entry flagging the result as inconclusive.
- New finding codes `VISION_INSPECTION_FAILED` and `HEURISTIC_INSPECTION_FAILED`
  (both resolvable via `describe_finding`). The existing `VISION_TIMEOUT` and the
  `LIBREOFFICE_TIMEOUT` / `IMAGEMAGICK_TIMEOUT` render codes are now classified
  into the `RENDER` namespace in the finding envelope.

## 4.53.0 (2026-05-22)

### Added

- **Unambiguous publishability status on the `make_deck` / `auto_repair`
  facades.** Both facades returned a successful MCP transport response with a
  `path` even when the deck was an exemplar skeleton or had failed the quality
  gate, conflating transport success, artifact existence, gate status,
  validation status, content provenance, and publishability into fields an agent
  had to infer. Agents could therefore ship an exemplar-filled or gate-failed
  deck because the call succeeded and returned a path. Both responses now carry
  explicit machine-readable status fields (all backwards-compatible additions —
  the prior fields are unchanged):
  - **`publishable`** (boolean) — the single authoritative ship-as-is flag.
    `true` only when the gate passed on complete evidence, the artifact is
    structurally valid, **and** content is author-supplied. Equivalent to
    `blocking_reasons` being empty.
  - **`manual_review_required`** (boolean) — affirmative inverse of
    `publishable`.
  - **`blocking_reasons[]`** — every reason the deck is not publishable; a
    superset of `gate_reasons[]` that also folds in incomplete-evidence and
    exemplar-content causes. Present only when `publishable: false`.
  - **`content_status`** (`author_supplied` | `exemplar_skeleton`) +
    **`uses_exemplar_content`** (boolean) — content provenance. `make_deck`
    always reports `exemplar_skeleton` / `true` and is therefore **never**
    `publishable: true`, no matter how cleanly it scores; `auto_repair` always
    reports `author_supplied` / `false`.
  - **`artifact_status`** (`generated` | `generated_invalid`) and
    **`validation_status`** (`passed` | `passed_degraded` | `failed`).
- `get_started` now surfaces the rendered visual-QA / manual-review branch in
  its `brief` and `revise` notes, and the `make_deck` fast path is advertised as
  a draft skeleton (not a publishable deck).
- Docs (`README.md`, `skills/generate-deck/TOOLS.md`,
  `skills/generate-deck/SKILL.md`) no longer describe exemplar-backed
  `make_deck` output as publishable without qualification.

  Response-shape additions only; the `PresentationInput` schema, MCP tool-name
  set, and `Fix.Kind` vocabulary are unchanged, so the schema fingerprint does
  not move. `schema_version` advances to 4.53.0. (bd `go-slide-creator-33oo`.)

## 4.52.0 (2026-05-22)

### Changed

- **Strict output-validation error envelope now emits an *executable* recovery
  hint.** When `generate_presentation` refuses with `output_validation: "strict"`,
  the structured error envelope previously set `next_tool_call.tool =
  "repair_slide"` with an empty `fixes: []` array. `repair_slide` rejects an empty
  `fixes` array, so an agent that followed the hint verbatim got a second
  `INVALID_PARAMETER`-style error instead of a repair. The envelope now:
  - points `next_tool_call` at **`describe_finding`** with
    `args_template.code` set to the first blocking finding's own code when it is
    in the describe vocabulary, otherwise the umbrella `OUTPUT_VALIDATION_ERROR`
    code (the specific `OPC_*`/`OOXML_*` codes are not individually registered) —
    a call that always satisfies the target tool's schema and resolves to real
    remediation steps;
  - adds `repairable` (always `false` here) and `repair_unavailable_reason`,
    explicitly marking that no executable `repair_slide` call is offered because
    output-validation findings carry no auto-derivable fix params;
  - preserves the full `findings[]` context (`code`, `scope`, `source_path`,
    `slide_index`) so the agent constructs the `repair_slide` directive itself.

  This is a response-shape change to an error envelope only; the
  `PresentationInput` schema, MCP tool-name set, and `Fix.Kind` vocabulary are
  unchanged. `schema_version` advances to 4.52.0. (bd `go-slide-creator-gy8j`.)

## 4.51.0 (2026-05-22)

### Added

- **`placeholder_policy` parameter + `unresolved_placeholder` finding.**
  `plan_deck` skeletons carry `__FILL__` tokens for agent-supplied content. They
  remain structurally valid (`__FILL__` is a non-empty string), so previously an
  agent could generate a publishable deck with leftover tokens and get no
  corrective finding. Both `validate_input` and `generate_presentation` now scan
  every user-visible string (placeholder text, bullets, speaker notes, shape_grid
  cell text, table cells, chart/diagram labels, pattern values) for the token via
  the shared `internal/policy/placeholder` scanner:
  - New `placeholder_policy` parameter (`off` | `warn` | `strict`, default
    `warn`) on `generate_presentation` and `validate_input`; CLI `validate` gains
    `--placeholder-policy`. `warn` reports each token with its JSON path while
    keeping `valid: true`; `strict` promotes them to errors that fail validation /
    refuse generation (the publishable/gated mode); `off` skips the scan.
  - New finding code `unresolved_placeholder` (category `POLICY`, `fix.kind:
    replace_placeholder`), documented in `docs/FIT_FINDINGS.md` and the
    describe-finding registry. Preflight (`generate -preflight`) emits it at
    warning severity in its `POLICY` stage.
  - `get_capabilities.features.placeholder_policy` advertises the ladder.

  These are additive input params and a new finding code; they do not change the
  MCP tool-name set, PresentationInput shape, or `Fix.Kind` repair vocabulary, so
  the schema fingerprint is unchanged. `schema_version` advances to 4.51.0 to mark
  the new surface. (bd `go-slide-creator-737h`.)

## 4.50.0 (2026-05-21)

### Added

- **Read-only discovery on `list_templates` / `skill-info` + `side_effects`
  block.** Template discovery generates layout-preview PNG **cache files** as a
  side effect (when LibreOffice + ImageMagick are present), which an agent in
  read-only planning mode may want to avoid. Two additions make this explicit and
  opt-out-able:
  - `read_only` (MCP `list_templates`) / `--no-preview` (CLI `skill-info`) — when
    set, layout-preview generation is skipped entirely so the call writes no cache
    files. `preview_png_path` is then omitted from `layout_summaries[]` / `layouts[]`.
  - `side_effects` response block (both surfaces) —
    `{preview_cache_writes, read_only, preview_cache_dir, disable_with}`. It
    reports whether this call writes (or could write) preview cache files, the
    cache directory the default mode touches, whether read-only mode was active,
    and the surface-specific opt-out (`read_only=true` / `--no-preview`).

  The `list_templates` tool classification now honestly carries `writes_files:
  true` (its default mode produces PNG cache artifacts); `read_only=true`
  suppresses them. These are additive input params / response fields and a
  classification flag — they do not change the MCP tool-name set, PresentationInput
  shape, or Fix.Kind vocabulary, so the schema fingerprint is unchanged;
  `schema_version` advances to 4.50.0 to mark the new surface.
  (bd `go-slide-creator-had3`.)

## 4.49.0 (2026-05-21)

### Added

- **`cli_only_commands` on `get_capabilities`.** The `get_capabilities` response
  (MCP and `json2pptx capabilities`) now carries a `cli_only_commands[]` array —
  `{name, cli_only_reason}` for every dispatchable CLI command that intentionally
  has no MCP tool. This is the reverse of the per-tool `mcp_only_reason` in
  `mcp_tools_available[]`: an agent that wants a capability absent from the MCP
  catalog can discover whether a CLI command covers it and why it was not exposed
  as a tool. Today the list is `preflight`, `validate-template`, `template-check`,
  and `preview-patterns`.

  A reverse parity gate (`TestEveryCLICommandHasMCPParityOrException`) now fails
  when an agent-facing CLI command (parsed from `main.dispatch()`) lacks both an
  MCP counterpart in the `mcpToCLI` table and a documented `CLIOnlyReason`.
  Server-lifecycle (`serve`, `mcp`) and meta (`version`, `help`) commands are
  exempt. Classifications live in `cliCommandClassifications()` and are kept in
  lockstep with the dispatch switch by `TestCLICommandClassificationCoversDispatch`.

  This is an additive response field and does not change the MCP tool-name set,
  PresentationInput shape, or Fix.Kind vocabulary, so the schema fingerprint is
  unchanged; `schema_version` advances to 4.49.0 to mark the new response surface.
  (bd `go-slide-creator-kq3m`.)

## 4.48.0 (2026-05-21)

### Added

- **`fast_path` on `get_started`.** The `get_started` response (MCP and
  `json2pptx get-started`) now leads with a recommended single-call workflow
  facade alongside the existing manual `sequence`:
  - `fast_path: {tool, when_to_call, falls_back_to[]}` — `make_deck` for
    `task=brief`, `auto_repair` for `task=revise`. `falls_back_to[]` echoes the
    manual primitive tool names in the same response's `sequence`, so the facade
    and the controllable path it collapses stay in lockstep.
  - Omitted for `task=validate-only` (pure diagnostics, no facade).
  - The `brief`/`revise` `notes[]` now explain when to use the facade versus the
    manual primitives.

  This routes cold-start agents to the best-deck path (`make_deck`) first while
  keeping the controllable manual workflow one field away. Discovery surfaces —
  `json2pptx help` (MCP-only tools list), the README MCP tool tables (grouped by
  `phase`), and `skills/generate-deck/TOOLS.md` — were brought into agreement with
  the registered tool set and classification metadata in the same change. Drift
  tests now fail when a registered MCP tool is missing from the README or
  TOOLS.md, when an MCP-only tool is missing from `json2pptx help`, or when the
  `get_started` `fast_path` names an unclassified/non-facade tool.

  This is an additive response field and does not change the MCP tool-name set,
  PresentationInput shape, or Fix.Kind vocabulary, so the schema fingerprint is
  unchanged; `schema_version` advances to 4.48.0 to mark the new response surface.
  (bd `go-slide-creator-hec9`.)

## 4.47.0 (2026-05-21)

### Added

- **Tool classification metadata on `get_capabilities`.** Every entry in
  `mcp_tools_available[]` (and the `json2pptx capabilities` CLI) now carries
  structured classification so agents can distinguish composable primitives from
  opinionated workflow facades without parsing tool descriptions:
  - `kind` — `primitive` (composable building block), `workflow_facade`
    (multi-step orchestration — `make_deck`, `auto_repair`), or `diagnostic`
    (read-only discovery / validation / inspection / scoring / recommendation).
    Tools are classed by primary purpose, not side effects.
  - `phase` — the workflow stage: `discovery`, `plan`, `vary`, `render`,
    `repair`, or `settings`.
  - `mutates_state`, `writes_files`, `render_dependency`, `api_key_dependency` —
    side-effect and dependency flags (default-mode behaviour).
  - `cli_counterpart` — the closest CLI subcommand; `mcp_only_reason` — why a
    tool has no 1:1 CLI command.
  - `primitive_alternatives` — for a `workflow_facade` (and batch convenience
    tools), the lower-level primitives an agent can drive by hand instead.

  These are additive response fields and do not change the MCP tool-name set,
  PresentationInput shape, or Fix.Kind vocabulary, so the schema fingerprint is
  unchanged; `schema_version` advances to 4.47.0 to mark the new response
  surface. Advertised as `features.feature_versions.tool_classification`.

## 4.46.0 (2026-05-21)

### Added

- **`apply_deck_patch` MCP tool.** A pure deck-JSON transform primitive. Accepts
  the full deck plus an ordered `ops[]` list of bounded structural operations and
  returns the patched deck plus validation/preflight findings. It never writes
  files, renders, or mutates server state — it is a primitive, not a workflow
  facade.
  - Operations: `insert_slide` (`index?`, `slide`), `remove_slide` (`index`),
    `replace_slide` (`index`, `slide`), `move_slide` (`from`, `to`),
    `duplicate_slide` (`index`, `to?`), and `replace_field` (`path`, `value`) —
    where `path` is an RFC 6901 JSON Pointer that must already exist (replace
    semantics, never create).
  - The patch is **atomic**: any invalid operation (index out of range, unknown
    op, missing field, JSON Pointer path that does not exist) or a change that
    produces a deck which no longer parses is rejected with a structured error
    envelope and no `patched_deck` is returned.
  - Response shape: `{patched_deck, applied_ops[], findings}` where `findings` is
    the shared [FindingEnvelope](AGENT_DIAGNOSTICS.md) (branch on `findings.ok`).
  - The deck round-trips through a generic JSON tree (numbers preserved), so
    fields the tool does not model survive the patch unchanged.

  Adding a tool changes the MCP tool-name set, so `schema_version` advances to
  `4.46.0` and the schema fingerprint advances to `968385126256b966`. (bd
  `go-slide-creator-wnx3`.)

## 4.45.0 (2026-05-21)

### Added

- **Template-aware `plan_deck`.** The tool gains an optional `template` input
  (a template name; `--template` on the CLI), reusing the same shared support
  helper (`generator.NewTemplateSupportContext` / `Support`) that powers
  template-aware `recommend_visual`. When supplied:
  - Every planned slide carries an additive `template_support` object —
    `{status, reasons[], required_layout}` — for its recommended pattern, and
    every `alternatives[]` entry carries the same object for its candidate
    pattern. `status` is `"supported"` / `"risky"` / `"unsupported"` with the
    same meaning as in `recommend_visual`. Because both tools call the same
    helper, plan_deck and recommend_visual agree for identical template
    constraints (a pattern's status is the same in both).
  - A recommended pattern the template cannot host (`unsupported`) is **swapped**
    for the first feasible (`supported`/`risky`) alternative for that slot, so the
    plan never assigns an impossible pattern when a supported one exists. The
    swapped slide's `rationale` records the substitution.
  - The response echoes the vetted template name in a new top-level `template`
    field.

  Without `template`, slides carry no `template_support` and `template` is
  omitted (template-agnostic plan, unchanged behavior). `make_deck` continues to
  plan template-agnostically (it builds slides from pattern exemplars and never
  surfaces `template_support`). `get_capabilities().features.feature_versions`
  gains `template_aware_plan: "4.45.0"`. (bd `go-slide-creator-452l`.)

## 4.44.0 (2026-05-21)

### Added

- **Template-aware `recommend_visual`.** The tool gains an optional `template`
  input (a template name; `--template` on the CLI). When supplied, every
  returned candidate carries an additive `template_support` object —
  `{status, reasons[], required_layout}` — reporting how well that candidate fits
  the named template:
  - `status` — `"supported"` (the template natively covers the needed
    layout/capability), `"risky"` (producible only via a synthesised/derived
    layout, or close to a body-capacity / content-zone limit), or
    `"unsupported"` (requires an absent canonical/derivable layout).
  - `reasons[]` — why the status applies (which layouts cover it, what is
    synthesised, which capacity/content-zone constraint bites, or what is
    missing).
  - `required_layout` — the canonical layout or derivable capability the
    candidate needs (`"Title Slide"`, `"Two Content"`, `"full-image"`,
    `"grid base"`, …); omitted when there is no specific layout requirement.

  The support assessment is grounded in the template's canonical layouts,
  derivable-layout analysis, font-aware placeholder capacities, and palette
  metadata (`data_palette`). Candidates the template cannot host (or that are
  risky) are **demoted** in the ranking so the top candidate is feasible whenever
  any feasible option exists; the displayed `score` is left untouched. Without
  `template`, candidates carry no `template_support` (template-agnostic ranking,
  unchanged). `get_capabilities().features.feature_versions` gains
  `template_aware_recommend: "4.44.0"`. The shared support helper
  (`generator.NewTemplateSupportContext` / `AnnotateTemplateSupport`) is reused
  by `plan_deck`. Existing callers that omit `template` see no behavior change.
  (bd `go-slide-creator-q5dx`.)

## 4.43.0 (2026-05-21)

### Added

- **Opt-in `visual_qa` mode for `auto_repair` and `make_deck`.** The default
  convergence loop is now truth-labeled `quality_mode: "deterministic"` — it
  scores the deck from static + render-fit findings only and never inspects a
  rendered pixel (no rendering, no API key). Both tools gain an optional
  `visual_qa` object — `{enabled, model?, audit_palette?, max_passes?, density?}`
  — that, when `enabled: true`, runs the agent-grade visual refinement loop
  AFTER the deterministic loop: render thumbnails → `inspect_slide_images` → map
  visual findings to `propose_repairs` → apply → re-render, plus an optional
  deterministic palette ΔE audit (`audit_palette`). The response is labeled
  `quality_mode: "deterministic+visual_qa"`.

  Both responses gain two additive fields:
  - `quality_mode` — `"deterministic"` (default) or `"deterministic+visual_qa"`.
    Always present.
  - `visual_qa` — present only when the mode was requested. Carries
    `{requested, inspection_mode, model, requirements, passes[], palette_audit?,
    notes[]}`. `requirements` reports the API-key env var + presence, default
    model, render dependencies + availability, and a cost note. Each
    `passes[]` entry records `{pass, inspection_mode, thumbnail_paths[],
    visual_findings[], proposed_repairs[], repairs_applied[]}`. Any repairs the
    phase applies are also reflected in `final_presentation`.

  The mode degrades transparently: when libreoffice/magick are missing the phase
  is `inspection_mode: "skipped"` with an explanatory note; when
  `ANTHROPIC_API_KEY` is unset it falls back to the pure-Go heuristic inspector
  (`inspection_mode: "heuristic"`, advisory P3 findings) instead of erroring.
  Only P0/P1 visual findings drive automatic repairs. `get_capabilities` gains a
  `features.quality_modes` block advertising the default mode, the mode list, and
  the opt-in. Existing deterministic-only callers see no behavior change.
  (bd `go-slide-creator-g0ch`.)

## 4.42.0 (2026-05-21)

### Added

- **Collision-free, content-addressed render artifacts.** When a rendered image
  exceeds the inline cap (~200KB), `render_slide_image`, `render_deck_thumbnails`,
  and `render_slide_image_from_json` (incl. `overlay=true`) previously returned a
  mutable path keyed only by slide index (`/tmp/json2pptx-slide-N.png`,
  `/tmp/json2pptx-thumb-N.png`), so a later render of a *different* deck at the
  same index silently overwrote it. The `path` is now content-addressed: the
  filename embeds the PNG's SHA-256 (`<render-cache>/artifacts/slide-<hash>.png`),
  so two decks at the same slide index never collide, and a path is only reused
  when the bytes are byte-identical.

  Each `SlideImage` in these responses gains three additive fields:
  - `content_hash` — SHA-256 of the rendered PNG bytes (stable identity
    regardless of inline-vs-path delivery; populated for both).
  - `source_hash` — identity of the upstream artifact (PPTX file content hash, or
    the caller-supplied cache key for keyed/from-JSON renders).
  - `cleanup` — lifetime/cleanup semantics of the on-disk artifact; set only when
    `path` is returned, empty for inline `png_base64`.

  Artifacts live under the render cache directory and are cleared together by
  cache invalidation or OS temp cleanup. Existing fields are unchanged; agents
  that read only `png_base64`/`path` keep working. (bd `go-slide-creator-3cjg`.)

## 4.41.0 (2026-05-21)

### Added

- **`examine_template` MCP tool.** The reusable template-examination service
  behind the `json2pptx examine-template` CLI subcommand is now also an MCP
  tool. Given a `template_name` (required) and optional `strict` (bool), it
  returns the full `examine.Report` inline as structured content — the same
  report.json shape the CLI writes: `template`, `sha256`, `aspect_ratio`,
  `slide` (dimensions in EMU + inches), `theme` (name, fonts, scheme→hex
  colors), `masters[]`, `canonical_coverage` (the four content-bearing layout
  families, each `{family, present, layouts[]}`), `derivable_layouts[]`, and
  `layouts[]` (per-layout canonical type/family + confidence, asset_base,
  xml_path, derived content_zone, and placeholders[] with role, font-aware
  `font_pt` + `max_chars`, exact bounds in EMU + inches, and z-order), plus a
  findings envelope folding every diagnostic — including
  `TPL.LAYOUT.MISSING_ROLE` for an absent canonical family.

  Unlike the CLI, MCP mode is side-effect-free: it never writes an artifact
  directory and exposes no out param for asset materialisation. Agents that
  need the rendered SVG/PNG artifact tree shell out to the CLI subcommand;
  agents that only need the capability facts read them straight off this
  response. Both surfaces call the shared `examine.Examine` core, so the report
  is identical for the same template and options.

## 4.40.0 (2026-05-21)

### Added

- **`audit_palette` MCP tool.** The deterministic palette-diff audit that
  previously shipped only as the `json2pptx audit-palette` CLI subcommand is now
  also an MCP tool. It renders a PPTX to PNG (via libreoffice + pdftoppm) and
  reports the CIE76 ΔE between every embedded chart/picture region and every
  native solid-filled shape region per slide — catching silent palette drift the
  vision-QA agent cannot see.

  Both surfaces call the shared `auditPalettePPTX` core, so the report is
  identical for the same PPTX and options. Input: `pptx_path` (required;
  validated with the same traversal/extension rules as `read_presentation`),
  plus optional `max_delta_e` (default 5.0), `chroma_min` (default 25), and
  `density` (default 150). Unlike the CLI, the MCP tool exposes no
  output/keep/tmp parameters: render artifacts are written only to an
  auto-removed OS temp directory, so the tool never writes to an agent-controlled
  path.

  Response shape: the full audit report (`pptx`, `slide_count`, `violations`,
  per-slide pic/shape regions and `(pic, shape)` pairs with `delta_e` + `pass`)
  promoted to the top level, plus a `findings` `FindingEnvelope` where each pair
  that exceeds `max_delta_e` becomes one `RENDER.palette_drift` error finding.
  The tool appears in `get_capabilities` (`mcp_tools_available`, `tool_list`)
  with `added_in: "4.40.0"`. (bd `go-slide-creator-a5nv`.)

## 4.39.0 (2026-05-21)

### Added

- **`final_presentation` field on `auto_repair` and `make_deck` responses.**
  Both tools now return the full deck JSON produced after the convergence loop,
  alongside the existing PPTX `path` and `trace[]`. `auto_repair` returns it on
  every successful run (including zero-repair runs, where it equals the resolved
  input); `make_deck` returns the deck it planned, expanded, and repaired.

  The value is the same shape as `generate_presentation`'s `presentation`
  input, so an agent can feed it straight back into `validate_input`,
  `generate_presentation`, or `repair_slide` to keep editing, diffing,
  patching, and re-running quality checks without reconstructing state from the
  trace. The JSON reflects every repair applied during the loop plus the
  up-front asset-path and canonical-layout resolution.

  Output schemas (`outputSchemaAutoRepair`, `outputSchemaMakeDeck`) add
  `final_presentation` to their `properties` and `required` lists. Both PPTX
  outputs and all prior trace/plan/gate fields are preserved. (bd
  `go-slide-creator-5k9k`.)

## 4.38.0 (2026-05-21)

### Added

- **`base_dir` parameter on `score_deck`, `auto_repair`, and `make_deck`.**
  The best-deck MCP tools now resolve relative local-asset paths
  (`image_value.path`, `background.image`, shape-grid `image`/`icon` paths)
  through the same `resolveBaseDir` + `resolveLocalAssetPaths` helpers that
  back `generate_presentation` and `validate_input`. Previously these tools
  ignored `base_dir`, so a deck that referenced a relative asset scored or
  repaired differently (or failed) depending on the server's process CWD.

  Contract is identical to `generate_presentation`: `base_dir` must be an
  absolute path to an existing directory; a relative, missing, or
  non-directory value is rejected with `INVALID_PARAMETER` (`path:
  "base_dir"`) before any per-asset finding; when omitted the server falls
  back to its process CWD (legacy, non-portable). A missing relative asset
  short-circuits with one structured finding per surface
  (`BACKGROUND_IMAGE_PATH`, `IMAGE_PATH`, `ICON_NOT_FOUND`, …).

  `score_deck` resolves before its render+score pass; `auto_repair` and
  `make_deck` resolve once before the convergence loop (paths are rewritten
  to absolute form in place, so every repair pass embeds the same assets).
  `get_capabilities().features.base_dir` now lists all seven tools that
  honour the parameter. (bd `go-slide-creator-5p6e`.)

## 4.37.0 (2026-05-21)

### Changed

- **Direct-label rendering for small-multiseries bar / line / area charts.**
  When a `bar_chart`, `line_chart`, `grouped_bar_chart`, or `area_chart` has
  between `tokens.ChartLegendMinSeries` (2) and
  `tokens.ChartDirectLabelMaxSeries` (4) series, the renderer now suppresses
  the legend and draws inline series labels at the rightmost data point
  (line/area) or above each series's last bar (bar). Above the threshold
  the legend wins because direct labels would collide. Single-series
  charts are unaffected (the legend was already suppressed).

  Existing override:

  - Set `chart_value.style.show_legend = true` to force the legend back on
    even inside the direct-label window. The field was previously a no-op;
    explicit `true` now means "render the legend regardless of series
    count". Omitting the field, or setting `false`, keeps the new default.

  Stacked variants (`stacked_bar_chart`, `stacked_area_chart`) and other
  chart types (`pie`, `donut`, `scatter`, `bubble`, `radar`, `waterfall`,
  `funnel`, `gauge`, `treemap`) keep the previous legend gating. svggen
  mirrors the token thresholds as `svggen.MinLegendSeriesCount` and
  `svggen.MaxDirectLabelSeriesCount`; the parity test
  `TestChartStyleDefaults_Parity_DirectLabelThreshold` in
  `internal/tokens/chart_style_test.go` enforces alignment.
  (bd `go-slide-creator-8ho6`.)

## 4.36.0 (2026-05-21)

### Added

- **`chart_style` block on `ChartSpec` and `DiagramSpec`.** Per-slide
  overrides for the executive chart-style tokens centralised in
  `internal/tokens` (e.g. `ChartHideVerticalGridlines`,
  `ChartLegendMinSeries`). Each field is a `*bool` so an absent override is
  distinguishable from an explicit `false`. Shipping with:
  - `show_vertical_gridlines` — opt back into vertical gridlines on
    Cartesian charts (default off).
  - `show_single_series_legend` — opt back into the legend for a chart with
    one series (default suppressed).

  Example:

  ```json
  {
    "type": "chart",
    "chart_value": {
      "type": "bar",
      "data": {"Q1": 12, "Q2": 18},
      "chart_style": {
        "show_vertical_gridlines": true,
        "show_single_series_legend": true
      }
    }
  }
  ```

  The override is forwarded through `diagramSpecToSVGGen` into
  `svggen.StyleSpec.ChartStyle`; svggen's chart factories copy the values
  onto `ChartConfig.ShowVerticalGrid` / `ChartConfig.ForceLegendSingleSeries`
  and the Cartesian Draw methods honour them. Token defaults stay aligned
  with `internal/tokens.Chart*` so omitting `chart_style` produces
  byte-identical output to prior versions. (bd `go-slide-creator-8a9l`.)

## 4.35.0 (2026-05-20)

### Added

- **`make_deck` MCP tool.** Cold-start facade: one call from an outline to a
  validated PPTX. Internally chains `plan_deck → expand patterns with exemplar
  content → auto_repair` (generate → inspect → repair) until the quality gate
  passes or `max_repair_passes` (default 3, clamped to [1, 10]) is exhausted.
  Replaces the manual cold-start path through 37 individual tools with a
  single call. Inputs: `outline` (required brief), `template` (default
  `midnight-blue`), `style_hints` (optional `slide_budget`, `audience`,
  `accent_strategy`, `must_include`), `gate` (same vocabulary as
  `auto_repair.gate`), and `output_filename`. Response shape:
  `{path, final_score, gate_passed, passes, trace[], gate_reasons[], plan}`
  where `plan.slides[]` exposes the per-slide pattern + role + title so
  follow-up `repair_slide` calls can target specific positions without
  re-planning. Reuses the `auto_repair` convergence-loop core, which has been
  refactored to a shared `runAutoRepairLoop` helper; `auto_repair` callers
  see no behavior change. The shared loop now also calls
  `resolveCanonicalLayoutIDs` on input slides so callers can ship portable
  canonical names (`title`, `blank`, `section`, `closing`) instead of
  template-specific `slideLayoutN` IDs. (bd `go-slide-creator-oji3`.)

## 4.34.0 (2026-05-20)

### Added

- **`auto_repair` MCP tool.** Server-side convergence loop that replaces the
  hand-coded `generate_presentation → score_deck → propose_repairs →
  repair_slides_batch → generate_presentation` chain. Accepts the same
  `presentation` payload as `repair_slide` plus an optional `gate` block
  (`min_score`, `max_p0_findings`, `max_p1_findings`,
  `require_takeaway_on_charts`) and `max_passes` (default 3, clamped to
  [1, 10]). Each pass renders the deck, collects static + render-time fit
  findings, scores deterministically, and applies the top-ranked
  `propose_repairs` directive per affected slide. Stops as soon as the gate is
  satisfied or `max_passes` is exhausted; the final PPTX is written either
  way. Response shape:
  `{path, final_score, gate_passed, passes, trace[], gate_reasons[]}` where
  `trace[i] = {pass, score, findings_count, repairs_applied[]}` records score
  progression. `gate_reasons` is omitted on success and lists every unmet
  criterion on failure so the agent can decide whether to relax the gate or
  escalate. The tool reuses `repair_slide`'s `Fix.Kind` vocabulary verbatim;
  the loop adapter translates BODY_TOO_LONG-style `max_words` params into
  `max_items`/`max_length` based on the actual slide content so the canned
  fit-finding fixes drive real repairs. (bd `go-slide-creator-p7j4`.)

## 4.33.0 (2026-05-20)

### Added

- **`repair_slides_batch` MCP tool.** Atomic multi-slide repair in a single
  call. Accepts the same `presentation` payload as `repair_slide` plus an
  ordered `fixes[]` array where each directive carries its own `slide_index`
  alongside the existing `{kind, params}` shape. Fixes execute in order
  through the same `applyRepairFix` engine, so the per-kind vocabulary is
  identical to `repair_slide`; a failed fix is reported with
  `applied: false` and does not abort the batch. Returns the patched deck,
  one outcome per directive (including the targeted `slide_index`), and a
  fresh deck-wide fit report after every fix has been applied. Halves
  round-trip latency on multi-slide repair plans typically produced by
  `propose_repairs`. (bd `go-slide-creator-5zmk`.)

## 4.32.0 (2026-05-20)

### Added

- **Strict XML safety check for remote SVGs.** `ResolveSVG` now parses
  downloaded bytes with `encoding/xml` (Strict, Entity=nil) before they reach
  the cache. The previous "starts with `<svg` or `<?xml`" prefix check is
  replaced with three structured codes: `SVG_INVALID_ROOT` (well-formed XML
  whose root is not `<svg>`), `SVG_UNSAFE_XML` (any `<!DOCTYPE …>` or
  `<!ENTITY …>` declaration — the carriers for XXE / billion-laughs /
  external-entity expansion), and `SVG_PARSE_ERROR` (malformed XML, empty
  payload, or content that does not start with `<`). Diagnostics emitted via
  `urlFetchDiagnostic` propagate the specific SVG code instead of the
  generic `URL_FETCH_FAILED` when the failure is a content-validation
  failure rather than a transport error. (bd `go-slide-creator-9vl8`.)
- **`preview_icon` MCP tool (CLI: `json2pptx preview-icon`).** Renders a single
  `IconInput` (bundled name, custom `.svg` path, HTTPS URL, or inline `svg_data`)
  to SVG bytes plus a base64 PNG without building a full deck. Lets agents
  verify an icon spec — including a custom-SVG path or a recolored bundled
  icon — before committing it to a slide, instead of round-tripping through
  `generate_presentation` + `render_slide_image`. Response carries `svg_data`,
  `png_base64`, `alt`, `source_kind` (`bundled` / `path` / `url` / `inline`),
  and `qualified_name` for bundled icons. The `fill` override is honored for
  bundled, path, and URL sources; for inline `svg_data` it is ignored with a
  warning (the agent supplies pre-styled markup). Path-based calls honor
  `base_dir` for relative path resolution. Failure codes:
  `ICON_BUNDLED_NAME_UNKNOWN` (with suggestions), `ICON_NOT_FOUND`,
  `ICON_PATH_EXT_INVALID`, `URL_FETCH_FAILED`, `INVALID_PARAMETER`. (bd
  `go-slide-creator-33el`.)

## 4.31.0 (2026-05-20)

### Changed

- **Numeric table column headers right-align by default.** When a column declares
  `column_types` of `number`, `currency`, `percent`, or `delta`, the header cell
  now paragraph-aligns right to match the data cells underneath. Previously the
  header inherited the default left alignment, producing the "Revenue" header
  drifting over a column of right-aligned dollar figures — a consulting-table
  anti-pattern. Headers follow the same `column_types`-wins-over-
  `column_alignments` precedence already used by data cells. Centralised in
  `internal/tokens.TableNumericHeaderAlignRight` per executive chart-style
  defaults (bd `go-slide-creator-bla5`).
- **Chart SVGs now render with tabular figures.** Every chart SVG emitted by
  `svggen` includes a top-level `text{font-variant-numeric:tabular-nums;}` rule
  so columns of numeric tick labels and value labels line up vertically.
  Non-numeric text is unaffected (the CSS property only swaps digit glyphs).
  Renderers without tabular figure support (or fonts that lack them) fall back
  to proportional digits silently. Centralised in
  `internal/tokens.ChartTickLabelTabularNums`; the parity test in
  `internal/tokens/chart_style_test.go` asserts the rule survives in rendered
  output.

## 4.30.0 (2026-05-20)

### Added

- **`describe_finding` MCP tool** — given a single finding code, returns
  `{code, summary, severity, when_emitted, remediation_steps[], example_before,
  example_after, related_codes[]}` so agents can resolve any unfamiliar finding
  code in one extra tool call without scanning `docs/FIT_FINDINGS.md` or the
  SKILL.md tables. Backed by a single `patterns.FindingMeta` registry whose
  coverage of `AllFitFindingCodes()` is enforced by
  `TestFindingMetaCoversAllSentinelCodes` — new sentinel codes added to
  `internal/patterns/errors.go` without a metadata entry fail the build.
  Unknown codes return a structured error whose `fix.params.allowed` enumerates
  the known vocabulary so the agent can self-correct without a second tool call.

## 4.29.0 (2026-05-20)

### Changed

- **`output_validation` defaults to `strict`** on both `generate_presentation`
  (MCP) and `json2pptx generate` (CLI; `--output-validation`). Every successful
  generate response now implies a clean OPC + OOXML pass. Override with
  `output_validation: "warn"` or `"off"` only when intentionally skipping the
  zero-needs-repair guarantee. Existing examples/*.json continue to generate
  cleanly under the new default.

### Added

- **`next_tool_call` on strict output-validation error envelopes** — when
  `generate_presentation` refuses with `output_validation: "strict"`, the
  structured error envelope now carries `next_tool_call.tool = "repair_slide"`
  and `args_template.slide_index` populated when all blocking findings pin to
  one source slide (otherwise `-1`). The `fixes` array is empty because
  output-validation codes don't share a canonical fix kind; agents inspect each
  finding's `code` and `scope` to choose the right `repair_slide` directive.

## 4.28.0 (2026-05-19)

### Fixed

- **MCP handlers now expand `structure` payloads** — `generate_presentation`,
  `validate_input`, `preview_presentation_plan`, and `score_deck` previously
  rejected structure-only payloads at the `len(slides) == 0` boundary check
  because they never ran the CLI's expansion step. They now apply
  `structure → flat slides` immediately after `applyDefaults` /
  `resolveInputNamedSettings`, exactly matching CLI behavior.
- **`score_deck` no longer silently downgrades `mode:"with_heuristics"`** —
  previously the handler accepted `with_heuristics`, ran the deterministic
  path anyway, and stamped `mode_used:"deterministic"` on the response, which
  let agents believe a stronger gate had run. The handler now returns
  `IsError=true` with diagnostic code `UNSUPPORTED_MODE` and a `next_tool_call`
  pointing at `inspect_slide_images` (the canonical vision-based QA tool).
  Unrecognized mode values also return `UNSUPPORTED_MODE`. Agents that want
  vision-based visual QA must call `inspect_slide_images` directly on
  rendered thumbnails.

### Added

- **`STRUCTURE_AND_SLIDES`** boundary diagnostic (severity error, path
  `structure`) — emitted when a payload sets both `structure` and top-level
  `slides`. Includes `fix.kind = "remove_field"` with `params.field = "slides"`.
- **`INVALID_STRUCTURE`** boundary diagnostic (severity error, path
  `structure`) — emitted when `expandStructure` fails (empty sections,
  missing section title, section with no slides, etc.). Includes
  `fix.kind = "fix_structure"` with the underlying error message in
  `params.error`.

## 4.27.0 (2026-05-19)

### Added

- **`plan_deck` per-slide skeleton + suggested-pattern triplet** — every
  `slides[]` entry in the `plan_deck` response now carries three new fields:
  `suggested_pattern` (first-choice pattern, currently identical to
  `recommended_pattern`), `suggested_pattern_fallback` (second choice drawn
  from `alternatives[0]`, omitted when no alternative exists), and
  `skeleton` (a partial `SlideInput` JSON object with the sentinel string
  `__FILL__` substituted for every agent-supplied text leaf).

  Numeric and boolean leaves inside `skeleton.pattern.values` are preserved
  so structural defaults (grid dimensions, flags) survive the round-trip.
  The skeleton always carries `layout_id`, a single `title` content entry,
  and a `pattern` envelope (`name` + `values`). `__FILL__` is a non-empty
  string, so the skeleton validates as-is with `validate_input` without
  requiring a special flag. Agents copy the skeleton and replace tokens
  rather than re-deriving slide structure from the prose `content_seed`,
  which was the primary source of pattern mis-selection errors observed in
  practice.

  `skeleton` is omitted when the recommended pattern does not implement
  `patterns.Exemplar` (those patterns fall back to the longer
  `show_pattern` → populate-values path documented in
  `docs/api/plan_deck.md`).

## 4.26.0 (2026-05-19)

### Added

- **`slide.takeaway`** — first-class slide field for the "so what" headline.
  Renders as bold dark-gray (12pt) text in the lower band of the slide,
  above the source-note row. Accepted by both `SlideInput` (typed JSON
  path) and the legacy `JSONSlide` shape. Propagates through
  `generator.SlideSpec.Takeaway`, the singlepass render context, and the
  output writer so it survives a `repair_slide` round-trip via
  `cloneSlideForRepair`.
- **`takeaway_missing` finding code** (`patterns.ErrCodeTakeawayMissing`,
  sentinel `patterns.ErrTakeawayMissing`) — emitted by `validate_input`
  and the CLI dry-run with severity `warning`, action `review`, when a
  slide carries chart or matrix content but leaves `takeaway` empty.
  "Chart or matrix content" means a `chart` content item, a `diagram`
  whose `diagram_value.type` is chart-shaped (bar/line/area/scatter/etc.),
  or a pattern whose `name` starts with `matrix-`. Documented in
  `docs/FIT_FINDINGS.md`.

## 4.25.0 (2026-05-19)

### Added

- **Canonical icon identifiers in `list_icons` / `icons list --json`** — each
  `sets[]` entry now also returns an `icons` array of
  `{name, qualified_name}` objects. `qualified_name` is always in
  `"<set>:<name>"` form (e.g. `"filled:chart-pie"`, `"outline:chart-pie"`)
  and is the canonical authoring token to drop directly into `icon.name`.
  This removes the inference burden agents previously carried for filled
  icons, where bare names resolved to the outline set. The legacy
  `sets[].names` bare-name array is unchanged and remains supported. CLI
  table output (`json2pptx icons list`) now prints qualified identifiers.

## 4.24.0 (2026-05-19)

### Added

- **`propose_repairs` MCP tool** — translates structured findings into a
  ranked list of `repair_slide` fix directives without mutating the deck.
  Accepts both fit-finding shapes (`{path, code, action, fix:{kind,params}}`)
  and visual QA finding shapes (`{slide_index, category, severity,
  suggested_fixes?}`); mixed input is supported. The tool resolves each
  finding to a target slide, selects candidate fix kinds (preferring
  `finding.fix` > `finding.suggested_fixes` > `visualqa` category mapping),
  and returns `{slides: [{slide_index, finding_count, directives, batch_tool_call}],
  unmapped, summary}`.

  Each directive carries `{kind, params, rank, source:{type, code|category,
  severity, action, path, message}, tool_call}` — the `tool_call` is a
  ready-to-invoke `repair_slide` payload, and each per-slide `batch_tool_call`
  bundles all directives for that slide into a single `repair_slide`
  invocation. Directives are sorted by action rank
  (`refuse > shrink_or_split > review > info`) and severity (`error|P0 >
  warning|P1 > info|P2 > P3`). Findings with no repair mapping
  (`image_quality`, `aspect_ratio`, `border_style`, or fit-finding kinds
  outside the `repair_slide` vocabulary like `adopt_pattern`) appear under
  `unmapped[]` with a stable `reason` code.

  MCP-only — CLI users translate findings to fixes manually and call
  `json2pptx repair`.

## 4.23.0 (2026-05-19)

### Changed

- **`svggen-mcp` diagnostic codes normalized to SCREAMING_SNAKE_CASE.**
  `render_diagram` and `validate_diagram` previously emitted lowercase_snake
  codes (`required`, `invalid_type`, `invalid_value`, `unknown_diagram_type`,
  `render_failed`, `parse_failed`, …). They now emit the SCREAMING_SNAKE
  equivalents (`REQUIRED`, `INVALID_TYPE`, `INVALID_VALUE`,
  `UNKNOWN_DIAGRAM_TYPE`, `RENDER_FAILED`, `PARSE_FAILED`, …) so an agent
  branching on `diagnostic.code` can share string-equality dispatch across
  `json2pptx-mcp` (which has always emitted SCREAMING_SNAKE — `MISSING_PARAMETER`,
  `INVALID_JSON`, `TEMPLATE_NOT_FOUND`, `UNKNOWN_PATTERN`, …) and `svggen-mcp`.

  `get_capabilities.deprecations` carries the legacy → canonical mapping as
  entries shaped `{path: "diagnostic.code:<legacy>", replacement: "<CANONICAL>"}`,
  so agents that branched on the old casing can look up the new code without a
  doc round-trip during the deprecation window.

## 4.22.0 (2026-05-19)

### Added

- **Pagination on `list_templates`, `list_patterns`, `list_icons`** — all
  three discovery tools now accept optional `cursor` (opaque continuation
  token) and `page_size` (default 50, clamped to [1, 200]). Responses
  always echo `total_count` and `page_size`; `next_cursor` is present
  only when more entries remain. Invalid cursors / page_size values
  surface as structured `INVALID_PARAMETER` errors.

  `list_templates` is backward compatible: the new fields are added
  alongside the existing top-level wrapper (`tool`, `templates`,
  `supported_types`, `input_formats`, `output_formats`).

  `list_patterns` and `list_icons` previously returned bare JSON arrays;
  responses are now wrapped envelopes:

  - `list_patterns`: `{groups: [...], total_count, page_size, next_cursor?}`.
    Categories are rebuilt per page in the canonical order
    (`data-display`, `narrative`, `structural`, `hero`); a category only
    appears on a page when it has at least one pattern in that slice.
  - `list_icons`: `{sets: [...], total_count, page_size, next_cursor?}`.
    Each per-set `count` reflects names on the current page; use
    `total_count` for the post-filter corpus total. The optional `set`
    and `search` arguments are honored before pagination.

  Agents that previously parsed the bare-array shape from `list_patterns`
  / `list_icons` must switch to the wrapped envelope.

## 4.21.0 (2026-05-19)

### Added

- **`overlay` parameter on `render_slide_image_from_json` / `--overlay`
  on `render-slide-from-json`** — when true, composites a diagnostic
  overlay on top of the rendered PNG: `shape_grid` cell rectangles
  (labelled `r,c`), density-band tints driven by the most severe attached
  fit finding (`info`=blue, `review`=amber, `shrink_or_split`=orange,
  `refuse`=red, semi-transparent), and per-cell severity badges
  (`INF`/`REV`/`SHR`/`REF`). Off-cell findings stack as small badges in
  the top-right corner.

  The base LibreOffice raster is still produced and cached as before;
  the overlay is composited on top per call (cheap given the cached
  base). Slides with no cells and no findings render without the overlay
  step. Large composites (>200 KB) get written to a stable on-disk path
  prefixed `json2pptx-slide-overlay-<key>.png`; smaller results return
  inline as `png_base64`. Errors during overlay generation surface as
  `OVERLAY_FAILED`.

  Use case: agents iterating on a single slide's design can *see*
  the diagnostic visually instead of cross-referencing finding JSON
  pointers against the raster.

## 4.20.0 (2026-05-19)

### Added

- **`preview_slide_wireframe` MCP tool / `preview-wireframe` CLI
  subcommand** — render an annotated wireframe of one slide's resolved
  plan as SVG and/or base64 PNG, without LibreOffice or ImageMagick.
  Reuses the same plan resolver as `preview_presentation_plan` and
  renders in-process via `svggen`.

  Wireframe shows: the slide frame, layout placeholders (dashed blue),
  `shape_grid` cells (labelled with row/col/kind/dimensions), occupancy
  %, per-cell fit-finding badges (severity-coded `REF`/`SHR`/`REV`/
  `INF`), and a footer strip for off-cell findings.

  Inputs mirror `preview_presentation_plan` (`presentation` JSON) plus
  required `slide_index` (0-based) and optional `format` ∈
  {`svg`, `png`, `both`} (default `both`) and `width_px` (default 960,
  clamped 320..2400).

  Response: `{index, svg, png_base64, width, height, cell_count,
  placeholder_count, finding_count, layout_id, layout_name, slide_type,
  warnings, errors}`.

  Use case: fast visual sanity-checks before paying for a full
  `generate_presentation` + `render_slide_image` round-trip. Pure-Go, no
  shell-outs.

## 4.19.0 (2026-05-19)

### Added

- **`render_slide_image_from_json` MCP tool / `render-slide-from-json` CLI
  subcommand** — render a single slide directly from its JSON definition + a
  template name, without first calling `generate_presentation` on the entire
  deck. Returns the same image envelope as `render_slide_image`
  (`{index, png_base64?, path?, width?, height?, size_error?}`).

  Designed for tight single-slide design-iteration loops: edit the slide
  JSON, see the rendered PNG, repeat. Avoids `O(N)` cost on deck size when
  iterating one slide.

  Behind the scenes the tool wraps the slide into a synthetic single-slide
  deck, generates a temp PPTX, and rasterizes via LibreOffice + ImageMagick.
  The intermediate PPTX is discarded after rendering. Results cache by
  `sha256(slide_json || template_content_hash)` + density, so the cache
  identity is the upstream design — not the (potentially non-deterministic)
  PPTX file content. Pass `force=true` to bypass the cache.

  Required params: `slide` (object), `template` (string). Optional:
  `density` (number, 50-300, default 100), `force` (boolean, default false).

  Error codes mirror `render_slide_image`: `MISSING_PARAMETER`,
  `TEMPLATE_NOT_FOUND`, `INVALID_JSON`, `GENERATION_FAILED`,
  `LIBREOFFICE_UNAVAILABLE`, `IMAGEMAGICK_UNAVAILABLE`, `RENDER_FAILED`.

## 4.18.0 (2026-05-19)

### Added

- **Standardized `get_capabilities` envelope across `json2pptx-mcp` and
  `svggen-mcp`** — both servers now expose the same shape so agents can
  detect cross-server drift with one parse path. The shared fields are:
  - `tool_list: [{name, description}]` — full tool catalog with the
    description string each tool advertises via `mcp.WithDescription`.
  - `registry: {charts: [], diagrams: [], patterns: []}` — canonical names
    grouped by category. `svggen-mcp` leaves `patterns` empty (it owns no
    pattern engine); `json2pptx-mcp` populates all three from the same
    sources `vocabularies` already exposes.
  - `vocabularies: {fix_kinds, finding_codes, ...}` — `svggen-mcp` newly
    exposes this block with the chart-finding remediation enum
    (`align_series`, `truncate_or_split`, `replace_value`, `explicit_scale`,
    `reduce_items`, `increase_canvas`) and the `chart.*` finding codes its
    renderer can surface. `json2pptx-mcp`'s richer vocabularies block is
    unchanged.
  - `deprecations: [{path, replacement, removed_in?}]` — `json2pptx-mcp`
    adds this alias for the existing `deprecated_fields` list; the two
    arrays carry identical content.
  `json2pptx-mcp`'s existing rich fields (`mcp_tools_available`, `runtime`,
  `changelog_url`, `tool_version`, `error_codes`, `deprecated_fields`) and
  `svggen-mcp`'s `chart_types` / `diagram_types` arrays remain in place for
  backwards compatibility; new agent code should prefer the standardized
  fields.

## 4.17.0 (2026-05-19)

### Added

- **`inspect_slide_images` heuristic fallback** — when `ANTHROPIC_API_KEY` is
  unset, the tool no longer fails with `INSPECT_DISABLED`. Instead it runs a
  deterministic pure-Go pass over the slide images that flags:
  - `missing_content` — slide is effectively blank
  - `text_overflow` — one of the 1%-wide edge bands contains a meaningful
    fraction of non-background pixels
  - `aspect_ratio` — image dimensions deviate from 16:9 or 4:3
  All heuristic findings are advisory (severity `P3`).
- **`Report.mode` field** — `"vision"` when results came from the Claude
  vision API, `"heuristic"` when they came from the offline fallback.
- **`Finding.source` field** — `"vision"` or `"heuristic"`, propagated from
  whichever backend produced the finding. Agents that want vision-only
  results should filter on this field.
- Output schema for `inspect_slide_images` documents both new fields.

### Behavior change

- The `INSPECT_DISABLED` error envelope is no longer emitted by
  `inspect_slide_images`. Callers that branched on it should now branch on
  `report.mode == "heuristic"` instead. The error code remains in
  `internal/diagnostics/codes.go` for potential future use but is unused on
  the inspect path.

## 4.16.0 (2026-05-18)

### Added

- **Nested pattern and sub-grid on `GridCellInput`** — `GridCellInput` gains
  two new fields that let a grid cell host a recursively-rendered nested
  layout:
  - `pattern` accepts a `PatternInput` payload (the same shape used at the
    slide level). At resolution time the pattern is expanded into a
    `ShapeGridInput` and rendered inside the cell rectangle (with a small
    4pt inset so the nested grid does not visually butt up against the
    parent cell edges). Accent inheritance follows the deck's
    `accent_strategy` — the same `ExpandContext` (slide index, section
    index) is reused for the nested pattern.
  - `grid` accepts a raw `ShapeGridInput` (recursive) for cases where the
    nested layout is hand-crafted rather than pattern-driven.
  Both fields are mutually exclusive with each other and with the cell's
  other payload keys (`shape`, `table`, `icon`, `image`, `diagram`,
  `composite`). At resolution time, cells hosting a nested grid become
  bounds-only `CellKindSubGrid` placeholders in the parent's `ResolvedCell`
  list; the renderer emits no XML for the placeholder, and the nested
  shapes/icons/images are appended to the parent result. The nested cells
  themselves are also exposed on `ShapeGridResult.Cells` so overlay
  anchor_cell lookups and fit-finding collectors can introspect them. This
  unblocks the agent workflow of dropping a `kpi-3up` into a `matrix-2x2`
  quadrant or an `icon-row` into a `strategy-house` foundation row without
  switching to the slide-level `compose` envelope. Closes
  `go-slide-creator-f1ic.9`.

## 4.15.0 (2026-05-18)

### Added

- **Slide-level `overlays` field on `SlideInput`** — `SlideInput` gains a new
  `overlays: []OverlayShape` field for free-floating shapes rendered on top
  of the slide's grid (or as standalone shapes on slides with no grid).
  Each `OverlayShape` has a `kind` of `"arrow"`, `"line"`, or `"badge"`, plus
  `from`/`to` endpoints expressed either as `{x, y}` percentages of slide
  width/height or as `{anchor_cell: {row, col, at}}` references that resolve
  to a named point on a grid cell (`center`, `top-left`, `top`, `top-right`,
  `right`, `bottom-right`, `bottom`, `bottom-left`, `left`). Arrows emit a
  `straightConnector1` with a triangle `tailEnd`; lines omit the arrowhead;
  badges emit a `roundRect` with optional centered text. Overlays render
  *after* the grid so they always appear on top. This unblocks the agent
  workflow of drawing cross-cell arrows on a 2x2 matrix, floating roof
  badges over strategy-house tiers, and standalone callout pointers without
  abusing `GridOverlayInput` (which is image-only) or `ShapeSpecInput.Icon`
  (which is single-cell). Closes `go-slide-creator-f1ic.10`.

## 4.14.0 (2026-05-18)

### Added

- **Composite stack cell on `GridCellInput`** — `GridCellInput` gains a new
  `composite` payload that bundles a native text shape (`text`) and an embedded
  sub-diagram (`sub_diagram`) inside a single grid cell. The cell is split
  vertically into two halves; `split: "top" | "bottom"` chooses which half
  hosts the text shape (default `"top"`) and `ratio` (a float in the open
  interval (0,1), default 0.5) controls the fraction of cell height allocated
  to the text portion. A composite cell expands at resolution time into two
  ResolvedCells sharing the same `(row,col)` index, so downstream consumers
  (renderer, accent-bar logic, connector targeting) treat the pair as one
  logical cell. Composite is mutually exclusive with the legacy payload keys
  (`shape`, `table`, `icon`, `image`, `diagram`); the validator emits a
  dedicated error listing the conflicting keys. This eliminates the
  agent-side hack of splitting every KPI into ≥2 adjacent cells with
  hand-tuned spans when stacking a number on top of a sparkline. Closes
  `go-slide-creator-zg8q.5`.

## 4.13.0 (2026-05-18)

### Added

- **Diagram segments on `ComposeInput`** — `SegmentInput` gains a third XOR
  alternative alongside `pattern` and `compose`: `diagram: types.DiagramSpec`
  carries a standalone svggen-rendered chart or diagram. Diagram segments
  synthesize a single-cell grid that participates in the parent merge
  identically to a pattern-expanded grid, so `compose.direction` +
  `size_pct` + `gap` drive placement and the gutter rhythm is unified across
  pattern and diagram segments. This is the canonical way to let a native
  pattern (e.g. `pyramid`, `kpi-3up`) coexist with an svggen visual
  (e.g. `process_flow`, `bar_chart`) on the same slide without flattening
  the pattern through a single cell. Diagram segments count toward
  `max_leaf_patterns` (they consume slide real-estate the same way pattern
  segments do). Capability descriptor advertises this via the new
  `get_capabilities().features.compose.supports_diagram_segments = true`
  flag. Closes `go-slide-creator-zg8q.6`.

## 4.12.0 (2026-05-18)

### Added

- **Envelope-level banner and callout on `ComposeInput`** — `ComposeInput`
  gains two new optional fields, `banner: BannerSpec` and `callout: PatternCallout`,
  which render full-width decoration bands respectively above and below the
  merged grid without consuming a segment slot. `BannerSpec` mirrors
  `PatternCallout` (`text`, optional `emphasis`, optional `accent`); the
  banner defaults to bold light text on the requested accent (`accent1` if
  unset). This lets agents add a Strategy-House-style header to arbitrary
  compose arrangements instead of spending a segment slot on a faux-banner
  pattern like `pull-quote`. Validation rejects `banner` when the first
  segment's pattern is itself banner-leading (currently `strategy-house` and
  `pull-quote`) to prevent duplicate banners. Preview metadata
  (`expanded_compose.segments[].row_range`) is offset to account for the
  banner/callout rows so segment-row mapping stays accurate. Closes
  `go-slide-creator-f1ic.11`.

## 4.11.0 (2026-05-18)

### Added

- **Compose envelope MCP discovery** — `recommend_visual` now emits candidates
  with `category == "compose"` when the intent contains a multi-pattern keyword
  ("side by side", "panels and quote", etc.) or when the top two pattern
  candidates declare mutual `PatternTaxonomy.composes_with` affinity. Each
  compose candidate carries `placement.composable_with` populated with the
  specific pair of sibling pattern names, so agents can drop them straight into
  a `ComposeInput.segments[]` without a second discovery call. Capability gate
  added at `get_capabilities().features.compose_envelope = true` (mirrors the
  pre-existing detailed `features.compose` struct). `skill-info` JSON now
  surfaces a top-level `compose` section with cap values and two worked example
  envelopes (vertical and horizontal). Closes `go-slide-creator-f1ic.5`.

- **`get_started` MCP tool / `json2pptx get-started` CLI subcommand** —
  first-call discovery returning an ordered MCP-call sequence keyed to the
  agent's stated task. Accepts an optional `task` parameter:
  - `"brief"` (default): `get_capabilities → list_templates → plan_deck →
    recommend_visual → preview_presentation_plan → generate_presentation →
    score_deck`
  - `"revise"`: `get_capabilities → read_presentation →
    preview_presentation_plan → repair_slide → generate_presentation →
    score_deck`
  - `"validate-only"`: `get_capabilities → list_templates → validate_input →
    preview_presentation_plan`
  Each step carries a one-line `when_to_call` hint. The response also echoes
  `available_tasks` so agents can discover the supported scopes. Unknown task
  values fall back to `"brief"`. Closes `go-slide-creator-lweh.11`.

## 4.10.0 (2026-05-18)

### Added

- **`get_capabilities().features.compose`** — surfaces the compose envelope
  capabilities so agents can discover the segment cap without reverse-engineering
  it from error messages. Returns `{max_segments: int, directions: [string],
  supports_smart_compose: bool}`. `max_segments` is bumped from 4 → **8** in
  this release; for larger arrangements nest a compose envelope inside a
  segment (see `go-slide-creator-f1ic.2`). The validator's error message also
  now points agents at this capability and the nested-compose escape hatch.
  Closes `go-slide-creator-f1ic.3`.

## 4.9.0 (2026-05-18)

### Added

- **`score_candidates` MCP tool** — predicts per-slot deterministic scores for
  alternative slide_json candidates without rendering. Accepts `presentation`,
  `slide_index`, and `candidates[]` (each a slide_json). For each candidate it
  substitutes at `slide_index`, runs `collectFitFindings` (no tempdir, no
  generation), and returns a combined score = `slide_score - rhythm_penalty`
  clamped to [0, 100]:
  - `slide_score`: 100 minus the sum of fit-finding severity weights for the
    target slide (occupancy findings such as `pattern_underfilled` and
    `pattern_overcrowded`, contrast preflight, text overflow, table preflight,
    etc.).
  - `rhythm_penalty`: 5 if the candidate would form a length-2 pattern run at
    that position, 15 if length-3+, 0 otherwise.
  Candidates are returned sorted best→worst with stable tiebreak by input
  index. Closes go-slide-creator-lweh.6.

## 4.8.0 (2026-05-18)

### Added

- **`expand_patterns` MCP tool** — batch, content-aware variant of
  `expand_pattern`. Accepts `names[]`, a single `theme_template`, and a
  per-pattern `content` map (`{patternName: {values, overrides?,
  cell_overrides?, bounds?, max_height_pct?}}`) and returns each candidate's
  full expansion + occupancy + `cell_budgets[]` + `capacity_warnings[]` +
  `layout_suggestions[]` under a SINGLE template load. Patterns omitted from
  `content` fall back to exemplar values and are flagged via
  `used_exemplar=true`. Per-pattern validation/expansion failures surface as
  per-entry `error` objects without aborting the batch, so agents can compare
  N candidates head-to-head against their real content in one round-trip
  instead of N. Closes go-slide-creator-lweh.7.

## 4.7.0 (2026-05-18)

### Added

- **`inspect_slide_images` MCP tool** — first-class entry point to the
  Claude-vision visual QA agent. Accepts an array of rendered slide images
  (filesystem path or base64-encoded PNG) plus optional per-slide metadata,
  and returns a structured `visualqa.Report` with per-slide findings.
  Each finding includes `suggested_fixes[]` pre-mapped to `repair_slide`
  fix kinds via `SuggestedFixesForCategory`, so agents can pipe findings
  directly into `repair_slide` `{kind: "autofix_visual", params: {category}}`.
  Requires `ANTHROPIC_API_KEY` on the server; returns `INSPECT_DISABLED` when
  unset.
- **`INSPECT_DISABLED` error code** — emitted by `inspect_slide_images` when
  the Anthropic API key is not configured.

## 4.6.0 (2026-05-08)

### Added

- **`validate_presentation_output` MCP tool** — validates a generated PPTX file
  using the unified output-validation suite (OPC package integrity + OOXML
  content checks). Returns structured findings with provenance metadata.
- **`output_validation` parameter** on `generate_presentation` — staged policy
  for post-generation PPTX validation: `off` (default, skip), `warn` (include
  findings in response), or `strict` (fail generation with diagnostics envelope
  if blocking findings exist).
- **`--output-validation` CLI flag** on `generate` subcommand — same semantics
  as the MCP parameter.
- **`output_validation_findings`** response field on `generate_presentation` —
  populated when `output_validation` is `warn` or `strict`.
- **`output_validation` feature flag** in `get_capabilities` features — lists
  supported policy values (`off`, `warn`, `strict`).
- **`OUTPUT_VALIDATION_ERROR` error code** — emitted when output validation
  infrastructure fails (distinct from blocking findings in strict mode).

## 4.5.0 (2026-05-07)

### Added

- **`PatternInput.bounds`** — explicit `GridBoundsInput` override (x, y, width,
  height as percentages) constraining the expanded grid to a sub-region of the
  layout area. Fixes density math for patterns that don't fill full content area.
- **`PatternInput.max_height_pct`** — convenience alias that constrains grid
  height to a percentage of the content area (equivalent to
  `bounds:{x:0,y:0,width:100,height:<value>}`).
- **`expand_pattern` MCP tool** gains `bounds` (object) and `max_height_pct`
  (number) parameters.
- **`bounds_assumption` response field** now reports `"explicit_override"` when
  bounds are applied (previously always `"full_content_area"`).
- **`capacity_warnings[].next_tool_call`** — underfilled cells now include a
  machine-readable `next_tool_call` suggesting re-expansion with a recommended
  `max_height_pct`, eliminating false underfill warnings for short-content grids.

## 4.4.0 (2026-05-06)

### Added

- **`rename_field` fix kind** now registered in `repair_slide` tool, enabling
  machine-driven field renames from unknown-key validation errors. Params:
  `{from, to}`.
- **`reshape_value` fix kind** added for structural value mismatches (e.g.,
  array where object expected). Params: `{path, value}`. Registered in
  `repair_slide` and `fixKindVocabulary`.
- **`validate_pattern` output schema** now inlines the `fix` object schema
  with `kind` (required) and `params` fields, replacing the untyped `object`.

## 4.3.0 (2026-05-06)

### Added

- **`design_mode_violation` diagnostics now include `next_tool_call`** with
  `{tool: "generate_presentation", args_template: {design_mode: "free"}}`,
  giving agents a machine-readable escape hatch when raw hex colors are
  intentional. Emitted from both `validate_input` and `generate_presentation`.

## 4.2.0 (2026-05-06)

### Added

- **`get_input_schema` MCP tool** returns the authoritative JSON Schema for
  `PresentationInput` and all nested types. Includes `x-field-scope`
  annotations (deck/slide/content/shape) and inline enum values. Supports
  digest-based caching to avoid redundant fetches.

## 4.1.0 (2026-05-06)

### Added

- **`plan_deck` and `recommend_visual` added to `mcp_tools_available`** in
  `get_capabilities`. These tools were registered and functional since 3.1.0
  but were omitted from the discovery catalog, making them invisible to agents
  that rely on `get_capabilities` for tool enumeration.
- **`get_capabilities` output schema** now includes `vocabularies` (enum
  registries) and `error_codes` fields, and the `features.fit_report` field
  is corrected from `boolean` to its actual `{supported, default_in}` shape.
- **`list_patterns` output schema** corrected from flat array to grouped
  `[{category, patterns}]` shape matching the runtime response.
- **CLI subcommands `plan-deck` and `recommend-visual`** added for parity
  with the MCP tools.

## 4.0.0 (2026-05-06)

### Breaking

- **Removed `fill_height`** from `shape_grid` input. Grid bounds are now
  authoritative and never shrink. The old "all-zero-heights shrinks bounds"
  behavior is retired. All grids distribute height using flex-like semantics.
  Existing decks that relied on `fill_height: true` are unaffected (the
  behavior is now the default). Decks that relied on implicit bounds-shrinking
  for raw grids will now fill their allocated layout area instead.

### Added

- `flex` — row-level field for proportional space distribution. Default is 1
  for rows with no explicit `height` and no `auto_height`. Rows with higher
  flex values receive proportionally more of the remaining space.
- `min_height` / `max_height` — row-level constraints in points. Applied
  after initial allocation with iterative clamping to redistribute overflow.

## 3.5.0 (2026-05-06)

### Added

- `compose` — slide-level field enabling pattern composition. Arranges 2–4
  patterns on a single slide via `direction` (`"vertical"` or `"horizontal"`)
  and `segments[]` with optional `size_pct` allocation. Child patterns validate
  independently; errors bubble up with `segment[N]` path prefix. The recommend
  endpoint now returns `compose_suggestions` for compound intents.
  (Bead: go-slide-creator-pbyh)

## 3.4.0 (2026-05-06)

### Added

- `surface_tints` — template metadata field mapping surface roles (`subtle`,
  `paper`, `elevated`, `inverse`) to scheme color names. Patterns resolve
  tinted background fills through this map, ensuring visual harmony with the
  template. All 5 bundled templates now define non-empty `surface_tints`.
  (Bead: go-slide-creator-avnm)
- `data_palette` — template metadata field providing an ordered list of scheme
  color names for chart series coloring. `svggen` uses this instead of fixed
  `accent1`–`accent6` ordering, letting templates control chart color priority.
  All 5 bundled templates now define non-empty `data_palette`.
  (Bead: go-slide-creator-avnm)

## 3.3.0 (2026-05-05)

### Added

- `accent_strategy` — top-level field controlling how default accent colors are
  chosen for patterns that don't specify an explicit `accent` override. Values:
  `"primary"` (default, always accent1), `"rotate"` (round-robin accent1–accent6
  by slide index), `"section-keyed"` (one accent per section, wrapping at 6).
  Existing decks with explicit accent overrides are unchanged.
  (Bead: go-slide-creator-jl9e)

## 3.1.0 (2026-05-05)

### Added

- `grid` — top-level field for deck-level layout rhythm configuration. Specifies
  `columns`, `gutter_emu`, `title_baseline_pct`, `content_top_pct`,
  `content_bottom_pct`, `left_margin_pct`, `right_margin_pct`. When set, the
  generator snaps all shape_grid bounds to the grid, ensuring consistent title
  and content positioning across the deck.
- `grid_violation` fit-finding code — emitted when a layout placeholder deviates
  from the grid configuration beyond the threshold (~0.05 inch). Carries
  `reposition_shape` fix suggestion with target EMU coordinates.
- `INVALID_GRID` MCP error code — returned when grid configuration is invalid
  (out-of-range percentages, contradictory ordering).

## 3.0.0 (2026-05-05)

**Breaking** — MCP tool parameter surface halved. All string-form JSON parameters
removed; only structured object parameters remain.

### Removed

- `json_input` (string) parameter from `generate_presentation`, `validate_input`,
  `repair_slide`, `preview_presentation_plan`, `score_deck`. Use `presentation`
  (object) instead.
- `values` (string), `overrides` (string), `cell_overrides` (string), `callout`
  (string) parameters from `validate_pattern` and `expand_pattern`. Use the
  corresponding object parameters instead.
- `values_object`, `overrides_object`, `cell_overrides_object`, `callout_object`
  parameter names. These are now simply `values`, `overrides`, `cell_overrides`,
  `callout` (the `_object` suffix was only needed to disambiguate from the
  now-removed string forms).

### Changed

- `presentation` parameter is now **required** on `generate_presentation`,
  `validate_input`, `repair_slide`, `preview_presentation_plan`, `score_deck`.
- `values` parameter is now **required** on `validate_pattern` and `expand_pattern`.
- All object parameters now advertise JSON Schema properties via the MCP tool
  schema (previously bare `type: object` with only a description).
- `resolveStringOrObject` helper removed; replaced by `objectParamAsJSON`.

### Migration guide

Before (2.x):
```json
{"name": "generate_presentation", "arguments": {"json_input": "{\"template\":\"midnight-blue\",\"slides\":[...]}"}}
```

After (3.0):
```json
{"name": "generate_presentation", "arguments": {"presentation": {"template":"midnight-blue","slides":[...]}}}
```

Agents should stop double-serializing JSON — pass the presentation as a
structured object directly.

## 2.9.0 (2026-05-05)

### Additions

- `get_capabilities` response now includes `changelog_url` pointing at `docs/SCHEMA_CHANGELOG.md`.
- `mcp_tools_available` changed from `string[]` to `{name, added_in}[]` — each tool entry now declares the schema version it was introduced in.
- `deprecated_fields[].removed_in` now populated on every deprecation entry (both deprecated fields target `3.0.0`).
- `features.feature_versions` — a map declaring when each feature flag was introduced.
- MCP tool `read_presentation` — best-effort PPTX content extraction (slides, placeholders, shapes, tables, speaker notes).
- `generate_presentation` now emits deprecation warnings when the deck uses the legacy `value` field instead of typed `*_value` fields.

## 2.8.0 (2026-05-05)

### Additions

- `chrome` — deck-level persistent chrome block with `confidentiality`, `client_name`, `project_code`, `footer_date`, `page_numbers` (with `enabled`, `format`, `skip`), and `section_crumb` fields. Composites into footer left text and supports formatted page numbers with `{current}` / `{total}` placeholders. Chrome is suppressed on title/closing slides by default (configurable via `page_numbers.skip`).

## 2.7.0 (2026-05-05)

### Additions

- `structure` — deck-level structural grammar block with `cover`, `closing`, `auto_agenda`, and `sections[]` (each with `title` and `slides[]`). When present, the generator expands sections into a flat slide sequence with auto-generated section dividers and optional agenda slide. Mutually exclusive with top-level `slides`.
- `agenda` pattern — numbered section list for agenda / table-of-contents slides, with optional `highlight` override to emphasize the current section.
- Structural validation: `missing_closing` warning emitted when a cover slide is present but no closing slide.

## 2.6.0 (2026-05-05)

### Additions

- `shape_grid.rows[].cells[].group` — boolean flag that wraps all child shapes of a cell in a `p:grpSp` group element. Grouped shapes move as a unit when edited in PowerPoint.

## 2.5.0 (2026-05-05)

### Additions

- `slides[].eyebrow` — small-caps label prepended to the title placeholder (e.g., "STRATEGY — Market Expansion").
- `body_and_lead` content type — lead-in paragraph (16pt bold) followed by supporting bullets (12pt). Use for thesis+evidence patterns.
- `bullet_groups[].groups[].group_label` — optional small-caps accent label rendered above each group header.
- Numbered lists now emit `<a:buAutoNum type="arabicPeriod"/>` for proper OOXML auto-numbering with hanging indent on multi-line wraps.

## 2.4.0 (2026-05-05)

### Additions

- MCP tool `get_shape_catalog` — returns all preset shape geometries grouped by use case (basic, arrow, flow, callout, star_banner, line_connector, symbol, math, action_button, chart_tab) with adjustment handle metadata. Enables agents to discover directional and decorative shapes beyond the default `rect`.

## 2.3.0 (2026-05-05)

### Additions

- `design_mode` — top-level field accepting `"constrained"` (default) or `"free"`. In constrained mode, raw hex colors and absolute font sizes in shape_grid, pattern overrides, and chart/diagram styles are rejected with `design_mode_violation` diagnostics suggesting the nearest scheme color.

## 2.1.0 (2026-05-05)

### Additions

- `table.style.highlight_column` — 1-indexed column to apply accent3 tint fill
- `table.style.totals_row` — last data row rendered bold with dk1 top border
- `table.style.column_types` — per-column type (`text`, `number`, `currency`, `percent`, `delta`); drives alignment and delta red/green text color
- `table.rows[][].conditional` — per-cell conditional formatting rule (`{rule, threshold, fill}`)

## 2.0.0 (2026-05-05)

**Breaking** — first versioned contract baseline. All prior changes that
accumulated under the frozen "1.0.0" are consolidated here.

### Additions since original 1.0.0

- `slides[].pattern` — named pattern expansion (replaces manual shape_grid authoring)
- `slides[].background` — slide background image support
- `slides[].transition`, `slides[].transition_speed`, `slides[].build` — animation fields
- `slides[].contrast_check` — per-slide contrast enforcement toggle
- `slides[].source` — source attribution field
- `defaults` — deck-level `table_style` and `cell_style` defaults block
- `theme_override` — deck-level theme color/font overrides
- `content[].font_size` — per-content-item font size override
- `content[].body_and_bullets_value`, `content[].bullet_groups_value` — new typed content fields
- `split_slide` type — automatic slide pagination
- `shape_grid` — callout support, connector specs, accent bars, overlays, image text
- MCP tools added: `expand_pattern`, `validate_pattern`, `show_pattern`, `list_patterns`,
  `recommend_pattern`, `render_slide_image`, `render_deck_thumbnails`, `score_deck`,
  `preview_presentation_plan`, `repair_slide`, `list_template_settings`,
  `register_template_setting`, `delete_template_setting`, `get_capabilities`,
  `table_density_guide`, `resolve_theme`, `list_icons`
- Fix.Kind vocabulary: `reduce_text`, `shorten_title`, `split_at_row`, `swap_layout`,
  `use_one_of`, `use_semantic_color`, `rewrite_field`, `truncation_summary`,
  `replace_color`, `rename_field`, `replace_value`, `reposition_shape`

### Removed / renamed

- `slides[].content[].placeholder` (raw OOXML name) — replaced by `placeholder_id`
- `slides[].content[].value` (untyped) — replaced by typed `*_value` fields

### Contract enforcement

A compile-time fingerprint test (`schema_fingerprint_test.go`) now fails CI if
struct fields, MCP tool names, or Fix.Kind vocabulary change without a
corresponding `SchemaVersion` bump and changelog entry.

## 1.0.0 (initial)

Original schema: `template`, `output_filename`, `footer`, `slides` with
`layout_id`, `slide_type`, `content` (placeholder + type + value).
MCP tools: `generate_presentation`, `list_templates`, `get_data_format_hints`,
`get_chart_capabilities`, `get_diagram_capabilities`, `validate_input`.
