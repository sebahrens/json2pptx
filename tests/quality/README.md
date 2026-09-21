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

## Fresh-agent benchmark

`go run ./cmd/qualitybench --agent "/absolute/path/to/agent"` runs the configured
agent for the 12 frozen briefs in `agent_briefs.json`, three template families,
two workflow configurations, and two repetitions. The runner invokes the agent
for every run and writes one evidence file containing the model/version, prompt,
tool calls, artifacts, cost, iterations, duration, and failures. It never calls
a provider in default CI and refuses to run without `--agent`.

Rate artifact pairs blind with two reviewers on five frozen 1–5 dimensions:
readability, hierarchy, template fidelity, factual completeness, and overall
usability. Reviewers also mark lost critical facts and critical template
defects. Aggregate reporting includes reviewer disagreements and a 95% Wilson
interval for the usable rate. The preregistered product bar is at least 80%
usable without manual slide editing, zero lost critical facts, zero critical
template defects, and a clear paired improvement over baseline. Until an actual
run and both ratings exist this remains a proposed bar and the release decision
is `inconclusive` or `hold`.

Templates default to `midnight-blue` (corporate), `warm-coral` (editorial) and
the purpose-built held-out `portability-side-logo` fixture outside the bundled
template directory. Override with `--templates name:family[:heldout],...` or
`--templates name=path/to/template.pptx:family[:heldout],...`. Missing template
files fail fast.

After the agent runs, every `.pptx` artifact is rendered (LibreOffice →
pdftoppm/ImageMagick) into a contact sheet named by an opaque blind ID under
`<out>/sheets/`, alongside `ratings_template.csv` (blind IDs only) and
`blind_key.json` (the un-blinding key — keep it away from reviewers). The
results JSON goes to `tests/quality/results/`. Give each human reviewer a
separate copy of the CSV, set `reviewer_type=human`, then apply both with
`go run ./cmd/qualitybench --report tests/quality/results/<file>.json
--ratings reviewer-a.csv,reviewer-b.csv`. Duplicate reviewers and optional
`reviewer_type=llm` rows do not satisfy the two-human release gate. The complete
rerun and rating procedure is in [docs/QUALITY_BENCHMARK.md](../../docs/QUALITY_BENCHMARK.md).

`go run ./cmd/qualitybench --dry-run` exercises the whole harness without an
agent: each brief gets a deterministic category-based reference deck
(`dry-run-reference` configuration) rendered on all three templates, producing
36 blind contact sheets for the 12 briefs and a results JSON. Reference decks
measure template/engine rendering, not agent authoring — never report them as
agent results.

Static fixture replay from the mechanical harness above is separate and must
never be reported as a fresh-agent benchmark result.

## Template portability matrix

`portability_matrix_test.go` generates a representative deck (title, bullets,
kpi pattern, chart, two-column; footer + takeaway + source bands) on four small
fixture templates in `fixtures/portability/templates/` — 4:3, 21:9 ultrawide,
two slide masters, and a master logo in the right margin — and asserts the
geometry contract with `testutil.CheckDeckPortability`: every shape inside the
canvas; takeaway/source bands on the layout's body column and clear of the
title, footer placeholders, rendered footers and logo; content inside the safe
area. Regenerate the fixtures with `make portability-fixtures`.

```bash
go build -o bin/json2pptx ./cmd/json2pptx
go test ./tests/quality -run TestPortability -v          # geometry (needs the binary)
PORTABILITY_RENDER_INTEGRATION=1 go test ./tests/quality -run TestPortability -v  # + LibreOffice render
```
