package patterns

import (
	"strings"
	"testing"
)

// TestRecommend_EvalSet verifies that the top-1 recommendation matches
// human-rated "best fit" on at least 80% of a 20-prompt eval set.
func TestRecommend_EvalSet(t *testing.T) {
	reg := NewRegistry()
	// Register stubs for all shipped patterns so Recommend can reference them.
	// The rules table uses pattern names directly, so we only need stubs.
	for _, name := range []string{
		"bmc-canvas", "card-grid", "comparison-2col", "icon-row",
		"kpi-3up", "kpi-4up", "matrix-2x2", "timeline-horizontal",
	} {
		reg.Register(&stubPattern{name: name, desc: name, useWhen: name, version: 1})
	}

	type evalCase struct {
		intent  string
		hints   *ContentHints
		wantTop string // expected top-1 pattern
	}

	cases := []evalCase{
		// KPI intents
		{intent: "show 3 KPIs", hints: &ContentHints{ItemCount: 3, HasMetrics: true}, wantTop: "kpi-3up"},
		{intent: "key metrics dashboard with 4 numbers", hints: &ContentHints{ItemCount: 4, HasMetrics: true}, wantTop: "kpi-4up"},
		{intent: "revenue, growth, and churn KPI", hints: &ContentHints{ItemCount: 3}, wantTop: "kpi-3up"},
		{intent: "display our top 4 stats", hints: &ContentHints{ItemCount: 4, HasMetrics: true}, wantTop: "kpi-4up"},

		// Comparison intents
		{intent: "compare two options", hints: &ContentHints{Columns: 2}, wantTop: "comparison-2col"},
		{intent: "pros and cons", hints: nil, wantTop: "comparison-2col"},
		{intent: "option A vs option B", hints: nil, wantTop: "comparison-2col"},
		{intent: "advantages and disadvantages of cloud migration", hints: nil, wantTop: "comparison-2col"},

		// BMC
		{intent: "business model canvas", hints: nil, wantTop: "bmc-canvas"},
		{intent: "fill out BMC for our startup", hints: nil, wantTop: "bmc-canvas"},

		// Matrix
		{intent: "2x2 priority matrix", hints: nil, wantTop: "matrix-2x2"},
		{intent: "impact vs effort quadrant", hints: nil, wantTop: "matrix-2x2"},
		{intent: "positioning matrix", hints: nil, wantTop: "matrix-2x2"},

		// Timeline
		{intent: "project roadmap with milestones", hints: nil, wantTop: "timeline-horizontal"},
		{intent: "timeline of product evolution", hints: nil, wantTop: "timeline-horizontal"},

		// Icon row
		{intent: "show 4 features with icons", hints: &ContentHints{ItemCount: 4}, wantTop: "icon-row"},
		{intent: "our three key capabilities", hints: &ContentHints{ItemCount: 3}, wantTop: "icon-row"},

		// Card grid
		{intent: "grid of 6 category cards", hints: &ContentHints{ItemCount: 6}, wantTop: "card-grid"},
		{intent: "team overview cards", hints: nil, wantTop: "card-grid"},

		// Mixed — should still pick the best
		{intent: "schedule milestones on a timeline", hints: nil, wantTop: "timeline-horizontal"},
	}

	correct := 0
	for _, tc := range cases {
		result := Recommend(reg, tc.intent, tc.hints, 3)
		topName := ""
		if len(result.Candidates) > 0 {
			topName = result.Candidates[0].PatternName
		}
		if topName == tc.wantTop {
			correct++
		} else {
			t.Errorf("intent=%q: got top=%q, want=%q (candidates=%v)",
				tc.intent, topName, tc.wantTop, result.Candidates)
		}
	}

	accuracy := float64(correct) / float64(len(cases))
	t.Logf("Eval accuracy: %d/%d (%.0f%%)", correct, len(cases), accuracy*100)
	if accuracy < 0.80 {
		t.Errorf("Eval accuracy %.0f%% is below 80%% threshold", accuracy*100)
	}
}

func TestRecommend_NoMatch(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{
		"bmc-canvas", "card-grid", "comparison-2col", "icon-row",
		"kpi-3up", "kpi-4up", "matrix-2x2", "timeline-horizontal",
	} {
		reg.Register(&stubPattern{name: name, desc: name, useWhen: name, version: 1})
	}

	result := Recommend(reg, "explain quantum computing theory", nil, 3)
	if len(result.Candidates) != 0 {
		t.Errorf("expected empty candidates for unrelated intent, got %v", result.Candidates)
	}
}

func TestRecommend_QueryUnderstood(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&stubPattern{name: "kpi-3up", desc: "kpi", useWhen: "kpi", version: 1})

	hints := &ContentHints{ItemCount: 3, HasMetrics: true}
	result := Recommend(reg, "show KPIs", hints, 3)
	if result.QueryUnderstood == "" {
		t.Error("expected non-empty QueryUnderstood")
	}
	if !strings.Contains(result.QueryUnderstood, "item_count=3") {
		t.Errorf("QueryUnderstood should reflect item_count, got %q", result.QueryUnderstood)
	}
	if !strings.Contains(result.QueryUnderstood, "has_metrics=true") {
		t.Errorf("QueryUnderstood should reflect has_metrics, got %q", result.QueryUnderstood)
	}
}

func TestRecommend_MaxCandidates(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{
		"bmc-canvas", "card-grid", "comparison-2col", "icon-row",
		"kpi-3up", "kpi-4up", "matrix-2x2", "timeline-horizontal",
	} {
		reg.Register(&stubPattern{name: name, desc: name, useWhen: name, version: 1})
	}

	// "feature icon card" should match multiple patterns
	result := Recommend(reg, "feature icon card grid", nil, 2)
	if len(result.Candidates) > 2 {
		t.Errorf("expected at most 2 candidates, got %d", len(result.Candidates))
	}
}

