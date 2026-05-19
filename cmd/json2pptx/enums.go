package main

import (
	"sort"

	"github.com/sebahrens/json2pptx/internal/generator"
)

// ---------------------------------------------------------------------------
// Canonical published enum vocabularies for PresentationInput fields.
//
// These lists are the SINGLE source of truth that drives:
//   - the validator in enum_validation.go
//   - the JSON Schema enumMap in mcp_input_schema.go
//   - the published vocabularies in mcp_capabilities.go (get_capabilities)
//   - the docs in SLIDE_FORMAT.md
//
// Aliases (e.g. "med" -> "medium") are explicitly listed in *Aliases maps.
// The validator accepts both canonical and alias values for backward
// compatibility, but only canonical values appear in the schema, capabilities
// vocabularies, and published examples. Anything not in the canonical or
// alias list is rejected as UNKNOWN_ENUM.
// ---------------------------------------------------------------------------

// canonicalTransitions returns the published transition vocabulary. The
// transition names themselves are sourced from generator.ValidTransitionNames()
// (the runtime authority for which OOXML transitions the engine emits); the
// "none" sentinel is appended because users may type it to explicitly disable
// a transition.
func canonicalTransitions() []string {
	base := generator.ValidTransitionNames()
	out := make([]string, 0, len(base)+1)
	out = append(out, base...)
	out = append(out, "none")
	sort.Strings(out)
	return out
}

// transitionAliases maps accepted aliases to their canonical transition.
// Currently empty; kept for symmetry with transitionSpeedAliases and to make
// future deprecations trivial to wire up.
var transitionAliases = map[string]string{}

// canonicalTransitionSpeeds is the published vocabulary for transition_speed.
// The internal OOXML attribute value is "med", but the documented user-facing
// canonical form is "medium"; "med" remains accepted as an alias.
var canonicalTransitionSpeeds = []string{"slow", "medium", "fast"}

// transitionSpeedAliases lists deprecated/short forms accepted by the
// validator. Keys map to the canonical form they alias to.
var transitionSpeedAliases = map[string]string{
	"med": "medium",
}

// canonicalBuilds is the published vocabulary for the slide-level "build"
// (entrance animation) field. Currently a single value; extend as new build
// types are added to the generator.
var canonicalBuilds = []string{"bullets"}

var buildAliases = map[string]string{}

// canonicalBackgroundFits is the published vocabulary for background.fit.
var canonicalBackgroundFits = []string{"cover", "stretch", "tile"}

var backgroundFitAliases = map[string]string{}

// canonicalDesignModes is the published vocabulary for the deck-level
// design_mode field.
var canonicalDesignModes = []string{"constrained", "free"}

var designModeAliases = map[string]string{}

// canonicalAccentStrategies is the published vocabulary for the deck-level
// accent_strategy field.
var canonicalAccentStrategies = []string{"primary", "rotate", "section-keyed"}

var accentStrategyAliases = map[string]string{}

// canonicalSlideTypes is the published vocabulary for the slide-level
// slide_type hint used by layout auto-selection.
//
// NOTE: "closing" is intentionally NOT in this list. "closing" is a
// template-specific layout_id (a layout name in a particular template), not a
// slide_type hint understood by the layout selector. Authors who want a
// closing slide should set layout_id: "closing" directly.
var canonicalSlideTypes = []string{
	"title", "content", "section", "two-column", "blank",
	"chart", "diagram", "image", "comparison",
}

var slideTypeAliases = map[string]string{}
