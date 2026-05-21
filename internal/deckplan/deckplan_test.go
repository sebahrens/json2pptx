package deckplan

import (
	"testing"

	"github.com/sebahrens/json2pptx/internal/patterns"
)

func TestDistributeRoles(t *testing.T) {
	tests := []struct {
		budget    int
		wantFirst string
		wantLast  string
	}{
		{3, "opening", "closing"},
		{5, "opening", "closing"},
		{10, "opening", "closing"},
		{20, "opening", "closing"},
	}

	for _, tt := range tests {
		roles := distributeRoles(tt.budget)
		if len(roles) != tt.budget {
			t.Errorf("budget %d: got %d roles", tt.budget, len(roles))
		}
		if roles[0] != tt.wantFirst {
			t.Errorf("budget %d: first role = %q, want %q", tt.budget, roles[0], tt.wantFirst)
		}
		if roles[len(roles)-1] != tt.wantLast {
			t.Errorf("budget %d: last role = %q, want %q", tt.budget, roles[len(roles)-1], tt.wantLast)
		}
		// All roles should be non-empty.
		for i, r := range roles {
			if r == "" {
				t.Errorf("budget %d: role at index %d is empty", tt.budget, i)
			}
		}
	}
}

func TestComputeAlternatives_ExcludesRecommended(t *testing.T) {
	reg := patterns.Default()
	slides := []Slide{
		{SlideIndex: 0, NarrativeRole: "opening", RecommendedPattern: "stat-hero"},
		{SlideIndex: 1, NarrativeRole: "evidence", RecommendedPattern: "card-grid"},
		{SlideIndex: 2, NarrativeRole: "closing", RecommendedPattern: "pull-quote"},
	}
	alts := computeAlternativesForSlot(reg, slides, 1, "demo brief", "team")
	for _, alt := range alts {
		if alt.PatternName == "card-grid" {
			t.Errorf("alternatives must not include the recommended pattern: %+v", alts)
		}
	}
}

func TestEnforceRhythm_BreaksLongRuns(t *testing.T) {
	reg := patterns.Default()

	// Create slides with a 5-slide run of the same pattern.
	slides := make([]Slide, 7)
	for i := range slides {
		slides[i] = Slide{
			SlideIndex:         i,
			NarrativeRole:      "evidence",
			RecommendedPattern: "card-grid",
			ContentSeed:        "test",
			Rationale:          "test",
		}
	}
	slides[0].NarrativeRole = "opening"
	slides[6].NarrativeRole = "closing"

	result := enforceRhythm(reg, slides, "test")

	// Count longest run.
	longestRun := 1
	currentRun := 1
	for i := 1; i < len(result); i++ {
		if result[i].RecommendedPattern == result[i-1].RecommendedPattern {
			currentRun++
			if currentRun > longestRun {
				longestRun = currentRun
			}
		} else {
			currentRun = 1
		}
	}

	if longestRun > 2 {
		t.Errorf("after enforceRhythm, longest run should be <=2, got %d", longestRun)
		for _, s := range result {
			t.Logf("  slide %d: %s (%s)", s.SlideIndex, s.RecommendedPattern, s.NarrativeRole)
		}
	}
}

// TestBuildDeckPlan_NilPredictor verifies the template-agnostic planning core
// works without a predictor: roles, rhythm, alternatives, and skeletons are all
// populated, while the render-coupled forecasts (cell budgets, fit findings)
// stay empty.
func TestBuildDeckPlan_NilPredictor(t *testing.T) {
	reg := patterns.Default()
	result := BuildDeckPlan(reg, Params{
		Brief:       "Pitch our Series B for an AI infra company",
		SlideBudget: 10,
		Audience:    "investors",
	}, nil)

	if len(result.Slides) != 10 {
		t.Fatalf("expected 10 slides, got %d", len(result.Slides))
	}
	if result.Slides[0].NarrativeRole != "opening" {
		t.Errorf("first slide should be opening, got %q", result.Slides[0].NarrativeRole)
	}
	if result.Slides[9].NarrativeRole != "closing" {
		t.Errorf("last slide should be closing, got %q", result.Slides[9].NarrativeRole)
	}
	if result.RhythmCheck.LongestPatternRun > 2 {
		t.Errorf("longest pattern run should be <=2, got %d", result.RhythmCheck.LongestPatternRun)
	}

	for i, s := range result.Slides {
		if s.RecommendedPattern == "" {
			t.Errorf("slide %d has empty recommended_pattern", i)
		}
		if s.SuggestedPattern != s.RecommendedPattern {
			t.Errorf("slide %d: suggested_pattern %q != recommended_pattern %q", i, s.SuggestedPattern, s.RecommendedPattern)
		}
		if len(s.Alternatives) == 0 {
			t.Errorf("slide %d: expected at least 1 alternative", i)
		}
		// A nil predictor must not synthesize render-coupled forecasts.
		if s.PredictedCellBudgets != nil {
			t.Errorf("slide %d: predicted_cell_budgets should be nil without a predictor", i)
		}
		if s.PredictedFindings != nil {
			t.Errorf("slide %d: predicted_findings should be nil without a predictor", i)
		}
	}

	// Template-agnostic plan: no template echo, no template_support.
	if result.Template != "" {
		t.Errorf("template should be empty for a template-agnostic plan, got %q", result.Template)
	}
	for _, s := range result.Slides {
		if s.TemplateSupport != nil {
			t.Errorf("slide %d: template_support should be nil without template context", s.SlideIndex)
		}
	}
}

func TestContainsStr(t *testing.T) {
	cases := []struct {
		name   string
		slice  []string
		needle string
		want   bool
	}{
		{"present", []string{"a", "b", "c"}, "b", true},
		{"missing", []string{"a", "b", "c"}, "z", false},
		{"empty", nil, "x", false},
		{"empty needle in empty slice", nil, "", false},
		{"empty needle present", []string{"", "a"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := containsStr(tc.slice, tc.needle); got != tc.want {
				t.Errorf("containsStr(%v, %q) = %v, want %v", tc.slice, tc.needle, got, tc.want)
			}
		})
	}
}

func TestTruncateBrief(t *testing.T) {
	if got := TruncateBrief("short", 80); got != "short" {
		t.Errorf("short string should pass through unchanged, got %q", got)
	}
	long := "this is a fairly long brief that should be truncated to a much smaller budget"
	got := TruncateBrief(long, 20)
	if len(got) != 20 {
		t.Errorf("expected truncated length 20, got %d (%q)", len(got), got)
	}
	if got[len(got)-3:] != "..." {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
}
