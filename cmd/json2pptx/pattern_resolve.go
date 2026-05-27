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
	Bounds        *jsonschema.GridBoundsInput    `json:"bounds,omitempty"`
	MaxHeightPct  float64                        `json:"max_height_pct,omitempty"`
}

// expandPattern looks up the named pattern in the registry, unmarshals the
// typed Values/Overrides/CellOverrides, validates, and expands to a
// ShapeGridInput. Returns the expanded grid, any warnings, and an error.
func expandPattern(p *PatternInput, ctx patterns.ExpandContext, reg *patterns.Registry) (*jsonschema.ShapeGridInput, []string, error) {
	pat, ok := reg.Get(p.Name)
	if !ok {
		msg := fmt.Sprintf("unknown pattern %q", p.Name)
		if suggestion, ok := reg.Suggest(p.Name); ok {
			msg += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		return nil, nil, fmt.Errorf("%s", msg)
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

	// Pre-expand callout support check (D18): fail before Expand if pattern
	// does not support callout — this keeps validate and expand parity (0kyd).
	if p.Callout != nil {
		cs, ok := pat.(patterns.CalloutSupport)
		if !ok || !cs.SupportsCallout() {
			return nil, nil, patterns.ErrCalloutUnsupportedFor(p.Name, reg.CalloutSupportedPatterns())
		}
	}

	// Expand
	grid, err := pat.Expand(ctx, values, overrides, cellOverrides)
	if err != nil {
		return nil, nil, fmt.Errorf("pattern %q: expand failed: %w", p.Name, err)
	}

	// Apply bounds_override: explicit bounds or max_height_pct convenience alias.
	// This constrains the grid to a sub-region of the layout area, which also
	// corrects density math (cell_budgets uses grid.Bounds when present).
	if b := resolvePatternBounds(p); b != nil {
		grid.Bounds = b
	}

	// Post-expand callout decorator (D18): append full-width callout row
	if p.Callout != nil {
		grid = appendCalloutRow(grid, p.Callout)
	}

	// Optional PostExpandWarner interface: patterns can surface structured
	// warning strings (e.g. CHART_PLACEHOLDER_EMPTY) describing known-degraded
	// states after a successful expansion. Downstream fit-report consumers
	// parse the leading "<CODE>: " prefix into FitFindings.
	var warnings []string
	if warner, ok := pat.(patterns.PostExpandWarner); ok {
		warnings = warner.PostExpandWarnings(ctx, values, overrides)
	}

	slog.Info("pattern expanded",
		slog.String("pattern", p.Name),
		slog.Int("version", pat.Version()),
	)

	return grid, warnings, nil
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
			Fill:     jsonStringRaw(accent),
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

// expandNestedCellPatterns walks every cell of grid (and any nested grids)
// and expands a cell-level Pattern into the cell's Grid field. After this
// pass, no cell still has a Pattern set; each formerly-pattern cell is
// represented as a Grid (a ShapeGridInput produced by pattern expansion).
//
// Mutual exclusion: a cell may not set both Pattern and Grid; that is rejected
// as a validation error. A cell that sets Pattern alongside Shape/Table/Icon/
// Image/Diagram/Composite is also rejected — a nested pattern occupies the
// whole cell rectangle and is incompatible with sibling content.
func expandNestedCellPatterns(grid *jsonschema.ShapeGridInput, ctx patterns.ExpandContext, reg *patterns.Registry) error {
	if grid == nil {
		return nil
	}
	for ri := range grid.Rows {
		for ci := range grid.Rows[ri].Cells {
			cell := grid.Rows[ri].Cells[ci]
			if cell == nil {
				continue
			}
			if len(cell.Pattern) > 0 {
				if cell.Grid != nil {
					return fmt.Errorf("grid cell row %d col %d: 'pattern' and 'grid' are mutually exclusive", ri, ci)
				}
				if cell.Shape != nil || cell.Table != nil || cell.Icon != nil ||
					cell.Image != nil || cell.Diagram != nil || cell.Composite != nil {
					return fmt.Errorf("grid cell row %d col %d: nested 'pattern' is incompatible with sibling cell content (shape/table/icon/image/diagram/composite)", ri, ci)
				}
				var pi PatternInput
				if err := json.Unmarshal(cell.Pattern, &pi); err != nil {
					return fmt.Errorf("grid cell row %d col %d: invalid pattern: %w", ri, ci, err)
				}
				expanded, _, err := expandPattern(&pi, ctx, reg)
				if err != nil {
					return fmt.Errorf("grid cell row %d col %d: %w", ri, ci, err)
				}
				cell.Pattern = nil
				cell.Grid = expanded
			}
			// Recurse into nested grids (covers multi-level nesting).
			if cell.Grid != nil {
				if err := expandNestedCellPatterns(cell.Grid, ctx, reg); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// resolvePatternBounds returns a GridBoundsInput from PatternInput's bounds
// fields, applying the max_height_pct convenience alias. Returns nil if no
// bounds override was specified.
func resolvePatternBounds(p *PatternInput) *jsonschema.GridBoundsInput {
	if p.Bounds != nil {
		// Explicit bounds take priority — use as-is.
		return p.Bounds
	}
	if p.MaxHeightPct > 0 && p.MaxHeightPct < 100 {
		// Convenience alias: constrain height while preserving full width.
		// X/Y/Width default to the layout content area (0,0 = top-left of layout).
		return &jsonschema.GridBoundsInput{
			X:      0,
			Y:      0,
			Width:  100,
			Height: p.MaxHeightPct,
		}
	}
	return nil
}
