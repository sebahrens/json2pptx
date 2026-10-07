package svggen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// programmeTimelineRequest is the go-slide-creator-a6sux review case: four
// overlapping programme phases and two milestones that fall on bar ends.
func programmeTimelineRequest() *RequestEnvelope {
	return &RequestEnvelope{Type: "timeline", Title: "Programme timeline", Data: map[string]any{
		"activities": []any{
			map[string]any{"label": "Discovery and diagnostic", "start_date": "2026-01-15", "end_date": "2026-03-31"},
			map[string]any{"label": "Target operating model design", "start_date": "2026-03-01", "end_date": "2026-06-30"},
			map[string]any{"label": "Pilot in two markets", "start_date": "2026-06-01", "end_date": "2026-10-31"},
			map[string]any{"label": "Global rollout", "start_date": "2026-10-01", "end_date": "2027-06-30"},
		},
		"milestones": []any{
			map[string]any{"label": "Board approval", "date": "2026-04-15"},
			map[string]any{"label": "Go-live wave 1", "date": "2026-11-01"},
		},
	}, Output: OutputSpec{Width: 800, Height: 400}}
}

var (
	timelineBarPath   = regexp.MustCompile(`<path d="M([\d.]+) ([\d.]+)H([\d.]+)V([\d.]+)H[\d.]+z" fill="(#[0-9a-f]{6})"`)
	timelineDiscPath  = regexp.MustCompile(`<path d="M([\d.]+) ([\d.]+)A([\d.]+) [\d.]+ 0 00[\d.]+ [\d.]+A[\d.]+ [\d.]+ 0 00[\d.]+ [\d.]+z" fill="(#[0-9a-f]{6})"`)
	timelineLabelSize = regexp.MustCompile(`font-size:([\d.]+)px;font-weight:700[^>]*><tspan[^>]*>([^<]+)<`)
)

func num(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestTimeline_OneAccentFlatBarsAndMilestoneBand pins go-slide-creator-a6sux
// on the milestone track (go-slide-creator-o8cqh): every bar shares one fill
// and every milestone marker another (no index-cycled rainbow), bars are flat
// rectangles hanging from the axis, milestone discs sit on the axis line
// where none intersects a bar, and every event name is drawn at one size.
func TestTimeline_OneAccentFlatBarsAndMilestoneBand(t *testing.T) {
	doc, err := (&Timeline{NewBaseDiagram("timeline")}).Render(programmeTimelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	svg := doc.String()

	type box struct{ x0, y0, x1, y1 float64 }
	var bars []box
	barFills := map[string]bool{}
	for _, m := range timelineBarPath.FindAllStringSubmatch(svg, -1) {
		x0, y1, x1, y0 := num(t, m[1]), num(t, m[2]), num(t, m[3]), num(t, m[4])
		bars = append(bars, box{x0, y0, x1, y1})
		barFills[m[5]] = true
	}
	if len(bars) != 4 {
		t.Fatalf("found %d flat bars, want 4 (rounded bars are not flat rectangles)", len(bars))
	}
	if len(barFills) != 1 {
		t.Errorf("bars use %d fills %v, want one accent tint", len(barFills), barFills)
	}

	// Each marker is a background-coloured ring under an accent disc.
	discFills := map[string]bool{}
	discs := 0
	for _, m := range timelineDiscPath.FindAllStringSubmatch(svg, -1) {
		if m[4] == "#ffffff" || m[4] == "#fff" {
			continue
		}
		discs++
		discFills[m[4]] = true
		if barFills[m[4]] {
			t.Errorf("milestone disc shares the bars' fill %s", m[4])
		}
		cy, r := num(t, m[2]), num(t, m[3])
		for _, bar := range bars {
			if cy >= bar.y0 || r <= 0 {
				t.Errorf("milestone disc centre y %.1f is not above bar %+v: discs sit on the axis, bars hang below it", cy, bar)
			}
		}
	}
	if discs != 2 {
		t.Fatalf("found %d milestone discs, want 2", discs)
	}
	if len(discFills) != 1 {
		t.Errorf("milestones use %d fills %v, want one", len(discFills), discFills)
	}

	sizes := map[string][]string{}
	for _, m := range timelineLabelSize.FindAllStringSubmatch(svg, -1) {
		if m[2] == "Programme timeline" {
			continue
		}
		sizes[m[1]] = append(sizes[m[1]], m[2])
	}
	if len(sizes) != 1 {
		t.Errorf("event names are drawn at %d sizes, want one: %v", len(sizes), sizes)
	}
	for _, label := range []string{"Discovery and diagnostic", "Board approval", "Go-live wave 1"} {
		if !strings.Contains(svg, ">"+label+"<") {
			t.Errorf("label %q missing or truncated", label)
		}
	}
}
