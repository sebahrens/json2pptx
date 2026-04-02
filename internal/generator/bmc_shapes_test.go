package generator

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestIsBMCDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "business_model_canvas type",
			spec:     &types.DiagramSpec{Type: "business_model_canvas"},
			expected: true,
		},
		{
			name:     "non-bmc type",
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
			got := isBMCDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isBMCDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseBMCSections_FlatFormat(t *testing.T) {
	data := map[string]any{
		"key_partners":           []any{"Cloud providers", "Integrators"},
		"key_activities":         []any{"Platform dev"},
		"key_resources":          []any{"Engineering team"},
		"value_propositions":     []any{"No-code pipelines"},
		"customer_relationships": []any{"Self-service"},
		"channels":              []any{"Website"},
		"customer_segments":     []any{"Mid-market"},
		"cost_structure":        []any{"Infrastructure 40%"},
		"revenue_streams":       []any{"Subscriptions"},
	}

	sections := parseBMCSections(data)

	if len(sections) != 9 {
		t.Fatalf("expected 9 sections, got %d", len(sections))
	}

	// Check key_partners
	kp := sections[bmcKeyPartners]
	if len(kp.items) != 2 {
		t.Errorf("key_partners: expected 2 items, got %d", len(kp.items))
	}

	// Check value_propositions alias maps to value_proposition key
	vp := sections[bmcValueProposition]
	if len(vp.items) != 1 || vp.items[0] != "No-code pipelines" {
		t.Errorf("value_proposition: unexpected items %v", vp.items)
	}
}

func TestParseBMCSections_NestedBoxesFormat(t *testing.T) {
	data := map[string]any{
		"boxes": map[string]any{
			"key_partners": map[string]any{
				"items": []any{"Cloud providers"},
				"title": "Partners",
			},
			"key_activities": map[string]any{
				"items": []any{"Dev", "Support"},
			},
		},
	}

	sections := parseBMCSections(data)

	kp := sections[bmcKeyPartners]
	if kp.title != "Partners" {
		t.Errorf("expected custom title 'Partners', got %q", kp.title)
	}
	if len(kp.items) != 1 {
		t.Errorf("expected 1 item, got %d", len(kp.items))
	}

	ka := sections[bmcKeyActivities]
	if len(ka.items) != 2 {
		t.Errorf("expected 2 items, got %d", len(ka.items))
	}
}

func TestParseBMCSections_CamelCaseAliases(t *testing.T) {
	data := map[string]any{
		"keyPartners":       []any{"Partner A"},
		"valueProposition":  []any{"VP"},
		"customerSegments":  []any{"Segment A"},
		"costStructure":     []any{"Cost A"},
		"revenueStreams":    []any{"Revenue A"},
	}

	sections := parseBMCSections(data)

	if kp := sections[bmcKeyPartners]; len(kp.items) != 1 {
		t.Errorf("keyPartners alias: expected 1 item, got %d", len(kp.items))
	}
	if vp := sections[bmcValueProposition]; len(vp.items) != 1 {
		t.Errorf("valueProposition alias: expected 1 item, got %d", len(vp.items))
	}
}

func TestParseBMCSections_StringValue(t *testing.T) {
	data := map[string]any{
		"key_partners": "Single partner",
	}

	sections := parseBMCSections(data)
	kp := sections[bmcKeyPartners]
	if len(kp.items) != 1 || kp.items[0] != "Single partner" {
		t.Errorf("string value: expected ['Single partner'], got %v", kp.items)
	}
}

func TestParseBMCSections_StringSlice(t *testing.T) {
	data := map[string]any{
		"key_partners": []string{"A", "B"},
	}

	sections := parseBMCSections(data)
	kp := sections[bmcKeyPartners]
	if len(kp.items) != 2 {
		t.Errorf("string slice: expected 2 items, got %d", len(kp.items))
	}
}

func TestGenerateBMCGroupXML_Basic(t *testing.T) {
	panels := []nativePanelData{
		{title: "Key Partners", body: "- Cloud providers\n- Integrators"},
		{title: "Key Activities", body: "- Platform dev"},
		{title: "Key Resources", body: "- Engineering team"},
		{title: "Value Proposition", body: "- No-code pipelines"},
		{title: "Customer Relationships", body: "- Self-service"},
		{title: "Channels", body: "- Website"},
		{title: "Customer Segments", body: "- Mid-market"},
		{title: "Cost Structure", body: "- Infrastructure 40%"},
		{title: "Revenue Streams", body: "- Subscriptions"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 5000000}

	result := generateBMCGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("generateBMCGroupXML returned empty string")
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

	// Should contain BMC group name
	if !strings.Contains(result, "Business Model Canvas") {
		t.Error("should contain 'Business Model Canvas' group name")
	}

	// Should contain all 9 section headers
	for _, label := range []string{
		"Key Partners", "Key Activities", "Key Resources",
		"Value Proposition", "Customer Relationships", "Channels",
		"Customer Segments", "Cost Structure", "Revenue Streams",
	} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain section label %q", label)
		}
	}

	// Should use scheme colors (multiple accents)
	for i := 1; i <= 6; i++ {
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
	if !strings.Contains(result, "Cloud providers") {
		t.Error("should contain bullet text 'Cloud providers'")
	}
	if !strings.Contains(result, "Subscriptions") {
		t.Error("should contain bullet text 'Subscriptions'")
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

func TestGenerateBMCGroupXML_WrongPanelCount(t *testing.T) {
	panels := []nativePanelData{
		{title: "A", body: "test"},
		{title: "B", body: "test"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateBMCGroupXML(panels, bounds, 100)
	if result != "" {
		t.Error("generateBMCGroupXML should return empty for non-9 panel count")
	}
}

func TestGenerateBMCGroupXML_EmptyBullets(t *testing.T) {
	panels := make([]nativePanelData, 9)
	for i := range panels {
		panels[i].title = "Section"
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateBMCGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("should generate XML even with empty bullets")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestGenerateBMCGroupXML_Layout(t *testing.T) {
	panels := make([]nativePanelData, 9)
	for i := range panels {
		panels[i].title = "Section"
		panels[i].body = "- Item"
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 8000000, Height: 6000000}

	result := generateBMCGroupXML(panels, bounds, 100)

	// Count the number of p:sp shapes — should be 18 (9 headers + 9 bodies)
	shapeCount := strings.Count(result, "<p:sp>")
	if shapeCount != 18 {
		t.Errorf("expected 18 shapes (9 headers + 9 bodies), got %d", shapeCount)
	}

	// The group bounds should match the input bounds
	if !strings.Contains(result, `x="100000"`) {
		t.Error("group should contain bounds X offset")
	}
	if !strings.Contains(result, `y="200000"`) {
		t.Error("group should contain bounds Y offset")
	}
}

func TestAllocatePanelIconRelIDs_BMCMode(t *testing.T) {
	panels := make([]nativePanelData, 9)
	for i := range panels {
		panels[i].title = "Section"
		panels[i].body = "- Item"
	}

	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000},
						panels:         panels,
						bmcMode:        true,
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
		t.Error("groupXML should be generated for BMC mode")
	}

	// Should use roundRect (BMC style)
	if !strings.Contains(inserts[0].groupXML, `prst="roundRect"`) {
		t.Error("BMC groupXML should use roundRect geometry")
	}

	// Should contain BMC group name
	if !strings.Contains(inserts[0].groupXML, "Business Model Canvas") {
		t.Error("BMC groupXML should contain 'Business Model Canvas' name")
	}

	// Should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed); err != nil {
		t.Errorf("BMC groupXML should be valid XML, got: %v", err)
	}
}
