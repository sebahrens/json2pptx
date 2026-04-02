# Shape Grid Stress Test Plan

## Summary

Comprehensive test plan covering 4 phases: unit tests (64 cases), integration fixtures (18 JSON files), cross-template validation (72 matrix tests), and Haiku visual QA (8 prompt types).

## Phase A: Unit Test Expansion (64 tests)

### Column Resolution (11 tests)
- Zero/negative column count, single column, 10/50 columns
- Array sum exceeding 100%, under 100%, with zeros, empty array
- Infer from empty rows, unsupported type

### Row Heights (7 tests)
- Single row, all specified, exceeding total, exceeding with unspecified
- All exceed with unspecified, 10 equal rows, negative height

### Grid Resolution (16 tests)
- Empty rows, single cell, zero/explicit/large/negative gap
- ColSpan+RowSpan combined, col_span/row_span exceeding grid
- More/fewer cells than columns, all empty cells, mixed shape+table
- 10x10 stress grid, ID allocation sequential, bounds non-overlapping

### Validation (7 tests)
- No rows, cell overlap from row_span, more cells than columns
- Empty grid, valid large grid, combined spans valid, multiple errors

### Shape XML Generation (16 tests)
- All Phase 1 geometries, zero/large bounds, adjustments
- Fill: hex with alpha, invalid JSON, hex without hash, empty string, all scheme colors
- Line: invalid JSON, zero width
- Text: invalid JSON, with font, default font, with insets, vertical align, empty content

### Bounds Calculation (7 tests)
- Full slide, zero, exceeding, overlapping title/footer, no gap, large gap, default matches constants

## Phase B: Integration JSON Fixtures (18 files)

| Fixture | Pattern | Key Stress |
|---|---|---|
| 3col_equal | 3 equal panels | Column equality |
| 4col_panels | 4-col header + content | 2-row mixed heights |
| 5col_equal | 5 narrow panels | Narrow columns |
| numbered_steps_5 | 5 numbered rows [10,90] | Unequal column widths |
| numbered_steps_10 | 10 rows at 10% each | Max row density |
| chevron_header | homePlate + chevron + content | Zero col_gap, mixed geometry |
| 2x2_grid | SWOT-style matrix | Multi-line text, top-aligned |
| 3x3_grid | Mixed geometries | 9 cells, varied shapes |
| 4x4_grid | 16-cell matrix | Maximum grid density |
| mixed_heights | Dashboard layout | col_span + 3 height tiers |
| colspan_title | Full-width banner | col_span across all cols |
| hierarchy | 4-row visual hierarchy | Banner + sub + content + footer |
| spacers | Null cell gaps | Empty/spacer columns |
| geometry_showcase | 20 geometries in 4x5 | All Phase 1 presets |
| mixed_shape_table | Shape + table cells | Cell type coexistence |
| process_flow | Arrows between boxes | 7-column alternating shapes |
| custom_bounds | Non-default bounds | Explicit x/y/w/h percentages |
| rowspan_sidebar | row_span=3 sidebar | Spanning + content grid |

## Phase C: Cross-Template Matrix (72 tests)

18 fixtures x 4 templates (midnight-blue, forest-green, warm-coral, template_2).

Pass criteria per test:
- Exit code 0
- Valid PPTX (validatepptx reports no errors)
- Correct slide count
- File size > 10KB

## Phase D: Haiku Visual QA (8 prompt types)

### Prompt Types
1. **Panel slides** — equal width, spacing, text readability, color distinction
2. **Numbered rows** — count, sequence, alignment, circle shape, spacing
3. **Chevron headers** — chevron shape, alignment, content boxes below, labeling
4. **Grid layouts** — dimensions, cell alignment, consistent size/gaps, no overlap
5. **Dashboard/mixed heights** — row count, height variation, full-width banner, hierarchy
6. **Spacer layouts** — shape count, no artifacts in spacer areas, even spacing
7. **Process flow** — flow direction, arrow shapes, process count, color progression
8. **General baseline** — not blank, no black/white boxes, contrast, clipping, professional

Each prompt returns structured JSON with PASS/FAIL per criterion.

## Implementation Priority

1. **Phase A** — immediate, catches regressions fast (pure Go unit tests)
2. **Phase B** — next, validates full JSON-to-PPTX pipeline (fixture files)
3. **Phase C** — parametric Go test looping fixtures x templates
4. **Phase D** — highest effort, integrates with Haiku API for visual scoring
