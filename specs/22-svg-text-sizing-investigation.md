# SVG Text Sizing Architecture Investigation

**Status**: Decision Record
**Date**: 2026-03-09
**Bead**: go-slide-creator-i1ydp

## Problem Statement

40+ commits fixing text sizing/positioning across SVG diagram types. Timeline.go alone had 12 fixes. Each diagram type handles font sizing independently with ad-hoc heuristics, leading to recurring regressions.

## Investigation Findings

### 1. Renderer Analysis (Hypothesis B)

**Production renderer**: `tdewolff/canvas` Go library (native SVG output).
**PPTX embedding**: Default strategy is `native` (SVG embedded via OOXML svgBlip extension).
**PNG fallback**: `rsvg-convert` → `resvg` → placeholder, used only with `-chart-png` flag.
**Golden tests**: Validated against `tdewolff/canvas` output only (string comparison with normalization). No external renderer dependency.

**Verdict**: Hypothesis B (renderer mismatch) is **not the primary cause**. Only 1-2 of 40+ fixes were renderer-related (font double-conversion). The canvas library provides deterministic output, and golden tests validate against it consistently.

### 2. Fix Categorization

Analysis of 40+ commits touching timeline.go, charts.go, waterfall.go:

| Category | Count | Examples |
|----------|-------|---------|
| Font size too small | ~12 | "labels too small", "SizeCaption→SizeSmall", "increase font sizes" |
| Label overlap/illegible | ~10 | "labels overlapping", "text overlapping and illegible" |
| Narrow container sizing | ~8 | "narrow two-column", "half-width", "slot2 truncation" |
| Density-adaptive logic | ~5 | "dense data", "waterfall density-adaptive", "auto-rotate" |
| Layout math bugs | ~5 | "axis alignment", "bounding box", "description below bars" |
| Renderer-specific | ~2 | "font double-conversion", "loScale workarounds" |

**~95% of fixes are layout math bugs, not renderer issues.**

### 3. Shared Infrastructure (Already Exists)

The codebase already has solid shared primitives in `internal/svggen/builder.go`:

- `ClampFontSize(text, maxWidth, preset, min)` — binary search for largest fitting font
- `ClampFontSizeForRect(text, maxW, maxH, preset, min)` — multi-line variant
- `MeasureText(text)` — actual font metrics via canvas
- `TruncateToWidth(text, maxWidth)` — ellipsis truncation
- `WrapText() / DrawWrappedText()` — multi-line rendering
- `DefaultMinFontSize = 9pt` — legibility floor
- `AdaptXLabels()` in `cartesian_layout.go` — shared x-axis label strategy

### 4. What's NOT Shared (Root Cause of Churn)

Each diagram type makes **independent strategic decisions**:

| Decision | charts.go | timeline.go | waterfall.go | process_flow.go |
|----------|-----------|-------------|--------------|-----------------|
| Base font | SizeSmall (10pt) | SizeBody (12pt) | SizeBody (12pt) | SizeBody (12pt) |
| Min font floor | 7-9pt (varies) | 9pt | 10pt | 5-9pt (context) |
| Density threshold | 10, 15 items | 15, 10 items | 8 bars | 8 steps |
| Shrink strategy | AdaptXLabels | Custom inline | AdaptXLabels | ClampFontSize |
| Multi-line? | No | Yes (wrapped) | No | Yes (ClampForRect) |
| Override MinFont? | No | No | No | Yes (temporarily) |

The inconsistency means each diagram type independently re-discovers the same solutions (e.g., "shrink font → rotate → truncate" cascade), with different thresholds and orders.

## Recommendation

### Do NOT build a monolithic TextSizer abstraction

The diagram types have genuinely different layout constraints (timeline bars vs chart axes vs process flow boxes). A single TextSizer would either be too generic to help or too complex to maintain.

### DO extract a shared LabelFitStrategy

A lightweight strategy pattern for the common "fit text into constrained space" decision chain:

```go
// LabelFitStrategy encodes the ordered cascade for fitting text.
type LabelFitStrategy struct {
    PreferredSize  float64  // Starting font size (e.g., SizeBody)
    MinSize        float64  // Absolute floor (e.g., 9pt)
    DensityBreaks  []int    // Item counts where behavior changes (e.g., [8, 15])
    AllowRotation  bool     // Can labels be rotated?
    AllowWrap      bool     // Can text wrap to multiple lines?
    AllowTruncate  bool     // Can text be truncated with ellipsis?
    TruncateMinLen int      // Min characters to preserve (e.g., 3)
}

// Fit returns the best font size and display text for the given constraints.
func (s *LabelFitStrategy) Fit(b *SVGBuilder, text string, maxWidth float64, itemCount int) LabelFitResult
```

This would:
1. **Centralize the cascade logic** (shrink → rotate → thin → truncate) in one place
2. **Let each diagram type configure** its strategy via struct fields
3. **Eliminate the most common bug pattern**: diagram types forgetting to handle narrow containers or high density

### Estimated effort

- **Phase 1** (2-3 sessions): Extract `LabelFitStrategy` from `AdaptXLabels`, apply to bar/waterfall x-axis labels
- **Phase 2** (2-3 sessions): Apply to timeline labels and process flow labels
- **Phase 3** (1-2 sessions): Apply to remaining diagram types, update golden tests

### Risk mitigation

- Golden test suite (64 files) provides regression safety
- Quality gate tests (`AssertSVGQuality`) catch font-size-below-minimum violations
- Incremental rollout: one diagram type at a time

## Decision

**Approved approach**: Extract `LabelFitStrategy` as a shared decision layer on top of existing primitives. Do NOT rewrite rendering logic — only centralize the font sizing decision cascade.

## Files to Change

- `internal/svggen/label_fit.go` (new — strategy implementation)
- `internal/svggen/cartesian_layout.go` (refactor AdaptXLabels to use strategy)
- `internal/svggen/charts.go` (adopt strategy for scatter labels, value labels)
- `internal/svggen/timeline.go` (adopt strategy for activity/date labels)
- `internal/svggen/waterfall.go` (adopt strategy for value labels)
- `internal/svggen/process_flow.go` (adopt strategy for step/description labels)
