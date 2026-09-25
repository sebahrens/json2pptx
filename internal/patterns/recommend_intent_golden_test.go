package patterns

import "testing"

// These plain-language intents are the advisory contract, not examples of the
// scoring implementation. A wrong top-1 sends an agent down the wrong visual
// authoring path even when every candidate is technically renderable.
func TestRecommendIntentTopOneGolden(t *testing.T) {
	reg := Default()
	patternCases := []struct {
		intent string
		hints  *ContentHints
		want   string
	}{
		{"one big number that matters", nil, "stat-hero"},
		{"one big number that matters", &ContentHints{ItemCount: 1, HasMetrics: true}, "stat-hero"},
		{"show 3 KPIs", &ContentHints{ItemCount: 3, HasMetrics: true}, "kpi-3up"},
		{"display 4 statistics", &ContentHints{ItemCount: 4, HasMetrics: true}, "kpi-4up"},
		{"explain the 5-year transformation phases", nil, "phase-roadmap"},
		{"current state assessment", nil, "before-after"},
		{"before and after the operating model change", nil, "before-after"},
		{"customer quotes", &ContentHints{ItemCount: 4}, "quote-cluster"},
		{"one customer testimonial", &ContentHints{ItemCount: 1}, "pull-quote"},
		{"compare 3 vendor options on 5 criteria", &ContentHints{ItemCount: 3}, "table-highlight"},
		{"pros and cons of outsourcing", nil, "comparison-2col"},
		{"our advisors", nil, "team-bios"},
		{"quarterly release plan across four workstreams", nil, "roadmap-phased"},
		{"project roadmap with milestones", nil, "timeline-horizontal"},
		{"four capabilities with icons", &ContentHints{ItemCount: 4}, "icon-row"},
		{"automation potential by function", nil, "capability-heatmap"},
		{"capability map of AI potential across five functions", &ContentHints{ItemCount: 5}, "capability-heatmap"},
		{"value chain matrix rated high medium low", nil, "capability-heatmap"},
		{"heatmap", nil, "capability-heatmap"},
		{"change management framework", nil, "framework-grid"},
		{"growth levers by dimension", &ContentHints{ItemCount: 3}, "framework-grid"},
		{"framework", nil, "framework-grid"},
	}
	for _, tc := range patternCases {
		t.Run("pattern/"+tc.intent, func(t *testing.T) {
			got := Recommend(reg, tc.intent, tc.hints, 3)
			if len(got.Candidates) == 0 || got.Candidates[0].PatternName != tc.want {
				t.Errorf("top = %+v, want %s", got.Candidates, tc.want)
			}
		})
	}

	visualCases := []struct {
		intent string
		hints  *VisualHints
		want   string
	}{
		{"one big number that matters", nil, "stat-hero"},
		{"show 3 KPIs", &VisualHints{ContentHints: ContentHints{ItemCount: 3, HasMetrics: true}}, "kpi-3up"},
		{"explain the 5-year transformation phases", nil, "phase-roadmap"},
		{"Q3 revenue by region trend over the last 8 quarters", &VisualHints{DataPoints: 8}, "line"},
		{"monthly revenue for 24 months across 3 product tiers with a target line", &VisualHints{DataPoints: 24, SeriesCount: 3}, "line"},
		{"budget allocation by department this quarter", &VisualHints{DataPoints: 5}, "pie"},
		{"revenue by product", &VisualHints{DataPoints: 8}, "bar"},
		{"compare market share of 8 competitors", &VisualHints{DataPoints: 8}, "bar"},
		{"EBITDA moved from 1250 to 1530 by drivers: volume, price, cost and FX", nil, "waterfall"},
		{"current state assessment", nil, "before-after"},
		{"customer quotes", &VisualHints{ContentHints: ContentHints{ItemCount: 4}}, "quote-cluster"},
		{"one customer testimonial", &VisualHints{ContentHints: ContentHints{ItemCount: 1}}, "pull-quote"},
		{"compare 3 vendor options on 5 criteria", &VisualHints{ContentHints: ContentHints{ItemCount: 3}}, "table-highlight"},
		{"team introductions with photos", nil, "team-bios"},
		{"process flow with three decision steps", nil, "process-flow"},
		{"business model canvas", nil, "bmc-canvas"},
	}
	for _, tc := range visualCases {
		t.Run("visual/"+tc.intent, func(t *testing.T) {
			got := RecommendVisual(reg, tc.intent, tc.hints, 3)
			if len(got.Candidates) == 0 || got.Candidates[0].Name != tc.want {
				t.Errorf("top = %+v, want %s", got.Candidates, tc.want)
			}
		})
	}
}

func TestRecommendDoesNotMatchAccidentalSubstrings(t *testing.T) {
	for _, recommend := range []struct {
		name string
		list func() []string
	}{
		{"pattern", func() []string {
			result := Recommend(Default(), "ERP consolidation risks", nil, 5)
			names := make([]string, 0, len(result.Candidates))
			for _, c := range result.Candidates {
				names = append(names, c.PatternName)
			}
			return names
		}},
		{"visual", func() []string {
			result := RecommendVisual(Default(), "ERP consolidation risks", nil, 5)
			names := make([]string, 0, len(result.Candidates))
			for _, c := range result.Candidates {
				names = append(names, c.Name)
			}
			return names
		}},
	} {
		t.Run(recommend.name, func(t *testing.T) {
			for _, name := range recommend.list() {
				if name == "comparison-2col" {
					t.Fatal("'cons' in 'consolidation' matched pros/cons comparison")
				}
			}
		})
	}
}

func TestRecommendVisualDataPointsExcludeOverloadedPie(t *testing.T) {
	hints := &VisualHints{DataPoints: 8, SeriesCount: 1}
	intent := "compare market share of 8 competitors"
	full := RecommendVisual(Default(), intent, hints, 8)
	for _, c := range full.Candidates {
		if c.Name == "pie" || c.Name == "donut" {
			t.Fatalf("overloaded %s leaked into normal recommendations: %+v", c.Name, full.Candidates)
		}
	}
	shortlist := RecommendVisual(Default(), intent, hints, 8, &RecommendOptions{Candidates: []string{"pie", "bar", "donut"}})
	if len(shortlist.Candidates) != 3 || shortlist.Candidates[0].Name != "bar" || shortlist.Candidates[1].Score >= 0.5 || shortlist.Candidates[2].Score >= 0.5 {
		t.Fatalf("shortlist scoring disagrees with overload rule: %+v", shortlist.Candidates)
	}
}
