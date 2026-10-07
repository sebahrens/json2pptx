package svggen

import (
	"math"
	"strings"
	"testing"
	"time"
)

// trackTestItems lays a track out the way drawTrack does at the floor size
// and returns the placed items.
func trackTestItems(t *testing.T, width, height float64, acts []TimelineActivity) ([]trackItem, trackType, Rect) {
	t.Helper()
	b := NewSVGBuilder(width, height)
	assumeSlidePlacement(b, &RequestEnvelope{})
	tc := NewTimelineChart(b, DefaultTimelineConfig(width, height))
	body := b.StyleGuide().Typography.SizeSmall
	tt := trackType{body: body, title: body, bodyLine: body * trackLineFactor, titleLine: body * trackLineFactor}
	plot := Rect{X: trackEdgePad, Y: trackEdgePad, W: width - 2*trackEdgePad, H: height - 2*trackEdgePad}
	data := TimelineData{Activities: normalizeTimelineActivities(acts)}
	items := tc.trackItems(data, plot, tt)
	hasBars := false
	for _, it := range items {
		hasBars = hasBars || it.isBar
	}
	trackAssignBarLanes(items)
	tc.trackPlaceBlocks(items, plot, tt, hasBars)
	return items, tt, plot
}

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// The example deck's shape: six dated events, each {date, title,
// description}. Every one is a moment on the axis (the old layout drew them
// as zero-length bars floating above it), carries its own date, and keeps
// its whole name.
func TestTimelineTrack_DatedEventsAreMomentsWithTheirDates(t *testing.T) {
	req := &RequestEnvelope{Type: "timeline", Data: map[string]any{"items": []any{
		map[string]any{"date": "2024-01-15", "title": "UX Research & Design", "description": "User research and design phase"},
		map[string]any{"date": "2024-02-15", "title": "Backend Development", "description": "Core API and services"},
		map[string]any{"date": "2024-03-01", "title": "Frontend Development", "description": "UI implementation"},
		map[string]any{"date": "2024-05-01", "title": "Alpha Release", "description": "Internal testing milestone"},
		map[string]any{"date": "2024-06-15", "title": "Beta Release", "description": "External beta program"},
		map[string]any{"date": "2024-08-01", "title": "GA Launch", "description": "General availability"},
	}}, Output: OutputSpec{Width: 899, Height: 315}}
	b, doc, err := (&Timeline{NewBaseDiagram("timeline")}).RenderWithBuilder(req)
	if err != nil {
		t.Fatal(err)
	}
	svg := doc.String()
	for _, want := range []string{"UX Research &amp; Design", "Backend Development", "Frontend Development", "15 Jan 2024", "1 Mar 2024", "1 Aug 2024", "General availability"} {
		if !strings.Contains(svg, ">"+want+"<") {
			t.Errorf("track is missing %q", want)
		}
	}
	if n := len(timelineBarPath.FindAllString(svg, -1)); n != 0 {
		t.Errorf("dated events drew %d bars; a date without an end is a moment", n)
	}
	for _, f := range b.Findings() {
		if f.Code == FindingLabelTruncated {
			t.Errorf("a six-event timeline on a body-sized canvas must not cut a label: %s", f.Message)
		}
	}
	if min := b.MinDrawnFontSize(); min+0.05 < diagramReadablePt {
		t.Errorf("drew text at %.1fpt, below the %.0fpt floor", min, diagramReadablePt)
	}
}

// Two events a week apart both keep their names: the second moves to another
// lane, and the block nearer the axis slides aside so no leader crosses it.
func TestTimelineTrack_CloseDatesAreBothLabelledWithoutCrossings(t *testing.T) {
	acts := []TimelineActivity{
		{Label: "Kick-off", Type: TimelineActivityTypeMilestone, Date: day(2025, 1, 6)},
		{Label: "Steering committee", Type: TimelineActivityTypeMilestone, Date: day(2025, 2, 1)},
		{Label: "Vendor shortlist agreed", Type: TimelineActivityTypeMilestone, Date: day(2025, 2, 8)},
		{Label: "Pilot site live", Type: TimelineActivityTypeMilestone, Date: day(2025, 6, 20)},
		{Label: "Pilot review", Type: TimelineActivityTypeMilestone, Date: day(2025, 6, 27)},
		{Label: "Go-live", Type: TimelineActivityTypeMilestone, Date: day(2025, 12, 1)},
	}
	items, tt, plot := trackTestItems(t, 899, 315, acts)
	gap := tt.body * trackBlockGap
	for i, it := range items {
		if it.truncated {
			t.Errorf("%q was cut", it.act.Label)
		}
		if it.blockLeft() < plot.X-0.01 || it.blockRight() > plot.X+plot.W+0.01 {
			t.Errorf("%q block [%.1f, %.1f] leaves the plot", it.act.Label, it.blockLeft(), it.blockRight())
		}
		if a := it.anchorX(); a < it.blockLeft()-0.01 || a > it.blockRight()+0.01 {
			t.Errorf("%q: leader at %.1f does not land in its block [%.1f, %.1f]", it.act.Label, a, it.blockLeft(), it.blockRight())
		}
		for j, o := range items {
			if i == j || o.above != it.above {
				continue
			}
			if o.lane == it.lane && j < i && o.blockRight()+gap > it.blockLeft()+0.01 {
				t.Errorf("%q and %q overlap in one lane", o.act.Label, it.act.Label)
			}
			if o.lane < it.lane && it.anchorX() > o.blockLeft() && it.anchorX() < o.blockRight() {
				t.Errorf("the leader of %q runs through the block of %q", it.act.Label, o.act.Label)
			}
		}
	}
}

// An item with a start and an end is a bar drawn to scale; a moment on the
// same timeline is labelled above the axis, the bar below it.
func TestTimelineTrack_DurationsAreBarsToScale(t *testing.T) {
	acts := []TimelineActivity{
		{Label: "Discovery", StartDate: day(2025, 1, 1), EndDate: day(2025, 4, 1)},
		{Label: "Build", StartDate: day(2025, 4, 1), EndDate: day(2025, 10, 1)},
		{Label: "Board approval", Type: TimelineActivityTypeMilestone, Date: day(2025, 7, 1)},
	}
	items, _, _ := trackTestItems(t, 899, 315, acts)
	var discovery, build, board trackItem
	for _, it := range items {
		switch it.act.Label {
		case "Discovery":
			discovery = it
		case "Build":
			build = it
		default:
			board = it
		}
	}
	if !discovery.isBar || !build.isBar || board.isBar {
		t.Fatalf("bars: discovery=%v build=%v board=%v, want true true false", discovery.isBar, build.isBar, board.isBar)
	}
	// 90 days against 183 days.
	if ratio := (build.x1 - build.x0) / (discovery.x1 - discovery.x0); math.Abs(ratio-183.0/90.0) > 0.02 {
		t.Errorf("bar widths are not to scale: build/discovery = %.3f, want %.3f", ratio, 183.0/90.0)
	}
	if !board.above || discovery.above || build.above {
		t.Errorf("sides: board above=%v, discovery above=%v, build above=%v; want moments above and durations below", board.above, discovery.above, build.above)
	}
	if got := trackDateText(discovery.act, discovery.act.StartDate, discovery.act.EndDate); got != "Jan – Apr 2025" {
		t.Errorf("date range = %q, want %q", got, "Jan – Apr 2025")
	}
}

// An authored label_position keeps the row-and-grid layout, axis ticks and all.
func TestTimelineTrack_LabelPositionKeepsTheRowLayout(t *testing.T) {
	req := &RequestEnvelope{Type: "timeline", Data: map[string]any{
		"label_position": "above",
		"items": []any{
			map[string]any{"date": "2024-01-15", "title": "Kick-off"},
			map[string]any{"date": "2024-06-15", "title": "Go-live"},
		},
	}, Output: OutputSpec{Width: 899, Height: 315}}
	doc, err := (&Timeline{NewBaseDiagram("timeline")}).Render(req)
	if err != nil {
		t.Fatal(err)
	}
	if svg := doc.String(); !strings.Contains(svg, "&#39;24") && !strings.Contains(svg, "'24") {
		t.Error("label_position should select the row layout with its date axis")
	}
}
