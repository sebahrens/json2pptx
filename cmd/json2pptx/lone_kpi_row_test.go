package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// loneRowTemplates are the templates the lone-row tests measure:
// modern-template has the shortest content area, p-style (local, optional)
// the tallest.
var loneRowTemplates = []string{"modern-template", "midnight-blue", "p-style"}

const loneKPIValues = `[{"big":"$4.2M","small":"Annual revenue"},{"big":"98.7%","small":"Uptime SLA"},{"big":"1,247","small":"Active users"},{"big":"+12%","small":"Growth"},{"big":"42","small":"NPS"},{"big":"3.1x","small":"LTV / CAC"}]`

// kpiValuesN returns the first n cells of loneKPIValues.
func kpiValuesN(t *testing.T, n int) json.RawMessage {
	t.Helper()
	var cells []json.RawMessage
	if err := json.Unmarshal([]byte(loneKPIValues), &cells); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(cells[:n])
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// slideBand resolves a pattern slide as preflight does and returns the share
// of the content area its ink spans top to bottom and the share left empty
// beneath it.
func slideBand(t *testing.T, tpl string, slide SlideInput) (span, below float64) {
	t.Helper()
	a := loadTemplateAnalysis(t, tpl)
	grid := expandSlidePatternGrid(&slide, 0, a.SlideWidth, a.SlideHeight, &a.Theme)
	if grid == nil {
		t.Fatalf("%s: pattern did not expand", tpl)
	}
	slide.ShapeGrid = grid
	geom := resolveGridGeometry(slide, a.Layouts, a.SlideWidth, a.SlideHeight)
	result := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, a.SlideWidth, a.SlideHeight)
	if result == nil {
		t.Fatalf("%s: grid did not resolve", tpl)
	}
	acc := &geomAccumulator{m: newGeomMeasurer(&a.Theme), slideArea: a.SlideWidth * a.SlideHeight, slideWidth: a.SlideWidth, slideHeight: a.SlideHeight}
	acc.walk(grid, result, "/slides/0/pattern", 0)
	safe := contentRelativeBoundsBase(geom.OverrideBounds, geom.Zone, a.SlideWidth, a.SlideHeight)
	first, last := safe.Y+safe.CY, safe.Y
	for _, r := range acc.ink {
		if r = intersectRect(r, safe); r.CY > 0 {
			first, last = minI64(first, r.Y), maxI64(last, r.Y+r.CY)
		}
	}
	return float64(last-first) / float64(safe.CY), float64(safe.Y+safe.CY-last) / float64(safe.CY)
}

// A kpi-Nup row alone on a slide used to be a strip of 26-37% of the content
// area with a third of the area blank under it (journey h-A15). The open row
// is now grown into the free height: its band of dividers spans at least 40%
// of the area on every template and count, and less than a third of the area
// stays empty beneath it (go-slide-creator-i7yju).
func TestLoneKPIRowGrowsIntoTheFreeHeight(t *testing.T) {
	for _, tpl := range loneRowTemplates {
		if _, err := os.Stat(filepath.Join("..", "..", "templates", tpl+".pptx")); err != nil {
			t.Logf("template %s not present; skipped", tpl)
			continue
		}
		for n := 2; n <= 6; n++ {
			name := "kpi-" + string(rune('0'+n)) + "up"
			span, below := slideBand(t, tpl, titledPatternSlide(name, kpiValuesN(t, n)))
			if span < 0.40 {
				t.Errorf("%s %s: the row spans %.0f%% of the content area, want at least 40%%", tpl, name, 100*span)
			}
			if below >= 1.0/3 {
				t.Errorf("%s %s: %.0f%% of the content area is empty under the row, want under a third", tpl, name, 100*below)
			}
		}
	}
}

// Growth is for an open strip that is the slide's own composed block. Tiles
// keep their content height (a filled tile grown the same way is an empty
// box), kpi-inline is a supporting band, and a row its author placed — a
// cap, an explicit vertical_align — stays where it was put.
func TestLoneRowGrowthIsForAnOpenComposedKPIRow(t *testing.T) {
	a := loadTemplateAnalysis(t, "midnight-blue")
	expand := func(slide SlideInput) *ShapeGridInput {
		t.Helper()
		grid := expandSlidePatternGrid(&slide, 0, a.SlideWidth, a.SlideHeight, &a.Theme)
		if grid == nil {
			t.Fatalf("%s did not expand", slide.Pattern.Name)
		}
		return grid
	}
	open := titledPatternSlide("kpi-3up", kpiValuesN(t, 3))
	if !growsLoneRow(expand(open)) {
		t.Error("an open kpi-3up row must be grown")
	}
	tiles := titledPatternSlide("kpi-3up", kpiValuesN(t, 3))
	tiles.Pattern.Overrides = json.RawMessage(`{"style":"tiles"}`)
	if growsLoneRow(expand(tiles)) {
		t.Error("kpi-3up tiles must keep their content height")
	}
	if growsLoneRow(expand(titledPatternSlide("kpi-inline", kpiValuesN(t, 4)))) {
		t.Error("kpi-inline must not be grown")
	}
	boxed := expand(open)
	boxed.Bounds = &GridBoundsInput{X: 0, Y: 0, Width: 100, Height: 40}
	if growsLoneRow(boxed) {
		t.Error("a row with its own bounds box must not be grown")
	}

	// An explicit vertical_align is not composed, so it is not grown either.
	top := titledPatternSlide("kpi-4up", kpiValuesN(t, 4))
	top.Pattern.VerticalAlign = "top"
	auto := titledPatternSlide("kpi-4up", kpiValuesN(t, 4))
	topSpan, _ := slideBand(t, "midnight-blue", top)
	autoSpan, _ := slideBand(t, "midnight-blue", auto)
	if topSpan >= autoSpan || topSpan >= 0.40 {
		t.Errorf("a top-pinned row spans %.0f%% against the composed row's %.0f%%: it must keep its content height", 100*topSpan, 100*autoSpan)
	}
}

// The grown row keeps its figures and captions at the sizes the type step
// gave them, on one baseline: only the row and its dividers are taller.
func TestGrownKPIRowKeepsItsType(t *testing.T) {
	a := loadTemplateAnalysis(t, "midnight-blue")
	slide := titledPatternSlide("kpi-3up", kpiValuesN(t, 3))
	grid := expandSlidePatternGrid(&slide, 0, a.SlideWidth, a.SlideHeight, &a.Theme)
	slide.ShapeGrid = grid
	geom := resolveGridGeometry(slide, a.Layouts, a.SlideWidth, a.SlideHeight)
	grown := resolveGridForStructural(grid, geom.OverrideBounds, geom.Zone, a.SlideWidth, a.SlideHeight)

	pinned := *grid
	pinned.Source = "" // the same grid, not known as a KPI strip
	plain := resolveGridForStructural(&pinned, geom.OverrideBounds, geom.Zone, a.SlideWidth, a.SlideHeight)
	if grown == nil || plain == nil || len(grown.Cells) != len(plain.Cells) {
		t.Fatalf("resolve: grown=%v plain=%v", grown, plain)
	}
	var top int64
	for i, c := range grown.Cells {
		if string(c.ShapeSpec.Text) != string(plain.Cells[i].ShapeSpec.Text) {
			t.Errorf("cell %d: text changed with the growth\n grown %s\n plain %s", i, c.ShapeSpec.Text, plain.Cells[i].ShapeSpec.Text)
		}
		if c.CellBounds.CY <= plain.Cells[i].CellBounds.CY {
			t.Errorf("cell %d: row is %d EMU tall, not taller than the ungrown %d", i, c.CellBounds.CY, plain.Cells[i].CellBounds.CY)
		}
		if i == 0 {
			top = c.Bounds.Y
		} else if c.Bounds.Y != top {
			t.Errorf("cell %d: figures no longer share a baseline (top %d vs %d)", i, c.Bounds.Y, top)
		}
	}
}
