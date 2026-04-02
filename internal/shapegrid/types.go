// Package shapegrid provides layout resolution for grids of preset geometry shapes.
// It converts a declarative grid specification (rows, columns, spans, gaps) into
// resolved cells with absolute EMU coordinates, ready for XML generation.
package shapegrid

import (
	"encoding/json"

	"github.com/ahrens/go-slide-creator/internal/pptx"
	"github.com/ahrens/go-slide-creator/internal/types"
)

// CellKind identifies the type of content in a grid cell.
type CellKind string

const (
	CellKindShape CellKind = "shape"
	CellKindTable CellKind = "table"
	CellKindIcon  CellKind = "icon"
	CellKindImage CellKind = "image"
)

// FitMode controls how a shape is scaled within its grid cell bounds.
type FitMode string

const (
	// FitStretch fills the entire cell (default behavior).
	FitStretch FitMode = ""
	// FitContain scales the shape to fit within the cell while preserving
	// a 1:1 aspect ratio, using the smaller dimension. Centered in cell.
	FitContain FitMode = "contain"
	// FitWidth matches the cell width; height equals width. Centered vertically.
	FitWidth FitMode = "fit-width"
	// FitHeight matches the cell height; width equals height. Centered horizontally.
	FitHeight FitMode = "fit-height"
)

// Grid is the domain representation of a shape grid layout.
type Grid struct {
	Bounds  pptx.RectEmu // Absolute bounds in EMU
	Columns []float64    // Column width percentages (sum to 100)
	Rows    []Row        // Row definitions
	ColGap  float64      // Column gap in points (1pt = 12700 EMU)
	RowGap  float64      // Row gap in points (1pt = 12700 EMU)
}

// Row is a single row in the grid.
type Row struct {
	Height     float64        // Percentage of grid height (0 = equal split of remaining)
	AutoHeight bool           // When true, estimate height from text content (overrides Height=0)
	Cells      []Cell         // Cells in this row
	Connector  *ConnectorSpec // Optional connector between adjacent cells in this row
}

// ConnectorSpec defines connectors drawn between adjacent cells in a row.
type ConnectorSpec struct {
	Style string  // "arrow" (with tail arrowhead), "line" (no arrowheads). Default: "line"
	Color string  // Hex color (e.g., "FF0000") or scheme ref (e.g., "accent1"). Default: "000000"
	Width float64 // Line width in points. Default: 1.0
	Dash  string  // Dash style: "solid", "dash", "dot", "lgDash", "dashDot". Default: "solid"
}

// Cell is a single cell in the grid.
// Only one of Shape, TableSpec, Icon, or Image should be set per cell.
type Cell struct {
	ColSpan   int              // Number of columns to span (default 1)
	RowSpan   int              // Number of rows to span (default 1)
	Fit       FitMode          // How the shape scales within cell bounds
	Shape     *ShapeSpec       // Shape specification (nil = empty cell unless TableSpec/Icon/Image set)
	TableSpec *types.TableSpec // Table specification (nil = empty cell unless Shape/Icon/Image set)
	Icon      *IconSpec        // Icon specification (nil = empty cell unless Shape/TableSpec/Image set)
	Image     *ImageSpec       // Image specification (nil = empty cell unless Shape/TableSpec/Icon set)
	AccentBar *AccentBarSpec   // Optional decorative accent bar alongside the cell
}

// AccentBarSpec defines a decorative accent bar rendered alongside a cell.
type AccentBarSpec struct {
	Position string  // "left", "right", "top", "bottom" (default: "left")
	Color    string  // Hex color (e.g., "FF0000") or scheme ref (e.g., "accent1"). Default: "accent1"
	Width    float64 // Bar thickness in points. Default: 4.0
}

// IconSpec defines an embedded SVG icon from the bundled icon library or a custom SVG file.
// Exactly one of Name or Path should be set.
type IconSpec struct {
	Name     string  // Bundled icon name (e.g., "chart-pie", "filled:alert-circle")
	Path     string  // File path to a custom SVG icon (absolute, resolved from JSON input directory)
	Fill     string  // Optional fill color override (hex, e.g., "#FF0000"). Only applies to bundled icons.
	Scale    float64 // Scale factor 0.0-1.0 for icon sizing (default: 1.0 for standalone, 0.6 for overlay on shape)
	Position string  // Icon position relative to text: "left", "top", "center". Auto-detected if empty.
}

// ImageSpec defines an image to embed in a grid cell.
type ImageSpec struct {
	Path    string       // File path to the image (PNG, JPG, etc.)
	Alt     string       // Alt text / description for accessibility
	Overlay *OverlaySpec // Optional semi-transparent overlay on top of image
	Text    *ImageText   // Optional text label rendered on top of image (and overlay)
}

// OverlaySpec defines a semi-transparent color overlay rendered on top of an image.
type OverlaySpec struct {
	Color string  // Hex color (e.g., "000000") or scheme ref (e.g., "dk1"). Default: "000000"
	Alpha float64 // Opacity from 0.0 (transparent) to 1.0 (opaque). Default: 0.4
}

// ImageText defines a text label rendered on top of an image cell.
type ImageText struct {
	Content       string  // Text content
	Size          float64 // Font size in points (default: 14)
	Bold          bool    // Bold text
	Color         string  // Text color (hex or scheme ref). Default: "FFFFFF" (white)
	Align         string  // Horizontal alignment: "l", "ctr", "r". Default: "ctr"
	VerticalAlign string  // Vertical alignment: "t", "ctr", "b". Default: "b" (bottom)
	Font          string  // Font family. Default: theme minor font
}


// ShapeSpec defines a preset geometry shape with fill, line, and text.
type ShapeSpec struct {
	Geometry    string
	Fill        json.RawMessage
	Line        json.RawMessage
	Text        json.RawMessage
	Rotation    float64
	Adjustments map[string]int64
}

// ResolvedCell is the output of grid resolution: a cell with its absolute
// position, size, kind, and associated specification.
type ResolvedCell struct {
	Kind       CellKind
	Bounds     pptx.RectEmu     // Shape bounds (may differ from cell bounds when fit mode is applied)
	CellBounds pptx.RectEmu     // Original cell bounds before fit adjustment
	IconBounds pptx.RectEmu     // Icon overlay bounds (contained square within shape bounds); zero when no icon overlay
	TextInsets [4]int64         // Extra text insets [L,T,R,B] in EMU to avoid icon overlap (added to any JSON-specified insets)
	ID         uint32
	ShapeSpec  *ShapeSpec       // Set when Kind == CellKindShape
	TableSpec  *types.TableSpec // Set when Kind == CellKindTable
	IconSpec   *IconSpec        // Set when Kind == CellKindIcon
	ImageSpec  *ImageSpec       // Set when Kind == CellKindImage
}

// ResolvedConnector is a connector line between two adjacent cells in a row.
type ResolvedConnector struct {
	Bounds    pptx.RectEmu   // Position and size of the connector
	ID        uint32         // Unique shape ID
	Spec      *ConnectorSpec // Connector styling
	SourceID  uint32         // Shape ID of the source cell
	TargetID  uint32         // Shape ID of the target cell
	StartSite int            // Connection site index on source
	EndSite   int            // Connection site index on target
}

// ResolvedAccentBar is a decorative accent bar shape attached to a cell.
type ResolvedAccentBar struct {
	Bounds pptx.RectEmu   // Position and size of the bar
	ID     uint32         // Unique shape ID
	Spec   *AccentBarSpec // Accent bar styling
}

// ResolveResult holds the output of grid resolution: resolved cells, connectors, and accent bars.
type ResolveResult struct {
	Cells      []ResolvedCell
	Connectors []ResolvedConnector
	AccentBars []ResolvedAccentBar
}
