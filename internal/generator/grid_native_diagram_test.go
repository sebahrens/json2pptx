package generator

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// nativePlacementCase is one row of the native placement matrix: a short
// fixture of a native diagram type and the authored text it must keep.
type nativePlacementCase struct {
	spec   *types.DiagramSpec
	labels []string
}

// nativePlacementMatrix covers every native diagram type — the ten canonical
// types that had no bounded region dispatch before go-slide-creator-3grgs,
// SWOT and five forces (go-slide-creator-ngbnf), and the three panel aliases —
// with the short fixtures of the 2026-10-02 layout-composition sweep.
func nativePlacementMatrix() map[string]nativePlacementCase {
	panels := []any{
		map[string]any{"title": "Growth", "body": "Scale"},
		map[string]any{"title": "Control", "body": "Manage"},
	}
	return map[string]nativePlacementCase{
		"business_model_canvas": {&types.DiagramSpec{Type: "business_model_canvas", Data: map[string]any{
			"key_partners": []any{"Partners"}, "key_activities": []any{"Develop"}, "key_resources": []any{"People"},
			"value_proposition": []any{"Value"}, "customer_relationships": []any{"Support"}, "channels": []any{"Sales"},
			"customer_segments": []any{"Teams"}, "cost_structure": []any{"Payroll"}, "revenue_streams": []any{"Licences"},
		}}, []string{"Partners", "Develop", "People", "Value", "Support", "Sales", "Teams", "Payroll", "Licences"}},
		"heatmap": {&types.DiagramSpec{Type: "heatmap", Data: map[string]any{
			"row_labels": []any{"North", "South"}, "col_labels": []any{"Q1", "Q2"},
			"values": []any{[]any{1.0, 2.0}, []any{3.0, 4.0}},
		}}, []string{"North", "South", "Q1", "Q2"}},
		"house_diagram": {&types.DiagramSpec{Type: "house_diagram", Data: map[string]any{
			"roof": "Vision", "foundation": "People",
			"sections": []any{map[string]any{"label": "Growth", "items": []any{"Scale"}}},
		}}, []string{"Vision", "People", "Growth", "Scale"}},
		"kpi_dashboard": {&types.DiagramSpec{Type: "kpi_dashboard", Data: map[string]any{
			"metrics": []any{map[string]any{"label": "Revenue", "value": "12M"}, map[string]any{"label": "Margin", "value": "42%"}},
		}}, []string{"Revenue", "12M", "Margin", "42%"}},
		"nine_box_talent": {&types.DiagramSpec{Type: "nine_box_talent", Data: map[string]any{
			"employees": []any{map[string]any{"name": "Alice", "performance": 2.0, "potential": 2.0}},
		}}, []string{"Alice"}},
		"panel_layout": {&types.DiagramSpec{Type: "panel_layout", Data: map[string]any{"layout": "columns", "panels": panels}},
			[]string{"Growth", "Scale", "Control", "Manage"}},
		"pestel": {&types.DiagramSpec{Type: "pestel", Data: map[string]any{
			"political": []any{"Trade"}, "economic": []any{"Growth"}, "social": []any{"Demand"},
			"technological": []any{"AI"}, "environmental": []any{"Energy"}, "legal": []any{"Privacy"},
		}}, []string{"Trade", "Growth", "Demand", "AI", "Energy", "Privacy"}},
		"porters_five_forces": {&types.DiagramSpec{Type: "porters_five_forces", Data: map[string]any{
			"rivalry":      map[string]any{"label": "Rivalry", "factors": []any{"R-item"}},
			"new_entrants": map[string]any{"label": "New entrants", "factors": []any{"E-item"}},
			"substitutes":  map[string]any{"label": "Substitutes", "factors": []any{"S-item"}},
			"suppliers":    map[string]any{"label": "Suppliers", "factors": []any{"P-item"}},
			"buyers":       map[string]any{"label": "Buyers", "factors": []any{"B-item"}},
		}}, []string{"R-item", "E-item", "S-item", "P-item", "B-item"}},
		"process_flow": {&types.DiagramSpec{Type: "process_flow", Data: map[string]any{
			"steps": []any{"Design", "Build", "Launch"},
		}}, []string{"Design", "Build", "Launch"}},
		"pyramid": {&types.DiagramSpec{Type: "pyramid", Data: map[string]any{
			"levels": []any{map[string]any{"label": "Strategy"}, map[string]any{"label": "Delivery"}, map[string]any{"label": "Operations"}},
		}}, []string{"Strategy", "Delivery", "Operations"}},
		"swot": {&types.DiagramSpec{Type: "swot", Data: map[string]any{
			"strengths": []any{"Talent"}, "weaknesses": []any{"Scale"}, "opportunities": []any{"Growth"}, "threats": []any{"Rivals"},
		}}, []string{"Talent", "Scale", "Growth", "Rivals"}},
		"value_chain": {&types.DiagramSpec{Type: "value_chain", Data: map[string]any{
			"primary": []any{
				map[string]any{"name": "Inbound", "activities": []any{"Receive"}},
				map[string]any{"name": "Operations", "activities": []any{"Build"}},
				map[string]any{"name": "Outbound", "activities": []any{"Ship"}},
			},
			"support": []any{map[string]any{"name": "People", "activities": []any{"Train"}}},
		}}, []string{"Inbound", "Operations", "Outbound", "People"}},
		"icon_columns": {&types.DiagramSpec{Type: "icon_columns", Data: map[string]any{"panels": panels}},
			[]string{"Growth", "Scale", "Control", "Manage"}},
		"icon_rows": {&types.DiagramSpec{Type: "icon_rows", Data: map[string]any{"panels": panels}},
			[]string{"Growth", "Scale", "Control", "Manage"}},
		"stat_cards": {&types.DiagramSpec{Type: "stat_cards", Data: map[string]any{"panels": []any{
			map[string]any{"title": "Revenue", "value": "12M"}, map[string]any{"title": "Margin", "value": "42%"},
		}}}, []string{"Revenue", "12M", "Margin", "42%"}},
	}
}

// go-slide-creator-3grgs: every native diagram type renders in a region —
// the 70% column of the PESTEL-beside-commentary reproduction — as one
// editable shape group inside the region, within the shape-ID range it
// reserved, carrying its alt text and every authored label.
func TestGenerateGridNativeDiagram_PlacementMatrix(t *testing.T) {
	region := NativeDiagramRegion{
		Bounds:      types.BoundingBox{X: 457200, Y: 1600200, Width: 7680960, Height: 4572000},
		ThemeColors: []types.ThemeColor{{Name: "accent1", RGB: "1F4E79"}},
		FontName:    "Calibri",
		Path:        "/slides/0/shape_grid/rows/0/cells/0/diagram",
	}
	idRe := regexp.MustCompile(`<p:cNvPr id="(\d+)"`)
	offRe := regexp.MustCompile(`<a:off x="(-?\d+)" y="(-?\d+)"/>`)
	for name, tc := range nativePlacementMatrix() {
		t.Run(name, func(t *testing.T) {
			if !IsGridNativeDiagram(tc.spec) {
				t.Fatalf("%s is not grid-native", name)
			}
			const base = 300
			reserved := 0
			d, err := GenerateGridNativeDiagram(tc.spec, region, "Alt for "+name, func(n int) uint32 {
				reserved = n
				return base
			})
			if err != nil {
				t.Fatal(err)
			}
			xml := d.XML
			if !strings.HasPrefix(xml, "<p:grpSp") {
				t.Fatalf("want one group, got %.80s", xml)
			}
			if strings.Contains(xml, "<p:pic") {
				t.Error("a native diagram must be editable shapes, not a picture")
			}
			if !strings.Contains(xml, `descr="Alt for `+name+`"`) {
				t.Error("group carries no alt text")
			}
			seen := map[int]bool{}
			for _, m := range idRe.FindAllStringSubmatch(xml, -1) {
				id, _ := strconv.Atoi(m[1])
				if id < base || id >= base+reserved {
					t.Errorf("shape id %d outside reserved range [%d,%d)", id, base, base+reserved)
				}
				if seen[id] {
					t.Errorf("shape id %d used twice", id)
				}
				seen[id] = true
			}
			b := region.Bounds
			for _, m := range offRe.FindAllStringSubmatch(xml, -1) {
				x, _ := strconv.ParseInt(m[1], 10, 64)
				y, _ := strconv.ParseInt(m[2], 10, 64)
				if x < b.X || y < b.Y || x > b.X+b.Width || y > b.Y+b.Height {
					t.Errorf("shape at %d,%d lies outside the region", x, y)
				}
			}
			for _, label := range tc.labels {
				// KPI labels are set in caps; the authored text is still there.
				if !strings.Contains(xml, label) && !strings.Contains(xml, strings.ToUpper(label)) {
					t.Errorf("authored text %q not rendered", label)
				}
			}
			if !strings.Contains(xml, "<a:schemeClr") {
				t.Error("native diagram fills must follow the template theme (schemeClr)")
			}
		})
	}
}

// The region path and the placeholder path are one adapter: the same spec in
// a body placeholder and in a grid cell of the same bounds produce identical
// group XML for the same shape-ID base.
func TestNativeDiagramPlacementParity(t *testing.T) {
	bounds := types.BoundingBox{X: 457200, Y: 1600200, Width: 7680960, Height: 4572000}
	for name, tc := range nativePlacementMatrix() {
		t.Run(name, func(t *testing.T) {
			ctx := newSinglePassContext("", nil, nil, false, nil)
			ctx.themeFontName = "Calibri"
			ctx.templateSlideData[1] = &slideXML{CommonSlideData: commonSlideDataXML{ShapeTree: shapeTreeXML{Shapes: []shapeXML{{
				NonVisualProperties: nonVisualPropertiesXML{
					ConnectionNonVisual: connectionNonVisualXML{ID: 2, Name: "body"},
					NvPr:                nvPrXML{Placeholder: &placeholderXML{Type: "body"}},
				},
				ShapeProperties: shapePropertiesXML{Transform: &transformXML{
					Offset: offsetXML{X: bounds.X, Y: bounds.Y},
					Extent: extentXML{CX: bounds.Width, CY: bounds.Height},
				}},
			}}}}}
			ctx.processNativeDiagramShapes(1, 0, ContentItem{PlaceholderID: "body", Type: ContentDiagram, Value: tc.spec}, 0)
			ctx.finalizePanelGroupXML()
			inserts := ctx.panelShapeInserts[1]
			if len(inserts) != 1 {
				t.Fatalf("placeholder path registered %d groups, want 1", len(inserts))
			}
			grid, err := GenerateGridNativeDiagram(tc.spec, NativeDiagramRegion{Bounds: bounds, FontName: "Calibri"},
				DiagramAltTextFor(tc.spec), func(int) uint32 { return 10000 })
			if err != nil {
				t.Fatal(err)
			}
			if grid.XML != inserts[0].groupXML {
				t.Errorf("grid and placeholder groups differ for the same spec and bounds")
			}
		})
	}
}

// A dense framework in a small region is refused with a precise capacity
// finding rather than published at an unreadable size; the same content in a
// region of the reported minimum size renders.
func TestGenerateGridNativeDiagram_RefusesUnreadableRegion(t *testing.T) {
	long := func(prefix string) []any {
		var out []any
		for i := 1; i <= 7; i++ {
			out = append(out, fmt.Sprintf("%s consideration %d with a supporting explanation of the trend", prefix, i))
		}
		return out
	}
	spec := &types.DiagramSpec{Type: "pestel", Data: map[string]any{
		"political": long("Political"), "economic": long("Economic"), "social": long("Social"),
		"technological": long("Technological"), "environmental": long("Environmental"), "legal": long("Legal"),
	}}
	region := NativeDiagramRegion{
		Bounds: types.BoundingBox{X: 457200, Y: 1600200, Width: 3200400, Height: 2286000},
		Path:   "/slides/0/shape_grid/rows/0/cells/0/diagram",
	}
	_, err := GenerateGridNativeDiagram(spec, region, "", func(int) uint32 { return 1 })
	var capErr *NativeRegionCapacityError
	if !errors.As(err, &capErr) {
		t.Fatalf("dense PESTEL in a 3.5x2.5in region: want a capacity refusal, got %v", err)
	}
	f := capErr.Finding
	if f.Code != patterns.ErrCodeDiagramRegionTooSmall || f.Action != "refuse" || f.Path != region.Path {
		t.Fatalf("finding = %+v", f)
	}
	if f.Fix == nil || f.Fix.Kind != "simplify_or_enlarge_diagram" {
		t.Fatalf("fix = %+v", f.Fix)
	}
	for _, k := range []string{"region_width_emu", "region_height_emu", "shapes_below_floor", "actual_pt", "min_pt", "alternatives"} {
		if _, ok := f.Fix.Params[k]; !ok {
			t.Errorf("fix.params missing %q", k)
		}
	}
	if pre := NativeDiagramRegionPreflight(spec, region); pre == nil || pre.Message != f.Message {
		t.Error("validate's preflight must report the refusal generate raises")
	}
	w, okW := f.Fix.Params["min_width_emu"].(int64)
	h, okH := f.Fix.Params["min_height_emu"].(int64)
	if !okW || !okH {
		t.Fatalf("no minimum region reported: %v", f.Message)
	}
	grown := region
	grown.Bounds.Width, grown.Bounds.Height = w, h
	if _, err := GenerateGridNativeDiagram(spec, grown, "", func(int) uint32 { return 1 }); err != nil {
		t.Errorf("the reported minimum region %dx%d must render: %v", w, h, err)
	}
}

func TestGenerateGridNativeDiagram_Rejects(t *testing.T) {
	if IsGridNativeDiagram(&types.DiagramSpec{Type: "bar_chart"}) {
		t.Error("an svggen chart is not grid-native")
	}
	if _, err := GenerateGridNativeDiagram(&types.DiagramSpec{Type: "swot"}, NativeDiagramRegion{}, "", func(int) uint32 { return 1 }); err == nil {
		t.Error("empty bounds should be refused")
	}
	if _, err := GenerateGridNativeDiagram(&types.DiagramSpec{Type: "pestel", Data: map[string]any{}}, NativeDiagramRegion{
		Bounds: types.BoundingBox{Width: 5000000, Height: 3000000},
	}, "", func(int) uint32 { return 1 }); err == nil {
		t.Error("a PESTEL with no segments has nothing to draw")
	}
}
