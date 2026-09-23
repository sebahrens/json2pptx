package patterns

import "testing"

func TestIntentKeywordMatchesWordsAndPhrases(t *testing.T) {
	for _, tc := range []struct {
		intent, keyword string
		want            bool
	}{
		{"current state assessment", "stat", false},
		{"ERP consolidation risks", "cons", false},
		{"our advisors", "vs", false},
		{"six KPIs", "kpi", true},
		{"customer quotes", "customer quote", true},
		{"5-year transformation phases", "phases", true},
		{"pros and cons", "pro/con", false},
		{"pros and cons", "cons", true},
		{"quoted customer feedback", "customer quotes", false},
	} {
		t.Run(tc.intent+"/"+tc.keyword, func(t *testing.T) {
			count, _ := matchIntentKeywords([]string{tc.keyword}, tc.intent)
			if (count > 0) != tc.want {
				t.Errorf("match = %v, want %v", count > 0, tc.want)
			}
		})
	}
}
