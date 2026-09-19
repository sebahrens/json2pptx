package types

import "regexp"

// TableSpec represents a parsed markdown table.
type TableSpec struct {
	Headers          []string      // Column headers (expanded: merged cells have empty strings)
	HeaderCells      []TableCell   // Header cells with colspan/rowspan info (nil if no merges in header)
	Rows             [][]TableCell // Data rows (each row is a slice of cells)
	Style            TableStyle    // Table styling options
	Merges           []CellMerge   // List of merge regions
	ColumnAlignments []string      // Per-column alignment: "left", "center", "right" (from separator row)
}

// TableCell represents a single cell in a table.
type TableCell struct {
	Content     string             // Text content of the cell
	ColSpan     int                // Number of columns this cell spans (default 1)
	RowSpan     int                // Number of rows this cell spans (default 1)
	IsMerged    bool               // True if this cell is part of a merge (not the origin)
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
	HeaderBackground string   // "accent1"-"accent6", "none", or hex color
	Borders          string   // "all", "horizontal", "outer", "none"
	Striped          *bool    // Alternating row colors (nil = default on, explicit false = off)
	StyleID          string   // OOXML table style GUID (e.g., "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
	UseTableStyle    bool     // When true, suppress all explicit formatting and let the table style control appearance
	HighlightColumn  int      // 1-indexed column to highlight with accent3 tint fill (0 = none)
	TotalsRow        bool     // When true, last data row rendered bold with top border
	ColumnTypes      []string // Per-column type: "text", "number", "currency", "percent", "delta"
}

// ConditionalFormat defines a rule-based cell fill for conditional formatting.
// The rule is evaluated against the cell's own content: a cell that does not
// satisfy it keeps the table's normal fill (go-slide-creator-6hlu).
type ConditionalFormat struct {
	// Rule is one of the ConditionalRule* values. An empty rule means "always",
	// which is how a cell asks for a plain tint.
	Rule string
	// Threshold is the operand the rule compares the cell against: a float64
	// for the numeric rules, a string for equals / contains, and a
	// [2]float64-shaped []float64 for between. It is nil when the rule needs
	// none. Text and numbers are both legitimate here — a RAG table compares
	// against "On track", a variance table against 0 — so this is deliberately
	// untyped rather than a number the parser would reject a string for.
	Threshold any
	// Fill is the scheme color or 6-digit hex applied when the rule matches.
	Fill string
}

// Conditional formatting rules. The vocabulary is closed: an unrecognised rule
// is reported by validation and applies no fill.
const (
	// ConditionalRuleAlways tints the cell unconditionally; it is what an empty
	// rule means, and the form a hand-highlighted cell uses.
	ConditionalRuleAlways = "always"
	// ConditionalRulePositive / Negative read the cell as a number.
	ConditionalRulePositive = "positive"
	ConditionalRuleNegative = "negative"
	// ConditionalRuleThreshold is the historical spelling of gte.
	ConditionalRuleThreshold = "threshold"
	ConditionalRuleGTE       = "gte"
	ConditionalRuleLTE       = "lte"
	ConditionalRuleBetween   = "between"
	// ConditionalRuleEquals / Contains compare the cell's text, case- and
	// space-insensitively. equals also matches numerically when both sides
	// parse as numbers, so 0.5 matches "50%".
	ConditionalRuleEquals   = "equals"
	ConditionalRuleContains = "contains"
)

// ConditionalRules is the closed rule vocabulary, in the order validation
// reports it.
var ConditionalRules = []string{
	ConditionalRuleAlways,
	ConditionalRulePositive,
	ConditionalRuleNegative,
	ConditionalRuleThreshold,
	ConditionalRuleGTE,
	ConditionalRuleLTE,
	ConditionalRuleBetween,
	ConditionalRuleEquals,
	ConditionalRuleContains,
}

// ConditionalRuleKnown reports whether a rule is in the vocabulary. The empty
// rule is known: it means "always".
func ConditionalRuleKnown(rule string) bool {
	if rule == "" {
		return true
	}
	for _, r := range ConditionalRules {
		if r == rule {
			return true
		}
	}
	return false
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
