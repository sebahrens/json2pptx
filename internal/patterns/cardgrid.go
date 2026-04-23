package patterns

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/pptx"
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
func (c *cardGrid) SupportsCallout() bool        { return true }
func (c *cardGrid) SupportsInlineMarkdown() bool { return true }

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

// CardGridOverrides is the standard text overrides (accent, header_size, body_size).
type CardGridOverrides = TextOverrides

// CardGridCellOverride is an alias for the shared CellOverride struct.
type CardGridCellOverride = CellOverride

// ---------------------------------------------------------------------------
// Interface methods
// ---------------------------------------------------------------------------

func (c *cardGrid) NewValues() any      { return &CardGridValues{} }
func (c *cardGrid) NewOverrides() any   { return &CardGridOverrides{} }
func (c *cardGrid) NewCellOverride() any { return &CardGridCellOverride{} }


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
			"overrides": textOverridesSchema(),
			"cell_overrides": CellOverridesSchema("cellOverride"),
		},
		[]string{"values"},
	).AsRoot().WithDefs(map[string]*Schema{
		"cellOverride": CellOverrideDefSchema(),
	}).WithDescription("Parameterized N×M grid of titled cards")
}

func (c *cardGrid) Validate(values, overrides any, cellOverrides map[int]any) error {
	vals, ok := values.(*CardGridValues)
	if !ok || vals == nil {
		return fmt.Errorf("card-grid: values must be *CardGridValues, got %T", values)
	}

	const name = "card-grid"
	var errs []error

	// Columns range
	if vals.Columns < 1 || vals.Columns > 5 {
		errs = append(errs, errOutOfRange(name, "columns", 1, 5, vals.Columns))
	}
	// Rows range
	if vals.Rows < 1 || vals.Rows > 5 {
		errs = append(errs, errOutOfRange(name, "rows", 1, 5, vals.Rows))
	}

	// Cell count must equal columns × rows (D4: hard error, no truncation)
	expectedCells := vals.Columns * vals.Rows
	if expectedCells > 0 && len(vals.Cells) != expectedCells {
		hint := ""
		if len(vals.Cells) == 9 {
			hint = "(hint: use pattern bmc-canvas for a 9-cell Business Model Canvas)"
		}
		e := errCountMismatch(name, "cells", expectedCells, len(vals.Cells), hint)
		e.Message = fmt.Sprintf("card-grid: cells must contain exactly %d items (columns=%d × rows=%d), got %d", expectedCells, vals.Columns, vals.Rows, len(vals.Cells))
		if hint != "" {
			e.Message += " " + hint
		}
		errs = append(errs, e)
	}

	// Per-cell validation
	for i, cell := range vals.Cells {
		path := fmt.Sprintf("cells[%d].header", i)
		if cell.Header == "" {
			errs = append(errs, errRequired(name, path))
		} else if len(cell.Header) > 80 {
			errs = append(errs, errMaxLength(name, path, 80, len(cell.Header)))
		}
		bodyPath := fmt.Sprintf("cells[%d].body", i)
		if cell.Body == "" {
			errs = append(errs, errRequired(name, bodyPath))
		} else if len(cell.Body) > 300 {
			errs = append(errs, errMaxLength(name, bodyPath, 300, len(cell.Body)))
		}
	}

	// Validate cell_overrides keys (D15 whitelist)
	if coErr := validateCellOverrideKeys(name, cellOverrides, len(vals.Cells), ""); coErr != nil {
		errs = append(errs, coErr)
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

	accent := ResolveAccent(ovr.Accent, ovr.SemanticAccent, ctx.Metadata)
	headerSize := ResolveSize(ovr.HeaderSize, 16.0)
	bodySize := ResolveSize(ovr.BodySize, 12.0)

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
// Body text supports inline markdown emphasis (**bold**, *italic*) which is
// converted to <b>/<i> tags for downstream processing by SplitInlineTags.
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
			{Content: pptx.ConvertMarkdownEmphasis(body), Size: bodySize, Color: "lt1", Align: "l"},
		},
		Align:         "l",
		VerticalAlign: "t",
	}

	data, _ := json.Marshal(textObj)
	return data
}
