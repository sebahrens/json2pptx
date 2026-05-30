package types

import "regexp"

// TableSpec represents a parsed markdown table.
type TableSpec struct {
	Headers          []string     // Column headers (expanded: merged cells have empty strings)
	HeaderCells      []TableCell  // Header cells with colspan/rowspan info (nil if no merges in header)
	Rows             [][]TableCell // Data rows (each row is a slice of cells)
	Style            TableStyle   // Table styling options
	Merges           []CellMerge  // List of merge regions
	ColumnAlignments []string     // Per-column alignment: "left", "center", "right" (from separator row)
}

// TableCell represents a single cell in a table.
type TableCell struct {
	Content     string           // Text content of the cell
	ColSpan     int              // Number of columns this cell spans (default 1)
	RowSpan     int              // Number of rows this cell spans (default 1)
	IsMerged    bool             // True if this cell is part of a merge (not the origin)
	Conditional *ConditionalFormat // Optional conditional formatting rule
}

// CellMerge represents a merge region in the table.
type CellMerge struct {
	StartRow int // Starting row index (0-based, relative to data rows, not header)
	StartCol int // Starting column index (0-based)
	EndRow   int // Ending row index (inclusive)
	EndCol   int // Ending column index (inclusive)
}

// TableStyle defines table appearance options.
type TableStyle struct {
	HeaderBackground string // "accent1"-"accent6", "none", or hex color
	Borders          string // "all", "horizontal", "outer", "none"
	Striped          *bool  // Alternating row colors (nil = default on, explicit false = off)
	StyleID          string // OOXML table style GUID (e.g., "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
	UseTableStyle    bool   // When true, suppress all explicit formatting and let the table style control appearance
	HighlightColumn  int    // 1-indexed column to highlight with accent3 tint fill (0 = none)
	TotalsRow        bool   // When true, last data row rendered bold with top border
	ColumnTypes      []string // Per-column type: "text", "number", "currency", "percent", "delta"
}

// ConditionalFormat defines a rule-based cell fill for conditional formatting.
type ConditionalFormat struct {
	Rule      string  // "positive", "negative", "threshold"
	Threshold float64 // Used when Rule is "threshold"
	Fill      string  // Scheme color to apply (e.g., "accent2")
}

// DefaultTableStyleID is the OOXML GUID for "Medium Style 2 - Accent 1",
// a professional themed table style that adapts to the presentation color scheme.
const DefaultTableStyleID = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"

// tableStyleIDPattern matches an OOXML ST_Guid table style identifier: braces
// around five hyphen-separated hexadecimal groups (8-4-4-4-12). This is the
// only shape PowerPoint and stricter readers accept for the value of
// <a:tableStyleId> and the styleId attribute in ppt/tableStyles.xml.
var tableStyleIDPattern = regexp.MustCompile(`^\{[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}$`)

// IsValidTableStyleID reports whether id is a GUID-shaped OOXML table style
// identifier. It gates user-authored style_id values before they reach the
// OOXML sinks: any other value — a typo, or an XML attribute/element injection
// attempt such as `bad"&<` — is rejected so raw text can never be emitted into
// <a:tableStyleId> text or a styleId="..." attribute.
func IsValidTableStyleID(id string) bool {
	return tableStyleIDPattern.MatchString(id)
}

// DefaultTableStyle provides sensible defaults for table styling.
// HeaderBackground is intentionally empty so the table style's firstRow
// appearance takes effect; set it explicitly to override.
var DefaultTableStyle = TableStyle{
	HeaderBackground: "",
	Borders:          "all",
	Striped:          nil,
	StyleID:          DefaultTableStyleID,
}
