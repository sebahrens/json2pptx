package main

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckinput"
	"github.com/sebahrens/json2pptx/internal/patterns"
	"github.com/sebahrens/json2pptx/internal/types"
)

// titleSlide builds a slide carrying one title text item.
func takeawayTitleSlide(title string) SlideInput {
	t := title
	return SlideInput{Content: []ContentInput{{
		PlaceholderID: "title", Type: "text", TextValue: &t,
	}}}
}

// TestSlideRequiresTakeaway_FollowsPatternTaxonomy pins the trigger set. The
// old test was a "matrix-" name prefix, so the nudge landed on exactly one
// pattern and never on chart-insights-split — the pattern that embeds a chart
// (go-slide-creator-g2cy).
func TestSlideRequiresTakeaway_FollowsPatternTaxonomy(t *testing.T) {
	dataVisual := []string{
		"chart-insights-split", "waterfall-bridge",
		"horizontal-bar-with-callouts", "table-highlight", "matrix-2x2",
	}
	for _, name := range dataVisual {
		t.Run(name, func(t *testing.T) {
			p, ok := patterns.Default().Get(name)
			if !ok {
				t.Fatalf("%s is not registered", name)
			}
			if !p.Taxonomy().DataVisual {
				t.Errorf("%s must be marked data_visual", name)
			}
			s := takeawayTitleSlide("Options")
			s.Pattern = &deckinput.PatternInput{Name: name}
			if !slideRequiresTakeaway(s) {
				t.Errorf("%s should require a takeaway", name)
			}
		})
	}

	// Patterns that display content without making a quantitative claim keep
	// their silence: the nudge is for slides arguing from data.
	for _, name := range []string{"card-grid", "icon-row", "agenda", "pull-quote", "process-flow"} {
		t.Run("quiet/"+name, func(t *testing.T) {
			if p, ok := patterns.Default().Get(name); ok && p.Taxonomy().DataVisual {
				t.Fatalf("%s should not be marked data_visual", name)
			}
			s := takeawayTitleSlide("Our approach")
			s.Pattern = &deckinput.PatternInput{Name: name}
			if slideRequiresTakeaway(s) {
				t.Errorf("%s should not require a takeaway", name)
			}
		})
	}
}

// TestSlideRequiresTakeaway_ChartContent keeps the content-item half of the
// trigger working alongside the pattern half.
func TestSlideRequiresTakeaway_ChartContent(t *testing.T) {
	chart := takeawayTitleSlide("Revenue")
	chart.Content = append(chart.Content, ContentInput{
		PlaceholderID: "body", Type: "diagram",
		DiagramValue: &types.DiagramSpec{Type: "bar_chart"},
	})
	if !slideRequiresTakeaway(chart) {
		t.Error("a chart-shaped diagram should require a takeaway")
	}

	structural := takeawayTitleSlide("How the migration runs")
	structural.Content = append(structural.Content, ContentInput{
		PlaceholderID: "body", Type: "diagram",
		DiagramValue: &types.DiagramSpec{Type: "process_flow"},
	})
	if slideRequiresTakeaway(structural) {
		t.Error("a structural diagram makes no quantitative claim")
	}
}

// TestSlideTitleStatesTakeaway covers the suppression: the one slide the old
// lint did fire on was a matrix titled "Prioritise the four initiatives in the
// top-right quadrant", which IS the takeaway — asking for a second sentence
// saying the same thing reads as a false positive.
func TestSlideTitleStatesTakeaway(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{"Prioritise the four initiatives in the top-right quadrant", true},
		{"Enterprise carried the year, SMB gave back growth", true},
		{"Revenue grew 18% against a 12% plan this quarter", true},
		{"The mandate is the opportunity and the threat", true},
		{"We must fund the SMB success pod in Q3", true},

		{"Options matrix", false},
		{"Revenue by segment", false},
		{"Cost drivers across the four regions we operate", false}, // long, but a label
		{"Grew", false},                                            // a verb, but not a sentence
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if got := slideTitleStatesTakeaway(takeawayTitleSlide(tt.title)); got != tt.want {
				t.Errorf("slideTitleStatesTakeaway(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}
