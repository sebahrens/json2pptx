package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func milestone(label, date, body string) map[string]any {
	m := map[string]any{"label": label}
	if date != "" {
		m["date"] = date
	}
	if body != "" {
		m["body"] = body
	}
	return m
}

func timelineBody(stops ...any) map[string]any {
	return map[string]any{"title": "How we got here", "milestones": stops}
}

// Dated milestones are not a phased roadmap, and the spec only had the latter:
// an author with five dates had to bend them into phases (go-slide-creator-wrsb).
func TestCompileTimelineUsesTheLineWhenItFits(t *testing.T) {
	body := timelineBody(
		milestone("Mandate published", "Mar 2024", "The regulator sets the T+1 date."),
		milestone("Programme approved", "Sep 2024", ""),
		milestone("Wave 1 live", "Jun 2025", ""),
		milestone("Deadline", "May 2027", ""),
	)
	if got := TimelinePattern(body); got != "timeline-horizontal" {
		t.Fatalf("TimelinePattern = %q, want timeline-horizontal", got)
	}
	slide, _, err := CompileTimeline(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Pattern == nil || slide.Pattern.Name != "timeline-horizontal" {
		t.Fatalf("pattern = %+v, want timeline-horizontal", slide.Pattern)
	}
	// The pattern's values are a bare array of stops, not an object.
	var stops []timelineStop
	if err := json.Unmarshal(slide.Pattern.Values, &stops); err != nil {
		t.Fatalf("values %s: %v", slide.Pattern.Values, err)
	}
	if len(stops) != 4 {
		t.Fatalf("compiled %d stops, want 4", len(stops))
	}
	if stops[0].Label != "Mandate published" || stops[0].Date != "Mar 2024" {
		t.Errorf("first stop = %+v", stops[0])
	}
	// Dots, not bars: nothing here spans a period.
	if len(slide.Pattern.Overrides) != 0 {
		t.Errorf("unexpected overrides on a point timeline: %s", slide.Pattern.Overrides)
	}
}

// A milestone with a start and an end is a bar, and the pattern rejects an
// end_date in any style but gantt — so the compiler has to say so.
func TestTimelineWithRangesAsksForGantt(t *testing.T) {
	body := timelineBody(
		map[string]any{"label": "Discovery", "date": "Jan 2025", "end_date": "Mar 2025"},
		map[string]any{"label": "Build", "date": "Apr 2025", "end_date": "Oct 2025"},
		map[string]any{"label": "Cutover", "date": "Nov 2025"},
	)
	slide, _, err := CompileTimeline(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if slide.Pattern == nil {
		t.Fatal("expected the timeline pattern")
	}
	var ovr timelineOverrides
	if err := json.Unmarshal(slide.Pattern.Overrides, &ovr); err != nil {
		t.Fatalf("overrides %s: %v", slide.Pattern.Overrides, err)
	}
	if ovr.Style != "gantt" {
		t.Errorf("style = %q, want gantt", ovr.Style)
	}
}

// Outside the pattern's bounds the dates degrade rather than being truncated,
// and the finding says which bound broke.
func TestTimelineDegradesWithAReason(t *testing.T) {
	cases := []struct {
		name   string
		body   map[string]any
		reason string
	}{
		{
			name:   "two dates are a comparison, not a line",
			body:   timelineBody(milestone("Start", "Jan", ""), milestone("End", "Dec", "")),
			reason: "at least 3",
		},
		{
			name: "eight stops overflow the line",
			body: timelineBody(
				milestone("a", "", ""), milestone("b", "", ""), milestone("c", "", ""), milestone("d", "", ""),
				milestone("e", "", ""), milestone("f", "", ""), milestone("g", "", ""), milestone("h", "", ""),
			),
			reason: "at most 7",
		},
		{
			name: "a label written as a sentence",
			body: timelineBody(
				milestone(strings.Repeat("a", 70), "", ""), milestone("b", "", ""), milestone("c", "", ""),
			),
			reason: "label is 70 characters",
		},
		{
			name: "a body written as a paragraph",
			body: timelineBody(
				milestone("a", "", strings.Repeat("a", 210)), milestone("b", "", ""), milestone("c", "", ""),
			),
			reason: "body is 210 characters",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := TimelinePattern(c.body); got != "" {
				t.Errorf("TimelinePattern = %q, want the bullet fallback", got)
			}
			if over := TimelineOverBudget(c.body); !strings.Contains(over, c.reason) {
				t.Errorf("TimelineOverBudget = %q, want it to mention %q", over, c.reason)
			}
			slide, _, err := CompileTimeline(Input{Body: c.body})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if slide.Pattern != nil {
				t.Errorf("expected the bullet fallback, got pattern %s", slide.Pattern.Name)
			}
			if len(slide.Content) == 0 {
				t.Error("the fallback lost the milestones entirely")
			}
		})
	}
}

// The degrade keeps the dates: a milestone list without them is just a list.
func TestTimelineFallbackKeepsTheDates(t *testing.T) {
	body := timelineBody(
		map[string]any{"label": "Discovery", "date": "Jan 2025", "end_date": "Mar 2025"},
		milestone("Cutover", "Nov 2025", "Two clearers migrate in one weekend."),
	)
	slide, _, err := CompileTimeline(Input{Body: body})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	encoded, err := json.Marshal(slide.Content)
	if err != nil {
		t.Fatalf("marshal content: %v", err)
	}
	for _, want := range []string{"Jan 2025–Mar 2025", "Discovery", "Nov 2025", "Two clearers migrate in one weekend."} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("the fallback dropped %q: %s", want, encoded)
		}
	}
}

// An empty list is the required-field gate's business, not the bounds'.
func TestTimelineWithNoMilestonesReportsNothing(t *testing.T) {
	if over := TimelineOverBudget(map[string]any{"milestones": []any{}}); over != "" {
		t.Errorf("an empty timeline reported %q", over)
	}
	if n := UsableTimelineStopCount(map[string]any{"title": "x"}); n != 0 {
		t.Errorf("UsableTimelineStopCount = %d, want 0", n)
	}
}

// The spellings an author reaches for all resolve, and an entry with no label is
// dropped rather than drawing a blank stop.
func TestTimelineStopAliases(t *testing.T) {
	for _, field := range []string{"milestones", "stops", "events", "timeline"} {
		body := map[string]any{field: []any{"Mandate", "Approval", "Launch"}}
		if n := len(TimelineStops(body)); n != 3 {
			t.Errorf("%s: resolved %d stops, want 3", field, n)
		}
	}
	for _, key := range []string{"date", "date_label", "when", "start", "start_date"} {
		body := map[string]any{"milestones": []any{map[string]any{"label": "Launch", key: "Q1 2026"}}}
		got := TimelineStops(body)
		if len(got) != 1 || got[0].Date != "Q1 2026" {
			t.Errorf("%s: resolved %+v", key, got)
		}
	}
	for _, key := range []string{"end_date", "end", "until"} {
		body := map[string]any{"milestones": []any{map[string]any{"label": "Build", "date": "Q1", key: "Q3"}}}
		got := TimelineStops(body)
		if len(got) != 1 || got[0].EndDate != "Q3" {
			t.Errorf("%s: resolved %+v", key, got)
		}
	}
	body := map[string]any{"milestones": []any{"Mandate", map[string]any{"date": "orphan"}, "Launch"}}
	if n := len(TimelineStops(body)); n != 2 {
		t.Errorf("resolved %d stops, want the 2 with labels", n)
	}
}
