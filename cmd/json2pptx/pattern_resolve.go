package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// PatternInput is the JSON schema for pattern-based slides.
// Placed at the same level as shape_grid in SlideInput (XOR — D1).
type PatternInput struct {
	Name          string                        `json:"name"`
	Values        json.RawMessage               `json:"values"`
	Overrides     json.RawMessage               `json:"overrides,omitempty"`
	CellOverrides map[string]json.RawMessage     `json:"cell_overrides,omitempty"`
	Callout       *patterns.PatternCallout       `json:"callout,omitempty"`
}

// expandPattern looks up the named pattern in the registry, unmarshals the
// typed Values/Overrides/CellOverrides, validates, and expands to a
// ShapeGridInput. Returns the expanded grid, any warnings, and an error.
func expandPattern(p *PatternInput, ctx patterns.ExpandContext, reg *patterns.Registry) (*jsonschema.ShapeGridInput, []string, error) {
	pat, ok := reg.Get(p.Name)
	if !ok {
		return nil, nil, fmt.Errorf("unknown pattern %q", p.Name)
	}

	// Unmarshal values
	values := pat.NewValues()
	if err := json.Unmarshal(p.Values, values); err != nil {
		return nil, nil, fmt.Errorf("pattern %q: invalid values: %w", p.Name, err)
	}

	// Unmarshal overrides
	var overrides any
	if len(p.Overrides) > 0 {
		overrides = pat.NewOverrides()
		if overrides != nil {
			if err := json.Unmarshal(p.Overrides, overrides); err != nil {
				return nil, nil, fmt.Errorf("pattern %q: invalid overrides: %w", p.Name, err)
			}
		}
	}

	// Unmarshal cell_overrides: string keys → int keys
	var cellOverrides map[int]any
	if len(p.CellOverrides) > 0 {
		cellOverrides = make(map[int]any, len(p.CellOverrides))
		for key, raw := range p.CellOverrides {
			idx, err := strconv.Atoi(key)
			if err != nil {
				return nil, nil, fmt.Errorf("pattern %q: cell_overrides key %q is not an integer", p.Name, key)
			}
			co := pat.NewCellOverride()
			if co == nil {
				return nil, nil, fmt.Errorf("pattern %q: does not support cell_overrides", p.Name)
			}
			if err := json.Unmarshal(raw, co); err != nil {
				return nil, nil, fmt.Errorf("pattern %q: invalid cell_overrides[%d]: %w", p.Name, idx, err)
			}
			cellOverrides[idx] = co
		}
	}

	// Validate
	if err := pat.Validate(values, overrides, cellOverrides); err != nil {
		return nil, nil, fmt.Errorf("pattern %q: validation failed: %w", p.Name, err)
	}

	// Expand
	grid, err := pat.Expand(ctx, values, overrides, cellOverrides)
	if err != nil {
		return nil, nil, fmt.Errorf("pattern %q: expand failed: %w", p.Name, err)
	}

	// Post-expand callout decorator (D18): append full-width callout row
	if p.Callout != nil {
		cs, ok := pat.(patterns.CalloutSupport)
		if !ok || !cs.SupportsCallout() {
			return nil, nil, fmt.Errorf("pattern %q does not support callout", p.Name)
		}
		grid = appendCalloutRow(grid, p.Callout)
	}

	slog.Info("pattern expanded",
		slog.String("pattern", p.Name),
		slog.Int("version", pat.Version()),
	)

	return grid, nil, nil
}

// appendCalloutRow appends a full-width callout row to the expanded grid.
// The callout spans all columns and uses AutoHeight for text-driven sizing.
// Callout cells are NOT addressable via cell_overrides (D18).
func appendCalloutRow(grid *jsonschema.ShapeGridInput, callout *patterns.PatternCallout) *jsonschema.ShapeGridInput {
	// Determine column count from the grid
	numCols := 1
	if len(grid.Rows) > 0 {
		numCols = len(grid.Rows[0].Cells)
	}

	accent := "accent1"
	if callout.Accent != "" {
		accent = callout.Accent
	}

	bold := callout.Emphasis == "bold" || callout.Emphasis == "bold-italic"
	italic := callout.Emphasis == "italic" || callout.Emphasis == "bold-italic"

	textContent := buildCalloutTextContent(callout.Text, 14.0, bold, italic, "lt1", "ctr")

	calloutCell := &jsonschema.GridCellInput{
		ColSpan: numCols,
		Shape: &jsonschema.ShapeSpecInput{
			Geometry: "rect",
			Fill:     json.RawMessage(fmt.Sprintf(`"%s"`, accent)),
			Text:     textContent,
		},
	}

	calloutRow := jsonschema.GridRowInput{
		AutoHeight: true,
		Cells:      []*jsonschema.GridCellInput{calloutCell},
	}

	grid.Rows = append(grid.Rows, calloutRow)
	return grid
}

// buildCalloutTextContent creates a JSON text object for a callout cell.
func buildCalloutTextContent(content string, size float64, bold, italic bool, color, align string) json.RawMessage {
	type paragraph struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Bold    bool    `json:"bold,omitempty"`
		Italic  bool    `json:"italic,omitempty"`
		Color   string  `json:"color,omitempty"`
		Align   string  `json:"align,omitempty"`
	}

	textObj := struct {
		Paragraphs    []paragraph `json:"paragraphs"`
		Align         string      `json:"align"`
		VerticalAlign string      `json:"vertical_align"`
	}{
		Paragraphs: []paragraph{
			{Content: content, Size: size, Bold: bold, Italic: italic, Color: color, Align: align},
		},
		Align:         align,
		VerticalAlign: "ctr",
	}

	data, _ := json.Marshal(textObj)
	return data
}
