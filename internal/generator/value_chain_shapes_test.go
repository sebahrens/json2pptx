package generator

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/ahrens/go-slide-creator/internal/types"
)

func TestIsValueChainDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "value_chain type",
			spec:     &types.DiagramSpec{Type: "value_chain"},
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
			got := isValueChainDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isValueChainDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseValueChainData(t *testing.T) {
	data := map[string]any{
		"primary_activities": []any{
			map[string]any{"name": "Inbound Logistics", "items": []any{"Receiving", "Warehousing"}},
			map[string]any{"label": "Operations"},
			"Outbound Logistics",
			map[string]any{"title": "Marketing & Sales"},
			map[string]any{"name": "Service", "description": "After-sales support"},
		},
		"support_activities": []any{
			map[string]any{"name": "Firm Infrastructure"},
			map[string]any{"name": "HR Management"},
			map[string]any{"name": "Technology Development"},
			map[string]any{"name": "Procurement"},
		},
		"margin_label": "Profit",
	}

	panels, meta := parseValueChainData(data)

	if meta.supportCount != 4 {
		t.Errorf("supportCount = %d, want 4", meta.supportCount)
	}
	if meta.primaryCount != 5 {
		t.Errorf("primaryCount = %d, want 5", meta.primaryCount)
	}
	if meta.marginLabel != "Profit" {
		t.Errorf("marginLabel = %q, want %q", meta.marginLabel, "Profit")
	}
	if len(panels) != 9 {
		t.Fatalf("len(panels) = %d, want 9", len(panels))
	}

	// Support activities come first
	if panels[0].title != "Firm Infrastructure" {
		t.Errorf("panels[0].title = %q, want %q", panels[0].title, "Firm Infrastructure")
	}

	// Primary activities follow
	if panels[4].title != "Inbound Logistics" {
		t.Errorf("panels[4].title = %q, want %q", panels[4].title, "Inbound Logistics")
	}
	// Inbound Logistics should have bullet items
	if !strings.Contains(panels[4].body, "Receiving") {
		t.Errorf("panels[4].body should contain 'Receiving', got %q", panels[4].body)
	}

	// String-only activity
	if panels[6].title != "Outbound Logistics" {
		t.Errorf("panels[6].title = %q, want %q", panels[6].title, "Outbound Logistics")
	}

	// Description fallback
	if panels[8].body != "After-sales support" {
		t.Errorf("panels[8].body = %q, want %q", panels[8].body, "After-sales support")
	}
}

func TestParseValueChainData_DefaultMargin(t *testing.T) {
	data := map[string]any{
		"primary": []any{"A", "B"},
	}
	_, meta := parseValueChainData(data)
	if meta.marginLabel != "Margin" {
		t.Errorf("default marginLabel = %q, want %q", meta.marginLabel, "Margin")
	}
}

func TestParseValueChainData_NoMargin(t *testing.T) {
	data := map[string]any{
		"primary":     []any{"A", "B"},
		"show_margin": false,
	}
	_, meta := parseValueChainData(data)
	if meta.marginLabel != "" {
		t.Errorf("marginLabel = %q, want empty (show_margin=false)", meta.marginLabel)
	}
}

func TestGenerateValueChainGroupXML_Basic(t *testing.T) {
	panels := []nativePanelData{
		{title: "Firm Infrastructure"},
		{title: "HR Management"},
		{title: "Technology Development"},
		{title: "Procurement"},
		{title: "Inbound Logistics", body: "- Receiving\n- Warehousing"},
		{title: "Operations", body: "- Manufacturing"},
		{title: "Outbound Logistics"},
		{title: "Marketing & Sales"},
		{title: "Service"},
	}
	meta := valueChainMeta{
		supportCount: 4,
		primaryCount: 5,
		marginLabel:  "Margin",
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 10000000, Height: 5000000}

	result := generateValueChainGroupXML(panels, bounds, 100, meta)

	if result == "" {
		t.Fatal("generateValueChainGroupXML returned empty string")
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

	// Should contain Value Chain name
	if !strings.Contains(result, "Value Chain") {
		t.Error("should contain 'Value Chain' group name")
	}

	// Should contain support activity labels
	for _, label := range []string{"Firm Infrastructure", "HR Management", "Technology Development", "Procurement"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain support label %q", label)
		}
	}

	// Should contain primary activity labels
	for _, label := range []string{"Inbound Logistics", "Operations", "Outbound Logistics", "Marketing &amp; Sales", "Service"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain primary label %q", label)
		}
	}

	// Should contain margin label
	if !strings.Contains(result, "Margin") {
		t.Error("should contain margin label 'Margin'")
	}

	// Should use scheme colors
	if !strings.Contains(result, `schemeClr val="accent1"`) {
		t.Error("should contain accent1 scheme color")
	}

	// Should use homePlate geometry for primary activities (not last)
	if !strings.Contains(result, `prst="homePlate"`) {
		t.Error("should use homePlate geometry for primary activities")
	}

	// Should use roundRect for support bars and last primary
	if !strings.Contains(result, `prst="roundRect"`) {
		t.Error("should use roundRect geometry for support bars")
	}

	// Should use dk1 for text color
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("should use dk1 for text color")
	}

	// Should NOT contain srgbClr (no hardcoded hex)
	if strings.Contains(result, "srgbClr") {
		t.Error("should not contain srgbClr (hardcoded hex colors)")
	}

	// Should contain bullet text
	if !strings.Contains(result, "Receiving") {
		t.Error("should contain bullet text 'Receiving'")
	}

	// Should contain vertical text direction for margin
	if !strings.Contains(result, `vert="vert270"`) {
		t.Error("margin should use vert270 for vertical text")
	}
}

func TestGenerateValueChainGroupXML_NoMargin(t *testing.T) {
	panels := []nativePanelData{
		{title: "Support A"},
		{title: "Primary A"},
		{title: "Primary B"},
	}
	meta := valueChainMeta{
		supportCount: 1,
		primaryCount: 2,
		marginLabel:  "",
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateValueChainGroupXML(panels, bounds, 100, meta)

	if result == "" {
		t.Fatal("should generate XML without margin")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}

	// Should not contain margin shape name
	if strings.Contains(result, "VC Margin") {
		t.Error("should not contain margin shape when marginLabel is empty")
	}
}

func TestGenerateValueChainGroupXML_PrimaryOnly(t *testing.T) {
	panels := []nativePanelData{
		{title: "Step 1"},
		{title: "Step 2"},
		{title: "Step 3"},
	}
	meta := valueChainMeta{
		supportCount: 0,
		primaryCount: 3,
		marginLabel:  "",
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateValueChainGroupXML(panels, bounds, 100, meta)

	if result == "" {
		t.Fatal("should generate XML with primary-only activities")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}

	// Last primary should use roundRect, not homePlate
	// Count occurrences of homePlate (should be 2, not 3)
	homePlateCount := strings.Count(result, `prst="homePlate"`)
	if homePlateCount != 2 {
		t.Errorf("expected 2 homePlate shapes, got %d", homePlateCount)
	}
}

func TestGenerateValueChainGroupXML_Empty(t *testing.T) {
	panels := []nativePanelData{}
	meta := valueChainMeta{supportCount: 0, primaryCount: 0}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 8000000, Height: 5000000}

	result := generateValueChainGroupXML(panels, bounds, 100, meta)
	if result != "" {
		t.Error("should return empty for zero activities")
	}
}

func TestAllocatePanelIconRelIDs_ValueChainMode(t *testing.T) {
	meta := valueChainMeta{supportCount: 2, primaryCount: 3, marginLabel: "Margin"}
	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx: 0,
						bounds:         types.BoundingBox{X: 0, Y: 0, Width: 10000000, Height: 5000000},
						panels: []nativePanelData{
							{title: "Support A"},
							{title: "Support B"},
							{title: "Primary A"},
							{title: "Primary B"},
							{title: "Primary C"},
						},
						valueChainMode: true,
						valueChainMeta: meta,
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
		t.Error("groupXML should be generated for valueChain mode")
	}

	// Should use homePlate geometry
	if !strings.Contains(inserts[0].groupXML, `prst="homePlate"`) {
		t.Error("groupXML should use homePlate geometry")
	}

	// Should contain Value Chain name
	if !strings.Contains(inserts[0].groupXML, "Value Chain") {
		t.Error("groupXML should contain 'Value Chain' name")
	}

	// Should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed); err != nil {
		t.Errorf("groupXML should be valid XML, got: %v", err)
	}
}
