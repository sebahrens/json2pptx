package patterns

import "github.com/sebahrens/json2pptx/internal/types"

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

// ResolveAccent returns the accent color for a pattern invocation.
// Priority: explicit accent > semantic_accent resolved via metadata > "accent1".
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
