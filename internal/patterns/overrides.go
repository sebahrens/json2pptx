package patterns

// TextOverrides contains pattern-level overrides common to patterns with
// header/body text: accent color, header font size, and body font size.
// Patterns with identical override shapes (card-grid, comparison-2col)
// alias this type directly. Patterns with extra fields (matrix-2x2) embed it.
type TextOverrides struct {
	Accent     string  `json:"accent,omitempty"`
	HeaderSize float64 `json:"header_size,omitempty"`
	BodySize   float64 `json:"body_size,omitempty"`
}

// ResolveAccent returns the accent color, defaulting to accent1.
func ResolveAccent(accent string) string {
	if accent != "" {
		return accent
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
// {accent, header_size, body_size} overrides object.
func textOverridesSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"accent":      StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
			"header_size": NumberSchema(6, 120).WithDescription("Font size for headers in points"),
			"body_size":   NumberSchema(6, 120).WithDescription("Font size for body text in points"),
		},
		nil,
	).WithAdditionalProperties(false)
}
