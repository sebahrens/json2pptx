package patterns

// CellOverrideDefSchema returns the canonical JSON Schema for a cell override
// object. All 8 patterns share this definition under $defs.cellOverride.
func CellOverrideDefSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"accent_bar":     BooleanSchema().WithDescription("Show accent bar decoration"),
			"emphasis":       EnumSchema("bold", "italic", "bold-italic").WithDescription("Text emphasis"),
			"align":          EnumSchema("l", "ctr", "r").WithDescription("Horizontal alignment"),
			"vertical_align": EnumSchema("t", "ctr", "b").WithDescription("Vertical alignment"),
			"font_size":      NumberSchema(6, 120).WithDescription("Font size in points"),
			"color":          StringSchema(0).WithDescription("Text color (scheme ref, e.g. \"dk1\")"),
		},
		nil,
	).WithAdditionalProperties(false)
}
