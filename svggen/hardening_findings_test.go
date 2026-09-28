package svggen

import (
	"regexp"
	"strings"
	"testing"
)

func findingByCode(t *testing.T, req *RequestEnvelope, code string) *Finding {
	t.Helper()
	findings, err := DryRender(req)
	if err != nil {
		t.Fatalf("dry render: %v", err)
	}
	for i := range findings {
		if findings[i].Code == code {
			return &findings[i]
		}
	}
	return nil
}

// go-slide-creator-7w2ed: a gauge value outside [min, max] pinned the needle
// at the end of the dial while the label printed the real value, silently.
func TestGauge_OutOfRangeValueIsReported(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value float64
		want  bool
	}{
		{"above_max", 500, true},
		{"below_min", -5, true},
		{"in_range", 75, false},
		{"at_max", 100, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &RequestEnvelope{Type: "gauge_chart", Data: map[string]any{
				"value": tc.value, "min": 0.0, "max": 100.0,
			}}
			f := findingByCode(t, req, FindingPointOutOfRange)
			if tc.want && f == nil {
				t.Fatalf("value %g on a 0-100 gauge emitted no %s", tc.value, FindingPointOutOfRange)
			}
			if !tc.want && f != nil {
				t.Fatalf("in-range value %g emitted %s: %s", tc.value, f.Code, f.Message)
			}
		})
	}
}

// A funnel with a stage larger than the one above it rendered without
// comment.
func TestFunnel_IncreasingStageIsReported(t *testing.T) {
	rising := &RequestEnvelope{Type: "funnel_chart", Data: map[string]any{
		"stages": []any{
			map[string]any{"label": "A", "value": 1.0},
			map[string]any{"label": "B", "value": 100.0},
			map[string]any{"label": "C", "value": 50.0},
		},
	}}
	f := findingByCode(t, rising, FindingFunnelStageIncrease)
	if f == nil {
		t.Fatalf("increasing funnel emitted no %s", FindingFunnelStageIncrease)
	}
	if !strings.Contains(f.Message, `"B"`) || strings.Contains(f.Message, `"C" (50)`) {
		t.Errorf("finding should name B only: %s", f.Message)
	}

	narrowing := &RequestEnvelope{Type: "funnel_chart", Data: map[string]any{
		"stages": []any{
			map[string]any{"label": "A", "value": 100.0},
			map[string]any{"label": "B", "value": 50.0},
			map[string]any{"label": "C", "value": 50.0},
		},
	}}
	if f := findingByCode(t, narrowing, FindingFunnelStageIncrease); f != nil {
		t.Errorf("non-increasing funnel emitted %s: %s", f.Code, f.Message)
	}
}

// A timeline whose dates do not parse used to derive its axis from
// time.Now(): the output changed every month, the axis printed invented
// dates, and nothing was reported.
func TestTimeline_UnparseableDatesAreReportedAndDeterministic(t *testing.T) {
	build := func() *RequestEnvelope {
		return &RequestEnvelope{Type: "timeline", Data: map[string]any{
			"events": []any{
				map[string]any{"date": "soon", "title": "A"},
				map[string]any{"date": "later", "title": "B"},
			},
		}}
	}
	f := findingByCode(t, build(), FindingInvalidTimeFormat)
	if f == nil {
		t.Fatalf("unparseable dates emitted no %s", FindingInvalidTimeFormat)
	}
	for _, want := range []string{`"soon"`, `"later"`, "no date axis"} {
		if !strings.Contains(f.Message, want) {
			t.Errorf("finding message missing %s: %s", want, f.Message)
		}
	}

	doc, err := Render(build())
	if err != nil {
		t.Fatal(err)
	}
	svg := string(doc.Content)
	// Only the two event labels may be drawn: no invented date tick labels.
	for _, m := range regexp.MustCompile(`<tspan[^>]*>([^<]*)</tspan>`).FindAllStringSubmatch(svg, -1) {
		if m[1] != "A" && m[1] != "B" {
			t.Errorf("dateless timeline drew %q (an invented date axis label?)", m[1])
		}
	}
	again, err := Render(build())
	if err != nil {
		t.Fatal(err)
	}
	if string(again.Content) != svg {
		t.Error("dateless timeline renders non-deterministically")
	}
}

// Phase names written in "date" with no other label are an accepted idiom:
// the string becomes the label and no finding is raised.
func TestTimeline_DateAsLabelIdiomIsNotReported(t *testing.T) {
	req := &RequestEnvelope{Type: "timeline", Data: map[string]any{
		"events": []any{
			map[string]any{"date": "Phase 1: Foundation"},
			map[string]any{"date": "Phase 2: Scale"},
		},
	}}
	if f := findingByCode(t, req, FindingInvalidTimeFormat); f != nil {
		t.Errorf("date-as-label idiom emitted %s: %s", f.Code, f.Message)
	}
}
