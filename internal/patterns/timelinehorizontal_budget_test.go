package patterns

import (
	"fmt"
	"strings"
	"testing"
)

func TestTimelineHorizontalDenseCopyWarnings(t *testing.T) {
	pat := &timelineHorizontal{}
	for _, tc := range []struct {
		style              string
		stops, label, body int
	}{
		{"dots", 5, 40, 176},
		{"dots", 6, 15, 181},
		{"dots", 7, 60, 76},
	} {
		t.Run(fmt.Sprintf("%s_%d_%d", tc.style, tc.stops, tc.label), func(t *testing.T) {
			v := TimelineHorizontalValues{}
			for i := 0; i < tc.stops; i++ {
				v = append(v, TimelineStop{Label: "Launch", Date: "Q1", Body: "Brief"})
			}
			v[1].Label = strings.Repeat("L", tc.label)
			v[1].Body = strings.Repeat("B", tc.body)
			o := &TimelineHorizontalOverrides{Style: tc.style}
			if got := pat.PostExpandWarnings(ExpandContext{}, &v, o); len(got) != 0 {
				t.Fatalf("at readable budget: %v", got)
			}
			v[1].Body += "B"
			got := pat.PostExpandWarnings(ExpandContext{}, &v, o)
			if len(got) != 1 || !strings.Contains(got[0], ErrCodeBodyTooLong) ||
				!strings.Contains(got[0], "values[1].body") ||
				!strings.Contains(got[0], fmt.Sprintf("about %d", tc.body)) {
				t.Fatalf("over readable budget: %v", got)
			}
		})
	}
	v := TimelineHorizontalValues{}
	for i := 0; i < 7; i++ {
		v = append(v, TimelineStop{Label: "Launch", Date: "Q1"})
	}
	v[2].Date = "Q1"
	o := &TimelineHorizontalOverrides{Style: "chevron"}
	if got := pat.PostExpandWarnings(ExpandContext{}, &v, o); len(got) != 0 {
		t.Fatalf("short date should fit one line: %v", got)
	}
	v[2].Date = strings.Repeat("D", 30)
	got := pat.PostExpandWarnings(ExpandContext{}, &v, o)
	if len(got) != 1 || !strings.Contains(got[0], "values[2].date") || !strings.Contains(got[0], "date row holds 1") {
		t.Fatalf("chevron date warning: %v", got)
	}
	if got := pat.PostExpandWarnings(ExpandContext{}, nil, nil); got != nil {
		t.Fatalf("nil values: %v", got)
	}
	fields := pat.Schema().raw.Properties["values"].raw.Items.raw.Properties
	if fields["body"].raw.MaxLength == nil || *fields["body"].raw.MaxLength != 200 ||
		!strings.Contains(fields["body"].raw.Description, "Chevron body capacity is measured") ||
		!strings.Contains(fields["date"].raw.Description, "one-line row") {
		t.Fatalf("schema loses sparse maximum or dense guidance")
	}
}

func TestTimelineGanttBodyContentIsReported(t *testing.T) {
	pat := &timelineHorizontal{}
	v := TimelineHorizontalValues{
		{Label: "Plan", Date: "Q1", EndDate: "Q2"},
		{Label: "Build", Date: "Q2", EndDate: "Q3", Body: "Critical handoff"},
		{Label: "Launch", Date: "Q3", EndDate: "Q4"},
	}
	got := pat.PostExpandWarnings(ExpandContext{}, &v, &TimelineHorizontalOverrides{Style: "gantt"})
	if len(got) != 1 || !strings.Contains(got[0], ErrCodeContentDropped) ||
		!strings.Contains(got[0], "values[1].body") || !strings.Contains(got[0], "dots or chevron") {
		t.Fatalf("gantt body warning: %v", got)
	}
	v[1].Body = ""
	if got := pat.PostExpandWarnings(ExpandContext{}, &v, &TimelineHorizontalOverrides{Style: "gantt"}); len(got) != 0 {
		t.Fatalf("empty gantt body should not warn: %v", got)
	}
	v[1].Body = "Critical handoff"
	if got := pat.PostExpandWarnings(ExpandContext{}, &v, &TimelineHorizontalOverrides{Style: "dots"}); len(got) != 0 {
		t.Fatalf("rendered dots body should not warn: %v", got)
	}
}

func TestTimelineChevronDateWarningUsesSizeOverride(t *testing.T) {
	v := make(TimelineHorizontalValues, 7)
	for i := range v {
		v[i] = TimelineStop{Label: "Launch", Date: "Q1"}
	}
	v[4].Date = "FY 2025 Close"
	pat := &timelineHorizontal{}
	if got := pat.PostExpandWarnings(ExpandContext{}, &v, &TimelineHorizontalOverrides{Style: "chevron"}); len(got) != 0 {
		t.Fatalf("date at default 12pt should fit: %v", got)
	}
	got := pat.PostExpandWarnings(ExpandContext{}, &v, &TimelineHorizontalOverrides{Style: "chevron", DateSize: 18})
	if len(got) != 1 || !strings.Contains(got[0], "values[4].date") || !strings.Contains(got[0], "at 18pt") {
		t.Fatalf("larger date font should trigger one wrap warning: %v", got)
	}
}
