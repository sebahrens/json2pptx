package generator

import (
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestIsHouseDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "house_diagram type",
			spec:     &types.DiagramSpec{Type: "house_diagram"},
			expected: true,
		},
		{
			name:     "pyramid type",
			spec:     &types.DiagramSpec{Type: "pyramid"},
			expected: false,
		},
		{
			name:     "swot type",
			spec:     &types.DiagramSpec{Type: "swot"},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isHouseDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isHouseDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseHouseDiagramNativeData_ClassicSections(t *testing.T) {
	data := map[string]any{
		"roof": "Vision: Global Leader",
		"sections": []any{
			map[string]any{"label": "Technology", "items": []any{"Cloud", "API"}},
			map[string]any{"label": "Product", "items": []any{"Mobile"}},
			map[string]any{"label": "People"},
		},
		"foundation": "Core Values",
	}

	panels, meta, err := parseHouseDiagramNativeData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// roof + 3 pillars + foundation = 5 panels
	if len(panels) != 5 {
		t.Fatalf("got %d panels, want 5", len(panels))
	}

	if panels[0].title != "Vision: Global Leader" {
		t.Errorf("roof panel title = %q, want %q", panels[0].title, "Vision: Global Leader")
	}
	if panels[1].title != "Technology" {
		t.Errorf("pillar 1 title = %q, want %q", panels[1].title, "Technology")
	}
	if panels[4].title != "Core Values" {
		t.Errorf("foundation panel title = %q, want %q", panels[4].title, "Core Values")
	}

	if meta.roofLabel != "Vision: Global Leader" {
		t.Errorf("meta.roofLabel = %q, want %q", meta.roofLabel, "Vision: Global Leader")
	}
	if meta.foundationLabel != "Core Values" {
		t.Errorf("meta.foundationLabel = %q, want %q", meta.foundationLabel, "Core Values")
	}
	if len(meta.floors) != 1 {
		t.Fatalf("got %d floors, want 1", len(meta.floors))
	}
	if meta.floors[0].floorType != "parallel" {
		t.Errorf("floor type = %q, want %q", meta.floors[0].floorType, "parallel")
	}
	if meta.floors[0].sectionCount != 3 {
		t.Errorf("floor section count = %d, want 3", meta.floors[0].sectionCount)
	}
}

func TestParseHouseDiagramNativeData_MultiFloor(t *testing.T) {
	data := map[string]any{
		"roof": "Mission",
		"floors": []any{
			map[string]any{"type": "single", "label": "Strategy Band"},
			map[string]any{
				"type": "parallel",
				"sections": []any{
					map[string]any{"label": "A"},
					map[string]any{"label": "B"},
				},
			},
			map[string]any{"type": "single", "label": "Enablers"},
		},
		"foundation": "Values",
	}

	panels, meta, err := parseHouseDiagramNativeData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// roof + 1 single + 2 parallel + 1 single + foundation = 6
	if len(panels) != 6 {
		t.Fatalf("got %d panels, want 6", len(panels))
	}

	if len(meta.floors) != 3 {
		t.Fatalf("got %d floors, want 3", len(meta.floors))
	}

	if meta.floors[0].floorType != "single" || meta.floors[0].sectionCount != 1 {
		t.Errorf("floor 0: type=%q count=%d, want single/1", meta.floors[0].floorType, meta.floors[0].sectionCount)
	}
	if meta.floors[1].floorType != "parallel" || meta.floors[1].sectionCount != 2 {
		t.Errorf("floor 1: type=%q count=%d, want parallel/2", meta.floors[1].floorType, meta.floors[1].sectionCount)
	}
	if meta.floors[2].floorType != "single" || meta.floors[2].sectionCount != 1 {
		t.Errorf("floor 2: type=%q count=%d, want single/1", meta.floors[2].floorType, meta.floors[2].sectionCount)
	}
}

func TestParseHouseDiagramNativeData_RoofMapFormat(t *testing.T) {
	data := map[string]any{
		"roof":       map[string]any{"label": "Roof Label"},
		"pillars":    []any{"P1", "P2"},
		"foundation": map[string]any{"label": "Found Label"},
	}

	panels, meta, err := parseHouseDiagramNativeData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.roofLabel != "Roof Label" {
		t.Errorf("roof label = %q, want %q", meta.roofLabel, "Roof Label")
	}
	if meta.foundationLabel != "Found Label" {
		t.Errorf("foundation label = %q, want %q", meta.foundationLabel, "Found Label")
	}
	if len(panels) != 4 { // roof + 2 pillars + foundation
		t.Errorf("got %d panels, want 4", len(panels))
	}
}

func TestParseHouseDiagramNativeData_OuterElements(t *testing.T) {
	data := map[string]any{
		"center_element":  map[string]any{"label": "Center"},
		"outer_elements":  []any{"Elem1", "Elem2", "Elem3"},
		"foundation":      "Base",
	}

	panels, meta, err := parseHouseDiagramNativeData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta.roofLabel != "Center" {
		t.Errorf("roof label = %q, want %q", meta.roofLabel, "Center")
	}
	if len(panels) != 5 { // roof + 3 elements + foundation
		t.Errorf("got %d panels, want 5", len(panels))
	}
}

func TestParseHouseDiagramNativeData_EmptyData(t *testing.T) {
	data := map[string]any{}
	panels, _, err := parseHouseDiagramNativeData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still have roof + foundation (both empty labels)
	if len(panels) != 2 {
		t.Errorf("got %d panels, want 2 (empty roof + foundation)", len(panels))
	}
}

func TestGenerateHouseDiagramGroupXML(t *testing.T) {
	bounds := types.BoundingBox{
		X: 500000, Y: 1000000,
		Width: 8000000, Height: 5000000,
	}

	panels := []nativePanelData{
		{title: "Vision: Global Leader"},                      // roof
		{title: "Technology", body: "- Cloud\n- API Platform"}, // pillar
		{title: "Product", body: "- Mobile Payments"},          // pillar
		{title: "People"},                                      // pillar
		{title: "Core Values: Integrity"},                      // foundation
	}

	meta := houseDiagramMeta{
		roofLabel:       "Vision: Global Leader",
		foundationLabel: "Core Values: Integrity",
		floors:          []houseFloorMeta{{floorType: "parallel", sectionCount: 3}},
	}

	xml := generateHouseDiagramGroupXML(panels, bounds, 10000, meta)
	if xml == "" {
		t.Fatal("generateHouseDiagramGroupXML returned empty string")
	}

	// Should contain group wrapper.
	if !strings.Contains(xml, "p:grpSp") {
		t.Error("output should contain p:grpSp group wrapper")
	}

	// Should contain roof label.
	if !strings.Contains(xml, "Vision: Global Leader") {
		t.Error("output should contain roof label")
	}

	// Should contain pillar labels.
	for _, label := range []string{"Technology", "Product", "People"} {
		if !strings.Contains(xml, label) {
			t.Errorf("output should contain pillar label %q", label)
		}
	}

	// Should contain foundation label.
	if !strings.Contains(xml, "Core Values: Integrity") {
		t.Error("output should contain foundation label")
	}

	// Should contain triangle geometry for roof.
	if !strings.Contains(xml, "triangle") {
		t.Error("output should contain triangle geometry for roof")
	}

	// Should contain rect geometry for pillars/foundation.
	if !strings.Contains(xml, "rect") {
		t.Error("output should contain rect geometry for pillars/foundation")
	}

	// Should contain bullet items.
	if !strings.Contains(xml, "Cloud") {
		t.Error("output should contain bullet item text")
	}

	// Should be valid XML.
	if err := isValidXMLFragment(xml); err != nil {
		t.Errorf("output is not valid XML: %v", err)
	}
}

func TestGenerateHouseDiagramGroupXML_MultiFloor(t *testing.T) {
	bounds := types.BoundingBox{
		X: 500000, Y: 1000000,
		Width: 8000000, Height: 5000000,
	}

	panels := []nativePanelData{
		{title: "Mission"},         // roof
		{title: "Strategy Band"},   // single floor
		{title: "Pillar A"},        // parallel floor section 1
		{title: "Pillar B"},        // parallel floor section 2
		{title: "Foundation Base"}, // foundation
	}

	meta := houseDiagramMeta{
		roofLabel:       "Mission",
		foundationLabel: "Foundation Base",
		floors: []houseFloorMeta{
			{floorType: "single", sectionCount: 1},
			{floorType: "parallel", sectionCount: 2},
		},
	}

	xml := generateHouseDiagramGroupXML(panels, bounds, 20000, meta)
	if xml == "" {
		t.Fatal("generateHouseDiagramGroupXML returned empty string for multi-floor")
	}

	for _, label := range []string{"Mission", "Strategy Band", "Pillar A", "Pillar B", "Foundation Base"} {
		if !strings.Contains(xml, label) {
			t.Errorf("output should contain label %q", label)
		}
	}

	if err := isValidXMLFragment(xml); err != nil {
		t.Errorf("output is not valid XML: %v", err)
	}
}

func TestGenerateHouseDiagramGroupXML_Empty(t *testing.T) {
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}
	result := generateHouseDiagramGroupXML(nil, bounds, 10000, houseDiagramMeta{})
	if result != "" {
		t.Error("expected empty string for nil panels")
	}
}

func TestGenerateHouseDiagramGroupXML_RoofAndFoundationOnly(t *testing.T) {
	bounds := types.BoundingBox{
		X: 500000, Y: 1000000,
		Width: 8000000, Height: 5000000,
	}

	panels := []nativePanelData{
		{title: "Roof"},
		{title: "Foundation"},
	}

	meta := houseDiagramMeta{
		roofLabel:       "Roof",
		foundationLabel: "Foundation",
	}

	xml := generateHouseDiagramGroupXML(panels, bounds, 10000, meta)
	if xml == "" {
		t.Fatal("generateHouseDiagramGroupXML returned empty string for roof+foundation only")
	}

	if !strings.Contains(xml, "triangle") {
		t.Error("output should contain triangle geometry for roof")
	}
	if !strings.Contains(xml, "Roof") {
		t.Error("output should contain roof label")
	}
	if !strings.Contains(xml, "Foundation") {
		t.Error("output should contain foundation label")
	}

	if err := isValidXMLFragment(xml); err != nil {
		t.Errorf("output is not valid XML: %v", err)
	}
}

func TestHouseDiagramEstimateShapeCount(t *testing.T) {
	panels := make([]nativePanelData, 5) // roof + 3 pillars + foundation
	got := houseDiagramEstimateShapeCount(panels)
	if got != 6 { // 1 group + 5 shapes
		t.Errorf("houseDiagramEstimateShapeCount() = %d, want 6", got)
	}
}

func TestInferHouseFloorType(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		expected string
	}{
		{
			name:     "explicit single",
			m:        map[string]any{"type": "single", "label": "Test"},
			expected: "single",
		},
		{
			name:     "explicit parallel",
			m:        map[string]any{"type": "parallel", "sections": []any{}},
			expected: "parallel",
		},
		{
			name:     "inferred parallel from sections key",
			m:        map[string]any{"sections": []any{"A", "B"}},
			expected: "parallel",
		},
		{
			name:     "inferred single when no sections",
			m:        map[string]any{"label": "Band"},
			expected: "single",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferHouseFloorType(tt.m)
			if got != tt.expected {
				t.Errorf("inferHouseFloorType() = %q, want %q", got, tt.expected)
			}
		})
	}
}
