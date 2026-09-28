package main

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// collectControlCharFindings reports the string values from which
// PresentationInput.UnmarshalJSON removed invisible bidi override / isolate
// controls or byte-order marks (go-slide-creator-7oz3c). The characters are
// already gone from the input, so the finding is informational: it tells the
// author the rendered text differs from the source.
func collectControlCharFindings(input *PresentationInput) []patterns.FitFinding {
	if input == nil || len(input.ControlCharSites) == 0 {
		return nil
	}
	out := make([]patterns.FitFinding, 0, len(input.ControlCharSites))
	for _, site := range input.ControlCharSites {
		out = append(out, patterns.FitFinding{
			ValidationError: patterns.ValidationError{
				Path: site.Path,
				Code: patterns.ErrCodeInputControlCharsRemoved,
				Message: fmt.Sprintf("removed %d invisible bidi control / byte-order-mark character(s) (U+202A-U+202E, U+2066-U+2069, U+FEFF); the rendered text differs from the source",
					site.Removed),
			},
			Action: "info",
		})
	}
	return out
}
