package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/shapegrid"
)

// patternRecommendedMax counts grid cells, and timeline-horizontal's dots
// layout emits three per stop (date, dot, label). Its entry was 6 — the
// pattern's own stop maximum, in the wrong unit — so every conforming
// timeline, down to three stops, was reported as overcrowded
// (go-slide-creator-wrsb).
func TestTimelineWithinItsOwnStopLimitIsNotOvercrowded(t *testing.T) {
	stops := make([]map[string]any, 0, 7)
	for _, label := range []string{"Mandate", "Approval", "Wave 1", "Wave 2", "Wave 3", "Cutover", "Deadline"} {
		stops = append(stops, map[string]any{"label": label, "date": "Q1 2026"})
	}
	values, err := json.Marshal(stops)
	if err != nil {
		t.Fatal(err)
	}
	input := &PresentationInput{Slides: []SlideInput{{
		SlideType: "content",
		LayoutID:  "blank-title",
		Pattern:   &PatternInput{Name: "timeline-horizontal", Values: values},
	}}}

	expanded, _ := expandPatternsForFit(input, shapegrid.DefaultSlideWidthEMU, shapegrid.DefaultSlideHeightEMU, nil)
	for _, f := range collectGridOccupancyFindings(expanded) {
		if strings.Contains(string(f.Code), "overcrowded") {
			t.Errorf("a 7-stop timeline — the pattern's own maximum — was reported overcrowded: %s", f.Message)
		}
	}
}
