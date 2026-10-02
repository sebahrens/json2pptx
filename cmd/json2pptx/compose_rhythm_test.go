package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/rhythm"
	"github.com/sebahrens/json2pptx/internal/testutil"
	"github.com/sebahrens/json2pptx/internal/types"
)

// go-slide-creator-dzv7d. The 2026-10-02 layout-composition review's three
// schema-valid composed slides (vertical panels+quote, horizontal KPI+flow,
// vertical quote+KPI) all fingerprinted as "compose" and drew a phantom
// break_run. The fixture is the review's exact analyze_deck_rhythm arguments.

func diverseComposeArgs(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile("testdata/rhythm/diverse_compose_args.json")
	if err != nil {
		t.Fatal(err)
	}
	var args map[string]any
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatal(err)
	}
	return args
}

// rhythmTemplates is midnight-blue plus the local p-style template when it is
// present; the rhythm projection must not depend on the deck's template.
func rhythmTemplates() []string {
	names := []string{"midnight-blue"}
	for _, name := range testutil.AllTestTemplateNames() {
		if name == "p-style" {
			names = append(names, name)
		}
	}
	return names
}

func withTemplate(args map[string]any, tpl string) map[string]any {
	pres := args["presentation"].(map[string]any)
	out := make(map[string]any, len(pres))
	for k, v := range pres {
		out[k] = v
	}
	out["template"] = tpl
	return out
}

func TestAnalyzeDeckRhythm_DiverseComposeFixtureHasNoPhantomRun(t *testing.T) {
	mc := testMCPConfig(t)
	args := diverseComposeArgs(t)
	for _, tpl := range rhythmTemplates() {
		t.Run(tpl, func(t *testing.T) {
			res, err := mc.handleAnalyzeDeckRhythm(context.Background(), makeRequest(map[string]any{
				"presentation": withTemplate(args, tpl),
			}))
			if err != nil || res.IsError {
				t.Fatalf("analyze_deck_rhythm: %v %s", err, resultText(res))
			}
			var got rhythm.Result
			structuredInto(t, res.StructuredContent, &got)

			seen := map[string]bool{}
			for _, s := range got.PerSlide {
				if !strings.HasPrefix(s.Pattern, "compose:") {
					t.Errorf("slide %d pattern %q lost its composed structure", s.SlideIndex, s.Pattern)
				}
				if seen[s.Pattern] {
					t.Errorf("slide %d pattern %q repeats a different composition's fingerprint", s.SlideIndex, s.Pattern)
				}
				seen[s.Pattern] = true
			}
			if len(got.Aggregates.PatternRuns) != 0 || got.Aggregates.LongestRun != 0 || got.Aggregates.RepetitionIndex != 0 {
				t.Errorf("distinct compositions read as repetition: runs=%+v longest=%d repetition=%.2f",
					got.Aggregates.PatternRuns, got.Aggregates.LongestRun, got.Aggregates.RepetitionIndex)
			}
			for _, rec := range got.Recommendations {
				if rec.Code == rhythm.CodeBreakRun {
					t.Errorf("compose-only break_run: %+v", rec)
				}
			}
			if got.CompositionScore != 100 {
				t.Errorf("composition_score = %d, want 100 (was 80 from the phantom run)", got.CompositionScore)
			}

			// score_deck's composition axis and candidate scoring use the
			// same fingerprint.
			var input PresentationInput
			raw, _ := json.Marshal(withTemplate(args, tpl))
			if err := json.Unmarshal(raw, &input); err != nil {
				t.Fatal(err)
			}
			axis := compositionAxis(input.Slides)
			for _, d := range axis.Diagnostics {
				if d.Code == "pattern_run" {
					t.Errorf("score_deck composition flags a compose run: %+v", d)
				}
			}
			if pen, notes := rhythmPenaltyAt(input.Slides, 1); pen != 0 {
				t.Errorf("candidate rhythm penalty = %d (%v), want 0 between distinct compositions", pen, notes)
			}
		})
	}
}

// repeatedComposeSlides are three equivalent compositions: the same vertical
// KPI + quote structure, with the regions reordered, a 55/45 split and a
// different KPI count — trivial changes that must not hide the monotony.
func repeatedComposeSlides() []SlideInput {
	kpi := func(name string, size float64) SegmentInput {
		return SegmentInput{Pattern: PatternInput{Name: name, Values: json.RawMessage(`[{"big":"42%","small":"Win rate"}]`)}, SizePct: size}
	}
	quote := func(size float64) SegmentInput {
		return SegmentInput{Pattern: PatternInput{Name: "pull-quote", Values: json.RawMessage(`{"quote":"Adoption gives confidence.","attribution":"Customer lead"}`)}, SizePct: size}
	}
	return []SlideInput{
		{LayoutID: "blank-title", Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{kpi("kpi-3up", 0), quote(0)}}},
		{LayoutID: "blank-title", Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{quote(45), kpi("kpi-4up", 55)}}},
		{LayoutID: "blank-title", Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{kpi("kpi-3up", 0), quote(0)}}},
	}
}

func TestAnalyzeDeckRhythm_RepeatedComposeStillBreaksRun(t *testing.T) {
	slides := repeatedComposeSlides()
	result := analyzeDeckRhythm(slides)
	if result.Aggregates.LongestRun != 3 {
		t.Fatalf("longest_run = %d, want 3; per_slide %+v", result.Aggregates.LongestRun, result.PerSlide)
	}
	var breakRun *rhythm.Recommendation
	for i := range result.Recommendations {
		if result.Recommendations[i].Code == rhythm.CodeBreakRun {
			breakRun = &result.Recommendations[i]
		}
	}
	if breakRun == nil || len(breakRun.RecommendedBreak) == 0 {
		t.Fatalf("no actionable break_run for repeated compositions: %+v", result.Recommendations)
	}
	for _, name := range breakRun.RecommendedBreak {
		if strings.HasPrefix(name, "kpi-") || name == "pull-quote" {
			t.Errorf("break advice %q repeats a region every slide of the run already shows", name)
		}
	}

	axis := compositionAxis(slides)
	found := false
	for _, d := range axis.Diagnostics {
		found = found || d.Code == "pattern_run"
	}
	if !found {
		t.Errorf("score_deck composition missed the repeated compose run: %+v", axis.Diagnostics)
	}
	if pen, _ := rhythmPenaltyAt(slides, 1); pen != 15 {
		t.Errorf("candidate rhythm penalty = %d, want 15 for a run of three equivalent compositions", pen)
	}
}

func TestToRhythmSlide_ProjectsComposeStructure(t *testing.T) {
	slide := SlideInput{Compose: &ComposeInput{Direction: "horizontal", Segments: []SegmentInput{
		{Pattern: PatternInput{Name: "kpi-3up"}, SizePct: 40},
		{Compose: &ComposeInput{Direction: "vertical", Segments: []SegmentInput{
			{Pattern: PatternInput{Name: "pull-quote"}},
			{Diagram: &types.DiagramSpec{Type: "bar_chart"}},
			{Diagram: &types.DiagramSpec{Type: "org_chart"}},
		}}},
	}}}
	got := toRhythmSlide(slide).Compose
	if got == nil || got.Direction != "horizontal" || len(got.Regions) != 2 {
		t.Fatalf("compose projection = %+v", got)
	}
	if got.Regions[0].Visual != "kpi-3up" || got.Regions[0].SizePct != 40 {
		t.Errorf("leaf region = %+v", got.Regions[0])
	}
	nested := got.Regions[1].Nested
	if nested == nil || nested.Direction != "vertical" || len(nested.Regions) != 3 {
		t.Fatalf("nested projection = %+v", nested)
	}
	if nested.Regions[1].Visual != "chart" || nested.Regions[2].Visual != "diagram" {
		t.Errorf("diagram regions = %+v / %+v, want chart / diagram", nested.Regions[1], nested.Regions[2])
	}
}
