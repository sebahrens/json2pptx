package shapegrid

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
)

func TestColumnRulesAvoidSpanningBanner(t *testing.T) {
	g := &Grid{Bounds: pptx.RectEmu{CX: 400 * 12700, CY: 200 * 12700}, Columns: []float64{50, 50}, ColGap: 20, RowGap: 8, ColRule: json.RawMessage(`"all"`), Rows: []Row{
		{Height: 20, Cells: []Cell{{ColSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}}}},
		{Height: 80, Cells: []Cell{{Shape: &ShapeSpec{Geometry: "rect"}}, {Shape: &ShapeSpec{Geometry: "rect"}}}},
	}}
	out, err := Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.AccentBars) != 1 {
		t.Fatalf("rules=%+v", out.AccentBars)
	}
	rule := out.AccentBars[0]
	if !rule.Decorative || rule.Bounds.CX != PtToEMU(.75) || rule.Bounds.X+rule.Bounds.CX/2 != 200*12700 {
		t.Fatalf("wrong native seam %+v", rule)
	}
	banner := out.Cells[0].CellBounds
	if rule.Bounds.Y < banner.Y+banner.CY {
		t.Fatal("rule intersects spanning banner")
	}
	if len(out.Cells) != 3 {
		t.Fatal("rule became a content cell")
	}
}
func TestColumnRulesValidateAndClipRowSpan(t *testing.T) {
	for _, raw := range []string{`"side"`, `[-1]`, `[1]`, `[0.5]`} {
		if _, err := columnRuleIndices(json.RawMessage(raw), 2); err == nil {
			t.Fatalf("invalid col_rule %s", raw)
		}
	}
	g := &Grid{Bounds: pptx.RectEmu{CX: 400 * 12700, CY: 200 * 12700}, Columns: []float64{50, 50}, ColRule: json.RawMessage(`[0,0]`), Rows: []Row{{Height: 50, Cells: []Cell{{ColSpan: 2, RowSpan: 2, Shape: &ShapeSpec{Geometry: "rect"}}}}, {Height: 50}}}
	out, err := Resolve(g, pptx.NewShapeIDAllocator(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.AccentBars) != 0 {
		t.Fatal("rule crosses two-row spanning cell")
	}
}
