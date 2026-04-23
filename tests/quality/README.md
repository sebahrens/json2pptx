# Quality Eval Harness

Minimal evaluation harness for measuring json2pptx output quality. Computes mechanical
metrics from JSON deck fixtures and fit-report output, with optional Haiku visual QA.

## Quick Start

```bash
# Run mechanical metrics (no API key needed)
go test ./tests/quality/ -v -count=1

# Or use the shell runner
./tests/quality/run.sh

# With visual QA (needs ANTHROPIC_API_KEY + LibreOffice)
./tests/quality/run.sh --visual-qa
```

## Metrics Computed

### From JSON input (mechanical)
| Metric | Description |
|--------|-------------|
| `tdr_violations` | Slides with tables exceeding TDR limits (rows>7 or cols>6) |
| `hex_fill_count` | Non-brand hex color fills (excludes black/white) |
| `hex_fill_ratio` | Hex fills / total fills |
| `tiny_divider_count` | Row pairs with computed gap < 3pt |
| `small_font_count` | Shape text cells with font_size < 9pt |
| `mixed_fill_slides` | Slides mixing hex and semantic fill colors |

### From fit-report (render-time)
| Metric | Description |
|--------|-------------|
| `fit_overflow_count` | Cells where text exceeds available space |
| `density_exceeded` | Tables exceeding TDR cell ceiling |
| `unfittable_rate` | Proportion of findings with action=unfittable |
| `shrink_rate` | Proportion of findings with action=shrink |

## Adding a Prompt

1. Create a `.txt` file in `prompts/` describing the deck to generate
2. Create a matching `.json` fixture in `fixtures/` with the actual JSON input
3. Run the harness to verify metrics
4. Update the baseline: `./run.sh --update-baseline`

## Baseline and Regression Guard

The harness compares current results against `baseline.csv`. Regressions (metric
values increasing beyond baseline) are logged as warnings. To create or update
the baseline:

```bash
./tests/quality/run.sh --update-baseline
```

The baseline is also computed for all decks in `examples/` — if an example's
quality metrics regress, the harness flags it for human review.

## Directory Structure

```
tests/quality/
  prompts/         # Text descriptions of test scenarios (10 prompts)
  fixtures/        # JSON deck inputs for mechanical testing
  baseline.csv     # Reference baseline (committed)
  results.csv      # Current run output (gitignored)
  run.sh           # Shell runner (mechanical + optional visual QA)
  quality_test.go  # Go test harness
  README.md        # This file
```

## Prompt Inventory

| Prompt | Tests |
|--------|-------|
| `dense-table-16x6` | TDR violation (16 rows × 6 cols) |
| `comparison-matrix-5vendors` | Near-boundary table density |
| `kpi-grid-9-tiles` | Shape grid text fitting in small cells |
| `narrative-strategy-15slide` | Multi-slide deck with varied content |
| `two-tables-stacked` | Stacked tables structural smell |
| `mixed-hex-and-semantic` | Mixed fill scheme smell |
| `multiline-cells-table` | Multiline TDR counting |
| `pattern-card-grid-overflow` | Shape grid text overflow |
| `pattern-bmc-canvas-dense` | BMC pattern density |
| `font-size-stress-test` | Small font regression guard |
