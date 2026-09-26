package main

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/deckinput"
)

func applyNativeContentSplit(input *PresentationInput, slideIdx int, fix repairFixInput) appliedFix {
	if fix.Kind == "split_bullets" {
		return applySplitBullets(input, slideIdx, fix.Params)
	}
	return applySplitAtRow(input, slideIdx, fix.Params)
}

func applySplitBullets(input *PresentationInput, slideIdx int, params map[string]any) appliedFix {
	if stringParam(params, "path", "") != "" {
		return appliedFix{Kind: "split_bullets", Code: "invalid_parameter", Message: "split_bullets coordinates all plain bullet columns; path targeting is unsupported"}
	}
	budget := 0
	switch value := params["max_items"].(type) {
	case int:
		budget = value
	case float64:
		budget = int(value)
		if float64(budget) != value {
			return appliedFix{Kind: "split_bullets", Code: "invalid_parameter", Message: "max_items must be a positive integer"}
		}
	default:
		return appliedFix{Kind: "split_bullets", Code: "invalid_parameter", Message: "max_items must be a positive integer"}
	}
	pages, err := deckinput.SplitBulletSlide(input.Slides[slideIdx], budget)
	if err != nil {
		return appliedFix{Kind: "split_bullets", Code: "invalid_parameter", Message: err.Error()}
	}
	if len(pages) == 1 {
		return appliedFix{Kind: "split_bullets", Message: "bullet content already fits the requested item budget; render to verify actual fit"}
	}
	// Numeric destinations refer to output positions. Expanding this slide
	// would shift them; refuse instead of silently redirecting navigation.
	if hasLinks, err := bulletSplitHasInternalLinks(input); err != nil || hasLinks {
		return appliedFix{Kind: "split_bullets", Code: "navigation_requires_authoring", Message: "deck contains internal slide links or cannot be inspected; author sibling slides and update numeric destinations explicitly"}
	}
	slides := make([]SlideInput, 0, len(input.Slides)-1+len(pages))
	slides = append(slides, input.Slides[:slideIdx]...)
	slides = append(slides, pages...)
	slides = append(slides, input.Slides[slideIdx+1:]...)
	input.Slides = slides
	return appliedFix{Kind: "split_bullets", Applied: true, Message: fmt.Sprintf("preserved bullet columns across %d slides; render all pages to verify fit", len(pages))}
}

func bulletSplitHasInternalLinks(input *PresentationInput) (bool, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return false, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return false, err
	}
	var walk func(any) bool
	walk = func(value any) bool {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if key == "link" || key == "source_link" {
					if link, ok := child.(map[string]any); ok {
						if number, ok := link["slide"].(float64); ok && number != 0 {
							return true
						}
					}
				}
				if walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range value {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(value), nil
}
