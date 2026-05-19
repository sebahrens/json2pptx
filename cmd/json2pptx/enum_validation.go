package main

import (
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/slidepath"
)

// ---------------------------------------------------------------------------
// Enum validation for slide-level fields.
//
// These constraints mirror what the generator silently ignores: unknown
// transition types become no-ops, unknown speeds default to "med", unknown
// build values are ignored, and unknown background.fit values default to
// "cover". Rather than swallowing bad input, we surface it as a
// ValidationError with code "UNKNOWN_ENUM" and a fix suggestion listing the
// allowed values.
//
// The canonical published vocabularies live in enums.go; this file consumes
// them so the validator, schema, capabilities, and docs cannot drift.
// Aliases (e.g. "med" -> "medium") are accepted by the validator but never
// included in the error message's allowed list — agents are always nudged
// toward the canonical form.
// ---------------------------------------------------------------------------

// checkInputEnumValues validates enum-constrained fields across all slides
// in a parsed PresentationInput. Returns ValidationError warnings for any
// field with a value not in its allowed set.
func checkInputEnumValues(input *PresentationInput) []*patterns.ValidationError {
	var errs []*patterns.ValidationError

	// Top-level design_mode enum
	if input.DesignMode != "" {
		if err := checkEnum("design_mode", input.DesignMode, canonicalDesignModes, designModeAliases); err != nil {
			errs = append(errs, err)
		}
	}

	// Top-level accent_strategy enum
	if input.AccentStrategy != "" {
		if err := checkEnum("accent_strategy", input.AccentStrategy, canonicalAccentStrategies, accentStrategyAliases); err != nil {
			errs = append(errs, err)
		}
	}

	allowedTransitions := canonicalTransitions()

	for i, slide := range input.Slides {
		prefix := slidepath.Slide(i)

		if slide.SlideType != "" {
			if err := checkEnum(prefix+"/slide_type", slide.SlideType, canonicalSlideTypes, slideTypeAliases); err != nil {
				errs = append(errs, err)
			}
		}
		if slide.Transition != "" {
			if err := checkEnum(prefix+"/transition", slide.Transition, allowedTransitions, transitionAliases); err != nil {
				errs = append(errs, err)
			}
		}
		if slide.TransitionSpeed != "" {
			if err := checkEnum(prefix+"/transition_speed", slide.TransitionSpeed, canonicalTransitionSpeeds, transitionSpeedAliases); err != nil {
				errs = append(errs, err)
			}
		}
		if slide.Build != "" {
			if err := checkEnum(prefix+"/build", slide.Build, canonicalBuilds, buildAliases); err != nil {
				errs = append(errs, err)
			}
		}
		if slide.Background != nil && slide.Background.Fit != "" {
			if err := checkEnum(prefix+"/background/fit", slide.Background.Fit, canonicalBackgroundFits, backgroundFitAliases); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errs
}

// checkEnum validates a single value against a canonical set plus an optional
// alias map. Returns nil if the value matches either a canonical value or an
// alias key (case-insensitive). On failure, the error message and the
// fix.allowed payload list only canonical values — aliases are accepted but
// never advertised so agents converge on the canonical form.
func checkEnum(path, value string, canonical []string, aliases map[string]string) *patterns.ValidationError {
	lower := strings.ToLower(value)
	for _, a := range canonical {
		if lower == a {
			return nil
		}
	}
	if _, ok := aliases[lower]; ok {
		return nil
	}
	return &patterns.ValidationError{
		Pattern: "input",
		Path:    path,
		Code:    patterns.ErrCodeUnknownEnum,
		Message: fmt.Sprintf("unknown value %q for %s (allowed: %s)", value, path, strings.Join(canonical, ", ")),
		Fix: &patterns.FixSuggestion{
			Kind: "use_one_of",
			Params: map[string]any{
				"allowed": canonical,
			},
		},
	}
}
