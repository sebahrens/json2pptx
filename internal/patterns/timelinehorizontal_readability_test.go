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

// timelineDotsAt expands a dots timeline into a w×h area and reports whether
// its text is written at or above the floor where the validator measures it
// (a text cell laid out in no height is not measured), whether any text cell
// was laid out in no height, and whether the pattern reported BODY_TOO_LONG.
func timelineDotsAt(t *testing.T, w, h float64, v *TimelineHorizontalValues) (reads, hidden, warned bool) {
	t.Helper()
	p := &timelineHorizontal{}
	below, warnings := patternWrittenBelowFloor(t, p, ExpandContext{}, w, h, v, &TimelineHorizontalOverrides{})
	ctx := ExpandContext{LayoutBounds: LayoutBounds{Width: int64(w * 12700), Height: int64(h * 12700)}}
	grid, err := p.Expand(ctx, v, &TimelineHorizontalOverrides{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyGridDefaults(grid)
	for _, c := range resolveGridAt(t, grid, pptx.RectEmu{CX: ctx.LayoutBounds.Width, CY: ctx.LayoutBounds.Height}).Cells {
		if c.Kind == shapegrid.CellKindShape && c.ShapeSpec != nil && len(c.ShapeSpec.Text) > 0 && c.Bounds.CY <= 0 {
			hidden = true
		}
	}
	return len(below) == 0, hidden, len(warnings) > 0
}

// More height never turns a timeline that reads into one that does not
// (go-slide-creator-wj8uz). The dots layout fixed a 47pt date row (a 12pt
// date in 28pt of inset and 4pt of padding) and inset stop text. A
// three-stop timeline in a regions cell passed validation at 25% of a stack
// — the grid's row gaps took the area and the labels were laid out in zero
// height, which nothing measures — was written shrunk at 35–45%, and read
// again from 50%. The rows now trim to their text before the labels give way,
// and a timeline that still does not fit is scaled whole: its text is always
// laid out, and BODY_TOO_LONG is reported exactly when it is written shrunk.
func TestTimelineHorizontalDotsFitMonotonicInHeight(t *testing.T) {
	step := 2.0
	if testing.Short() {
		step = 6
	}
	type tc struct {
		name              string
		stops             int
		label, date, body string
		widths            []float64
	}
	cases := []tc{
		{"3-short-dated", 3, "Design", "Oct", "", []float64{260, 450, 850}},
		{"3-wrapping-label", 3, "Pilot in two regions", "Q2 2026", "", []float64{260, 450}},
		{"3-undated", 3, "Rollout", "", "", []float64{450}},
		{"4-body", 4, "Pilot", "Q1 2025", "Run the new model in two regions", []float64{450, 850}},
		{"7-short", 7, "Phase", "2025", "", []float64{450, 850}},
	}
	for _, c := range cases {
		v := make(TimelineHorizontalValues, c.stops)
		for i := range v {
			v[i] = TimelineStop{Label: c.label, Date: c.date, Body: c.body}
		}
		for _, w := range c.widths {
			t.Run(fmt.Sprintf("%s/w%.0f", c.name, w), func(t *testing.T) {
				readsFrom, quietFrom := 0.0, 0.0
				for h := 12.0; h <= 360; h += step {
					reads, hidden, warned := timelineDotsAt(t, w, h, &v)
					if hidden {
						t.Errorf("at %.0fpt text is laid out in no height", h)
					}
					if !reads && !warned {
						t.Errorf("at %.0fpt text is written below the floor without BODY_TOO_LONG", h)
					}
					switch {
					case reads && readsFrom == 0:
						readsFrom = h
					case !reads && readsFrom > 0:
						t.Fatalf("reads at %.0fpt but not at %.0fpt", readsFrom, h)
					}
					switch {
					case !warned && quietFrom == 0:
						quietFrom = h
					case warned && quietFrom > 0:
						t.Fatalf("no BODY_TOO_LONG at %.0fpt but one at %.0fpt", quietFrom, h)
					}
				}
				if readsFrom == 0 {
					t.Fatal("never reads up to 360pt")
				}
			})
		}
	}
}
