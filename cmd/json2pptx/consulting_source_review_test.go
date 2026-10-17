package main

import (
	"github.com/sebahrens/json2pptx/internal/patterns"
	"testing"
)

func TestConsultingNumericVisualsRequireActualSources(t *testing.T) {
	for _, name := range []string{"progress-list", "numeric-heatmap"} {
		t.Run(name, func(t *testing.T) {
			input := &PresentationInput{Slides: []SlideInput{{SlideType: "content", Pattern: &PatternInput{Name: name}}}}
			if fs := findingsByCode(collectDataWithoutSourceFindings(input), patterns.ErrCodeDataWithoutSource); len(fs) != 1 {
				t.Fatalf("numeric pattern missed: %+v", fs)
			}
			input.Slides[0].Source = "Finance plan, FY26"
			if fs := collectDataWithoutSourceFindings(input); len(fs) != 0 {
				t.Fatalf("authored source ignored: %+v", fs)
			}
		})
	}
	for _, name := range []string{"mekko", "marimekko", "mekko_chart", "combo", "combo_chart", "butterfly", "butterfly_chart"} {
		if !isChartishDiagramType(name) {
			t.Errorf("numeric chart alias missed: %s", name)
		}
	}
}
