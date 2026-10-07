package generator

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/types"
	"github.com/sebahrens/json2pptx/svggen"
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
	if forces[0].intensity == nil || *forces[0].intensity != 0.85 {
		t.Errorf("expected intensity 0.85, got %v", forces[0].intensity)
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
	if forces[0].intensity == nil || *forces[0].intensity != 0.85 {
		t.Errorf("expected rivalry intensity 0.85, got %v", forces[0].intensity)
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

	result := generatePortersFiveGroupXML(panels, bounds, 100, nativeDiagramEnv{})

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

	// Scored forces use one accent with ordered lightness, not categorical hues.
	if !strings.Contains(result, `val="accent1"`) {
		t.Error("should contain accent1 fill")
	}
	for _, tint := range []string{`lumMod val="50000"`, `lumMod val="20000"`, `lumMod val="10000"`} {
		if !strings.Contains(result, tint) {
			t.Errorf("should contain intensity tint %q", tint)
		}
	}

	// One surface style: square corners, as the patterns (go-slide-creator-amtkg).
	if strings.Contains(result, "roundRect") || !strings.Contains(result, `prst="rect"`) {
		t.Error("cards should be square-cornered rects, not roundRect")
	}

	// The forces point at rivalry with their shape, not with connector
	// lines (go-slide-creator-av25u).
	if strings.Contains(result, "p:cxnSp") {
		t.Error("should contain no connector: each force's point carries the direction")
	}
	if got := strings.Count(result, " Point\""); got != 4 {
		t.Errorf("should contain 4 force points, got %d", got)
	}
	if !strings.Contains(result, `prst="triangle"`) || !strings.Contains(result, `prst="homePlate"`) {
		t.Error("points should be triangle (above / below) and homePlate (left / right) presets")
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
	result := generatePortersFiveGroupXML(nil, types.BoundingBox{Width: 8000000, Height: 5000000}, 100, nativeDiagramEnv{})
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

	result := generatePortersFiveGroupXML(panels, bounds, 100, nativeDiagramEnv{})

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

	result := generatePortersFiveGroupXML(panels, bounds, 100, nativeDiagramEnv{})

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
		intensity float64
		wantMod   int
		wantOff   int
	}{
		{0.0, 10000, 90000},  // Low: Lighter 90%
		{0.33, 10000, 90000}, // Low boundary
		{0.34, 20000, 80000}, // Medium: Lighter 80%
		{0.50, 20000, 80000}, // Medium
		{0.66, 20000, 80000}, // Medium boundary
		{0.67, 50000, 50000}, // High: Lighter 50%
		{1.0, 50000, 50000},  // High
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("intensity_%.2f", tt.intensity), func(t *testing.T) {
			scheme, mod, off := porterIntensityColor(&tt.intensity)
			if scheme != "accent1" || mod != tt.wantMod || off != tt.wantOff {
				t.Errorf("porterIntensityColor(%f) = (%q, %d, %d), want (accent1, %d, %d)", tt.intensity, scheme, mod, off, tt.wantMod, tt.wantOff)
			}
		})
	}
	// An unscored force sits on the shared neutral card, not on an outlined
	// lt2 box (go-slide-creator-amtkg).
	wantMod, wantOff := patterns.NeutralSurfaceMods(patterns.NeutralTint4)
	if scheme, mod, off := porterIntensityColor(nil); scheme != porterNeutralScheme || mod != wantMod || off != wantOff {
		t.Errorf("unstated intensity = (%q, %d, %d), want (%q, %d, %d)", scheme, mod, off, porterNeutralScheme, wantMod, wantOff)
	}
}

func TestPorterIntensityColor_ThemeLightnessOrdering(t *testing.T) {
	white := svggen.MustParseColor("#FFFFFF")
	black := svggen.MustParseColor("#000000")
	for name, hex := range map[string]string{
		"forest-green": "#2E7D32", "midnight-blue": "#2E5090",
		"modern-template": "#B5485A", "warm-coral": "#E64A19",
	} {
		t.Run(name, func(t *testing.T) {
			base := svggen.MustParseColor(hex)
			lightness := make([]svggen.Color, 3)
			for i, intensity := range []float64{0.9, 0.5, 0.1} {
				_, mod, _ := porterIntensityColor(&intensity)
				lightness[i] = patterns.EffectiveColorMods(base, diagramTintMods(mod, 100000-mod), white)
			}
			if !(lightness[0].Luminance() < lightness[1].Luminance() && lightness[1].Luminance() < lightness[2].Luminance()) {
				t.Errorf("intensity lightness not ordered high→medium→low: %s, %s, %s", lightness[0].Hex(), lightness[1].Hex(), lightness[2].Hex())
			}
			if contrast := lightness[0].ContrastWith(lightness[2]); contrast < 1.5 {
				t.Errorf("high and low tints too similar: %.2f:1", contrast)
			}
			if contrast := black.ContrastWith(lightness[0]); contrast < 4.5 {
				t.Errorf("high-intensity box loses dark-text contrast: %.2f:1", contrast)
			}
		})
	}
}

func TestPorterTintAndIntensityTextOnSaturatedAccent(t *testing.T) {
	theme := []types.ThemeColor{
		{Name: "lt1", RGB: "FFFFFF"}, {Name: "dk1", RGB: "000000"},
		{Name: "dk2", RGB: "1B2A4A"}, {Name: "accent1", RGB: "0097A7"},
	}
	base := svggen.MustParseColor("#0097A7")
	white := svggen.MustParseColor("#FFFFFF")
	for _, intensity := range []float64{0.1, 0.5, 0.9} {
		force := porterForceData{label: "Rivalry", intensity: &intensity}
		xml := porterTestBoxXML(force, 1, theme)
		_, retained, off := porterIntensityColor(&intensity)
		if !strings.Contains(xml, fmt.Sprintf(`<a:lumMod val="%d"/><a:lumOff val="%d"/>`, retained, off)) || strings.Contains(xml, "<a:tint") {
			t.Errorf("intensity %.1f: expected a lumMod / lumOff tint, got %s", intensity, xml)
		}
		fill := patterns.EffectiveColorMods(base, diagramTintMods(retained, off), white)
		textScheme := porterIntensityTextColor("accent1", retained, off, theme)
		text := svggen.MustParseColor(resolveSchemeColorToHex(textScheme, theme))
		if ratio := text.ContrastWith(fill); ratio < svggen.WCAGAANormal {
			t.Errorf("intensity %.1f: text %s on %s has %.2f:1 contrast", intensity, textScheme, fill.Hex(), ratio)
		}
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

	// One surface style: square corners, as the patterns (go-slide-creator-amtkg).
	if strings.Contains(inserts[0].groupXML, "roundRect") || !strings.Contains(inserts[0].groupXML, `prst="rect"`) {
		t.Error("cards should be square-cornered rects, not roundRect")
	}

	// Should contain Porters Five Forces group name
	if !strings.Contains(inserts[0].groupXML, "Porters Five Forces") {
		t.Error("Porters groupXML should contain 'Porters Five Forces' name")
	}

	// The forces' points replace the connectors.
	if strings.Contains(inserts[0].groupXML, "p:cxnSp") || !strings.Contains(inserts[0].groupXML, " Point\"") {
		t.Error("Porters groupXML should contain force points and no connectors")
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

// Each peripheral force is a pentagon aimed at rivalry: its point sits on the
// side facing the centre, spans that side, takes the force's own fill and
// stops just short of the rivalry block (go-slide-creator-av25u). The thin
// connector arrows this replaces attached to the far side of their boxes once
// (go-slide-creator-2zej) and vanished in PowerPoint once
// (go-slide-creator-7ec2s); a shape has neither failure.
func TestPorterForcesPointAtRivalry(t *testing.T) {
	spec := porterTestSpec(3, "A factor")
	bounds := ptBounds(900, 400)
	layout := layoutPorter(porterForcesFromPanels(porterPanels(spec)), bounds, nativeDiagramEnv{fontName: "Arial"})
	var center pptx.RectEmu
	for _, box := range layout.boxes {
		if box.force.forceType == porterRivalry {
			center = box.rect
		}
	}
	for _, box := range layout.boxes {
		if box.force.forceType == porterRivalry {
			continue
		}
		tip, ok := porterTipRect(box, center)
		if !ok {
			t.Errorf("%s: no room for a point", box.force.forceType)
			continue
		}
		r := box.rect
		switch box.force.forceType {
		case porterNewEntrant:
			if tip.X != r.X || tip.CX != r.CX || tip.Y+tip.CY > center.Y || tip.Y+tip.CY < center.Y-2*porterTipGapEMU-porterTipMaxDepthEMU || tip.Y > r.Y+r.CY {
				t.Errorf("new entrants point %+v does not hang under %+v toward %+v", tip, r, center)
			}
		case porterSubstitute:
			if tip.X != r.X || tip.CX != r.CX || tip.Y < center.Y+center.CY || tip.Y+tip.CY < r.Y {
				t.Errorf("substitutes point %+v does not rise from %+v toward %+v", tip, r, center)
			}
		case porterSupplier:
			if tip.Y != r.Y || tip.CY != r.CY || tip.X+tip.CX > center.X || tip.X > r.X+r.CX {
				t.Errorf("suppliers point %+v does not extend %+v toward %+v", tip, r, center)
			}
		case porterBuyer:
			if tip.Y != r.Y || tip.CY != r.CY || tip.X < center.X+center.CX || tip.X+tip.CX < r.X {
				t.Errorf("buyers point %+v does not extend %+v toward %+v", tip, r, center)
			}
		}
	}

	xml := generatePorterFiveForcesXMLForTest(t)
	flips := map[string]string{}
	for _, m := range regexp.MustCompile(`(?s)name="Porter ([^"]*) Point"/>.*?<a:xfrm([^>]*)>`).FindAllStringSubmatch(xml, -1) {
		flips[m[1]] = strings.TrimSpace(m[2])
	}
	want := map[string]string{
		"Threat of New Entrants": `flipV="1"`, // triangle turned to point down
		"Threat of Substitutes":  "",
		"Supplier Power":         "",
		"Buyer Power":            `flipH="1"`, // pentagon turned to point left
	}
	for name, flip := range want {
		got, ok := flips[name]
		if !ok || got != flip {
			t.Errorf("%s point: xfrm flags %q (present %t), want %q", name, got, ok, flip)
		}
	}
}

// Rivalry is the one solid accent block; its text takes the ink that reads
// on it.
func TestPorterRivalryIsTheSolidAccent(t *testing.T) {
	theme := portersTestTheme()
	intensity := 0.2
	for _, f := range []porterForceData{
		{forceType: porterRivalry, label: "Competitive Rivalry", factors: []string{"Price wars"}},
		{forceType: porterRivalry, label: "Competitive Rivalry", intensity: &intensity},
	} {
		text, _, _ := porterBoxText(f, true, false, 2000000, pptx.ShapeTextInsetEMU, nativeDiagramEnv{themeColors: theme})
		xml := generatePorterForceBoxXML(porterBox{force: f, rect: pptx.RectEmu{CX: 2000000, CY: 1000000}, text: text}, 5, nativeDiagramEnv{themeColors: theme})
		fill := regexp.MustCompile(`(?s)</a:prstGeom>\s*<a:solidFill>(.*?)</a:solidFill>`).FindStringSubmatch(xml)
		if fill == nil || !strings.Contains(fill[1], `val="accent1"`) || strings.Contains(fill[1], "lumOff") {
			t.Errorf("rivalry fill = %v, want the solid accent", fill)
		}
		ink := nativeSurface{colors: theme}.emphasis().ink
		if n := strings.Count(xml, `<a:schemeClr val="`+ink+`"/>`); n < len(text.Paragraphs) {
			t.Errorf("rivalry text is not set in the emphasis ink %s throughout:\n%s", ink, xml)
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
	xml := generatePortersFiveGroupXML(panels, bounds, 100, nativeDiagramEnv{})
	if xml == "" {
		t.Fatal("generatePortersFiveGroupXML returned empty XML")
	}
	return xml
}

// porterTestBoxXML draws one stacked peripheral force box, 2000000 x 1000000
// EMU at the origin, the way the layout writes it.
func porterTestBoxXML(f porterForceData, shapeID uint32, theme []types.ThemeColor) string {
	text, _, _ := porterBoxText(f, false, false, 2000000, pptx.ShapeTextInsetEMU, nativeDiagramEnv{themeColors: theme})
	return generatePorterForceBoxXML(porterBox{
		force: f, rect: pptx.RectEmu{CX: 2000000, CY: 1000000}, text: text,
	}, shapeID, nativeDiagramEnv{themeColors: theme})
}
