package generator

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestIsKPIDashboardDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "kpi_dashboard type",
			spec:     &types.DiagramSpec{Type: "kpi_dashboard"},
			expected: true,
		},
		{
			name:     "non-kpi type",
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
			got := isKPIDashboardDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isKPIDashboardDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseKPIMetrics(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expected int
	}{
		{
			name: "metrics key",
			data: map[string]any{
				"metrics": []any{
					map[string]any{"label": "Revenue", "value": "$21M"},
					map[string]any{"label": "NPS", "value": "72"},
				},
			},
			expected: 2,
		},
		{
			name: "kpis alias",
			data: map[string]any{
				"kpis": []any{
					map[string]any{"label": "Revenue", "value": "$21M"},
				},
			},
			expected: 1,
		},
		{
			name:     "empty data",
			data:     map[string]any{},
			expected: 0,
		},
		{
			name:     "nil data",
			data:     nil,
			expected: 0,
		},
		{
			name: "numeric value",
			data: map[string]any{
				"metrics": []any{
					map[string]any{"label": "Count", "value": float64(42)},
				},
			},
			expected: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseKPIMetrics(tt.data)
			if len(result) != tt.expected {
				t.Errorf("parseKPIMetrics() returned %d metrics, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestParseKPIMetrics_FieldParsing(t *testing.T) {
	data := map[string]any{
		"metrics": []any{
			map[string]any{
				"label": "Revenue",
				"value": "$21M",
				"delta": "+17%",
				"trend": "up",
			},
		},
	}
	metrics := parseKPIMetrics(data)
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	m := metrics[0]
	if m.label != "Revenue" {
		t.Errorf("label = %q, want %q", m.label, "Revenue")
	}
	if m.value != "$21M" {
		t.Errorf("value = %q, want %q", m.value, "$21M")
	}
	if m.delta != "+17%" {
		t.Errorf("delta = %q, want %q", m.delta, "+17%")
	}
	if m.trend != "up" {
		t.Errorf("trend = %q, want %q", m.trend, "up")
	}
}

func TestBuildKPIDeltaText(t *testing.T) {
	tests := []struct {
		name     string
		delta    string
		trend    string
		expected string
	}{
		{"up trend", "+17%", "up", "\u25B2 +17%"},
		{"down trend", "-3%", "down", "\u25BC -3%"},
		{"flat trend", "0%", "flat", "\u2192 0%"},
		{"no trend", "+5%", "", "+5%"},
		{"no delta up", "", "up", "\u25B2"},
		{"empty both", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKPIDeltaText(tt.delta, tt.trend)
			if got != tt.expected {
				t.Errorf("buildKPIDeltaText(%q, %q) = %q, want %q", tt.delta, tt.trend, got, tt.expected)
			}
		})
	}
}

func TestGenerateKPIDashboardGroupXML_Basic(t *testing.T) {
	panels := []nativePanelData{
		{title: "Revenue", value: "$21M", body: "\u25B2 +17%"},
		{title: "Customers", value: "1,250", body: "\u25B2 +8%"},
		{title: "NPS", value: "72", body: "\u25B2 +5"},
		{title: "Churn", value: "2.1%", body: "\u25BC -0.3%"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 4000000}

	result := generateKPIDashboardGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("generateKPIDashboardGroupXML returned empty string")
	}

	// Should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}

	// Should contain group element
	if !strings.Contains(result, "p:grpSp") {
		t.Error("should contain p:grpSp group element")
	}

	// Should contain KPI Dashboard name
	if !strings.Contains(result, "KPI Dashboard") {
		t.Error("should contain 'KPI Dashboard' group name")
	}

	// Should contain all metric values
	for _, v := range []string{"$21M", "1,250", "72", "2.1%"} {
		if !strings.Contains(result, v) {
			t.Errorf("should contain metric value %q", v)
		}
	}

	// Should contain uppercase labels
	for _, label := range []string{"REVENUE", "CUSTOMERS", "NPS", "CHURN"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain uppercase label %q", label)
		}
	}

	// Should use scheme colors (accent1 for values, accent6 for up, accent2 for down)
	for _, color := range []string{"accent1", "accent2", "accent6"} {
		if !strings.Contains(result, color) {
			t.Errorf("should contain scheme color %q", color)
		}
	}

	// Should use roundRect geometry
	if !strings.Contains(result, `prst="roundRect"`) {
		t.Error("should use roundRect geometry")
	}

	// Should NOT contain srgbClr (no hardcoded hex)
	if strings.Contains(result, "srgbClr") {
		t.Error("should not contain srgbClr (hardcoded hex colors)")
	}

	// Should use dk1 for label text
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("label text should use dk1 scheme color")
	}
}

func TestGenerateKPIDashboardGroupXML_Empty(t *testing.T) {
	result := generateKPIDashboardGroupXML(nil, types.BoundingBox{}, 100)
	if result != "" {
		t.Error("should return empty for nil panels")
	}
}

func TestGenerateKPIDashboardGroupXML_SingleMetric(t *testing.T) {
	panels := []nativePanelData{
		{title: "Revenue", value: "$21M", body: "\u25B2 +17%"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 4000000}

	result := generateKPIDashboardGroupXML(panels, bounds, 100)
	if result == "" {
		t.Fatal("should generate XML for single metric")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestGenerateKPIDashboardGroupXML_NoDeltas(t *testing.T) {
	panels := []nativePanelData{
		{title: "Revenue", value: "$21M"},
		{title: "NPS", value: "72"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 4000000}

	result := generateKPIDashboardGroupXML(panels, bounds, 100)
	if result == "" {
		t.Fatal("should generate XML without delta text")
	}

	// Should still contain values and labels
	if !strings.Contains(result, "$21M") {
		t.Error("should contain value '$21M'")
	}
	if !strings.Contains(result, "REVENUE") {
		t.Error("should contain label 'REVENUE'")
	}
}

func TestKPIGridLayout(t *testing.T) {
	tests := []struct {
		n            int
		expectedCols int
		expectedRows int
	}{
		{1, 1, 1},
		{2, 2, 1},
		{3, 3, 1},
		{4, 4, 1},
		{5, 3, 2},
		{6, 3, 2},
		{7, 4, 2},
		{8, 4, 2},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			cols, rows := kpiGridLayout(tt.n)
			if cols != tt.expectedCols || rows != tt.expectedRows {
				t.Errorf("kpiGridLayout(%d) = (%d, %d), want (%d, %d)",
					tt.n, cols, rows, tt.expectedCols, tt.expectedRows)
			}
		})
	}
}

func TestAllocatePanelIconRelIDs_KPIDashboardMode(t *testing.T) {
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 4000000},
						panels: []nativePanelData{
							{title: "Revenue", value: "$21M", body: "\u25B2 +17%"},
							{title: "Customers", value: "1,250", body: "\u25B2 +8%"},
							{title: "NPS", value: "72", body: "\u25B2 +5"},
							{title: "Churn", value: "2.1%", body: "\u25BC -0.3%"},
						},
						kpiDashboardMode: true,
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
		t.Error("groupXML should be generated for KPI dashboard mode")
	}

	if !strings.Contains(inserts[0].groupXML, "KPI Dashboard") {
		t.Error("groupXML should contain 'KPI Dashboard' name")
	}

	if !strings.Contains(inserts[0].groupXML, `prst="roundRect"`) {
		t.Error("groupXML should use roundRect geometry")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed); err != nil {
		t.Errorf("groupXML should be valid XML, got: %v", err)
	}
}
