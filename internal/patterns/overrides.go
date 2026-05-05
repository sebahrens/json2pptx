package patterns

import (
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
)

// TextOverrides contains pattern-level overrides common to patterns with
// header/body text: accent color, header font size, and body font size.
// Patterns with identical override shapes (card-grid, comparison-2col)
// alias this type directly. Patterns with extra fields (matrix-2x2) embed it.
type TextOverrides struct {
	Accent         string  `json:"accent,omitempty"`
	SemanticAccent string  `json:"semantic_accent,omitempty"`
	HeaderSize     float64 `json:"header_size,omitempty"`
	BodySize       float64 `json:"body_size,omitempty"`
}

// ValidSemanticAccents is the set of recognised semantic accent roles.
var ValidSemanticAccents = map[string]bool{
	"positive": true,
	"negative": true,
	"neutral":  true,
}

// AccentStrategy controls how the default accent color is chosen for patterns
// that don't set an explicit accent.
type AccentStrategy string

const (
	// AccentStrategyPrimary always uses accent1 (the legacy default).
	AccentStrategyPrimary AccentStrategy = "primary"
	// AccentStrategyRotate round-robins through accent1-accent6 by slide index.
	AccentStrategyRotate AccentStrategy = "rotate"
	// AccentStrategySectionKeyed assigns one accent per section (wraps at 6).
	AccentStrategySectionKeyed AccentStrategy = "section-keyed"
)

// NumAccentSlots is the number of OOXML accent color slots (accent1–accent6).
const NumAccentSlots = 6

// ValidAccentStrategies is the set of allowed AccentStrategy values.
var ValidAccentStrategies = []string{
	string(AccentStrategyPrimary),
	string(AccentStrategyRotate),
	string(AccentStrategySectionKeyed),
}

// IsValidAccentStrategy returns true if s is a recognised accent strategy.
func IsValidAccentStrategy(s string) bool {
	for _, v := range ValidAccentStrategies {
		if s == v {
			return true
		}
	}
	return false
}

// AccentForStrategy returns the default accent name (e.g. "accent2") for the
// given strategy, slide index, and section index. The caller should only use
// this when no explicit accent or semantic_accent was provided.
func AccentForStrategy(strategy AccentStrategy, slideIndex, sectionIndex int) string {
	switch strategy {
	case AccentStrategyRotate:
		return fmt.Sprintf("accent%d", (slideIndex%NumAccentSlots)+1)
	case AccentStrategySectionKeyed:
		return fmt.Sprintf("accent%d", (sectionIndex%NumAccentSlots)+1)
	default: // primary or empty
		return "accent1"
	}
}

// ResolveAccent returns the accent color for a pattern invocation.
// Priority: explicit accent > semantic_accent resolved via metadata > strategy-derived default > "accent1".
func ResolveAccent(accent, semanticAccent string, metadata *types.TemplateMetadata) string {
	if accent != "" {
		return accent
	}
	if semanticAccent != "" && metadata != nil && len(metadata.SemanticAccents) > 0 {
		if resolved, ok := metadata.SemanticAccents[semanticAccent]; ok {
			return resolved
		}
	}
	return "accent1"
}

// ResolveAccentWithStrategy is like ResolveAccent but uses the deck-level
// accent strategy to choose the default when neither explicit accent nor
// semantic_accent is specified.
func ResolveAccentWithStrategy(accent, semanticAccent string, metadata *types.TemplateMetadata, strategy AccentStrategy, slideIndex, sectionIndex int) string {
	if accent != "" {
		return accent
	}
	if semanticAccent != "" && metadata != nil && len(metadata.SemanticAccents) > 0 {
		if resolved, ok := metadata.SemanticAccents[semanticAccent]; ok {
			return resolved
		}
	}
	return AccentForStrategy(strategy, slideIndex, sectionIndex)
}

// ValidSurfaceTintRoles is the set of recognised surface tint roles.
var ValidSurfaceTintRoles = map[string]bool{
	"subtle":   true,
	"paper":    true,
	"elevated": true,
	"inverse":  true,
}

// ResolveSurface returns the scheme color name for a surface tint role.
// Falls back to defaultColor if the role is not defined in the template metadata.
func ResolveSurface(role string, metadata *types.TemplateMetadata, defaultColor string) string {
	if metadata != nil && len(metadata.SurfaceTints) > 0 {
		if resolved, ok := metadata.SurfaceTints[role]; ok {
			return resolved
		}
	}
	return defaultColor
}

// ResolveSize returns size if positive, otherwise defaultSize.
func ResolveSize(size, defaultSize float64) float64 {
	if size > 0 {
		return size
	}
	return defaultSize
}

// textOverridesSchema returns the JSON Schema for the standard
// {accent, semantic_accent, header_size, body_size} overrides object.
func textOverridesSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"accent":          StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"semantic_accent": EnumSchema("positive", "negative", "neutral").WithDescription("Semantic accent role resolved via template metadata; ignored when accent is set"),
			"header_size":     NumberSchema(6, 120).WithDescription("Font size for headers in points"),
			"body_size":       NumberSchema(6, 120).WithDescription("Font size for body text in points"),
		},
		nil,
	).WithAdditionalProperties(false)
}
