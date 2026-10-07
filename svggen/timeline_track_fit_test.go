package svggen

import (
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/svggen/core"
)

// governanceTimeline is the showcase slide of go-slide-creator-aqc39: three
// moments above the axis and three durations, two of them back to back, whose
// labels go below it.
func governanceTimeline(width, height float64) *RequestEnvelope {
	return &RequestEnvelope{Type: "timeline", Data: map[string]any{
		"items": []any{
			map[string]any{"title": "EU AI Act phase-in", "start_date": "2024-08-01", "end_date": "2026-08-02"},
			map[string]any{"title": "Internal policy rollout", "start_date": "2025-01-15", "end_date": "2025-09-30"},
			map[string]any{"title": "Audit programme", "start_date": "2025-10-01", "end_date": "2026-06-30"},
			map[string]any{"title": "In force", "date": "2024-08-01"},
			map[string]any{"title": "GPAI duties", "date": "2025-08-02"},
			map[string]any{"title": "General application", "date": "2026-08-02"},
		},
	}, Output: OutputSpec{Width: int(width), Height: int(height)}}
}

// A timeline in a compose zone of about 60% of the body keeps every label:
// the track gives up air and lanes before a label leaves the canvas. It used
// to keep its roomy lanes, so the lowest label was clipped by the zone or
// fell outside it while its bar and leader stayed, and nothing was reported.
func TestTimelineTrack_ShortZoneKeepsEveryLabel(t *testing.T) {
	for _, size := range [][2]float64{{880, 200}, {780, 190}, {700, 180}} {
		b, doc, err := (&Timeline{NewBaseDiagram("timeline")}).RenderWithBuilder(governanceTimeline(size[0], size[1]))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range b.Findings() {
			if f.Code == FindingLabelTruncated && f.Severity != "info" {
				t.Errorf("%.0fx%.0f: %s", size[0], size[1], f.Message)
			}
		}
		svg := doc.String()
		for _, want := range []string{"In force", "GPAI duties", "General application", "Audit programme", "1 Aug 2024 – 2 Aug 2026"} {
			if !strings.Contains(svg, ">"+want+"<") {
				t.Errorf("%.0fx%.0f: track is missing %q", size[0], size[1], want)
			}
		}
	}
}

// The fit itself: on a canvas the roomy track does not fit, the fitted track
// does.
func TestTimelineTrack_FitHeightGivesUpAirThenLanes(t *testing.T) {
	const width, height = 780.0, 150.0
	b := NewSVGBuilder(width, height)
	assumeSlidePlacement(b, &RequestEnvelope{})
	tc := NewTimelineChart(b, DefaultTimelineConfig(width, height))
	data, err := parseTimelineData(governanceTimeline(width, height))
	if err != nil {
		t.Fatal(err)
	}
	body := b.StyleGuide().Typography.SizeSmall
	tt := newTrackType(body, body*trackTitleStep)
	plot := Rect{X: trackEdgePad, Y: trackEdgePad, W: width - 2*trackEdgePad, H: height - 2*trackEdgePad}
	items, barLanes := tc.trackLayout(data, plot, tt)
	need := func(tt trackType, items []trackItem, barLanes int) float64 {
		_, above, below := trackSideHeights(items, tt, 0, tt.barsHeight(barLanes))
		return above + below + trackAxisWidth
	}
	if barLanes != 2 {
		t.Errorf("bar lanes = %d, want 2: a duration that starts the day after another ends shares its lane", barLanes)
	}
	if roomy := need(tt, items, barLanes); roomy <= plot.H {
		t.Fatalf("the roomy track needs %.0fpt of %.0fpt: the canvas is not short enough to test the fit", roomy, plot.H)
	}
	fitTT, fitItems, fitLanes := tc.trackFitHeight(data, plot, tt, items, barLanes)
	if got := need(fitTT, fitItems, fitLanes); got > plot.H+trackFitSlack {
		t.Errorf("the fitted track needs %.0fpt of %.0fpt", got, plot.H)
	}
	if fitTT.leader != trackTightLeader {
		t.Errorf("leader = %.2f, want the tight %.2f", fitTT.leader, trackTightLeader)
	}
}

// A canvas too short for any arrangement leaves the label out with its
// leader and says so, blocking, at the item.
func TestTimelineTrack_UndrawnLabelIsReported(t *testing.T) {
	b, _, err := (&Timeline{NewBaseDiagram("timeline")}).RenderWithBuilder(governanceTimeline(780, 70))
	if err != nil {
		t.Fatal(err)
	}
	reported := 0
	for _, f := range b.Findings() {
		if f.Code == FindingLabelTruncated && strings.Contains(f.Message, "is not drawn") {
			reported++
			if f.Severity != core.SeverityShrinkOrSplit || f.Field == "" {
				t.Errorf("undrawn label finding = %+v, want shrink_or_split at the item's field", f)
			}
		}
	}
	if reported == 0 {
		t.Error("a 70pt canvas drew six labels and reported none missing")
	}
}
