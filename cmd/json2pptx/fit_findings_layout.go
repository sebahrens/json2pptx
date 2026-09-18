package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
	"github.com/sebahrens/json2pptx/internal/types"
)

// collectLayoutResolutionFindings runs the SAME layout auto-selection generation
// runs and reports every slide the template cannot host.
//
// Neither validate_input nor preview ran layout resolution, so a template
// missing a role (e.g. one carrying only Title Slide and Blank) returned
// valid:true with no findings while generate then failed — naming only the first
// affected slide of seven, and naming the INTERNAL coerced slide type rather than
// what the author wrote (go-slide-creator-9svyz).
func collectLayoutResolutionFindings(input *PresentationInput, layouts []types.LayoutMetadata) []patterns.FitFinding {
	if len(layouts) == 0 || input == nil {
		return nil
	}

	candidates := make([]string, 0, len(layouts))
	for _, l := range layouts {
		candidates = append(candidates, l.ID)
	}

	var findings []patterns.FitFinding
	usedLayouts := map[string]int{}
	for i := range input.Slides {
		slide := input.Slides[i]
		if slide.LayoutID != "" {
			continue // an explicit layout is the author's call
		}

		req := layout.SelectionRequest{
			Slide:   jsonSlideToDefinition(slide),
			Layouts: layouts,
			Context: layout.SelectionContext{
				Position:    i,
				TotalSlides: len(input.Slides),
				UsedLayouts: usedLayouts,
			},
		}
		result, err := layout.SelectLayout(req)
		if err == nil {
			usedLayouts[result.LayoutID]++
			continue
		}

		hasComposition := slide.Pattern != nil || slide.Compose != nil || slide.ShapeGrid != nil
		fallbackID, hasFallback := compositionFallbackLayoutID(layouts)

		message := fmt.Sprintf(
			"slide %d: no layout in this template can host slide_type %q — set layout_id explicitly or use a template that declares the role",
			i+1, authoredSlideType(slide))
		if hasComposition && hasFallback {
			message = fmt.Sprintf(
				"slide %d: no layout matches slide_type %q, so its %s content will be placed on the %q canvas layout — set layout_id explicitly to choose differently",
				i+1, authoredSlideType(slide), compositionKind(slide), fallbackID)
		}

		params := map[string]any{
			"slide_type": authoredSlideType(slide),
			"candidates": candidates,
		}
		if hasComposition && hasFallback {
			params["fallback_layout_id"] = fallbackID
		}

		findings = append(findings, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path:    slidepath.Slide(i),
				Code:    patterns.ErrCodeLayoutUnresolvable,
				Message: message,
				Fix: &patterns.FixSuggestion{
					Kind:   "swap_layout",
					Params: params,
				},
			},
			Action: "review",
		})
	}
	return findings
}
