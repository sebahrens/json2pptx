package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func dataWithoutSourceSlides(t *testing.T, raw string) *PresentationInput {
	t.Helper()
	var input PresentationInput
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &input
}

func dataWithoutSourceCodes(findings []patterns.FitFinding) map[string]bool {
	out := map[string]bool{}
	for _, f := range findings {
		if f.Code == patterns.ErrCodeDataWithoutSource {
			out[f.Path] = true
		}
	}
	return out
}

// TestDataWithoutSourceFinding pins which slides DATA_WITHOUT_SOURCE fires on
// (go-slide-creator-cuszt): chart/KPI/stat patterns, charts and tables of
// figures without a source — and not qualitative patterns, word tables, title
// slides, or slides whose pattern carries a source generation lifts.
func TestDataWithoutSourceFinding(t *testing.T) {
	input := dataWithoutSourceSlides(t, `{"slides":[
	 {"slide_type":"title","layout_id":"title","content":[{"placeholder_id":"title","type":"text","text_value":"Cover 12%"}]},
	 {"layout_id":"content","content":[{"placeholder_id":"title","type":"text","text_value":"Metrics"}],"pattern":{"name":"kpi-3up","values":[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]}},
	 {"layout_id":"content","source":"Company filings","pattern":{"name":"kpi-3up","values":[{"big":"1","small":"a"},{"big":"2","small":"b"},{"big":"3","small":"c"}]}},
	 {"layout_id":"content","pattern":{"name":"stat-hero","values":{"value":"$4M","label":"ARR","source":"Finance"}}},
	 {"layout_id":"content","pattern":{"name":"matrix-2x2","values":{"top_left":{"header":"a"},"top_right":{"header":"b"},"bottom_left":{"header":"c"},"bottom_right":{"header":"d"}}}},
	 {"layout_id":"content","content":[{"placeholder_id":"body","type":"table","table_value":{"headers":["Role","Owner"],"rows":[["Lead","Ana"]]}}]},
	 {"layout_id":"content","content":[{"placeholder_id":"body","type":"table","table_value":{"headers":["Region","Revenue"],"rows":[["EU","12.4"]]}}]},
	 {"layout_id":"content","compose":{"direction":"horizontal","segments":[{"pattern":{"name":"stat-hero","values":{"value":"9","label":"x"}}},{"pattern":{"name":"icon-row","values":[]}}]}}
	]}`)
	got := dataWithoutSourceCodes(collectDataWithoutSourceFindings(input))
	want := map[string]bool{"/slides/1/source": true, "/slides/6/source": true, "/slides/7/source": true}
	for p := range want {
		if !got[p] {
			t.Errorf("DATA_WITHOUT_SOURCE missing at %s (got %v)", p, got)
		}
	}
	for p := range got {
		if !want[p] {
			t.Errorf("unexpected DATA_WITHOUT_SOURCE at %s", p)
		}
	}
}

// TestDataWithoutSourceReachesFitReport checks the collector is wired into the
// shared fit-report path every surface calls.
func TestDataWithoutSourceReachesFitReport(t *testing.T) {
	data, err := os.ReadFile("../../examples/board-deck.json")
	if err != nil {
		t.Fatal(err)
	}
	var input PresentationInput
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	for i := range input.Slides {
		input.Slides[i].Source = ""
	}
	findings := collectFitFindings(&input, nil, 12192000, 6858000, nil)
	if len(dataWithoutSourceCodes(findings)) == 0 {
		t.Fatalf("collectFitFindings emitted no DATA_WITHOUT_SOURCE on an unsourced board deck")
	}
}

// TestLiftPatternSourceMovesSourceToSlide pins the lift: a stat-hero source
// becomes the slide's source (merged with, not duplicating, an existing one)
// and leaves the pattern values.
func TestLiftPatternSourceMovesSourceToSlide(t *testing.T) {
	input := dataWithoutSourceSlides(t, `{"slides":[
	 {"layout_id":"content","pattern":{"name":"stat-hero","values":{"value":"$4M","label":"ARR","source":"Finance"}}},
	 {"layout_id":"content","source":"Source: Finance","pattern":{"name":"stat-hero","values":{"value":"$4M","label":"ARR","source":"Finance"}}},
	 {"layout_id":"content","source":"Board pack","pattern":{"name":"chart-insights-split","values":{"insights":["a"],"source":"CRM export"}}}
	]}`)
	applyDefaults(input)
	wants := []string{"Finance", "Source: Finance", "Board pack; CRM export"}
	for i, want := range wants {
		s := input.Slides[i]
		if s.Source != want {
			t.Errorf("slide %d source = %q, want %q", i, s.Source, want)
		}
		if strings.Contains(string(s.Pattern.Values), `"source"`) {
			t.Errorf("slide %d pattern still carries a source: %s", i, s.Pattern.Values)
		}
	}
}
