package generator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// houseFloorsExample is the floors shape the house_diagram capability entry
// documents (cmd/json2pptx/skill_info.go nativeDiagramDataContracts).
const houseFloorsExample = `{"type": "house_diagram", "data": {
	"roof": "Leader in digital payments",
	"sections": [{"label": "Technology", "items": ["Cloud platform"]}, {"label": "Product", "items": ["Mobile wallet"]}, {"label": "People", "items": ["Talent"]}],
	"floors": ["Shared data platform", {"sections": ["Risk", "Controls", "Partners"]}],
	"foundation": "Trust and compliance"}}`

func renderHouse(t *testing.T, raw string) string {
	t.Helper()
	spec := hdx2lSpec(t, raw)
	bounds := types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}
	env := nativeDiagramEnv{fontName: "Arial"}
	layout, err := layoutNativeDiagram(spec, bounds, env, nativeDiagramSite{path: "/slides/0/content/1"})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	return renderNativeInsert(&layout.insert, 100, env)
}

// TestHouseFloorsDocumentedShapeRendersEverything pins go-slide-creator-8hmdq:
// the documented floors example draws the roof, every pillar, every floor and
// the foundation.
func TestHouseFloorsDocumentedShapeRendersEverything(t *testing.T) {
	xml := renderHouse(t, houseFloorsExample)
	for _, want := range []string{
		"Leader in digital payments", "Technology", "Cloud platform", "Product", "Mobile wallet", "People", "Talent",
		"Shared data platform", "Risk", "Controls", "Partners", "Trust and compliance",
	} {
		if !strings.Contains(xml, ">"+want+"<") {
			t.Errorf("documented floors example does not draw %q", want)
		}
	}
	boxes := houseBoxes(xml)
	pillars, band, cells, foundation := boxes["Pillar Technology"], boxes["Floor Shared data platform"], boxes["Floor Controls"], boxes["House Foundation"]
	if !(pillars[1] < band[1] && band[1] < cells[1] && cells[1] < foundation[1]) {
		t.Errorf("levels out of order: pillars y=%d, band y=%d, cells y=%d, foundation y=%d", pillars[1], band[1], cells[1], foundation[1])
	}
	bandW, cellW := band[2], cells[2] //nolint:gosec // G602 false positive: [4]int64 indexed by a constant
	if bandW != 10515600 || cellW >= bandW/2 {
		t.Errorf("band is %d wide and a cell %d: want a full-width band over a row of three cells", bandW, cellW)
	}
}

// TestHouseFloorsNeverDropContent replays the journey findings (B9): string
// floors used to vanish, and object floors beside sections used to hide every
// pillar. Each documented spelling now draws all of its text.
func TestHouseFloorsNeverDropContent(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want []string
	}{
		"string floors beside sections": {
			`{"type": "house_diagram", "data": {"roof": "Vision", "sections": [{"label": "Grow"}, {"label": "Run"}], "floors": ["Enablers", "Shared platform"], "foundation": "Values"}}`,
			[]string{"Vision", "Grow", "Run", "Enablers", "Shared platform", "Values"},
		},
		"object floors beside sections": {
			`{"type": "house_diagram", "data": {"roof": "Vision", "sections": [{"label": "A"}, {"label": "B"}, {"label": "C"}, {"label": "D"}], "floors": [{"label": "Enabling layer one"}, {"label": "Enabling layer two", "items": ["Data", "People"]}], "foundation": "Values"}}`,
			[]string{"Vision", "A", "B", "C", "D", "Enabling layer one", "Enabling layer two", "Data · People", "Values"},
		},
		"floors alone, in the order written": {
			`{"type": "house_diagram", "data": {"roof": "Mission", "floors": [{"type": "single", "label": "Strategy band"}, {"type": "parallel", "sections": [{"label": "A", "items": ["One"]}, {"label": "B"}]}, {"label": "Enablers"}], "foundation": "Values"}}`,
			[]string{"Mission", "Strategy band", "A", "One", "B", "Enablers", "Values"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			xml := renderHouse(t, tc.raw)
			for _, want := range tc.want {
				if !strings.Contains(xml, ">"+want+"<") {
					t.Errorf("%q is not drawn", want)
				}
			}
		})
	}
}

// TestHouseFloorsOtherShapesAreRefused: any floors value the builder would
// skip is refused with the shape it draws, in every placement (validate and
// generate both run NativeDiagramDataErrors).
func TestHouseFloorsOtherShapesAreRefused(t *testing.T) {
	thirteen := make([]string, 13)
	for i := range thirteen {
		thirteen[i] = fmt.Sprintf("%q", fmt.Sprintf("S%d", i))
	}
	cases := []struct {
		name, data, field, want string
	}{
		{"floors is a string", `"floors": "Enablers"`, "data.floors", "must be a list of levels"},
		{"floors is an object", `"floors": {"label": "Enablers"}`, "data.floors", "must be a list of levels"},
		{"number floor", `"floors": [3]`, "data.floors[0]", "is not a level"},
		{"nested list floor", `"floors": [["People", "Data"]]`, "data.floors[0]", "is not a level"},
		{"empty object floor", `"floors": [{}]`, "data.floors[0]", "has neither a label nor sections"},
		{"empty string floor", `"floors": [" "]`, "data.floors[0]", "is an empty band"},
		{"band and row at once", `"floors": [{"label": "Enablers", "sections": ["A", "B"]}]`, "data.floors[0]", "mixes a band"},
		{"parallel without sections", `"floors": [{"type": "parallel", "label": "X"}]`, "data.floors[0]", `is "parallel" but has no sections`},
		{"unknown floor type", `"floors": [{"type": "stacked", "label": "X"}]`, "data.floors[0].type", `a floor's type is "single"`},
		{"empty sections row", `"floors": [{"sections": []}]`, "data.floors[0].sections", "must be a list of strings or"},
		{"items not strings", `"floors": [{"label": "X", "items": [1, 2]}]`, "data.floors[0].items", "must be a list of strings"},
		{"section not drawable", `"sections": ["A", 7]`, "data.sections[1]", "must be a string or a {label, items?} object"},
		{"two pillar rows", `"sections": ["A"], "pillars": ["B"]`, "data.sections", "the pillar row is already given"},
		{"too many cells", `"sections": [` + strings.Join(thirteen, ",") + `]`, "data.sections", "has 13 cells; a level holds at most 12"},
		{"no common grid", `"sections": ["A","B","C","D","E","F","G"], "floors": [{"sections": ["A","B","C","D","E"]}, {"sections": ["A","B","C"]}]`, "data", "cannot share one column grid"},
		{"outer elements beside sections", `"sections": ["A"], "outer_elements": ["B"]`, "data.outer_elements", "is only drawn when the house has no sections"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := hdx2lSpec(t, `{"type": "house_diagram", "data": {"roof": "Vision", "foundation": "Values", `+tc.data+`}}`)
			errs := NativeDiagramDataErrors(spec)
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), tc.want) && (e.Field == tc.field || tc.name == "two pillar rows") {
					found = true
				}
			}
			if !found {
				t.Fatalf("errors = %v, want %q at %s", errs, tc.want, tc.field)
			}
			if tc.name == "floors is a string" || tc.name == "number floor" {
				if msg := errs[0].Error(); !strings.Contains(msg, `{"sections": [{"label": "…", "items"?: ["…"]}]}`) {
					t.Errorf("refusal does not state the expected shape: %s", msg)
				}
			}
			if _, err := layoutNativeDiagram(spec, types.BoundingBox{Width: 9000000, Height: 4000000}, nativeDiagramEnv{}, nativeDiagramSite{}); err == nil {
				t.Error("generate laid the refused house out")
			}
		})
	}
	// The shapes that were always accepted stay accepted.
	for _, ok := range []string{
		`"sections": ["A", "B", "C"]`,
		`"pillars": [{"label": "A", "items": ["x"]}]`,
		`"center_element_unused": 1`,
	} {
		spec := hdx2lSpec(t, `{"type": "house_diagram", "data": {"roof": "Vision", "foundation": "Values", `+ok+`}}`)
		errs := NativeDiagramDataErrors(spec)
		if strings.Contains(ok, "unused") != (len(errs) > 0) {
			t.Errorf("%s: errors = %v", ok, errs)
		}
	}
}
