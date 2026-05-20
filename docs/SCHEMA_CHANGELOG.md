# Schema Changelog

Tracks backward-incompatible and notable additions to the JSON input schema,
MCP tool surface, and Fix.Kind vocabulary. Agents compare `schema_version`
(from `get_capabilities`) across sessions to detect contract drift.

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
