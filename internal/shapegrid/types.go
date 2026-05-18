// Package shapegrid provides layout resolution for grids of preset geometry shapes.
// It converts a declarative grid specification (rows, columns, spans, gaps) into
// resolved cells with absolute EMU coordinates, ready for XML generation.
package shapegrid

import (
	"encoding/json"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
)

// CellKind identifies the type of content in a grid cell.
type CellKind string

const (
	CellKindShape   CellKind = "shape"
	CellKindTable   CellKind = "table"
	CellKindIcon    CellKind = "icon"
	CellKindImage   CellKind = "image"
	CellKindDiagram CellKind = "diagram"
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
	Bounds  pptx.RectEmu // Absolute bounds in EMU (authoritative — never shrunk)
	Columns []float64    // Column width percentages (sum to 100)
	Rows    []Row        // Row definitions
	ColGap  float64      // Column gap in points (1pt = 12700 EMU)
	RowGap  float64      // Row gap in points (1pt = 12700 EMU)
}

// Row is a single row in the grid.
//
// Height allocation uses CSS-flex-like semantics:
//   - Height > 0: fixed percentage of grid height (allocated first)
//   - AutoHeight: estimate preferred height from text content (allocated second)
//   - Flex > 0: proportional share of remaining space (default Flex=1 for unspecified rows)
//
// MinHeight and MaxHeight (in points) constrain the resolved height.
type Row struct {
	Height     float64        // Percentage of grid height (0 = flex item)
	AutoHeight bool           // When true, estimate height from text content
	Flex       float64        // Flex-grow factor for remaining space distribution (default 1 when Height==0 && !AutoHeight)
	MinHeight  float64        // Minimum row height in points (0 = no minimum)
	MaxHeight  float64        // Maximum row height in points (0 = no maximum)
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
// Only one of Shape, TableSpec, Icon, Image, DiagramSpec, or Composite should be
// set per cell. Composite is the sole exception: it bundles a native text shape
// and an embedded sub-diagram inside the same cell rectangle.
type Cell struct {
	ColSpan     int                // Number of columns to span (default 1)
	RowSpan     int                // Number of rows to span (default 1)
	Fit         FitMode            // How the shape scales within cell bounds
	Group       bool               // Wrap cell content in a p:grpSp group shape
	Shape       *ShapeSpec         // Shape specification (nil = empty cell unless other content set)
	TableSpec   *types.TableSpec   // Table specification
	Icon        *IconSpec          // Icon specification
	Image       *ImageSpec         // Image specification
	DiagramSpec *types.DiagramSpec // Diagram/chart specification (rendered via svggen)
	Composite   *CompositeSpec     // Composite stack: native text + sub-diagram in one cell
	AccentBar   *AccentBarSpec     // Optional decorative accent bar alongside the cell
}

// CompositeSpec defines a composite cell that stacks a native text shape and
// an embedded sub-diagram (chart) inside the same cell rectangle. The cell is
// vertically split into a "text" portion and a "diagram" portion; Split chooses
// which portion is on top, and Ratio controls the fraction of cell height
// allocated to the Text portion.
//
// Use cases: KPI + sparkline, headline + mini chart, callout + small diagram.
type CompositeSpec struct {
	Text       *ShapeSpec         // Native text shape (required)
	SubDiagram *types.DiagramSpec // Sub-diagram/chart rendered via svggen (required)
	Split      CompositeSplit     // Which portion is on top ("top" = text on top, "bottom" = text on bottom). Default: "top".
	Ratio      float64            // Fraction of cell height for the Text portion (0.0–1.0, exclusive). Default: 0.5.
}

// CompositeSplit identifies which portion of a composite cell hosts the text shape.
type CompositeSplit string

const (
	// CompositeSplitDefault is the unset value; treated as CompositeSplitTop.
	CompositeSplitDefault CompositeSplit = ""
	// CompositeSplitTop places the text shape in the top portion, sub-diagram below.
	CompositeSplitTop CompositeSplit = "top"
	// CompositeSplitBottom places the text shape in the bottom portion, sub-diagram above.
	CompositeSplitBottom CompositeSplit = "bottom"
)

// AccentBarSpec defines a decorative accent bar rendered alongside a cell.
type AccentBarSpec struct {
	Position string  // "left", "right", "top", "bottom" (default: "left")
	Color    string  // Hex color (e.g., "FF0000") or scheme ref (e.g., "accent1"). Default: "accent1"
	Width    float64 // Bar thickness in points. Default: 4.0
}

// IconSpec defines an embedded SVG icon from the bundled icon library, a custom SVG file,
// or inline SVG markup. Exactly one of Name, Path, or SVGData should be set.
type IconSpec struct {
	Name     string  // Bundled icon name (e.g., "chart-pie", "filled:alert-circle")
	Path     string  // File path to a custom SVG icon (absolute, resolved from JSON input directory)
	SVGData  string  // Inline SVG markup. When set, no disk I/O is performed and Fill is ignored.
	Alt      string  // Optional explicit alt text; falls back to a derived value when empty.
	Fill     string  // Optional fill color override (hex, e.g., "#FF0000"). Applies to bundled and path-based SVG icons.
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
	Kind        CellKind
	Bounds      pptx.RectEmu       // Shape bounds (may differ from cell bounds when fit mode is applied)
	CellBounds  pptx.RectEmu       // Original cell bounds before fit adjustment
	IconBounds  pptx.RectEmu       // Icon overlay bounds (contained square within shape bounds); zero when no icon overlay
	TextInsets  [4]int64           // Extra text insets [L,T,R,B] in EMU to avoid icon overlap (added to any JSON-specified insets)
	ID          uint32
	RowIdx      int                // Zero-based row index in the source grid
	ColIdx      int                // Zero-based column index in the source grid
	Group       bool               // Wrap cell content in a p:grpSp group shape
	ShapeSpec   *ShapeSpec         // Set when Kind == CellKindShape
	TableSpec   *types.TableSpec   // Set when Kind == CellKindTable
	IconSpec    *IconSpec          // Set when Kind == CellKindIcon
	ImageSpec   *ImageSpec         // Set when Kind == CellKindImage
	DiagramSpec *types.DiagramSpec // Set when Kind == CellKindDiagram
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

// RowOverflow records a row whose content exceeds its max_height constraint.
type RowOverflow struct {
	RowIndex   int     // Zero-based row index
	ContentPt  float64 // Estimated content height in points
	MaxHeightPt float64 // Max height constraint in points
}

// ResolveResult holds the output of grid resolution: resolved cells, connectors, accent bars,
// and any row overflow diagnostics.
type ResolveResult struct {
	Cells        []ResolvedCell
	Connectors   []ResolvedConnector
	AccentBars   []ResolvedAccentBar
	RowOverflows []RowOverflow // Rows whose content exceeded max_height
}
