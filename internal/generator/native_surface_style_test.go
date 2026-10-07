package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-amtkg: every native framework draws the patterns' surface
// — square corners, neutral dk1 tints, one accent — and style.colors is the
// opt-in back to accent-tinted cards.

func nativeSurfaceSpecs() map[string]*types.DiagramSpec {
	panels := []any{
		map[string]any{"title": "Assess", "value": "41%", "body": "- Map the estate"},
		map[string]any{"title": "Design", "value": "12", "body": "- Target model"},
		map[string]any{"title": "Deliver", "value": "3x", "body": "- Migrate in waves"},
	}
	list := func(items ...string) []any {
		out := make([]any, len(items))
		for i, s := range items {
			out[i] = s
		}
		return out
	}
	return map[string]*types.DiagramSpec{
		"swot": {Type: "swot", Data: map[string]any{
			"strengths": list("Brand"), "weaknesses": list("Cost"), "opportunities": list("Asia"), "threats": list("Rates"),
		}},
		"pestel": {Type: "pestel", Data: map[string]any{
			"political": list("Trade"), "economic": list("Rates"), "social": list("Hybrid work"),
			"technological": list("GenAI"), "environmental": list("Scope 3"), "legal": list("AI Act"),
		}},
		"business_model_canvas": {Type: "business_model_canvas", Data: map[string]any{
			"key_partners": list("Cloud"), "key_activities": list("Build"), "key_resources": list("Team"),
			"value_proposition": list("Speed"), "customer_relationships": list("Self-serve"), "channels": list("Direct"),
			"customer_segments": list("Mid-market"), "cost_structure": list("Infra"), "revenue_streams": list("Subscriptions"),
		}},
		"panel_layout": {Type: "panel_layout", Data: map[string]any{"layout": "columns", "panels": panels}},
		"icon_rows":    {Type: "icon_rows", Data: map[string]any{"panels": panels}},
		"stat_cards":   {Type: "stat_cards", Data: map[string]any{"panels": panels}},
		"kpi_dashboard": {Type: "kpi_dashboard", Data: map[string]any{"metrics": []any{
			map[string]any{"label": "Revenue", "value": "4.2M", "delta": "+12%", "trend": "up"},
			map[string]any{"label": "Churn", "value": "3.1%", "delta": "+0.4", "trend": "down"},
		}}},
		"value_chain": {Type: "value_chain", Data: map[string]any{
			"primary_activities": []any{
				map[string]any{"name": "Inbound", "items": list("Sourcing")},
				map[string]any{"name": "Operations", "items": list("Assembly")},
				map[string]any{"name": "Service", "items": list("Support")},
			},
			"support_activities": []any{
				map[string]any{"name": "Infrastructure", "items": list("Finance")},
				map[string]any{"name": "Technology", "items": list("R&D")},
			},
			"margin": "Margin",
		}},
		"porters_five_forces": {Type: "porters_five_forces", Data: map[string]any{"forces": []any{
			map[string]any{"type": "rivalry", "label": "Rivalry", "intensity": 0.9},
			map[string]any{"type": "new_entrants", "label": "Entrants"},
			map[string]any{"type": "substitutes", "label": "Substitutes", "intensity": 0.5},
			map[string]any{"type": "suppliers", "label": "Suppliers", "intensity": 0.2},
			map[string]any{"type": "buyers", "label": "Buyers"},
		}}},
	}
}

func renderNativeSurfaceSpec(t *testing.T, spec *types.DiagramSpec) string {
	t.Helper()
	env := nativeDiagramEnv{fontName: "Arial", themeColors: []types.ThemeColor{
		{Name: "dk1", RGB: "#111111"}, {Name: "lt1", RGB: "#FFFFFF"}, {Name: "dk2", RGB: "#1B2A4A"}, {Name: "lt2", RGB: "#EEEEEE"},
		{Name: "accent1", RGB: "#2B4C8C"}, {Name: "accent2", RGB: "#E8A838"}, {Name: "accent3", RGB: "#26A69A"},
		{Name: "accent4", RGB: "#7E57C2"}, {Name: "accent5", RGB: "#EF5350"}, {Name: "accent6", RGB: "#66BB6A"},
	}}
	layout, err := layoutNativeDiagram(spec, types.BoundingBox{X: 500000, Y: 1500000, Width: 11000000, Height: 4800000}, env, nativeDiagramSite{})
	if err != nil {
		t.Fatalf("%s: layout: %v", spec.Type, err)
	}
	xml := renderNativeInsert(&layout.insert, 100, env)
	if xml == "" {
		t.Fatalf("%s: empty group", spec.Type)
	}
	return xml
}

var nativeSchemeRE = regexp.MustCompile(`schemeClr val="(accent[1-6])"`)

func TestNativeFrameworksShareOneSurface(t *testing.T) {
	// Types whose extra hues are data: trend ink on KPI / stat deltas.
	semantic := map[string]map[string]bool{
		"kpi_dashboard": {"accent2": true, "accent6": true},
		"stat_cards":    {"accent2": true, "accent6": true},
	}
	for name, spec := range nativeSurfaceSpecs() {
		t.Run(name, func(t *testing.T) {
			xml := renderNativeSurfaceSpec(t, spec)
			if strings.Contains(xml, "roundRect") {
				t.Error("native cards are square-cornered; found roundRect")
			}
			// pestel and kpi_dashboard are open: text on the page, a rule
			// or a divider, and no tile (go-slide-creator-w107j).
			open := name == "pestel" || name == "kpi_dashboard"
			if has := strings.Contains(xml, `<a:schemeClr val="dk1"><a:lumMod val="4000"/><a:lumOff val="96000"/>`); has == open {
				t.Errorf("card on the neutral 4%% surface: %t, want %t", has, !open)
			}
			for _, m := range nativeSchemeRE.FindAllStringSubmatch(xml, -1) {
				if m[1] != "accent1" && !semantic[name][m[1]] {
					t.Errorf("uses %s; the default is one accent plus neutrals", m[1])
				}
			}
			if strings.Contains(xml, `schemeClr val="tx1"`) {
				t.Error("found a tx1 outline; filled surfaces carry no outline")
			}
		})
	}
}

func TestNativeFrameworksStyleColorsRestoreAccentTints(t *testing.T) {
	for _, name := range []string{"swot", "pestel", "business_model_canvas", "panel_layout", "icon_rows", "stat_cards", "kpi_dashboard", "value_chain"} {
		t.Run(name, func(t *testing.T) {
			spec := nativeSurfaceSpecs()[name]
			spec.Style = &types.DiagramStyle{Colors: []string{"accent3", "accent4"}}
			xml := renderNativeSurfaceSpec(t, spec)
			for _, want := range []string{`<a:schemeClr val="accent3"><a:lumMod val="20000"/><a:lumOff val="80000"/>`, `<a:schemeClr val="accent4"><a:lumMod val="20000"/><a:lumOff val="80000"/>`} {
				if !strings.Contains(xml, want) {
					t.Errorf("style.colors not applied: missing %s", want)
				}
			}
			if strings.Contains(xml, "roundRect") {
				t.Error("authored colours keep the square corners")
			}
		})
	}
}
