package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/textcapacity"
	"github.com/sebahrens/json2pptx/internal/textfit"
	"github.com/sebahrens/json2pptx/internal/tokens"
	"github.com/sebahrens/json2pptx/internal/types"
)

// collectTakeawayFitFindings measures the late-injected 14pt takeaway against
// the same chrome frame generation uses. No other text-fit pass sees this shape.
func collectTakeawayFitFindings(input *PresentationInput, layouts []types.LayoutMetadata, slideWidth, slideHeight int64) []patterns.FitFinding {
	if input == nil {
		return nil
	}
	var findings []patterns.FitFinding
	for i, slide := range input.Slides {
		if slide.Takeaway == "" {
			continue
		}
		band := slideChromeFrame(slide, "", layouts, slideWidth, slideHeight).Takeaway
		// The shape has a 7.2pt left inset and no right inset. Both
		// textcapacity and textfit assume 7.2pt on each side, so add 7.2pt
		// to the supplied width to reproduce the actual text rectangle.
		width := band.CX + 91440
		fontPt := float64(tokens.CardTitleMaxHPt) / 100
		budget := textcapacity.ForPlaceholder(types.PlaceholderInfo{
			Bounds:   types.BoundingBox{Width: width, Height: band.CY},
			FontSize: tokens.CardTitleMaxHPt,
		}, slide.Takeaway)
		measured, err := textfit.MeasureRun(slide.Takeaway, "Liberation Sans", fontPt, width, budget.MaxLines)
		if err != nil || measured.Lines <= budget.MaxLines {
			continue
		}
		path := slidepath.SlideField(i, "takeaway")
		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: path,
				Code: patterns.ErrCodeBodyTooLong,
				Message: fmt.Sprintf("slide %d: takeaway needs %d lines at 14pt but its template chrome band holds %d; shorten it to one line (about %d average-width characters)",
					i+1, measured.Lines, budget.MaxLines, budget.MaxChars),
				Fix: &patterns.FixSuggestion{Kind: "rewrite_field", Params: map[string]any{
					"path": path, "max_lines": budget.MaxLines, "max_chars": budget.MaxChars,
				}},
			},
			Action:   "refuse",
			Measured: &patterns.Extent{HeightEMU: measured.RequiredEMU},
			Allowed:  &patterns.Extent{HeightEMU: band.CY},
		})
	}
	return findings
}
