# 24 — Virtual Layouts

## Summary

Virtual layouts allow slides with `shape_grid` content to automatically select a suitable base layout from the template without requiring the caller to specify a `layout_id`. The system prefers "blank" layouts that provide a title placeholder and an open canvas area for the grid.

## Motivation

Shape grids draw arbitrary preset geometry shapes onto a slide. They do not use the template's body or content placeholders — they need open space. Requiring callers to know which layout is "blank enough" couples them to template internals. Virtual layout resolution decouples grid content from template structure.

## Concepts

### Virtual Layout

A *virtual layout* is not a new layout type inside the PPTX template. It is a runtime selection strategy: given a slide that carries a `shape_grid`, the system picks the best existing layout and computes the available canvas bounds automatically.

### Grid-on-Blank Architecture

The core idea is **grid-on-blank**: overlay a grid of preset geometry shapes onto a blank (or near-blank) layout that has a title but no body placeholder consuming the canvas.

## Resolution Algorithm

`resolveVirtualLayout(layouts)` runs when `needsVirtualLayout(slide)` returns `true`:

1. **Scan layouts** for candidates tagged `"blank"` or `"blank-title"`.
2. **Priority order:**
   - A `blank` layout that has a title placeholder (ideal: title + open canvas).
   - A synthesized `blank-title` layout (title-only, no body/content).
   - Any layout with a body or content placeholder (fallback — grid overlays the body area).
3. **Compute bounds:**
   - If the chosen layout has a title placeholder, call `BoundsFromTitleAndFooter()` to derive the grid area between the title bottom edge and the footer top edge, with a 9pt gap.
   - If no title is found, use `DefaultBounds()` (5%, 18%, 90%, 72% of slide dimensions).
4. **Return** the selected `layout_id` and computed bounds in EMU.

### When Virtual Layout Triggers

`needsVirtualLayout(slide)` returns `true` when **all** of:

- The slide has a non-nil `shape_grid`.
- The slide has no explicit `layout_id`, **or** `slide_type` is `"blank"` or `"virtual"`.

If the caller provides an explicit `layout_id` (and `slide_type` is not blank/virtual), the system respects it and skips virtual resolution.

## Bounds Computation

### BoundsFromTitleAndFooter

Given title and footer placeholder rectangles (in EMU):

```
grid_top    = title.Y + title.Height + gap
grid_bottom = footer.Y - gap
grid_left   = title.X              (aligned with title)
grid_width  = title.Width          (same width as title)
grid_height = grid_bottom - grid_top
```

The gap default is 9pt (114300 EMU).

### BoundsFromPlaceholder

When falling back to a layout with a body placeholder, the grid occupies exactly the body placeholder's bounding rectangle.

### DefaultBounds

When no placeholder geometry is available:

| Field  | Value | EMU (16:9)  |
|--------|-------|-------------|
| X      | 5%    | 609,600     |
| Y      | 18%   | 1,234,440   |
| Width  | 90%   | 10,972,800  |
| Height | 72%   | 4,937,760   |

## Caller-Supplied Bounds Override

If the JSON input includes `shape_grid.bounds`, those percentages override the auto-computed bounds entirely. This allows callers to position the grid anywhere on the slide regardless of layout.

## Key Files

| File | Purpose |
|------|---------|
| `cmd/json2pptx/shape_grid.go` | `resolveVirtualLayout()`, `needsVirtualLayout()`, `pickBlankLayout()` |
| `cmd/json2pptx/json_mode.go` | Integration point calling virtual layout resolution |
| `internal/shapegrid/bounds.go` | `BoundsFromTitleAndFooter()`, `BoundsFromPlaceholder()`, `DefaultBounds()` |
| `internal/shapegrid/grid.go` | Grid resolution engine |

## Acceptance Criteria

- Slides with `shape_grid` and no `layout_id` automatically select a blank layout.
- Grid bounds are derived from the title/footer geometry of the chosen layout.
- Caller-supplied `bounds` in `shape_grid` override auto-computed bounds.
- Templates without a suitable blank layout fall back to body placeholder bounds or defaults.
