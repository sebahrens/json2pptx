package generator

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

var (
	houseShapeXfrm = regexp.MustCompile(`<p:cNvPr id="\d+" name="([^"]+)"(?s:.*?)<a:off x="(-?\d+)" y="(-?\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)
	houseRoofAdj2  = regexp.MustCompile(`<a:gd name="adj2" fmla="val (\d+)"/>`)
	houseSchemeClr = regexp.MustCompile(`<a:schemeClr val="(accent\d)"`)
)

func houseTestData() map[string]any {
	return map[string]any{
		"roof": "Become the default platform for mid-market finance teams",
		"sections": []any{
			map[string]any{"label": "Grow EMEA", "items": []any{"Hire 40 reps", "Localise in 6 markets"}},
			map[string]any{"label": "Launch payments", "items": []any{"Embedded cards", "Bill pay"}},
			map[string]any{"label": "Lift NRR to 120%", "items": []any{"Usage pricing", "Expansion playbook"}},
			map[string]any{"label": "Open the platform", "items": []any{"Partner APIs"}},
		},
		"foundation": "People, data platform, partner ecosystem",
	}
}

func houseBoxes(xml string) map[string][4]int64 {
	boxes := map[string][4]int64{}
	for _, m := range houseShapeXfrm.FindAllStringSubmatch(xml, -1) {
		var v [4]int64
		for i := range v {
			v[i], _ = strconv.ParseInt(m[i+2], 10, 64)
		}
		boxes[m[1]] = v
	}
	return boxes
}

// TestHouseRoofIsAReadableGable pins go-slide-creator-x25dq: the native house
// has a gable whose pitch reads as a roof at body-placeholder size, uses one
// accent with neutral pillar surfaces, and takes the placeholder's height
// instead of its top third.
func TestHouseRoofIsAReadableGable(t *testing.T) {
	panels, meta, err := parseHouseDiagramNativeData(houseTestData())
	if err != nil {
		t.Fatal(err)
	}
	placeholder := types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}
	bounds, _ := fitNativeFrameworkAt("house_diagram", placeholder, panels, meta, "Arial", nativeDiagramSite{})
	xml := generateHouseDiagramGroupXML(panels, bounds, 100, meta, nativeDiagramEnv{fontName: "Arial"})

	boxes := houseBoxes(xml)
	roof, ok := boxes["House Roof"]
	if !ok {
		t.Fatalf("roof shape missing: %v", boxes)
	}
	if !strings.Contains(xml, `prst="upArrow"`) || !strings.Contains(xml, `<a:gd name="adj1" fmla="val 100000"/>`) {
		t.Fatalf("roof is not the full-width gable pentagon: %s", xml)
	}
	adj := houseRoofAdj2.FindStringSubmatch(xml)
	if adj == nil {
		t.Fatal("roof has no gable adjustment")
	}
	adj2, _ := strconv.ParseFloat(adj[1], 64)
	rise := adj2 / 100000 * float64(min(roof[2], roof[3]))
	// One in ten of the half-span is the flattest pitch that reads as a roof.
	if rise < float64(roof[2])/20 {
		t.Errorf("gable rises %.0f EMU over a %d EMU span — too shallow to read as a roof", rise, roof[2])
	}
	if roof[2] != placeholder.Width {
		t.Errorf("roof is %d wide, placeholder %d", roof[2], placeholder.Width)
	}

	// The house uses the placeholder: over half its height, not a third.
	if bounds.Height*2 < placeholder.Height {
		t.Errorf("house takes %d of %d EMU of the placeholder height", bounds.Height, placeholder.Height)
	}
	foundation := boxes["House Foundation"]
	if bottom := foundation[1] + foundation[3]; bottom > placeholder.Y+placeholder.Height {
		t.Errorf("foundation ends at %d, below the placeholder bottom %d", bottom, placeholder.Y+placeholder.Height)
	}

	// One accent hue plus neutrals.
	hues := map[string]bool{}
	for _, m := range houseSchemeClr.FindAllStringSubmatch(xml, -1) {
		hues[m[1]] = true
	}
	if len(hues) != 1 {
		t.Errorf("house uses %d accent hues %v, want one", len(hues), hues)
	}
	for _, name := range []string{"Pillar Grow EMEA", "Pillar Open the platform", "House Foundation"} {
		if _, ok := boxes[name]; !ok {
			t.Errorf("shape %q missing: %v", name, boxes)
		}
	}
}

// TestHouseDiagramMatchesTheStrategyHousePattern: the native diagram and the
// pattern draw the same content through one builder — same rows, geometry,
// fills and text.
func TestHouseDiagramMatchesTheStrategyHousePattern(t *testing.T) {
	panels, meta, err := parseHouseDiagramNativeData(houseTestData())
	if err != nil {
		t.Fatal(err)
	}
	bounds := types.BoundingBox{Width: 10515600, Height: 4351338}
	native, err := layoutHouse(panels, meta, bounds, nativeDiagramEnv{})
	if err != nil {
		t.Fatal(err)
	}

	pat, _ := patterns.Default().Get("strategy-house")
	values := pat.NewValues()
	raw := `{"objective": "Become the default platform for mid-market finance teams", "pillars": [
		{"title": "Grow EMEA", "body": ["Hire 40 reps", "Localise in 6 markets"]},
		{"title": "Launch payments", "body": ["Embedded cards", "Bill pay"]},
		{"title": "Lift NRR to 120%", "body": ["Usage pricing", "Expansion playbook"]},
		{"title": "Open the platform", "body": ["Partner APIs"]}],
		"foundation": "People, data platform, partner ecosystem"}`
	if err := json.Unmarshal([]byte(raw), values); err != nil {
		t.Fatal(err)
	}
	grid, err := pat.Expand(patterns.ExpandContext{LayoutBounds: patterns.LayoutBounds{Width: bounds.Width, Height: bounds.Height}}, values, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(native.Grid)
	want, _ := json.Marshal(grid)
	if string(got) != string(want) {
		t.Errorf("native house and strategy-house pattern drifted:\nnative:  %s\npattern: %s", got, want)
	}
}

// TestHouseDiagramSamePlacementEverywhere: the same data in the same bounds
// renders the same shapes whichever placement asks (body placeholder, grid
// cell and compose segment all reach layoutNativeDiagram).
func TestHouseDiagramSamePlacementEverywhere(t *testing.T) {
	spec := &types.DiagramSpec{Type: "house_diagram", Data: houseTestData()}
	bounds := types.BoundingBox{X: 500000, Y: 1500000, Width: 5200000, Height: 4300000}
	env := nativeDiagramEnv{fontName: "Arial"}
	a, err := layoutNativeDiagram(spec, bounds, env, nativeDiagramSite{path: "/slides/0/content/1"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := layoutNativeDiagram(spec, bounds, env, nativeDiagramSite{path: "/slides/0/shape_grid/rows/0/cells/0/diagram"})
	if err != nil {
		t.Fatal(err)
	}
	if x, y := renderNativeInsert(&a.insert, 200, env), renderNativeInsert(&b.insert, 200, env); x != y || x == "" {
		t.Error("the same house rendered differently in two placements")
	}
}
