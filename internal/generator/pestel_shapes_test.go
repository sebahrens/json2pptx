package generator

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestIsPESTELDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "pestel type",
			spec:     &types.DiagramSpec{Type: "pestel"},
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
			got := isPESTELDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isPESTELDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParsePESTELSegments_IndividualKeys(t *testing.T) {
	data := map[string]any{
		"political":     []any{"Trade policies", "Government stability"},
		"economic":      []any{"Interest rates"},
		"social":        []any{"Remote work"},
		"technological": []any{"AI advancement", "Cloud computing"},
		"environmental": []any{"Carbon regulations"},
		"legal":         []any{"Data privacy laws"},
	}

	panels := parsePESTELSegments(data)
	if len(panels) != 6 {
		t.Fatalf("expected 6 segments, got %d", len(panels))
	}

	// Check labels are in canonical order
	expectedLabels := []string{"Political", "Economic", "Social", "Technological", "Environmental", "Legal"}
	for i, label := range expectedLabels {
		if panels[i].title != label {
			t.Errorf("segment %d: expected title %q, got %q", i, label, panels[i].title)
		}
	}

	// Check body has bullet format
	if !strings.Contains(panels[0].body, "- Trade policies") {
		t.Error("Political segment should contain '- Trade policies'")
	}
}

func TestParsePESTELSegments_SegmentsArray(t *testing.T) {
	data := map[string]any{
		"segments": []any{
			map[string]any{"name": "Political", "items": []any{"Trade policies"}},
			map[string]any{"name": "Economic", "items": []any{"Interest rates"}},
		},
	}

	panels := parsePESTELSegments(data)
	if len(panels) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(panels))
	}
	if panels[0].title != "Political" {
		t.Errorf("expected title 'Political', got %q", panels[0].title)
	}
}

func TestParsePESTELSegments_FactorsAlias(t *testing.T) {
	data := map[string]any{
		"factors": []any{
			map[string]any{"name": "Legal", "items": []any{"GDPR"}},
		},
	}

	panels := parsePESTELSegments(data)
	if len(panels) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(panels))
	}
	if panels[0].title != "Legal" {
		t.Errorf("expected title 'Legal', got %q", panels[0].title)
	}
}

func TestParsePESTELSegments_CategoryAlias(t *testing.T) {
	data := map[string]any{
		"segments": []any{
			map[string]any{"category": "Social", "items": []any{"Demographics"}},
		},
	}

	panels := parsePESTELSegments(data)
	if len(panels) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(panels))
	}
	if panels[0].title != "Social" {
		t.Errorf("expected title 'Social', got %q", panels[0].title)
	}
}

func TestParsePESTELSegments_Empty(t *testing.T) {
	panels := parsePESTELSegments(map[string]any{})
	if len(panels) != 0 {
		t.Errorf("expected 0 segments from empty data, got %d", len(panels))
	}
}

func TestGeneratePESTELGroupXML_Basic(t *testing.T) {
	panels := []nativePanelData{
		{title: "Political", body: "- Trade policies\n- Government stability"},
		{title: "Economic", body: "- Interest rates"},
		{title: "Social", body: "- Remote work\n- Demographics"},
		{title: "Technological", body: "- AI advancement\n- Cloud computing"},
		{title: "Environmental", body: "- Carbon regulations"},
		{title: "Legal", body: "- Data privacy laws"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 9000000, Height: 5000000}

	result := generatePESTELGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("generatePESTELGroupXML returned empty string")
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

	// Should contain PESTEL Analysis name
	if !strings.Contains(result, "PESTEL Analysis") {
		t.Error("should contain 'PESTEL Analysis' group name")
	}

	// Should contain all 6 segment headers
	for _, label := range []string{"Political", "Economic", "Social", "Technological", "Environmental", "Legal"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain segment label %q", label)
		}
	}

	// Should use scheme colors (accent1-6)
	for i := 1; i <= 6; i++ {
		accent := fmt.Sprintf("accent%d", i)
		if !strings.Contains(result, accent) {
			t.Errorf("should contain scheme color %q", accent)
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

	// Should contain bullet text
	if !strings.Contains(result, "Trade policies") {
		t.Error("should contain bullet text 'Trade policies'")
	}

	// Should use dk1 for header text
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("header text should use dk1 scheme color")
	}
}

func TestGeneratePESTELGroupXML_3x2_Layout(t *testing.T) {
	panels := make([]nativePanelData, 6)
	for i := range panels {
		panels[i] = nativePanelData{title: fmt.Sprintf("Cat%d", i), body: "- item"}
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 9000000, Height: 6000000}

	result := generatePESTELGroupXML(panels, bounds, 100)

	// Verify 3x2 layout positions
	numCols := 3
	numRows := 2
	cellW := (bounds.Width - int64(numCols-1)*pestelGap) / int64(numCols)
	cellH := (bounds.Height - int64(numRows-1)*pestelGap) / int64(numRows)

	// Check key positions exist in XML
	checks := []struct {
		desc  string
		value string
	}{
		// First cell (0,0)
		{"cell(0,0) x", fmt.Sprintf(`x="%d"`, bounds.X)},
		{"cell(0,0) y", fmt.Sprintf(`y="%d"`, bounds.Y)},
		// Second cell (1,0) - second column
		{"cell(1,0) x", fmt.Sprintf(`x="%d"`, bounds.X+cellW+pestelGap)},
		// Third cell (2,0) - third column
		{"cell(2,0) x", fmt.Sprintf(`x="%d"`, bounds.X+2*(cellW+pestelGap))},
		// Fourth cell (0,1) - second row
		{"cell(0,1) y", fmt.Sprintf(`y="%d"`, bounds.Y+cellH+pestelGap)},
	}
	for _, c := range checks {
		if !strings.Contains(result, c.value) {
			t.Errorf("should contain %s: %s", c.desc, c.value)
		}
	}
}

func TestGeneratePESTELGroupXML_EmptyPanels(t *testing.T) {
	result := generatePESTELGroupXML(nil, types.BoundingBox{Width: 8000000, Height: 5000000}, 100)
	if result != "" {
		t.Error("should return empty for nil panels")
	}
}

func TestGeneratePESTELGroupXML_FewerThan6(t *testing.T) {
	panels := []nativePanelData{
		{title: "Political", body: "- item1"},
		{title: "Economic", body: "- item2"},
		{title: "Social", body: "- item3"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 9000000, Height: 5000000}

	result := generatePESTELGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("should generate XML for fewer than 6 segments")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestGeneratePESTELGroupXML_EmptyBullets(t *testing.T) {
	panels := make([]nativePanelData, 6)
	for i := range panels {
		panels[i] = nativePanelData{title: fmt.Sprintf("Cat%d", i), body: ""}
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generatePESTELGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("should generate XML even with empty bullets")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestAllocatePanelIconRelIDs_PESTELMode(t *testing.T) {
	panels := make([]nativePanelData, 6)
	for i := range panels {
		panels[i] = nativePanelData{
			title: pestelSegmentColors[i].label,
			body:  "- item",
		}
	}

	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 9000000, Height: 5000000},
						panels:         panels,
						pestelMode:     true,
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
		t.Error("groupXML should be generated for PESTEL mode")
	}

	// Should use roundRect (PESTEL style)
	if !strings.Contains(inserts[0].groupXML, `prst="roundRect"`) {
		t.Error("PESTEL groupXML should use roundRect geometry")
	}

	// Should contain PESTEL Analysis group name
	if !strings.Contains(inserts[0].groupXML, "PESTEL Analysis") {
		t.Error("PESTEL groupXML should contain 'PESTEL Analysis' name")
	}

	// Should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed); err != nil {
		t.Errorf("PESTEL groupXML should be valid XML, got: %v", err)
	}
}
