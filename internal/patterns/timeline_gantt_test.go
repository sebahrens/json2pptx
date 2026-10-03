package patterns

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/sebahrens/json2pptx/internal/jsonschema"
)

// ganttBarCell returns the bar or marker cell of stop i in an expanded gantt.
func ganttBarCell(t *testing.T, grid *jsonschema.ShapeGridInput, stop int) *jsonschema.GridCellInput {
	t.Helper()
	row := grid.Rows[len(grid.Rows)-ganttStopCount(grid)+stop]
	for _, cell := range row.Cells {
		if cell != nil && cell.Shape != nil && (cell.Shape.Geometry == "roundRect" || cell.Shape.Geometry == "diamond") {
			return cell
		}
	}
	t.Fatalf("stop %d has no bar or marker cell", stop)
	return nil
}

// ganttStopCount is the number of stop rows: every row whose first cell is a
// label.
func ganttStopCount(grid *jsonschema.ShapeGridInput) int {
	n := 0
	for _, row := range grid.Rows {
		if len(row.Cells) > 0 && row.Cells[0] != nil && row.Cells[0].Shape != nil {
			n++
		}
	}
	return n
}

// ganttExtent returns where stop i's bar starts and how wide it is, as
// percentages of the grid, with the geometry of its bar cell.
func ganttExtent(t *testing.T, grid *jsonschema.ShapeGridInput, stop int) (start, width float64, geometry string) {
	t.Helper()
	var cols []float64
	if err := json.Unmarshal(grid.Columns, &cols); err != nil {
		t.Fatalf("columns: %v", err)
	}
	row := grid.Rows[len(grid.Rows)-ganttStopCount(grid)+stop]
	col := 0
	for _, cell := range row.Cells {
		span := 1
		if cell != nil && cell.ColSpan > 1 {
			span = cell.ColSpan
		}
		w := 0.0
		for k := col; k < col+span; k++ {
			w += cols[k]
		}
		if cell != nil && cell.Shape != nil && (cell.Shape.Geometry == "roundRect" || cell.Shape.Geometry == "diamond") {
			x := 0.0
			for k := 0; k < col; k++ {
				x += cols[k]
			}
			return x, w, cell.Shape.Geometry
		}
		col += span
	}
	t.Fatalf("stop %d has no bar or marker cell", stop)
	return 0, 0, ""
}

// TestTimelineGanttBarsAreScaledToTheirDates pins go-slide-creator-o34er: a
// point milestone, a three-month range and a seven-month range used to be
// three identical full-width bars.
func TestTimelineGanttBarsAreScaledToTheirDates(t *testing.T) {
	p := &timelineHorizontal{}
	stops := TimelineHorizontalValues{
		{Label: "Board decision", Date: "Oct 2026"},
		{Label: "Renegotiate cloud contract", Date: "Oct 2026", EndDate: "Dec 2026"},
		{Label: "Re-platform storage tier", Date: "Oct 2026", EndDate: "Apr 2027"},
	}
	ovr := &TimelineHorizontalOverrides{Style: "gantt"}
	if err := p.Validate(&stops, ovr, nil); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, &stops, ovr, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}

	pointX, pointW, pointGeom := ganttExtent(t, grid, 0)
	shortX, shortW, shortGeom := ganttExtent(t, grid, 1)
	longX, longW, longGeom := ganttExtent(t, grid, 2)
	if pointGeom != "diamond" {
		t.Errorf("a stop without end_date is drawn as %q, want a diamond marker", pointGeom)
	}
	if shortGeom != "roundRect" || longGeom != "roundRect" {
		t.Errorf("ranges are drawn as %q and %q, want bars", shortGeom, longGeom)
	}
	// Oct–Dec is three of the seven months Oct–Apr covers.
	if ratio := shortW / longW; math.Abs(ratio-3.0/7.0) > 0.02 {
		t.Errorf("three-month bar is %.2f of the seven-month bar, want 3/7 (widths %.1f%% and %.1f%%)", ratio, shortW, longW)
	}
	if shortX != longX {
		t.Errorf("both ranges start in October but the bars start at %.1f%% and %.1f%%", shortX, longX)
	}
	// The marker sits in mid-October: inside the first month, not at its edge.
	if month := longW / 7; pointX <= longX || pointX+pointW >= longX+month {
		t.Errorf("marker spans %.1f%%–%.1f%%, want inside October (%.1f%%–%.1f%%)", pointX, pointX+pointW, longX, longX+month)
	}

	// The axis is labelled: one unit label per month, the year on the first
	// and on January.
	var labels []string
	for _, cell := range grid.Rows[0].Cells {
		if cell == nil || cell.Shape == nil {
			continue
		}
		var text struct {
			Paragraphs []struct {
				Content string `json:"content"`
			} `json:"paragraphs"`
		}
		if err := json.Unmarshal(cell.Shape.Text, &text); err != nil || len(text.Paragraphs) == 0 {
			t.Fatalf("axis label: %v", err)
		}
		labels = append(labels, text.Paragraphs[0].Content)
	}
	if got, want := strings.Join(labels, "|"), "Oct '26|Nov|Dec|Jan '27|Feb|Mar|Apr"; got != want {
		t.Errorf("axis labels = %q, want %q", got, want)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, &stops, ovr); len(got) != 0 {
		t.Errorf("dated stops should not warn: %v", got)
	}
}

// TestTimelineGanttOffsetsAndDayPrecision: bars that start at different dates
// start at different places, and a day-level date lands inside its month.
func TestTimelineGanttOffsetsAndDayPrecision(t *testing.T) {
	p := &timelineHorizontal{}
	stops := TimelineHorizontalValues{
		{Label: "Discovery", Date: "2026-01-16", EndDate: "2026-03-31"},
		{Label: "Design", Date: "Q2 2026", EndDate: "Q3 2026"},
		{Label: "Rollout", Date: "2026-10", EndDate: "2026-12"},
	}
	grid, err := p.Expand(ExpandContext{}, &stops, &TimelineHorizontalOverrides{Style: "gantt"}, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	x0, w0, _ := ganttExtent(t, grid, 0)
	x1, w1, _ := ganttExtent(t, grid, 1)
	x2, w2, _ := ganttExtent(t, grid, 2)
	track := 70.0
	year := 365.0
	for i, c := range []struct{ x, w, fromDay, days float64 }{
		{x0, w0, 15, 75},  // 16 Jan to 31 Mar
		{x1, w1, 90, 183}, // Apr to Sep
		{x2, w2, 273, 92}, // Oct to Dec
	} {
		if wantX := 30 + track*c.fromDay/year; math.Abs(c.x-wantX) > 0.6 {
			t.Errorf("stop %d starts at %.1f%%, want %.1f%%", i, c.x, wantX)
		}
		if wantW := track * c.days / year; math.Abs(c.w-wantW) > 0.6 {
			t.Errorf("stop %d is %.1f%% wide, want %.1f%%", i, c.w, wantW)
		}
	}
}

// TestTimelineGanttUnparseableDatesAreReported: a stop whose dates are not
// dates gets no bar, and the pattern says which value it could not read.
func TestTimelineGanttUnparseableDatesAreReported(t *testing.T) {
	p := &timelineHorizontal{}
	stops := TimelineHorizontalValues{
		{Label: "Board decision", Date: "Oct 2026"},
		{Label: "Renegotiate", Date: "Autumn", EndDate: "Year end"},
		{Label: "Re-platform", Date: "Mar 2027", EndDate: "Jan 2027"},
		{Label: "Review", Date: "Oct 2026", EndDate: "Apr 2027"},
	}
	ovr := &TimelineHorizontalOverrides{Style: "gantt"}
	if err := p.Validate(&stops, ovr, nil); err != nil {
		t.Fatalf("free-text dates stay valid input: %v", err)
	}
	grid, err := p.Expand(ExpandContext{}, &stops, ovr, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	for _, stop := range []int{1, 2} {
		row := grid.Rows[len(grid.Rows)-len(stops)+stop]
		for _, cell := range row.Cells {
			if cell != nil && cell.Shape != nil && (cell.Shape.Geometry == "roundRect" || cell.Shape.Geometry == "diamond") {
				t.Errorf("stop %d has dates that cannot be placed but is drawn as a %s", stop, cell.Shape.Geometry)
			}
		}
		if !strings.Contains(string(row.Cells[1].Shape.Text), stops[stop].Date) {
			t.Errorf("stop %d lost its date text", stop)
		}
	}
	warnings := p.PostExpandWarnings(ExpandContext{}, &stops, ovr)
	if len(warnings) != 1 || !strings.HasPrefix(warnings[0], ErrCodeTimelineDateUnparseable+": ") {
		t.Fatalf("warnings = %v, want one %s", warnings, ErrCodeTimelineDateUnparseable)
	}
	for _, want := range []string{`values[1].date "Autumn" is not a date`, `values[2].end_date "Jan 2027" is before its date "Mar 2027"`} {
		if !strings.Contains(warnings[0], want) {
			t.Errorf("warning does not name %q: %s", want, warnings[0])
		}
	}

	// With no date readable there is no axis, and no bars.
	none := TimelineHorizontalValues{
		{Label: "Discovery", Date: "Early spring", EndDate: "Summer"},
		{Label: "Build", Date: "Summer", EndDate: "Year end"},
		{Label: "Rollout", Date: "Next year", EndDate: "Later"},
	}
	grid, err = p.Expand(ExpandContext{}, &none, ovr, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(grid.Rows) != 3 {
		t.Errorf("undated gantt has %d rows, want 3 (no axis)", len(grid.Rows))
	}
	if strings.Contains(mustJSON(t, grid), "roundRect") {
		t.Error("undated gantt still draws bars")
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestTimelineGanttYearlessDates: dates without a year order among themselves,
// run on across a year end, and print no year on the axis.
func TestTimelineGanttYearlessDates(t *testing.T) {
	p := &timelineHorizontal{}
	stops := TimelineHorizontalValues{
		{Label: "Plan", Date: "Nov", EndDate: "Dec"},
		{Label: "Build", Date: "Dec", EndDate: "Jan"},
		{Label: "Launch", Date: "Feb", EndDate: "Mar"},
	}
	ovr := &TimelineHorizontalOverrides{Style: "gantt"}
	grid, err := p.Expand(ExpandContext{}, &stops, ovr, nil)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	x0, _, _ := ganttExtent(t, grid, 0)
	x1, _, _ := ganttExtent(t, grid, 1)
	x2, _, _ := ganttExtent(t, grid, 2)
	if !(x0 < x1 && x1 < x2) {
		t.Errorf("Nov, Dec, Feb start at %.1f%%, %.1f%%, %.1f%%; want them in order across the year end", x0, x1, x2)
	}
	if axis := mustJSON(t, grid.Rows[0]); strings.Contains(axis, "'") {
		t.Errorf("yearless axis prints a year: %s", axis)
	}
	if got := p.PostExpandWarnings(ExpandContext{}, &stops, ovr); len(got) != 0 {
		t.Errorf("yearless stops should not warn: %v", got)
	}

	// A yearless stop beside dated ones cannot be placed.
	mixed := TimelineHorizontalValues{
		{Label: "Plan", Date: "Q1 2026", EndDate: "Q2 2026"},
		{Label: "Build", Date: "Q3", EndDate: "Q4"},
		{Label: "Launch", Date: "Q1 2027"},
	}
	got := p.PostExpandWarnings(ExpandContext{}, &mixed, ovr)
	if len(got) != 1 || !strings.Contains(got[0], `values[1].date "Q3" has no year`) {
		t.Errorf("mixed yearless warning = %v", got)
	}
}

func TestParseTimelinePeriod(t *testing.T) {
	day := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		in         string
		start, end time.Time
		yearless   bool
	}{
		{"2026-03-15", day(2026, 3, 15), day(2026, 3, 16), false},
		{"2026-03-15T09:30:00Z", day(2026, 3, 15), day(2026, 3, 16), false},
		{"2026/03/15", day(2026, 3, 15), day(2026, 3, 16), false},
		{"2026-03", day(2026, 3, 1), day(2026, 4, 1), false},
		{"2026", day(2026, 1, 1), day(2027, 1, 1), false},
		{"Mar 2026", day(2026, 3, 1), day(2026, 4, 1), false},
		{"March 2026", day(2026, 3, 1), day(2026, 4, 1), false},
		{"Mar '26", day(2026, 3, 1), day(2026, 4, 1), false},
		{"Sept 2026", day(2026, 9, 1), day(2026, 10, 1), false},
		{"Mar 5, 2026", day(2026, 3, 5), day(2026, 3, 6), false},
		{"5 March 2026", day(2026, 3, 5), day(2026, 3, 6), false},
		{"1st Nov 2026", day(2026, 11, 1), day(2026, 11, 2), false},
		{"05-Mar-2026", day(2026, 3, 5), day(2026, 3, 6), false},
		{"Q1 2026", day(2026, 1, 1), day(2026, 4, 1), false},
		{"2026 Q4", day(2026, 10, 1), day(2027, 1, 1), false},
		{"2026Q2", day(2026, 4, 1), day(2026, 7, 1), false},
		{"Q3 '26", day(2026, 7, 1), day(2026, 10, 1), false},
		{"H2 2026", day(2026, 7, 1), day(2027, 1, 1), false},
		{"Mar", day(2000, 3, 1), day(2000, 4, 1), true},
		{"Apr 30", day(2000, 4, 30), day(2000, 5, 1), true},
		{"30 Apr", day(2000, 4, 30), day(2000, 5, 1), true},
		{"Q2", day(2000, 4, 1), day(2000, 7, 1), true},
		{"H1", day(2000, 1, 1), day(2000, 7, 1), true},
	} {
		got, ok := parseTimelinePeriod(tc.in)
		if !ok {
			t.Errorf("%q did not parse", tc.in)
			continue
		}
		if !got.start.Equal(tc.start) || !got.end.Equal(tc.end) || got.yearless != tc.yearless {
			t.Errorf("%q = %s to %s (yearless %v), want %s to %s (yearless %v)", tc.in,
				got.start.Format(time.DateOnly), got.end.Format(time.DateOnly), got.yearless,
				tc.start.Format(time.DateOnly), tc.end.Format(time.DateOnly), tc.yearless)
		}
	}
	for _, in := range []string{"", "Summer", "TBD", "Phase 1", "Jan-Mar", "2026-13", "Feb 31 2026", "Q5 2026", "H3 2026", "12345", "soon after launch"} {
		if got, ok := parseTimelinePeriod(in); ok {
			t.Errorf("%q parsed as %s to %s, want not a date", in, got.start.Format(time.DateOnly), got.end.Format(time.DateOnly))
		}
	}
}

// TestTimelineGanttAxisUnits: the axis takes the finest unit that covers the
// stops in at most eight divisions.
func TestTimelineGanttAxisUnits(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		first    string
		ticks    int
	}{
		{"2026-03-02", "2026-03-06", "2 Mar", 5},
		{"2026-03-02", "2026-04-10", "2 Mar", 6},
		{"Oct 2026", "Apr 2027", "Oct '26", 7},
		{"Jan 2026", "Dec 2026", "Q1 '26", 4},
		{"Q1 2026", "Q4 2028", "H1 '26", 6},
		{"2026", "2031", "2026", 6},
		{"2026", "2045", "2025", 5},
	} {
		dates := resolveTimelineGanttDates([]TimelineStop{{Label: "A", Date: tc.from, EndDate: tc.to}})
		if len(dates.problems) > 0 {
			t.Fatalf("%s to %s: %v", tc.from, tc.to, dates.problems)
		}
		axis := newTimelineGanttAxis(dates, timelineGanttMaxTicks)
		if len(axis.ticks) != tc.ticks || axis.ticks[0].label != tc.first {
			t.Errorf("%s to %s: %d ticks from %q, want %d from %q", tc.from, tc.to, len(axis.ticks), axis.ticks[0].label, tc.ticks, tc.first)
		}
		if axis.start.After(dates.spans[0].start) || axis.end.Before(dates.spans[0].end) {
			t.Errorf("%s to %s: axis %s–%s does not cover the stop", tc.from, tc.to, axis.start.Format(time.DateOnly), axis.end.Format(time.DateOnly))
		}
	}
}
