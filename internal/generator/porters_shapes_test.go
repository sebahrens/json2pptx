package generator

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
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

func TestParsePorterForces_ObjectKeyed(t *testing.T) {
	// Field-test 1.3 shape: top-level force keys with {label, intensity, description}.
	data := map[string]any{
		"rivalry":        map[string]any{"label": "Competitive Rivalry", "intensity": 0.85, "description": "Price wars"},
		"new_entrants":   map[string]any{"label": "New Entrants", "intensity": 0.35, "description": "High barriers"},
		"substitutes":    map[string]any{"label": "Substitutes", "intensity": 0.30, "description": "Open source"},
		"buyer_power":    map[string]any{"label": "Buyer Power", "intensity": 0.70, "description": "Multi-vendor"},
		"supplier_power": map[string]any{"label": "Supplier Power", "intensity": 0.45, "description": "Cloud providers"},
	}

	forces := parsePorterForces(data)
	if len(forces) != 5 {
		t.Fatalf("expected 5 forces from object-keyed data, got %d", len(forces))
	}

	// Emitted in canonical layout order: rivalry, new_entrants, substitutes, suppliers, buyers.
	wantOrder := []porterForceType{porterRivalry, porterNewEntrant, porterSubstitute, porterSupplier, porterBuyer}
	for i, want := range wantOrder {
		if forces[i].forceType != want {
			t.Errorf("force[%d] type = %q, want %q", i, forces[i].forceType, want)
		}
	}

	// buyer_power should map to the buyers force type.
	if forces[4].forceType != porterBuyer {
		t.Errorf("expected buyer_power to map to porterBuyer, got %q", forces[4].forceType)
	}
	if forces[0].intensity != 0.85 {
		t.Errorf("expected rivalry intensity 0.85, got %f", forces[0].intensity)
	}
	// description should become a single factor line.
	if len(forces[0].factors) != 1 || forces[0].factors[0] != "Price wars" {
		t.Errorf("expected description as single factor, got %v", forces[0].factors)
	}
}

func TestParsePorterForces_ObjectKeyedFactorsList(t *testing.T) {
	// Object-keyed with an explicit factors array takes precedence over description.
	data := map[string]any{
		"rivalry": map[string]any{
			"intensity":   0.9,
			"description": "ignored when factors present",
			"factors":     []any{"Price wars", "Feature parity"},
		},
	}
	forces := parsePorterForces(data)
	if len(forces) != 1 {
		t.Fatalf("expected 1 force, got %d", len(forces))
	}
	if forces[0].label != "Competitive Rivalry" {
		t.Errorf("expected default label, got %q", forces[0].label)
	}
	if len(forces[0].factors) != 2 {
		t.Errorf("expected 2 factors from list, got %v", forces[0].factors)
	}
}

func TestParsePorterForces_ObjectKeyedSynonyms(t *testing.T) {
	data := map[string]any{
		"competitive_rivalry":           map[string]any{"intensity": 0.8},
		"threat_of_new_entrants":        map[string]any{"intensity": 0.4},
		"bargaining_power_of_buyers":    map[string]any{"intensity": 0.6},
		"bargaining_power_of_suppliers": map[string]any{"intensity": 0.5},
	}
	forces := parsePorterForces(data)
	if len(forces) != 4 {
		t.Fatalf("expected 4 forces from synonym keys, got %d", len(forces))
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

	ctx.finalizePanelGroupXML()

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

// go-slide-creator-2zej: connectorPairs assumed rect connection site 1 = right
// and 3 = left, but OOXML lists rect sites counter-clockwise from the top
// (0 = top, 1 = left, 2 = bottom, 3 = right). Both horizontal connectors
// therefore attached to the FAR side of their box and ran straight through its
// text into Rivalry. Sites must come from pptx.ConnectionSiteIndex so this
// cannot drift from the shapegrid connector code again.
func TestPorterConnectorSites(t *testing.T) {
	xml := generatePorterFiveForcesXMLForTest(t)

	type link struct{ from, to string }
	// The side each connector must leave / enter, expressed as the canonical
	// site index for a roundRect.
	want := map[link][2]int{
		{"Threat of New Entrants", "Competitive Rivalry"}: {
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideBottom),
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideTop),
		},
		{"Threat of Substitutes", "Competitive Rivalry"}: {
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideTop),
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideBottom),
		},
		{"Supplier Power", "Competitive Rivalry"}: {
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideRight),
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideLeft),
		},
		{"Buyer Power", "Competitive Rivalry"}: {
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideLeft),
			pptx.ConnectionSiteIndex(pptx.GeomRoundRect, pptx.SideRight),
		},
	}

	names := map[string]string{}
	for _, m := range regexp.MustCompile(`<p:cNvPr id="(\d+)" name="Porter ([^"]*)"`).FindAllStringSubmatch(xml, -1) {
		names[m[1]] = m[2]
	}

	starts := regexp.MustCompile(`<a:stCxn id="(\d+)" idx="(\d+)"/>`).FindAllStringSubmatch(xml, -1)
	ends := regexp.MustCompile(`<a:endCxn id="(\d+)" idx="(\d+)"/>`).FindAllStringSubmatch(xml, -1)
	if len(starts) != 4 || len(ends) != 4 {
		t.Fatalf("expected 4 connectors, got %d stCxn / %d endCxn", len(starts), len(ends))
	}

	got := map[link][2]int{}
	for i := range starts {
		fromName := names[starts[i][1]]
		toName := names[ends[i][1]]
		fromIdx, _ := strconv.Atoi(starts[i][2])
		toIdx, _ := strconv.Atoi(ends[i][2])
		got[link{fromName, toName}] = [2]int{fromIdx, toIdx}
	}

	for l, sites := range want {
		g, ok := got[l]
		if !ok {
			t.Errorf("no connector from %q to %q; got %v", l.from, l.to, got)
			continue
		}
		if g != sites {
			t.Errorf("%s -> %s attaches at sites %v, want %v (a wrong site routes the line through the box text)",
				l.from, l.to, g, sites)
		}
	}
}

// The factor bullets must name their buFont: a <a:buChar> resolved in the theme
// font can fall back to a different glyph (the reported stray "*").
func TestPorterFactorBulletsDeclareBuFont(t *testing.T) {
	xml := generatePorterFiveForcesXMLForTest(t)

	chars := regexp.MustCompile(`<a:buChar char="([^"]*)"/>`).FindAllStringSubmatch(xml, -1)
	if len(chars) == 0 {
		t.Fatal("no bullet characters emitted for the factor lists")
	}
	for _, c := range chars {
		if c[1] != pptx.DefaultBulletChar {
			t.Errorf("bullet char = %q, want %q", c[1], pptx.DefaultBulletChar)
		}
	}
	fonts := regexp.MustCompile(`<a:buFont typeface="([^"]*)"/>`).FindAllStringSubmatch(xml, -1)
	if len(fonts) != len(chars) {
		t.Errorf("%d buChar but %d buFont — a bullet without a buFont may substitute a different glyph",
			len(chars), len(fonts))
	}
	for _, f := range fonts {
		if f[1] != pptx.DefaultBulletFont {
			t.Errorf("buFont = %q, want %q", f[1], pptx.DefaultBulletFont)
		}
	}
}

// generatePorterFiveForcesXMLForTest builds the native Porter shape group XML
// for a complete five-force spec.
func generatePorterFiveForcesXMLForTest(t *testing.T) string {
	t.Helper()
	panels := []nativePanelData{
		{title: "Competitive Rivalry", value: string(porterRivalry) + ":0.5", body: "- Three scaled players"},
		{title: "Supplier Power", value: string(porterSupplier) + ":0.4", body: "- Two key vendors"},
		{title: "Buyer Power", value: string(porterBuyer) + ":0.7", body: "- Concentrated demand"},
		{title: "Threat of New Entrants", value: string(porterNewEntrant) + ":0.3", body: "- High capital need"},
		{title: "Threat of Substitutes", value: string(porterSubstitute) + ":0.5", body: "- In-house build"},
	}
	bounds := types.BoundingBox{X: 0, Y: 0, Width: 9144000, Height: 5143500}
	xml := generatePortersFiveGroupXML(panels, bounds, 100)
	if xml == "" {
		t.Fatal("generatePortersFiveGroupXML returned empty XML")
	}
	return xml
}
