# Text Layout Architecture: Breaking the Fix Cycle

**Status**: Decision Record
**Date**: 2026-03-15
**Supersedes**: specs/22-svg-text-sizing-investigation.md (extends findings)

## Problem Statement

208 text-related fix commits in 2 months (~19.4% of all development work). Text truncation, overlap, and collision bugs keep recurring across process_flow, timeline, panel_layout, fishbone, and other diagram types despite being repeatedly "fixed." The root cause is architectural, not parametric.

## Evidence

### Quantitative
- **208 fix commits** for text sizing/truncation/overlap out of 1,064 real commits
- **189 `SetFontSize` calls** across 31 files; only **104 use `LabelFitStrategy`** — 45% bypass the shared system
- **81 magic numbers** in process_flow.go alone (scaling fractions, padding ratios, thresholds)
- **13 diagram types** have zero narrow-width golden tests
- `checkRectOverlap` in quality_test.go uses 50% area threshold; **zero text-text overlap detection**
- **5 competing minimum font floors** (6.0, 7.0, 9.0, 10.0, 11.0) across different code paths

### The Fishbone Case Study
Between March 9–14, the **same symptom** (fishbone cause label truncation) was "fixed" **10 separate times**. Each fix addressed one width/density scenario while leaving others broken.

### The Min Font Size Yo-Yo
The global minimum font size oscillated 6 times in 6 weeks: 7→9→raise 60%→lower 50%→7→10. Each direction fixes one failure mode (too small / doesn't fit) while creating the other.

## Root Causes (Agreed by Advocate and Devil's Advocate)

1. **Detection gap**: Tests do not catch text overlap or illegible truncation
2. **Prevention gap**: 45% of font sizing bypasses `LabelFitStrategy`
3. **Scale-then-clamp**: Uniform scaling followed by individual min clamping creates unrecoverable space deficits
4. **No parametric testing**: Golden tests verify one width/density; fixes at one width break another
5. **Each diagram is an island**: 20+ types with independent layout math

## Decision: Pragmatic Incremental Fix (Not Architectural Rewrite)

### Rejected Approaches

| Approach | Reason |
|----------|--------|
| Constraint solver (Cassowary/VPSC) | 2+ month project; Go has no mature solver; complexity cliff |
| Universal `LayoutPlan` interface | Fishbone ≠ treemap ≠ org chart; abstraction would be too generic to add value |
| Runtime collision detection + re-layout | Convergence problem; may oscillate or fail |
| Numeric "quality scores" (0.0–1.0) | Arbitrary thresholds; binary pass/fail assertions are actionable, float scores are not |
| Shared renderer for all diagram types | Genuinely different topologies; forced abstraction would leak everywhere |

### Accepted Approach: Test-First + Complete LabelFitStrategy Adoption

## Implementation Plan

### Phase 1: Close the Detection Gap (Week 1, ~5 days)

#### 1a. Add `checkTextTextOverlap` to `AssertSVGQuality`
Parse `<text>` and `<tspan>` elements from SVG output, estimate bounding boxes (position + font-size × 0.6 × len), check all pairs for overlap. An 80% accurate heuristic that runs on every test is better than no check.

#### 1b. Add `checkTextTruncation` quality gate
Flag any visible text element shorter than 4 characters that ends with ellipsis. "A..." is never useful.

#### 1c. Add parametric golden tests for all 13 missing diagram types
Every diagram type gets `_halfwidth` and `_thirdwidth` variants. Test infrastructure already supports `OutputSpec{Preset: "half_16x9"}`.

**Expected impact**: Catches 60–70% of recurring bug classes at CI time.

### Phase 2: Close the Prevention Gap (Weeks 2–3)

#### 2a. Complete `LabelFitStrategy` adoption
Convert all 85+ ad-hoc `SetFontSize` calls to use `LabelFitStrategy`. Mechanical refactoring — the shared system already exists.

Target: reduce non-LabelFitStrategy `SetFontSize` calls from 85 to <10.

#### 2b. Font-size quantization
Snap to discrete tiers: `[14, 12, 11, 10, 9, 8, 7]` pt. All labels in a logical group (e.g., all process flow step labels) use the same tier. Eliminates visual noise from 11.3pt vs 10.7pt.

#### 2c. Consolidate font-size floors
One canonical minimum (7pt for labels, 9pt for body text). Remove all competing floors. Enforce via `LabelFitStrategy` defaults, not scattered hardcoded values.

### Phase 3: Targeted Layout Improvements (Weeks 3–4)

#### 3a. Extract `FitLabelsInGrid` helper
For the 8 grid-based diagram types (process_flow, panel_layout, heatmap, nine_box, SWOT, PESTEL, BMC, value_chain): given N items and a bounding Rect, compute box sizes and the maximum font size that lets all labels fit. Single shared function replaces independent per-diagram guessing.

#### 3b. Dense-data stress golden tests
Add 10+ item variants at `third_16x9` for all diagram types. This exercises worst-case layout paths.

#### 3c. Fix all bugs surfaced by new tests
There will be many. Each fix is now automatically regression-tested by Phase 1 infrastructure.

### Phase 4: Process Flow & Timeline Specific (Week 4+)

#### 4a. Process flow auto-layout switching
- ≤6 steps: horizontal (current)
- 7–12 steps: two-row zigzag (existing code, lower threshold)
- >12 steps: vertical
- Narrow width (<400pt): always vertical

#### 4b. Timeline label staggering improvements
- Enforce minimum label separation distance
- Stagger labels above/below for dense timelines
- Suppress descriptions (Tier 2 content) when density exceeds threshold

## Key Principles

1. **Tests ship before fixes** — detection gap closes first
2. **No architectural rewrite** — works with existing 20 independent renderers
3. **Incremental adoption** — each diagram type migrates independently
4. **Binary pass/fail** — no floating-point quality scores
5. **Content tiering** — titles are Tier 1 (must show), descriptions are Tier 2 (drop first)

## Success Criteria

- Zero text-text overlaps in any golden test across all widths
- Zero illegible truncation (text < 4 chars with ellipsis)
- All 20+ diagram types tested at full, half, and third width
- <10 non-LabelFitStrategy `SetFontSize` calls remaining
- Fishbone/timeline/process_flow text bugs stop recurring (measured by commit history)

## References

- [Occupancy Bitmap Label Layout](https://idl.cs.washington.edu/files/2021-FastLabels-VIS.pdf) — IEEE VIS 2021
- [Automatic Label Placement in Diagrams](https://www.yworks.com/pages/automatic-label-placement-in-diagrams) — yWorks
- specs/22-svg-text-sizing-investigation.md — Prior investigation (March 9, 2026)
