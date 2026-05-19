package main

import (
	"reflect"
	"sort"
	"testing"
)

// TestCanonicalEnums_ValidatorSchemaCapabilitiesParity locks the centralization:
// the schema enumMap, the get_capabilities vocabularies, and the validator's
// accepted set must all derive from the same canonical lists in enums.go.
func TestCanonicalEnums_ValidatorSchemaCapabilitiesParity(t *testing.T) {
	t.Run("design_mode parity", func(t *testing.T) {
		assertEnumParity(t, "PresentationInput", "design_mode", canonicalDesignModes)
	})
	t.Run("accent_strategy parity", func(t *testing.T) {
		assertEnumParity(t, "PresentationInput", "accent_strategy", canonicalAccentStrategies)
	})
	t.Run("slide_type parity", func(t *testing.T) {
		assertEnumParity(t, "SlideInput", "slide_type", canonicalSlideTypes)
	})
	t.Run("transition parity", func(t *testing.T) {
		assertEnumParity(t, "SlideInput", "transition", canonicalTransitions())
	})
	t.Run("transition_speed parity", func(t *testing.T) {
		assertEnumParity(t, "SlideInput", "transition_speed", canonicalTransitionSpeeds)
	})
	t.Run("build parity", func(t *testing.T) {
		assertEnumParity(t, "SlideInput", "build", canonicalBuilds)
	})
	t.Run("background.fit parity", func(t *testing.T) {
		assertEnumParity(t, "BackgroundInput", "fit", canonicalBackgroundFits)
	})
}

// TestCanonicalEnums_NoAliasesInPublishedLists ensures aliases never leak into
// the canonical published vocabularies. The validator may accept them, but
// the schema, capabilities, and docs only show canonical forms.
func TestCanonicalEnums_NoAliasesInPublishedLists(t *testing.T) {
	for alias := range transitionSpeedAliases {
		for _, c := range canonicalTransitionSpeeds {
			if c == alias {
				t.Errorf("transition_speed alias %q must not appear in canonical list %v", alias, canonicalTransitionSpeeds)
			}
		}
	}
}

// TestCanonicalEnums_AliasesAcceptedByValidator confirms aliases (e.g. "med")
// remain accepted for backward compatibility even though they are no longer
// published.
func TestCanonicalEnums_AliasesAcceptedByValidator(t *testing.T) {
	input := &PresentationInput{
		Slides: []SlideInput{{TransitionSpeed: "med"}},
	}
	errs := checkInputEnumValues(input)
	if len(errs) != 0 {
		t.Errorf("expected \"med\" to be accepted as alias for transition_speed, got %d errors: %v", len(errs), errs)
	}
}

// TestCanonicalEnums_RejectedValues confirms values dropped from the canonical
// vocabularies (and not listed as aliases) are now rejected. These were
// previously advertised in only one surface, causing cross-surface drift.
func TestCanonicalEnums_RejectedValues(t *testing.T) {
	cases := []struct {
		name  string
		input *PresentationInput
	}{
		// "closing" was in the schema enum but never in the validator's
		// slide_type list (it's a layout_id, not a slide_type hint).
		{"slide_type=closing", &PresentationInput{Slides: []SlideInput{{SlideType: "closing"}}}},
		// "split" and "reveal" were in the schema enum but the generator
		// emits no OOXML for them — they were silently no-ops.
		{"transition=split", &PresentationInput{Slides: []SlideInput{{Transition: "split"}}}},
		{"transition=reveal", &PresentationInput{Slides: []SlideInput{{Transition: "reveal"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := checkInputEnumValues(tc.input)
			if len(errs) == 0 {
				t.Errorf("expected validation error for %s", tc.name)
			}
		})
	}
}

// assertEnumParity verifies that the schema's enumMap entry for (typeName,
// jsonName) equals the canonical list (set equality — order-independent).
func assertEnumParity(t *testing.T, typeName, jsonName string, canonical []string) {
	t.Helper()
	got, ok := enumMap[typeName][jsonName]
	if !ok {
		t.Fatalf("enumMap[%s][%s] missing", typeName, jsonName)
	}
	if !sortedEqual(got, canonical) {
		t.Errorf("enumMap[%s][%s] = %v; canonical = %v", typeName, jsonName, got, canonical)
	}

	// Capabilities vocabularies for the four surfaced fields.
	vocab := buildVocabularies()
	switch jsonName {
	case "transition":
		if !sortedEqual(vocab.SlideTransitions, canonical) {
			t.Errorf("vocabularies.slide_transitions = %v; canonical = %v", vocab.SlideTransitions, canonical)
		}
	case "transition_speed":
		if !sortedEqual(vocab.TransitionSpeeds, canonical) {
			t.Errorf("vocabularies.transition_speeds = %v; canonical = %v", vocab.TransitionSpeeds, canonical)
		}
	case "build":
		if !sortedEqual(vocab.BuildAnimations, canonical) {
			t.Errorf("vocabularies.build_animations = %v; canonical = %v", vocab.BuildAnimations, canonical)
		}
	}
}

func sortedEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string{}, a...)
	bb := append([]string{}, b...)
	sort.Strings(aa)
	sort.Strings(bb)
	return reflect.DeepEqual(aa, bb)
}
