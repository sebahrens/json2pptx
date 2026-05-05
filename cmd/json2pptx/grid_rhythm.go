package main

import (
	"fmt"
	"math"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// resolvedGrid holds the computed absolute EMU values derived from a GridConfig
// and slide dimensions. It is the internal representation used to snap bounds.
type resolvedGrid struct {
	TitleBaselineY int64 // Y where title bottom should align
	ContentTopY    int64 // Y where content starts
	ContentBottomY int64 // Y where content ends
	LeftMarginX    int64 // X left content edge
	RightEdgeX     int64 // X right content edge
	SlideWidth     int64
	SlideHeight    int64
}

// defaultContentBottomPct is the default content bottom as a percentage of slide height.
const defaultContentBottomPct = 92.0

// resolveGrid computes absolute EMU positions from a GridConfig, filling in
// defaults from template layout metadata when fields are omitted.
func resolveGrid(cfg *GridConfig, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) *resolvedGrid {
	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}

	rg := &resolvedGrid{
		SlideWidth:  slideWidth,
		SlideHeight: slideHeight,
	}

	// Derive defaults from template layouts if possible.
	var tmplTitleBottom, tmplContentTop, tmplLeftMargin, tmplRightEdge int64
	if len(layouts) > 0 {
		tmplTitleBottom, tmplContentTop, tmplLeftMargin, tmplRightEdge = extractLayoutDefaults(layouts, slideWidth, slideHeight)
	}

	// Title baseline.
	if cfg.TitleBaselinePct > 0 {
		rg.TitleBaselineY = shapegrid.PctToEMU(cfg.TitleBaselinePct, slideHeight)
	} else if tmplTitleBottom > 0 {
		rg.TitleBaselineY = tmplTitleBottom
	} else {
		// Fallback: 8% of slide height (sensible default for 16:9).
		rg.TitleBaselineY = shapegrid.PctToEMU(8, slideHeight)
	}

	// Content top.
	if cfg.ContentTopPct > 0 {
		rg.ContentTopY = shapegrid.PctToEMU(cfg.ContentTopPct, slideHeight)
	} else if tmplContentTop > 0 {
		rg.ContentTopY = tmplContentTop
	} else {
		// Fallback: title baseline + 2% gap.
		rg.ContentTopY = rg.TitleBaselineY + shapegrid.PctToEMU(2, slideHeight)
	}

	// Content bottom.
	if cfg.ContentBottomPct > 0 {
		rg.ContentBottomY = shapegrid.PctToEMU(cfg.ContentBottomPct, slideHeight)
	} else {
		rg.ContentBottomY = shapegrid.PctToEMU(defaultContentBottomPct, slideHeight)
	}

	// Left margin.
	if cfg.LeftMarginPct > 0 {
		rg.LeftMarginX = shapegrid.PctToEMU(cfg.LeftMarginPct, slideWidth)
	} else if tmplLeftMargin > 0 {
		rg.LeftMarginX = tmplLeftMargin
	} else {
		rg.LeftMarginX = shapegrid.PctToEMU(5, slideWidth)
	}

	// Right edge.
	if cfg.RightMarginPct > 0 {
		rg.RightEdgeX = slideWidth - shapegrid.PctToEMU(cfg.RightMarginPct, slideWidth)
	} else if tmplRightEdge > 0 {
		rg.RightEdgeX = tmplRightEdge
	} else {
		// Mirror left margin.
		rg.RightEdgeX = slideWidth - rg.LeftMarginX
	}

	return rg
}

// extractLayoutDefaults scans layout metadata for the most common title bottom,
// content top, and margin positions across layouts. This gives a representative
// baseline when no explicit grid config percentages are provided.
func extractLayoutDefaults(layouts []types.LayoutMetadata, slideWidth, slideHeight int64) (titleBottom, contentTop, leftMargin, rightEdge int64) {
	var titleBottoms, contentTops, leftMargins, rightEdges []int64

	for _, layout := range layouts {
		for _, ph := range layout.Placeholders {
			switch ph.Type {
			case types.PlaceholderTitle:
				tb := ph.Bounds.Y + ph.Bounds.Height
				if tb > 0 {
					titleBottoms = append(titleBottoms, tb)
				}
			case types.PlaceholderBody, types.PlaceholderContent:
				if ph.Bounds.Y > 0 {
					contentTops = append(contentTops, ph.Bounds.Y)
				}
				if ph.Bounds.X > 0 {
					leftMargins = append(leftMargins, ph.Bounds.X)
				}
				re := ph.Bounds.X + ph.Bounds.Width
				if re > 0 {
					rightEdges = append(rightEdges, re)
				}
			}
		}
	}

	titleBottom = medianInt64(titleBottoms)
	contentTop = medianInt64(contentTops)
	leftMargin = medianInt64(leftMargins)
	rightEdge = medianInt64(rightEdges)
	return
}

// medianInt64 returns the median of a sorted int64 slice, or 0 if empty.
func medianInt64(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	// Simple insertion sort for small slices.
	sorted := make([]int64, len(vals))
	copy(sorted, vals)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted[len(sorted)/2]
}

// snapBoundsToGrid adjusts shape grid bounds to align with the resolved grid.
// It returns the snapped bounds and a list of adjustments made.
func snapBoundsToGrid(bounds pptx.RectEmu, rg *resolvedGrid) (pptx.RectEmu, []gridAdjustment) {
	var adjustments []gridAdjustment
	snapped := bounds

	// Snap top edge to content top.
	if snapped.Y != rg.ContentTopY {
		adj := gridAdjustment{
			Field:    "content_top",
			Original: snapped.Y,
			Snapped:  rg.ContentTopY,
		}
		delta := rg.ContentTopY - snapped.Y
		snapped.Y = rg.ContentTopY
		snapped.CY -= delta
		if snapped.CY < 0 {
			snapped.CY = 0
		}
		adjustments = append(adjustments, adj)
	}

	// Snap bottom edge to content bottom.
	bottom := snapped.Y + snapped.CY
	if bottom != rg.ContentBottomY {
		adj := gridAdjustment{
			Field:    "content_bottom",
			Original: bottom,
			Snapped:  rg.ContentBottomY,
		}
		snapped.CY = rg.ContentBottomY - snapped.Y
		if snapped.CY < 0 {
			snapped.CY = 0
		}
		adjustments = append(adjustments, adj)
	}

	// Snap left margin.
	if snapped.X != rg.LeftMarginX {
		adj := gridAdjustment{
			Field:    "left_margin",
			Original: snapped.X,
			Snapped:  rg.LeftMarginX,
		}
		delta := rg.LeftMarginX - snapped.X
		snapped.X = rg.LeftMarginX
		snapped.CX -= delta
		if snapped.CX < 0 {
			snapped.CX = 0
		}
		adjustments = append(adjustments, adj)
	}

	// Snap right edge.
	right := snapped.X + snapped.CX
	if right != rg.RightEdgeX {
		adj := gridAdjustment{
			Field:    "right_edge",
			Original: right,
			Snapped:  rg.RightEdgeX,
		}
		snapped.CX = rg.RightEdgeX - snapped.X
		if snapped.CX < 0 {
			snapped.CX = 0
		}
		adjustments = append(adjustments, adj)
	}

	return snapped, adjustments
}

// gridAdjustment records a single snap adjustment for diagnostic purposes.
type gridAdjustment struct {
	Field    string // which edge was adjusted
	Original int64  // original EMU value
	Snapped  int64  // snapped EMU value
}

// gridToContentZone converts a resolvedGrid to a shapegrid.ContentZone, making
// the grid compatible with the existing bounds computation pipeline.
func gridToContentZone(rg *resolvedGrid) *shapegrid.ContentZone {
	return &shapegrid.ContentZone{
		TitleBottom: rg.TitleBaselineY,
		FooterTop:   rg.ContentBottomY,
		LeftMargin:  rg.LeftMarginX,
		RightEdge:   rg.RightEdgeX,
		SlideWidth:  rg.SlideWidth,
		SlideHeight: rg.SlideHeight,
	}
}

// validateGridConfig checks a GridConfig for out-of-range or contradictory values.
func validateGridConfig(cfg *GridConfig) error {
	if cfg == nil {
		return nil
	}

	if cfg.Columns < 0 || cfg.Columns > 24 {
		return fmt.Errorf("grid.columns must be 1-24, got %d", cfg.Columns)
	}
	if cfg.GutterEMU < 0 {
		return fmt.Errorf("grid.gutter_emu must be >= 0, got %d", cfg.GutterEMU)
	}

	// Percentage fields must be in (0, 100] if set.
	pctFields := []struct {
		name string
		val  float64
	}{
		{"title_baseline_pct", cfg.TitleBaselinePct},
		{"content_top_pct", cfg.ContentTopPct},
		{"content_bottom_pct", cfg.ContentBottomPct},
		{"left_margin_pct", cfg.LeftMarginPct},
		{"right_margin_pct", cfg.RightMarginPct},
	}
	for _, f := range pctFields {
		if f.val != 0 && (f.val < 0 || f.val > 100) {
			return fmt.Errorf("grid.%s must be 0-100, got %.1f", f.name, f.val)
		}
	}

	// Ordering constraints.
	if cfg.TitleBaselinePct > 0 && cfg.ContentTopPct > 0 && cfg.TitleBaselinePct >= cfg.ContentTopPct {
		return fmt.Errorf("grid.title_baseline_pct (%.1f) must be < grid.content_top_pct (%.1f)",
			cfg.TitleBaselinePct, cfg.ContentTopPct)
	}
	if cfg.ContentTopPct > 0 && cfg.ContentBottomPct > 0 && cfg.ContentTopPct >= cfg.ContentBottomPct {
		return fmt.Errorf("grid.content_top_pct (%.1f) must be < grid.content_bottom_pct (%.1f)",
			cfg.ContentTopPct, cfg.ContentBottomPct)
	}
	if cfg.LeftMarginPct > 0 && cfg.RightMarginPct > 0 && cfg.LeftMarginPct+cfg.RightMarginPct >= 100 {
		return fmt.Errorf("grid.left_margin_pct (%.1f) + grid.right_margin_pct (%.1f) must be < 100",
			cfg.LeftMarginPct, cfg.RightMarginPct)
	}

	return nil
}

// gridViolationThresholdEMU is the minimum deviation (in EMU) between a
// layout's natural position and the grid position before a violation is emitted.
// ~0.05 inch = 45720 EMU. Small rounding differences are ignored.
const gridViolationThresholdEMU int64 = 45720

// detectGridViolations compares layout placeholder positions against the
// resolved grid and emits grid_violation fit-findings for layouts that can't
// reconcile within the threshold.
func detectGridViolations(rg *resolvedGrid, layouts []types.LayoutMetadata, slides []SlideInput) []patterns.FitFinding {
	var findings []patterns.FitFinding

	for si, slide := range slides {
		layout := findLayoutForSlide(&slide, layouts)
		if layout == nil {
			continue
		}

		for _, ph := range layout.Placeholders {
			switch ph.Type {
			case types.PlaceholderTitle:
				titleBottom := ph.Bounds.Y + ph.Bounds.Height
				deviation := absInt64(titleBottom - rg.TitleBaselineY)
				if deviation > gridViolationThresholdEMU {
					findings = append(findings, patterns.FitFinding{
						ValidationError: patterns.ValidationError{
							Code: "grid_violation",
							Path: slidepath.Content(si, "title"),
							Message: fmt.Sprintf(
								"title bottom (%.1f%%) deviates from grid title_baseline (%.1f%%) by %.2f%%",
								emuToPct(titleBottom, rg.SlideHeight),
								emuToPct(rg.TitleBaselineY, rg.SlideHeight),
								emuToPct(deviation, rg.SlideHeight),
							),
							Fix: &patterns.FixSuggestion{
								Kind: "reposition_shape",
								Params: map[string]any{
									"field":       "title_baseline",
									"current_emu": titleBottom,
									"target_emu":  rg.TitleBaselineY,
								},
							},
						},
						Action:   "review",
						Measured: &patterns.Extent{HeightEMU: titleBottom},
						Allowed:  &patterns.Extent{HeightEMU: rg.TitleBaselineY},
					})
				}

			case types.PlaceholderBody, types.PlaceholderContent:
				// Check content top alignment.
				deviation := absInt64(ph.Bounds.Y - rg.ContentTopY)
				if deviation > gridViolationThresholdEMU {
					findings = append(findings, patterns.FitFinding{
						ValidationError: patterns.ValidationError{
							Code: "grid_violation",
							Path: slidepath.Content(si, ph.ID),
							Message: fmt.Sprintf(
								"content top (%.1f%%) deviates from grid content_top (%.1f%%) by %.2f%%",
								emuToPct(ph.Bounds.Y, rg.SlideHeight),
								emuToPct(rg.ContentTopY, rg.SlideHeight),
								emuToPct(deviation, rg.SlideHeight),
							),
							Fix: &patterns.FixSuggestion{
								Kind: "reposition_shape",
								Params: map[string]any{
									"field":       "content_top",
									"current_emu": ph.Bounds.Y,
									"target_emu":  rg.ContentTopY,
								},
							},
						},
						Action:   "review",
						Measured: &patterns.Extent{HeightEMU: ph.Bounds.Y},
						Allowed:  &patterns.Extent{HeightEMU: rg.ContentTopY},
					})
				}

				// Check left margin alignment.
				deviation = absInt64(ph.Bounds.X - rg.LeftMarginX)
				if deviation > gridViolationThresholdEMU {
					findings = append(findings, patterns.FitFinding{
						ValidationError: patterns.ValidationError{
							Code: "grid_violation",
							Path: slidepath.Content(si, ph.ID),
							Message: fmt.Sprintf(
								"content left (%.1f%%) deviates from grid left_margin (%.1f%%) by %.2f%%",
								emuToPct(ph.Bounds.X, rg.SlideWidth),
								emuToPct(rg.LeftMarginX, rg.SlideWidth),
								emuToPct(deviation, rg.SlideWidth),
							),
							Fix: &patterns.FixSuggestion{
								Kind: "reposition_shape",
								Params: map[string]any{
									"field":       "left_margin",
									"current_emu": ph.Bounds.X,
									"target_emu":  rg.LeftMarginX,
								},
							},
						},
						Action:   "review",
						Measured: &patterns.Extent{HeightEMU: ph.Bounds.X},
						Allowed:  &patterns.Extent{HeightEMU: rg.LeftMarginX},
					})
				}
			}
		}
	}

	return findings
}

// emuToPct converts an EMU value to a percentage of a reference dimension.
func emuToPct(emu, ref int64) float64 {
	if ref <= 0 {
		return 0
	}
	return math.Round(float64(emu)/float64(ref)*10000) / 100
}

// absInt64 returns the absolute value of an int64.
func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
