package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func assertBudgetFindingAcrossTemplates(t *testing.T, pattern string, values any, field, target string) {
	t.Helper()
	encoded, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	for _, templateName := range schemaMaximaTemplates {
		t.Run(templateName, func(t *testing.T) {
			layouts, width, height := schemaMaximaLayouts(t, templateName)
			deck := &PresentationInput{Template: templateName, Slides: []SlideInput{{
				SlideType: "content", LayoutID: "blank-title",
				Pattern: &PatternInput{Name: pattern, Values: encoded},
			}}}
			findings := collectFitFindings(deck, layouts, width, height, nil)
			for _, finding := range findings {
				if finding.Code == patterns.ErrCodeBodyTooLong &&
					strings.Contains(finding.Message, field) && strings.Contains(finding.Message, target) {
					return
				}
			}
			t.Fatalf("%s %s budget %q missing from fit report: %+v", pattern, field, target, findings)
		})
	}
}

func TestBMCCellBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := bmcProbeValues()
	values.KeyActivities.Bullets = []string{strings.Repeat("long ", 30), "short", "short"}
	assertBudgetFindingAcrossTemplates(t, "bmc-canvas", values, "key_activities.bullets[0]", "about 52 characters")
}

func TestDriverTreeBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.DriverTreeValues{Root: patterns.DriverTreeNode{Label: "Root"}}
	for _, count := range []int{4, 4, 2} {
		branch := patterns.DriverTreeBranch{Label: "Branch"}
		for i := 0; i < count; i++ {
			branch.Leaves = append(branch.Leaves, "Item")
		}
		values.Branches = append(values.Branches, branch)
	}
	values.Branches[0].Leaves[0] = strings.Repeat("long ", 20)
	assertBudgetFindingAcrossTemplates(t, "driver-tree", values, "branches[0].leaves[0]", "about 75 per leaf")
}
func TestComparisonBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.Comparison2colValues{Headers: [2]string{"Left", "Right"}}
	for i := 0; i < 7; i++ {
		values.Rows = append(values.Rows, patterns.Comparison2colRow{Left: "Item", Right: "Other"})
	}
	values.Rows[6].Right = strings.Repeat("word ", 20)
	assertBudgetFindingAcrossTemplates(t, "comparison-2col", values, "rows[6].right", "about 66 characters")
}
func TestRoadmapPhasedBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.RoadmapPhasedValues{}
	for i := 0; i < 8; i++ {
		values.Phases = append(values.Phases, "Q")
	}
	for i := 0; i < 6; i++ {
		ws := patterns.RoadmapWorkstream{Name: "Team"}
		for j := 0; j < 8; j++ {
			ws.Items = append(ws.Items, "Task")
		}
		values.Workstreams = append(values.Workstreams, ws)
	}
	values.Workstreams[2].Items[4] = strings.Repeat("word ", 10)
	assertBudgetFindingAcrossTemplates(t, "roadmap-phased", values, "workstreams[2].items[4]", "about 32 per activity pill")
}
func TestTeamBiosBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.TeamBiosValues{}
	for i := 0; i < 5; i++ {
		values.Members = append(values.Members, patterns.TeamBiosMember{Name: "Jane Doe", Role: "Lead", Bio: "Short"})
	}
	values.Members[4].Bio = strings.Repeat("word ", 30)
	assertBudgetFindingAcrossTemplates(t, "team-bios", values, "members[4].bio", "about 141 bio characters")
}
func TestSwimlaneBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.SwimlaneValues{}
	for i := 0; i < 6; i++ {
		lane := patterns.SwimlaneLane{Actor: "Team"}
		for j := 0; j < 8; j++ {
			lane.Steps = append(lane.Steps, "Task")
		}
		values.Lanes = append(values.Lanes, lane)
	}
	values.Lanes[5].Steps[7] = strings.Repeat("word ", 10)
	assertBudgetFindingAcrossTemplates(t, "swimlane", values, "lanes[5].steps[7]", "about 32 per step")
}
func TestNumberedStepBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.NumberedStepStripValues{Style: "chevron"}
	for i := 0; i < 6; i++ {
		values.Steps = append(values.Steps, patterns.NumberedStepStripStep{Label: "Step", Body: "Detail"})
	}
	values.Steps[4].Label = strings.Repeat("word ", 10)
	assertBudgetFindingAcrossTemplates(t, "numbered-step-strip", values, "steps[4].label", "about 47")
}
func TestKPIInlineBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := patterns.KPINupValues{}
	for i := 0; i < 5; i++ {
		values = append(values, patterns.KPICell{Big: "42%", Small: "Revenue"})
	}
	values[3].Big = "12345678"
	values[3].Small = strings.Repeat("C", 17)
	values[3].Icon = &patterns.IconRef{Name: "rocket"}
	assertBudgetFindingAcrossTemplates(t, "kpi-inline", values, "values[3].small", "about 16 caption characters")
}

func TestHorizontalBarBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.HorizontalBarCalloutsValues{Unit: "%"}
	for i := 0; i < 8; i++ {
		values.Bars = append(values.Bars, patterns.HorizontalBarCalloutsBar{Label: "Item", Value: 80, Callout: "Insight"})
	}
	values.Bars[3].Callout = strings.Repeat("C", 122)
	assertBudgetFindingAcrossTemplates(t, "horizontal-bar-with-callouts", values, "bars[3].callout", "about 121 characters")
}

func TestAgendaWithImagesBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.AgendaWithImagesValues{}
	for i := 0; i < 5; i++ {
		values.Items = append(values.Items, patterns.AgendaWithImagesItem{Title: "Title", Subtitle: "Summary", ImageLabel: "Image"})
	}
	values.Items[2].Title = strings.Repeat("T", 80)
	values.Items[2].Subtitle = strings.Repeat("S", 76)
	assertBudgetFindingAcrossTemplates(t, "agenda-with-images", values, "items[2].subtitle", "about 75 subtitle characters")
}

func TestAgendaWithImagesFullAgendaWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.AgendaWithImagesValues{}
	for i := 0; i < 6; i++ {
		values.Items = append(values.Items, patterns.AgendaWithImagesItem{Title: "Title", ImageLabel: "Image"})
	}
	values.Items[4].Title = strings.Repeat("T", 80)
	values.Items[4].Subtitle = "Summary"
	assertBudgetFindingAcrossTemplates(t, "agenda-with-images", values, "items[4].subtitle", "no readable subtitle room")
}

func TestBeforeAfterCompactBudgetWarningReachesFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.BeforeAfterValues{
		Before: patterns.BeforeAfterColumn{Header: "Current"},
		After:  patterns.BeforeAfterColumn{Header: "Future"},
	}
	for i := 0; i < 8; i++ {
		item := "Short"
		if i < 3 {
			item = strings.Repeat("word ", 40)
		}
		values.Before.Items = append(values.Before.Items, item)
		values.After.Items = append(values.After.Items, "Short")
	}
	assertBudgetFindingAcrossTemplates(t, "before-after-compact", values, "before.items", "holds about 12 lines")
}

func TestPhaseRoadmapBudgetWarningsReachFitReportAcrossTemplates(t *testing.T) {
	values := &patterns.PhaseRoadmapValues{}
	for i := 0; i < 6; i++ {
		values.Phases = append(values.Phases, patterns.PhaseRoadmapPhase{Name: "Plan", DateLabel: "Q1", Description: "Brief", Milestone: "Gate"})
	}
	values.Phases[2].DateLabel = strings.Repeat("D", 23)
	values.Phases[2].Milestone = strings.Repeat("M", 43)
	assertBudgetFindingAcrossTemplates(t, "phase-roadmap", values, "phases[2].date_label", "about 22 readable date-label characters")
	assertBudgetFindingAcrossTemplates(t, "phase-roadmap", values, "phases[2].milestone", "about 42 readable milestone characters")
}

// postExpandDeck builds a two-slide deck whose patterns both object to their
// own content: a chart panel with no chart, and two dense-grid bios over budget.
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
						{"name":"Arun Patel","role":"Lead Engineer","bio":"` + longBio + `"},
						{"name":"Lila Romero","role":"Design Lead","bio":"Short bio."},
						{"name":"Tom Becker","role":"Client Partner","bio":"Short bio."},
						{"name":"Maya Chen","role":"Analyst","bio":"Short bio."}
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
