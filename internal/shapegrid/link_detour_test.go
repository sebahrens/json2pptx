package shapegrid

import (
	"github.com/sebahrens/json2pptx/internal/pptx"
	"strings"
	"testing"
)

func TestExplicitLinkBypassesUnrelatedBox(t *testing.T) {
	spacer := Cell{}
	grid := &Grid{Bounds: pptx.RectEmu{CX: 600 * 12700, CY: 300 * 12700}, Columns: []float64{20, 60, 20}, RowGap: 20}
	for range 3 {
		grid.Rows = append(grid.Rows, Row{Cells: []Cell{spacer, {Shape: filled("rect")}, spacer}})
	}
	grid.Links = []Link{{FromRow: 0, FromCol: 1, ToRow: 2, ToCol: 1, Spec: &ConnectorSpec{Style: "arrow"}}}
	r, err := Resolve(grid, newAlloc(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Connectors) != 3 {
		t.Fatalf("want three native segments, got %+v", r.Connectors)
	}
	for i, c := range r.Connectors {
		if c.NoArrow != (i < 2) {
			t.Errorf("segment %d arrow policy: %+v", i, c)
		}
		// The middle box occupies the center column; all line segments must
		// be outside its interior, including their short endpoint stubs.
		middle := r.Cells[1].Bounds
		if c.Bounds.CX <= 1 && c.Bounds.X > middle.X && c.Bounds.X < middle.X+middle.CX && c.Bounds.Y < middle.Y+middle.CY && c.Bounds.Y+c.Bounds.CY > middle.Y {
			t.Errorf("segment crosses unrelated box: %+v", c)
		}
	}
	if r.Connectors[0].SourceID == 0 || r.Connectors[2].TargetID == 0 {
		t.Fatal("detour lost endpoint shape attachments")
	}
}

func TestBlockedLinkRequestsAGutter(t *testing.T) {
	grid := &Grid{Bounds: pptx.RectEmu{CX: 600 * 12700, CY: 300 * 12700}, Columns: []float64{100}, RowGap: 20, Links: []Link{{FromRow: 0, ToRow: 2, Spec: &ConnectorSpec{Style: "arrow"}}}}
	for range 3 {
		grid.Rows = append(grid.Rows, Row{Cells: []Cell{{Shape: filled("rect")}}})
	}
	_, err := Resolve(grid, newAlloc(1))
	if err == nil || !strings.Contains(err.Error(), "free column/row gutter") {
		t.Fatalf("blocked link should explain remedy: %v", err)
	}
}
