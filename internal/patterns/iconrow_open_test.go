package patterns

import (
	"strings"
	"testing"
)

func openIconItems(n int) IconRowValues {
	names := []string{"rocket", "trending-up", "currency-dollar", "clock", "star"}
	items := make(IconRowValues, n)
	for i := range items {
		items[i] = IconRowItem{Icon: &IconRef{Name: names[i]}, Caption: "Launch"}
	}
	return items
}

// go-slide-creator-hjqn2: the default icon-row draws no container. Icons are
// standalone cells on one row, captions unfilled text on the row beneath.
func TestIconRowOpenDefaultHasNoContainers(t *testing.T) {
	p := &iconRow{}
	ctx := testThemeCtx()
	for _, n := range []int{3, 4, 5} {
		items := openIconItems(n)
		grid, err := p.Expand(ctx, &items, nil, nil)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if len(grid.Rows) != 2 || len(grid.Rows[0].Cells) != n || len(grid.Rows[1].Cells) != n {
			t.Fatalf("n=%d: want an icon row and a caption row of %d cells, got %d rows", n, n, len(grid.Rows))
		}
		for i, c := range grid.Rows[0].Cells {
			if c.Shape != nil || c.Icon == nil || c.Icon.Fill == "" {
				t.Errorf("n=%d icon %d: want a standalone coloured icon, got %+v", n, i, c)
			}
		}
		for i, c := range grid.Rows[1].Cells {
			if c.Shape == nil || string(c.Shape.Fill) != `"none"` || string(c.Shape.Line) != `"none"` || c.AccentBar != nil {
				t.Errorf("n=%d caption %d: want unfilled, unlined text, got %+v", n, i, c.Shape)
			}
			if !strings.Contains(string(c.Shape.Text), `"vertical_align":"t"`) {
				t.Errorf("n=%d caption %d is not top-anchored: captions must share a baseline", n, i)
			}
		}
		if grid.VerticalAlign != GridVerticalAlignDefault {
			t.Errorf("n=%d vertical_align = %q", n, grid.VerticalAlign)
		}
	}
}

// Icons scale with the content-area height inside the 40–88pt band, and never
// take more than half their column.
func TestIconRowOpenIconScalesWithArea(t *testing.T) {
	p := &iconRow{}
	iconPt := func(ctx ExpandContext, n int, ovr *IconRowOverrides) float64 {
		items := openIconItems(n)
		var o any
		if ovr != nil {
			o = ovr
		}
		grid, err := p.Expand(ctx, &items, o, nil)
		if err != nil {
			t.Fatal(err)
		}
		return grid.Rows[0].MaxHeight
	}
	ctx := testThemeCtx()
	_, areaH := sizingAreaPt(ctx)
	got := iconPt(ctx, 3, nil)
	if got < iconRowOpenIconMinPt || got > iconRowOpenIconMaxPt {
		t.Errorf("icon %.0fpt outside the %.0f–%.0fpt band", got, iconRowOpenIconMinPt, iconRowOpenIconMaxPt)
	}
	if want := clampPt(areaH*iconRowOpenIconFrac, iconRowOpenIconMinPt, iconRowOpenIconMaxPt); got != float64(int(want+0.5)) {
		t.Errorf("icon %.0fpt, want %.0fpt (%.0f%% of a %.0fpt area)", got, want, iconRowOpenIconFrac*100, areaH)
	}
	short := ctx
	short.LayoutBounds = LayoutBounds{Width: 9000000, Height: 1200000} // ~94pt tall
	if got := iconPt(short, 3, nil); got != iconRowOpenIconMinPt {
		t.Errorf("icon on a short area = %.0fpt, want the %.0fpt minimum", got, iconRowOpenIconMinPt)
	}
	if got := iconPt(ctx, 3, &IconRowOverrides{IconSize: 60}); got != 60 {
		t.Errorf("icon_size 60 gives %.0fpt", got)
	}
}

// A description is one muted line under the bold caption, and grows the
// caption row for every item alike.
func TestIconRowOpenDescription(t *testing.T) {
	p := &iconRow{}
	ctx := testThemeCtx()
	plain := openIconItems(4)
	described := openIconItems(4)
	described[2].Description = "Net revenue retention at 127%"
	if err := p.Validate(&described, nil, nil); err != nil {
		t.Fatalf("validate: %v", err)
	}
	a, _ := p.Expand(ctx, &plain, nil, nil)
	b, _ := p.Expand(ctx, &described, nil, nil)
	if b.Rows[1].MaxHeight <= a.Rows[1].MaxHeight {
		t.Errorf("a description must grow the caption row: %.0f vs %.0f", b.Rows[1].MaxHeight, a.Rows[1].MaxHeight)
	}
	if !strings.Contains(string(b.Rows[1].Cells[2].Shape.Text), "Net revenue retention") {
		t.Error("description missing from the caption cell")
	}
	described[0].Description = strings.Repeat("x", iconRowDescriptionMax+1)
	if err := p.Validate(&described, nil, nil); err == nil || !strings.Contains(err.Error(), "values[0].description") {
		t.Errorf("want a max-length error on values[0].description, got %v", err)
	}
}

func TestIconRowStyleValidatedAndSecondaryKeepsTiles(t *testing.T) {
	p := &iconRow{}
	items := openIconItems(3)
	if err := p.Validate(&items, &IconRowOverrides{Style: "card"}, nil); err == nil || !strings.Contains(err.Error(), "overrides.style") {
		t.Fatalf("want an overrides.style enum error, got %v", err)
	}
	items[0].Secondary = &SecondaryChart{Type: "sparkline", Values: []float64{1, 2, 3}}
	grid, err := p.Expand(testThemeCtx(), &items, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.Rows) != 1 {
		t.Errorf("a secondary chart keeps the tile layout, got %d rows", len(grid.Rows))
	}
}

// Captions past what the content area holds are reported, not shrunk.
func TestIconRowOpenWarnsWhenTooTall(t *testing.T) {
	p := &iconRow{}
	ctx := testThemeCtx()
	ctx.LayoutBounds = LayoutBounds{Width: 9000000, Height: 1000000} // ~79pt tall
	items := openIconItems(5)
	for i := range items {
		items[i].Caption = strings.Repeat("word ", 12)
	}
	got := p.PostExpandWarnings(ctx, &items, nil)
	if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) {
		t.Fatalf("want one BODY_TOO_LONG, got %v", got)
	}
	ok := openIconItems(3)
	if got := p.PostExpandWarnings(testThemeCtx(), &ok, nil); len(got) != 0 {
		t.Errorf("exemplar-sized row warns: %v", got)
	}
}
