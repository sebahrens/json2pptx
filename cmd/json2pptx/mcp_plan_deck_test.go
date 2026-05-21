package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sebahrens/json2pptx/internal/deckplan"
	"github.com/sebahrens/json2pptx/internal/patterns"
)

// testBuildPlan invokes the deckplan core wired with package main's
// render-coupled predictor — the same combination handlePlanDeck and make_deck
// use, so these tests exercise the cmd-side prediction seam (cell budgets and
// fit findings) end to end.
func testBuildPlan(reg *patterns.Registry, brief string, budget int, audience string, mustInclude []string) *deckplan.Result {
	return deckplan.BuildDeckPlan(reg, deckplan.Params{
		Brief:       brief,
		SlideBudget: budget,
		Audience:    audience,
		MustInclude: mustInclude,
	}, planPredictor{reg: reg})
}

func TestBuildDeckPlan_Basic12Slides(t *testing.T) {
	reg := patterns.Default()
	result := testBuildPlan(reg, "Pitch our Series B for an AI infra company", 12, "investors", nil)

	if len(result.Slides) != 12 {
		t.Fatalf("expected 12 slides, got %d", len(result.Slides))
	}

	// Verify slide indices are sequential.
	for i, s := range result.Slides {
		if s.SlideIndex != i {
			t.Errorf("slide %d has index %d", i, s.SlideIndex)
		}
	}

	// Verify opening and closing roles.
	if result.Slides[0].NarrativeRole != "opening" {
		t.Errorf("first slide should be opening, got %q", result.Slides[0].NarrativeRole)
	}
	if result.Slides[11].NarrativeRole != "closing" {
		t.Errorf("last slide should be closing, got %q", result.Slides[11].NarrativeRole)
	}

	// Verify no pattern runs exceed 2 (rhythm rule).
	if result.RhythmCheck.LongestPatternRun > 2 {
		t.Errorf("longest pattern run should be <=2, got %d", result.RhythmCheck.LongestPatternRun)
		for _, s := range result.Slides {
			t.Logf("  slide %d: %s (%s) — %s", s.SlideIndex, s.RecommendedPattern, s.NarrativeRole, s.Rationale)
		}
	}

	// Verify at least one emphasis pattern.
	if !result.RhythmCheck.HasEmphasis {
		t.Error("expected at least one emphasis pattern (stat-hero or pull-quote)")
	}

	// Verify all slides have patterns.
	for i, s := range result.Slides {
		if s.RecommendedPattern == "" {
			t.Errorf("slide %d has empty recommended_pattern", i)
		}
		if s.ContentSeed == "" {
			t.Errorf("slide %d has empty content_seed", i)
		}
		if s.Rationale == "" {
			t.Errorf("slide %d has empty rationale", i)
		}
	}

	// Verify pattern variety (should use more than 3 unique patterns for a 12-slide deck).
	if result.RhythmCheck.PatternVariety < 3 {
		t.Errorf("expected at least 3 unique patterns, got %d", result.RhythmCheck.PatternVariety)
	}
}

func TestBuildDeckPlan_MustInclude(t *testing.T) {
	reg := patterns.Default()
	result := testBuildPlan(reg, "Business model overview", 8, "", []string{"bmc-canvas", "kpi-3up"})

	// Verify must_include patterns appear in the plan.
	found := map[string]bool{}
	for _, s := range result.Slides {
		if s.RecommendedPattern == "bmc-canvas" || s.RecommendedPattern == "kpi-3up" {
			found[s.RecommendedPattern] = true
		}
	}

	if !found["bmc-canvas"] {
		t.Error("must_include pattern bmc-canvas not found in plan")
	}
	if !found["kpi-3up"] {
		t.Error("must_include pattern kpi-3up not found in plan")
	}
}

func TestBuildDeckPlan_MinBudget(t *testing.T) {
	reg := patterns.Default()
	result := testBuildPlan(reg, "Quick update", 3, "", nil)

	if len(result.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(result.Slides))
	}
	if result.Slides[0].NarrativeRole != "opening" {
		t.Errorf("first slide should be opening, got %q", result.Slides[0].NarrativeRole)
	}
	if result.Slides[2].NarrativeRole != "closing" {
		t.Errorf("last slide should be closing, got %q", result.Slides[2].NarrativeRole)
	}
}

func TestBuildDeckPlan_AttachesPredictions(t *testing.T) {
	reg := patterns.Default()
	result := testBuildPlan(reg, "Quarterly business review", 10, "executives", nil)

	if len(result.Slides) != 10 {
		t.Fatalf("expected 10 slides, got %d", len(result.Slides))
	}

	// Every slide should have alternatives populated (next-best ranked patterns).
	for i, s := range result.Slides {
		if len(s.Alternatives) == 0 {
			t.Errorf("slide %d (%s): expected at least 1 alternative, got 0", i, s.RecommendedPattern)
		}
		if len(s.Alternatives) > deckplan.MaxAlternatives {
			t.Errorf("slide %d: alternatives exceed cap %d, got %d", i, deckplan.MaxAlternatives, len(s.Alternatives))
		}
		for _, alt := range s.Alternatives {
			if alt.PatternName == s.RecommendedPattern {
				t.Errorf("slide %d: alternative %q matches the recommended pattern", i, alt.PatternName)
			}
			if alt.PatternName == "" {
				t.Errorf("slide %d: empty alternative pattern_name", i)
			}
		}
	}

	// At least one grid-shaped pattern in the deck should have non-empty
	// predicted_cell_budgets.
	anyBudgets := false
	for _, s := range result.Slides {
		if len(s.PredictedCellBudgets) > 0 {
			anyBudgets = true
			break
		}
	}
	if !anyBudgets {
		t.Error("expected at least one slide to have predicted_cell_budgets")
	}

	// predicted_findings is capped at maxPredictedFindings.
	for i, s := range result.Slides {
		if len(s.PredictedFindings) > maxPredictedFindings {
			t.Errorf("slide %d: predicted_findings exceeds cap %d, got %d",
				i, maxPredictedFindings, len(s.PredictedFindings))
		}
	}
}

func TestPredictCellBudgets_CardGrid(t *testing.T) {
	reg := patterns.Default()
	budgets := predictCellBudgetsForPattern(reg, "card-grid")
	if len(budgets) == 0 {
		t.Fatal("expected card-grid to produce predicted_cell_budgets")
	}
	for _, b := range budgets {
		if b.Columns <= 0 || b.Rows <= 0 {
			t.Errorf("budget has zero dimensions: %+v", b)
		}
		if b.BodyMaxChars <= 0 {
			t.Errorf("budget has zero body_max_chars: %+v", b)
		}
	}
}

func TestPredictCellBudgets_NonGridPattern(t *testing.T) {
	reg := patterns.Default()
	// pull-quote is not grid-shaped; should return nil.
	budgets := predictCellBudgetsForPattern(reg, "pull-quote")
	if budgets != nil {
		t.Errorf("expected nil budgets for pull-quote, got %+v", budgets)
	}
}

func TestPredictCellBudgets_UnknownPattern(t *testing.T) {
	reg := patterns.Default()
	if got := predictCellBudgetsForPattern(reg, "no-such-pattern"); got != nil {
		t.Errorf("expected nil for unknown pattern, got %+v", got)
	}
}

func TestBuildDeckPlan_AttachesSkeletonAndFallback(t *testing.T) {
	reg := patterns.Default()
	result := testBuildPlan(reg, "Pitch our Series B for an AI infra company", 5, "investors", nil)

	if len(result.Slides) != 5 {
		t.Fatalf("expected 5 slides, got %d", len(result.Slides))
	}

	for i, s := range result.Slides {
		if s.SuggestedPattern == "" {
			t.Errorf("slide %d: suggested_pattern is empty", i)
		}
		if s.SuggestedPattern != s.RecommendedPattern {
			t.Errorf("slide %d: suggested_pattern %q != recommended_pattern %q",
				i, s.SuggestedPattern, s.RecommendedPattern)
		}
		if len(s.Alternatives) > 0 && s.SuggestedPatternFallback == "" {
			t.Errorf("slide %d: alternatives present but suggested_pattern_fallback empty", i)
		}
		if s.SuggestedPatternFallback == s.RecommendedPattern && s.SuggestedPatternFallback != "" {
			t.Errorf("slide %d: fallback %q must differ from suggested pattern", i, s.SuggestedPatternFallback)
		}

		if len(s.Skeleton) == 0 {
			// Patterns without an Exemplar produce no skeleton; surface as a
			// soft warning so we know which patterns need an exemplar added.
			if _, ok := reg.Get(s.RecommendedPattern); !ok {
				t.Errorf("slide %d: skeleton missing and pattern %q not registered", i, s.RecommendedPattern)
				continue
			}
			t.Logf("slide %d (%s): no skeleton (pattern likely lacks Exemplar)", i, s.RecommendedPattern)
			continue
		}

		var slide map[string]any
		if err := json.Unmarshal(s.Skeleton, &slide); err != nil {
			t.Errorf("slide %d: skeleton is invalid JSON: %v", i, err)
			continue
		}
		if _, ok := slide["layout_id"]; !ok {
			t.Errorf("slide %d: skeleton missing layout_id", i)
		}
		if _, ok := slide["pattern"]; !ok {
			t.Errorf("slide %d: skeleton missing pattern envelope", i)
		}
		if !strings.Contains(string(s.Skeleton), patterns.FillPlaceholder) {
			t.Errorf("slide %d: skeleton has no %s placeholder", i, patterns.FillPlaceholder)
		}
	}
}

func TestBuildDeckPlan_SkeletonParsesAsSlideInput(t *testing.T) {
	// Every produced skeleton must decode into a SlideInput so an agent can
	// drop it straight into a PresentationInput.slides[] array and run
	// validate_input on the result.
	reg := patterns.Default()
	result := testBuildPlan(reg, "Quarterly business review", 7, "executives", nil)

	for i, s := range result.Slides {
		if len(s.Skeleton) == 0 {
			continue
		}
		var slide SlideInput
		if err := json.Unmarshal(s.Skeleton, &slide); err != nil {
			t.Errorf("slide %d skeleton does not decode as SlideInput: %v\n%s", i, err, string(s.Skeleton))
			continue
		}
		if slide.LayoutID == "" {
			t.Errorf("slide %d skeleton has empty layout_id after decode", i)
		}
		if slide.Pattern == nil {
			t.Errorf("slide %d skeleton has nil Pattern after decode", i)
			continue
		}
		if slide.Pattern.Name == "" {
			t.Errorf("slide %d skeleton has empty Pattern.Name", i)
		}
		if len(slide.Pattern.Values) == 0 {
			t.Errorf("slide %d skeleton has empty Pattern.Values", i)
		}
		if len(slide.Content) == 0 {
			t.Errorf("slide %d skeleton has no content[] entries", i)
		}
	}
}
