package patterns

import (
	"encoding/json"
	"testing"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// kpiTestCtx is a 16:9 content area (~11.5in x 4.9in) like the bundled templates.
func kpiTestCtx() ExpandContext {
	return ExpandContext{
		SlideWidth:   12192000,
		SlideHeight:  6858000,
		LayoutBounds: LayoutBounds{X: 457200, Y: 1400000, Width: 10515600, Height: 4480000},
	}
}

type kpiTextObj struct {
	Paragraphs []struct {
		Content string  `json:"content"`
		Size    float64 `json:"size"`
		Align   string  `json:"align"`
	} `json:"paragraphs"`
	Align         string `json:"align"`
	VerticalAlign string `json:"vertical_align"`
}

func expandKPIForTest(t *testing.T, n int, cells KPINupValues) *jsonschema.ShapeGridInput {
	t.Helper()
	p, ok := Default().Get(kpiName(n))
	if !ok {
		t.Fatalf("pattern %s not registered", kpiName(n))
	}
	grid, err := p.Expand(kpiTestCtx(), &cells, nil, nil)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	return grid
}

func kpiName(n int) string { return map[int]string{2: "kpi-2up", 3: "kpi-3up", 4: "kpi-4up", 5: "kpi-5up", 6: "kpi-6up"}[n] }

// TestKPI3up_ValueSingleLineCentred covers go-slide-creator-5lbo: "$4.2M" with
// an icon must render on one line (no "$4 / .2 / M" break) and centred.
func TestKPI3up_ValueSingleLineCentred(t *testing.T) {
	cells := KPINupValues{
		{Big: "$4.2M", Small: "ARR added in H1", Icon: &IconRef{Name: "currency-dollar"}},
		{Big: "127%", Small: "Net revenue retention", Icon: &IconRef{Name: "trending-up"}},
		{Big: "12 days", Small: "Median sales cycle", Icon: &IconRef{Name: "clock"}},
	}
	grid := expandKPIForTest(t, 3, cells)
	geo := kpiCardGeometryFor(kpiTestCtx(), 3)
	for i, c := range grid.Rows[0].Cells {
		var txt kpiTextObj
		if err := json.Unmarshal(c.Shape.Text, &txt); err != nil {
			t.Fatalf("cell %d text: %v", i, err)
		}
		big := txt.Paragraphs[0]
		if big.Align != "ctr" || txt.Align != "ctr" {
			t.Errorf("cell %d: value must be centred, got para=%q body=%q", i, big.Align, txt.Align)
		}
		w := geo.valueWidthPt(cells[i].Icon, c.Shape.Icon.Position)
		if lines := measuredLines(big.Content, "", true, big.Size, w); lines != 1 {
			t.Errorf("cell %d: %q at %gpt wraps to %d lines in %.0fpt", i, big.Content, big.Size, lines, w)
		}
	}
}

// TestKPI6up_NarrowCardsUseTopIcon: narrow cards stack the icon on top so the
// value keeps the full card width; values shrink uniformly instead of wrapping.
func TestKPI6up_NarrowCardsUseTopIcon(t *testing.T) {
	cells := KPINupValues{}
	for _, v := range []string{"$4.2M", "127%", "12d", "98%", "42", "3.2x"} {
		cells = append(cells, KPICell{Big: v, Small: "caption", Icon: &IconRef{Name: "star"}})
	}
	grid := expandKPIForTest(t, 6, cells)
	size := -1.0
	for i, c := range grid.Rows[0].Cells {
		if c.Shape.Icon == nil || c.Shape.Icon.Position != "top" {
			t.Errorf("cell %d: want top icon on a narrow card, got %+v", i, c.Shape.Icon)
		}
		var txt kpiTextObj
		_ = json.Unmarshal(c.Shape.Text, &txt)
		if size >= 0 && txt.Paragraphs[0].Size != size {
			t.Errorf("cell %d: big sizes must be uniform, got %g vs %g", i, txt.Paragraphs[0].Size, size)
		}
		size = txt.Paragraphs[0].Size
	}
}

// TestKPI_ExplicitIconPositionWins keeps a user-authored icon position.
func TestKPI_ExplicitIconPositionWins(t *testing.T) {
	cells := KPINupValues{
		{Big: "1", Small: "a", Icon: &IconRef{Name: "star", Position: "left"}},
		{Big: "2", Small: "b"},
		{Big: "3", Small: "c"},
		{Big: "4", Small: "d"},
		{Big: "5", Small: "e"},
		{Big: "6", Small: "f"},
	}
	grid := expandKPIForTest(t, 6, cells)
	if got := grid.Rows[0].Cells[0].Shape.Icon.Position; got != "left" {
		t.Errorf("explicit icon position must win, got %q", got)
	}
}

func TestFitSingleLineSize(t *testing.T) {
	if got := fitSingleLineSize("$4.2M", "", true, 36, 16, 400); got != 36 {
		t.Errorf("wide box should keep 36pt, got %g", got)
	}
	got := fitSingleLineSize("$4.2M", "", true, 36, 16, 60)
	if got >= 36 || got < 16 {
		t.Errorf("narrow box should shrink into [16,36), got %g", got)
	}
	if lines := measuredLines("$4.2M", "", true, got, 60); got > 16 && lines != 1 {
		t.Errorf("shrunk size %g still wraps (%d lines)", got, lines)
	}
	if got := fitSingleLineSize("", "", true, 36, 16, 10); got != 36 {
		t.Errorf("empty text keeps size, got %g", got)
	}
	if got := fitSingleLineSize("WWWWWWWWWWWWWW", "", true, 36, 16, 5); got != 16 {
		t.Errorf("impossible fit floors at min, got %g", got)
	}
}
