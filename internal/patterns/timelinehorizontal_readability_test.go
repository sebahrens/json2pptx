package patterns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// writtenFloorAreas are the short content areas of the shipped templates
// (TestShortContentAreaExemplarsStayAboveFloor) plus the local p-style area.
var writtenFloorAreas = []struct {
	name string
	w, h float64
}{
	{"abstract", 687, 294},
	{"modern", 851, 311},
	{"warm-coral", 828, 349},
	{"p-style", 899, 360},
}

// patternWrittenBelowFloor expands a pattern at an area and returns the text the
// writer would store shrunk below the 12pt floor, together with the
// BODY_TOO_LONG warnings the pattern reported for the same payload. The
// sizing rule (go-slide-creator-n1muf): a legal payload is either laid out
// readably or reported by the pattern itself — never silently shrunk.
func patternWrittenBelowFloor(t *testing.T, p Pattern, ctx ExpandContext, w, h float64, values, overrides any) (below []string, warned []string) {
	t.Helper()
	ctx.LayoutBounds = LayoutBounds{Width: int64(w * 12700), Height: int64(h * 12700)}
	if wr, ok := p.(PostExpandWarner); ok {
		for _, msg := range wr.PostExpandWarnings(ctx, values, overrides) {
			if strings.Contains(msg, ErrCodeBodyTooLong) {
				warned = append(warned, msg)
			}
		}
	}
	grid, err := p.Expand(ctx, values, overrides, nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyGridDefaults(grid)
	res := resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height})
	for _, c := range res.Cells {
		if c.Kind != shapegrid.CellKindShape || c.ShapeSpec == nil || len(c.ShapeSpec.Text) == 0 {
			continue
		}
		tb, err := shapegrid.ResolveTextInput(c.ShapeSpec.Text)
		if err != nil || tb == nil {
			continue
		}
		for j := range tb.Insets {
			tb.Insets[j] += c.TextInsets[j]
		}
		smallest := smallestRunPt(tb)
		if smallest == 0 {
			continue
		}
		if scale := writtenScaleIn(ctx, tb, c.Bounds); smallest*scale < 11.95 {
			below = append(below, fmt.Sprintf("%q at %.0fpt × %.0f%% = %.1fpt in %.0f×%.0fpt",
				firstText(tb), smallest, scale*100, smallest*scale, float64(c.Bounds.CX)/12700, float64(c.Bounds.CY)/12700))
		}
	}
	return below, warned
}

func timelineStops(n, label, body int) *TimelineHorizontalValues {
	v := make(TimelineHorizontalValues, n)
	for i := range v {
		v[i] = TimelineStop{Label: wordsOfLength("Phase", label), Date: "Q1 2025", Body: wordsOfLength("lorem ipsum", body)}
	}
	return &v
}

// wordsOfLength repeats word (space-separated) to exactly n characters.
func wordsOfLength(word string, n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	for b.Len() < n {
		b.WriteString(word)
		b.WriteString(" ")
	}
	return strings.TrimSpace(b.String()[:n])
}

// Dots stops were pinned to 40% of the content height by the theme-font
// model, so on the short areas the documented per-stop budgets (3 stops: 200
// body characters beside a 35-character label, …) were written at 9–11.8pt
// with no warning; gantt bars and label-only chevrons were capped the same
// way. Rows now grow to the written fit of their tallest cell, the label steps
// 14 → 12pt when they cannot, and whatever still does not fit is reported as
// BODY_TOO_LONG (go-slide-creator-n1muf).
func TestTimelineHorizontalWrittenFitOnShortAreas(t *testing.T) {
	p := &timelineHorizontal{}
	type tc struct {
		style              string
		stops, label, body int
	}
	var cases []tc
	for stops, bands := range timelineBodyBudgetBands["dots"] {
		for _, band := range bands {
			cases = append(cases, tc{"dots", stops, band.maxLabel, band.body})
		}
	}
	for stops := 3; stops <= 7; stops++ {
		for _, label := range []int{10, 20, 40, 60} {
			cases = append(cases, tc{"chevron", stops, label, 0})
		}
		cases = append(cases, tc{"gantt", stops, timelineGanttLabelBudget, 0}, tc{"gantt", stops, 20, 0})
	}
	for _, c := range cases {
		for _, a := range writtenFloorAreas {
			name := fmt.Sprintf("%s/%d-stops/label%d/body%d/%s", c.style, c.stops, c.label, c.body, a.name)
			t.Run(name, func(t *testing.T) {
				below, warned := patternWrittenBelowFloor(t, p, ExpandContext{}, a.w, a.h, timelineStops(c.stops, c.label, c.body), &TimelineHorizontalOverrides{Style: c.style})
				if len(below) > 0 && len(warned) == 0 {
					t.Errorf("written below the floor without BODY_TOO_LONG: %v", below)
				}
			})
		}
	}
}
