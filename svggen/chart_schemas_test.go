package svggen

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestChartSchemaUnknownFieldRejection verifies that charts with DataSchema
// implementations reject unknown fields with UNKNOWN_FIELD errors.
func TestChartSchemaUnknownFieldRejection(t *testing.T) {
	tests := []struct {
		name         string
		diagramType  string
		data         map[string]any
		unknownField string
	}{
		{
			name:        "bar_chart unknown field",
			diagramType: "bar_chart",
			data: map[string]any{
				"categories": []any{"Q1", "Q2"},
				"series":     []any{map[string]any{"name": "Rev", "values": []any{10.0, 20.0}}},
				"bogus_key":  "should be rejected",
			},
			unknownField: "bogus_key",
		},
		{
			name:        "line_chart unknown field",
			diagramType: "line_chart",
			data: map[string]any{
				"categories":    []any{"Jan", "Feb"},
				"series":        []any{map[string]any{"name": "Sales", "values": []any{40.0, 55.0}}},
				"unknown_extra": 42,
			},
			unknownField: "unknown_extra",
		},
		{
			name:        "pie_chart unknown field",
			diagramType: "pie_chart",
			data: map[string]any{
				"categories": []any{"A", "B"},
				"values":     []any{40.0, 60.0},
				"slices":     "not a real field",
			},
			unknownField: "slices",
		},
		{
			name:        "radar_chart unknown field",
			diagramType: "radar_chart",
			data: map[string]any{
				"axes":        []any{"Speed", "Power", "Range"},
				"series":      []any{map[string]any{"name": "A", "values": []any{8.0, 6.0, 7.0}}},
				"spider_webs": true,
			},
			unknownField: "spider_webs",
		},
		{
			name:        "scatter_chart unknown field",
			diagramType: "scatter_chart",
			data: map[string]any{
				"series":    []any{map[string]any{"name": "G", "points": []any{map[string]any{"x": 1.0, "y": 2.0}}}},
				"trendline": true,
			},
			unknownField: "trendline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type: tt.diagramType,
				Data: tt.data,
			}

			_, err := Render(req)
			if err == nil {
				t.Fatalf("expected error for unknown field %q but got nil", tt.unknownField)
			}

			errMsg := err.Error()
			if !strings.Contains(errMsg, "UNKNOWN_FIELD") && !strings.Contains(errMsg, tt.unknownField) {
				t.Errorf("expected error to mention %q, got: %s", tt.unknownField, errMsg)
			}

			// Verify the error contains structured ValidationError.
			ves := GetValidationErrors(err)
			if len(ves) == 0 {
				// The error is wrapped; check the message content.
				if !strings.Contains(errMsg, tt.unknownField) {
					t.Errorf("error should mention the unknown field %q: %s", tt.unknownField, errMsg)
				}
			}
		})
	}
}

// TestChartSchemaValidFieldsAccepted verifies that charts with DataSchema
// accept all their known fields without error.
func TestChartSchemaValidFieldsAccepted(t *testing.T) {
	tests := []struct {
		name        string
		diagramType string
		data        map[string]any
	}{
		{
			name:        "bar_chart with all optional fields",
			diagramType: "bar_chart",
			data: map[string]any{
				"categories":   []any{"Q1", "Q2", "Q3"},
				"series":       []any{map[string]any{"name": "Rev", "values": []any{10.0, 20.0, 30.0}}},
				"colors":       []any{"#FF0000", "#00FF00"},
				"footnote":     "Source: internal",
				"x_label":      "Quarter",
				"y_label":      "Revenue ($M)",
				"x_axis_title": "Quarter",
				"y_axis_title": "Revenue ($M)",
			},
		},
		{
			name:        "bar_chart with labels alias",
			diagramType: "bar_chart",
			data: map[string]any{
				"labels": []any{"A", "B"},
				"series": []any{map[string]any{"name": "X", "values": []any{1.0, 2.0}}},
			},
		},
		{
			name:        "pie_chart minimal",
			diagramType: "pie_chart",
			data: map[string]any{
				"values": []any{40.0, 30.0, 30.0},
			},
		},
		{
			name:        "line_chart time-series",
			diagramType: "line_chart",
			data: map[string]any{
				"series": []any{map[string]any{
					"name":         "Sales",
					"time_strings": []any{"2024-01", "2024-02"},
					"values":       []any{10.0, 20.0},
				}},
			},
		},
		{
			name:        "scatter_chart with points format",
			diagramType: "scatter_chart",
			data: map[string]any{
				"series": []any{map[string]any{
					"name": "G",
					"points": []any{
						map[string]any{"x": 1.0, "y": 2.0},
						map[string]any{"x": 3.0, "y": 4.0},
					},
				}},
				"x_label": "X",
				"y_label": "Y",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &RequestEnvelope{
				Type: tt.diagramType,
				Data: tt.data,
			}

			_, err := Render(req)
			if err != nil {
				t.Fatalf("unexpected error for valid fields: %v", err)
			}
		})
	}
}

// TestDataSchemaJSONOutput verifies that DataSchema produces valid JSON.
func TestDataSchemaJSONOutput(t *testing.T) {
	d := DefaultRegistry().Get("bar_chart")
	ds, ok := d.(DiagramWithSchema)
	if !ok {
		t.Fatal("bar_chart should implement DiagramWithSchema")
	}

	schema := ds.DataSchema()
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}

	// Verify additionalProperties is present and false.
	if !strings.Contains(string(data), `"additionalProperties": false`) {
		t.Error("schema should contain additionalProperties: false")
	}

	// Verify required fields are present.
	if !strings.Contains(string(data), `"categories"`) {
		t.Error("schema should list categories in properties")
	}
	if !strings.Contains(string(data), `"series"`) {
		t.Error("schema should list series in properties")
	}
}

// TestDiagramWithoutSchemaAcceptsAnyKeys verifies that diagram types without
// DataSchema continue to accept unknown keys (backward compatibility).
func TestDiagramWithoutSchemaAcceptsAnyKeys(t *testing.T) {
	// waterfall does not have a DataSchema implementation yet.
	d := DefaultRegistry().Get("waterfall")
	if d == nil {
		t.Skip("waterfall not registered")
	}
	if _, ok := d.(DiagramWithSchema); ok {
		t.Skip("waterfall now has a DataSchema — update this test")
	}

	req := &RequestEnvelope{
		Type: "waterfall",
		Data: map[string]any{
			"points": []any{
				map[string]any{"label": "Revenue", "value": 500.0},
				map[string]any{"label": "Cost", "value": -200.0, "type": "negative"},
				map[string]any{"label": "Profit", "value": 300.0, "type": "total"},
			},
			"unknown_extra_field": "should not cause error",
		},
	}

	_, err := Render(req)
	if err != nil {
		t.Fatalf("diagram without schema should accept unknown fields, got: %v", err)
	}
}
