package patterns

import "testing"

func TestRecommendVisualAttritionAndConversionNeedStages(t *testing.T) {
	for _, intent := range []string{
		"show how our 5 regions compare on on-time delivery and attrition, with the North highlighted",
		"Compare employee attrition across five departments",
		"Compare conversion rates across regions",
	} {
		t.Run(intent, func(t *testing.T) {
			result := RecommendVisual(Default(), intent, nil, 5)
			if len(result.Candidates) == 0 || result.Candidates[0].Name == "funnel" {
				t.Fatalf("metric comparison needs a comparison visual: %+v", result.Candidates)
			}
			for _, c := range result.Candidates {
				if c.Name == "funnel" {
					t.Errorf("non-sequential metric recommended as funnel: %+v", c)
				}
			}
			shortlist := RecommendVisual(Default(), intent, nil, 5, &RecommendOptions{Candidates: []string{"funnel", "bar"}})
			for _, c := range shortlist.Candidates {
				if c.Name == "funnel" && c.Score > 0 {
					t.Errorf("shortlist bypasses stage requirement: %+v", c)
				}
			}
		})
	}
	for _, intent := range []string{"Recruitment attrition through the hiring pipeline", "Conversion and drop-off by stage", "A funnel of four conversion stages"} {
		result := RecommendVisual(Default(), intent, nil, 5, &RecommendOptions{Candidates: []string{"funnel", "bar"}})
		if len(result.Candidates) == 0 || result.Candidates[0].Name != "funnel" || result.Candidates[0].Score < 0.9 {
			t.Errorf("lost explicit funnel intent %q: %+v", intent, result.Candidates)
		}
	}
}
