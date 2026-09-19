package main

import (
	"github.com/sebahrens/json2pptx/internal/layout"
	"github.com/sebahrens/json2pptx/internal/types"
)

// Predicting the layout a slide will land on (go-slide-creator-t64e).
//
// The measured title-fit check only ran when a slide carried an explicit
// layout_id, because layoutForSlideResolved bailed out on an empty one. So the
// same 105-character title produced the measured message
// ("only fits its title placeholder at 60% of the template 45pt size; shorten
// to <= 74 chars") with layout_id: "content", and the legacy character
// heuristic ("title too long (105 chars, max 60)") with slide_type: "content".
//
// That is not a corner: the semantic compilers emit SlideType and no LayoutID,
// so the DeckSpec path — the one get_started recommends — never measured a
// title at all. A 195-character title passed validate_deck_spec with zero
// findings.
//
// The generator does not need an explicit layout_id either: it runs the
// heuristic selector over slide_type and content. Running the SAME selection
// here, with the same running context, makes the checks see the layout the
// deck will actually use.

// predictSlideLayouts returns, per slide, the layout the generator will place
// it on: the explicit layout_id when set, otherwise the heuristic selector's
// choice. Entries are nil when no layout can be resolved (no template layouts,
// or a slide the selector rejects — generation reports that separately).
//
// The running context mirrors generation: slides are resolved in order and each
// choice feeds the variety bonus for the next, so the prediction matches what
// the deck will get rather than what each slide would get in isolation.
func predictSlideLayouts(input *PresentationInput, layouts []types.LayoutMetadata) []*types.LayoutMetadata {
	if input == nil {
		return nil
	}
	out := make([]*types.LayoutMetadata, len(input.Slides))
	if len(layouts) == 0 {
		return out
	}

	usedLayouts := map[string]int{}
	previousType := ""
	for i := range input.Slides {
		slide := &input.Slides[i]

		if l := layoutForSlideResolved(slide, layouts); l != nil {
			out[i] = l
			usedLayouts[l.ID]++
			previousType = l.ID
			continue
		}

		req := layout.SelectionRequest{
			Slide:   jsonSlideToDefinition(*slide),
			Layouts: layouts,
			Context: layout.SelectionContext{
				Position:     i,
				TotalSlides:  len(input.Slides),
				UsedLayouts:  usedLayouts,
				PreviousType: previousType,
			},
		}
		result, err := layout.SelectLayout(req)
		if err != nil {
			// The selector rejected the slide; generation reports that with its
			// own error. Leave the prediction empty rather than guessing.
			continue
		}
		if l := findLayoutByID(layouts, result.LayoutID); l != nil {
			out[i] = l
			usedLayouts[result.LayoutID]++
			previousType = result.LayoutID
		}
	}
	return out
}
