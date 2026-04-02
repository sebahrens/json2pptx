# Integration Tests

This directory contains comprehensive integration tests for the go-slide-creator project. The tests exercise all PowerPoint (PPTX) and SVG diagram generation features by running JSON/YAML fixtures through the CLI tools.

## Directory Structure

```
tests/integration/
  run_pptx_tests.sh       # PPTX test runner script
  run_svg_tests.sh        # SVG test runner script
  json_fixtures/           # JSON input files for json2pptx
  svg_fixtures/            # YAML input files for svggen
  README.md                # This file
```

## Running Tests

### Prerequisites

- Go 1.21+ installed
- `unzip` command available (for PPTX validation)
- `python3` available (for JSON output parsing in PPTX tests)
- Optionally, `xmllint` for full SVG XML validation

### PPTX Tests

```bash
# Run all PPTX tests
./tests/integration/run_pptx_tests.sh

# Run with verbose output
./tests/integration/run_pptx_tests.sh --verbose

# Keep generated PPTX files for manual inspection
./tests/integration/run_pptx_tests.sh --keep-output

# Run only chart-related tests
./tests/integration/run_pptx_tests.sh --filter chart

# Run a specific test
./tests/integration/run_pptx_tests.sh --filter 09_
```

### SVG Tests

```bash
# Run all SVG tests
./tests/integration/run_svg_tests.sh

# Run with verbose output
./tests/integration/run_svg_tests.sh --verbose

# Keep generated SVG files for manual inspection
./tests/integration/run_svg_tests.sh --keep-output

# Run only diagram-related tests
./tests/integration/run_svg_tests.sh --filter diagram

# Run a specific test
./tests/integration/run_svg_tests.sh --filter 16_
```

### Run Both

```bash
./tests/integration/run_pptx_tests.sh && ./tests/integration/run_svg_tests.sh
```

## PPTX Test Fixtures (json_fixtures/)

| # | Fixture | Feature Tested |
|---|---------|----------------|
| 01 | title_slide | Title slide with title and subtitle |
| 02 | content_bullets | Content slide with bullet list |
| 03 | section_divider | Section divider slide |
| 04 | two_column | Two-column layout with bullet groups |
| 05 | body_and_bullets | Body paragraph + bullets + trailing body |
| 06 | bullet_groups | Grouped bullets with headers |
| 07 | table_simple | Simple table with styling |
| 08 | table_merged_cells | Table with col_span/row_span |
| 09 | chart_bar | Bar chart |
| 10 | chart_line | Line chart |
| 11 | chart_pie | Pie chart |
| 12 | chart_donut | Donut chart |
| 13 | chart_area | Area chart |
| 14 | chart_radar | Radar chart |
| 15 | chart_scatter | Scatter chart with series data |
| 16 | chart_bubble | Bubble chart with size data |
| 17 | chart_stacked_bar | Stacked bar chart (multi-series) |
| 18 | chart_stacked_area | Stacked area chart (multi-series) |
| 19 | chart_grouped_bar | Grouped bar chart (multi-series) |
| 20 | chart_waterfall | Waterfall chart |
| 21 | chart_funnel | Funnel chart |
| 22 | chart_gauge | Gauge chart |
| 23 | chart_treemap | Treemap chart |
| 24 | diagram_timeline | Timeline diagram |
| 25 | diagram_process_flow | Process flow diagram |
| 26 | diagram_pyramid | Pyramid diagram |
| 27 | diagram_venn | Venn diagram (3 circles) |
| 28 | diagram_swot | SWOT analysis |
| 29 | diagram_org_chart | Org chart (tree format) |
| 30 | diagram_gantt | Gantt chart |
| 31 | diagram_matrix2x2 | 2x2 prioritization matrix |
| 32 | diagram_porters | Porter's Five Forces |
| 33 | diagram_house | Strategy house diagram |
| 34 | diagram_bmc | Business Model Canvas |
| 35 | diagram_value_chain | Value chain analysis |
| 36 | diagram_nine_box | Nine-box talent grid |
| 37 | diagram_kpi_dashboard | KPI dashboard |
| 38 | diagram_heatmap | Heatmap |
| 39 | diagram_fishbone | Fishbone/Ishikawa diagram |
| 40 | diagram_pestel | PESTEL analysis |
| 41 | diagram_panel_layout | Panel layout diagram |
| 42 | inline_formatting | Bold, italic, underline inline tags |
| 43 | transitions | All transition types (fade, push, wipe, cover, cut, none) |
| 44 | build_animation | Bullet build animation |
| 45 | footer_and_theme | Footer injection + theme color overrides |
| 46 | speaker_notes_source | Speaker notes and source attribution |
| 47 | shape_grid | Shape grid with preset geometries |
| 48 | image_slide | Image content type |
| 49 | legacy_value_format | Legacy "value" field (backward compatibility) |
| 50 | edge_unicode | Unicode and special characters |
| 51 | edge_long_text | Very long title and bullet text |
| 52 | edge_many_slides | 15-slide deck |
| 53 | edge_minimal | Minimal single-slide deck |
| 54 | mixed_content_deck | Full deck with mixed content types |
| 55 | blank_slide | Blank slide type |
| 56 | all_templates | Alternate template (midnight-blue) |

## SVG Test Fixtures (svg_fixtures/)

| # | Fixture | Diagram Type |
|---|---------|-------------|
| 01-11 | Charts | bar, line, pie, donut, area, radar, scatter, bubble, stacked_bar, stacked_area, grouped_bar |
| 12-15 | Specialized Charts | waterfall, funnel, gauge, treemap |
| 16-33 | Diagrams | timeline, process_flow, pyramid, venn, swot, org_chart, gantt, matrix_2x2, porters_five_forces, house_diagram, business_model_canvas, value_chain, nine_box_talent, kpi_dashboard, heatmap, fishbone, pestel, panel_layout |
| 34-48 | Edge Cases | single data point, many data points, two-slice pie, two-circle venn, flat org chart nodes, special characters, small/large dimensions, minimal data, alternate formats, multi-series, linear flow |

## What Gets Validated

### PPTX Tests
- json2pptx exits successfully
- JSON output reports `success: true`
- Output file exists and is non-empty
- Output file is a valid ZIP archive
- ZIP contains `[Content_Types].xml`
- ZIP contains `ppt/presentation.xml`
- ZIP contains at least one slide (`ppt/slides/slide*.xml`)

### SVG Tests
- svggen exits successfully
- Output file exists and is non-empty
- Output contains `<svg` element
- Output contains closing `</svg>` tag
- If xmllint is available: full XML well-formedness validation

## Adding New Tests

1. Create a new JSON file in `json_fixtures/` or YAML file in `svg_fixtures/`
2. Follow the naming convention: `NN_descriptive_name.json` / `NN_descriptive_name.yaml`
3. Use the documented schemas from `SLIDE_FORMAT.md` (PPTX) or `svggen/core/types.go` (SVG)
4. The test runner will automatically pick up new fixtures
