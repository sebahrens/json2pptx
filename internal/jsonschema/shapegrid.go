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
	Bounds  *GridBoundsInput `json:"bounds,omitempty"`
	Gap     float64          `json:"gap,omitempty"`     // Gap in points (default 8pt). Applies to both col and row gaps.
	ColGap  float64          `json:"col_gap,omitempty"` // Column gap in points (overrides gap)
	RowGap  float64          `json:"row_gap,omitempty"` // Row gap in points (overrides gap)
	Columns json.RawMessage  `json:"columns,omitempty"` // number | number[]
	Rows    []GridRowInput   `json:"rows"`
}

// GridBoundsInput defines the bounding rectangle as percentages of slide dimensions.
type GridBoundsInput struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// GridRowInput defines a single row in the shape grid.
type GridRowInput struct {
	Height     float64             `json:"height,omitempty"`      // Percentage of grid height (0 = equal split)
	AutoHeight bool                `json:"auto_height,omitempty"` // Estimate height from text content
	Cells      []*GridCellInput    `json:"cells"`
	Connector  *ConnectorSpecInput `json:"connector,omitempty"` // Optional connector lines between adjacent cells
}

// ConnectorSpecInput defines the style of connector lines between adjacent cells in a row.
type ConnectorSpecInput struct {
	Style string  `json:"style,omitempty"` // "arrow" or "line" (default: "line")
	Color string  `json:"color,omitempty"` // Hex color or scheme ref (default: "000000")
	Width float64 `json:"width,omitempty"` // Width in points (default: 1.0)
	Dash  string  `json:"dash,omitempty"`  // "solid", "dash", "dot", "lgDash", "dashDot" (default: "solid")
}

// GridCellInput defines a single cell in the shape grid.
// Only one of Shape, Table, Icon, Image, or Diagram should be set per cell.
type GridCellInput struct {
	ColSpan   int                `json:"col_span,omitempty"`
	RowSpan   int                `json:"row_span,omitempty"`
	Fit       string             `json:"fit,omitempty"` // "contain", "fit-width", "fit-height" (default: stretch)
	Shape     *ShapeSpecInput    `json:"shape,omitempty"`
	Table     *TableInput        `json:"table,omitempty"`
	Icon      *IconInput         `json:"icon,omitempty"`
	Image     *GridImageInput    `json:"image,omitempty"`
	Diagram   *types.DiagramSpec `json:"diagram,omitempty"` // Chart/diagram rendered via svggen
	AccentBar *AccentBarInput    `json:"accent_bar,omitempty"` // Optional decorative accent bar
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

// IconInput defines an SVG icon from the bundled icon library, a custom SVG file, or a URL.
// Exactly one of Name, Path, or URL must be set.
type IconInput struct {
	Name     string `json:"name,omitempty"`      // Bundled icon name (e.g., "chart-pie", "filled:alert-circle")
	Path     string `json:"path,omitempty"`      // File path to a custom SVG icon (relative to JSON input directory)
	URL      string `json:"url,omitempty"`       // HTTP/HTTPS URL to download an SVG icon from
	Fill     string `json:"fill,omitempty"`      // Optional fill color override (hex, e.g., "#FF0000"). Only supported for bundled icons.
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
