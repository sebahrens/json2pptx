# Schema Changelog

Tracks backward-incompatible and notable additions to the JSON input schema,
MCP tool surface, and Fix.Kind vocabulary. Agents compare `schema_version`
(from `get_capabilities`) across sessions to detect contract drift.

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
