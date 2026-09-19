package generator

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-r87g: the preflight dry-render asked svggen about diagram
// types svggen does not draw, got "unknown diagram type", and reported a REFUSE
// claiming the deck would render a grey "Data unavailable" placeholder — for
// diagrams this package draws as OOXML shapes, perfectly. This list is the seam
// between the two renderers, so it has to match the dispatch that uses it.

func TestIsNativeDiagramType(t *testing.T) {
	for _, typ := range NativeDiagramTypeNames() {
		if !IsNativeDiagramType(&types.DiagramSpec{Type: typ}) {
			t.Errorf("%s is listed as native but IsNativeDiagramType says otherwise", typ)
		}
	}
	// svggen's own types must NOT be claimed, or their dry-render findings
	// would be silently dropped.
	for _, typ := range []string{"bar_chart", "line_chart", "pie_chart", "org_chart", "venn", "matrix_2x2", "gantt", "fishbone", "timeline"} {
		if IsNativeDiagramType(&types.DiagramSpec{Type: typ}) {
			t.Errorf("%s is an svggen type but was claimed as native", typ)
		}
	}
	if IsNativeDiagramType(nil) || IsNativeDiagramType(&types.DiagramSpec{}) {
		t.Error("a nil or typeless spec is not native")
	}
}

// The list and the render dispatch must agree: a type the dispatch draws
// natively but the list omits gets a false refuse, and one the list claims but
// the dispatch does not draw loses its dry-render findings.
func TestNativeDiagramTypesMatchDispatch(t *testing.T) {
	dispatch := map[string]func(*types.DiagramSpec) bool{
		"swot":                  isSWOTDiagram,
		"pestel":                isPESTELDiagram,
		"nine_box_talent":       isNineBoxDiagram,
		"value_chain":           isValueChainDiagram,
		"kpi_dashboard":         isKPIDashboardDiagram,
		"porters_five_forces":   isPortersFiveForcesDiagram,
		"business_model_canvas": isBMCDiagram,
		"process_flow":          isProcessFlowDiagram,
		"heatmap":               isHeatmapDiagram,
		"pyramid":               isPyramidDiagram,
		"house_diagram":         isHouseDiagram,
	}
	for typ, isNative := range dispatch {
		spec := &types.DiagramSpec{Type: typ}
		if !isNative(spec) {
			t.Errorf("fixture wrong: the dispatch predicate for %s does not match its own type", typ)
		}
		if !IsNativeDiagramType(spec) {
			t.Errorf("%s is drawn natively by the dispatch but is missing from the native list", typ)
		}
	}
	// The panel family routes through isPanelNativeLayout, including its
	// aliases and its default layout.
	for _, spec := range []*types.DiagramSpec{
		{Type: "panel_layout"},
		{Type: "panel_layout", Data: map[string]any{"layout": "rows"}},
		{Type: "panel_layout", Data: map[string]any{"layout": "stat_cards"}},
		{Type: "icon_columns"},
		{Type: "icon_rows"},
		{Type: "stat_cards"},
	} {
		if !IsNativeDiagramType(spec) {
			t.Errorf("%+v is drawn by the native panel path but was not claimed", spec)
		}
	}
}
