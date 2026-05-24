package svggen

import (
	"testing"
)

// TestTimeline_StartOnlyActivities_DateRangeNotCollapsed reproduces the bug
// where activities carry only a start date (no end date). With the old code
// maxDate stayed at the zero time, producing a nonsensical range spanning back
// to year 0001 — every real date then mapped to ~the same pixel and all bars
// stacked at plotArea.X. calculateDateRange must instead yield a positive
// duration that spans the activity start dates.
func TestTimeline_StartOnlyActivities_DateRangeNotCollapsed(t *testing.T) {
	tc := &TimelineChart{config: DefaultTimelineConfig(600, 300)}

	data := TimelineData{
		Activities: []TimelineActivity{
			{Type: TimelineActivityTypeActivity, StartDate: date(2026, 1, 1)},
			{Type: TimelineActivityTypeActivity, StartDate: date(2026, 6, 1)},
			{Type: TimelineActivityTypeActivity, StartDate: date(2026, 12, 1)},
		},
	}

	dr := tc.calculateDateRange(data)

	if dr.duration <= 0 {
		t.Fatalf("expected positive duration for start-only activities, got %v (start=%v end=%v)",
			dr.duration, dr.start, dr.end)
	}
	if !dr.end.After(dr.start) {
		t.Errorf("expected end (%v) after start (%v)", dr.end, dr.start)
	}
	// The range must actually span the activity dates, not collapse near year 0001.
	if !dr.start.Before(date(2026, 1, 2)) || !dr.end.After(date(2026, 11, 30)) {
		t.Errorf("range [%v, %v] does not span the activity start dates Jan..Dec 2026", dr.start, dr.end)
	}
}

// TestTimeline_StartOnlyBecomesMilestone verifies that an event carrying only a
// start date (no end date, no explicit milestone date) is normalized into a
// point-in-time milestone — semantically, an event with only a start IS a
// milestone. Real duration activities and existing milestones are left intact,
// and the caller's slice is not mutated.
func TestTimeline_StartOnlyBecomesMilestone(t *testing.T) {
	orig := []TimelineActivity{
		{Type: TimelineActivityTypeActivity, StartDate: date(2026, 1, 1)},                          // start only
		{Type: TimelineActivityTypeActivity, StartDate: date(2026, 6, 1), EndDate: date(2026, 9, 1)}, // real activity
		{Type: TimelineActivityTypeMilestone, Date: date(2026, 12, 1)},                             // already a milestone
		{Type: TimelineActivityTypePhase, StartDate: date(2026, 1, 1)},                             // phase, leave alone
	}

	got := normalizeTimelineActivities(orig)

	if got[0].Type != TimelineActivityTypeMilestone {
		t.Errorf("start-only activity should become a milestone, got %q", got[0].Type)
	}
	if !got[0].Date.Equal(date(2026, 1, 1)) {
		t.Errorf("milestone Date should be the start date, got %v", got[0].Date)
	}
	if got[1].Type != TimelineActivityTypeActivity {
		t.Errorf("duration activity should stay an activity, got %q", got[1].Type)
	}
	if got[2].Type != TimelineActivityTypeMilestone || !got[2].Date.Equal(date(2026, 12, 1)) {
		t.Errorf("existing milestone should be untouched, got type=%q date=%v", got[2].Type, got[2].Date)
	}
	if got[3].Type != TimelineActivityTypePhase {
		t.Errorf("phase should stay a phase, got %q", got[3].Type)
	}

	// The caller's slice must not be mutated.
	if orig[0].Type != TimelineActivityTypeActivity {
		t.Errorf("normalizeTimelineActivities must not mutate the input slice; orig[0].Type=%q", orig[0].Type)
	}
}
