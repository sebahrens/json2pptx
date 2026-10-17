package shapegrid

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func bodyPeerCell(x int64, size float64, height int64, bold bool) ResolvedCell {
	text, _ := json.Marshal(map[string]any{"content": strings.Repeat("Detailed evidence ", 4), "size": size, "bold": bold})
	return ResolvedCell{Kind: CellKindShape, Bounds: pptx.RectEmu{X: x, CX: 250 * 12700, CY: height}, ShapeSpec: &ShapeSpec{Geometry: "rect", Text: text}}
}
func TestCompactSiblingBodiesShareReadableSize(t *testing.T) {
	first, second := bodyPeerCell(0, 14, 100*12700, false), bodyPeerCell(300*12700, 12, 100*12700, false)
	scope := NewPeerScope(&ResolveResult{Cells: []ResolvedCell{first}}, &ResolveResult{Cells: []ResolvedCell{second}})
	if scope == nil {
		t.Fatal("compact siblings did not share")
	}
	cells := []ResolvedCell{first, second}
	scope.shareBodySizes(cells)
	for _, c := range cells {
		tb, err := ResolveTextInput(c.ShapeSpec.Text)
		if err != nil {
			t.Fatal(err)
		}
		if tb.Paragraphs[0].Runs[0].FontSize != 1200 {
			t.Fatalf("body size not shared: %s", c.ShapeSpec.Text)
		}
	}
	if string(first.ShapeSpec.Text) == string(cells[0].ShapeSpec.Text) {
		t.Fatal("source was mutated")
	}
}
func TestSiblingBodySharingKeepsUnsafeAndDistinctRoles(t *testing.T) {
	first := bodyPeerCell(0, 14, 100*12700, false)
	for _, other := range []ResolvedCell{bodyPeerCell(300*12700, 12, 20*12700, false), bodyPeerCell(300*12700, 12, 100*12700, true)} {
		scope := NewPeerScope(&ResolveResult{Cells: []ResolvedCell{first}}, &ResolveResult{Cells: []ResolvedCell{other}})
		if scope != nil && len(scope.bodySizes) > 0 {
			t.Fatalf("unsafe or bold role changed: %+v", scope.bodySizes)
		}
	}
}
