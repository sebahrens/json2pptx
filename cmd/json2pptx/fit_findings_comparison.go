package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// collectComparisonSlideFindings steers slide_type "comparison" slides to
// the designed comparison patterns. The slide type renders as Two Content
// with bold column headers; before go-slide-creator-gndpw it was
// indistinguishable from two-column and no doc said so.
func collectComparisonSlideFindings(input *PresentationInput) []patterns.FitFinding {
	var out []patterns.FitFinding
	for i, slide := range input.Slides {
		if types.SlideType(slide.SlideType) != types.SlideTypeComparison || slide.Pattern != nil || slide.ShapeGrid != nil {
			continue
		}
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    slidepath.SlideField(i, "slide_type"),
				Code:    patterns.ErrCodeComparisonPreferPattern,
				Message: fmt.Sprintf("slide %d: slide_type \"comparison\" renders two plain bullet columns under bold option headers; the comparison-2col (or before-after) pattern draws a designed comparison", i+1),
				Fix: &patterns.FixSuggestion{Kind: "swap_pattern", Params: map[string]any{
					"from":         "slide_type:comparison",
					"to":           "comparison-2col",
					"alternatives": []string{"before-after"},
					"layout_id":    "blank-title",
				}},
			},
			Action: "info",
		})
	}
	return out
}
