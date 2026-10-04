package patterns

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// go-slide-creator-kjrxx: "EUR 48.2M" was fitted to its card edge to edge and
// then written a type step larger by the grid, where a renderer broke it at
// the space; the card grew a line, was shrunk on its own, and the row read at
// two sizes. A row whose value has no room to grow keeps the sizes it was
// measured at, and a value sits inside kpiValueLineFrac of its line.
func TestKPIValueWithoutRoomPinsItsRow(t *testing.T) {
	long := KPINupValues{
		{Big: "EUR 48.2M", Small: "Annual revenue", Sub: "+12%"},
		{Big: "118%", Small: "Net retention"},
		{Big: "41d", Small: "Sales cycle", Sub: "-6d"},
		{Big: "USD 1.25bn", Small: "Pipeline"},
	}
	short := KPINupValues{{Big: "18", Small: "Plants"}, {Big: "41", Small: "Days"}, {Big: "7", Small: "Markets"}, {Big: "92", Small: "Sites"}}
	pat, _ := Default().Get("kpi-4up")
	// The semantic compiler authors these sizes on a kpi_snapshot slide.
	ovr := &KPIOverrides{BigSize: 44, SmallSize: 16}
	for _, area := range writtenFitAreas {
		ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(area.w * 12700), Height: int64(area.h * 12700)}}
		ctx.Theme.BodyFont = area.font
		grid, err := pat.Expand(ctx, &long, ovr, nil)
		if err != nil {
			t.Fatalf("%s: %v", area.name, err)
		}
		width := kpiCardGeometryWithGap(ctx, 4, ctx.Gap(kpiOpenGapPt)).valueWidthPt(nil, "")
		sizes := map[float64]bool{}
		for i, c := range grid.Rows[0].Cells {
			var text kpiTextObj
			if err := json.Unmarshal(c.Shape.Text, &text); err != nil {
				t.Fatal(err)
			}
			value := text.Paragraphs[0]
			sizes[value.Size] = true
			if c.Shape.TypeScale != peerTextTypeScale {
				t.Errorf("%s: cell %d takes part in the grid's type step (type_scale %q); its row has a value with no room to grow", area.name, i, c.Shape.TypeScale)
			}
			if !kpiValueFits(value.Content, area.font, value.Size, width*kpiValueLineFrac) {
				t.Errorf("%s: %q at %.0fpt does not sit inside its %.0fpt line", area.name, value.Content, value.Size, width)
			}
		}
		if len(sizes) != 1 {
			t.Errorf("%s: the values are written at %d sizes: %v", area.name, len(sizes), sizes)
		}
		// Values with room to grow leave the row to the grid, as before.
		grid, err = pat.Expand(ctx, &short, ovr, nil)
		if err != nil {
			t.Fatalf("%s: %v", area.name, err)
		}
		if got := grid.Rows[0].Cells[0].Shape.TypeScale; got != "" {
			t.Errorf("%s: a row of short values is pinned (type_scale %q)", area.name, got)
		}
	}
}

// A value that does not fit one line even at the floor says how many of its
// characters do.
func TestKPIValueTooLongNamesItsBudget(t *testing.T) {
	cells := make(KPINupValues, 6)
	for i := range cells {
		cells[i] = KPICell{Big: "42%", Small: "Value"}
	}
	cells[2].Big = "WWWWWWWWWWWW"
	pat, _ := Default().Get("kpi-6up")
	got := pat.(PostExpandWarner).PostExpandWarnings(kpiTestCtx(), &cells, nil)
	if len(got) != 1 || !strings.HasPrefix(got[0], ErrCodeBodyTooLong+": ") || !strings.Contains(got[0], "values[2].big") {
		t.Fatalf("warnings = %v, want one BODY_TOO_LONG on values[2].big", got)
	}
	m := regexp.MustCompile(`holds about (\d+) characters`).FindStringSubmatch(got[0])
	if m == nil {
		t.Fatalf("the warning names no character budget: %s", got[0])
	}
	if n, _ := strconv.Atoi(m[1]); n < 1 || n >= 12 {
		t.Errorf("budget %d, want fewer than the 12 characters that do not fit", n)
	}
}
