package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/template"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Template-level grid system (go-slide-creator-5ms8c).
//
// A template may declare a "grid" block in its metadata (margin_pct, columns,
// gutter_pt, title_gap_pt; types.TemplateGrid). ParseLayouts attaches it to
// every layout, so every geometry path that starts from the parsed layouts —
// generation, preflight, fit report, previews — applies it identically:
//
//   - margin_pct narrows the content zone's left / right edges (never widens
//     them over template artwork);
//   - gutter_pt is the default shape_grid column / row gap (ContentZone.GutterPt);
//   - title_gap_pt is the measured-title-to-body gap (reserveMeasuredTitle);
//   - columns is advisory, reported for agents sizing their own columns.
//
// The same content frame is what grid_violation checks rendered content
// against by default (detectContentFrameViolations).

// applyTemplateGrid applies the template grid's margin and gutter to the
// resolved zone (and to virtual override bounds).
func applyTemplateGrid(g GridGeometry, layouts []types.LayoutMetadata, slideWidth int64) GridGeometry {
	grid := types.TemplateGridOf(layouts)
	if grid == nil || g.Zone == nil {
		return g
	}
	zone := *g.Zone
	zone.GutterPt = grid.GutterPt
	if w := templateGridWidth(zone, slideWidth); grid.MarginPct > 0 && w > 0 {
		margin := shapegrid.PctToEMU(grid.MarginPct, w)
		zone.LeftMargin = maxI64(zone.LeftMargin, margin)
		zone.RightEdge = minI64(zone.RightEdge, w-margin)
		if g.OverrideBounds != nil {
			b := *g.OverrideBounds
			if b.X < zone.LeftMargin {
				b.CX -= zone.LeftMargin - b.X
				b.X = zone.LeftMargin
			}
			if b.X+b.CX > zone.RightEdge {
				b.CX = zone.RightEdge - b.X
			}
			b.CX = maxI64(b.CX, 0)
			g.OverrideBounds = &b
		}
	}
	g.Zone = &zone
	return g
}

func templateGridWidth(zone shapegrid.ContentZone, slideWidth int64) int64 {
	if zone.SlideWidth > 0 {
		return zone.SlideWidth
	}
	return slideWidth
}

// contentFrame is the set of lines every content slide's elements should sit
// on: the title baseline (title box bottom), the content top (the body line)
// and the left content edge.
type contentFrame struct {
	TitleBottomY int64
	ContentTopY  int64
	LeftX        int64
	SlideWidth   int64
	SlideHeight  int64
	// Source names the frame in messages and fix params: "template" (the
	// template's reference one-content layout + metadata grid) or
	// "deck_grid" (the deck's own grid config).
	Source string
	// Action is the finding weight: info for the template frame (an advisory
	// layer reported by default), review for a grid the deck asked for.
	Action string
	// TopTolerance / LeftTolerance are the deviations (EMU) ignored.
	TopTolerance, LeftTolerance int64
	// CheckGrids measures shape_grid / pattern blocks. Off for a deck grid,
	// which generation snaps every grid block to.
	CheckGrids bool
}

// Template-frame tolerances: a content top within one 12pt line of the body
// line still reads as the same start line (full-area grids on p-style start
// ~11pt above it by design, see bodyStartTop); left edges must agree within
// 6pt.
const (
	templateFrameTopToleranceEMU  int64 = 12 * 12700
	templateFrameLeftToleranceEMU int64 = 6 * 12700
)

// templateContentFrame is the template's content frame
// (template.ResolveTemplateGrid): the reference one-content layout's title
// box bottom, body placeholder top (else title bottom + the grid's title gap)
// and title left edge, narrowed to the metadata margin when declared.
func templateContentFrame(layouts []types.LayoutMetadata, slideWidth, slideHeight int64) (contentFrame, bool) {
	if slideWidth <= 0 {
		slideWidth = shapegrid.DefaultSlideWidthEMU
	}
	if slideHeight <= 0 {
		slideHeight = shapegrid.DefaultSlideHeightEMU
	}
	resolved := template.ResolveTemplateGrid(layouts, slideWidth, slideHeight)
	if resolved.Frame == nil {
		return contentFrame{}, false
	}
	return contentFrame{
		TitleBottomY:  resolved.Frame.TitleBottomEMU,
		ContentTopY:   resolved.Frame.ContentTopEMU,
		LeftX:         resolved.Frame.LeftEMU,
		SlideWidth:    slideWidth,
		SlideHeight:   slideHeight,
		Source:        "template",
		Action:        "info",
		TopTolerance:  templateFrameTopToleranceEMU,
		LeftTolerance: templateFrameLeftToleranceEMU,
		CheckGrids:    true,
	}, true
}

// deckGridFrame is the frame of a deck-level grid config: deviations from a
// grid the author asked for are review findings at the legacy 0.05in
// threshold.
func deckGridFrame(rg *resolvedGrid) contentFrame {
	return contentFrame{
		TitleBottomY:  rg.TitleBaselineY,
		ContentTopY:   rg.ContentTopY,
		LeftX:         rg.LeftMarginX,
		SlideWidth:    rg.SlideWidth,
		SlideHeight:   rg.SlideHeight,
		Source:        "deck_grid",
		Action:        "review",
		TopTolerance:  gridViolationThresholdEMU,
		LeftTolerance: gridViolationThresholdEMU,
	}
}

// contentFrameFindings collects grid_violation findings for a deck: against
// the deck's grid when one is configured and valid, else against the
// template's content frame (info weight, reported by default).
func contentFrameFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []patterns.FitFinding {
	if input == nil || len(layouts) == 0 {
		return nil
	}
	if input.Grid != nil {
		if validateGridConfig(input.Grid) != nil {
			return nil
		}
		return detectGridViolations(resolveGrid(input.Grid, layouts, slideWidth, slideHeight), layouts, input.Slides)
	}
	frame, ok := templateContentFrame(layouts, slideWidth, slideHeight)
	if !ok {
		return nil
	}
	// Slides without a layout_id are measured on the layout generation will
	// pick for them (the same prediction the placeholder checks use).
	return detectContentFrameViolationsOn(frame, layouts, input.Slides, predictSlideLayouts(input, layouts))
}

// detectContentFrameViolations checks the RENDERED content of every content
// slide against the frame: the title placeholder the slide fills (title
// baseline), the body / content placeholders it fills (content top, left
// edge), its shape_grid / expanded pattern block (top and left of the
// resolved cells) and its takeaway band (left edge). Title, section and
// closing slides and blank canvases are exempt: they position content on
// purpose. Unfilled placeholders are never measured.
func detectContentFrameViolations(frame contentFrame, layouts []types.LayoutMetadata, slides []SlideInput) []patterns.FitFinding {
	return detectContentFrameViolationsOn(frame, layouts, slides, nil)
}

// detectContentFrameViolationsOn is detectContentFrameViolations with the
// layout each slide renders on predicted where it names none.
func detectContentFrameViolationsOn(frame contentFrame, layouts []types.LayoutMetadata, slides []SlideInput, predicted []*types.LayoutMetadata) []patterns.FitFinding {
	var out []patterns.FitFinding
	for si := range slides {
		slide := &slides[si]
		layout := findLayoutByID(layouts, canonicalGridLayoutID(slide.LayoutID, layouts))
		if layout == nil && si < len(predicted) {
			layout = predicted[si]
		}
		if frameExemptSlide(slide, layout, layouts) {
			continue
		}
		out = append(out, framePlaceholderFindings(frame, si, slide, layout, layouts)...)
		out = append(out, frameGridFindings(frame, si, slide, layouts)...)
		out = append(out, frameTakeawayFindings(frame, si, slide, layout, layouts)...)
	}
	return out
}

// frameExemptSlide reports slides that position content on purpose.
func frameExemptSlide(slide *SlideInput, layout *types.LayoutMetadata, layouts []types.LayoutMetadata) bool {
	switch strings.ToLower(strings.TrimSpace(slide.SlideType)) {
	case "title", "section", "closing":
		return true
	}
	if slide.LayoutID != "" && isBlankCanvasLayout(slide.LayoutID, layouts) {
		return true
	}
	return layout != nil && isGridExcludedLayout(*layout)
}

func framePlaceholderFindings(frame contentFrame, si int, slide *SlideInput, layout *types.LayoutMetadata, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if layout == nil {
		return nil
	}
	// Generation clamps body placeholders into the chrome frame's content
	// column (side artwork insets it); measure where they render.
	chrome := slideChromeFrame(*slide, layout.ID, layouts, frame.SlideWidth, frame.SlideHeight)
	filled := map[string]bool{}
	for _, c := range slide.Content {
		filled[c.PlaceholderID] = true
	}
	var out []patterns.FitFinding
	bodyChecked := false
	for _, ph := range layout.Placeholders {
		if !filled[ph.ID] {
			continue
		}
		path := gridPlaceholderAuthoredPath(si, slide, ph.ID)
		switch ph.Type {
		case types.PlaceholderTitle:
			if f, ok := frameDeviation(frame, path, "title_baseline", ph.Bounds.Y+ph.Bounds.Height, frame.TitleBottomY, frame.SlideHeight, frame.TopTolerance); ok {
				out = append(out, f)
			}
		case types.PlaceholderBody, types.PlaceholderContent:
			if bodyChecked || types.IsDisclosurePlaceholder(ph) {
				continue
			}
			bodyChecked = true
			if f, ok := frameDeviation(frame, path, "content_top", ph.Bounds.Y, frame.ContentTopY, frame.SlideHeight, frame.TopTolerance); ok {
				out = append(out, f)
			}
			left := ph.Bounds.X
			if chrome.Content.CX > 0 && left < chrome.Content.X {
				left = chrome.Content.X
			}
			if f, ok := frameDeviation(frame, path, "left_margin", left, frame.LeftX, frame.SlideWidth, frame.LeftTolerance); ok {
				out = append(out, f)
			}
		}
	}
	return out
}

func frameGridFindings(frame contentFrame, si int, slide *SlideInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if slide.ShapeGrid == nil || !frame.CheckGrids {
		return nil
	}
	path := slidepath.ShapeGrid(si)
	// A pattern nested in a cell draws nothing until it is expanded: the
	// block's left edge was read from the first authored shape, half a slide
	// in when the pattern is the left cell (go-slide-creator-2ym44). Only the
	// block's corner is read here, so the fallback font is enough.
	grid := gridWithNestedPatternsExpanded(*slide, si, path, layouts, frame.SlideWidth, frame.SlideHeight, nil, nil, "", 0)
	if grid == nil {
		return nil
	}
	geom := resolveGridGeometry(*slide, layouts, frame.SlideWidth, frame.SlideHeight)
	res := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, frame.SlideWidth, frame.SlideHeight)
	if res == nil {
		return nil
	}
	block, ok := resolvedBlockRect(res)
	if !ok {
		return nil
	}
	var out []patterns.FitFinding
	// A sparse content-sized block is placed at the optical centre of the
	// content area on purpose (go-slide-creator-yhzxt); its top is not held
	// to the frame's content line. Its left edge still is.
	composed := res.Composed || boxBlockComposed(slide.ShapeGrid, geom.OverrideBounds, geom.Zone, frame.SlideWidth, frame.SlideHeight)
	if f, ok := frameDeviation(frame, path, "content_top", block.Y, frame.ContentTopY, frame.SlideHeight, frame.TopTolerance); ok && !composed {
		out = append(out, f)
	}
	if f, ok := frameDeviation(frame, path, "left_margin", block.X, frame.LeftX, frame.SlideWidth, frame.LeftTolerance); ok {
		out = append(out, f)
	}
	return out
}

// resolvedBlockRect is the top-left corner of a resolved grid's drawn cells.
func resolvedBlockRect(res *shapegrid.ResolveResult) (pptx.RectEmu, bool) {
	var top, left int64 = -1, -1
	for _, c := range res.Cells {
		if c.Bounds.CX <= 0 || c.Bounds.CY <= 0 {
			continue
		}
		if top < 0 || c.Bounds.Y < top {
			top = c.Bounds.Y
		}
		if left < 0 || c.Bounds.X < left {
			left = c.Bounds.X
		}
	}
	if top < 0 {
		return pptx.RectEmu{}, false
	}
	return pptx.RectEmu{X: left, Y: top}, true
}

func frameTakeawayFindings(frame contentFrame, si int, slide *SlideInput, layout *types.LayoutMetadata, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if strings.TrimSpace(slide.Takeaway) == "" {
		return nil
	}
	layoutID := slide.LayoutID
	if layout != nil {
		layoutID = layout.ID
	}
	chrome := slideChromeFrame(*slide, layoutID, layouts, frame.SlideWidth, frame.SlideHeight)
	if !chrome.Fits || chrome.Takeaway.CX <= 0 {
		return nil
	}
	if f, ok := frameDeviation(frame, slidepath.SlideField(si, "takeaway"), "left_margin", chrome.Takeaway.X, frame.LeftX, frame.SlideWidth, frame.LeftTolerance); ok {
		return []patterns.FitFinding{f}
	}
	return nil
}

// frameFieldLabels names each checked line in messages.
var frameFieldLabels = map[string][2]string{
	"title_baseline": {"title bottom", "title_baseline"},
	"content_top":    {"content top", "content_top"},
	"left_margin":    {"content left", "left_margin"},
}

// frameDeviation builds one grid_violation when current is more than
// tolerance away from target. ref is the slide dimension the percentages in
// the message are relative to.
func frameDeviation(frame contentFrame, path, field string, current, target, ref, tolerance int64) (patterns.FitFinding, bool) {
	deviation := absInt64(current - target)
	if target <= 0 || deviation <= tolerance {
		return patterns.FitFinding{}, false
	}
	labels := frameFieldLabels[field]
	frameName := "the deck grid's"
	if frame.Source == "template" {
		frameName = "the template frame's"
	}
	return patterns.FitFinding{
		ValidationError: patterns.ValidationError{
			Code: "grid_violation",
			Path: path,
			Message: fmt.Sprintf("%s (%.1f%%) deviates from %s %s (%.1f%%) by %.2f%% (%.1fpt)",
				labels[0], emuToPct(current, ref), frameName, labels[1], emuToPct(target, ref), emuToPct(deviation, ref), float64(deviation)/12700),
			Fix: &patterns.FixSuggestion{
				Kind: "reposition_shape",
				Params: map[string]any{
					"field":       field,
					"frame":       frame.Source,
					"current_emu": current,
					"target_emu":  target,
				},
			},
		},
		Action:   frame.Action,
		Measured: &patterns.Extent{HeightEMU: current},
		Allowed:  &patterns.Extent{HeightEMU: target},
	}, true
}
