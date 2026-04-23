package patterns

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// matrix-2x2 pattern — quadrant/positioning matrix with axis labels
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&matrix2x2{})
}

type matrix2x2 struct{}

func (m *matrix2x2) Name() string        { return "matrix-2x2" }
func (m *matrix2x2) Description() string { return "2×2 quadrant matrix with axis labels" }
func (m *matrix2x2) UseWhen() string     { return "Quadrant/positioning matrix with axis labels" }
func (m *matrix2x2) Version() int        { return 1 }
func (m *matrix2x2) CellsHint() string   { return "4 + axes" }

func (m *matrix2x2) ExemplarValues() any {
	return &Matrix2x2Values{
		XAxisLabel:  "Market Share",
		YAxisLabel:  "Market Growth",
		TopLeft:     Matrix2x2Quadrant{Header: "Stars", Body: "High growth, high share"},
		TopRight:    Matrix2x2Quadrant{Header: "Question Marks", Body: "High growth, low share"},
		BottomLeft:  Matrix2x2Quadrant{Header: "Cash Cows", Body: "Low growth, high share"},
		BottomRight: Matrix2x2Quadrant{Header: "Dogs", Body: "Low growth, low share"},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Matrix2x2Quadrant is a single quadrant cell with a header and body.
type Matrix2x2Quadrant struct {
	Header string `json:"header"`
	Body   string `json:"body,omitempty"`
}

// Matrix2x2Values is the values type for matrix-2x2.
type Matrix2x2Values struct {
	XAxisLabel  string             `json:"x_axis_label"`
	YAxisLabel  string             `json:"y_axis_label"`
	TopLeft     Matrix2x2Quadrant  `json:"top_left"`
	TopRight    Matrix2x2Quadrant  `json:"top_right"`
	BottomLeft  Matrix2x2Quadrant  `json:"bottom_left"`
	BottomRight Matrix2x2Quadrant  `json:"bottom_right"`
}

// Matrix2x2Overrides contains pattern-level overrides for matrix-2x2.
type Matrix2x2Overrides struct {
	Accent     string  `json:"accent,omitempty"`
	HeaderSize float64 `json:"header_size,omitempty"`
	BodySize   float64 `json:"body_size,omitempty"`
	LabelSize  float64 `json:"label_size,omitempty"`
}

// Matrix2x2CellOverride contains per-cell overrides for matrix-2x2 quadrant cells.
type Matrix2x2CellOverride struct {
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

func (m *matrix2x2) NewValues() any       { return &Matrix2x2Values{} }
func (m *matrix2x2) NewOverrides() any    { return &Matrix2x2Overrides{} }
func (m *matrix2x2) NewCellOverride() any { return &Matrix2x2CellOverride{} }

// matrix2x2CellOverrideAllowed is the whitelist of per-cell override keys (D15).
var matrix2x2CellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}

func (m *matrix2x2) Schema() *Schema {
	quadrantSchema := ObjectSchema(
		map[string]*Schema{
			"header": StringSchema(80).WithDescription("Quadrant header text"),
			"body":   StringSchema(200).WithDescription("Quadrant body text"),
		},
		[]string{"header"},
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
			"x_axis_label": StringSchema(60).WithDescription("X-axis label (horizontal dimension)"),
			"y_axis_label": StringSchema(60).WithDescription("Y-axis label (vertical dimension)"),
			"top_left":     quadrantSchema.WithDescription("Top-left quadrant"),
			"top_right":    quadrantSchema.WithDescription("Top-right quadrant"),
			"bottom_left":  quadrantSchema.WithDescription("Bottom-left quadrant"),
			"bottom_right": quadrantSchema.WithDescription("Bottom-right quadrant"),
		},
		[]string{"x_axis_label", "y_axis_label", "top_left", "top_right", "bottom_left", "bottom_right"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values": valuesSchema,
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":      StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"header_size": NumberSchema(6, 120).WithDescription("Font size for quadrant headers in points"),
					"body_size":   NumberSchema(6, 120).WithDescription("Font size for quadrant body text in points"),
					"label_size":  NumberSchema(6, 120).WithDescription("Font size for axis labels in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": cellOverrideSchema,
	}).WithDescription("2×2 quadrant matrix with axis labels")
}

func (m *matrix2x2) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*Matrix2x2Values)
	if !ok || vals == nil {
		return fmt.Errorf("matrix-2x2: values must be *Matrix2x2Values, got %T", values)
	}

	var errs []error

	// Axis labels required
	if vals.XAxisLabel == "" {
		errs = append(errs, fmt.Errorf("matrix-2x2: x_axis_label is required"))
	} else if len(vals.XAxisLabel) > 60 {
		errs = append(errs, fmt.Errorf("matrix-2x2: x_axis_label exceeds maxLength 60 (%d chars)", len(vals.XAxisLabel)))
	}
	if vals.YAxisLabel == "" {
		errs = append(errs, fmt.Errorf("matrix-2x2: y_axis_label is required"))
	} else if len(vals.YAxisLabel) > 60 {
		errs = append(errs, fmt.Errorf("matrix-2x2: y_axis_label exceeds maxLength 60 (%d chars)", len(vals.YAxisLabel)))
	}

	// Validate each quadrant
	quads := []struct {
		name string
		q    Matrix2x2Quadrant
	}{
		{"top_left", vals.TopLeft},
		{"top_right", vals.TopRight},
		{"bottom_left", vals.BottomLeft},
		{"bottom_right", vals.BottomRight},
	}
	for _, qd := range quads {
		if qd.q.Header == "" {
			errs = append(errs, fmt.Errorf("matrix-2x2: %s.header is required", qd.name))
		} else if len(qd.q.Header) > 80 {
			errs = append(errs, fmt.Errorf("matrix-2x2: %s.header exceeds maxLength 80 (%d chars)", qd.name, len(qd.q.Header)))
		}
		if len(qd.q.Body) > 200 {
			errs = append(errs, fmt.Errorf("matrix-2x2: %s.body exceeds maxLength 200 (%d chars)", qd.name, len(qd.q.Body)))
		}
	}

	// Validate cell_overrides: indices 0-3 only
	const totalCells = 4
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= totalCells {
			errs = append(errs, fmt.Errorf("matrix-2x2: cell_overrides key %d out of range [0,%d] (hint: 0=top_left, 1=top_right, 2=bottom_left, 3=bottom_right)", idx, totalCells-1))
			continue
		}
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("matrix-2x2: cell_overrides[%d]: %w", idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("matrix-2x2: cell_overrides[%d]: %w", idx, err))
			continue
		}
		for key := range keyMap {
			if !matrix2x2CellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("matrix-2x2: cell_overrides[%d] contains unknown key %q", idx, key))
			}
		}
	}

	return errors.Join(errs...)
}

func (m *matrix2x2) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*Matrix2x2Values)
	if !ok {
		return nil, fmt.Errorf("matrix-2x2: values must be *Matrix2x2Values, got %T", values)
	}
	ovr := &Matrix2x2Overrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*Matrix2x2Overrides)
		if !ovrOk {
			return nil, fmt.Errorf("matrix-2x2: overrides must be *Matrix2x2Overrides, got %T", overrides)
		}
	}

	accent := "accent1"
	if ovr.Accent != "" {
		accent = ovr.Accent
	}
	headerSize := 16.0
	if ovr.HeaderSize > 0 {
		headerSize = ovr.HeaderSize
	}
	bodySize := 12.0
	if ovr.BodySize > 0 {
		bodySize = ovr.BodySize
	}
	labelSize := 14.0
	if ovr.LabelSize > 0 {
		labelSize = ovr.LabelSize
	}

	// Layout: 3 columns [y-axis label, left quadrants, right quadrants]
	// Row 0: [empty corner, x-axis label (col_span=2)]
	// Row 1: [y-axis label (row_span=2, rotation=270), TL quadrant, TR quadrant]
	// Row 2: [BL quadrant, BR quadrant]  (y-axis spans from row 1)

	// X-axis label cell
	xAxisCell := &jsonschema.GridCellInput{
		ColSpan: 2,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     buildMatrix2x2LabelContent(vals.XAxisLabel, labelSize, "lt1", "ctr"),
		},
	}

	// Empty corner cell
	cornerCell := &jsonschema.GridCellInput{
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(`"lt2"`),
		},
	}

	// Y-axis label cell (row_span=2, rotated 270° for vertical reading)
	yAxisCell := &jsonschema.GridCellInput{
		RowSpan: 2,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     buildMatrix2x2LabelContent(vals.YAxisLabel, labelSize, "lt1", "ctr"),
			Rotation: 270,
		},
	}

	// Quadrant cells: cell index 0=TL, 1=TR, 2=BL, 3=BR
	quadrants := []Matrix2x2Quadrant{vals.TopLeft, vals.TopRight, vals.BottomLeft, vals.BottomRight}
	quadrantCells := make([]*jsonschema.GridCellInput, 4)
	for i, q := range quadrants {
		quadrantCells[i] = &jsonschema.GridCellInput{
			Shape: &jsonschema.ShapeSpecInput{
				Geometry: "rect",
				Fill:     json.RawMessage(`"lt1"`),
				Text:     buildMatrix2x2QuadrantContent(q, headerSize, bodySize, accent),
			},
		}
		applyMatrix2x2CellOverride(quadrantCells[i], cellOverrides, i, accent)
	}

	rows := []jsonschema.GridRowInput{
		// Row 0: corner + x-axis label
		{
			Height: 12,
			Cells:  []*jsonschema.GridCellInput{cornerCell, xAxisCell},
		},
		// Row 1: y-axis label + TL + TR
		{
			Cells: []*jsonschema.GridCellInput{yAxisCell, quadrantCells[0], quadrantCells[1]},
		},
		// Row 2: BL + BR (y-axis spans from row 1, so only 2 cells)
		{
			Cells: []*jsonschema.GridCellInput{quadrantCells[2], quadrantCells[3]},
		},
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(`[12, 44, 44]`),
		Gap:     6,
		Rows:    rows,
	}

	return grid, nil
}

// buildMatrix2x2LabelContent creates a JSON text object for an axis label.
func buildMatrix2x2LabelContent(content string, size float64, color, align string) json.RawMessage {
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
			{Content: content, Size: size, Bold: true, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// buildMatrix2x2QuadrantContent creates a JSON text object for a quadrant cell
// with a bold header and optional body text.
func buildMatrix2x2QuadrantContent(q Matrix2x2Quadrant, headerSize, bodySize float64, accent string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}
	paras := []paragraph{
		{Content: q.Header, Size: headerSize, Bold: true, Color: accent, Align: "ctr"},
	}
	if q.Body != "" {
		paras = append(paras, paragraph{Content: q.Body, Size: bodySize, Color: "dk1", Align: "ctr"})
	}
	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs:    paras,
		Align:         "ctr",
		VerticalAlign: "ctr",
	}
	data, _ := json.Marshal(textObj)
	return data
}

// applyMatrix2x2CellOverride applies cell_overrides for a given quadrant cell index.
func applyMatrix2x2CellOverride(cell *jsonschema.GridCellInput, cellOverrides map[int]any, idx int, accent string) {
	co, ok := cellOverrides[idx]
	if !ok {
		return
	}
	cellOvr, coOk := co.(*Matrix2x2CellOverride)
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
