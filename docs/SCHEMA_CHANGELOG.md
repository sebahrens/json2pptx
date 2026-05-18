# Schema Changelog

Tracks backward-incompatible and notable additions to the JSON input schema,
MCP tool surface, and Fix.Kind vocabulary. Agents compare `schema_version`
(from `get_capabilities`) across sessions to detect contract drift.

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
