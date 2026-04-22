package types

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
	Content  string // Text content of the cell
	ColSpan  int    // Number of columns this cell spans (default 1)
	RowSpan  int    // Number of rows this cell spans (default 1)
	IsMerged bool   // True if this cell is part of a merge (not the origin)
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
	Striped          bool   // Alternating row colors
	StyleID          string // OOXML table style GUID (e.g., "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
	UseTableStyle    bool   // When true, suppress all explicit formatting and let the table style control appearance
}

// DefaultTableStyleID is the OOXML GUID for "Medium Style 2 - Accent 1",
// a professional themed table style that adapts to the presentation color scheme.
const DefaultTableStyleID = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"

// DefaultTableStyle provides sensible defaults for table styling.
// HeaderBackground is intentionally empty so the table style's firstRow
// appearance takes effect; set it explicitly to override.
var DefaultTableStyle = TableStyle{
	HeaderBackground: "",
	Borders:          "all",
	Striped:          false,
	StyleID:          DefaultTableStyleID,
}
