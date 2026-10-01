package generator

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-ngbnf: SWOT and five forces render natively in a grid cell,
// inside the cell bounds, within the shape-ID range the caller reserved.
func TestGenerateGridNativeDiagramXML(t *testing.T) {
	bounds := types.BoundingBox{X: 500000, Y: 1800000, Width: 10000000, Height: 4000000}
	specs := map[string]*types.DiagramSpec{
		"swot": {Type: "swot", Data: map[string]any{
			"strengths": []any{"S-item"}, "weaknesses": []any{"W-item"},
			"opportunities": []any{"O-item"}, "threats": []any{"T-item"},
		}},
		"porters_five_forces": {Type: "porters_five_forces", Data: map[string]any{
			"rivalry":      map[string]any{"label": "Competitive rivalry", "factors": []any{"R-item"}, "intensity": 0.85},
			"new_entrants": map[string]any{"label": "Threat of new entrants", "factors": []any{"E-item"}},
			"substitutes":  map[string]any{"label": "Threat of substitutes", "factors": []any{"Sub-item"}},
			"suppliers":    map[string]any{"label": "Bargaining power of suppliers", "factors": []any{"P-item"}},
			"buyers":       map[string]any{"label": "Bargaining power of buyers", "factors": []any{"B-item"}},
		}},
	}
	idRe := regexp.MustCompile(`<p:cNvPr id="(\d+)"`)
	offRe := regexp.MustCompile(`<a:off x="(\d+)" y="(\d+)"/>`)
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			if !IsGridNativeDiagram(spec) {
				t.Fatalf("%s is not grid-native", name)
			}
			const base = 300
			n := GridNativeDiagramShapeIDs(spec)
			xml, err := GenerateGridNativeDiagramXML(spec, bounds, base, nil, "", "Alt for "+name)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(xml, "<p:grpSp") {
				t.Fatalf("want one group, got %.80s", xml)
			}
			if !strings.Contains(xml, `descr="Alt for `+name+`"`) {
				t.Error("group carries no alt text")
			}
			for _, m := range idRe.FindAllStringSubmatch(xml, -1) {
				id, _ := strconv.Atoi(m[1])
				if id < base || id >= base+n {
					t.Errorf("shape id %d outside reserved range [%d,%d)", id, base, base+n)
				}
			}
			for _, m := range offRe.FindAllStringSubmatch(xml, -1) {
				x, _ := strconv.ParseInt(m[1], 10, 64)
				y, _ := strconv.ParseInt(m[2], 10, 64)
				if x < bounds.X || y < bounds.Y || x > bounds.X+bounds.Width || y > bounds.Y+bounds.Height {
					t.Errorf("shape at %d,%d lies outside the cell", x, y)
				}
			}
			if got := strings.Count(xml, "-item"); got != len(spec.Data) {
				t.Errorf("%d of %d authored items rendered", got, len(spec.Data))
			}
		})
	}

	if IsGridNativeDiagram(&types.DiagramSpec{Type: "bar_chart"}) {
		t.Error("an svggen chart is not grid-native")
	}
	if _, err := GenerateGridNativeDiagramXML(&types.DiagramSpec{Type: "swot"}, types.BoundingBox{}, 1, nil, "", ""); err == nil {
		t.Error("empty bounds should be refused")
	}
}
