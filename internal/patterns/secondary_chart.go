package patterns

import (
	"fmt"
	"strconv"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
	"github.com/sebahrens/json2pptx/internal/types"
)

// ---------------------------------------------------------------------------
// SecondaryChart — small embedded chart slot for card-grid / icon-row cells
// ---------------------------------------------------------------------------

// SecondaryChart describes an optional small chart embedded inside a card-grid
// or icon-row cell. Exactly one secondary may be attached per cell (enforced by
// being a single field, not an array). Chart type is restricted to a small set
// suitable for in-card rendering.
//
// Conceptual types:
//   - sparkline:  a small line chart with no legend or chrome; renders as a
//     line_chart sub-diagram with hidden legend.
//   - bar_chart:  a small bar chart with bars per category.
//   - line_chart: a regular line chart with one series per cell.
type SecondaryChart struct {
	// Type is the chart kind. Must be one of "sparkline", "bar_chart",
	// "line_chart".
	Type string `json:"type"`
	// Values are the data points for the single embedded series. 2–12 points.
	Values []float64 `json:"values"`
	// Categories are optional x-axis labels. When omitted, numeric indices are
	// synthesized for chart types that require them.
	Categories []string `json:"categories,omitempty"`
	// Color is an optional hex/scheme accent color override. When empty, the
	// cell accent is used.
	Color string `json:"color,omitempty"`
}

// validSecondaryChartTypes enumerates the allowed Type values.
var validSecondaryChartTypes = map[string]bool{
	"sparkline":  true,
	"bar_chart":  true,
	"line_chart": true,
}

// SecondaryChartSchema returns the JSON schema for a single SecondaryChart.
// Caps: type restricted to {sparkline, bar_chart, line_chart}; values 2–12
// numbers; categories length capped at 12 and must match values length when
// non-empty (enforced at Validate time).
func SecondaryChartSchema() *Schema {
	return ObjectSchema(
		map[string]*Schema{
			"type":       EnumSchema("sparkline", "bar_chart", "line_chart").WithDescription("Chart kind: sparkline (mini line, no legend), bar_chart (bars), line_chart (line with axes)"),
			"values":     ArraySchema(NumberSchema(0, 0), 2, 12).WithDescription("2–12 numeric data points for the single embedded series"),
			"categories": ArraySchema(StringSchema(40), 0, 12).WithDescription("Optional x-axis labels; length must equal values length when set"),
			"color":      StringSchema(0).WithDescription("Optional hex or scheme color (defaults to the cell accent)"),
		},
		[]string{"type", "values"},
	).WithAdditionalProperties(false).WithDescription("Optional embedded chart rendered below the cell text (one per cell, restricted chart types)")
}

// validateSecondaryChart returns errors describing any problems with the given
// secondary payload. pattern and pathPrefix are used to build user-facing error
// paths like "cells[2].secondary.values".
func validateSecondaryChart(pattern, pathPrefix string, sec *SecondaryChart) []error {
	if sec == nil {
		return nil
	}
	var errs []error
	typePath := pathPrefix + ".type"
	if sec.Type == "" {
		errs = append(errs, errRequired(pattern, typePath))
	} else if !validSecondaryChartTypes[sec.Type] {
		errs = append(errs, &ValidationError{
			Pattern: pattern,
			Path:    typePath,
			Code:    "invalid_enum",
			Message: fmt.Sprintf("%s: %s must be one of sparkline, bar_chart, line_chart; got %q", pattern, typePath, sec.Type),
		})
	}
	valuesPath := pathPrefix + ".values"
	if len(sec.Values) < 2 {
		errs = append(errs, errMinItems(pattern, valuesPath, 2, len(sec.Values), ""))
	} else if len(sec.Values) > 12 {
		errs = append(errs, errMaxItems(pattern, valuesPath, 12, len(sec.Values), ""))
	}
	if len(sec.Categories) > 0 && len(sec.Categories) != len(sec.Values) {
		errs = append(errs, &ValidationError{
			Pattern: pattern,
			Path:    pathPrefix + ".categories",
			Code:    "length_mismatch",
			Message: fmt.Sprintf("%s: %s.categories length (%d) must equal values length (%d) when set",
				pattern, pathPrefix, len(sec.Categories), len(sec.Values)),
		})
	}
	return errs
}

// buildSecondaryDiagram converts a SecondaryChart into a DiagramSpec suitable
// for embedding as the sub-diagram of a composite cell. "sparkline" is mapped
// to a line_chart with hidden legend; bar_chart and line_chart pass through.
// fallbackAccent is used when sec.Color is empty.
func buildSecondaryDiagram(sec *SecondaryChart, fallbackAccent string) *types.DiagramSpec {
	if sec == nil {
		return nil
	}
	diagramType := sec.Type
	if diagramType == "sparkline" {
		diagramType = "line_chart"
	}

	// Synthesize numeric categories when omitted; the bar_chart and line_chart
	// validators require non-empty categories.
	cats := sec.Categories
	if len(cats) == 0 {
		cats = make([]string, len(sec.Values))
		for i := range sec.Values {
			cats[i] = strconv.Itoa(i + 1)
		}
	}
	catsAny := make([]any, len(cats))
	for i, c := range cats {
		catsAny[i] = c
	}
	valuesAny := make([]any, len(sec.Values))
	for i, v := range sec.Values {
		valuesAny[i] = v
	}

	data := map[string]any{
		"categories": catsAny,
		"series": []any{
			map[string]any{
				"name":   "",
				"values": valuesAny,
			},
		},
	}

	var style *types.DiagramStyle
	color := sec.Color
	if color == "" {
		color = fallbackAccent
	}
	if color != "" {
		style = &types.DiagramStyle{Colors: []string{color}}
	}
	// Sparkline aesthetic: hide the legend (single series, no label).
	if sec.Type == "sparkline" {
		if style == nil {
			style = &types.DiagramStyle{}
		}
		style.ShowLegend = false
	}

	return &types.DiagramSpec{
		Type:  diagramType,
		Data:  data,
		Style: style,
	}
}

// wrapCellWithSecondary takes a base cell (with Shape set) and converts it
// into a Composite cell whose Text is the original shape and whose SubDiagram
// is the rendered SecondaryChart. The original AccentBar, ColSpan, RowSpan,
// Fit, and NamedStyle are preserved. The base cell's Shape is moved into the
// Composite.Text slot; the Icon overlay (if any) stays attached to the shape.
//
// If sec is nil or base.Shape is nil, the cell is returned unchanged.
func wrapCellWithSecondary(base *jsonschema.GridCellInput, sec *SecondaryChart, fallbackAccent string) *jsonschema.GridCellInput {
	if base == nil || sec == nil || base.Shape == nil {
		return base
	}
	composite := &jsonschema.CompositeInput{
		Text:       base.Shape,
		SubDiagram: buildSecondaryDiagram(sec, fallbackAccent),
		Split:      "top",
		Ratio:      0.6,
	}
	base.Shape = nil
	base.Composite = composite
	return base
}

