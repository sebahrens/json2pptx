package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestBMCCellBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := bmcProbeValues()
	values.KeyActivities.Bullets = []string{strings.Repeat("long ", 30), "short", "short"}
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range schemaMaximaTemplates {
		t.Run(name, func(t *testing.T) {
			layouts, width, height := schemaMaximaLayouts(t, name)
			deck := &PresentationInput{Template: name, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title",
				Pattern: &PatternInput{Name: "bmc-canvas", Values: encoded},
			}}}
			found := false
			for _, finding := range collectFitFindings(deck, layouts, width, height, nil) {
				if finding.Code == patterns.ErrCodeBodyTooLong &&
					strings.Contains(finding.Message, "key_activities.bullets[0]") &&
					strings.Contains(finding.Message, "about 52 characters") {
					found = true
				}
			}
			if !found {
				t.Fatal("BMC copy target missing from fit report")
			}
		})
	}
}

// postExpandDeck builds a two-slide deck whose patterns both object to their
// own content: a chart panel with no chart, and two bios over the budget.
func postExpandDeck() *PresentationInput {
	const longBio = "Jane has spent more than ten years designing and running supply-chain strategy programmes for retailers and industrial groups across Europe and Asia, most recently at McKinsey."
	return &PresentationInput{
		Slides: []SlideInput{
			{
				SlideType: "content",
				Pattern: &PatternInput{
					Name:   "chart-insights-split",
					Values: json.RawMessage(`{"insights":["New business is the larger engine.","Expansion held flat."]}`),
				},
			},
			{
				SlideType: "content",
				Pattern: &PatternInput{
					Name: "team-bios",
					Values: json.RawMessage(`{"members":[
						{"name":"Jane Smith","role":"Project Lead","bio":"` + longBio + `"},
						{"name":"Arun Patel","role":"Lead Engineer","bio":"` + longBio + `"}
					]}`),
				},
			},
		},
	}
}

// codeCounts tallies findings by code.
func codeCounts(findings []patterns.FitFinding) map[string]int {
	out := map[string]int{}
	for _, f := range findings {
		out[f.Code]++
	}
	return out
}

// TestPostExpandWarningsReachTheFitReport is go-slide-creator-wn4v: a pattern's
// own warnings used to be read only by preview_presentation_plan, so validate
// and generate called a degraded deck clean.
func TestPostExpandWarningsReachTheFitReport(t *testing.T) {
	got := codeCounts(collectFitFindings(postExpandDeck(), nil, 0, 0, nil))
	if got[patterns.ErrCodeChartPlaceholderEmpty] != 1 {
		t.Errorf("CHART_PLACEHOLDER_EMPTY count = %d, want 1 (codes: %v)", got[patterns.ErrCodeChartPlaceholderEmpty], got)
	}
	if got[patterns.ErrCodeBodyTooLong] != 2 {
		t.Errorf("BODY_TOO_LONG count = %d, want one per over-budget bio (codes: %v)", got[patterns.ErrCodeBodyTooLong], got)
	}
}

// TestPostExpandWarningsSurviveAnExpandedGrid: on the generate path the slide
// already carries the shape_grid its pattern produced. The collector must still
// read the pattern, or the surface this fixes stays silent.
func TestPostExpandWarningsSurviveAnExpandedGrid(t *testing.T) {
	deck := postExpandDeck()
	for i := range deck.Slides {
		grid := expandSlidePatternGrid(&deck.Slides[i], i, 0, 0, nil)
		if grid == nil {
			t.Fatalf("slide %d did not expand", i)
		}
		deck.Slides[i].ShapeGrid = grid
	}
	got := codeCounts(collectPatternPostExpandFindings(deck, 0, 0, nil))
	if got[patterns.ErrCodeChartPlaceholderEmpty] != 1 || got[patterns.ErrCodeBodyTooLong] != 2 {
		t.Errorf("warnings lost once the grid was expanded: %v", got)
	}
}

// TestPostExpandFindingsAreScopedAndActionable: each finding points at the
// slide's pattern and carries the pattern name, so an agent can act on it.
func TestPostExpandFindingsAreScopedAndActionable(t *testing.T) {
	for _, f := range collectPatternPostExpandFindings(postExpandDeck(), 0, 0, nil) {
		switch f.Code {
		case patterns.ErrCodeChartPlaceholderEmpty:
			if f.Path != "/slides/0/pattern" {
				t.Errorf("chart finding path = %q, want /slides/0/pattern", f.Path)
			}
			if f.Pattern != "chart-insights-split" {
				t.Errorf("chart finding pattern = %q", f.Pattern)
			}
		case patterns.ErrCodeBodyTooLong:
			if f.Path != "/slides/1/pattern" {
				t.Errorf("bio finding path = %q, want /slides/1/pattern", f.Path)
			}
		default:
			t.Errorf("unexpected code %q from the post-expand collector", f.Code)
		}
		if f.Action != "review" {
			t.Errorf("%s action = %q, want review", f.Code, f.Action)
		}
	}
}

// TestPostExpandCollectorQuietOnCleanPatterns: a pattern with nothing to say
// must add nothing.
func TestPostExpandCollectorQuietOnCleanPatterns(t *testing.T) {
	deck := &PresentationInput{Slides: []SlideInput{{
		SlideType: "content",
		Pattern: &PatternInput{
			Name:   "kpi-3up",
			Values: json.RawMessage(`{"cells":[{"big":"$42M","small":"ARR"},{"big":"118%","small":"NRR"},{"big":"4.2%","small":"Churn"}]}`),
		},
	}}}
	if got := collectPatternPostExpandFindings(deck, 0, 0, nil); len(got) != 0 {
		t.Errorf("a clean pattern produced %d finding(s): %+v", len(got), got)
	}
	if got := collectPatternPostExpandFindings(nil, 0, 0, nil); got != nil {
		t.Errorf("nil input should produce nothing, got %+v", got)
	}
}

// TestExpandPatternResultCarriesWarnings: expand_pattern is the tool for
// inspecting what a pattern does with your values, so it reports the pattern's
// own objection to them.
func TestExpandPatternResultCarriesWarnings(t *testing.T) {
	pi := &PatternInput{
		Name:   "chart-insights-split",
		Values: json.RawMessage(`{"insights":["New business is the larger engine.","Expansion held flat."]}`),
	}
	ctx, boundsSource, err := resolveExpandContext("", "")
	if err != nil {
		t.Fatalf("resolve expand context: %v", err)
	}
	res, err := buildPatternExpansionResult(pi, ctx, boundsSource, patterns.Default())
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected the pattern's post-expand warning in the response")
	}
	found := false
	for _, w := range res.Warnings {
		if len(w) > 0 && w[0] == 'C' {
			found = true
		}
	}
	if !found {
		t.Errorf("warnings = %v, want the CHART_PLACEHOLDER_EMPTY line", res.Warnings)
	}
}
