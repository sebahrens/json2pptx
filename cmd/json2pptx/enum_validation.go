package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

// ---------------------------------------------------------------------------
// Enum validation for slide-level fields.
//
// These constraints mirror what the generator silently ignores: unknown
// transition types become no-ops, unknown speeds default to "med", unknown
// build values are ignored, and unknown background.fit values default to
// "cover". Rather than swallowing bad input, we surface it as a
// ValidationError with code "unknown_enum" and a fix suggestion listing the
// allowed values.
// ---------------------------------------------------------------------------

// Allowed enum values for each field.
var (
	allowedTransitions = []string{
		"fade", "push", "wipe", "cover", "uncover", "cut", "dissolve", "none",
	}
	allowedTransitionSpeeds = []string{
		"slow", "med", "medium", "fast",
	}
	allowedBuilds = []string{
		"bullets",
	}
	allowedBackgroundFits = []string{
		"cover", "stretch", "tile",
	}
)

// checkInputEnumValues validates enum-constrained fields across all slides
// in a parsed PresentationInput. Returns ValidationError warnings for any
// field with a value not in its allowed set.
func checkInputEnumValues(input *PresentationInput) []*patterns.ValidationError {
	var errs []*patterns.ValidationError
	for i, slide := range input.Slides {
		prefix := fmt.Sprintf("slides[%d]", i)

		if slide.Transition != "" {
			if err := checkEnum(prefix+".transition", slide.Transition, allowedTransitions); err != nil {
				errs = append(errs, err)
			}
		}
		if slide.TransitionSpeed != "" {
			if err := checkEnum(prefix+".transition_speed", slide.TransitionSpeed, allowedTransitionSpeeds); err != nil {
				errs = append(errs, err)
			}
		}
		if slide.Build != "" {
			if err := checkEnum(prefix+".build", slide.Build, allowedBuilds); err != nil {
				errs = append(errs, err)
			}
		}
		if slide.Background != nil && slide.Background.Fit != "" {
			if err := checkEnum(prefix+".background.fit", slide.Background.Fit, allowedBackgroundFits); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// checkEnum validates a single value against an allowed set. Returns nil if
// valid, or a ValidationError with code "unknown_enum" and a fix suggestion.
func checkEnum(path, value string, allowed []string) *patterns.ValidationError {
	lower := strings.ToLower(value)
	for _, a := range allowed {
		if lower == a {
			return nil
		}
	}
	return &patterns.ValidationError{
		Pattern: "input",
		Path:    path,
		Code:    patterns.ErrCodeUnknownEnum,
		Message: fmt.Sprintf("unknown value %q for %s (allowed: %s)", value, path, strings.Join(allowed, ", ")),
		Fix: &patterns.FixSuggestion{
			Kind: "use_one_of",
			Params: map[string]any{
				"allowed": allowed,
			},
		},
	}
}
