// Package jsonschema defines the JSON DTO (Data Transfer Object) types used to
// deserialize structured JSON input for PPTX generation. These types are the
// canonical input schema for shape grids and tables, shared by cmd/json2pptx
// (the CLI) and internal/patterns (the pattern library).
package jsonschema

import (
	"encoding/json"

	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// Shape Grid types
// ---------------------------------------------------------------------------

// ShapeGridInput defines a grid of preset geometry shapes placed on a slide.
type ShapeGridInput struct {
	Bounds     *GridBoundsInput `json:"bounds,omitempty"`
	Gap        float64          `json:"gap,omitempty"`         // Gap in points (default 8pt). Applies to both col and row gaps.
	ColGap     float64          `json:"col_gap,omitempty"`     // Column gap in points (overrides gap)
	RowGap     float64          `json:"row_gap,omitempty"`     // Row gap in points (overrides gap)
	Columns    json.RawMessage  `json:"columns,omitempty"`     // number | number[]
	Rows       []GridRowInput   `json:"rows"`
}

// GridBoundsInput defines the bounding rectangle as percentages of slide dimensions.
type GridBoundsInput struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// GridRowInput defines a single row in the shape grid.
//
// Row sizing uses CSS-flex-like semantics:
//   - height > 0: fixed percentage of grid height
//   - auto_height: estimate from text content
//   - flex > 0: proportional share of remaining space (default 1 for unspecified rows)
//   - min_height / max_height: constraints in points
type GridRowInput struct {
	Height     float64             `json:"height,omitempty"`      // Percentage of grid height (0 = flex item)
	AutoHeight bool                `json:"auto_height,omitempty"` // Estimate height from text content
	Flex       float64             `json:"flex,omitempty"`        // Flex-grow factor (default 1 when height==0 && !auto_height)
	MinHeight  float64             `json:"min_height,omitempty"`  // Minimum row height in points (0 = no minimum)
	MaxHeight  float64             `json:"max_height,omitempty"`  // Maximum row height in points (0 = no maximum)
	Cells      []*GridCellInput    `json:"cells"`
	Connector  *ConnectorSpecInput `json:"connector,omitempty"`   // Optional connector lines between adjacent cells
}

// ConnectorSpecInput defines the style of connector lines between adjacent cells in a row.
type ConnectorSpecInput struct {
	Style string  `json:"style,omitempty"` // "arrow" or "line" (default: "line")
	Color string  `json:"color,omitempty"` // Hex color or scheme ref (default: "000000")
	Width float64 `json:"width,omitempty"` // Width in points (default: 1.0)
	Dash  string  `json:"dash,omitempty"`  // "solid", "dash", "dot", "lgDash", "dashDot" (default: "solid")
}

// GridCellInput defines a single cell in the shape grid.
// Only one of Shape, Table, Icon, Image, Diagram, Composite, Pattern, or Grid
// should be set per cell. Composite is the sole exception to the single-content
// rule: it bundles a native text shape (text) and an embedded chart
// (sub_diagram) inside the same cell.
//
// Pattern hosts a named pattern (e.g. kpi-3up) nested inside the cell — the
// pattern is expanded to a sub-grid that renders within the cell rectangle.
// Grid hosts a raw sub-grid (recursive ShapeGridInput) rendered within the cell.
// Pattern and Grid are mutually exclusive; setting both is a validation error.
type GridCellInput struct {
	ColSpan    int                `json:"col_span,omitempty"`
	RowSpan    int                `json:"row_span,omitempty"`
	Fit        string             `json:"fit,omitempty"` // "contain", "fit-width", "fit-height" (default: stretch)
	Group      bool               `json:"group,omitempty"` // Wrap cell content in a p:grpSp group shape
	Shape      *ShapeSpecInput    `json:"shape,omitempty"`
	Table      *TableInput        `json:"table,omitempty"`
	Icon       *IconInput         `json:"icon,omitempty"`
	Image      *GridImageInput    `json:"image,omitempty"`
	Diagram    *types.DiagramSpec `json:"diagram,omitempty"` // Chart/diagram rendered via svggen
	Composite  *CompositeInput    `json:"composite,omitempty"` // Composite stack: native text + sub-diagram (KPI + sparkline)
	Pattern    json.RawMessage    `json:"pattern,omitempty"`     // Nested PatternInput (expanded to a sub-grid before resolution)
	Grid       *ShapeGridInput    `json:"grid,omitempty"`        // Recursive sub-grid rendered within the cell rectangle
	AccentBar  *AccentBarInput    `json:"accent_bar,omitempty"`  // Optional decorative accent bar
	NamedStyle string             `json:"named_style,omitempty"` // Named cell style reference resolved from template settings
}

// CompositeInput defines a composite cell that stacks a native text shape and
// an embedded sub-diagram inside one grid cell. Both Text and SubDiagram are
// required when Composite is set; legacy mutually-exclusive cell keys (shape,
// table, icon, image, diagram) must not be set on the same cell.
//
// Use cases: KPI + sparkline, headline + mini chart, callout + small diagram.
type CompositeInput struct {
	Text       *ShapeSpecInput    `json:"text,omitempty"`        // Native text shape (required)
	SubDiagram *types.DiagramSpec `json:"sub_diagram,omitempty"` // Sub-diagram (required)
	Split      string             `json:"split,omitempty"`       // "top" (text on top, default) or "bottom" (text on bottom)
	Ratio      float64            `json:"ratio,omitempty"`       // Fraction of cell height for the Text portion (0.0–1.0, exclusive). Default: 0.5.
}

// AccentBarInput defines a decorative accent bar rendered alongside a cell.
type AccentBarInput struct {
	Position string  `json:"position,omitempty"` // "left", "right", "top", "bottom" (default: "left")
	Color    string  `json:"color,omitempty"`    // Hex color or scheme ref (default: "accent1")
	Width    float64 `json:"width,omitempty"`    // Bar thickness in points (default: 4)
}

// GridImageInput defines an image to embed in a shape grid cell.
type GridImageInput struct {
	Path    string              `json:"path,omitempty"`    // File path to the image
	URL     string              `json:"url,omitempty"`     // HTTP/HTTPS URL to download the image from
	Alt     string              `json:"alt,omitempty"`     // Alt text for accessibility
	Overlay *GridOverlayInput   `json:"overlay,omitempty"` // Semi-transparent overlay on top of image
	Text    *GridImageTextInput `json:"text,omitempty"`    // Text label on top of image
}

// GridOverlayInput defines a semi-transparent color overlay on an image.
type GridOverlayInput struct {
	Color string  `json:"color,omitempty"` // Hex color or scheme ref (default: "000000")
	Alpha float64 `json:"alpha,omitempty"` // Opacity 0.0-1.0 (default: 0.4)
}

// GridImageTextInput defines text rendered on top of an image cell.
type GridImageTextInput struct {
	Content       string  `json:"content"`                  // Text content
	Size          float64 `json:"size,omitempty"`           // Font size in points (default: 14)
	Bold          bool    `json:"bold,omitempty"`           // Bold text
	Color         string  `json:"color,omitempty"`          // Text color (default: "FFFFFF")
	Align         string  `json:"align,omitempty"`          // Horizontal: "l", "ctr", "r" (default: "ctr")
	VerticalAlign string  `json:"vertical_align,omitempty"` // Vertical: "t", "ctr", "b" (default: "b")
	Font          string  `json:"font,omitempty"`           // Font family
}

// IconInput defines an SVG icon from the bundled icon library, a custom SVG file,
// a URL, or inline SVG markup. Exactly one of Name, Path, URL, or SVGData must be set.
type IconInput struct {
	Name     string `json:"name,omitempty"`      // Bundled icon name (e.g., "chart-pie", "filled:alert-circle")
	Path     string `json:"path,omitempty"`      // File path to a custom SVG icon (relative to JSON input directory)
	URL      string `json:"url,omitempty"`       // HTTP/HTTPS URL to download an SVG icon from
	SVGData  string `json:"svg_data,omitempty"`  // Inline SVG markup (e.g., output of svggen-mcp render_diagram). When set, no disk I/O is performed.
	Alt      string `json:"alt,omitempty"`       // Alt text / description for accessibility. Falls back to name/path/"icon" when empty.
	Fill     string `json:"fill,omitempty"`      // Optional fill color override (hex, e.g., "#FF0000"). Applies to bundled and custom SVG icons. Ignored for inline svg_data — emits ICON_FILL_IGNORED_ON_INLINE warning if both are set.
	Position string `json:"position,omitempty"`  // Icon position relative to text: "left", "top", "center". Auto-detected if empty.
}

// ShapeSpecInput defines a preset geometry shape with fill, line, and text.
type ShapeSpecInput struct {
	Geometry    string           `json:"geometry"`
	Fill        json.RawMessage  `json:"fill,omitempty"`
	Line        json.RawMessage  `json:"line,omitempty"`
	Text        json.RawMessage  `json:"text,omitempty"`
	Rotation    float64          `json:"rotation,omitempty"`
	Adjustments map[string]int64 `json:"adjustments,omitempty"`
	Icon        *IconInput       `json:"icon,omitempty"` // Optional icon overlay rendered on top of the shape
}

// ShapeFillInput is the expanded object form for shape fill.
type ShapeFillInput struct {
	Color string  `json:"color"`
	Alpha float64 `json:"alpha,omitempty"` // 0-100, percentage
}

// ---------------------------------------------------------------------------
// Slide-level overlays (free-floating shapes on top of the grid)
// ---------------------------------------------------------------------------

// OverlayShapeInput defines a free-floating shape rendered on top of the
// slide's grid (or as the sole content if the slide has no grid). Positioning
// is by percent-of-slide unless an anchor_cell reference overrides the
// from/to point.
//
// Use cases: diagonal arrows between matrix quadrants, floating "roof" badges
// labelling a strategy-house tier, callout pointers, watermark stripes.
//
// Overlays render *after* the grid so they always appear on top.
type OverlayShapeInput struct {
	Kind   string             `json:"kind"`             // "arrow", "line", "badge"
	From   *OverlayPointInput `json:"from,omitempty"`   // Start point (required for arrow/line; defines top-left for badge)
	To     *OverlayPointInput `json:"to,omitempty"`     // End point (required for arrow/line; defines bottom-right for badge when set)
	Color  string             `json:"color,omitempty"`  // Line/arrow stroke color or badge fill (hex or scheme name; default "accent1")
	Width  float64            `json:"width,omitempty"`  // Line/arrow stroke width in points (default 1.5); badge: width in slide-percent when To is omitted
	Height float64            `json:"height,omitempty"` // Badge: height in slide-percent when To is omitted (ignored for line/arrow)
	Dash   string             `json:"dash,omitempty"`   // "solid", "dash", "dot", "lgDash", "dashDot" (line/arrow)
	Text   string             `json:"text,omitempty"`   // Badge label text
}

// OverlayPointInput specifies a position via percent-of-slide or via
// cell-anchor reference. AnchorCell, when set, overrides X/Y.
type OverlayPointInput struct {
	X          float64                `json:"x,omitempty"`           // Percent of slide width (0–100)
	Y          float64                `json:"y,omitempty"`           // Percent of slide height (0–100)
	AnchorCell *OverlayAnchorCellInput `json:"anchor_cell,omitempty"` // Optional cell reference (overrides x/y)
}

// OverlayAnchorCellInput references a cell in the slide's shape_grid by
// 0-based row and column index, and selects a point on its bounds.
type OverlayAnchorCellInput struct {
	Row int    `json:"row"`          // 0-based row index in the resolved grid
	Col int    `json:"col"`          // 0-based column index in the resolved grid
	At  string `json:"at,omitempty"` // "center" (default), "top-left", "top-right", "bottom-left", "bottom-right", "top", "right", "bottom", "left"
}
