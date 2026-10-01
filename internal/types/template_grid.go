package types

import "fmt"

// Engine defaults for the template grid (go-slide-creator-5ms8c). A template
// that declares no grid metadata — every bundled template today — renders
// exactly as before: these are the constants the engine used before the keys
// existed.
const (
	// DefaultGridColumns is the advisory column count of the content frame.
	DefaultGridColumns = 12
	// DefaultGridGutterPt is the shape_grid column / row gap applied when a
	// grid sets no gap (shapegrid's built-in 8pt default).
	DefaultGridGutterPt = 8.0
	// DefaultGridTitleGapPt is the distance from the bottom of a measured,
	// top-anchored title's text to the top of the body zone.
	DefaultGridTitleGapPt = 18.0
)

// Bounds the engine accepts for each grid key; values outside them are
// reported by validate-template (TEMPLATE_GRID_INVALID) and ignored.
const (
	maxGridMarginPct  = 25.0
	maxGridColumns    = 24
	maxGridGutterPt   = 72.0
	maxGridTitleGapPt = 144.0
)

// TemplateGrid is the optional template-level grid system declared in the
// template metadata's "grid" block. Zero / omitted fields fall back to the
// engine defaults (DefaultGrid*); margin_pct falls back to the margin the
// layouts' placeholders imply.
type TemplateGrid struct {
	// MarginPct is the outer left / right content margin as a percentage of
	// the slide width. The engine only ever narrows the placeholder-derived
	// content zone to it, never widens it over template artwork.
	MarginPct float64 `json:"margin_pct,omitempty"`
	// Columns is the advisory column count of the content frame (agents align
	// custom shape_grid column widths to it); not enforced.
	Columns int `json:"columns,omitempty"`
	// GutterPt is the default shape_grid column / row gap (points) for grids
	// that set no gap of their own.
	GutterPt float64 `json:"gutter_pt,omitempty"`
	// TitleGapPt is the gap (points) between a measured top-anchored title's
	// text and the start of the body zone.
	TitleGapPt float64 `json:"title_gap_pt,omitempty"`
}

// Problems lists the grid values outside the range the engine accepts.
func (g *TemplateGrid) Problems() []string {
	if g == nil {
		return nil
	}
	var out []string
	if g.MarginPct < 0 || g.MarginPct > maxGridMarginPct {
		out = append(out, fmt.Sprintf("grid.margin_pct must be 0-%.0f, got %g", maxGridMarginPct, g.MarginPct))
	}
	if g.Columns < 0 || g.Columns > maxGridColumns {
		out = append(out, fmt.Sprintf("grid.columns must be 0-%d, got %d", maxGridColumns, g.Columns))
	}
	if g.GutterPt < 0 || g.GutterPt > maxGridGutterPt {
		out = append(out, fmt.Sprintf("grid.gutter_pt must be 0-%.0f, got %g", maxGridGutterPt, g.GutterPt))
	}
	if g.TitleGapPt < 0 || g.TitleGapPt > maxGridTitleGapPt {
		out = append(out, fmt.Sprintf("grid.title_gap_pt must be 0-%.0f, got %g", maxGridTitleGapPt, g.TitleGapPt))
	}
	return out
}

// Sanitized returns a copy with out-of-range fields dropped (so they take the
// engine default), or nil when nothing usable is left.
func (g *TemplateGrid) Sanitized() *TemplateGrid {
	if g == nil {
		return nil
	}
	out := *g
	if out.MarginPct < 0 || out.MarginPct > maxGridMarginPct {
		out.MarginPct = 0
	}
	if out.Columns < 0 || out.Columns > maxGridColumns {
		out.Columns = 0
	}
	if out.GutterPt < 0 || out.GutterPt > maxGridGutterPt {
		out.GutterPt = 0
	}
	if out.TitleGapPt < 0 || out.TitleGapPt > maxGridTitleGapPt {
		out.TitleGapPt = 0
	}
	if out == (TemplateGrid{}) {
		return nil
	}
	return &out
}

// ColumnsOrDefault returns the declared column count or DefaultGridColumns.
func (g *TemplateGrid) ColumnsOrDefault() int {
	if g != nil && g.Columns > 0 {
		return g.Columns
	}
	return DefaultGridColumns
}

// GutterPtOrDefault returns the declared gutter or DefaultGridGutterPt.
func (g *TemplateGrid) GutterPtOrDefault() float64 {
	if g != nil && g.GutterPt > 0 {
		return g.GutterPt
	}
	return DefaultGridGutterPt
}

// TitleGapPtOrDefault returns the declared title gap or DefaultGridTitleGapPt.
func (g *TemplateGrid) TitleGapPtOrDefault() float64 {
	if g != nil && g.TitleGapPt > 0 {
		return g.TitleGapPt
	}
	return DefaultGridTitleGapPt
}

// TemplateGridOf returns the template grid carried by a template's layouts
// (ParseLayouts attaches the sanitized metadata grid to every parsed layout),
// or nil when the template declares none.
func TemplateGridOf(layouts []LayoutMetadata) *TemplateGrid {
	for i := range layouts {
		if layouts[i].TemplateGrid != nil {
			return layouts[i].TemplateGrid
		}
	}
	return nil
}
