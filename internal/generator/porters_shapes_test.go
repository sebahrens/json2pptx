package generator

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"

	"github.com/sebahrens/json2pptx/internal/types"
)

func TestIsPortersFiveForcesDiagram(t *testing.T) {
	tests := []struct {
		name     string
		spec     *types.DiagramSpec
		expected bool
	}{
		{
			name:     "porters_five_forces type",
			spec:     &types.DiagramSpec{Type: "porters_five_forces"},
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
			got := isPortersFiveForcesDiagram(tt.spec)
			if got != tt.expected {
				t.Errorf("isPortersFiveForcesDiagram() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParsePorterForces_Basic(t *testing.T) {
	data := map[string]any{
		"forces": []any{
			map[string]any{
				"type":      "rivalry",
				"label":     "Competitive Rivalry",
				"intensity": 0.85,
				"factors":   []any{"Price wars", "Feature parity"},
			},
			map[string]any{
				"type":      "new_entrants",
				"label":     "New Entrants",
				"intensity": 0.35,
				"factors":   []any{"High barriers"},
			},
			map[string]any{
				"type":      "substitutes",
				"label":     "Substitutes",
				"intensity": 0.3,
			},
			map[string]any{
				"type":      "suppliers",
				"intensity": 0.45,
			},
			map[string]any{
				"type":      "buyers",
				"label":     "Buyer Power",
				"intensity": 0.7,
				"factors":   []any{"Multi-vendor", "Price transparency"},
			},
		},
	}

	forces := parsePorterForces(data)
	if len(forces) != 5 {
		t.Fatalf("expected 5 forces, got %d", len(forces))
	}

	// Check rivalry
	if forces[0].forceType != porterRivalry {
		t.Errorf("expected rivalry, got %q", forces[0].forceType)
	}
	if forces[0].label != "Competitive Rivalry" {
		t.Errorf("expected label 'Competitive Rivalry', got %q", forces[0].label)
	}
	if forces[0].intensity != 0.85 {
		t.Errorf("expected intensity 0.85, got %f", forces[0].intensity)
	}
	if len(forces[0].factors) != 2 {
		t.Errorf("expected 2 factors, got %d", len(forces[0].factors))
	}

	// Check suppliers uses default label
	if forces[3].label != "Supplier Power" {
		t.Errorf("expected default label 'Supplier Power', got %q", forces[3].label)
	}
}

func TestParsePorterForces_Empty(t *testing.T) {
	forces := parsePorterForces(map[string]any{})
	if len(forces) != 0 {
		t.Errorf("expected 0 forces from empty data, got %d", len(forces))
	}
}

func TestParsePorterForces_NoForcesKey(t *testing.T) {
	forces := parsePorterForces(map[string]any{"other": "data"})
	if len(forces) != 0 {
		t.Errorf("expected 0 forces, got %d", len(forces))
	}
}

func TestParsePorterForces_InvalidType(t *testing.T) {
	// Forces without a type should be skipped
	data := map[string]any{
		"forces": []any{
			map[string]any{"label": "No Type"},
		},
	}
	forces := parsePorterForces(data)
	if len(forces) != 0 {
		t.Errorf("expected 0 forces (no type), got %d", len(forces))
	}
}

func TestGeneratePortersFiveGroupXML_Basic(t *testing.T) {
	panels := []nativePanelData{
		{title: "Competitive Rivalry", body: "- Price wars\n- Feature parity", value: "rivalry:0.85"},
		{title: "New Entrants", body: "- High barriers", value: "new_entrants:0.35"},
		{title: "Substitutes", body: "- Open source", value: "substitutes:0.30"},
		{title: "Supplier Power", body: "- Cloud providers", value: "suppliers:0.45"},
		{title: "Buyer Power", body: "- Multi-vendor\n- Price transparency", value: "buyers:0.70"},
	}
	bounds := types.BoundingBox{X: 100000, Y: 200000, Width: 9000000, Height: 6000000}

	result := generatePortersFiveGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("generatePortersFiveGroupXML returned empty string")
	}

	// Should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v\nXML:\n%s", err, result)
	}

	// Should contain group element
	if !strings.Contains(result, "p:grpSp") {
		t.Error("should contain p:grpSp group element")
	}

	// Should contain "Porters Five Forces" group name
	if !strings.Contains(result, "Porters Five Forces") {
		t.Error("should contain 'Porters Five Forces' group name")
	}

	// Should contain all force labels
	for _, label := range []string{"Competitive Rivalry", "New Entrants", "Substitutes", "Supplier Power", "Buyer Power"} {
		if !strings.Contains(result, label) {
			t.Errorf("should contain force label %q", label)
		}
	}

	// Should use scheme colors (accent1, accent3, accent5 for intensity mapping)
	for _, accent := range []string{"accent1", "accent3", "accent5"} {
		if !strings.Contains(result, accent) {
			t.Errorf("should contain scheme color %q", accent)
		}
	}

	// Should use roundRect geometry
	if !strings.Contains(result, `prst="roundRect"`) {
		t.Error("should use roundRect geometry")
	}

	// Should contain connectors
	if !strings.Contains(result, "p:cxnSp") {
		t.Error("should contain p:cxnSp connector elements")
	}

	// Should contain straightConnector1
	if !strings.Contains(result, `prst="straightConnector1"`) {
		t.Error("should use straightConnector1 geometry for connectors")
	}

	// Should contain triangle arrowheads
	if !strings.Contains(result, `type="triangle"`) {
		t.Error("should contain triangle arrowheads")
	}

	// Should contain factor text
	if !strings.Contains(result, "Price wars") {
		t.Error("should contain factor text 'Price wars'")
	}
	if !strings.Contains(result, "Multi-vendor") {
		t.Error("should contain factor text 'Multi-vendor'")
	}

	// Should use dk1 for text
	if !strings.Contains(result, `schemeClr val="dk1"`) {
		t.Error("text should use dk1 scheme color")
	}

	// Should contain intensity labels
	if !strings.Contains(result, "High") {
		t.Error("should contain 'High' intensity label")
	}
	if !strings.Contains(result, "Low") {
		t.Error("should contain 'Low' intensity label")
	}
}

func TestGeneratePortersFiveGroupXML_EmptyPanels(t *testing.T) {
	result := generatePortersFiveGroupXML(nil, types.BoundingBox{Width: 8000000, Height: 5000000}, 100)
	if result != "" {
		t.Error("should return empty for nil panels")
	}
}

func TestGeneratePortersFiveGroupXML_PartialForces(t *testing.T) {
	// Only 3 forces provided — should still generate valid XML
	panels := []nativePanelData{
		{title: "Rivalry", body: "", value: "rivalry:0.80"},
		{title: "New Entrants", body: "", value: "new_entrants:0.40"},
		{title: "Buyers", body: "", value: "buyers:0.60"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 9000000, Height: 6000000}

	result := generatePortersFiveGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("should generate XML for partial forces")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestGeneratePortersFiveGroupXML_NoFactors(t *testing.T) {
	panels := []nativePanelData{
		{title: "Competitive Rivalry", body: "", value: "rivalry:0.85"},
		{title: "New Entrants", body: "", value: "new_entrants:0.35"},
		{title: "Substitutes", body: "", value: "substitutes:0.30"},
		{title: "Supplier Power", body: "", value: "suppliers:0.45"},
		{title: "Buyer Power", body: "", value: "buyers:0.70"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 9000000, Height: 6000000}

	result := generatePortersFiveGroupXML(panels, bounds, 100)

	if result == "" {
		t.Fatal("should generate XML even without factors")
	}

	var parsed interface{}
	if err := xml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("generated XML should be valid, got: %v", err)
	}
}

func TestPorterIntensityColor(t *testing.T) {
	tests := []struct {
		intensity    float64
		wantScheme   string
	}{
		{0.0, "accent5"},   // Low
		{0.33, "accent5"},  // Low boundary
		{0.34, "accent3"},  // Medium
		{0.50, "accent3"},  // Medium
		{0.66, "accent3"},  // Medium boundary
		{0.67, "accent1"},  // High
		{1.0, "accent1"},   // High
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("intensity_%.2f", tt.intensity), func(t *testing.T) {
			scheme, _, _ := porterIntensityColor(tt.intensity)
			if scheme != tt.wantScheme {
				t.Errorf("porterIntensityColor(%f) scheme = %q, want %q", tt.intensity, scheme, tt.wantScheme)
			}
		})
	}
}

func TestPorterIntensityLabel(t *testing.T) {
	tests := []struct {
		intensity float64
		want      string
	}{
		{0.0, "Low"},
		{0.33, "Low"},
		{0.34, "Medium"},
		{0.66, "Medium"},
		{0.67, "High"},
		{1.0, "High"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("intensity_%.2f", tt.intensity), func(t *testing.T) {
			got := porterIntensityLabel(tt.intensity)
			if got != tt.want {
				t.Errorf("porterIntensityLabel(%f) = %q, want %q", tt.intensity, got, tt.want)
			}
		})
	}
}

func TestAllocatePanelIconRelIDs_PortersMode(t *testing.T) {
	panels := []nativePanelData{
		{title: "Competitive Rivalry", body: "- Price wars", value: "rivalry:0.85"},
		{title: "New Entrants", body: "- High barriers", value: "new_entrants:0.35"},
		{title: "Substitutes", body: "- Open source", value: "substitutes:0.30"},
		{title: "Supplier Power", body: "- Cloud providers", value: "suppliers:0.45"},
		{title: "Buyer Power", body: "- Multi-vendor", value: "buyers:0.70"},
	}

	ctx := &singlePassContext{
		SlideContext: SlideContext{
			panelShapeInserts: map[int][]panelShapeInsert{
				1: {
					{
						placeholderIdx:  0,
						bounds:          types.BoundingBox{X: 0, Y: 0, Width: 9000000, Height: 6000000},
						panels:          panels,
						portersFiveMode: true,
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

	if inserts[0].groupXML == "" {
		t.Error("groupXML should be generated for Porter's Five Forces mode")
	}

	// Should use roundRect (Porter's style)
	if !strings.Contains(inserts[0].groupXML, `prst="roundRect"`) {
		t.Error("Porters groupXML should use roundRect geometry")
	}

	// Should contain Porters Five Forces group name
	if !strings.Contains(inserts[0].groupXML, "Porters Five Forces") {
		t.Error("Porters groupXML should contain 'Porters Five Forces' name")
	}

	// Should contain connectors
	if !strings.Contains(inserts[0].groupXML, "p:cxnSp") {
		t.Error("Porters groupXML should contain connectors")
	}

	// Should be well-formed XML
	var parsed interface{}
	if err := xml.Unmarshal([]byte(inserts[0].groupXML), &parsed); err != nil {
		t.Errorf("Porters groupXML should be valid XML, got: %v", err)
	}
}

func TestPorterDefaultLabel(t *testing.T) {
	tests := []struct {
		ft   porterForceType
		want string
	}{
		{porterRivalry, "Competitive Rivalry"},
		{porterNewEntrant, "Threat of New Entrants"},
		{porterSubstitute, "Threat of Substitutes"},
		{porterSupplier, "Supplier Power"},
		{porterBuyer, "Buyer Power"},
		{porterForceType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.ft), func(t *testing.T) {
			got := porterDefaultLabel(tt.ft)
			if got != tt.want {
				t.Errorf("porterDefaultLabel(%q) = %q, want %q", tt.ft, got, tt.want)
			}
		})
	}
}
