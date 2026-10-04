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

// go-slide-creator-6xgxm: the 12-character maximum read as a budget, yet six
// values of 12 characters were refused on a wide-faced template. The budget is
// measured per KPI count: that many digits are never reported, one more is,
// and the finding quotes the same count.
func TestKPIValueLineBudgetAgreesWithTheFinding(t *testing.T) {
	for _, area := range writtenFitAreas {
		ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(area.w * 12700), Height: int64(area.h * 12700)}}
		ctx.Theme.BodyFont = area.font
		for n := 2; n <= 6; n++ {
			budget := KPIValueLineBudget(ctx, n)
			if budget < 6 || budget > kpiNupBigMaxChars {
				t.Errorf("%s: %d KPIs hold %d digits, want 6–12", area.name, n, budget)
			}
			pat, _ := Default().Get("kpi-" + strconv.Itoa(n) + "up")
			cells := make(KPINupValues, n)
			for digits := budget; digits <= min(budget+1, kpiNupBigMaxChars); digits++ {
				for i := range cells {
					cells[i] = KPICell{Big: strings.Repeat("8", digits), Small: "Value"}
				}
				got := pat.(PostExpandWarner).PostExpandWarnings(ctx, &cells, nil)
				switch {
				case digits == budget && len(got) != 0:
					t.Errorf("%s: %d values of %d digits (the budget) were reported: %v", area.name, n, digits, got[0])
				case digits > budget && (len(got) != n || !strings.Contains(got[0], "holds about "+strconv.Itoa(budget)+" characters")):
					t.Errorf("%s: %d values of %d digits: warnings = %v, want each reported with the %d-digit budget", area.name, n, digits, got, budget)
				}
			}
		}
	}
}

// A value fitted down under the lead step would sit two points or less over
// a 14–16pt caption; the caption then drops to the body step so the value —
// marked "figure", so the grid writes it at its fitted size, 16pt at the
// least (go-slide-creator-a5ogo) — stays the larger line. A row of short
// values keeps the authored caption.
func TestKPILongValuesKeepTheValueAboveItsCaption(t *testing.T) {
	sizes := func(t *testing.T, cells KPINupValues) (value, caption float64) {
		t.Helper()
		pat, _ := Default().Get("kpi-6up")
		ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: 797 * 12700, Height: 349 * 12700}}
		ctx.Theme.BodyFont = "Calibri"
		grid, err := pat.Expand(ctx, &cells, &KPIOverrides{BigSize: 44, SmallSize: 16}, nil)
		if err != nil {
			t.Fatal(err)
		}
		var text kpiTextObj
		if err := json.Unmarshal(grid.Rows[0].Cells[0].Shape.Text, &text); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(grid.Rows[0].Cells[0].Shape.Text), `"figure":true`) {
			t.Errorf("the value paragraph is not marked as a figure: %s", grid.Rows[0].Cells[0].Shape.Text)
		}
		return text.Paragraphs[0].Size, text.Paragraphs[1].Size
	}
	long, short := make(KPINupValues, 6), make(KPINupValues, 6)
	for i := range long {
		long[i] = KPICell{Big: "EUR 987.65bn", Small: "Transactions processed"}
		short[i] = KPICell{Big: "42%", Small: "Transactions processed"}
	}
	if value, caption := sizes(t, long); value >= scaleLeadPt || caption != scaleBodyPt {
		t.Errorf("six 12-character values: value %.0fpt over a %.0fpt caption, want a value under %.0fpt over the %.0fpt body step", value, caption, scaleLeadPt, scaleBodyPt)
	}
	if value, caption := sizes(t, short); value < scaleLeadPt || caption != 16 {
		t.Errorf("six short values: value %.0fpt over a %.0fpt caption, want the authored 16pt caption", value, caption)
	}
}
