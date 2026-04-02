# Panel Layout Diagrams (icon_columns, icon_rows, stat_cards)

Three new visual diagram layouts rendered as a single `panel_layout` diagram type with layout mode switching.

## Scope

This specification covers:
- `internal/svggen/panel_layout.go` — one diagram type with three layout modes
- `internal/svggen/icon_loader.go` — icon loading from URL, base64, or inline SVG
- Addition of `DrawImage(img image.Image, rect Rect)` to `SVGBuilder`
- Registry aliases mapping `icon_columns`, `icon_rows`, `stat_cards` → `panel_layout`

## Visual Reference

Three slide designs from `New-layouts/`:

| Slide | Layout Mode | Description |
|-------|------------|-------------|
| Slide 1 | `columns` | 2-5 vertical panels: icon top → heading → bullet body |
| Slide 2 | `rows` | 2-5 horizontal panels: icon left → text right |
| Slide 3 | `stat_cards` | 2-5 number tiles + optional callout row below |

## Architecture Decision: One Type, Three Modes

**Decision:** Single `PanelLayoutDiagram` registered as `"panel_layout"` with three aliases.

**Rationale:** The three layouts share identical data structures (icon, title, body per panel). Only spatial arrangement differs. One type avoids ~80% code duplication in parsing, validation, icon rendering, and card drawing. Aliases give ergonomic separate names. The `req.Type` field carries the original alias string, allowing mode inference without Registry changes.

**Precedent:** `process_flow.go` already uses a `direction` field to switch between horizontal and vertical modes within one diagram type.

## Data Types

```go
type PanelLayoutConfig struct {
    ChartConfig
    Gap               float64 // between panels (default: style.Spacing.MD)
    PanelCornerRadius float64 // default: 6
    PanelOpacity      float64 // background fill opacity (default: 0.08)
    IconSize          float64 // 0 = auto-calculate
    SeparatorWidth    float64 // vertical separator for columns mode; 0 = none
}

type PanelItem struct {
    Icon  string   // URL, data:URI, inline SVG, or "" for fallback circle
    Title string   // heading text
    Body  string   // body text; newlines = bullet points for columns mode
    Value string   // hero number for stat_cards mode (e.g., "10%", "$1.2M")
}

type PanelCallout struct {
    Icon string
    Text string
}

type PanelLayoutData struct {
    Title    string
    Subtitle string
    Layout   string         // "columns", "rows", "stat_cards"
    Panels   []PanelItem
    Callout  *PanelCallout  // optional, primarily for stat_cards
    Footnote string
}
```

## JSON Input Format

Unified schema with `layout` discriminator:

```json
{
  "type": "panel_layout",
  "title": "Our Approach",
  "data": {
    "layout": "columns",
    "panels": [
      {
        "icon": "https://example.com/icon.svg",
        "title": "Discovery",
        "body": "- Stakeholder interviews\n- Data analysis\n- Gap assessment"
      }
    ],
    "callout": {
      "icon": "data:image/svg+xml;base64,PHN2Zy...",
      "text": "All metrics measured against Q4 2025 baseline."
    }
  }
}
```

**Alias inference:** When `data.layout` is absent, infer from `req.Type`:
- `icon_columns` → `"columns"`
- `icon_rows` → `"rows"`
- `stat_cards` → `"stat_cards"`
- `panel_layout` → defaults to `"columns"`

## Dimension Algorithms

All values in points. Standard canvas is configurable via `output.width`/`output.height`.

### Common Setup

```
plotArea = config.PlotArea()              // margins subtracted
headerH = titleHeight(style, title, subtitle)
footnoteH = FootnoteReservedHeight(style) if footnote != ""
plotArea.Y += headerH
plotArea.H -= headerH + footnoteH
```

### Layout: `columns` (Slide 1)

```
N = len(panels), capped at 6
gap = config.Gap

columnRects = SplitHorizontal(plotArea, N, gap)
// cardW = (plotArea.W - (N-1)*gap) / N

For each column (cardW × cardH):
    padding = style.Spacing.MD
    contentRect = card.Inset(padding, padding, padding, padding)

    iconZoneH  = contentRect.H * 0.20
    iconSize   = clamp(min(iconZoneH * 0.8, contentRect.W * 0.4), 24, 56)

    titleZoneH = contentRect.H * 0.12
    titleFont  = ClampFontSize(title, contentRect.W, style.SizeHeading, compact.SizeSmall)

    bodyZoneH  = contentRect.H - iconZoneH - titleZoneH
    bodyFont   = ClampFontSize(bodyLine, contentRect.W, style.SizeBody, compact.SizeCaption)

    Separator: 1pt vertical line between columns (not after last)
    Left accent: 2pt accent-colored bar on left edge
    Background: #F5F5F5 (or theme lt2 tinted) at PanelOpacity
```

### Layout: `rows` (Slide 2)

```
N = len(panels), capped at 6
gap = config.Gap

rowRects = SplitVertical(plotArea, N, gap)
// rowH = (plotArea.H - (N-1)*gap) / N

For each row (rowW × rowH):
    padding = style.Spacing.MD
    contentRect = row.Inset(padding, padding, padding, padding)

    iconZoneW  = contentRect.W * 0.12
    iconSize   = clamp(min(contentRect.H * 0.6, iconZoneW * 0.7), 24, 48)
    // icon vertically centered in zone

    dividerX   = contentRect.X + iconZoneW + padding/2
    // 1pt vertical divider line

    textZoneX  = dividerX + padding/2
    textZoneW  = contentRect.X + contentRect.W - textZoneX

    titleFont  = ClampFontSize(title, textZoneW, style.SizeHeading, compact.SizeSmall)
    bodyFont   = ClampFontSize(bodyLine, textZoneW, style.SizeBody, compact.SizeCaption)

    Background: #F5F5F5 rounded rect per row
```

### Layout: `stat_cards` (Slide 3)

```
N = len(panels), capped at 6
gap = config.Gap

If callout present:
    calloutH = clamp(plotArea.H * 0.18, 60, 80)
    cardsH = plotArea.H - calloutH - gap
Else:
    cardsH = plotArea.H

cardsArea = Rect{plotArea.X, plotArea.Y, plotArea.W, cardsH}
tileRects = SplitHorizontal(cardsArea, N, gap)

For each tile (tileW × tileH):
    padding = style.Spacing.MD
    contentRect = tile.Inset(padding, padding, padding, padding)

    valueZoneH = contentRect.H * 0.55
    heroFont   = ClampFontSize(value, contentRect.W * 0.85,
                    style.SizeTitle * 2.0, compact.SizeHeading)
    // Value centered horizontally and vertically in zone

    descZoneH  = contentRect.H * 0.45
    descFont   = ClampFontSize(desc, contentRect.W * 0.9,
                    style.SizeBody, compact.SizeCaption)
    // Description centered below value

    Background: #F5F5F5 rounded rect

Callout row (if present):
    calloutRect = Rect{plotArea.X, cardsArea.Y + cardsH + gap, plotArea.W, calloutH}
    iconSize = clamp(calloutH * 0.6, 24, 48)
    // Icon on left, text on right (same pattern as rows layout)
```

## Icon Loading Strategy

**Decision:** Rasterize icons to `image.Image`, embed via `SVGBuilder.DrawImage`.

### Pipeline

```
Icon string → classify (URL / data:URI / inline SVG / empty)
    │
    ├─ URL: HTTP GET (5s timeout, 512KB limit) → []byte
    ├─ data:URI: base64 decode → []byte
    ├─ inline SVG: treat as []byte directly
    └─ empty/error: colored circle fallback
    │
    ▼
Parse as SVG via canvas library (canvas.NewFromReader)
    │
    ▼
Rasterize to image.Image at target size (e.g., 128×128 px for 64pt icon at 2×)
    │
    ▼
DrawImage(img, iconRect) on SVGBuilder
```

### Fallback

If icon loading fails for any reason:
- Draw a colored circle using accent color from style palette
- Draw the first letter of the panel title, centered, in white
- Log a warning (slog.Warn) with the failure reason

### SVGBuilder Addition

```go
// DrawImage draws a raster image within the given rectangle.
// The image is scaled to fit the rectangle dimensions.
func (b *SVGBuilder) DrawImage(img image.Image, r Rect) *SVGBuilder
```

Implementation: ~20 LOC wrapper around `canvas.Context.DrawImage` with pt→mm conversion and Y-flip. Already proven in `internal/preview/pdf.go:302`.

## Registry Integration

In `init.go`:

```go
// builtinDiagrams — add:
&PanelLayoutDiagram{BaseDiagram{typeID: "panel_layout"}},

// builtinAliases — add:
"icon_columns":  "panel_layout",
"icon_rows":     "panel_layout",
"stat_cards":    "panel_layout",
"panel":         "panel_layout",
"icon_panel":    "panel_layout",
"number_tiles":  "panel_layout",
"callout_cards": "panel_layout",
```

## Implementation Phases

### Phase 1: Foundation (columns layout, colored-circle icons)
1. `panel_layout.go`: PanelLayoutDiagram with Validate/Render/RenderWithBuilder
2. `drawColumns` method with SplitHorizontal
3. Colored-circle icon placeholders
4. Register in init.go with aliases
5. Unit tests + golden tests

### Phase 2: Additional layouts
6. `drawRows` method
7. `drawStatCards` method with optional callout
8. Tests for each layout mode

### Phase 3: Real icon support
9. `builder.go`: Add `DrawImage` method (~20 LOC)
10. `icon_loader.go`: URL/base64/inline SVG → image.Image pipeline
11. Wire into panel_layout.go, replacing colored-circle placeholders
12. Keep colored circle as fallback
13. Icon loader tests

### Phase 4: Polish
14. Font clamping for long titles/values
15. Narrow-canvas adaptation (half-width placeholders)
16. 5+ panel handling
17. Update INPUT_FORMAT.md with new diagram types

## File Summary

| File | Action | ~LOC |
|------|--------|------|
| `internal/svggen/panel_layout.go` | Create | ~450 |
| `internal/svggen/panel_layout_test.go` | Create | ~250 |
| `internal/svggen/icon_loader.go` | Create | ~180 |
| `internal/svggen/icon_loader_test.go` | Create | ~120 |
| `internal/svggen/builder.go` | Modify | +30 |
| `internal/svggen/builder_test.go` | Modify | +20 |
| `internal/svggen/init.go` | Modify | +8 |
| `docs/INPUT_FORMAT.md` | Modify | +60 |
| **Total** | | **~1120** |

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Icon rasterization latency | Icons are small (32-64px); rasterization is <50ms. No caching in v1; add if profiling shows need. |
| `DrawImage` changes SVGBuilder API | Thin wrapper, follows existing `Draw*` pattern, already proven via `preview/pdf.go` |
| Network I/O in render path | 5s timeout + 512KB limit. Fallback always works. Icon loading is best-effort. |
| `canvas.NewFromReader` SVG parsing gaps | Noun Project icons are simple single-path SVGs. Fallback to `rsvg-convert` if canvas parser fails. |
| Panel text overflow | Use existing `ClampFontSize` + `TruncateToWidth` — same strategy as KPI dashboard |
