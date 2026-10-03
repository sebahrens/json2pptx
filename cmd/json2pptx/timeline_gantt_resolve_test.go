package main

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/pptx"
	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// TestTimelineGanttResolvedBarsFollowTheirDates pins go-slide-creator-o34er
// on the resolved slide geometry, the journey's own case: a point milestone,
// Oct–Dec 2026 and Oct 2026–Apr 2027 were three bars of one length.
func TestTimelineGanttResolvedBarsFollowTheirDates(t *testing.T) {
	ctx := patterns.ExpandContext{
		SlideWidth:   12192000,
		SlideHeight:  6858000,
		LayoutBounds: patterns.LayoutBounds{X: contentRect.X, Y: contentRect.Y, Width: contentRect.CX, Height: contentRect.CY},
	}
	grid, warnings, err := expandPattern(&PatternInput{
		Name:      "timeline-horizontal",
		Overrides: json.RawMessage(`{"style":"gantt"}`),
		Values: json.RawMessage(`[
			{"label":"Board decision","date":"Oct 2026"},
			{"label":"Renegotiate cloud contract","date":"Oct 2026","end_date":"Dec 2026"},
			{"label":"Re-platform storage tier","date":"Oct 2026","end_date":"Apr 2027"}]`),
	}, ctx, patterns.Default())
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("dated gantt should not warn: %v", warnings)
	}
	b := contentRect
	res, err := resolveShapeGrid(grid, pptx.NewShapeIDAllocator(nil), &b, nil, 12192000, 6858000, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	var bars, markers []shapegrid.ResolvedCell
	for _, c := range res.Cells {
		if c.ShapeSpec == nil {
			continue
		}
		switch c.ShapeSpec.Geometry {
		case "roundRect":
			bars = append(bars, c)
		case "diamond":
			markers = append(markers, c)
		}
	}
	if len(bars) != 2 || len(markers) != 1 {
		t.Fatalf("resolved %d bars and %d markers, want 2 and 1", len(bars), len(markers))
	}
	short, long, marker := bars[0].CellBounds, bars[1].CellBounds, markers[0].CellBounds
	if ratio := float64(short.CX) / float64(long.CX); math.Abs(ratio-3.0/7.0) > 0.02 {
		t.Errorf("Oct–Dec bar is %.2f of the Oct–Apr bar, want 3/7 (%d and %d EMU)", ratio, short.CX, long.CX)
	}
	if d := short.X - long.X; d > 12700 || d < -12700 {
		t.Errorf("both ranges start in October but the bars start at %d and %d", short.X, long.X)
	}
	// The label column is 30% of the content width: empty spacer cells in
	// front of the marker must keep their footprint, or it lands on the label.
	trackX := contentRect.X + contentRect.CX*30/100
	if long.X < trackX-12700 {
		t.Errorf("bar starts at %d, left of the track (%d)", long.X, trackX)
	}
	month := long.CX / 7
	if marker.X <= long.X || marker.X+marker.CX >= long.X+month {
		t.Errorf("marker spans %d–%d, want inside October (%d–%d)", marker.X, marker.X+marker.CX, long.X, long.X+month)
	}
}

func TestTimelineGanttUnparseableDateReachesFitReportAcrossTemplates(t *testing.T) {
	values := patterns.TimelineHorizontalValues{
		{Label: "Board decision", Date: "Oct 2026"},
		{Label: "Renegotiate", Date: "Autumn", EndDate: "Year end"},
		{Label: "Re-platform", Date: "Oct 2026", EndDate: "Apr 2027"},
	}
	assertPatternFindingAcrossTemplates(t, patterns.ErrCodeTimelineDateUnparseable, "timeline-horizontal", &values,
		`values[1].date "Autumn"`, "draws no bar", &patterns.TimelineHorizontalOverrides{Style: "gantt"})
}

// TestTimelineGanttIsNotOvercrowded: the track's date-segment columns are
// geometry, not content cells, so a dated gantt does not read as overcrowded.
func TestTimelineGanttIsNotOvercrowded(t *testing.T) {
	values := patterns.TimelineHorizontalValues{
		{Label: "Discovery", Date: "2026-01-15", EndDate: "2026-03-31"},
		{Label: "Design", Date: "Q1 2026", EndDate: "Q2 2026"},
		{Label: "Pilot", Date: "2026-06", EndDate: "2026-10"},
		{Label: "Go-live", Date: "1 Nov 2026"},
		{Label: "Rollout", Date: "Q4 2026", EndDate: "Q1 2027"},
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	layouts, width, height := schemaMaximaLayouts(t, "midnight-blue")
	deck := &PresentationInput{Template: "midnight-blue", Slides: []SlideInput{{
		SlideType: "content", LayoutID: "blank-title",
		Pattern: &PatternInput{Name: "timeline-horizontal", Values: encoded, Overrides: json.RawMessage(`{"style":"gantt"}`)},
	}}}
	for _, f := range collectFitFindings(deck, layouts, width, height, nil) {
		if f.Code == patterns.ErrCodePatternOvercrowded || f.Code == patterns.ErrCodeTimelineDateUnparseable {
			t.Errorf("unexpected %s on a five-stop dated gantt: %s", f.Code, f.Message)
		}
	}
}
