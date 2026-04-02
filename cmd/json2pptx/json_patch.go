package main

import "fmt"

// PresentationPatchInput represents a patch operation on a PresentationInput.
// It allows modifying individual slides without regenerating the entire deck.
type PresentationPatchInput struct {
	// Base is the original presentation input (required)
	Base PresentationInput `json:"base"`

	// Operations to apply in order
	Operations []PresentationPatchOperation `json:"operations"`
}

// PresentationPatchOperation represents a single patch operation on a slide.
type PresentationPatchOperation struct {
	// Op is the operation type: "replace", "add", "remove"
	Op string `json:"op"`

	// SlideIndex is the 0-based slide index to operate on
	SlideIndex int `json:"slide_index"`

	// Slide is the new or replacement slide (for "add" and "replace")
	Slide *SlideInput `json:"slide,omitempty"`
}

// applyPresentationPatch applies a sequence of patch operations to a base PresentationInput
// and returns the resulting PresentationInput. Operations are applied in order.
//
// Supported operations:
//   - "replace": replace slide at index with new slide
//   - "add": insert slide at index (shifts subsequent slides right)
//   - "remove": remove slide at index (shifts subsequent slides left)
func applyPresentationPatch(patch PresentationPatchInput) (*PresentationInput, error) {
	// Start with a copy of the base
	result := PresentationInput{
		Template:       patch.Base.Template,
		OutputFilename: patch.Base.OutputFilename,
		Footer:         patch.Base.Footer,
		ThemeOverride:  patch.Base.ThemeOverride,
	}
	// Deep copy the slides slice so mutations don't affect the original
	result.Slides = make([]SlideInput, len(patch.Base.Slides))
	copy(result.Slides, patch.Base.Slides)

	for i, op := range patch.Operations {
		n := len(result.Slides)

		switch op.Op {
		case "replace":
			if op.SlideIndex < 0 || op.SlideIndex >= n {
				return nil, fmt.Errorf("operation %d: replace index %d out of range (0..%d)", i, op.SlideIndex, n-1)
			}
			if op.Slide == nil {
				return nil, fmt.Errorf("operation %d: replace requires a slide", i)
			}
			result.Slides[op.SlideIndex] = *op.Slide

		case "add":
			if op.SlideIndex < 0 || op.SlideIndex > n {
				return nil, fmt.Errorf("operation %d: add index %d out of range (0..%d)", i, op.SlideIndex, n)
			}
			if op.Slide == nil {
				return nil, fmt.Errorf("operation %d: add requires a slide", i)
			}
			// Insert at index
			result.Slides = append(result.Slides, SlideInput{})
			copy(result.Slides[op.SlideIndex+1:], result.Slides[op.SlideIndex:])
			result.Slides[op.SlideIndex] = *op.Slide

		case "remove":
			if op.SlideIndex < 0 || op.SlideIndex >= n {
				return nil, fmt.Errorf("operation %d: remove index %d out of range (0..%d)", i, op.SlideIndex, n-1)
			}
			result.Slides = append(result.Slides[:op.SlideIndex], result.Slides[op.SlideIndex+1:]...)

		default:
			return nil, fmt.Errorf("operation %d: unknown op %q (must be replace, add, or remove)", i, op.Op)
		}
	}

	return &result, nil
}

// Legacy types kept for backward compatibility with existing tests and callers.

// JSONPatchInput represents a patch operation on an existing JSON input.
// Deprecated: Use PresentationPatchInput for typed field support.
type JSONPatchInput struct {
	Base       JSONInput        `json:"base"`
	Operations []PatchOperation `json:"operations"`
}

// PatchOperation represents a single patch operation on a slide.
// Deprecated: Use PresentationPatchOperation for typed field support.
type PatchOperation struct {
	Op         string     `json:"op"`
	SlideIndex int        `json:"slide_index"`
	Slide      *JSONSlide `json:"slide,omitempty"`
}

