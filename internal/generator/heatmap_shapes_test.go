package generator

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestIsHeatmapDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "heatmap type",
			spec:     &types.DiagramSpec{Type: "heatmap"},
			expected: true,
		},
		{
			name:     "swot type",
			spec:     &types.DiagramSpec{Type: "swot"},
			expected: false,
		},
		{
			name:     "panel_layout type",
			spec:     &types.DiagramSpec{Type: "panel_layout"},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHeatmapDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isHeatmapDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseHeatmapData(t *testing.T) {
	tests := []struct {
		name       string
		data       map[string]any
		wantErr    bool
		wantRows   int
		wantCols   int
		wantMin    float64
		wantMax    float64
	}{
		{
			name: "basic values",
			data: map[string]any{
				"row_labels": []any{"Alice", "Bob"},
				"col_labels": []any{"Go", "Python"},
				"values":     []any{[]any{float64(9), float64(7)}, []any{float64(5), float64(8)}},
			},
			wantRows: 2,
			wantCols: 2,
			wantMin:  5,
			wantMax:  9,
		},
		{
			name: "y_labels and x_labels aliases",
			data: map[string]any{
				"y_labels": []any{"R1"},
				"x_labels": []any{"C1", "C2"},
				"values":   []any{[]any{float64(1), float64(2)}},
			},
			wantRows: 1,
			wantCols: 2,
			wantMin:  1,
			wantMax:  2,
		},
		{
			name: "rows alias for values",
			data: map[string]any{
				"rows": []any{[]any{float64(3), float64(4)}},
			},
			wantRows: 1,
			wantCols: 2,
			wantMin:  3,
			wantMax:  4,
		},
		{
			name: "diverging color scale",
			data: map[string]any{
				"values":      []any{[]any{float64(-5), float64(0), float64(5)}},
				"color_scale": "diverging",
			},
			wantRows: 1,
			wantCols: 3,
			wantMin:  -5,
			wantMax:  5,
		},
		{
			name:    "missing values",
			data:    map[string]any{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseHeatmapData(tt.data)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(parsed.values) != tt.wantRows {
				t.Errorf("got %d rows, want %d", len(parsed.values), tt.wantRows)
			}
			if len(parsed.values) > 0 && len(parsed.values[0]) != tt.wantCols {
				t.Errorf("got %d cols, want %d", len(parsed.values[0]), tt.wantCols)
			}
			if parsed.minVal != tt.wantMin {
				t.Errorf("minVal = %g, want %g", parsed.minVal, tt.wantMin)
			}
			if parsed.maxVal != tt.wantMax {
				t.Errorf("maxVal = %g, want %g", parsed.maxVal, tt.wantMax)
			}
		})
	}
}

func TestEncodeDecodeHeatmapMeta(t *testing.T) {
	parsed := heatmapParsedData{
		rowLabels:  []string{"Alice", "Bob"},
		colLabels:  []string{"Go", "Python", "SQL"},
		minVal:     1.5,
		maxVal:     9,
		colorScale: "sequential",
	}

	encoded := encodeHeatmapMeta(parsed)
	minVal, maxVal, colorScale, rowLabels, colLabels := decodeHeatmapMeta(encoded)

	if minVal != 1.5 {
		t.Errorf("minVal = %g, want 1.5", minVal)
	}
	if maxVal != 9 {
		t.Errorf("maxVal = %g, want 9", maxVal)
	}
	if colorScale != "sequential" {
		t.Errorf("colorScale = %q, want %q", colorScale, "sequential")
	}
	if len(rowLabels) != 2 || rowLabels[0] != "Alice" || rowLabels[1] != "Bob" {
		t.Errorf("rowLabels = %v, want [Alice Bob]", rowLabels)
	}
	if len(colLabels) != 3 || colLabels[0] != "Go" || colLabels[2] != "SQL" {
		t.Errorf("colLabels = %v, want [Go Python SQL]", colLabels)
	}
}

func TestFormatHeatmapVal(t *testing.T) {
	tests := []struct {
		val      float64
		expected string
	}{
		{9, "9"},
		{0, "0"},
		{3.14, "3.14"},
		{-1, "-1"},
		{100, "100"},
	}
	for _, tt := range tests {
		got := formatHeatmapVal(tt.val)
		if got != tt.expected {
			t.Errorf("formatHeatmapVal(%g) = %q, want %q", tt.val, got, tt.expected)
		}
	}
}

func TestGenerateHeatmapGroupXML_Basic(t *testing.T) {
	// Build panels: 1 meta + 4 cells (2x2)
	parsed := heatmapParsedData{
		rowLabels:  []string{"Alice", "Bob"},
		colLabels:  []string{"Go", "Python"},
		minVal:     5,
		maxVal:     9,
		colorScale: "sequential",
		values:     [][]float64{{9, 7}, {5, 8}},
	}
	panels := []nativePanelData{
		{title: "__heatmap_meta__", body: encodeHeatmapMeta(parsed)},
		{title: "9"},
		{title: "7"},
		{title: "5"},
		{title: "8"},
	}

	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 5000000}
	meta := heatmapMeta{numRows: 2, numCols: 2, colorScale: "sequential"}

	result := generateHeatmapGroupXML(panels, bounds, 100, meta)

	if result == "" {
		t.Fatal("generateHeatmapGroupXML returned empty string")
	}

	// Should be well-formed XML
	var parsed2 interface{}
	if err := xml.Unmarshal([]byte(result), &parsed2); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}

	// Should contain group element
	if !strings.Contains(result, "p:grpSp") {
		t.Error("should contain p:grpSp group element")
	}

	// Should contain Heatmap group name
	if !strings.Contains(result, "Heatmap") {
		t.Error("should contain 'Heatmap' group name")
	}

	// Should contain cell values
	for _, v := range []string{"9", "7", "5", "8"} {
		if !strings.Contains(result, ">"+v+"<") {
			t.Errorf("should contain cell value %q", v)
		}
	}

	// Should contain row and column labels
	for _, label := range []string{"Alice", "Bob", "Go", "Python"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain label %q", label)
		}
	}

	// Should use scheme colors (accent1 for cell fills)
	if !strings.Contains(result, "accent1") {
		t.Error("should use accent1 scheme color for cell fills")
	}

	// Should NOT contain srgbClr (no hardcoded hex)
	if strings.Contains(result, "srgbClr") {
		t.Error("should not contain srgbClr (hardcoded hex colors)")
	}

	// Should use rect geometry (not roundRect)
	if !strings.Contains(result, `prst="rect"`) {
		t.Error("should use rect geometry")
	}

	// Should use dk1 for text
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("text should use dk1 scheme color")
	}
}

func TestGenerateHeatmapGroupXML_Empty(t *testing.T) {
	result := generateHeatmapGroupXML(nil, types.BoundingBox{}, 100, heatmapMeta{})
	if result != "" {
		t.Error("should return empty for nil panels")
	}
}

func TestGenerateHeatmapGroupXML_NoLabels(t *testing.T) {
	parsed := heatmapParsedData{
		minVal:     1,
		maxVal:     4,
		colorScale: "sequential",
		values:     [][]float64{{1, 2}, {3, 4}},
	}
	panels := []nativePanelData{
		{title: "__heatmap_meta__", body: encodeHeatmapMeta(parsed)},
		{title: "1"},
		{title: "2"},
		{title: "3"},
		{title: "4"},
	}

	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}
	meta := heatmapMeta{numRows: 2, numCols: 2, colorScale: "sequential"}

	result := generateHeatmapGroupXML(panels, bounds, 100, meta)
	if result == "" {
		t.Fatal("should generate XML without labels")
	}

	var parsed2 interface{}
	if err := xml.Unmarshal([]byte(result), &parsed2); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestGenerateHeatmapGroupXML_Diverging(t *testing.T) {
	parsed := heatmapParsedData{
		rowLabels:  []string{"R1"},
		colLabels:  []string{"C1", "C2", "C3"},
		minVal:     -5,
		maxVal:     5,
		colorScale: "diverging",
		values:     [][]float64{{-5, 0, 5}},
	}
	panels := []nativePanelData{
		{title: "__heatmap_meta__", body: encodeHeatmapMeta(parsed)},
		{title: "-5"},
		{title: "0"},
		{title: "5"},
	}

	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 3000000}
	meta := heatmapMeta{numRows: 1, numCols: 3, colorScale: "diverging"}

	result := generateHeatmapGroupXML(panels, bounds, 100, meta)
	if result == "" {
		t.Fatal("should generate XML for diverging scale")
	}

	// Should contain both accent1 and accent2 for diverging scale
	if !strings.Contains(result, "accent1") {
		t.Error("diverging scale should use accent1")
	}
	if !strings.Contains(result, "accent2") {
		t.Error("diverging scale should use accent2")
	}
}

func TestHeatmapCellFill_Sequential(t *testing.T) {
	// Test that sequential fills use accent1 with varying lumMod
	// These are scheme fills, so we can't directly inspect hex values,
	// but we can verify they don't panic and return non-zero fills.
	fill := heatmapCellFill(5, 0, 10, "sequential")
	_ = fill // just verify no panic

	// Equal min/max should not divide by zero
	fill = heatmapCellFill(5, 5, 5, "sequential")
	_ = fill
}

func TestHeatmapCellFill_Diverging(t *testing.T) {
	fill := heatmapCellFill(-5, -5, 5, "diverging")
	_ = fill

	fill = heatmapCellFill(0, -5, 5, "diverging")
	_ = fill

	fill = heatmapCellFill(5, -5, 5, "diverging")
	_ = fill
}

func TestAllocatePanelIconRelIDs_HeatmapMode(t *testing.T) {
	parsed := heatmapParsedData{
		rowLabels:  []string{"Alice", "Bob"},
		colLabels:  []string{"Go", "Python"},
		minVal:     5,
		maxVal:     9,
		colorScale: "sequential",
		values:     [][]float64{{9, 7}, {5, 8}},
	}
	panels := []nativePanelData{
		{title: "__heatmap_meta__", body: encodeHeatmapMeta(parsed)},
		{title: "9"},
		{title: "7"},
		{title: "5"},
		{title: "8"},
	}

	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000},
						panels:         panels,
						heatmapMode:    true,
						heatmapMeta:    heatmapMeta{numRows: 2, numCols: 2, colorScale: "sequential"},
					},
				},
			},
		},
		MediaContext: MediaContext{
			mediaCounter:    1,
			usedExtensions:  make(map[string]bool),
			mediaFiles:      make(map[string]string),
			slideRelUpdates: make(map[int][]mediaRel),
		},
		SVGContext: SVGContext{
			nativeSVGInserts: make(map[int][]nativeSVGInsert),
		},
	}

	ctx.allocatePanelIconRelIDs()

	inserts := ctx.panelShapeInserts[1]
	if len(inserts) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(inserts))
	}

	if inserts[0].groupXML == "" {
		t.Error("groupXML should be generated for heatmap mode")
	}

	if !strings.Contains(inserts[0].groupXML, "Heatmap") {
		t.Error("groupXML should contain 'Heatmap' name")
	}

	var parsed2 interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed2); err != nil {
		t.Errorf("groupXML should be valid XML, got: %v", err)
	}
}
