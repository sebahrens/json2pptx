# Style Defaults Evidence Report

Measured 2026-04-23 against all 17 example decks in `examples/`.

## Methodology

For each deck:
1. Extract all shape-grid cell style objects (geometry + fill + text formatting props, excluding text content)
2. Extract table_style / cell_style blocks from table content items
3. Hash each style block; count identical duplicates
4. Calculate bytes saved if each unique style were defined once in a `defaults` block
   and each repeated occurrence replaced by a ~25-byte `style_ref`
5. Estimate tokens as bytes / 4 (conservative for JSON)

## Per-deck results

| Deck | Deck bytes | ~Tokens | SG cells | Unique | Bytes saved | ~Tokens saved | % deck |
|------|-----------|---------|----------|--------|------------|--------------|--------|
| sovereign-ai-strategy.json | 94,911 | ~23,727 | 116 | 31 | 9,819 | ~2,454 | 10.3% |
| consulting-layouts.json | 30,723 | ~7,680 | 53 | 20 | 3,817 | ~954 | 12.4% |
| visual-maturity-stress-test.json | 49,693 | ~12,423 | 86 | 38 | 3,492 | ~873 | 7.0% |
| shape-grid-panels.json | 15,716 | ~3,929 | 25 | 5 | 2,460 | ~615 | 15.7% |
| icons-and-svg-grid-test.json | 27,829 | ~6,957 | 33 | 14 | 2,077 | ~519 | 7.5% |
| directional-comparison.json | 19,289 | ~4,822 | 37 | 14 | 2,031 | ~507 | 10.5% |
| business-model-canvas.json | 14,126 | ~3,531 | 20 | 4 | 1,896 | ~474 | 13.4% |
| kpi-big-numbers.json | 12,989 | ~3,247 | 26 | 8 | 1,660 | ~415 | 12.8% |
| gear-process-chain.json | 17,309 | ~4,327 | 31 | 19 | 1,436 | ~359 | 8.3% |
| contrast-fixer-test.json | 17,988 | ~4,497 | 21 | 11 | 496 | ~124 | 2.8% |

**7 decks have no shape grids or tables and gain 0 tokens from style defaults:**
- basic-deck.json (3,520 bytes)
- charts.json (4,076 bytes)
- diagrams.json (3,368 bytes)
- full-showcase.json (7,767 bytes)
- patterns-smoke.json (6,314 bytes)
- split-slide-vendor-matrix.json (3,234 bytes)
- table-style-demo.json (1,114 bytes)

## Distribution

### All 17 decks

| Metric | Bytes saved | ~Tokens saved |
|--------|------------|--------------|
| Min | 0 | ~0 |
| P25 | 0 | ~0 |
| **Median (P50)** | **1,436** | **~359** |
| P75 | 2,077 | ~519 |
| P90 | 3,817 | ~954 |
| P95 | 9,819 | ~2,454 |
| Max | 9,819 | ~2,454 |
| Mean | 1,716 | ~429 |

### Decks with shape grids (10/17)

| Metric | Bytes saved | ~Tokens saved |
|--------|------------|--------------|
| Min | 496 | ~124 |
| **Median** | **2,077** | **~519** |
| P90 | 9,819 | ~2,454 |
| Max | 9,819 | ~2,454 |
| Mean | 2,918 | ~729 |

## Verdict

**GO** — All-deck median (~359 tokens) exceeds the 200-token threshold.

The deck-local `defaults` block (shipped in go-slide-creator-mbh2) captures within-deck
savings. The cross-deck `styles.json` sidecar's ROI depends on cross-deck reuse — if
agents reuse identical style patterns across separate deck JSON files, the additional
savings would compound on top of within-deck deduplication.

## Notes

- Token estimates use bytes/4, which is conservative for JSON (actual LLM tokenization
  typically produces fewer tokens than bytes/4 for structured text)
- `sovereign-ai-strategy.json` is the clear outlier at ~2,454 tokens saved — it has 116
  shape-grid cells with only 31 unique style patterns
- Style repetition correlates strongly with shape-grid cell count: decks without shape
  grids gain nothing from style defaults
- The within-deck `defaults` block already captures this value; cross-deck styles.json
  would only add value for multi-deck workflows with shared visual identity
