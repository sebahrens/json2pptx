package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ---------------------------------------------------------------------------
// card-grid pattern — parameterized N×M titled cards (D5)
// ---------------------------------------------------------------------------

func init() {
	Default().Register(&cardGrid{})
}

type cardGrid struct{}

func (c *cardGrid) Name() string           { return "card-grid" }
func (c *cardGrid) Description() string    { return "Parameterized N×M grid of titled cards" }
func (c *cardGrid) UseWhen() string        { return "N×M titled cards" }
func (c *cardGrid) Version() int           { return 1 }
func (c *cardGrid) CellsHint() string      { return "rows × cols" }
func (c *cardGrid) SupportsCallout() bool  { return true }

func (c *cardGrid) ExemplarValues() any {
	return &CardGridValues{
		Columns: 3,
		Rows:    2,
		Cells: []CardGridCell{
			{Header: "Card 1", Body: "Description 1"},
			{Header: "Card 2", Body: "Description 2"},
			{Header: "Card 3", Body: "Description 3"},
			{Header: "Card 4", Body: "Description 4"},
			{Header: "Card 5", Body: "Description 5"},
			{Header: "Card 6", Body: "Description 6"},
		},
	}
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// CardGridCell is a single card with a header and body.
// Supports string shorthand: "Header | Body" unmarshals to {header:"Header", body:"Body"}.
type CardGridCell struct {
	Header string `json:"header"`
	Body   string `json:"body"`
}

// UnmarshalJSON supports string shorthand "Header | Body" or object {header, body}.
func (c *CardGridCell) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		parts := strings.SplitN(s, " | ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("CardGridCell string must be \"Header | Body\", got %q", s)
		}
		c.Header = parts[0]
		c.Body = parts[1]
		return nil
	}
	type alias CardGridCell
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("CardGridCell must be string \"Header | Body\" or {header, body}: %w", err)
	}
	*c = CardGridCell(a)
	return nil
}

// CardGridValues is the values type for card-grid.
type CardGridValues struct {
	Columns int            `json:"columns"`
	Rows    int            `json:"rows"`
	Cells   []CardGridCell `json:"cells"`
}

// CardGridOverrides contains pattern-level overrides for card-grid.
type CardGridOverrides struct {
	Accent     string  `json:"accent,omitempty"`
	HeaderSize float64 `json:"header_size,omitempty"`
	BodySize   float64 `json:"body_size,omitempty"`
}

// CardGridCellOverride contains per-cell overrides for card-grid.
type CardGridCellOverride struct {
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

func (c *cardGrid) NewValues() any      { return &CardGridValues{} }
func (c *cardGrid) NewOverrides() any   { return &CardGridOverrides{} }
func (c *cardGrid) NewCellOverride() any { return &CardGridCellOverride{} }

// cardGridCellOverrideAllowed is the whitelist of per-cell override keys (D15).
var cardGridCellOverrideAllowed = map[string]bool{
	"accent_bar":     true,
	"emphasis":       true,
	"align":          true,
	"vertical_align": true,
	"font_size":      true,
	"color":          true,
}

func (c *cardGrid) Schema() *Schema {
	cellSchema := OneOfSchema(
		StringSchema(0).WithDescription("Shorthand: \"Header | Body\""),
		ObjectSchema(
			map[string]*Schema{
				"header": StringSchema(80).WithDescription("Card header/title"),
				"body":   StringSchema(300).WithDescription("Card body content"),
			},
			[]string{"header", "body"},
		).WithAdditionalProperties(false),
	).WithDescription("Card cell: string \"Header | Body\" or {header, body}")

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

	// Max cells: 5 cols * 5 rows = 25
	cellOverridesProps := make(map[string]*Schema)
	for i := 0; i < 25; i++ {
		cellOverridesProps[fmt.Sprintf("%d", i)] = cellOverrideSchema
	}

	valuesSchema := ObjectSchema(
		map[string]*Schema{
			"columns": IntegerSchema(1, 5).WithDescription("Number of columns (1–5)"),
			"rows":    IntegerSchema(1, 5).WithDescription("Number of rows (1–5)"),
			"cells":   ArraySchema(cellSchema, 1, 25).WithDescription("Cards in row-major order (length must equal columns × rows)"),
		},
		[]string{"columns", "rows", "cells"},
	).WithAdditionalProperties(false)

	return ObjectSchema(
		map[string]*Schema{
			"values": valuesSchema,
			"overrides": ObjectSchema(
				map[string]*Schema{
					"accent":      StringSchema(0).WithDescription("Accent scheme color (default accent1)").WithDefault("accent1"),
					"header_size": NumberSchema(6, 120).WithDescription("Font size for card headers in points"),
					"body_size":   NumberSchema(6, 120).WithDescription("Font size for card body in points"),
				},
				nil,
			).WithAdditionalProperties(false),
			"cell_overrides": ObjectSchema(
				cellOverridesProps,
				nil,
			).WithAdditionalProperties(false),
		},
		[]string{"values"},
	).AsRoot().WithDescription("Parameterized N×M grid of titled cards")
}

func (c *cardGrid) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*CardGridValues)
	if !ok || vals == nil {
		return fmt.Errorf("card-grid: values must be *CardGridValues, got %T", values)
	}

	var errs []error

	// Columns range
	if vals.Columns < 1 || vals.Columns > 5 {
		errs = append(errs, fmt.Errorf("card-grid: columns must be 1–5, got %d", vals.Columns))
	}
	// Rows range
	if vals.Rows < 1 || vals.Rows > 5 {
		errs = append(errs, fmt.Errorf("card-grid: rows must be 1–5, got %d", vals.Rows))
	}

	// Cell count must equal columns × rows (D4: hard error, no truncation)
	expectedCells := vals.Columns * vals.Rows
	if expectedCells > 0 && len(vals.Cells) != expectedCells {
		hint := ""
		if len(vals.Cells) == 9 {
			hint = " (hint: use pattern bmc-canvas for a 9-cell Business Model Canvas)"
		}
		errs = append(errs, fmt.Errorf("card-grid: cells must contain exactly %d items (columns=%d × rows=%d), got %d%s",
			expectedCells, vals.Columns, vals.Rows, len(vals.Cells), hint))
	}

	// Per-cell validation
	for i, cell := range vals.Cells {
		if cell.Header == "" {
			errs = append(errs, fmt.Errorf("card-grid: cells[%d].header is required", i))
		} else if len(cell.Header) > 80 {
			errs = append(errs, fmt.Errorf("card-grid: cells[%d].header exceeds maxLength 80 (%d chars)", i, len(cell.Header)))
		}
		if cell.Body == "" {
			errs = append(errs, fmt.Errorf("card-grid: cells[%d].body is required", i))
		} else if len(cell.Body) > 300 {
			errs = append(errs, fmt.Errorf("card-grid: cells[%d].body exceeds maxLength 300 (%d chars)", i, len(cell.Body)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	totalCells := len(vals.Cells)
	for idx, co := range cellOverrides {
		if idx < 0 || idx >= totalCells {
			errs = append(errs, fmt.Errorf("card-grid: cell_overrides key %d out of range [0,%d]", idx, totalCells-1))
			continue
		}
		raw, err := json.Marshal(co)
		if err != nil {
			errs = append(errs, fmt.Errorf("card-grid: cell_overrides[%d]: %w", idx, err))
			continue
		}
		var keyMap map[string]json.RawMessage
		if err := json.Unmarshal(raw, &keyMap); err != nil {
			errs = append(errs, fmt.Errorf("card-grid: cell_overrides[%d]: %w", idx, err))
			continue
		}
		for key := range keyMap {
			if !cardGridCellOverrideAllowed[key] {
				errs = append(errs, fmt.Errorf("card-grid: cell_overrides[%d] contains unknown key %q", idx, key))
			}
		}
	}

	return errors.Join(errs...)
}

func (c *cardGrid) Expand(ctx ExpandContext, values, overrides any, cellOverrides map[int]any) (*jsonschema.ShapeGridInput, error) {
	vals, ok := values.(*CardGridValues)
	if !ok {
		return nil, fmt.Errorf("card-grid: values must be *CardGridValues, got %T", values)
	}
	ovr := &CardGridOverrides{}
	if overrides != nil {
		var ovrOk bool
		ovr, ovrOk = overrides.(*CardGridOverrides)
		if !ovrOk {
			return nil, fmt.Errorf("card-grid: overrides must be *CardGridOverrides, got %T", overrides)
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

	var rows []jsonschema.GridRowInput
	cellIdx := 0

	for r := 0; r < vals.Rows; r++ {
		gridCells := make([]*jsonschema.GridCellInput, vals.Columns)
		for col := 0; col < vals.Columns; col++ {
			cell := vals.Cells[cellIdx]
			textContent := buildCardGridTextContent(cell.Header, headerSize, cell.Body, bodySize)

			gc := &jsonschema.GridCellInput{
				Shape: &jsonschema.ShapeSpecInput{
					Geometry: "roundRect",
					Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
					Text:     textContent,
				},
			}

			// Apply cell overrides
			if co, ok := cellOverrides[cellIdx]; ok {
				cellOvr, coOk := co.(*CardGridCellOverride)
				if coOk && cellOvr.AccentBar {
					gc.AccentBar = &jsonschema.AccentBarInput{
						Position: "top",
						Color:    accent,
						Width:    4,
					}
				}
			}

			gridCells[col] = gc
			cellIdx++
		}
		rows = append(rows, jsonschema.GridRowInput{
			Cells: gridCells,
		})
	}

	grid := &jsonschema.ShapeGridInput{
		Columns: json.RawMessage(fmt.Sprintf(`%d`, vals.Columns)),
		Gap:     10,
		Rows:    rows,
	}

	return grid, nil
}

// buildCardGridTextContent creates a JSON text object with header + body paragraphs.
func buildCardGridTextContent(header string, headerSize float64, body string, bodySize float64) json.RawMessage {
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
			{Content: header, Size: headerSize, Bold: true, Color: "lt1", Align: "l"},
			{Content: body, Size: bodySize, Color: "lt1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "t",
	}

	data, _ := json.Marshal(textObj)
	return data
}
