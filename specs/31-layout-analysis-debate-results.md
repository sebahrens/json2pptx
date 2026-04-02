# Layout Analysis: Debate Results (2026-03-30)

## Sources Analyzed
- **Free Template Bundle** (120 slides, SlideModel.com) — generic template showcase
- **Bain & Company Resilience Deck** (27 slides, AmCham 2021) — real MBB consulting deck

## Method
4 Opus-class agents debated in parallel: advocate + devil's advocate for each source.

## Confirmed Additions (4 new examples)

### 1. Business Model Canvas (Osterwalder 9-block)
- **Source**: Template bundle slide 10
- **Verdict**: Both sides agreed — iconic framework, proves col_span/row_span capability
- **Implementation**: shape_grid with 5-col top (Key Partners/Activities/Value Prop/Customer Rels/Segments), center rows with row_span (Resources/Channels), 2-col bottom (Cost Structure/Revenue Streams)
- **Effort**: ~45 min JSON, pure existing features
- **Bead**: go-slide-creator-1f4

### 2. Directional Comparison (Before→After with Chevron Divider)
- **Source**: Bain deck p22 (Risk Exposure → Resilience)
- **Verdict**: Only Bain pattern to survive devil's advocate — adds semantic directionality to comparison
- **Implementation**: 3-column grid [42, 16, 42], center column holds chevron/rightArrow shapes, left/right panels hold comparison content. Red callout banner at bottom via col_span row.
- **Effort**: ~20 min JSON
- **Bead**: go-slide-creator-e4d

### 3. Gear Process Chain (geometry showcase)
- **Source**: Template bundle slides 90, 100, 110
- **Verdict**: Advocate's strongest argument: "gear6, gear9, funnel, trapezoid geometries exist but have ZERO examples"
- **Implementation**: Single row of gear6 shapes with alternating accent colors + description row below
- **Effort**: ~20 min JSON
- **Bead**: go-slide-creator-wf8

### 4. KPI Big Numbers (dark background hero variant)
- **Source**: Bain deck p6 (1.6x / 3x / 6x on black), template bundle
- **Verdict**: Distinct from existing scorecard — uses dk1 fill, dramatic number sizing, full-width narrative bar below
- **Implementation**: 2-row grid: row 1 = 3 dark roundRects with large accent-colored numbers; row 2 = full-width dk2 bar with takeaway text
- **Effort**: ~15 min JSON
- **Bead**: go-slide-creator-g84

## Rejected Patterns (with reasoning)

### Photo-Dependent (5 patterns rejected)
- Image strip, section divider with photo, photo overlay, numbered list on photo, case study with logos
- **Reason**: shape_grid cannot embed images. Without photos, these collapse to existing patterns.

### Already Covered by Existing Examples (6 patterns rejected)
- Table of Contents → numbered takeaways (swap roundRect to ellipse)
- 5-col category matrix → N-column horizontal panels
- Checklist → numbered rows with ellipse indicators
- Horizontal timeline → existing homePlate timeline
- Numbered steps → existing numbered takeaways
- 3-col category comparison → existing workstream grid

### Decorative/Metaphor (4 patterns rejected)
- Road/path → use timeline instead
- Bucket/funnel → existing funnel SVG diagram
- Pyramid → existing pyramid SVG diagram
- Calendar → 49 cells, wrong tool for the job

### Requires Unsupported Features (2 patterns rejected)
- 3-panel mixed architecture → needs nested grids
- Staircase/escalation → needs diagonal positioning

## Key Insight

> "Our system already supports gear6, gear9, funnel, trapezoid, pie, donut, and can geometries, but we have zero examples using any of them." — Advocate

> "The template bundle is selling visual novelty. Our system should be selling informational clarity." — Devil's Advocate

Both are right. The action is to create examples showcasing underused geometries in realistic business contexts, not to copy decorative templates.

## Validation of Existing Library

The Bain deck analysis confirmed that our existing 8 patterns (chevron flow, MECE matrix, numbered takeaways, workstream grid, comparison, timeline, KPI scorecard, panels) cover the structural archetypes used by top-tier consulting firms. The 4 additions above fill genuine gaps; the rest is parameterization the LLM can handle.
