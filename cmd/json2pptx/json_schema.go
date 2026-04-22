package main

import (
	"encoding/json"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/types"
)

// PresentationInput is the top-level typed JSON input.
// Maps to generator.GenerationRequest.
type PresentationInput struct {
	Template       string       `json:"template"`
	OutputFilename string       `json:"output_filename,omitempty"`
	Footer         *JSONFooter  `json:"footer,omitempty"`
	ThemeOverride  *ThemeInput  `json:"theme_override,omitempty"`
	Slides         []SlideInput `json:"slides"`
}

// ThemeInput maps to types.ThemeOverride.
type ThemeInput struct {
	Colors    map[string]string `json:"colors,omitempty"`
	TitleFont string            `json:"title_font,omitempty"`
	BodyFont  string            `json:"body_font,omitempty"`
}

// ToThemeOverride converts ThemeInput to types.ThemeOverride.
func (t *ThemeInput) ToThemeOverride() *types.ThemeOverride {
	if t == nil {
		return nil
	}
	return &types.ThemeOverride{
		Colors:    t.Colors,
		TitleFont: t.TitleFont,
		BodyFont:  t.BodyFont,
	}
}

// SlideInput maps to generator.SlideSpec with full metadata.
type SlideInput struct {
	LayoutID        string           `json:"layout_id,omitempty"`
	SlideType       string           `json:"slide_type,omitempty"` // Optional hint: content, title, section, chart, two-column, diagram, image, comparison, blank
	Background      *BackgroundInput `json:"background,omitempty"`
	Content         []ContentInput   `json:"content"`
	ShapeGrid       *ShapeGridInput  `json:"shape_grid,omitempty"`
	SpeakerNotes    string           `json:"speaker_notes,omitempty"`
	Source          string           `json:"source,omitempty"`
	Transition      string           `json:"transition,omitempty"`
	TransitionSpeed string           `json:"transition_speed,omitempty"`
	Build           string           `json:"build,omitempty"`
}

// BackgroundInput defines a slide background image.
type BackgroundInput struct {
	Image string `json:"image,omitempty"` // File path to background image
	URL   string `json:"url,omitempty"`   // HTTP/HTTPS URL to download background image from
	Fit   string `json:"fit,omitempty"`   // "cover" (default), "stretch", "tile"
}

// ContentInput is a discriminated union for content items.
// The "type" field determines which typed value field to use.
// For backward compat, "value" (json.RawMessage) is also supported.
type ContentInput struct {
	PlaceholderID string `json:"placeholder_id"`
	Type          string `json:"type"`

	// Legacy field — used when typed fields are not set.
	Value json.RawMessage `json:"value,omitempty"`

	// Typed value fields (use ONE, matching the "type" discriminator):
	TextValue           *string              `json:"text_value,omitempty"`
	BulletsValue        *[]string            `json:"bullets_value,omitempty"`
	BodyAndBulletsValue *BodyAndBulletsInput `json:"body_and_bullets_value,omitempty"`
	BulletGroupsValue   *BulletGroupsInput   `json:"bullet_groups_value,omitempty"`
	TableValue          *TableInput          `json:"table_value,omitempty"`
	ChartValue          *types.ChartSpec     `json:"chart_value,omitempty"` //nolint:staticcheck // ChartSpec is deprecated but still used for backward compat
	DiagramValue        *types.DiagramSpec   `json:"diagram_value,omitempty"`
	ImageValue          *ImageInput          `json:"image_value,omitempty"`

	// FontSize overrides the template's default font size for this content item.
	// Value is in points (e.g., 72 for 72pt). Only applies to text-based content types.
	FontSize *float64 `json:"font_size,omitempty"`
}

// ResolveValue returns the typed value for this content item.
// Priority: typed field > legacy Value json.RawMessage.
// Returns (value, error). A nil value with nil error signals
// that the caller should use the legacy decode path.
func (c *ContentInput) ResolveValue() (any, error) { //nolint:gocognit,gocyclo
	switch c.Type {
	case "text":
		if c.TextValue != nil {
			return *c.TextValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("text content requires text_value or value")
		}
		var s string
		if err := json.Unmarshal(c.Value, &s); err != nil {
			return nil, fmt.Errorf("invalid text value: %w", err)
		}
		return s, nil

	case "bullets":
		if c.BulletsValue != nil {
			return *c.BulletsValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("bullets content requires bullets_value or value")
		}
		var b []string
		if err := json.Unmarshal(c.Value, &b); err != nil {
			return nil, fmt.Errorf("invalid bullets value: %w", err)
		}
		return b, nil

	case "body_and_bullets":
		if c.BodyAndBulletsValue != nil {
			return c.BodyAndBulletsValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("body_and_bullets content requires body_and_bullets_value or value")
		}
		var v BodyAndBulletsInput
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return nil, fmt.Errorf("invalid body_and_bullets value: %w", err)
		}
		return &v, nil

	case "bullet_groups":
		if c.BulletGroupsValue != nil {
			return c.BulletGroupsValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("bullet_groups content requires bullet_groups_value or value")
		}
		var v BulletGroupsInput
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return nil, fmt.Errorf("invalid bullet_groups value: %w", err)
		}
		return &v, nil

	case "table":
		if c.TableValue != nil {
			return c.TableValue, nil
		}
		if len(c.Value) == 0 {
			return nil, fmt.Errorf("table content requires table_value or value")
		}
		var v TableInput
		if err := json.Unmarshal(c.Value, &v); err != nil {
			return nil, fmt.Errorf("invalid table value: %w", err)
		}
		return &v, nil

	case "chart":
		if c.ChartValue != nil {
			return c.ChartValue, nil
		}
		// nil signals: use legacy decode path in json_mode.go
		return nil, nil

	case "diagram":
		if c.DiagramValue != nil {
			return c.DiagramValue, nil
		}
		return nil, nil

	case "image":
		if c.ImageValue != nil {
			return c.ImageValue, nil
		}
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown content type: %q", c.Type)
	}
}

// BodyAndBulletsInput maps to generator.BodyAndBulletsContent.
type BodyAndBulletsInput struct {
	Body         string   `json:"body"`
	Bullets      []string `json:"bullets"`
	TrailingBody string   `json:"trailing_body,omitempty"`
}

// BulletGroupsInput maps to generator.BulletGroupsContent.
type BulletGroupsInput struct {
	Body         string             `json:"body,omitempty"`
	Groups       []BulletGroupInput `json:"groups"`
	TrailingBody string             `json:"trailing_body,omitempty"`
}

// BulletGroupInput maps to generator.BulletGroup.
type BulletGroupInput struct {
	Header  string   `json:"header,omitempty"`
	Body    string   `json:"body,omitempty"`
	Bullets []string `json:"bullets"`
}

// TableInput represents a table with headers, rows, and optional styling.
type TableInput struct {
	Headers          []string           `json:"headers"`
	Rows             [][]TableCellInput `json:"rows"`
	Style            *TableStyleInput   `json:"style,omitempty"`
	ColumnAlignments []string           `json:"column_alignments,omitempty"`
}

// ToTableSpec converts TableInput to types.TableSpec.
func (t *TableInput) ToTableSpec() *types.TableSpec {
	if t == nil {
		return nil
	}
	spec := &types.TableSpec{
		Headers:          t.Headers,
		ColumnAlignments: t.ColumnAlignments,
	}
	for _, row := range t.Rows {
		cells := make([]types.TableCell, len(row))
		for j, cell := range row {
			cells[j] = types.TableCell{
				Content: cell.Content,
				ColSpan: cell.ColSpan,
				RowSpan: cell.RowSpan,
			}
		}
		spec.Rows = append(spec.Rows, cells)
	}
	if t.Style != nil {
		spec.Style = types.TableStyle{
			Borders: t.Style.Borders,
			Striped: t.Style.Striped,
		}
		if t.Style.HeaderBackground != nil {
			spec.Style.HeaderBackground = *t.Style.HeaderBackground
		}
	} else {
		spec.Style = types.DefaultTableStyle
	}
	return spec
}

// TableCellInput supports both string shorthand and full object form.
type TableCellInput struct {
	Content string `json:"content"`
	ColSpan int    `json:"col_span,omitempty"`
	RowSpan int    `json:"row_span,omitempty"`
}

// UnmarshalJSON supports string shorthand: "cell text" or {"content":"cell text","col_span":2}.
func (c *TableCellInput) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		c.Content = s
		c.ColSpan = 1
		c.RowSpan = 1
		return nil
	}
	type alias TableCellInput
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("TableCellInput must be string or {content, col_span, row_span}: %w", err)
	}
	*c = TableCellInput(a)
	if c.ColSpan == 0 {
		c.ColSpan = 1
	}
	if c.RowSpan == 0 {
		c.RowSpan = 1
	}
	return nil
}

// TableStyleInput maps to types.TableStyle.
type TableStyleInput struct {
	HeaderBackground *string `json:"header_background,omitempty"`
	Borders          string  `json:"borders,omitempty"`
	Striped          bool    `json:"striped,omitempty"`
}

// ImageInput maps to generator.ImageContent.
type ImageInput struct {
	Path string `json:"path,omitempty"`
	URL  string `json:"url,omitempty"` // HTTP/HTTPS URL to download the image from
	Alt  string `json:"alt,omitempty"`
}

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
// Only one of Shape, Table, Icon, or Image should be set per cell.
type GridCellInput struct {
	ColSpan   int              `json:"col_span,omitempty"`
	RowSpan   int              `json:"row_span,omitempty"`
	Fit       string           `json:"fit,omitempty"` // "contain", "fit-width", "fit-height" (default: stretch)
	Shape     *ShapeSpecInput  `json:"shape,omitempty"`
	Table     *TableInput      `json:"table,omitempty"`
	Icon      *IconInput       `json:"icon,omitempty"`
	Image     *GridImageInput  `json:"image,omitempty"`
	AccentBar *AccentBarInput  `json:"accent_bar,omitempty"` // Optional decorative accent bar
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

