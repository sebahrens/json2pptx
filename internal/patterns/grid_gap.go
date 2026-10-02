package patterns

import "github.com/sebahrens/json2pptx/internal/types"

// Template-grid spacing for pattern expansion (go-slide-creator-5ms8c).
//
// Every pattern-internal gap — the Gap / ColGap / RowGap a pattern writes on
// its shape_grid and the same gaps its content sizing subtracts — is authored
// against the engine's 8pt gutter (types.DefaultGridGutterPt). A template that
// declares grid.gutter_pt scales them all by gutter_pt / 8, so a template with
// a 12pt gutter opens every pattern's gaps by 1.5x (an 8pt gutter becomes 12,
// a 4pt hairline-adjacent gap 6) and patterns, plain shape_grids and the
// template frame share one spacing system. Without a declared gutter the
// scale is exactly 1 and every gap is returned unchanged, so output stays
// byte-identical.

// gapScaleFloorPt is the largest gap that never scales: hairline / sentinel
// gaps (0.01pt "no gap", 0.1pt numeral columns, 1pt rule spacing) are part of
// a pattern's drawing, not its gutter.
const gapScaleFloorPt = 1.0

// TemplateGrid returns the template grid this expansion runs under: the
// sanitized metadata grid, or a grid carrying only the content zone's gutter
// when the caller passed a zone but no metadata. nil when the template
// declares none.
func (c ExpandContext) TemplateGrid() *types.TemplateGrid {
	if c.Metadata != nil {
		if g := c.Metadata.Grid.Sanitized(); g != nil {
			return g
		}
	}
	if c.ContentZone != nil && c.ContentZone.GutterPt > 0 {
		return &types.TemplateGrid{GutterPt: c.ContentZone.GutterPt}
	}
	return nil
}

// GutterPt is the template grid's gutter (points), default
// types.DefaultGridGutterPt (8pt).
func (c ExpandContext) GutterPt() float64 {
	if c.ContentZone != nil && c.ContentZone.GutterPt > 0 {
		return c.ContentZone.GutterPt
	}
	return c.TemplateGrid().GutterPtOrDefault()
}

// GapScale is the factor pattern-internal gaps scale by: the template
// gutter over the engine's 8pt default; exactly 1 when no gutter is declared.
func (c ExpandContext) GapScale() float64 {
	g := c.GutterPt()
	if g <= 0 || g == types.DefaultGridGutterPt {
		return 1
	}
	return g / types.DefaultGridGutterPt
}

// Gap scales a pattern's authored gap (points, designed against the 8pt
// default gutter) to the template grid. Hairline gaps (<= 1pt) and every gap
// under a template without a declared gutter are returned unchanged.
func (c ExpandContext) Gap(pt float64) float64 {
	if pt <= gapScaleFloorPt {
		return pt
	}
	s := c.GapScale()
	if s == 1 {
		return pt
	}
	return pt * s
}

// gridDefaultArea narrows a default content size (EMU, derived from the
// shape-grid default bounds because the caller passed no layout bounds) to the
// template grid: a declared margin_pct caps the width at the margin frame,
// and a title_gap_pt above the 18pt default takes the extra gap off the
// height, mirroring how the real content zone moves. Without a declared grid
// the size is returned unchanged.
func (c ExpandContext) gridDefaultArea(w, h, slideW int64) (int64, int64) {
	g := c.TemplateGrid()
	if g == nil {
		return w, h
	}
	if g.MarginPct > 0 && slideW > 0 {
		if frame := slideW - 2*int64(g.MarginPct/100*float64(slideW)); frame > 0 && frame < w {
			w = frame
		}
	}
	if extra := g.TitleGapPt - types.DefaultGridTitleGapPt; extra > 0 {
		h = max(h-int64(extra*float64(types.EMUPerPoint)), 0)
	}
	return w, h
}
