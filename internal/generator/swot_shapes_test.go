package generator

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestIsSWOTDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "swot type",
			spec:     &types.DiagramSpec{Type: "swot"},
			expected: true,
		},
		{
			name:     "non-swot type",
			spec:     &types.DiagramSpec{Type: "pestel"},
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
			got := isSWOTDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isSWOTDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseSWOTStringList(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected int
	}{
		{"nil", nil, 0},
		{"empty slice", []any{}, 0},
		{"string items", []any{"a", "b", "c"}, 3},
		{"mixed types", []any{"a", 42, "b"}, 2},
		{"string slice", []string{"x", "y"}, 2},
		{"wrong type", 42, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSWOTStringList(tt.input)
			if len(result) != tt.expected {
				t.Errorf("parseSWOTStringList() returned %d items, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestGenerateSWOTGroupXML_Basic(t *testing.T) {
	panels := []nativePanelData{
		{title: "Strengths", body: "- Strong brand\n- Loyal customers"},
		{title: "Weaknesses", body: "- Limited market share"},
		{title: "Opportunities", body: "- New markets\n- AI adoption"},
		{title: "Threats", body: "- Competition"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 5000000}

	result := generateSWOTGroupXML(panels, bounds, 100)

	// Should produce non-empty XML
	if result == "" {
		t.Fatal("generateSWOTGroupXML returned empty string")
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

	// Should contain SWOT Analysis name
	if !strings.Contains(result, "SWOT Analysis") {
		t.Error("should contain 'SWOT Analysis' group name")
	}

	// Should contain all 4 quadrant headers
	for _, label := range []string{"Strengths", "Weaknesses", "Opportunities", "Threats"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain quadrant label %q", label)
		}
	}

	// Should use scheme colors (accent1-4)
	for i := 1; i <= 4; i++ {
		accent := "accent" + string(rune('0'+i))
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
	if !strings.Contains(result, "Strong brand") {
		t.Error("should contain bullet text 'Strong brand'")
	}
	if !strings.Contains(result, "Competition") {
		t.Error("should contain bullet text 'Competition'")
	}

	// Should contain bullet character
	if !strings.Contains(result, `buChar char=`) {
		t.Error("should contain bullet character elements")
	}

	// Should use dk1 for header text
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("header text should use dk1 scheme color")
	}
}

func TestGenerateSWOTGroupXML_Wrong_Panel_Count(t *testing.T) {
	// Should return empty for non-4 panel counts
	panels := []nativePanelData{
		{title: "A", body: "test"},
		{title: "B", body: "test"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateSWOTGroupXML(panels, bounds, 100)
	if result != "" {
		t.Error("generateSWOTGroupXML should return empty for non-4 panel count")
	}
}

func TestGenerateSWOTGroupXML_2x2_Layout(t *testing.T) {
	panels := []nativePanelData{
		{title: "Strengths", body: "- A"},
		{title: "Weaknesses", body: "- B"},
		{title: "Opportunities", body: "- C"},
		{title: "Threats", body: "- D"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 6000000}

	result := generateSWOTGroupXML(panels, bounds, 100)

	// Verify 2x2 layout positions
	quadW := (bounds.Width - swotGap) / 2
	quadH := (bounds.Height - swotGap) / 2

	// Top-left (Strengths): starts at bounds origin
	topLeftX := bounds.X
	topLeftY := bounds.Y

	// Top-right (Weaknesses): offset by quadW + gap
	topRightX := bounds.X + quadW + swotGap

	// Bottom-left (Opportunities): offset by quadH + gap
	bottomLeftY := bounds.Y + quadH + swotGap

	// Bottom-right (Threats): both offsets
	bottomRightX := bounds.X + quadW + swotGap
	bottomRightY := bounds.Y + quadH + swotGap

	// Check positions exist in XML (each position appears in both header and body shapes)
	checks := []struct {
		desc  string
		value string
	}{
		{"top-left x", formatAttr("x", topLeftX)},
		{"top-left y", formatAttr("y", topLeftY)},
		{"top-right x", formatAttr("x", topRightX)},
		{"bottom-left y", formatAttr("y", bottomLeftY)},
		{"bottom-right x", formatAttr("x", bottomRightX)},
		{"bottom-right y", formatAttr("y", bottomRightY)},
	}
	for _, c := range checks {
		if !strings.Contains(result, c.value) {
			t.Errorf("should contain %s: %s", c.desc, c.value)
		}
	}
}

func formatAttr(name string, value int64) string {
	return fmt.Sprintf(`%s="%d"`, name, value)
}

func TestGenerateSWOTGroupXML_EmptyBullets(t *testing.T) {
	panels := []nativePanelData{
		{title: "Strengths", body: ""},
		{title: "Weaknesses", body: ""},
		{title: "Opportunities", body: ""},
		{title: "Threats", body: ""},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateSWOTGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("should generate XML even with empty bullets")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestAllocatePanelIconRelIDs_SWOTMode(t *testing.T) {
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000},
						panels: []nativePanelData{
							{title: "Strengths", body: "- Strong brand"},
							{title: "Weaknesses", body: "- Costs"},
							{title: "Opportunities", body: "- Growth"},
							{title: "Threats", body: "- Risk"},
						},
						swotMode: true,
					},
				},
			},
		},
		MediaContext: MediaContext{
			media:           pptx.NewMediaAllocator(),
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

	// groupXML should be generated using SWOT layout
	if inserts[0].groupXML == "" {
		t.Error("groupXML should be generated for SWOT mode")
	}

	// Should use roundRect (SWOT style) not rect (panel style)
	if !strings.Contains(inserts[0].groupXML, `prst="roundRect"`) {
		t.Error("SWOT groupXML should use roundRect geometry")
	}

	// Should contain SWOT Analysis group name
	if !strings.Contains(inserts[0].groupXML, "SWOT Analysis") {
		t.Error("SWOT groupXML should contain 'SWOT Analysis' name")
	}

	// Should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed); err != nil {
		t.Errorf("SWOT groupXML should be valid XML, got: %v", err)
	}
}
