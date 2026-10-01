package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
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
		// The band carries the uniform shape text margin, clamped on a band
		// too short for it (pptx.EffectiveTextInsets). textcapacity and
		// textfit assume the OOXML 7.2pt / 3.6pt default sides, so hand them
		// the written text rectangle grown by those defaults.
		fontPt := float64(tokens.TypeScaleSubheadHPt) / 100
		in := pptx.EffectiveTextInsets(&pptx.TextBody{
			Insets:     pptx.ShapeTextInsets(),
			Paragraphs: []pptx.Paragraph{{Runs: []pptx.Run{{Text: slide.Takeaway, FontSize: tokens.TypeScaleSubheadHPt}}}},
		}, pptx.RectEmu{CX: band.CX, CY: band.CY})
		width := band.CX - in[0] - in[2] + 2*91440
		height := band.CY - in[1] - in[3] + 2*45720
		budget := textcapacity.ForPlaceholder(types.PlaceholderInfo{
			Bounds:   types.BoundingBox{Width: width, Height: height},
			FontSize: tokens.TypeScaleSubheadHPt,
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
