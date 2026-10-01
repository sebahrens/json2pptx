package generator

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/sebahrens/json2pptx/internal/types"
)

var houseShapeXfrm = regexp.MustCompile(`name="(House Roof(?: Label)?)"(?s:.*?)<a:off x="(-?\d+)" y="(-?\d+)"/><a:ext cx="(\d+)" cy="(\d+)"/>`)

// TestHouseRoofLabelSitsInTheTriangle pins go-slide-creator-n0zpq: the roof
// label's centre lies in the middle third of the roof triangle, it keeps 8pt
// clear of the pillar row, it fits the triangle's width at its top edge, and
// a one-line objective gets a roof only as tall as its label needs.
func TestHouseRoofLabelSitsInTheTriangle(t *testing.T) {
	data := map[string]any{
		"roof": "Become the default platform for mid-market finance teams",
		"sections": []any{
			map[string]any{"label": "Grow EMEA", "items": []any{"Hire 40 reps", "Localise in 6 markets"}},
			map[string]any{"label": "Launch payments", "items": []any{"Embedded cards", "Bill pay"}},
			map[string]any{"label": "Lift NRR to 120%", "items": []any{"Usage pricing", "Expansion playbook"}},
		},
		"foundation": "People, data platform, partner ecosystem",
	}
	panels, meta, err := parseHouseDiagramNativeData(data)
	if err != nil {
		t.Fatal(err)
	}
	bounds := types.BoundingBox{X: 838200, Y: 1825625, Width: 10515600, Height: 4351338}
	xml := generateHouseDiagramGroupXML(panels, bounds, 100, meta)

	boxes := map[string][4]int64{}
	for _, m := range houseShapeXfrm.FindAllStringSubmatch(xml, -1) {
		var v [4]int64
		for i := range v {
			v[i], _ = strconv.ParseInt(m[i+2], 10, 64)
		}
		boxes[m[1]] = v
	}
	roof, ok1 := boxes["House Roof"]
	label, ok2 := boxes["House Roof Label"]
	if !ok1 || !ok2 {
		t.Fatalf("roof or roof label shape missing: %v", boxes)
	}
	roofY, roofH := roof[1], roof[3]
	centre := label[1] + label[3]/2
	if centre < roofY+roofH/3 || centre > roofY+2*roofH/3 {
		t.Errorf("label centre %d is outside the roof's middle third [%d, %d]", centre, roofY+roofH/3, roofY+2*roofH/3)
	}
	if clear := roofY + roofH - (label[1] + label[3]); clear < 8*int64(types.EMUPerPoint) {
		t.Errorf("label keeps %.1fpt above the pillar row, want >= 8pt", float64(clear)/float64(types.EMUPerPoint))
	}
	// The triangle is roof[2] wide at its base and narrows to its apex.
	if widthAtTop := roof[2] * (label[1] - roofY) / roofH; label[2] > widthAtTop {
		t.Errorf("label box %d wider than the triangle at its top edge %d", label[2], widthAtTop)
	}
	if roofH > bounds.Height/5 {
		t.Errorf("one-line roof is %d EMU tall (%.0f%% of the frame); it should shrink to its label", roofH, 100*float64(roofH)/float64(bounds.Height))
	}
}
