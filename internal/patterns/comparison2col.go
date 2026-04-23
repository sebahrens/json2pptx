package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// comparison-2col pattern — two-column compare (pros/cons, before/after)
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&comparison2col{})
}

type comparison2col struct{}

func (c *comparison2col) Name() string           { return "comparison-2col" }
func (c *comparison2col) Description() string    { return "Two-column comparison with optional headers" }
func (c *comparison2col) UseWhen() string        { return "Two-column compare (pros/cons, vs.)" }
func (c *comparison2col) Version() int           { return 1 }
func (c *comparison2col) CellsHint() string      { return "2 + header" }
func (c *comparison2col) SupportsCallout() bool  { return true }

func (c *comparison2col) ExemplarValues() any {
	return &Comparison2colValues{
		HeaderLeft:  "Pros",
		HeaderRight: "Cons",
		Rows: []Comparison2colRow{
			{Left: "Fast", Right: "Expensive"},
			{Left: "Reliable", Right: "Complex"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Comparison2colRow is a single row with a left and right cell.
type Comparison2colRow struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

// Comparison2colValues is the values type for comparison-2col.
type Comparison2colValues struct {
	HeaderLeft  string              `json:"header_left,omitempty"`
	HeaderRight string              `json:"header_right,omitempty"`
	Rows        []Comparison2colRow `json:"rows"`
}

// Comparison2colOverrides contains pattern-level overrides for comparison-2col.
type Comparison2colOverrides struct {
	Accent       string  `json:"accent,omitempty"`
	HeaderSize   float64 `json:"header_size,omitempty"`
	BodySize     float64 `json:"body_size,omitempty"`
}

// Comparison2colCellOverride contains per-cell overrides for comparison-2col.
type Comparison2colCellOverride struct {
	AccentBar     bool    `json:"accent_bar,omitempty"`
	Emphasis      string  `json:"emphasis,omitempty"`
	Align         string  `json:"align,omitempty"`
	VerticalAlign string  `json:"vertical_align,omitempty"`
	FontSize      float64 `json:"font_size,omitempty"`
	Color         string  `json:"color,omitempty"`
}

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (c *comparison2col) NewValues() any      { return &Comparison2colValues{} }
func (c *comparison2col) NewOverrides() any   { return &Comparison2colOverrides{} }
func (c *comparison2col) NewCellOverride() any { return &Comparison2colCellOverride{} }

// comparison2colCellOverrideAllowed is the whitelist of per-cell override keys (D15).
var comparison2colCellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}

func (c *comparison2col) Schema() *Schema {
	rowSchema := ObjectSchema(
		map[string]*Schema{
			"left":  StringSchema(200).WithDescription("Left column content"),
			"right": StringSchema(200).WithDescription("Right column content"),
		},
		[]string{"left", "right"},
	).WithAdditionalProperties(false)

	cellOverrideSchema := ObjectSchema(
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

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"header_left":  StringSchema(60).WithDescription("Optional left column header"),
			"header_right": StringSchema(60).WithDescription("Optional right column header"),
			"rows":         ArraySchema(rowSchema, 1, 10).WithDescription("Comparison rows (1–10)"),
		},
		[]string{"rows"},
	).WithAdditionalProperties(false)

	// Cell indices: 0 = left header, 1 = right header, then 2..2+2*len(rows)-1
	// for body cells (left0, right0, left1, right1, ...).
	// We define a pattern of cell_overrides with string keys.
	cellOverridesProps := make(map[string]*Schema)
	for i := 0; i < 22; i++ { // max: 2 headers + 10 rows * 2 cells = 22
		cellOverridesProps[fmt.Sprintf("%d", i)] = cellOverrideSchema
	}

	return ObjectSchema(
		map[string]*Schema{
			"values": valuesSchema,
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":      StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"header_size": NumberSchema(6, 120).WithDescription("Font size for header text in points"),
					"body_size":   NumberSchema(6, 120).WithDescription("Font size for body text in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": ObjectSchema(
				cellOverridesProps,
				nil,
			).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDescription("Two-column comparison with optional headers")
}

func (c *comparison2col) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*Comparison2colValues)
	if !ok || vals == nil {
		return fmt.Errorf("comparison-2col: values must be *Comparison2colValues, got %T", values)
	}

	var errs []error

	// Rows required and count check
	if len(vals.Rows) == 0 {
		errs = append(errs, fmt.Errorf("comparison-2col: rows must contain at least 1 row"))
	}
	if len(vals.Rows) > 10 {
		errs = append(errs, fmt.Errorf("comparison-2col: rows must contain at most 10 rows, got %d", len(vals.Rows)))
	}

	// Per-row validation
	for i, row := range vals.Rows {
		if row.Left == "" {
			errs = append(errs, fmt.Errorf("comparison-2col: rows[%d].left is required", i))
		} else if len(row.Left) > 200 {
			errs = append(errs, fmt.Errorf("comparison-2col: rows[%d].left exceeds maxLength 200 (%d chars)", i, len(row.Left)))
		}
		if row.Right == "" {
			errs = append(errs, fmt.Errorf("comparison-2col: rows[%d].right is required", i))
		} else if len(row.Right) > 200 {
			errs = append(errs, fmt.Errorf("comparison-2col: rows[%d].right exceeds maxLength 200 (%d chars)", i, len(row.Right)))
		}
	}

	// Header length checks
	if len(vals.HeaderLeft) > 60 {
		errs = append(errs, fmt.Errorf("comparison-2col: header_left exceeds maxLength 60 (%d chars)", len(vals.HeaderLeft)))
	}
	if len(vals.HeaderRight) > 60 {
		errs = append(errs, fmt.Errorf("comparison-2col: header_right exceeds maxLength 60 (%d chars)", len(vals.HeaderRight)))
	}

	// Compute total cell count for cell_overrides validation
	totalCells := len(vals.Rows) * 2
	hasHeaders := vals.HeaderLeft != "" || vals.HeaderRight != ""
	if hasHeaders {
		totalCells += 2
	}

	// Validate cell_overrides keys (D15 whitelist)
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= totalCells {
			errs = append(errs, fmt.Errorf("comparison-2col: cell_overrides key %d out of range [0,%d]", idx, totalCells-1))
			continue
		}
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("comparison-2col: cell_overrides[%d]: %w", idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("comparison-2col: cell_overrides[%d]: %w", idx, err))
			continue
		}
		for key := range keyMap {
			if !comparison2colCellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("comparison-2col: cell_overrides[%d] contains unknown key %q", idx, key))
			}
		}
	}

	return errors.Join(errs...)
}

func (c *comparison2col) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*Comparison2colValues)
	if !ok {
		return nil, fmt.Errorf("comparison-2col: values must be *Comparison2colValues, got %T", values)
	}
	ovr := &Comparison2colOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*Comparison2colOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("comparison-2col: overrides must be *Comparison2colOverrides, got %T", overrides)
		}
	}

	accent := "accent1"
	if ovr.Accent != "" {
		accent = ovr.Accent
	}
	headerSize := 18.0
	if ovr.HeaderSize > 0 {
		headerSize = ovr.HeaderSize
	}
	bodySize := 14.0
	if ovr.BodySize > 0 {
		bodySize = ovr.BodySize
	}

	hasHeaders := vals.HeaderLeft != "" || vals.HeaderRight != ""
	cellIdx := 0 // running cell index for cell_overrides

	var rows []jsonschema.GridRowInput

	// Header row (optional)
	if hasHeaders {
		leftHeader := buildComparison2colTextContent(vals.HeaderLeft, headerSize, true, "lt1", "ctr")
		rightHeader := buildComparison2colTextContent(vals.HeaderRight, headerSize, true, "lt1", "ctr")

		leftCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     leftHeader,
			},
		}
		applyComparison2colCellOverride(leftCell, cellOverrides, cellIdx, accent)
		cellIdx++

		rightCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
				Text:     rightHeader,
			},
		}
		applyComparison2colCellOverride(rightCell, cellOverrides, cellIdx, accent)
		cellIdx++

		rows = append(rows, jsonschema.GridRowInput{
			Cells: []*jsonschema.GridCellInput{leftCell, rightCell},
		})
	}

	// Body rows
	for _, row := range vals.Rows {
		leftText := buildComparison2colTextContent(row.Left, bodySize, false, "dk1", "l")
		rightText := buildComparison2colTextContent(row.Right, bodySize, false, "dk1", "l")

		leftCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt1"`),
				Text:     leftText,
			},
		}
		applyComparison2colCellOverride(leftCell, cellOverrides, cellIdx, accent)
		cellIdx++

		rightCell := &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt1"`),
				Text:     rightText,
			},
		}
		applyComparison2colCellOverride(rightCell, cellOverrides, cellIdx, accent)
		cellIdx++

		rows = append(rows, jsonschema.GridRowInput{
			Cells: []*jsonschema.GridCellInput{leftCell, rightCell},
		})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`2`),
		Gap:     8,
		Rows:    rows,
	}

	return grid, nil
}

// buildComparison2colTextContent creates a JSON text object for a comparison cell.
func buildComparison2colTextContent(content string, size float64, bold bool, color, align string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: bold, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}

// applyComparison2colCellOverride applies cell_overrides for a given cell index.
func applyComparison2colCellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*Comparison2colCellOverride)
	if !coOk {
		return
	}
	if cellOvr.AccentBar {
		cell.AccentBar = &jsonschema.AccentBarInput{
			Position: "left",
			Color:    accent,
			Width:    4,
		}
	}
}
